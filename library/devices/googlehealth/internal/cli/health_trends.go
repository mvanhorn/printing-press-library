// Copyright 2026 ryanc00per and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/devices/googlehealth/internal/health"
	"github.com/mvanhorn/printing-press-library/library/devices/googlehealth/internal/store"
	"github.com/spf13/cobra"
)

// pp:data-source local
// trends reads only the local SQLite store; it computes over already-synced
// data points and never calls the API.
func newTrendsCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var metric string
	var window int

	cmd := &cobra.Command{
		Use:   "trends",
		Short: "De-noised rolling-average trend lines per health metric",
		Long: `Compute a trailing rolling-average trend line for each synced health
metric (steps, weight, resting heart rate, …) so you see the real
direction of travel instead of day-to-day noise. Reports each metric's
first vs. last rolling value and the net delta.

Data must be synced first with the sync command.`,
		Example: `  # Trends for every metric, 7-day rolling window
  googlehealth-pp-cli trends --window 7

  # Just weight, 14-day window, as JSON
  googlehealth-pp-cli trends --metric weight --window 14 --json`,
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
			trends := health.Trends(points, metric, window)

			if flags.asJSON {
				return printJSONFiltered(out, trends, flags)
			}
			if len(trends) == 0 {
				fmt.Fprintln(out, "No health data points found. Run 'googlehealth-pp-cli sync' first.")
				return nil
			}
			fmt.Fprintf(out, "%-32s %6s %12s %12s %12s\n", "METRIC", "DAYS", "FIRST", "LAST", "DELTA")
			for _, tr := range trends {
				fmt.Fprintf(out, "%-32s %6d %12.2f %12.2f %+12.2f\n",
					tr.Metric, tr.Days, tr.First, tr.Last, tr.Delta)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&metric, "metric", "", "Limit to a single metric (e.g. steps, weight)")
	cmd.Flags().IntVar(&window, "window", 7, "Rolling-average window in days")
	return cmd
}
