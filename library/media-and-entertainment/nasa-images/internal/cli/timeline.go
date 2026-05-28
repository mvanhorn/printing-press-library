// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written by the Printing Press operator on top of generated scaffolding.

package cli

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newTimelineCmd(flags *rootFlags) *cobra.Command {
	var (
		q, center, bucket, dbPath string
		limit                     int
	)
	cmd := &cobra.Command{
		Use:   "timeline",
		Short: "Month/year-bucket histogram of synced assets matching a query",
		Long: `Group locally-mirrored assets by date_created bucket (year or month) and
print the count per bucket. Useful for spotting publishing patterns —
"when did Perseverance go quiet, then publish again?" — that the upstream
keyword-only API can't surface.

Requires a populated local store; run 'mirror search --q <topic>' first.`,
		Example:     "  nasa-images-pp-cli timeline --q \"perseverance\" --bucket month",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would compute timeline histogram")
				return nil
			}
			ctx := cmd.Context()
			s, err := openNasaStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer s.Close()

			var bucketExpr string
			switch strings.ToLower(bucket) {
			case "year":
				bucketExpr = "substr(COALESCE(json_extract(r.data, '$.date_created'), ''), 1, 4)"
			case "month", "":
				bucketExpr = "substr(COALESCE(json_extract(r.data, '$.date_created'), ''), 1, 7)"
				bucket = "month"
			default:
				return fmt.Errorf("invalid --bucket %q: must be month or year", bucket)
			}

			var conds []string
			var argv []any
			conds = append(conds, "r.resource_type = 'asset'")
			if strings.TrimSpace(q) != "" {
				conds = append(conds, "r.id IN (SELECT id FROM resources_fts WHERE resource_type = 'asset' AND resources_fts MATCH ?)")
				argv = append(argv, quoteFTS(q))
			}
			if center != "" {
				conds = append(conds, "json_extract(r.data, '$.center') = ?")
				argv = append(argv, center)
			}
			where := strings.Join(conds, " AND ")
			if limit <= 0 {
				limit = 120
			}
			query := fmt.Sprintf(`
				SELECT %s AS b, COUNT(*) AS c
				FROM resources r
				WHERE %s
				GROUP BY b
				HAVING b <> ''
				ORDER BY b
				LIMIT ?
			`, bucketExpr, where)
			argv = append(argv, limit)

			rows, err := s.DB().QueryContext(ctx, query, argv...)
			if err != nil {
				return fmt.Errorf("querying timeline: %w", err)
			}
			defer rows.Close()

			type bucketRow struct {
				Bucket string `json:"bucket"`
				Count  int    `json:"count"`
			}
			var buckets []bucketRow
			total := 0
			for rows.Next() {
				var b sql.NullString
				var c int
				if err := rows.Scan(&b, &c); err == nil && b.Valid && b.String != "" {
					buckets = append(buckets, bucketRow{Bucket: b.String, Count: c})
					total += c
				}
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating timeline: %w", err)
			}

			result := map[string]any{
				"query":   q,
				"center":  center,
				"bucket":  bucket,
				"total":   total,
				"buckets": buckets,
			}
			if total == 0 {
				result["note"] = "no matching assets in the local mirror; run 'mirror search --q <topic>' first"
			}
			return flags.printJSON(cmd, result)
		},
	}
	cmd.Flags().StringVar(&q, "q", "", "FTS5 query terms (matched against title, description, keywords)")
	cmd.Flags().StringVar(&center, "center", "", "Limit to one NASA center code (e.g. JPL)")
	cmd.Flags().StringVar(&bucket, "bucket", "month", "Histogram bucket size: month (YYYY-MM, default) or year (YYYY)")
	cmd.Flags().IntVar(&limit, "limit", 120, "Maximum buckets to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/nasa-images-pp-cli/data.db)")
	return cmd
}
