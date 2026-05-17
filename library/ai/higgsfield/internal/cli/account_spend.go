// Copyright 2026 higgsfield-ai. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature for higgsfield-pp-cli (Phase 3 transcendence).

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/ai/higgsfield/internal/store"
)

type spendRow struct {
	Bucket string `json:"bucket"`
	Cost   int    `json:"cost"`
	Count  int    `json:"count"`
}

func newAccountSpendCmd(flags *rootFlags) *cobra.Command {
	var since string
	var groupBy string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "spend",
		Short: "Aggregate synced credit transactions by model, day, or Soul ID",
		Long: `Reads the local synced transactions table and aggregates credit spend by the
chosen bucket. Run 'higgsfield-pp-cli sync' first to populate the store.

Buckets:
  model   group by the model column on each transaction
  day     group by the date portion of created_at
  soul_id group by the soul_id (joins through generations on request_id)`,
		Example: strings.Trim(`
  higgsfield-pp-cli account spend --since 2026-04-01 --group-by model
  higgsfield-pp-cli account spend --since 2026-05-01 --group-by day --json
  higgsfield-pp-cli account spend --group-by soul_id --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("higgsfield-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			var bucketExpr string
			var fromClause string
			switch strings.ToLower(groupBy) {
			case "", "model":
				bucketExpr = "COALESCE(json_extract(t.data, '$.model'), '(unknown)')"
				fromClause = "FROM resources t WHERE t.resource_type IN ('transactions')"
			case "day":
				bucketExpr = "substr(COALESCE(json_extract(t.data, '$.created_at'), ''), 1, 10)"
				fromClause = "FROM resources t WHERE t.resource_type IN ('transactions')"
			case "soul_id", "soul-id", "soul":
				// Join transactions to generations via request_id, then group by
				// the soul_id on the generation.
				bucketExpr = "COALESCE(json_extract(g.data, '$.soul_id'), '(none)')"
				fromClause = `
					FROM resources t
					LEFT JOIN resources g
					  ON g.resource_type IN ('generations')
					 AND json_extract(g.data, '$.request_id') = json_extract(t.data, '$.request_id')
					WHERE t.resource_type IN ('transactions')`
			default:
				return fmt.Errorf("unknown --group-by %q (allowed: model, day, soul_id)", groupBy)
			}

			query := fmt.Sprintf(`
				SELECT %s AS bucket,
				       COALESCE(SUM(CAST(json_extract(t.data, '$.amount') AS INTEGER)), 0) AS cost,
				       COUNT(*) AS row_count
				%s
				  %s
				GROUP BY bucket
				ORDER BY cost DESC`, bucketExpr, fromClause, buildSinceFilter(since))

			params := []any{}
			if since != "" {
				params = append(params, since)
			}

			rows, err := db.DB().QueryContext(cmd.Context(), query, params...)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()

			var results []spendRow
			var total int
			for rows.Next() {
				var r spendRow
				if err := rows.Scan(&r.Bucket, &r.Cost, &r.Count); err != nil {
					return err
				}
				results = append(results, r)
				total += r.Cost
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !humanFriendly) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"group_by":      groupBy,
					"since":         since,
					"total_credits": total,
					"row_count":     len(results),
					"rows":          results,
				}, flags)
			}
			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No transactions in the local store. Run `higgsfield-pp-cli sync` first.")
				return nil
			}
			if groupBy == "" {
				groupBy = "model"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Spend by %s — %d total credits across %d rows\n\n", groupBy, total, len(results))
			fmt.Fprintf(cmd.OutOrStdout(), "  %-32s %-10s %s\n", strings.ToUpper(groupBy), "CREDITS", "COUNT")
			for _, r := range results {
				fmt.Fprintf(cmd.OutOrStdout(), "  %-32s %-10d %d\n", truncate(r.Bucket, 30), r.Cost, r.Count)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&since, "since", "", "Only include transactions on or after this ISO timestamp (e.g. 2026-04-01)")
	cmd.Flags().StringVar(&groupBy, "group-by", "model", "Group by: model, day, or soul_id")
	cmd.Flags().StringVar(&dbPath, "db", "", "Override path to the local SQLite database")
	return cmd
}

func buildSinceFilter(since string) string {
	if since == "" {
		return ""
	}
	return "AND COALESCE(json_extract(t.data, '$.created_at'), '') >= ?"
}
