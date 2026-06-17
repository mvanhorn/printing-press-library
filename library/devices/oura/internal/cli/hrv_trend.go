// Copyright 2026 slinsmaier and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/devices/oura/internal/store"
	"github.com/spf13/cobra"
)

type hrvTrendView struct {
	WindowDays      int     `json:"window_days"`
	SampleSize      int     `json:"sample_size"`
	Mean7Day        float64 `json:"mean_7day,omitempty"`
	Mean30Day       float64 `json:"mean_30day,omitempty"`
	CoeffOfVariance float64 `json:"coefficient_of_variation,omitempty"`
	Verdict         string  `json:"verdict"`
	Note            string  `json:"note,omitempty"`
}

func newNovelHrvTrendCmd(flags *rootFlags) *cobra.Command {
	var flagDays int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "hrv-trend",
		Short: "Track whether overnight HRV is improving or declining: 7-day and 30-day rolling means, coefficient of variation",
		Long: `Computes 7-day and 30-day trailing rolling means of overnight HRV, the
coefficient of variation across the window, and a plain-English verdict
comparing the most recent 7 days against the 7 days before that.`,
		Example:     `  oura-pp-cli hrv-trend --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		// pp:data-source local
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would compute 7/30-day HRV rolling means and trend verdict")
				return nil
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

			spec, _ := resolveMetric("hrv")
			end := today()
			start := addDays(end, -flagDays)
			series, err := metricSeries(db, spec, start, end)
			if err != nil {
				return fmt.Errorf("querying HRV series: %w", err)
			}

			view := hrvTrendView{WindowDays: flagDays, SampleSize: len(series)}
			if len(series) == 0 {
				view.Verdict = "no-data"
				view.Note = "no synced HRV data in this window — run 'oura-pp-cli sync' first"
				return emitHrvTrend(cmd, flags, view)
			}

			allVals := make([]float64, 0, len(series))
			for _, v := range series {
				allVals = append(allVals, v)
			}
			mean, stdDev := meanStdDev(allVals)
			view.Mean30Day = round2(mean)
			if mean != 0 {
				view.CoeffOfVariance = round2(stdDev / mean)
			}

			last7Start := addDays(end, -6)
			prior7Start := addDays(end, -13)
			prior7End := addDays(end, -7)
			last7 := valuesInRange(series, last7Start, end)
			prior7 := valuesInRange(series, prior7Start, prior7End)

			if len(last7) > 0 {
				m, _ := meanStdDev(last7)
				view.Mean7Day = round2(m)
			}

			switch {
			case len(last7) == 0:
				view.Verdict = "no-recent-data"
			case len(prior7) == 0:
				view.Verdict = "insufficient-history"
				view.Note = "not enough prior-week data to compare trend direction yet"
			default:
				recentMean, _ := meanStdDev(last7)
				priorMean, _ := meanStdDev(prior7)
				delta := recentMean - priorMean
				pct := 0.0
				if priorMean != 0 {
					pct = delta / priorMean * 100
				}
				switch {
				case pct > 3:
					view.Verdict = fmt.Sprintf("improving (+%.1f%% vs prior week)", pct)
				case pct < -3:
					view.Verdict = fmt.Sprintf("declining (%.1f%% vs prior week)", pct)
				default:
					view.Verdict = "stable"
				}
			}

			return emitHrvTrend(cmd, flags, view)
		},
	}
	cmd.Flags().IntVar(&flagDays, "days", 30, "Lookback window in days for the rolling HRV trend")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func valuesInRange(series map[string]float64, start, end string) []float64 {
	var vals []float64
	for day, v := range series {
		if day >= start && day <= end {
			vals = append(vals, v)
		}
	}
	return vals
}

func emitHrvTrend(cmd *cobra.Command, flags *rootFlags, view hrvTrendView) error {
	if flags.asJSON || flags.agent {
		return printJSONFiltered(cmd.OutOrStdout(), view, flags)
	}
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "HRV trend (%d-day window, %d samples): %s\n", view.WindowDays, view.SampleSize, view.Verdict)
	if view.SampleSize > 0 {
		fmt.Fprintf(out, "  7-day mean:  %.1f ms\n", view.Mean7Day)
		fmt.Fprintf(out, "  30-day mean: %.1f ms\n", view.Mean30Day)
		fmt.Fprintf(out, "  coefficient of variation: %.3f\n", view.CoeffOfVariance)
	}
	if view.Note != "" {
		fmt.Fprintln(out, "note:", view.Note)
	}
	return nil
}
