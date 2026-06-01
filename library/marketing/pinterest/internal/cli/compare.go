package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"pinterest-pp-cli/internal/store"
)

type compareRow struct {
	Metric  string  `json:"metric"`
	Paid    float64 `json:"paid"`
	Organic float64 `json:"organic"`
	Delta   float64 `json:"delta_pct,omitempty"`
}

type compareResult struct {
	Period  string       `json:"period_days"`
	Rows    []compareRow `json:"metrics"`
	Note    string       `json:"note,omitempty"`
}

func newNovelCompareCmd(flags *rootFlags) *cobra.Command {
	var days int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "compare",
		Short: "Compare paid campaign performance against organic pin performance side-by-side.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  pinterest-pp-cli compare --days 30 --json
  pinterest-pp-cli compare --agent --select metric,paid,organic`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would compare paid vs organic performance over %d days\n", days)
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

			cutoff := time.Now().AddDate(0, 0, -days).Format(time.RFC3339)

			// Organic pins
			organicRow := db.DB().QueryRowContext(cmd.Context(), `
				SELECT
					COALESCE(AVG(CAST(json_extract(data, '$.save_count') AS REAL)), 0),
					COALESCE(AVG(CAST(json_extract(data, '$.view_count') AS REAL)), 0),
					COUNT(*)
				FROM resources
				WHERE resource_type IN ('pins', 'boards_pins')
					AND json_extract(data, '$.created_at') >= ?`, cutoff)

			var orgAvgSaves, orgAvgViews float64
			var orgCount int
			organicRow.Scan(&orgAvgSaves, &orgAvgViews, &orgCount)

			// Paid ads
			adsRow := db.DB().QueryRowContext(cmd.Context(), `
				SELECT
					COALESCE(AVG(CAST(json_extract(data, '$.outbound_clicks') AS REAL)), 0),
					COALESCE(AVG(CAST(json_extract(data, '$.impressions') AS REAL)), 0),
					COUNT(*)
				FROM resources
				WHERE resource_type IN ('ads', 'ad_accounts_ads')
					AND json_extract(data, '$.created_time') >= ?`, cutoff)

			var paidAvgClicks, paidAvgImpressions float64
			var adsCount int
			adsRow.Scan(&paidAvgClicks, &paidAvgImpressions, &adsCount)

			result := compareResult{
				Period: fmt.Sprintf("%d", days),
				Rows: []compareRow{
					{Metric: "avg_saves_per_pin", Organic: orgAvgSaves, Paid: 0},
					{Metric: "avg_views_per_pin", Organic: orgAvgViews, Paid: paidAvgImpressions},
					{Metric: "avg_clicks_per_unit", Organic: 0, Paid: paidAvgClicks},
					{Metric: "unit_count", Organic: float64(orgCount), Paid: float64(adsCount)},
				},
			}

			if orgCount == 0 && adsCount == 0 {
				result.Note = "No data found for this period. Run 'pinterest-pp-cli sync' to populate the local store."
			}

			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(result)
		},
	}
	cmd.Flags().IntVar(&days, "days", 30, "Number of days to compare")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: auto)")
	return cmd
}
