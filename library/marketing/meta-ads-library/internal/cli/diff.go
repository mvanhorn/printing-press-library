// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source auto
// pp:client-call

package cli

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/meta-ads-library/internal/adlibrary"
	"github.com/mvanhorn/printing-press-library/library/marketing/meta-ads-library/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/marketing/meta-ads-library/internal/store"
	"github.com/spf13/cobra"
)

func newNovelDiffCmd(flags *rootFlags) *cobra.Command {
	var refresh bool
	var country string
	var limit int
	var maxPages int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "diff [page-or-id]",
		Short: "See exactly which ads an advertiser launched, stopped, or scaled between timestamped snapshots.",
		Long: `Use this command to see which ads an advertiser launched, stopped, or scaled between timestamped snapshots.
Do NOT use this command to rank longest-running evergreen ads (use 'winners') or benchmark multiple advertisers side by side (use 'compare').`,
		Example: strings.Trim(`
  meta-ads-library-pp-cli diff 15087023444 --refresh --country US --agent
  meta-ads-library-pp-cli diff 15087023444 --json
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "auto",
			"pp:happy-args":  "page=15087023444",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "diff")
			}
			if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("facebook page ID or advertiser name is required"))
			}
			pageArg := strings.TrimSpace(args[0])

			if flags.dataSource == "local" && refresh {
				return usageErr(fmt.Errorf("--refresh conflicts with --data-source local"))
			}
			if flags.dataSource == "live" {
				refresh = true
			}

			resolvedDB := dbPath
			if resolvedDB == "" {
				resolvedDB = defaultDBPath("meta-ads-library-pp-cli")
			}

			if !refresh {
				if _, statErr := os.Stat(resolvedDB); os.IsNotExist(statErr) {
					fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: meta-ads-library-pp-cli ads search --page-id %s --country %s\n", resolvedDB, pageArg, country)
					empty := adlibrary.DiffReport{
						PageID:   pageArg,
						Launched: make([]adlibrary.DiffAdBrief, 0),
						Stopped:  make([]adlibrary.DiffAdBrief, 0),
						Scaled:   make([]adlibrary.DiffScaledAd, 0),
						Note:     "no local mirror exists yet; run ads search or pass --refresh to capture a snapshot",
					}
					if !wantsHumanTable(cmd.OutOrStdout(), flags) {
						return printJSONFiltered(cmd.OutOrStdout(), empty, flags)
					}
					return nil
				}
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if refresh && !cliutil.IsVerifyEnv() {
				if localDataDisabled() {
					return fmt.Errorf("diff --refresh is disabled while %s=1", noLocalDataEnvVar)
				}
				fetchPages := maxPages
				fetchLimit := limit
				if cliutil.IsDogfoodEnv() {
					fetchPages = 1
					if fetchLimit > 10 {
						fetchLimit = 10
					}
				}
				client := newAdLibraryClient(flags)
				pageID := pageArg
				pageName := ""
				// If non-numeric, resolve via typeahead first.
				if !isAllDigits(pageID) {
					advs, err := client.SearchAdvertisers(ctx, pageArg, country)
					if err != nil {
						return classifyAdLibraryErr(cmd, flags, err)
					}
					top, ok := firstValidAdvertiser(advs)
					if !ok {
						return notFoundErr(fmt.Errorf("no advertiser page found matching %q in country %s", pageArg, country))
					}
					pageID = top.PageID
					pageName = top.PageName
				}
				sr, err := client.SearchAds(ctx, adlibrary.SearchParams{
					PageIDs:      []string{pageID},
					Country:      country,
					ActiveStatus: "ACTIVE",
					SearchType:   "PAGE",
					Limit:        fetchLimit,
					MaxPages:     fetchPages,
				})
				if err != nil {
					return classifyAdLibraryErr(cmd, flags, err)
				}
				db, err := store.OpenWithContext(ctx, resolvedDB)
				if err != nil {
					return err
				}
				if err := adlibrary.SaveAdsBatch(ctx, db, sr.Ads); err != nil {
					_ = db.Close()
					return err
				}
				if pageName == "" && len(sr.Ads) > 0 {
					pageName = sr.Ads[0].PageName
				}
				if _, err := adlibrary.RecordSnapshot(ctx, db, pageID, pageName, country, "", sr.Ads, time.Now().UTC()); err != nil {
					_ = db.Close()
					return err
				}
				pageArg = pageID
				_ = db.Close()
			}

			if _, statErr := os.Stat(resolvedDB); os.IsNotExist(statErr) {
				empty := adlibrary.DiffReport{
					PageID:   pageArg,
					Launched: make([]adlibrary.DiffAdBrief, 0),
					Stopped:  make([]adlibrary.DiffAdBrief, 0),
					Scaled:   make([]adlibrary.DiffScaledAd, 0),
					Note:     "no snapshots stored locally yet",
				}
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), empty, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), empty.Note)
				return nil
			}

			db, err := store.OpenReadOnlyContext(ctx, resolvedDB)
			if err != nil {
				return err
			}
			defer db.Close()

			if !refresh {
				if !hintIfUnsynced(cmd, db, "ads") {
					hintIfStale(cmd, db, "ads", flags.maxAge)
				}
			}

			report, err := adlibrary.ComputeDiff(ctx, db, pageArg)
			if err != nil {
				return err
			}
			if !refresh && report.SnapshotCount == 0 && !isAllDigits(pageArg) {
				nf := notFoundErr(fmt.Errorf("advertiser %q not found in local snapshots; pass a numeric Facebook Page ID or --refresh", pageArg))
				writeAPIErrorEnvelope(cmd.OutOrStdout(), flags, nf, ExitCode(nf))
				return nf
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), report, flags)
			}
			rows := [][]string{
				{"Page", fmt.Sprintf("%s (%s)", report.PageName, report.PageID)},
				{"Snapshots", fmt.Sprintf("%d", report.SnapshotCount)},
				{"Previous Snapshot", report.PreviousSnapshot},
				{"Current Snapshot", report.CurrentSnapshot},
				{"Active Volume", fmt.Sprintf("%d -> %d (%+d)", report.PreviousActive, report.CurrentActive, report.NetChange)},
				{"Launched", fmt.Sprintf("%d", len(report.Launched))},
				{"Stopped", fmt.Sprintf("%d", len(report.Stopped))},
				{"Scaled (Collation Delta)", fmt.Sprintf("%d", len(report.Scaled))},
			}
			if report.Note != "" {
				rows = append(rows, []string{"Note", report.Note})
			}
			return flags.printTable(cmd, []string{"METRIC", "VALUE"}, rows)
		},
	}

	cmd.Flags().BoolVar(&refresh, "refresh", false, "Fetch a fresh live snapshot from Meta Ad Library before computing the diff")
	cmd.Flags().StringVarP(&country, "country", "c", "US", "ISO country code used when --refresh fetches a live snapshot")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum active ads to fetch when --refresh is set")
	cmd.Flags().IntVar(&maxPages, "max-pages", 3, "Maximum GraphQL pages to fetch when --refresh is set")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to local SQLite database")
	return cmd
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
