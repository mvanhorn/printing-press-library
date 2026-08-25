// Copyright 2026 laci141 and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type trialListView struct {
	Query             string        `json:"query"`
	Filter            string        `json:"filter,omitempty"`
	TotalMatching     int           `json:"total_matching"`
	Returned          int           `json:"returned"`
	PhaseDistribution []rankedEntry `json:"phase_distribution,omitempty"`
	TopCountries      []rankedEntry `json:"top_countries,omitempty"`
	Trials            []Trial       `json:"trials"`
	Note              string        `json:"note,omitempty"`
}

func newRecruitingCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "recruiting <condition>",
		Short: "List actively-recruiting trials for a condition, with phase and location context.",
		Long: "Returns trials whose overall status is RECRUITING for a condition or disease,\n" +
			"with the total recruiting count, phase distribution, and top recruiting countries.",
		Example:     "  clinical-trials-pp-cli recruiting cancer --json\n  clinical-trials-pp-cli recruiting \"type 2 diabetes\" --limit 25",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch recruiting trials from ClinicalTrials.gov")
				return nil
			}
			term := strings.TrimSpace(strings.Join(args, " "))
			if term == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a condition argument is required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			params := ctgovParams("cond", term)
			params["filter.overallStatus"] = "RECRUITING"
			total, err := ctgovCount(ctx, c, params)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			trials, err := ctgovFetch(ctx, c, params, limit, 3)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			view := buildTrialListView(term, "RECRUITING", total, trials)
			if len(trials) == 0 {
				view.Note = "no recruiting trials matched; try a broader condition term"
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				if err := printJSONFiltered(cmd.OutOrStdout(), view, flags); err != nil {
					return err
				}
			} else if err := renderTrialList(cmd, view); err != nil {
				return err
			}
			if len(trials) == 0 {
				return noResultsErr(term)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "Max trials to return")
	return cmd
}

// buildTrialListView assembles the shared summary from a fetched trial slice.
func buildTrialListView(query, filter string, total int, trials []Trial) trialListView {
	phase := newCounter()
	geo := newCounter()
	for _, t := range trials {
		for _, ph := range t.Phases {
			phase.add(phaseLabel(ph))
		}
		for _, country := range t.Countries {
			geo.add(country)
		}
	}
	return trialListView{
		Query:             query,
		Filter:            filter,
		TotalMatching:     total,
		Returned:          len(trials),
		PhaseDistribution: phase.top(8),
		TopCountries:      geo.top(10),
		Trials:            trials,
	}
}

func renderTrialList(cmd *cobra.Command, v trialListView) error {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "%s — %d matching", v.Query, v.TotalMatching)
	if v.Filter != "" {
		fmt.Fprintf(w, " (%s)", v.Filter)
	}
	fmt.Fprintf(w, ", showing %d\n\n", v.Returned)
	for _, t := range v.Trials {
		fmt.Fprintf(w, "%s  %s\n", t.NCTID, truncate(t.Title, 80))
		fmt.Fprintf(w, "    status=%s phase=%s sponsor=%s\n", t.Status, phaseDisplay(t.Phases), truncate(t.Sponsor, 40))
	}
	if v.Note != "" {
		fmt.Fprintf(w, "\nnote: %s\n", v.Note)
	}
	return nil
}

func phaseDisplay(phases []string) string {
	if len(phases) == 0 {
		return "N/A"
	}
	labels := make([]string, 0, len(phases))
	for _, p := range phases {
		labels = append(labels, phaseLabel(p))
	}
	return strings.Join(labels, "/")
}
