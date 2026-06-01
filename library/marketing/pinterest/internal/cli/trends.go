package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"pinterest-pp-cli/internal/store"
)

type weeklyTrend struct {
	Week        string  `json:"week"`
	PinsCreated int     `json:"pins_created"`
	TotalSaves  int     `json:"total_saves"`
	TotalViews  int     `json:"total_views"`
	AvgSaves    float64 `json:"avg_saves_per_pin"`
}

func newNovelTrendsCmd(flags *rootFlags) *cobra.Command {
	var weeks int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "trends",
		Short: "Track how your pin impressions and saves change week-over-week.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  pinterest-pp-cli trends --weeks 4 --json
  pinterest-pp-cli trends --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would show pin performance trends over %d weeks\n", weeks)
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("pinterest-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'pinterest-pp-cli sync' first.", err)
			}
			defer db.Close()

			cutoff := time.Now().AddDate(0, 0, -weeks*7).Format(time.RFC3339)
			rows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT
					strftime('%Y-W%W', json_extract(data, '$.created_at')) as week,
					COUNT(*) as pins_created,
					COALESCE(SUM(CAST(json_extract(data, '$.save_count') AS INTEGER)), 0) as total_saves,
					COALESCE(SUM(CAST(json_extract(data, '$.view_count') AS INTEGER)), 0) as total_views
				FROM resources
				WHERE resource_type IN ('pins', 'boards_pins')
					AND json_extract(data, '$.created_at') >= ?
				GROUP BY week
				ORDER BY week ASC`, cutoff)
			if err != nil {
				return fmt.Errorf("querying trends: %w", err)
			}
			defer rows.Close()

			var results []weeklyTrend
			for rows.Next() {
				var r weeklyTrend
				if err := rows.Scan(&r.Week, &r.PinsCreated, &r.TotalSaves, &r.TotalViews); err != nil {
					continue
				}
				if r.PinsCreated > 0 {
					r.AvgSaves = float64(r.TotalSaves) / float64(r.PinsCreated)
				}
				results = append(results, r)
			}

			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No pin data found. Run 'pinterest-pp-cli sync' first.")
				return nil
			}

			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(results)
		},
	}
	cmd.Flags().IntVar(&weeks, "weeks", 8, "Number of weeks of history to show")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: auto)")
	return cmd
}
