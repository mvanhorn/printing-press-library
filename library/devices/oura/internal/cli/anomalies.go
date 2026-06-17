// Copyright 2026 slinsmaier and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/devices/oura/internal/store"
	"github.com/spf13/cobra"
)

type anomalyRow struct {
	Day    string  `json:"day"`
	Value  float64 `json:"value"`
	Mean   float64 `json:"baseline_mean"`
	StdDev float64 `json:"baseline_stdev"`
	ZScore float64 `json:"z_score"`
}

type anomaliesView struct {
	Metric    string       `json:"metric"`
	Since     string       `json:"since"`
	SigmaCut  float64      `json:"sigma_threshold"`
	Days      int          `json:"days_checked"`
	Anomalies []anomalyRow `json:"anomalies"`
	Note      string       `json:"note,omitempty"`
}

func newNovelAnomaliesCmd(flags *rootFlags) *cobra.Command {
	var flagSince string
	var flagSigma float64
	var flagMetric string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "anomalies",
		Short: "Flag days where any metric fell more than N standard deviations from your personal rolling mean",
		Long: `Computes a trailing 30-day rolling mean and standard deviation for each day
in the window (excluding that day itself) and flags days whose value
deviates from that personal baseline by more than --sigma standard
deviations. Data-driven, not population norms.`,
		Example: `  oura-pp-cli anomalies --metric readiness --since 30d
  oura-pp-cli anomalies --metric sleep --sigma 1.5 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		// pp:data-source local
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would flag days deviating more than --sigma stdevs from the personal rolling baseline")
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
			start, err := resolveSinceDay(flagSince, 30)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			end := today()

			if dbPath == "" {
				dbPath = defaultDBPath("oura-pp-cli")
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprint(cmd.ErrOrStderr(), missingMirrorMessage(dbPath))
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
				}
				return nil
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			// Pull a wider window than requested so the rolling baseline for
			// the earliest checked days still has up to 30 prior days to draw on.
			fullSeries, err := metricSeries(db, spec, addDays(start, -30), end)
			if err != nil {
				return fmt.Errorf("querying %s: %w", flagMetric, err)
			}

			view := anomaliesView{Metric: flagMetric, Since: start, SigmaCut: flagSigma}
			checked := 0
			for d := start; d <= end; d = addDays(d, 1) {
				v, ok := fullSeries[d]
				if !ok {
					continue
				}
				checked++
				baseline := valuesInRange(fullSeries, addDays(d, -30), addDays(d, -1))
				if len(baseline) < 7 {
					continue
				}
				mean, stdDev := meanStdDev(baseline)
				if stdDev == 0 {
					continue
				}
				z := (v - mean) / stdDev
				if z < 0 {
					z = -z
				}
				if z >= flagSigma {
					view.Anomalies = append(view.Anomalies, anomalyRow{
						Day: d, Value: round2(v), Mean: round2(mean), StdDev: round2(stdDev), ZScore: round2(z),
					})
				}
			}
			view.Days = checked
			if checked == 0 {
				view.Note = "no synced data for this metric in the requested window — run 'oura-pp-cli sync' first"
			}

			if flags.asJSON || flags.agent {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			out := cmd.OutOrStdout()
			if len(view.Anomalies) == 0 {
				fmt.Fprintf(out, "no anomalies found for %s across %d day(s) (sigma >= %.1f)\n", flagMetric, view.Days, flagSigma)
			} else {
				fmt.Fprintln(out, "day\tvalue\tbaseline_mean\tz_score")
				for _, a := range view.Anomalies {
					fmt.Fprintf(out, "%s\t%.1f\t%.1f\t%.2f\n", a.Day, a.Value, a.Mean, a.ZScore)
				}
			}
			if view.Note != "" {
				fmt.Fprintln(out, "note:", view.Note)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagMetric, "metric", "", "Metric to scan for anomalies: "+joinMetrics())
	cmd.Flags().StringVar(&flagSince, "since", "", "Start of the window: a duration like 30d or an absolute YYYY-MM-DD date (default 30d)")
	cmd.Flags().Float64Var(&flagSigma, "sigma", 2.0, "Standard-deviation threshold for flagging a day as anomalous")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
