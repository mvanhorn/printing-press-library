// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/ai/cartesia/internal/store"
	"github.com/spf13/cobra"
)

type trendBucket struct {
	Bucket    string  `json:"bucket"`
	Mean      float64 `json:"mean"`
	N         int     `json:"n"`
	PassCount int     `json:"pass_count"`
}

// newMetricsTrendCmd registers `metrics trend <metric-id>` — bucketed
// aggregation of score and pass count over time.
func newMetricsTrendCmd(flags *rootFlags) *cobra.Command {
	var bucket string
	var agentID string
	var since string
	var dbPath string
	cmd := &cobra.Command{
		Use:         "trend [metric-id]",
		Short:       "Time-bucketed metric trends across the local store",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example:     `  cartesia-pp-cli metrics trend metric_xyz --bucket day --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			metricID := args[0]
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("cartesia-pp-cli")
			}
			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			var fmtExpr string
			switch bucket {
			case "hour":
				fmtExpr = "%Y-%m-%dT%H:00:00"
			case "day", "":
				fmtExpr = "%Y-%m-%dT00:00:00"
			default:
				return fmt.Errorf("--bucket must be 'day' or 'hour' (got %q)", bucket)
			}

			q := fmt.Sprintf(`SELECT strftime('%s', evaluated_at) AS bucket,
                                   AVG(score) AS mean,
                                   COUNT(*) AS n,
                                   COALESCE(SUM(passed), 0) AS pass_count
                            FROM metric_results
                            WHERE metric_id = ?`, fmtExpr)
			argv := []any{metricID}
			if since != "" {
				t, err := parseSince(since)
				if err != nil {
					return fmt.Errorf("--since: %w", err)
				}
				q += " AND evaluated_at >= ?"
				argv = append(argv, t.UTC().Format("2006-01-02T15:04:05Z"))
			}
			if agentID != "" {
				q += " AND agent_id = ?"
				argv = append(argv, agentID)
			}
			q += " GROUP BY bucket ORDER BY bucket"

			rows, err := db.DB().QueryContext(cmd.Context(), q, argv...)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()

			out := []trendBucket{}
			for rows.Next() {
				var b trendBucket
				var bucketStr sql.NullString
				var mean sql.NullFloat64
				if err := rows.Scan(&bucketStr, &mean, &b.N, &b.PassCount); err != nil {
					return fmt.Errorf("scan: %w", err)
				}
				if bucketStr.Valid {
					b.Bucket = bucketStr.String
				}
				if mean.Valid {
					b.Mean = mean.Float64
				}
				out = append(out, b)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("rows: %w", err)
			}
			return flags.printJSON(cmd, map[string]any{
				"metric_id": metricID,
				"buckets":   out,
			})
		},
	}

	cmd.Flags().StringVar(&bucket, "bucket", "day", "Time bucket size: day or hour")
	cmd.Flags().StringVar(&agentID, "agent", "", "Restrict to a specific agent")
	cmd.Flags().StringVar(&since, "since", "", "Restrict to evaluations newer than this window")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
