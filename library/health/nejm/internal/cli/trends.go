// Copyright 2026 laci141 and contributors. Licensed under Apache-2.0. See LICENSE.
// Phase 3: specialty/type trends aggregated across the synced corpus.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelTrendsCmd(flags *rootFlags) *cobra.Command {
	var groupBy string
	var limit int

	cmd := &cobra.Command{
		Use:         "trends",
		Short:       "Show how NEJM output distributes by specialty, article type, and issue across the synced corpus.",
		Long:        "Aggregates the local NEJM corpus by specialty (default), article type, or issue/volume, showing counts. Useful for quantifying NEJM coverage patterns across repeated syncs.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example:     "  nejm-pp-cli trends\n  nejm-pp-cli trends --group type --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			db, err := nejmOpenStore(ctx)
			if err != nil {
				return err
			}
			defer db.Close()
			maybeEmitSyncHints(cmd, db, "article", flags.maxAge)

			groupField := "specialties"
			switch strings.ToLower(groupBy) {
			case "type":
				groupField = "article_type"
			case "issue", "volume":
				groupField = "substr(date, 1, 7)" // YYYY-MM
			}

			q := fmt.Sprintf(`SELECT %s as grp, COUNT(*) as count FROM article WHERE %s != '' GROUP BY %s ORDER BY count DESC`,
				groupField, groupField, groupField)
			if limit > 0 {
				q += fmt.Sprintf(" LIMIT %d", limit)
			}

			rows, err := db.DB().QueryContext(ctx, q)
			if err != nil {
				return fmt.Errorf("querying trends: %w", err)
			}
			defer rows.Close()

			type trendRow struct {
				Group string `json:"group"`
				Count int    `json:"count"`
			}
			var results []trendRow
			for rows.Next() {
				var grp string
				var count int
				if err := rows.Scan(&grp, &count); err != nil {
					continue
				}
				results = append(results, trendRow{Group: grp, Count: count})
			}
			if err := rows.Err(); err != nil {
				return err
			}

			// Sort by count desc
			sort.Slice(results, func(i, j int) bool {
				return results[i].Count > results[j].Count
			})

			if len(results) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "hint: no data. Run 'nejm-pp-cli sync' to fetch articles, then 'nejm-pp-cli article <doi> --enrich' to enrich individual articles with specialties and types.")
				if flags.asJSON {
					return printOutputWithFlags(cmd.OutOrStdout(), json.RawMessage("[]"), flags)
				}
				return nil
			}

			data, err := json.Marshal(results)
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), json.RawMessage(data), flags)
		},
	}
	cmd.Flags().StringVar(&groupBy, "group", "specialty", "Group by: specialty, type, or issue")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum number of groups (0 = all)")
	return cmd
}
