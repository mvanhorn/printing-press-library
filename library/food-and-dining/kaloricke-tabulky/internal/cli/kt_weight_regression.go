package cli

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// weight regression [--days N] [--target-kg K]
// OLS regression over /statistic/summary/<date>/get's monthWeight[].
// Walks back up to N days, samples one summary per week to cover the
// window without hammering the API. Reports slope (kg/week), R^2,
// projected days-to-target.
func newKTWeightRegressionCmd(flags *rootFlags) *cobra.Command {
	var days int
	var targetKg float64

	cmd := &cobra.Command{
		Use:   "regression",
		Short: "Linear regression on weight history with projection to target",
		Long: `Fits an ordinary least-squares regression to your weight history and
reports slope (kg/week), R^2, and — if --target-kg is set — projected
calendar days until you hit the target.

Data source: /statistic/summary/<date>/get returns monthWeight[] (your
weight entries for the trailing month at that anchor). This command
samples summaries weekly across --days, deduplicates by date, and fits
OLS over the resulting series.

Requires at least 3 weight entries within the window. Empty windows
return an honest error rather than a 0-slope ghost result.`,
		Example: `  kaloricke-tabulky-pp-cli weight regression
  kaloricke-tabulky-pp-cli weight regression --days 90 --target-kg 75 --json`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		// pp:client-call — samples /statistic/summary/<date>/get every 7 days
		// via ktFetchSummaryDay (which wraps client.GetWithHeadersNoCache).
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, _, err := ktNewAuthenticatedClient(flags)
			if err != nil {
				return err
			}
			if days < 7 {
				days = 7
			}
			// Sample summaries every 7 days, plus today, then dedupe.
			today := time.Now().Local()
			seen := map[string]float64{}
			for offset := 0; offset <= days; offset += 7 {
				d := today.AddDate(0, 0, -offset)
				summary, err := ktFetchSummaryDay(c, d.Format("02.01.2006"))
				if err != nil {
					// One missing day is fine; continue.
					continue
				}
				for _, w := range summary.MonthWeight {
					if w.Value <= 0 {
						continue
					}
					// description is DD.MM.YYYY
					if _, err := time.Parse("02.01.2006", w.Description); err == nil {
						seen[w.Description] = w.Value
					}
				}
			}
			if len(seen) < 3 {
				return fmt.Errorf("not enough weight entries within %d days (have %d, need at least 3)", days, len(seen))
			}

			// Build sorted (date, value) slice.
			type point struct {
				date  time.Time
				value float64
			}
			pts := make([]point, 0, len(seen))
			for dateStr, v := range seen {
				t, _ := time.Parse("02.01.2006", dateStr)
				pts = append(pts, point{date: t, value: v})
			}
			sort.Slice(pts, func(i, j int) bool { return pts[i].date.Before(pts[j].date) })

			// OLS: y = a + b*x, where x is days from first sample.
			t0 := pts[0].date
			n := float64(len(pts))
			var sumX, sumY, sumXY, sumXX, sumYY float64
			for _, p := range pts {
				x := p.date.Sub(t0).Hours() / 24
				y := p.value
				sumX += x
				sumY += y
				sumXY += x * y
				sumXX += x * x
				sumYY += y * y
			}
			meanX := sumX / n
			meanY := sumY / n
			denom := sumXX - n*meanX*meanX
			if math.Abs(denom) < 1e-9 {
				return fmt.Errorf("weight samples have no time spread (all on same day?)")
			}
			b := (sumXY - n*meanX*meanY) / denom
			a := meanY - b*meanX

			// R^2
			ssTot := sumYY - n*meanY*meanY
			var ssRes float64
			for _, p := range pts {
				x := p.date.Sub(t0).Hours() / 24
				pred := a + b*x
				ssRes += (p.value - pred) * (p.value - pred)
			}
			r2 := 0.0
			if ssTot > 0 {
				r2 = 1 - ssRes/ssTot
			}

			latestX := pts[len(pts)-1].date.Sub(t0).Hours() / 24
			currentWeight := a + b*latestX

			result := map[string]any{
				"start_date":         pts[0].date.Format("2006-01-02"),
				"end_date":           pts[len(pts)-1].date.Format("2006-01-02"),
				"sample_count":       len(pts),
				"slope_kg_per_day":   b,
				"slope_kg_per_week":  b * 7,
				"intercept_kg":       a,
				"r_squared":          r2,
				"latest_kg":          pts[len(pts)-1].value,
				"current_modeled_kg": currentWeight,
			}
			if targetKg > 0 {
				if math.Abs(b) < 1e-6 {
					result["target_eta_days"] = nil
					result["target_eta_note"] = "slope is effectively zero; no ETA"
				} else {
					daysToTarget := (targetKg - currentWeight) / b
					result["target_kg"] = targetKg
					result["target_eta_days"] = math.Round(daysToTarget)
					if daysToTarget >= 0 {
						eta := time.Now().AddDate(0, 0, int(math.Round(daysToTarget)))
						result["target_eta_date"] = eta.Format("2006-01-02")
					} else {
						result["target_eta_note"] = "would require reversing the current trend"
					}
				}
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return ktEmit(cmd.OutOrStdout(), flags, result)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Weight regression (%s to %s, %d samples)\n", result["start_date"], result["end_date"], len(pts))
			fmt.Fprintf(w, "  Slope:   %+.3f kg/week  (R² = %.3f)\n", b*7, r2)
			fmt.Fprintf(w, "  Latest:  %.2f kg\n", pts[len(pts)-1].value)
			fmt.Fprintf(w, "  Model:   %.2f kg today\n", currentWeight)
			if targetKg > 0 {
				if eta, ok := result["target_eta_date"].(string); ok {
					fmt.Fprintf(w, "  Target:  %.2f kg → ETA %s (%v days)\n", targetKg, eta, result["target_eta_days"])
				} else if note, ok := result["target_eta_note"].(string); ok {
					fmt.Fprintf(w, "  Target:  %.2f kg → %s\n", targetKg, note)
				}
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&days, "days", 30, "Trailing days to sample")
	cmd.Flags().Float64Var(&targetKg, "target-kg", 0, "Target weight for ETA projection")
	return cmd
}

var _ = strings.TrimSpace // keep import
