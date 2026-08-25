// Copyright 2026 laci141 and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// growthEntry reports an intervention/condition's recent vs prior cohort counts.
type growthEntry struct {
	Label      string `json:"label"`
	Recent     int    `json:"recent"`
	Prior      int    `json:"prior"`
	GrowthPct  int    `json:"growth_pct"`
	NewlyAdded bool   `json:"newly_added,omitempty"`
}

type emergingView struct {
	Query              string        `json:"query"`
	TotalTrials        int           `json:"total_trials"`
	RecruitingTrials   int           `json:"recruiting_trials"`
	RecruitingPct      int           `json:"recruiting_pct"`
	SampleSize         int           `json:"sample_size"`
	RecentCohort       int           `json:"recent_cohort"`
	PriorCohort        int           `json:"prior_cohort"`
	RecentSinceYear    int           `json:"recent_since_year"`
	PhaseDistribution  []rankedEntry `json:"phase_distribution,omitempty"`
	FastestGrowing     []growthEntry `json:"fastest_growing_interventions,omitempty"`
	EmergingConditions []growthEntry `json:"emerging_conditions,omitempty"`
	GeographicHotspots []rankedEntry `json:"geographic_hotspots,omitempty"`
	Note               string        `json:"note,omitempty"`
}

// pp:data-source live
func newNovelEmergingCmd(flags *rootFlags) *cobra.Command {
	var limit, sample, maxScanPages, recentYears int
	cmd := &cobra.Command{
		Use:   "emerging [category]",
		Short: "See the fastest-growing trial categories for a disease area, with percent change.",
		Long: "Detect where clinical research is moving for a disease area or therapy category.\n" +
			"Compares a recent cohort of trials against an older one to surface the fastest-growing\n" +
			"interventions and conditions, plus active-recruiting share, phase mix, and geographic hotspots.\n" +
			"Omit the category to analyze the most recently updated trials across all areas.",
		Example:     "  clinical-trials-pp-cli emerging cancer --json\n  clinical-trials-pp-cli emerging \"alzheimer\" --limit 15",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would analyze emerging trends from ClinicalTrials.gov")
				return nil
			}
			if sample < 1 {
				return usageErr(fmt.Errorf("--sample must be at least 1"))
			}
			category := strings.TrimSpace(strings.Join(args, " "))
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			baseParams := ctgovParams("cond", category)
			baseParams["sort"] = "LastUpdatePostDate:desc"

			total, err := ctgovCount(ctx, c, ctgovParams("cond", category))
			if err != nil {
				return classifyAPIError(err, flags)
			}
			recParams := ctgovParams("cond", category)
			recParams["filter.overallStatus"] = "RECRUITING"
			recruiting, err := ctgovCount(ctx, c, recParams)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			trials, err := ctgovFetch(ctx, c, baseParams, sample, maxScanPages)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			cutoff := time.Now().Year() - recentYears
			recentIv, priorIv := newCounter(), newCounter()
			recentCond, priorCond := newCounter(), newCounter()
			phase := newCounter()
			geo := newCounter()
			recentN, priorN := 0, 0
			for _, t := range trials {
				yr := startYear(t.StartDate)
				recent := yr == 0 || yr >= cutoff // unknown dates count as recent (most-recently-updated sample)
				if recent {
					recentN++
				} else {
					priorN++
				}
				for _, iv := range t.Interventions {
					key := normalizeCategoryToken(iv)
					if recent {
						recentIv.add(key)
					} else {
						priorIv.add(key)
					}
				}
				for _, cond := range t.Conditions {
					key := normalizeCategoryToken(cond)
					if recent {
						recentCond.add(key)
					} else {
						priorCond.add(key)
					}
				}
				for _, ph := range t.Phases {
					phase.add(phaseLabel(ph))
				}
				for _, country := range t.Countries {
					geo.add(country)
				}
			}

			view := emergingView{
				Query:              category,
				TotalTrials:        total,
				RecruitingTrials:   recruiting,
				RecruitingPct:      percentOf(recruiting, total),
				SampleSize:         len(trials),
				RecentCohort:       recentN,
				PriorCohort:        priorN,
				RecentSinceYear:    cutoff,
				PhaseDistribution:  phase.top(8),
				FastestGrowing:     growth(recentIv, priorIv, limit),
				EmergingConditions: growth(recentCond, priorCond, limit),
				GeographicHotspots: geo.top(10),
			}
			if len(trials) == 0 {
				view.Note = "no trials matched; try a broader category term"
			} else if priorN == 0 {
				view.Note = "all sampled trials fall in the recent cohort; growth percentages reflect newly-appearing categories. Widen --sample or lower --recent-years for a comparison baseline."
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				if err := printJSONFiltered(cmd.OutOrStdout(), view, flags); err != nil {
					return err
				}
			} else if err := renderEmergingHuman(cmd, view); err != nil {
				return err
			}
			if len(trials) == 0 {
				return noResultsErr(category)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 10, "Max categories to list in each trend section")
	cmd.Flags().IntVar(&sample, "sample", 300, "Max trials to analyze for trend computation")
	cmd.Flags().IntVar(&maxScanPages, "max-scan-pages", 2, "Max ClinicalTrials.gov pages to scan")
	cmd.Flags().IntVar(&recentYears, "recent-years", 3, "Trials started within this many years count as the recent cohort")
	return cmd
}

