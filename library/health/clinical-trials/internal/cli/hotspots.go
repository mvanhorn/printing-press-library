// Copyright 2026 laci141 and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type hotspotsView struct {
	Query        string        `json:"query"`
	SampleSize   int           `json:"sample_size"`
	TopCountries []rankedEntry `json:"top_countries"`
	Note         string        `json:"note,omitempty"`
}

func newHotspotsCmd(flags *rootFlags) *cobra.Command {
	var sample, maxScanPages int
	cmd := &cobra.Command{
		Use:   "hotspots <condition>",
		Short: "Rank the countries where trials for a condition are concentrated.",
		Long: "Aggregates trial locations across a sampled set of trials for a condition and\n" +
			"ranks the countries with the most recruiting/active sites.",
		Example:     "  clinical-trials-pp-cli hotspots cancer --json\n  clinical-trials-pp-cli hotspots \"rare disease\" --sample 500",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would aggregate trial locations from ClinicalTrials.gov")
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
			trials, err := ctgovFetch(ctx, c, ctgovParams("cond", term), sample, maxScanPages)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			geo := newCounter()
			for _, t := range trials {
				for _, country := range t.Countries {
					geo.add(country)
				}
			}
			view := hotspotsView{Query: term, SampleSize: len(trials), TopCountries: geo.top(20)}
			if len(trials) == 0 {
				view.Note = "no trials matched; try a broader condition term"
			} else if len(view.TopCountries) == 0 {
				view.Note = "matched trials have no recorded locations in the sample"
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				if err := printJSONFiltered(cmd.OutOrStdout(), view, flags); err != nil {
					return err
				}
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Geographic hotspots — %s (sample %d)\n\n", term, view.SampleSize)
				for _, g := range view.TopCountries {
					fmt.Fprintf(cmd.OutOrStdout(), "  %-28s %d\n", g.Label, g.Count)
				}
				if view.Note != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "\nnote: %s\n", view.Note)
				}
			}
			if len(trials) == 0 {
				return noResultsErr(term)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&sample, "sample", 300, "Max trials to analyze")
	cmd.Flags().IntVar(&maxScanPages, "max-scan-pages", 2, "Max ClinicalTrials.gov pages to scan")
	return cmd
}
