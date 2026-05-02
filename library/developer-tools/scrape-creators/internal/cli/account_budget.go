package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// newAccountBudgetCmd reports current credits, recent burn rate, and a
// projected runway. Combines /v1/account/credit-balance with the API's
// daily usage history (no local store dependency for the projection).
func newAccountBudgetCmd(flags *rootFlags) *cobra.Command {
	var days int
	cmd := &cobra.Command{
		Use:   "budget",
		Short: "Report current credits, daily burn rate, and projected days remaining",
		Long: `Calls /v1/account/credit-balance and /v1/account/get-daily-usage-count and
projects how many days of credits remain at the average burn rate over
the last N days.

Use --days to widen or narrow the projection window. Returns JSON with
the same fields a Grafana panel would graph.`,
		Example:     "  scrape-creators-pp-cli account budget --json --days 7",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			balRaw, err := c.Get("/v1/account/credit-balance", nil)
			if err != nil {
				return classifyAPIError(err)
			}
			usageRaw, err := c.Get("/v1/account/get-daily-usage-count", nil)
			if err != nil {
				return classifyAPIError(err)
			}

			var bal map[string]any
			_ = json.Unmarshal(balRaw, &bal)
			credits := lookupInt(bal, []string{"creditCount", "credits"})

			// Daily usage is typically a list of {date, count}.
			var dailyArr []any
			_ = json.Unmarshal(usageRaw, &dailyArr)
			if len(dailyArr) == 0 {
				// Some payloads wrap in {data: [...]}
				var wrapped struct {
					Data []any `json:"data"`
				}
				if json.Unmarshal(usageRaw, &wrapped) == nil {
					dailyArr = wrapped.Data
				}
			}

			type dayPoint struct {
				Date  string `json:"date"`
				Count int64  `json:"count"`
			}
			// Helper: read count, parsing both numeric and string-encoded forms.
			parseAnyInt := func(v any) int64 {
				switch x := v.(type) {
				case float64:
					return int64(x)
				case int64:
					return x
				case int:
					return int64(x)
				case string:
					n, _ := strconv.ParseInt(x, 10, 64)
					return n
				}
				return 0
			}
			var points []dayPoint
			for _, item := range dailyArr {
				m, ok := item.(map[string]any)
				if !ok {
					continue
				}
				point := dayPoint{
					Date: lookupString(m, []string{"date", "day", "usage_date"}),
				}
				for _, k := range []string{"count", "callCount", "request_count", "total_credits"} {
					if v := m[k]; v != nil {
						if n := parseAnyInt(v); n > 0 {
							point.Count = n
							break
						}
					}
				}
				if point.Date != "" {
					points = append(points, point)
				}
			}

			// Average over the last N days (most recent first or last? Use last `days` entries).
			window := days
			if window <= 0 {
				window = 7
			}
			if window > len(points) {
				window = len(points)
			}
			var totalRecent int64
			for i := len(points) - window; i < len(points) && i >= 0; i++ {
				totalRecent += points[i].Count
			}
			var avgPerDay float64
			if window > 0 {
				avgPerDay = float64(totalRecent) / float64(window)
			}

			projectedDays := -1.0 // unknown
			if avgPerDay > 0 {
				projectedDays = float64(credits) / avgPerDay
			}

			result := map[string]any{
				"credits_remaining": credits,
				"window_days":       window,
				"calls_in_window":   totalRecent,
				"avg_calls_per_day": avgPerDay,
				"projected_days":    projectedDays,
				"projected_until":   "",
				"daily_history":     points,
				"computed_at":       time.Now().UTC().Format(time.RFC3339),
			}
			if projectedDays > 0 {
				result["projected_until"] = time.Now().UTC().Add(time.Duration(projectedDays*24) * time.Hour).Format(time.RFC3339)
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Credits remaining: %d\n", credits)
			fmt.Fprintf(cmd.OutOrStdout(), "Last %d days: %d calls (avg %.1f/day)\n", window, totalRecent, avgPerDay)
			if projectedDays > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "Projected runway: %.1f days (until %s)\n", projectedDays, result["projected_until"])
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "Projected runway: unable to compute (no recent usage)")
			}
			if len(points) > 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "")
				tw := newTabWriter(cmd.OutOrStdout())
				fmt.Fprintln(tw, "DATE\tCALLS")
				for _, p := range points {
					fmt.Fprintf(tw, "%s\t%d\n", p.Date, p.Count)
				}
				return tw.Flush()
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&days, "days", 7, "Number of recent days to average over")
	_ = strings.Title // keep imports simple
	return cmd
}
