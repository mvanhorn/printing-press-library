// Copyright 2026 laci141 and contributors. Licensed under Apache-2.0. See LICENSE.

// velocity.go implements the `velocity` command: how fast a disease area's trial
// activity and enrollment are growing over time. It samples trials, buckets them
// by start year, and reports the per-year trial and enrollment series plus a
// recent-vs-prior growth rate so an analyst can see acceleration or decline.
package cli

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type yearBucket struct {
	Year       int `json:"year"`
	Trials     int `json:"trials"`
	Enrollment int `json:"enrollment"`
}

type velocityView struct {
	Query             string       `json:"query"`
	SampleSize        int          `json:"sample_size"`
	RegistryTotal     int          `json:"registry_total,omitempty"`
	WindowYears       int          `json:"window_years"`
	RecentFrom        int          `json:"recent_from"`
	RecentTo          int          `json:"recent_to"`
	PriorFrom         int          `json:"prior_from"`
	PriorTo           int          `json:"prior_to"`
	TrialCountsSource string       `json:"trial_counts_source"`
	RecentTrials      int          `json:"recent_trials"`
	PriorTrials       int          `json:"prior_trials"`
	TrialGrowthPct    int          `json:"trial_growth_pct"`
	RecentEnroll      int          `json:"recent_enrollment"`
	PriorEnroll       int          `json:"prior_enrollment"`
	EnrollGrowthPct   int          `json:"enrollment_growth_pct"`
	LaterTrials       int          `json:"later_trials"`
	OlderTrials       int          `json:"older_trials"`
	ByYear            []yearBucket `json:"by_year,omitempty"`
	Note              string       `json:"note,omitempty"`
}

// pp:data-source live
func newNovelVelocityCmd(flags *rootFlags) *cobra.Command {
	var sample, maxScanPages, windowYears int
	cmd := &cobra.Command{
		Use:   "velocity <area>",
		Short: "Measure how fast a disease area's trials and enrollment are growing over time.",
		Long: "Sample trials for a disease area, bucket them by start year, and report the\n" +
			"per-year trial and enrollment series plus a recent-vs-prior growth rate, so you\n" +
			"can see whether research is accelerating or slowing. Both windows are runs of\n" +
			"complete calendar years of the same length: the recent one ends last year and\n" +
			"the prior one sits just before it. The current year is incomplete and is left out.",
		Example: "  clinical-trials-pp-cli velocity alzheimer --json\n" +
			"  clinical-trials-pp-cli velocity \"long covid\" --window-years 5",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would measure trial velocity from ClinicalTrials.gov")
				return nil
			}
			term := strings.TrimSpace(strings.Join(args, " "))
			if term == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("an area argument is required"))
			}
			if windowYears < 1 {
				return usageErr(fmt.Errorf("--window-years must be at least 1"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			view, err := buildVelocityView(ctx, c, term, sample, maxScanPages, windowYears, time.Now())
			if err != nil {
				return classifyAPIError(err, flags)
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				if err := printJSONFiltered(cmd.OutOrStdout(), view, flags); err != nil {
					return err
				}
			} else if err := renderVelocityHuman(cmd, view); err != nil {
				return err
			}
			if view.SampleSize == 0 {
				return noResultsErr(term)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&sample, "sample", 400, "Max trials to analyze")
	cmd.Flags().IntVar(&maxScanPages, "max-scan-pages", 3, "Max ClinicalTrials.gov pages to scan")
	cmd.Flags().IntVar(&windowYears, "window-years", 3, "Length in complete calendar years of each window; the recent window ends last year and the prior window is the same length just before it")
	return cmd
}

// growthPct returns recent-vs-prior growth as a rounded percentage. When prior
// is zero it reports 100 (all activity is new) rather than dividing by zero.
func growthPct(recent, prior int) int {
	if prior <= 0 {
		if recent > 0 {
			return 100
		}
		return 0
	}
	return int(math.Round(float64(recent-prior) * 100.0 / float64(prior)))
}

func renderVelocityHuman(cmd *cobra.Command, v velocityView) error {
	return writeVelocityHuman(cmd.OutOrStdout(), v)
}
