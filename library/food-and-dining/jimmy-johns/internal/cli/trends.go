// PATCH: hand-authored insight `trends` — order-history trend analysis from
// the local store. Aggregates orders by week and computes ordering cadence.
// Uses the SQLite store populated by `sync`. See .printing-press-patches.json
// patch id "insight-trends".

package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/jimmy-johns/internal/store"
)

type trendsResult struct {
	WindowDays      int              `json:"window_days"`
	OrdersFound     int              `json:"orders_found"`
	OrdersPerWeek   float64          `json:"avg_orders_per_week"`
	WeeklyBreakdown map[string]int   `json:"weekly_breakdown,omitempty"`
	Notes           []string         `json:"notes,omitempty"`
}

func newTrendsCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var windowDays int
	cmd := &cobra.Command{
		Use:   "trends",
		Short: "Show ordering cadence and weekly breakdown from local order history",
		Long: `Aggregate the local order history into a weekly breakdown plus an
overall average orders-per-week metric.

The 'orders' resource is populated by 'sync' when an authenticated session
is configured (see 'auth import-cookies'). With no orders synced, this
returns zeros and an explanatory note.`,
		Example: `  jimmy-johns-pp-cli trends --window-days 90 --json
  jimmy-johns-pp-cli trends --window-days 30`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("jimmy-johns-pp-cli")
			}
			db, err := store.OpenWithContext(context.Background(), dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer db.Close()

			result := trendsResult{WindowDays: windowDays, WeeklyBreakdown: map[string]int{}}

			// PATCH: apply --window-days as a SQL filter on COALESCE(json_extract(data,'$.placedAt'), updated_at).
			// Previously the flag was only echoed in result.WindowDays; the queries returned every synced order
			// regardless of window, so `--window-days 30` and `--window-days 3650` produced identical output.
			// The filter is parameterized as `now - <N> days`; clamp at 1 day to avoid an empty/negative window.
			if windowDays < 1 {
				windowDays = 1
				result.WindowDays = 1
				result.Notes = append(result.Notes, "window-days clamped to 1 (minimum)")
			}
			windowFilter := `(COALESCE(json_extract(data,'$.placedAt'), updated_at) >= datetime('now', ?))`
			windowArg := fmt.Sprintf("-%d days", windowDays)

			// We use the resources table since orders may not have a dedicated table.
			// resources table has resource_type column; orders rows would be tagged "orders".
			err = db.DB().QueryRowContext(cmd.Context(),
				`SELECT COUNT(*) FROM resources WHERE resource_type = 'orders' AND `+windowFilter, windowArg).Scan(&result.OrdersFound)
			if err != nil {
				result.Notes = append(result.Notes, fmt.Sprintf("orders count failed: %v", err))
			}

			rows, err := db.DB().QueryContext(cmd.Context(),
				`SELECT strftime('%Y-W%W', COALESCE(json_extract(data,'$.placedAt'), updated_at)) AS week,
				        COUNT(*)
				 FROM resources
				 WHERE resource_type = 'orders' AND `+windowFilter+`
				 GROUP BY week ORDER BY week`, windowArg)
			if err == nil {
				defer rows.Close()
				weekCount := 0
				for rows.Next() {
					var week string
					var n int
					if err := rows.Scan(&week, &n); err == nil {
						result.WeeklyBreakdown[week] = n
						weekCount++
					}
				}
				if weekCount > 0 && result.OrdersFound > 0 {
					result.OrdersPerWeek = float64(result.OrdersFound) / float64(weekCount)
				}
			}

			if result.OrdersFound == 0 {
				result.Notes = append(result.Notes,
					"no orders found in local store — run 'auth import-cookies' then 'sync' to populate order history")
			}

			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Window:           %d days\n", result.WindowDays)
			fmt.Fprintf(w, "Orders found:     %d\n", result.OrdersFound)
			fmt.Fprintf(w, "Avg orders/week:  %.2f\n", result.OrdersPerWeek)
			if len(result.WeeklyBreakdown) > 0 {
				fmt.Fprintln(w, "\nWeekly breakdown:")
				for wk, n := range result.WeeklyBreakdown {
					fmt.Fprintf(w, "  %-12s  %d\n", wk, n)
				}
			}
			for _, n := range result.Notes {
				fmt.Fprintf(w, "Note: %s\n", n)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to the local SQLite store (defaults to the user cache dir)")
	cmd.Flags().IntVar(&windowDays, "window-days", 90, "Time window for the trend aggregation, in days (default 90 covers last quarter)")
	return cmd
}
