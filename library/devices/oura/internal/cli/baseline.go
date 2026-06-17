// Copyright 2026 slinsmaier and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"os"

	"github.com/mvanhorn/printing-press-library/library/devices/oura/internal/store"
	"github.com/spf13/cobra"
)

type baselineView struct {
	Metric        string  `json:"metric"`
	Date          string  `json:"date"`
	Value         float64 `json:"value,omitempty"`
	HasValue      bool    `json:"has_value"`
	WindowDays    int     `json:"window_days"`
	BaselineMean  float64 `json:"baseline_mean"`
	BaselineStdev float64 `json:"baseline_stdev"`
	SampleSize    int     `json:"sample_size"`
	ZScore        float64 `json:"z_score,omitempty"`
	Band          string  `json:"band,omitempty"`
	Note          string  `json:"note,omitempty"`
}

func newNovelBaselineCmd(flags *rootFlags) *cobra.Command {
	var flagMetric string
	var flagWindow int
	var flagDate string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "baseline",
		Short: "See today's metric against your personal rolling mean and standard deviation bands — not Oura's population norms",
		Long: `Compares a single day's metric value against your own trailing rolling
mean and standard deviation, not Oura's population-wide norms. Reports a
z-score and a band (normal / elevated / notable) so you can see how today
compares to your personal history.`,
		Example: `  oura-pp-cli baseline --metric readiness
  oura-pp-cli baseline --metric sleep --date 2026-06-10 --window 60 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		// pp:data-source local
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would compute personal baseline band for --metric against trailing window")
				return nil
			}
			if flagMetric == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--metric is required (supported: %v)", knownMetrics()))
			}
			spec, err := resolveMetric(flagMetric)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			date := flagDate
			if date == "" {
				date = today()
			}

			if dbPath == "" {
				dbPath = defaultDBPath("oura-pp-cli")
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprint(cmd.ErrOrStderr(), missingMirrorMessage(dbPath))
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "{}")
				}
				return nil
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			windowStart := addDays(date, -flagWindow)
			windowEnd := addDays(date, -1)
			series, err := metricSeries(db, spec, windowStart, windowEnd)
			if err != nil {
				return fmt.Errorf("querying baseline window: %w", err)
			}
			vals := make([]float64, 0, len(series))
			for _, v := range series {
				vals = append(vals, v)
			}
			mean, stdDev := meanStdDev(vals)

			view := baselineView{
				Metric:        flagMetric,
				Date:          date,
				WindowDays:    flagWindow,
				BaselineMean:  round2(mean),
				BaselineStdev: round2(stdDev),
				SampleSize:    len(vals),
			}

			todaySeries, err := metricSeries(db, spec, date, date)
			if err != nil {
				return fmt.Errorf("querying %s value: %w", date, err)
			}
			if v, ok := todaySeries[date]; ok {
				view.Value = v
				view.HasValue = true
				if stdDev > 0 {
					view.ZScore = round2((v - mean) / stdDev)
					view.Band = bandFor(view.ZScore)
				} else if len(vals) > 0 {
					view.Band = "insufficient-variance"
				}
			} else {
				view.Note = fmt.Sprintf("no %s value synced for %s", flagMetric, date)
			}
			if len(vals) < 7 {
				if view.Note != "" {
					view.Note += "; "
				}
				view.Note += fmt.Sprintf("only %d day(s) of baseline data — sync more history for a reliable baseline", len(vals))
			}

			if flags.asJSON || flags.agent {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "%s on %s: ", flagMetric, date)
			if view.HasValue {
				fmt.Fprintf(out, "%.1f (baseline %.1f ± %.1f over %d days, z=%.2f, %s)\n", view.Value, view.BaselineMean, view.BaselineStdev, view.SampleSize, view.ZScore, view.Band)
			} else {
				fmt.Fprintf(out, "no value (baseline %.1f ± %.1f over %d days)\n", view.BaselineMean, view.BaselineStdev, view.SampleSize)
			}
			if view.Note != "" {
				fmt.Fprintln(out, "note:", view.Note)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagMetric, "metric", "", "Metric to baseline: "+joinMetrics())
	cmd.Flags().IntVar(&flagWindow, "window", 30, "Trailing window size in days used to compute the personal baseline")
	cmd.Flags().StringVar(&flagDate, "date", "", "Date to evaluate against the baseline (YYYY-MM-DD, default today)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func bandFor(z float64) string {
	az := math.Abs(z)
	switch {
	case az < 1:
		return "normal"
	case az < 2:
		return "elevated"
	default:
		return "notable"
	}
}

func round2(f float64) float64 {
	return math.Round(f*100) / 100
}

func joinMetrics() string {
	b, _ := json.Marshal(knownMetrics())
	return string(b)
}
