// Copyright 2026 waveriderai and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: rank founder case studies by revenue parsed from the slug.

package cli

import (
	"database/sql"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/starterstory/internal/store"
	"github.com/spf13/cobra"
)

// revenueKMonthRE matches "<n>k-month" / "<n>k month" / "<n>k-mo" style
// suffixes in a slug (thousands/month). revenueMMonthRE is the millions
// variant. The two never both match a given digit run because the unit letter
// (k vs m) immediately follows the number.
var (
	revenueKMonthRE = regexp.MustCompile(`(\d+)\s*k[- ]?(month|mo)`)
	revenueMMonthRE = regexp.MustCompile(`(\d+)\s*m[- ]?(month|mo)`)
)

// parseRevenueFromSlug extracts an approximate monthly revenue (in dollars)
// from a StarterStory slug using the "<n>k-month" / "<n>m-month" heuristic.
// Returns 0 when no revenue token is present. The heuristic is deliberately
// simple: "6-5k-month" (6.5k) parses as 5000, which is acceptable for ranking.
func parseRevenueFromSlug(slug string) int64 {
	s := strings.ToLower(slug)
	if m := revenueKMonthRE.FindStringSubmatch(s); m != nil {
		if n, err := strconv.ParseInt(m[1], 10, 64); err == nil {
			return n * 1000
		}
	}
	if m := revenueMMonthRE.FindStringSubmatch(s); m != nil {
		if n, err := strconv.ParseInt(m[1], 10, 64); err == nil {
			return n * 1000000
		}
	}
	return 0
}

// topRevenueRow is one ranked case study.
type topRevenueRow struct {
	Slug    string `json:"slug"`
	Title   string `json:"title"`
	Revenue int64  `json:"revenue"`
	URL     string `json:"url"`
}

// pp:data-source local
func newNovelTopRevenueCmd(flags *rootFlags) *cobra.Command {
	var flagLimit int
	var dbPath string

	cmd := &cobra.Command{
		Use:         "top-revenue",
		Short:       "Rank founder case studies by monthly revenue parsed from their titles.",
		Example:     "  starterstory-pp-cli top-revenue --limit 20",
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

			rows, err := db.DB().QueryContext(ctx, `
				SELECT slug, title, revenue, url
				FROM ss_index
				WHERE section='stories' AND revenue > 0
				ORDER BY revenue DESC, slug
				LIMIT ?`, flagLimit)
			if err != nil {
				return fmt.Errorf("querying top revenue: %w", err)
			}
			out := make([]topRevenueRow, 0, flagLimit)
			for rows.Next() {
				var slug, title sql.NullString
				var revenue sql.NullInt64
				var url sql.NullString
				if err := rows.Scan(&slug, &title, &revenue, &url); err != nil {
					_ = rows.Close()
					return fmt.Errorf("scanning row: %w", err)
				}
				out = append(out, topRevenueRow{
					Slug:    slug.String,
					Title:   title.String,
					Revenue: revenue.Int64,
					URL:     url.String,
				})
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return fmt.Errorf("iterating rows: %w", err)
			}
			_ = rows.Close()

			if flags.asJSON || flags.agent {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no revenue-tagged stories in the local index")
				return nil
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "REVENUE/MO\tSLUG\tTITLE")
			for _, r := range out {
				fmt.Fprintf(tw, "$%d\t%s\t%s\n", r.Revenue, r.Slug, truncate(r.Title, 60))
			}
			return tw.Flush()
		},
	}
	cmd.Flags().IntVar(&flagLimit, "limit", 20, "Maximum number of stories to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	return cmd
}

// missingIndexGuard reports whether the local index is absent. When it is, it
// prints an actionable hint to stderr (and an empty JSON array to stdout for
// machine consumers) and returns true so the caller can return nil. Keeping
// this in one helper keeps the five novel read commands consistent.
func missingIndexGuard(cmd *cobra.Command, flags *rootFlags, dbPath string) bool {
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		fmt.Fprintf(cmd.ErrOrStderr(), "no local index at %s\nrun: starterstory-pp-cli index\n", dbPath)
		if flags.asJSON || flags.agent {
			fmt.Fprintln(cmd.OutOrStdout(), "[]")
		}
		return true
	}
	return false
}
