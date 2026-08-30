// Copyright 2026 laci141 and contributors. Licensed under Apache-2.0. See LICENSE.

// report.go implements the `report` command: a single briefing for a topic that
// composes the same primitives the standalone intelligence commands use —
// headline counts (search), phase mix (emerging), geographic concentration
// (hotspots), and the leading sponsors — rendered as Markdown or CSV.
package cli

import (
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// pp:data-source live
func newNovelReportCmd(flags *rootFlags) *cobra.Command {
	var flagFormat string
	var sample, maxScanPages, topN int
	cmd := &cobra.Command{
		Use:   "report <topic>",
		Short: "Compose search, emerging, hotspots, and sponsors into one markdown or CSV briefing for a topic.",
		Long: "Produce a one-shot briefing for a topic by composing the building blocks of the\n" +
			"other intelligence commands: total and recruiting counts, phase distribution,\n" +
			"top countries, and leading sponsors. Render it as Markdown (default) or CSV.",
		Example: "  clinical-trials-pp-cli report \"long covid\" --format md\n" +
			"  clinical-trials-pp-cli report diabetes --format csv",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would compose a topic briefing from ClinicalTrials.gov")
				return nil
			}
			format := strings.ToLower(strings.TrimSpace(flagFormat))
			if format == "" {
				format = "md"
			}
			if format != "md" && format != "markdown" && format != "csv" {
				return usageErr(fmt.Errorf("--format must be one of: md, csv"))
			}
			term := strings.TrimSpace(strings.Join(args, " "))
			if term == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a topic argument is required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			total, err := ctgovCount(ctx, c, ctgovParams("cond", term))
			if err != nil {
				return classifyAPIError(err, flags)
			}
			recParams := ctgovParams("cond", term)
			recParams["filter.overallStatus"] = "RECRUITING"
			recruiting, err := ctgovCount(ctx, c, recParams)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			trials, err := ctgovFetch(ctx, c, ctgovParams("cond", term), sample, maxScanPages)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			phase := newCounter()
			geo := newCounter()
			sponsor := newCounter()
			for _, t := range trials {
				for _, ph := range t.Phases {
					phase.add(phaseLabel(ph))
				}
				for _, country := range t.Countries {
					geo.add(country)
				}
				sponsor.add(t.Sponsor)
			}

			rep := topicReport{
				Topic:         term,
				Total:         total,
				Recruiting:    recruiting,
				RecruitingPct: percentOf(recruiting, total),
				SampleSize:    len(trials),
				Phases:        phase.top(8),
				Countries:     geo.top(topN),
				Sponsors:      sponsor.top(topN),
			}
			if format == "csv" {
				if err := renderReportCSV(cmd, rep); err != nil {
					return err
				}
			} else if err := renderReportMarkdown(cmd, rep); err != nil {
				return err
			}
			if total == 0 && len(trials) == 0 {
				return noResultsErr(term)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagFormat, "format", "md", "Output format: md (Markdown) or csv")
	cmd.Flags().IntVar(&sample, "sample", 400, "Max trials to analyze")
	cmd.Flags().IntVar(&maxScanPages, "max-scan-pages", 3, "Max ClinicalTrials.gov pages to scan")
	cmd.Flags().IntVar(&topN, "top", 10, "Number of countries/sponsors to list")
	return cmd
}

type topicReport struct {
	Topic         string
	Total         int
	Recruiting    int
	RecruitingPct int
	SampleSize    int
	Phases        []rankedEntry
	Countries     []rankedEntry
	Sponsors      []rankedEntry
}

func renderReportMarkdown(cmd *cobra.Command, r topicReport) error {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "# Clinical Trials Briefing: %s\n\n", r.Topic)
	fmt.Fprintf(w, "- **Total trials:** %d\n", r.Total)
	fmt.Fprintf(w, "- **Actively recruiting:** %d (%d%%)\n", r.Recruiting, r.RecruitingPct)
	fmt.Fprintf(w, "- **Sample analyzed:** %d\n\n", r.SampleSize)

	writeMDTable(w, "Phase distribution", "Phase", r.Phases)
	writeMDTable(w, "Top countries", "Country", r.Countries)
	writeMDTable(w, "Leading sponsors", "Sponsor", r.Sponsors)
	return nil
}

func writeMDTable(w interface{ Write([]byte) (int, error) }, heading, keyCol string, rows []rankedEntry) {
	if len(rows) == 0 {
		return
	}
	fmt.Fprintf(w, "## %s\n\n", heading)
	fmt.Fprintf(w, "| %s | Count |\n|---|---|\n", keyCol)
	for _, r := range rows {
		fmt.Fprintf(w, "| %s | %d |\n", r.Label, r.Count)
	}
	fmt.Fprintln(w)
}

func renderReportCSV(cmd *cobra.Command, r topicReport) error {
	cw := csv.NewWriter(cmd.OutOrStdout())
	rows := [][]string{
		{"section", "label", "count"},
		{"summary", "total_trials", strconv.Itoa(r.Total)},
		{"summary", "recruiting", strconv.Itoa(r.Recruiting)},
		{"summary", "recruiting_pct", strconv.Itoa(r.RecruitingPct)},
		{"summary", "sample_size", strconv.Itoa(r.SampleSize)},
	}
	appendSection := func(section string, entries []rankedEntry) {
		for _, e := range entries {
			rows = append(rows, []string{section, e.Label, strconv.Itoa(e.Count)})
		}
	}
	appendSection("phase", r.Phases)
	appendSection("country", r.Countries)
	appendSection("sponsor", r.Sponsors)
	for _, row := range rows {
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}
