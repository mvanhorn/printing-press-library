package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/ai/zylos/internal/store"
)

func newStatsCmd(flags *rootFlags) *cobra.Command {
	var days int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Conversation analytics: message counts, direction breakdown, activity trends",
		Example: strings.Trim(`
  zylos-pp-cli stats
  zylos-pp-cli stats --days 30
  zylos-pp-cli stats --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			if dbPath == "" {
				dbPath = defaultDBPath("zylos-pp-cli")
			}
			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'zylos-pp-cli sync' first.", err)
			}
			defer db.Close()

			since := time.Now().AddDate(0, 0, -days).Format(time.RFC3339)

			type statsResult struct {
				Days            int              `json:"days"`
				TotalMessages   int              `json:"total_messages"`
				MessagesIn      int              `json:"messages_in"`
				MessagesOut     int              `json:"messages_out"`
				AvgPerDay       float64          `json:"avg_messages_per_day"`
				MostActiveDay   string           `json:"most_active_day"`
				MostActiveCount int              `json:"most_active_day_count"`
				MessagesPerDay  []map[string]any `json:"messages_per_day"`
			}

			// Total message count
			var total int
			db.DB().QueryRowContext(cmd.Context(),
				`SELECT COUNT(*) FROM resources
				 WHERE resource_type = 'conversations'
				 AND json_extract(data, '$.timestamp') >= ?`,
				since,
			).Scan(&total)

			// Messages by direction
			var inCount, outCount int
			db.DB().QueryRowContext(cmd.Context(),
				`SELECT
				   SUM(CASE WHEN json_extract(data, '$.direction') = 'in' THEN 1 ELSE 0 END),
				   SUM(CASE WHEN json_extract(data, '$.direction') = 'out' THEN 1 ELSE 0 END)
				 FROM resources
				 WHERE resource_type = 'conversations'
				 AND json_extract(data, '$.timestamp') >= ?`,
				since,
			).Scan(&inCount, &outCount)

			// Messages per day
			rows, err := db.DB().QueryContext(cmd.Context(),
				`SELECT DATE(json_extract(data, '$.timestamp')) as day, COUNT(*) as cnt
				 FROM resources
				 WHERE resource_type = 'conversations'
				 AND json_extract(data, '$.timestamp') >= ?
				 GROUP BY day
				 ORDER BY day`,
				since,
			)
			if err != nil {
				return fmt.Errorf("querying daily stats: %w", err)
			}
			defer rows.Close()

			var perDay []map[string]any
			mostActiveDay := ""
			mostActiveCount := 0
			for rows.Next() {
				var day string
				var cnt int
				if err := rows.Scan(&day, &cnt); err != nil {
					continue
				}
				perDay = append(perDay, map[string]any{
					"date":  day,
					"count": cnt,
				})
				if cnt > mostActiveCount {
					mostActiveCount = cnt
					mostActiveDay = day
				}
			}

			avgPerDay := 0.0
			if days > 0 && total > 0 {
				avgPerDay = float64(total) / float64(days)
			}

			result := statsResult{
				Days:            days,
				TotalMessages:   total,
				MessagesIn:      inCount,
				MessagesOut:     outCount,
				AvgPerDay:       avgPerDay,
				MostActiveDay:   mostActiveDay,
				MostActiveCount: mostActiveCount,
				MessagesPerDay:  perDay,
			}

			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}

	cmd.Flags().IntVar(&days, "days", 7, "Analyze last N days")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")

	return cmd
}
