// Copyright 2026 Joseph Alvin Castillo and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source live

package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/revenuecat/internal/client"
	"github.com/mvanhorn/printing-press-library/library/payments/revenuecat/internal/cliutil"
	"github.com/spf13/cobra"
)

// funnelStage is one stage of the trials → paying funnel with the count that
// reached it and the conversion ratio from the prior stage.
type funnelStage struct {
	Stage          string  `json:"stage"`
	Count          float64 `json:"count"`
	ConversionRate float64 `json:"conversion_from_prev"`
	DropOff        float64 `json:"drop_off_from_prev"`
}

type trialFunnelView struct {
	ProjectID  string        `json:"project_id"`
	Resolution string        `json:"resolution"`
	Stages     []funnelStage `json:"stages"`
	NewTrials  float64       `json:"new_trials_total"`
	Converted  float64       `json:"converted_total"`
	OverallPct float64       `json:"overall_conversion_pct"`
	Note       string        `json:"note,omitempty"`
}

func newNovelTrialFunnelCmd(flags *rootFlags) *cobra.Command {
	var projectFlag string
	var since string
	cmd := &cobra.Command{
		Use:   "trial-funnel",
		Short: "New-trials to conversion-to-paying funnel with stage-to-stage drop-off",
		Long: `Fetches the live 'trials_new' and 'conversion_to_paying' charts for a project
and joins them into a two-stage funnel: trials started → converted to paying,
with the conversion ratio and drop-off between stages and an overall conversion
percentage.

Data source: live (chart data is served live by /charts/{chart_name}).`,
		Example: "  revenuecat-pp-cli trial-funnel --project proj1ab2c3d4 --json",
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "live",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return usageErr(err)
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch the trials_new and conversion_to_paying charts and build the funnel")
				return nil
			}
			projectID, err := resolveProjectID(projectFlag)
			if err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			view, err := runTrialFunnel(cmd.Context(), c, projectID, since)
			if err != nil {
				return apiErr(err)
			}
			return emitTrialFunnel(cmd, flags, view)
		},
	}
	cmd.Flags().StringVar(&projectFlag, "project", "", "RevenueCat project id (or set REVENUECAT_PROJECT_ID)")
	cmd.Flags().StringVar(&since, "since", "", "Only count trials since this window (e.g. 7d, 30d, 12w); default = all available data")
	return cmd
}

func emitTrialFunnel(cmd *cobra.Command, flags *rootFlags, view trialFunnelView) error {
	if len(view.Stages) > 0 && wantsHumanTable(cmd.OutOrStdout(), flags) {
		items := make([]map[string]any, 0, len(view.Stages))
		for _, s := range view.Stages {
			items = append(items, map[string]any{
				"stage":           s.Stage,
				"count":           fmt.Sprintf("%.0f", s.Count),
				"conversion_prev": fmt.Sprintf("%.1f%%", s.ConversionRate*100),
				"drop_off_prev":   fmt.Sprintf("%.1f%%", s.DropOff*100),
			})
		}
		if err := printAutoTable(cmd.OutOrStdout(), items); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "\nProject %s  trials=%.0f converted=%.0f overall=%.1f%%\n",
			view.ProjectID, view.NewTrials, view.Converted, view.OverallPct)
		if view.Note != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "Note: %s\n", view.Note)
		}
		return nil
	}
	return flags.printJSON(cmd, view)
}

// runTrialFunnel fetches trials_new + conversion_to_paying and builds the
// stage-to-stage funnel.
//
// Live-verified 2026-05-30 against the RevenueCat v2 API: conversion_to_paying
// reports a per-period COUNT of converted customers, so summing the series is
// correct. convertedTotal additionally guards against a future ratio-shaped
// response (fractional values in (0,1)) by deriving new_trials * mean(ratio)
// instead of producing a nonsensical sum-of-ratios.
func runTrialFunnel(ctx context.Context, c *client.Client, projectID, since string) (trialFunnelView, error) {
	view := trialFunnelView{ProjectID: projectID, Stages: []funnelStage{}}

	params := map[string]string{}
	if since != "" {
		window, err := cliutil.ParseDurationLoose(since)
		if err != nil {
			return view, usageErr(fmt.Errorf("invalid --since %q: %w", since, err))
		}
		params["start_date"] = time.Now().UTC().Add(-window).Format("2006-01-02")
	}

	trialsChart, err := fetchChart(ctx, c, projectID, "trials_new", params)
	if err != nil {
		return view, fmt.Errorf("fetching trials_new chart: %w", err)
	}
	convChart, err := fetchChart(ctx, c, projectID, "conversion_to_paying", params)
	if err != nil {
		return view, fmt.Errorf("fetching conversion_to_paying chart: %w", err)
	}
	view.Resolution = trialsChart.Resolution
	view.NewTrials = sumFirstSeries(trialsChart)
	view.Converted = convertedTotal(convChart, view.NewTrials)
	var note string
	view.Stages, view.OverallPct, note = buildFunnel(view.NewTrials, view.Converted)
	if view.NewTrials == 0 && view.Converted == 0 {
		view.Note = "trials_new and conversion_to_paying charts returned no data points"
	} else if note != "" {
		view.Note = note
	}
	return view, nil
}

// buildFunnel computes the two-stage funnel and overall conversion percentage
// from the new-trial and converted totals. Pure function for unit-testing.
//
// The leading trials_new stage has no prior stage, so its conversion_from_prev
// is reported as 0 (not 1.0) — a 100%-of-nothing ratio is misleading. When the
// converted cohort exceeds the new-trials cohort in the window (conversions
// whose trials started earlier), the funnel can't be expressed as a clean
// drop-off, so conversion/drop-off on the converted stage are left at 0 and a
// note explains the out-of-window mismatch instead of emitting a contradictory
// >100% ratio.
func buildFunnel(newTrials, converted float64) ([]funnelStage, float64, string) {
	stages := []funnelStage{{
		Stage:          "trials_new",
		Count:          newTrials,
		ConversionRate: 0,
	}}
	conv := funnelStage{Stage: "converted_to_paying", Count: converted}
	overall := 0.0
	note := ""
	if newTrials > 0 && converted <= newTrials {
		conv.ConversionRate = converted / newTrials
		conv.DropOff = 1.0 - conv.ConversionRate
		overall = 100.0 * converted / newTrials
	} else if converted > 0 && converted > newTrials {
		note = "converted_to_paying exceeds trials_new in this window; conversions counted here include trials started before the window, so a stage-to-stage rate isn't meaningful"
	}
	stages = append(stages, conv)
	return stages, overall, note
}

// sumFirstSeries sums the first numeric series value across every period of a
// chart.
func sumFirstSeries(cd chartData) float64 {
	var total float64
	for _, p := range cd.points() {
		total += p.firstSeriesValue()
	}
	return total
}

// convertedTotal interprets the conversion_to_paying chart into a total count of
// converted customers. RevenueCat reports a per-period count (live-verified), so
// the default is the sum. If the series is instead ratio-shaped — any period
// value is strictly fractional in (0,1) — summing would be meaningless, so it
// derives newTrials * mean(ratio) as the converted count.
func convertedTotal(conv chartData, newTrials float64) float64 {
	pts := conv.points()
	if len(pts) == 0 {
		return 0
	}
	var sum float64
	ratioShaped := false
	for _, p := range pts {
		v := p.firstSeriesValue()
		sum += v
		if v > 0 && v < 1 {
			ratioShaped = true
		}
	}
	if ratioShaped {
		return newTrials * (sum / float64(len(pts)))
	}
	return sum
}
