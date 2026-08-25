// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel command: aggregate the synced store by a facet.
// Preserved across `generate --force`.
// pp:data-source local

package cli

import (
	"database/sql"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/amazon-jobs/internal/store"
)

// statFacets maps the --by value to a NULL-safe grouping expression over the
// typed postings table (team lives in the JSON blob).
var statFacets = map[string]string{
	"city":     "COALESCE(NULLIF(city, ''), '(unknown)')",
	"state":    "COALESCE(NULLIF(state, ''), '(unknown)')",
	"country":  "COALESCE(NULLIF(country_code, ''), '(unknown)')",
	"category": "COALESCE(NULLIF(job_category, ''), '(unknown)')",
	"team":     "COALESCE(NULLIF(json_extract(data, '$.team.label'), ''), '(unknown)')",
	"schedule": "COALESCE(NULLIF(job_schedule_type, ''), '(unknown)')",
}

type statGroup struct {
	Key   string `json:"key"`
	Count int    `json:"count"`
}

type statsView struct {
	By     string      `json:"by"`
	Total  int         `json:"total"`
	Groups []statGroup `json:"groups"`
}

func newNovelStatsCmd(flags *rootFlags) *cobra.Command {
	var by, dbFlag string
	var limit int

	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Aggregate synced Amazon reqs by city, state, country, team, category, or schedule",
		Long: strings.Trim(`
Group the locally-synced jobs by a facet and count them — the aggregation the
server's empty facets[] never returns, and unbounded by the 10000 live-hit cap.

Run 'sync' first to populate the store. Use 'stats' for counts across a
structured facet; use 'skills' to rank reqs by demand for a qualification keyword.`, "\n"),
		Example: strings.Trim(`
  amazon-jobs-pp-cli stats --by city
  amazon-jobs-pp-cli stats --by team --limit 15 --agent`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would aggregate the local store")
				return nil
			}
			if err := guardDataSource(flags, "local"); err != nil {
				return err
			}
			if by == "" {
				by = "city"
			}
			by = strings.ToLower(strings.TrimSpace(by))
			expr, ok := statFacets[by]
			if !ok {
				return usageErr(fmt.Errorf("invalid --by %q: choose one of city, state, country, category, team, schedule", by))
			}
			if limit < 1 {
				limit = 20
			}
			dbPath := resolveDBPath(dbFlag)
			if storeMissing(cmd, flags, dbPath) {
				return nil
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer db.Close()

			// True store total (denominator), independent of the top-N groups.
			var total int
			if err := db.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM postings`).Scan(&total); err != nil {
				return fmt.Errorf("counting store: %w", err)
			}

			query := fmt.Sprintf(
				`SELECT %s AS k, COUNT(*) AS n FROM postings GROUP BY k ORDER BY n DESC, k ASC LIMIT ?`, expr)
			rows, err := db.DB().QueryContext(ctx, query, limit)
			if err != nil {
				return fmt.Errorf("aggregating: %w", err)
			}
			groups := make([]statGroup, 0, limit)
			for rows.Next() {
				var k sql.NullString
				var n int
				if err := rows.Scan(&k, &n); err != nil {
					_ = rows.Close()
					return fmt.Errorf("scanning group: %w", err)
				}
				key := k.String
				if !k.Valid || key == "" {
					key = "(unknown)"
				}
				groups = append(groups, statGroup{Key: key, Count: n})
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return fmt.Errorf("iterating groups: %w", err)
			}
			if err := rows.Close(); err != nil {
				return fmt.Errorf("closing rows: %w", err)
			}

			view := statsView{By: by, Total: total, Groups: groups}
			return emitResult(cmd, flags, view, func(w io.Writer) {
				if len(groups) == 0 {
					fmt.Fprintf(w, "no synced jobs yet; run: amazon-jobs-pp-cli sync --max-pages 5\n")
					return
				}
				fmt.Fprintf(w, "jobs by %s (top %d of %d synced):\n\n", by, len(groups), total)
				for _, g := range groups {
					fmt.Fprintf(w, "%6d  %s\n", g.Count, g.Key)
				}
			})
		},
	}

	cmd.Flags().StringVar(&by, "by", "city", "Facet to group by: city, state, country, category, team, schedule")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum groups to return")
	cmd.Flags().StringVar(&dbFlag, "db", "", "Local SQLite store path (default: platform data dir)")

	return cmd
}
