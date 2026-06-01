package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"pinterest-pp-cli/internal/store"
)

type dayTiming struct {
	DayOfWeek   string  `json:"day_of_week"`
	PinCount    int     `json:"pins_created"`
	AvgSaves    float64 `json:"avg_saves"`
	TotalSaves  int     `json:"total_saves"`
}

func newNovelTimingCmd(flags *rootFlags) *cobra.Command {
	var weeks int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "timing",
		Short: "Surface which days of the week historically generate the most saves for your account.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  pinterest-pp-cli timing --weeks 8 --json
  pinterest-pp-cli timing --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would analyze pin save timing over the last %d weeks\n", weeks)
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
					CAST(strftime('%w', json_extract(data, '$.created_at')) AS INTEGER) as day_num,
					COUNT(*) as pin_count,
					COALESCE(AVG(CAST(json_extract(data, '$.save_count') AS REAL)), 0) as avg_saves,
					COALESCE(SUM(CAST(json_extract(data, '$.save_count') AS INTEGER)), 0) as total_saves
				FROM resources
				WHERE resource_type IN ('pins', 'boards_pins')
					AND json_extract(data, '$.created_at') >= ?
				GROUP BY day_num
				ORDER BY avg_saves DESC`, cutoff)
			if err != nil {
				return fmt.Errorf("querying timing data: %w", err)
			}
			defer rows.Close()

			dayNames := []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}
			var results []dayTiming
			for rows.Next() {
				var dayNum, pinCount, totalSaves int
				var avgSaves float64
				if err := rows.Scan(&dayNum, &pinCount, &avgSaves, &totalSaves); err != nil {
					continue
				}
				name := "Unknown"
				if dayNum >= 0 && dayNum < len(dayNames) {
					name = dayNames[dayNum]
				}
				results = append(results, dayTiming{
					DayOfWeek:  name,
					PinCount:   pinCount,
					AvgSaves:   avgSaves,
					TotalSaves: totalSaves,
				})
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
	cmd.Flags().IntVar(&weeks, "weeks", 8, "Number of weeks of history to analyze")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: auto)")
	return cmd
}
