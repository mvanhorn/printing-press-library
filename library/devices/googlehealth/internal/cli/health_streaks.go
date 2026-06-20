// Copyright 2026 ryanc00per and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/devices/googlehealth/internal/health"
	"github.com/mvanhorn/printing-press-library/library/devices/googlehealth/internal/store"
	"github.com/spf13/cobra"
)

// pp:data-source local
// streaks reads only the local SQLite store; it computes over already-synced
// data points and never calls the API.
func newStreaksCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var metric string
	var threshold float64
	var op string

	cmd := &cobra.Command{
		Use:   "streaks",
		Short: "Consecutive-day goal streaks for a health metric",
		Long: `Find the current and longest run of consecutive calendar days where a
metric's daily value met a goal — e.g. days in a row hitting 10,000 steps
or keeping resting heart rate under 60. A gap in days breaks a streak,
so the current streak reflects a genuinely unbroken run ending today.

Data must be synced first with the sync command.`,
		Example: `  # Days in a row hitting 10k steps
  googlehealth-pp-cli streaks --metric steps --threshold 10000 --op ">="

  # Resting HR kept under 60, as JSON
  googlehealth-pp-cli streaks --metric daily-resting-heart-rate --threshold 60 --op "<" --json`,
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
			// Auto-select the most data-rich metric when none is named so a
			// bare `streaks` still returns something useful.
			if metric == "" {
				top := health.TopMetrics(points, 1)
				if len(top) == 0 {
					if flags.asJSON {
						return printJSONFiltered(out, map[string]any{"status": "no_data"}, flags)
					}
					fmt.Fprintln(out, "No synced health data yet. Run 'googlehealth-pp-cli sync' first.")
					return nil
				}
				metric = top[0]
			}
			res, err := health.Streaks(points, metric, threshold, op)
			if err != nil {
				return err
			}

			if flags.asJSON {
				return printJSONFiltered(out, res, flags)
			}
			fmt.Fprintf(out, "Metric:          %s\n", res.Metric)
			fmt.Fprintf(out, "Goal:            value %s %g\n", res.Op, res.Threshold)
			fmt.Fprintf(out, "Days recorded:   %d (%d qualifying)\n", res.TotalDays, res.QualifyingDays)
			fmt.Fprintf(out, "Current streak:  %d day(s)\n", res.Current)
			fmt.Fprintf(out, "Longest streak:  %d day(s)", res.Longest)
			if res.LongestStart != "" {
				fmt.Fprintf(out, " (%s → %s)", res.LongestStart, res.LongestEnd)
			}
			fmt.Fprintln(out)
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&metric, "metric", "", "Metric to measure (e.g. steps, daily-resting-heart-rate)")
	cmd.Flags().Float64Var(&threshold, "threshold", 0, "Goal threshold value")
	cmd.Flags().StringVar(&op, "op", ">=", "Comparison operator: >=, >, <=, <, ==")
	return cmd
}
