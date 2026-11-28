// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/meta-ads-library/internal/adlibrary"
	"github.com/mvanhorn/printing-press-library/library/marketing/meta-ads-library/internal/store"
	"github.com/spf13/cobra"
)

func newNovelWinnersCmd(flags *rootFlags) *cobra.Command {
	var minDays int
	var minCollation int
	var limit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "winners [page-or-id]",
		Short: "Rank active ads by proven longevity (days running), collation scale, and platform breadth.",
		Long: `Use this command to rank an advertiser's longest-running and highest-collation winning ads.
Do NOT use this command to track newly launched or stopped ads over time (use 'diff'), inspect copy hooks (use 'hooks'), or map landing pages (use 'funnels').`,
		Example: strings.Trim(`
  meta-ads-library-pp-cli winners 15087023444 --min-days 30 --agent
  meta-ads-library-pp-cli winners --min-days 14 --limit 15 --json
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "local",
			"pp:happy-args":  "--limit=5",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "winners")
			}

			pageFilter := ""
			if len(args) > 0 {
				pageFilter = strings.TrimSpace(args[0])
			}

			resolvedDB := dbPath
			if resolvedDB == "" {
				resolvedDB = defaultDBPath("meta-ads-library-pp-cli")
			}
			if _, statErr := os.Stat(resolvedDB); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: meta-ads-library-pp-cli ads search --query nike --country US\n", resolvedDB)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), make([]adlibrary.WinnerRow, 0), flags)
				}
				return nil
			}

			db, err := store.OpenReadOnlyContext(cmd.Context(), resolvedDB)
			if err != nil {
				return err
			}
			defer db.Close()

			if !hintIfUnsynced(cmd, db, "ads") {
				hintIfStale(cmd, db, "ads", flags.maxAge)
			}

			rows, err := adlibrary.ComputeWinners(cmd.Context(), db, pageFilter, minDays, minCollation, limit)
			if err != nil {
				return err
			}
			if pageFilter != "" && len(rows) == 0 {
				nf := notFoundErr(fmt.Errorf("no active ads found for %q in local SQLite; run 'meta-ads-library-pp-cli ads search --page-name %q --country US' first", pageFilter, pageFilter))
				writeAPIErrorEnvelope(cmd.OutOrStdout(), flags, nf, ExitCode(nf))
				return nf
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No active winning ads match the current filters in local SQLite.")
				return nil
			}
			table := make([][]string, 0, len(rows))
			for _, r := range rows {
				hook := truncate(r.OpeningHook, 50)
				table = append(table, []string{
					fmt.Sprintf("#%d", r.Rank),
					r.AdArchiveID,
					r.PageName,
					fmt.Sprintf("%dd", r.DaysRunning),
					fmt.Sprintf("%dx", r.CollationCount),
					fmt.Sprintf("%.0f", r.WinnerScore),
					r.DisplayFormat,
					r.LandingDomain,
					hook,
				})
			}
			return flags.printTable(cmd, []string{"RANK", "AD ID", "PAGE", "DAYS", "COLL", "SCORE", "FORMAT", "DOMAIN", "HOOK"}, table)
		},
	}

	cmd.Flags().IntVar(&minDays, "min-days", 0, "Minimum active days running (e.g. 30, 60, 90 for evergreen winners)")
	cmd.Flags().IntVar(&minCollation, "min-collation", 0, "Minimum active collation count (scale filter)")
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum ranked winning ads to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to local SQLite database")
	return cmd
}