// startYear parses the 4-digit year from a CT.gov date string ("2024-11-21",
// "2024-11", or "2024"); returns 0 when unknown.
func startYear(date string) int {
	if len(date) < 4 {
		return 0
	}
	y, err := strconv.Atoi(date[:4])
	if err != nil {
		return 0
	}
	return y
}

// normalizeCategoryToken lowercases and trims an intervention/condition name so
// near-duplicates collapse into one tally bucket.
func normalizeCategoryToken(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	// Drop dosage/parenthetical detail that fragments otherwise-equal names.
	if i := strings.IndexByte(s, '('); i > 0 {
		s = strings.TrimSpace(s[:i])
	}
	return s
}

// growth computes per-label growth between recent and prior cohorts, returning
// the top n by recent count (only labels with a meaningful recent presence).
func growth(recent, prior *counter, n int) []growthEntry {
	var out []growthEntry
	for label, rc := range recent.counts {
		if rc < 2 {
			continue // ignore one-off noise
		}
		pc := prior.counts[label]
		entry := growthEntry{Label: label, Recent: rc, Prior: pc}
		if pc == 0 {
			entry.NewlyAdded = true
			entry.GrowthPct = 100
		} else {
			entry.GrowthPct = int(float64(rc-pc)*100.0/float64(pc) + 0.5)
		}
		out = append(out, entry)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].GrowthPct != out[j].GrowthPct {
			return out[i].GrowthPct > out[j].GrowthPct
		}
		if out[i].Recent != out[j].Recent {
			return out[i].Recent > out[j].Recent
		}
		return out[i].Label < out[j].Label
	})
	if n > 0 && len(out) > n {
		out = out[:n]
	}
	return out
}

func renderEmergingHuman(cmd *cobra.Command, v emergingView) error {
	w := cmd.OutOrStdout()
	label := v.Query
	if label == "" {
		label = "all areas"
	}
	fmt.Fprintf(w, "Emerging trends — %s\n", label)
	fmt.Fprintf(w, "  Total trials: %d   Recruiting: %d (%d%%)   Sample analyzed: %d (recent %d / prior %d)\n\n",
		v.TotalTrials, v.RecruitingTrials, v.RecruitingPct, v.SampleSize, v.RecentCohort, v.PriorCohort)
	if len(v.FastestGrowing) > 0 {
		fmt.Fprintln(w, "Fastest-growing interventions (recent vs prior cohort):")
		for _, g := range v.FastestGrowing {
			tag := fmt.Sprintf("%+d%%", g.GrowthPct)
			if g.NewlyAdded {
				tag = "new"
			}
			fmt.Fprintf(w, "  %-40s %d trials (%s)\n", truncate(g.Label, 40), g.Recent, tag)
		}
		fmt.Fprintln(w)
	}
	if len(v.PhaseDistribution) > 0 {
		fmt.Fprintln(w, "Phase distribution (sample):")
		for _, p := range v.PhaseDistribution {
			fmt.Fprintf(w, "  %-14s %d\n", p.Label, p.Count)
		}
		fmt.Fprintln(w)
	}
	if len(v.GeographicHotspots) > 0 {
		fmt.Fprintln(w, "Geographic hotspots:")
		for _, g := range v.GeographicHotspots {
			fmt.Fprintf(w, "  %-24s %d\n", g.Label, g.Count)
		}
	}
	if v.Note != "" {
		fmt.Fprintf(w, "\nnote: %s\n", v.Note)
	}
	return nil
}
