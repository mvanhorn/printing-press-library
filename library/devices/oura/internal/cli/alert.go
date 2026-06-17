// Copyright 2026 slinsmaier and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/devices/oura/internal/store"
	"github.com/spf13/cobra"
)

type alertView struct {
	Metric      string   `json:"metric"`
	Direction   string   `json:"direction"`
	Threshold   float64  `json:"threshold"`
	Consecutive int      `json:"consecutive_required"`
	Streak      int      `json:"streak"`
	Triggered   bool     `json:"triggered"`
	Days        []string `json:"days_checked"`
	Note        string   `json:"note,omitempty"`
}

func newNovelAlertCmd(flags *rootFlags) *cobra.Command {
	var flagMetric string
	var flagThreshold float64
	var flagConsecutive int
	var flagDirection string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "alert",
		Short: "Exit 1 when a metric has been above or below a threshold for N consecutive days — the scripting primitive that Oura has never exposed",
		Long: `Checks whether the most recent N consecutive days all sit above or below a
threshold for a given metric. Exits 1 when the alert condition is met, 0
otherwise — the missing primitive for shell-scripted health alerting.`,
		Example: `  oura-pp-cli alert --metric readiness --threshold 70 --direction below --consecutive 3
  oura-pp-cli alert --metric stress --threshold 120 --direction above --consecutive 2 --json`,
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,1"},
		// pp:data-source local
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would check whether --metric crossed --threshold for --consecutive consecutive days")
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
			if flagDirection != "above" && flagDirection != "below" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--direction must be 'above' or 'below', got %q", flagDirection))
			}
			if flagConsecutive < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--consecutive must be >= 1, got %d", flagConsecutive))
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

			end := today()
			start := addDays(end, -(flagConsecutive + 7))
			series, err := metricSeries(db, spec, start, end)
			if err != nil {
				return fmt.Errorf("querying %s: %w", flagMetric, err)
			}

			view := alertView{
				Metric:      flagMetric,
				Direction:   flagDirection,
				Threshold:   flagThreshold,
				Consecutive: flagConsecutive,
			}

			// Anchor the streak walk on the most recent day that actually has
			// synced data, not literal "today" — today's reading often hasn't
			// synced yet, which would otherwise always report streak=0 even
			// when a real streak ends yesterday.
			days := sortedDays(series)
			if len(days) > 0 {
				end = days[len(days)-1]
			}

			streak := 0
			for d := end; ; d = addDays(d, -1) {
				v, ok := series[d]
				if !ok {
					break
				}
				crossed := (flagDirection == "below" && v < flagThreshold) || (flagDirection == "above" && v > flagThreshold)
				if !crossed {
					break
				}
				view.Days = append([]string{d}, view.Days...)
				streak++
				if streak >= flagConsecutive {
					break
				}
			}
			view.Streak = streak
			view.Triggered = streak >= flagConsecutive
			if len(series) == 0 {
				view.Note = "no synced data for this metric — run 'oura-pp-cli sync' first"
			}

			if flags.asJSON || flags.agent {
				if err := printJSONFiltered(cmd.OutOrStdout(), view, flags); err != nil {
					return err
				}
			} else {
				out := cmd.OutOrStdout()
				fmt.Fprintf(out, "%s %s %v: streak=%d/%d triggered=%v\n", flagMetric, flagDirection, flagThreshold, view.Streak, view.Consecutive, view.Triggered)
				if view.Note != "" {
					fmt.Fprintln(out, "note:", view.Note)
				}
			}

			if view.Triggered {
				return fmt.Errorf("alert: %s has been %s %v for %d consecutive day(s)", flagMetric, flagDirection, flagThreshold, view.Streak)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagMetric, "metric", "", "Metric to check: "+joinMetrics())
	cmd.Flags().Float64Var(&flagThreshold, "threshold", 0, "Threshold value to compare the metric against")
	cmd.Flags().IntVar(&flagConsecutive, "consecutive", 3, "Number of consecutive recent days that must cross the threshold")
	cmd.Flags().StringVar(&flagDirection, "direction", "below", "Direction to alert on: 'above' or 'below'")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
