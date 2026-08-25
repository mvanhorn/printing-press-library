// Copyright 2026 laci141 and contributors. Licensed under Apache-2.0. See LICENSE.

// sponsors.go implements the `sponsors` command: who runs the most trials in a
// disease area, ranked, with each lead sponsor classified as industry,
// government, or academic/other from the CT.gov LeadSponsorClass enum.
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type sponsorEntry struct {
	Name  string `json:"name"`
	Class string `json:"class"`
	Count int    `json:"count"`
}

type sponsorsView struct {
	Query             string         `json:"query"`
	SampleSize        int            `json:"sample_size"`
	ClassDistribution []rankedEntry  `json:"class_distribution,omitempty"`
	TopSponsors       []sponsorEntry `json:"top_sponsors,omitempty"`
	Note              string         `json:"note,omitempty"`
}

// pp:data-source live
func newNovelSponsorsCmd(flags *rootFlags) *cobra.Command {
	var limit, sample, maxScanPages int
	cmd := &cobra.Command{
		Use:   "sponsors <area>",
		Short: "Rank who runs the most trials in an area, classified as industry, academic, or government.",
		Long: "Aggregate the lead sponsors across a sampled set of trials for a disease area\n" +
			"and rank them by trial volume, classifying each as industry, government, or\n" +
			"academic/other from the ClinicalTrials.gov sponsor class.",
		Example: "  clinical-trials-pp-cli sponsors diabetes --json\n" +
			"  clinical-trials-pp-cli sponsors \"breast cancer\" --limit 25",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would rank trial sponsors from ClinicalTrials.gov")
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

			class := newCounter()
			sponsorCount := newCounter()
			sponsorClass := map[string]string{}
			for _, t := range trials {
				if t.Sponsor == "" {
					continue
				}
				sponsorCount.add(t.Sponsor)
				cl := sponsorClassLabel(t.SponsorClass)
				class.add(cl)
				sponsorClass[t.Sponsor] = cl
			}

			ranked := sponsorCount.top(limit)
			top := make([]sponsorEntry, 0, len(ranked))
			for _, r := range ranked {
				top = append(top, sponsorEntry{Name: r.Label, Class: sponsorClass[r.Label], Count: r.Count})
			}

			view := sponsorsView{
				Query:             term,
				SampleSize:        len(trials),
				ClassDistribution: class.top(0),
				TopSponsors:       top,
			}
			if len(trials) == 0 {
				view.Note = "no trials matched; try a broader area term"
			} else if len(top) == 0 {
				view.Note = "matched trials have no recorded lead sponsor in the sample"
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				if err := printJSONFiltered(cmd.OutOrStdout(), view, flags); err != nil {
					return err
				}
			} else if err := renderSponsorsHuman(cmd, view); err != nil {
				return err
			}
			if len(trials) == 0 {
				return noResultsErr(term)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "Max sponsors to list")
	cmd.Flags().IntVar(&sample, "sample", 300, "Max trials to analyze")
	cmd.Flags().IntVar(&maxScanPages, "max-scan-pages", 2, "Max ClinicalTrials.gov pages to scan")
	return cmd
}

// sponsorClassLabel maps a CT.gov LeadSponsorClass enum to a coarse bucket.
func sponsorClassLabel(class string) string {
	switch strings.ToUpper(class) {
	case "INDUSTRY":
		return "industry"
	case "NIH", "FED", "OTHER_GOV":
		return "government"
	case "OTHER", "NETWORK", "INDIV":
		return "academic/other"
	default:
		return "unknown"
	}
}

func renderSponsorsHuman(cmd *cobra.Command, v sponsorsView) error {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "Top sponsors — %s (sample %d)\n\n", v.Query, v.SampleSize)
	if len(v.ClassDistribution) > 0 {
		fmt.Fprintln(w, "By sponsor class:")
		for _, c := range v.ClassDistribution {
			fmt.Fprintf(w, "  %-16s %d\n", c.Label, c.Count)
		}
		fmt.Fprintln(w)
	}
	for _, s := range v.TopSponsors {
		fmt.Fprintf(w, "  %-44s %-15s %d\n", truncate(s.Name, 44), s.Class, s.Count)
	}
	if v.Note != "" {
		fmt.Fprintf(w, "\nnote: %s\n", v.Note)
	}
	return nil
}
