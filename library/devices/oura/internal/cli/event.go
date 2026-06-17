// Copyright 2026 slinsmaier and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/devices/oura/internal/store"
)

type eventMetricImpact struct {
	Metric     string  `json:"metric"`
	BeforeMean float64 `json:"before_mean,omitempty"`
	OnDay      float64 `json:"on_day_value,omitempty"`
	HasOnDay   bool    `json:"has_on_day_value"`
	AfterMean  float64 `json:"after_mean,omitempty"`
	DeltaPct   float64 `json:"after_vs_before_delta_pct,omitempty"`
}

type eventView struct {
	Date    string              `json:"date"`
	Label   string              `json:"label,omitempty"`
	Window  int                 `json:"window_days"`
	Impacts []eventMetricImpact `json:"impacts"`
	Note    string              `json:"note,omitempty"`
}

func newNovelEventCmd(flags *rootFlags) *cobra.Command {
	var flagDate string
	var flagLabel string
	var flagWindow int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "event",
		Short: "Show all key metrics for N days before and after a named date to measure the health impact of travel, illness, a race",
		Long: `Compares before/on/after averages for sleep, readiness, activity, and stress
around a named date, to measure the health impact of travel, illness, a
race, or any other life event.`,
		Example:     `  oura-pp-cli event --date 2026-06-01 --label "marathon" --window 3 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		// pp:data-source local
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would compare before/on/after metric averages around --date")
				return nil
			}
			if flagDate == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--date is required (YYYY-MM-DD)"))
			}
			if flagWindow < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--window must be >= 1, got %d", flagWindow))
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

			before := addDays(flagDate, -flagWindow)
			beforeEnd := addDays(flagDate, -1)
			after := addDays(flagDate, 1)
			afterEnd := addDays(flagDate, flagWindow)

			view := eventView{Date: flagDate, Window: flagWindow, Label: flagLabel}

			samples := 0
			for _, name := range []string{"sleep", "readiness", "activity", "stress"} {
				spec, _ := resolveMetric(name)
				full, err := metricSeries(db, spec, before, afterEnd)
				if err != nil {
					return fmt.Errorf("querying %s: %w", name, err)
				}
				impact := eventMetricImpact{Metric: name}
				if beforeVals := valuesInRange(full, before, beforeEnd); len(beforeVals) > 0 {
					m, _ := meanStdDev(beforeVals)
					impact.BeforeMean = round2(m)
				}
				if v, ok := full[flagDate]; ok {
					impact.OnDay = round2(v)
					impact.HasOnDay = true
				}
				if afterVals := valuesInRange(full, after, afterEnd); len(afterVals) > 0 {
					m, _ := meanStdDev(afterVals)
					impact.AfterMean = round2(m)
				}
				if impact.BeforeMean != 0 {
					impact.DeltaPct = round2((impact.AfterMean - impact.BeforeMean) / impact.BeforeMean * 100)
				}
				samples += len(full)
				view.Impacts = append(view.Impacts, impact)
			}
			if samples == 0 {
				view.Note = "no synced data around this date — run 'oura-pp-cli sync' first"
			}

			if flags.asJSON || flags.agent {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			out := cmd.OutOrStdout()
			label := view.Label
			if label == "" {
				label = "event"
			}
			fmt.Fprintf(out, "%s on %s (±%d days)\n", label, view.Date, view.Window)
			for _, imp := range view.Impacts {
				fmt.Fprintf(out, "  %-10s before=%.1f on-day=", imp.Metric, imp.BeforeMean)
				if imp.HasOnDay {
					fmt.Fprintf(out, "%.1f", imp.OnDay)
				} else {
					fmt.Fprint(out, "-")
				}
				fmt.Fprintf(out, " after=%.1f (%+.1f%%)\n", imp.AfterMean, imp.DeltaPct)
			}
			if view.Note != "" {
				fmt.Fprintln(out, "note:", view.Note)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagDate, "date", "", "Event date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&flagLabel, "label", "", "Human-readable label for the event (e.g. 'marathon', 'flight to Tokyo')")
	cmd.Flags().IntVar(&flagWindow, "window", 3, "Number of days before/after the event date to compare")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
