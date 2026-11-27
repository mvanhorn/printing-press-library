// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source auto
// pp:client-call

package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/meta-ads-library/internal/adlibrary"
	"github.com/mvanhorn/printing-press-library/library/marketing/meta-ads-library/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/marketing/meta-ads-library/internal/store"
	"github.com/spf13/cobra"
)

func newNovelSovCmd(flags *rootFlags) *cobra.Command {
	var live bool
	var country string
	var limit int
	var maxPages int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "sov [keyword]",
		Short: "Rank which advertisers dominate a category keyword by active ad count, collation-weighted volume, dominant format, and median longevity.",
		Long: `Use this command to rank which advertisers dominate a category keyword by active ad count, collation-weighted volume, dominant format, and median longevity.
Do NOT use this command to benchmark a known list of specific competitors (use 'compare') or retrieve raw ad creatives for a keyword (use 'ads search').`,
		Example: strings.Trim(`
  meta-ads-library-pp-cli sov "running shoes" --country US --live --agent
  meta-ads-library-pp-cli sov "cold plunge" --limit 15 --json
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "auto",
			"pp:happy-args":  "keyword=nike",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "sov")
			}
			keyword := strings.TrimSpace(strings.Join(args, " "))
			if keyword == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("category keyword is required"))
			}

			if flags.dataSource == "local" && live {
				return usageErr(fmt.Errorf("--live conflicts with --data-source local"))
			}
			if flags.dataSource == "live" {
				live = true
			}

			resolvedDB := dbPath
			if resolvedDB == "" {
				resolvedDB = defaultDBPath("meta-ads-library-pp-cli")
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			var liveAds []adlibrary.Ad
			liveFetched := false
			if live && !cliutil.IsVerifyEnv() {
				fetchPages := maxPages
				fetchLimit := 40
				if cliutil.IsDogfoodEnv() {
					fetchPages = 1
					fetchLimit = 10
				}
				client := newAdLibraryClient(flags)
				sr, err := client.SearchAds(ctx, adlibrary.SearchParams{
					Query:        keyword,
					Country:      country,
					ActiveStatus: "ACTIVE",
					Limit:        fetchLimit,
					MaxPages:     fetchPages,
				})
				if err != nil {
					return classifyAdLibraryErr(cmd, flags, err)
				}
				liveAds = sr.Ads
				liveFetched = true
				if !localDataDisabled() && len(sr.Ads) > 0 {
					db, err := store.OpenWithContext(ctx, resolvedDB)
					if err != nil {
						return err
					}
					if err := adlibrary.SaveAdsBatch(ctx, db, sr.Ads); err != nil {
						_ = db.Close()
						return err
					}
					_ = db.Close()
				}
			}

			var rows []adlibrary.SOVRow
			if liveFetched && (localDataDisabled() || flags.dataSource == "live" || len(liveAds) == 0) {
				rows = adlibrary.ComputeSOVFromAds(liveAds, keyword, limit)
			} else {
				if _, statErr := os.Stat(resolvedDB); os.IsNotExist(statErr) {
					fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: meta-ads-library-pp-cli sov %q --live --country %s\n", resolvedDB, keyword, country)
					if !wantsHumanTable(cmd.OutOrStdout(), flags) {
						return printJSONFiltered(cmd.OutOrStdout(), make([]adlibrary.SOVRow, 0), flags)
					}
					return nil
				}

				db, err := store.OpenReadOnlyContext(ctx, resolvedDB)
				if err != nil {
					return err
				}
				defer db.Close()

				if !live {
					if !hintIfUnsynced(cmd, db, "ads") {
						hintIfStale(cmd, db, "ads", flags.maxAge)
					}
				}

				rows, err = adlibrary.ComputeSOV(ctx, db, keyword, limit)
				if err != nil {
					return err
				}
				if !live && len(rows) == 0 {
					nf := notFoundErr(fmt.Errorf("no advertisers matched keyword %q in local SQLite (add --live to fetch from Meta Ad Library)", keyword))
					writeAPIErrorEnvelope(cmd.OutOrStdout(), flags, nf, ExitCode(nf))
					return nf
				}
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No advertisers matched keyword %q in local SQLite (add --live to fetch from Meta Ad Library).\n", keyword)
				return nil
			}
			table := make([][]string, 0, len(rows))
			for _, r := range rows {
				table = append(table, []string{
					fmt.Sprintf("#%d", r.Rank),
					r.PageName,
					fmt.Sprintf("%d (%.1f%%)", r.ActiveAds, r.AdSharePct),
					fmt.Sprintf("%d (%.1f%%)", r.TotalCollations, r.CollationSharePct),
					r.DominantFormat,
					fmt.Sprintf("%dd", r.MedianDaysRunning),
					r.TopLandingDomain,
				})
			}
			return flags.printTable(cmd, []string{"RANK", "ADVERTISER", "ACTIVE ADS", "COLLATIONS", "FORMAT", "MEDIAN AGE", "TOP DOMAIN"}, table)
		},
	}

	cmd.Flags().BoolVar(&live, "live", false, "Fetch fresh ads for this keyword from Meta Ad Library before computing share of voice")
	cmd.Flags().StringVarP(&country, "country", "c", "US", "ISO country code used when --live fetches fresh ads")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum ranked advertisers to return")
	cmd.Flags().IntVar(&maxPages, "max-pages", 2, "Maximum GraphQL pages to fetch when --live is set")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to local SQLite database")
	return cmd
}
