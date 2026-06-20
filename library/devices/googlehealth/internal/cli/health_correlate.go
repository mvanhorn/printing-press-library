// Copyright 2026 ryanc00per and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"errors"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/devices/googlehealth/internal/health"
	"github.com/mvanhorn/printing-press-library/library/devices/googlehealth/internal/store"
	"github.com/spf13/cobra"
)

// pp:data-source local
// correlate reads only the local SQLite store; it computes over already-synced
// data points and never calls the API.
func newCorrelateCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var metricA string
	var metricB string
	var maxLag int

	cmd := &cobra.Command{
		Use:   "correlate",
		Short: "Pearson + best-lag correlation between two health metrics",
		Long: `Measure how two metrics move together across days — the Pearson
correlation at zero lag, plus the day-lag that maximizes the correlation.
A strong correlation at a non-zero lag is the interesting signal: e.g.
today's step count predicting tomorrow's resting heart rate.

Data must be synced first with the sync command.`,
		Example: `  # Steps vs resting heart rate, scanning ±3 day lags
  googlehealth-pp-cli correlate --a steps --b daily-resting-heart-rate --max-lag 3

  # Sleep vs HRV, as JSON
  googlehealth-pp-cli correlate --a sleep --b heart-rate-variability --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			if dbPath == "" {
				dbPath = defaultDBPath("googlehealth-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'googlehealth-pp-cli sync' first.", err)
			}
			defer db.Close()
			maybeEmitSyncHints(cmd, db, "", flags.maxAge)

			points, err := loadHealthPoints(db)
			if err != nil {
				return err
			}
			// When the user doesn't name both metrics, auto-select the most
			// data-rich ones so a bare `correlate` still surfaces a real signal.
			if metricA == "" || metricB == "" {
				top := health.TopMetrics(points, 10)
				if len(top) < 2 {
					if flags.asJSON {
						return printJSONFiltered(out, map[string]any{"status": "insufficient_metrics", "metrics_available": top}, flags)
					}
					fmt.Fprintln(out, "Need at least two metrics with synced data to correlate. Run 'googlehealth-pp-cli sync' first.")
					return nil
				}
				if metricA == "" {
					metricA = top[0]
				}
				if metricB == "" {
					for _, m := range top {
						if m != metricA {
							metricB = m
							break
						}
					}
				}
			}
			res, err := health.Correlate(points, metricA, metricB, maxLag)
			if err != nil {
				// Not enough overlapping data yet is a sync hint, not a failure.
				if errors.Is(err, health.ErrInsufficientData) {
					if flags.asJSON {
						return printJSONFiltered(out, map[string]any{"metric_a": metricA, "metric_b": metricB, "status": "insufficient_data"}, flags)
					}
					fmt.Fprintf(out, "%s vs %s: not enough overlapping days yet. Run 'googlehealth-pp-cli sync' first.\n", metricA, metricB)
					return nil
				}
				return err
			}

			if flags.asJSON {
				return printJSONFiltered(out, res, flags)
			}
			fmt.Fprintf(out, "%s vs %s\n", res.MetricA, res.MetricB)
			fmt.Fprintf(out, "Overlapping days:  %d\n", res.OverlapDays)
			fmt.Fprintf(out, "Pearson r (lag 0): %+.3f  (%s)\n", res.R, correlationStrength(res.R))
			fmt.Fprintf(out, "Best lag:          %+d day(s)  r=%+.3f over %d day(s)\n",
				res.BestLagDays, res.BestLagR, res.BestLagN)
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&metricA, "a", "", "First metric")
	cmd.Flags().StringVar(&metricB, "b", "", "Second metric")
	cmd.Flags().IntVar(&maxLag, "max-lag", 3, "Maximum lag in days to scan")
	return cmd
}

// correlationStrength labels a Pearson r with a plain-language band so
// non-statisticians can read the result at a glance.
func correlationStrength(r float64) string {
	abs := r
	if abs < 0 {
		abs = -abs
	}
	switch {
	case abs >= 0.7:
		return "strong"
	case abs >= 0.4:
		return "moderate"
	case abs >= 0.2:
		return "weak"
	default:
		return "negligible"
	}
}
