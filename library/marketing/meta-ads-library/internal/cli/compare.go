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

func newNovelCompareCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "compare [page1] [page2] [page3...]",
		Short: "Benchmark multiple advertisers side by side on active volume, median age, format mix, platforms, CTAs, and refresh velocity.",
		Long: `Use this command to benchmark 2-5 known advertisers side by side on volume, format mix, platforms, CTAs, and refresh velocity.
Do NOT use this command to discover who dominates a keyword niche without naming competitors upfront (use 'sov') or inspect single-advertiser snapshot changes (use 'diff').`,
		Example: strings.Trim(`
  meta-ads-library-pp-cli compare 15087023444 182162001806727 --agent
  meta-ads-library-pp-cli compare "Nike" "Adidas" --json
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "local",
			"pp:happy-args":  "page1=15087023444;page2=182162001806727",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "compare")
			}
			if len(args) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("compare requires at least 2 advertiser page IDs or names"))
			}

			resolvedDB := dbPath
			if resolvedDB == "" {
				resolvedDB = defaultDBPath("meta-ads-library-pp-cli")
			}
			if _, statErr := os.Stat(resolvedDB); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: meta-ads-library-pp-cli ads search --page-id <page_id> --country US\n", resolvedDB)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), make([]adlibrary.CompetitorBenchmark, 0), flags)
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

			results, err := adlibrary.ComputeCompare(cmd.Context(), db, args)
			if err != nil {
				return err
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}
			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No local advertiser data found to compare.")
				return nil
			}
			table := make([][]string, 0, len(results))
			for _, r := range results {
				name := r.PageName
				if name == "" {
					name = r.Input
				}
				table = append(table, []string{
					name,
					fmt.Sprintf("%d", r.ActiveAds),
					fmt.Sprintf("%d", r.TotalCollations),
					fmt.Sprintf("%dd", r.MedianDaysRunning),
					fmt.Sprintf("%d/%d", r.LaunchedLast7d, r.LaunchedLast30d),
					fmt.Sprintf("%d", r.UniqueDomains),
					strings.Join(r.TopCTAs, ", "),
				})
			}
			return flags.printTable(cmd, []string{"ADVERTISER", "ACTIVE", "COLLATIONS", "MEDIAN AGE", "NEW 7D/30D", "DOMAINS", "TOP CTAS"}, table)
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Path to local SQLite database")
	return cmd
}
