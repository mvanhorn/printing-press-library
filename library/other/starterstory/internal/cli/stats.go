// Copyright 2026 waveriderai and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: corpus statistics over the local index.

package cli

import (
	"database/sql"
	"fmt"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/other/starterstory/internal/store"
	"github.com/spf13/cobra"
)

// pp:data-source local
func newNovelStatsCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:         "stats",
		Short:       "Counts by section plus the revenue distribution across the case-study corpus.",
		Example:     "  starterstory-pp-cli stats",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if dbPath == "" {
				dbPath = defaultDBPath("starterstory-pp-cli")
			}
			if missingIndexGuard(cmd, flags, dbPath) {
				return nil
			}

			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer db.Close()
			if err := db.EnsureStarterStoryIndex(ctx); err != nil {
				return err
			}

			bySection := map[string]int{}
			total := 0
			secRows, err := db.DB().QueryContext(ctx, `SELECT section, COUNT(*) FROM ss_index GROUP BY section`)
			if err != nil {
				return fmt.Errorf("querying section counts: %w", err)
			}
			for secRows.Next() {
				var section sql.NullString
				var count int
				if err := secRows.Scan(&section, &count); err != nil {
					_ = secRows.Close()
					return fmt.Errorf("scanning section count: %w", err)
				}
				bySection[section.String] = count
				total += count
			}
			if err := secRows.Err(); err != nil {
				_ = secRows.Close()
				return fmt.Errorf("iterating section counts: %w", err)
			}
			_ = secRows.Close()

			// Revenue distribution for stories, bucketed.
			buckets := map[string]int{
				"0":       0,
				"1-9999":  0,
				"10k-49k": 0,
				"50k-99k": 0,
				"100k+":   0,
			}
			revRows, err := db.DB().QueryContext(ctx, `SELECT revenue FROM ss_index WHERE section='stories'`)
			if err != nil {
				return fmt.Errorf("querying story revenue: %w", err)
			}
			for revRows.Next() {
				var revenue sql.NullInt64
				if err := revRows.Scan(&revenue); err != nil {
					_ = revRows.Close()
					return fmt.Errorf("scanning revenue: %w", err)
				}
				buckets[revenueBucket(revenue.Int64)]++
			}
			if err := revRows.Err(); err != nil {
				_ = revRows.Close()
				return fmt.Errorf("iterating revenue: %w", err)
			}
			_ = revRows.Close()

			if flags.asJSON || flags.agent {
				payload := map[string]any{
					"total":                 total,
					"by_section":            bySection,
					"story_revenue_buckets": buckets,
				}
				return printJSONFiltered(cmd.OutOrStdout(), payload, flags)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Total indexed: %d\n\n", total)
			sections := make([]string, 0, len(bySection))
			for s := range bySection {
				sections = append(sections, s)
			}
			sort.Strings(sections)
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "SECTION\tCOUNT")
			for _, s := range sections {
				fmt.Fprintf(tw, "%s\t%d\n", s, bySection[s])
			}
			if err := tw.Flush(); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "\nStory revenue distribution:")
			tw2 := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw2, "BUCKET\tCOUNT")
			for _, b := range []string{"0", "1-9999", "10k-49k", "50k-99k", "100k+"} {
				fmt.Fprintf(tw2, "%s\t%d\n", b, buckets[b])
			}
			return tw2.Flush()
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	return cmd
}

func revenueBucket(rev int64) string {
	switch {
	case rev <= 0:
		return "0"
	case rev < 10000:
		return "1-9999"
	case rev < 50000:
		return "10k-49k"
	case rev < 100000:
		return "50k-99k"
	default:
		return "100k+"
	}
}
