// Copyright 2026 laci141 and contributors. Licensed under Apache-2.0. See LICENSE.

// velocity.go implements the `velocity` command: how fast a disease area's trial
// activity and enrollment are growing over time. It samples trials, buckets them
// by start year, and reports the per-year trial and enrollment series plus a
// recent-vs-prior growth rate so an analyst can see acceleration or decline.
package cli

import (
	"fmt"
	"math"
	"sort"
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
	Query           string       `json:"query"`
	SampleSize      int          `json:"sample_size"`
	WindowYears     int          `json:"window_years"`
	RecentTrials    int          `json:"recent_trials"`
	PriorTrials     int          `json:"prior_trials"`
	TrialGrowthPct  int          `json:"trial_growth_pct"`
	RecentEnroll    int          `json:"recent_enrollment"`
	PriorEnroll     int          `json:"prior_enrollment"`
	EnrollGrowthPct int          `json:"enrollment_growth_pct"`
	ByYear          []yearBucket `json:"by_year,omitempty"`
	Note            string       `json:"note,omitempty"`
}

// pp:data-source live
func newNovelVelocityCmd(flags *rootFlags) *cobra.Command {
	var sample, maxScanPages, windowYears int
	cmd := &cobra.Command{
		Use:   "velocity <area>",
		Short: "Measure how fast a disease area's trials and enrollment are growing over time.",
		Long: "Sample trials for a disease area, bucket them by start year, and report the\n" +
			"per-year trial and enrollment series plus a recent-vs-prior growth rate over\n" +
			"the chosen window, so you can see whether research is accelerating or slowing.",
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
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			trials, err := ctgovFetch(ctx, c, ctgovParams("cond", term), sample, maxScanPages)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			byYear := map[int]*yearBucket{}
			cutoff := time.Now().Year() - windowYears
			var recentT, priorT, recentE, priorE int
			for _, t := range trials {
				yr := startYear(t.StartDate)
				if yr == 0 {
					continue // unknown start date can't be placed on the timeline
				}
				b, ok := byYear[yr]
				if !ok {
					b = &yearBucket{Year: yr}
					byYear[yr] = b
				}
				b.Trials++
				if t.Enrollment > 0 {
					b.Enrollment += t.Enrollment
				}
				if yr >= cutoff {
					recentT++
					recentE += t.Enrollment
				} else {
					priorT++
					priorE += t.Enrollment
				}
			}

			series := make([]yearBucket, 0, len(byYear))
			for _, b := range byYear {
				series = append(series, *b)
			}
			sort.Slice(series, func(i, j int) bool { return series[i].Year < series[j].Year })

			view := velocityView{
				Query:           term,
				SampleSize:      len(trials),
				WindowYears:     windowYears,
				RecentTrials:    recentT,
				PriorTrials:     priorT,
				TrialGrowthPct:  growthPct(recentT, priorT),
				RecentEnroll:    recentE,
				PriorEnroll:     priorE,
				EnrollGrowthPct: growthPct(recentE, priorE),
				ByYear:          series,
			}
			switch {
			case len(trials) == 0:
				view.Note = "no trials matched; try a broader area term"
			case len(series) == 0:
				view.Note = "matched trials have no parseable start dates; cannot place them on a timeline"
			case priorT == 0:
				view.Note = "no trials in the prior window; growth reflects newly-appearing activity. Widen --window-years or --sample for a baseline."
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				if err := printJSONFiltered(cmd.OutOrStdout(), view, flags); err != nil {
					return err
				}
			} else if err := renderVelocityHuman(cmd, view); err != nil {
				return err
			}
			if len(trials) == 0 {
				return noResultsErr(term)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&sample, "sample", 400, "Max trials to analyze")
	cmd.Flags().IntVar(&maxScanPages, "max-scan-pages", 3, "Max ClinicalTrials.gov pages to scan")
	cmd.Flags().IntVar(&windowYears, "window-years", 3, "Trials started within this many years count as the recent window")
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
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "Trial velocity — %s (sample %d)\n", v.Query, v.SampleSize)
	fmt.Fprintf(w, "  Recent %dy: %d trials (%+d%%), %d enrolled (%+d%%)  vs prior: %d trials, %d enrolled\n\n",
		v.WindowYears, v.RecentTrials, v.TrialGrowthPct, v.RecentEnroll, v.EnrollGrowthPct, v.PriorTrials, v.PriorEnroll)
	if len(v.ByYear) > 0 {
		fmt.Fprintln(w, "By start year:")
		for _, b := range v.ByYear {
			fmt.Fprintf(w, "  %d  %4d trials  %7d enrolled\n", b.Year, b.Trials, b.Enrollment)
		}
	}
	if v.Note != "" {
		fmt.Fprintf(w, "\nnote: %s\n", v.Note)
	}
	return nil
}
