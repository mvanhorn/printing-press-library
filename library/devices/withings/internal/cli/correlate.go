// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.
//
// `correlate` — Pearson + best-lag correlation between any two daily metrics.
// pp:data-source local
//
// Hand-authored implementation (RunE body replaced from the generated stub).

package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/withings/internal/store"
	"github.com/spf13/cobra"
)

// correlateResult is the JSON shape emitted by `correlate`.
type correlateResult struct {
	MetricA     string   `json:"metric_a"`
	MetricB     string   `json:"metric_b"`
	MatchedDays int      `json:"matched_days"`
	PearsonR    *float64 `json:"pearson_r"`
	BestLagDays int      `json:"best_lag_days"`
	BestLagR    *float64 `json:"best_lag_r"`
}

func newNovelCorrelateCmd(flags *rootFlags) *cobra.Command {
	var flagSince string
	var dbPath string

	cmd := &cobra.Command{
		Use:         "correlate <metric-a> <metric-b>",
		Short:       "Pearson + best-lag correlation between any two daily metrics (weight vs sleep score, steps vs resting HR).",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		Long: `Builds two daily series from the local mirror and reports their Pearson
correlation over matched days, plus the best correlation across day lags of
-3..+3 (does metric B lead or lag metric A?).

Valid metrics: ` + strings.Join(correlateMetrics, ", ") + `

Reads local data only. Sync first with:
  withings-pp-cli sync --resources measure,activity,sleep`,
		Example: `  withings-pp-cli correlate weight sleep_score
  withings-pp-cli correlate steps resting_hr --since 60d`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) != 2 {
				return usageErr(fmt.Errorf("correlate requires exactly two metrics: METRIC_A METRIC_B (valid: %s)", strings.Join(correlateMetrics, ", ")))
			}
			metricA := strings.ToLower(strings.TrimSpace(args[0]))
			metricB := strings.ToLower(strings.TrimSpace(args[1]))
			if !isKnownMetric(metricA) {
				return usageErr(fmt.Errorf("unknown metric %q (valid: %s)", args[0], strings.Join(correlateMetrics, ", ")))
			}
			if !isKnownMetric(metricB) {
				return usageErr(fmt.Errorf("unknown metric %q (valid: %s)", args[1], strings.Join(correlateMetrics, ", ")))
			}

			since, err := parseSinceFlag(flagSince, 90*24*time.Hour)
			if err != nil {
				return usageErr(err)
			}
			cutoff := time.Now().Add(-since)

			db, handled, err := openLocalForAnalytics(cmd, flags, dbPath, "measure,activity,sleep", false)
			if err != nil {
				return err
			}
			if handled {
				return nil
			}
			defer db.Close()

			res, err := computeCorrelate(db, metricA, metricB, cutoff)
			if err != nil {
				return err
			}
			return flags.printJSON(cmd, res)
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "90d", "Lookback window (e.g. 90d, 12w)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local mirror path (default: standard data dir)")
	return cmd
}

// computeCorrelate builds both daily series and computes the zero-lag Pearson r
// plus the best-lag correlation over lags -3..3.
func computeCorrelate(db *store.Store, metricA, metricB string, cutoff time.Time) (correlateResult, error) {
	res := correlateResult{MetricA: metricA, MetricB: metricB}

	seriesA, err := buildDailySeries(db, metricA, cutoff)
	if err != nil {
		return res, err
	}
	seriesB, err := buildDailySeries(db, metricB, cutoff)
	if err != nil {
		return res, err
	}

	xs, ys := matchedPairs(seriesA, seriesB, 0)
	res.MatchedDays = len(xs)
	if r, ok := pearson(xs, ys); ok {
		rr := roundN(r, 4)
		res.PearsonR = &rr
	}

	if lag, r, ok := bestLagCorrelation(seriesA, seriesB, -3, 3); ok {
		res.BestLagDays = lag
		rr := roundN(r, 4)
		res.BestLagR = &rr
	}

	return res, nil
}
