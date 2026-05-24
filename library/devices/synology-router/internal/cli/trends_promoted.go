// Copyright 2026 eric-jung. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"synology-router-pp-cli/internal/store"
)

func newTrendsPromotedCmd(flags *rootFlags) *cobra.Command {
	var flagPeriod string
	cmd := &cobra.Command{
		Use:         "trends",
		Short:       "Network traffic trends comparing time periods",
		Long:        "Queries the local store for traffic data grouped by day or week, computing rate-of-change between consecutive periods.",
		Example:     "  synology-router-pp-cli trends --period day",
		Annotations: map[string]string{"pp:endpoint": "trends", "pp:method": "GET", "pp:path": "/trends", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			dbPath := defaultDBPath("synology-router-pp-cli")
			s, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer s.Close()

			groupExpr := "DATE(synced_at)"
			if flagPeriod == "week" {
				groupExpr = "strftime('%Y-W%W', synced_at)"
			}

			rows, err := s.DB().Query(fmt.Sprintf(
				`SELECT %s AS period, COUNT(*) AS record_count, SUM(CAST(json_extract(data, '$.download') AS INTEGER)) AS total_download, SUM(CAST(json_extract(data, '$.upload') AS INTEGER)) AS total_upload FROM resources WHERE resource_type = 'traffic' GROUP BY period ORDER BY period ASC LIMIT 14`, groupExpr))
			if err != nil {
				return fmt.Errorf("querying traffic trends: %w", err)
			}
			defer rows.Close()

			var trends []map[string]any
			for rows.Next() {
				var period string
				var count int
				var dl, ul int64
				if rows.Scan(&period, &count, &dl, &ul) != nil {
					continue
				}
				entry := map[string]any{
					"period":        period,
					"record_count":  count,
					"download":      dl,
					"upload":        ul,
					"download_hr":   formatBytes(dl),
					"upload_hr":     formatBytes(ul),
				}
				if len(trends) > 0 {
					prev := trends[len(trends)-1]
					if prevDl, ok := prev["download"].(int64); ok && prevDl > 0 {
						change := float64(dl-prevDl) * 100 / float64(prevDl)
						entry["download_change_pct"] = fmt.Sprintf("%.1f%%", change)
					}
					if prevUl, ok := prev["upload"].(int64); ok && prevUl > 0 {
						change := float64(ul-prevUl) * 100 / float64(prevUl)
						entry["upload_change_pct"] = fmt.Sprintf("%.1f%%", change)
					}
				}
				trends = append(trends, entry)
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), trends, flags)
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) && len(trends) > 0 {
				return printAutoTable(cmd.OutOrStdout(), trends)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), mustMarshal(trends), flags)
		},
	}
	cmd.Flags().StringVar(&flagPeriod, "period", "day", "Grouping period for trend calculation: day or week")

	return cmd
}
