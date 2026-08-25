// Copyright 2026 laci141 and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type phase3View struct {
	Query           string        `json:"query"`
	ScannedTrials   int           `json:"scanned_trials"`
	MaxScanPages    int           `json:"max_scan_pages"`
	Matched         int           `json:"matched"`
	Returned        int           `json:"returned"`
	StatusBreakdown []rankedEntry `json:"status_breakdown,omitempty"`
	TopCountries    []rankedEntry `json:"top_countries,omitempty"`
	Trials          []Trial       `json:"trials"`
	Note            string        `json:"note,omitempty"`
}

func newPhase3Cmd(flags *rootFlags) *cobra.Command {
	var limit, maxScanPages int
	cmd := &cobra.Command{
		Use:   "phase3 <condition>",
		Short: "Find Phase 3 trials for a condition by scanning and filtering locally.",
		Long: "ClinicalTrials.gov has no direct phase filter, so this command scans the\n" +
			"matching trials for a condition and keeps those in Phase 3 (including Phase 2/3).\n" +
			"Use --max-scan-pages to widen the scan and --limit to bound returned matches.",
		Example:     "  clinical-trials-pp-cli phase3 diabetes --json\n  clinical-trials-pp-cli phase3 \"breast cancer\" --max-scan-pages 5",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would scan ClinicalTrials.gov for Phase 3 trials")
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
			// Scan a broad set and filter locally for Phase 3.
			scanned, err := ctgovFetch(ctx, c, ctgovParams("cond", term), maxScanPages*200, maxScanPages)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			status := newCounter()
			geo := newCounter()
			matched := make([]Trial, 0, limit)
			for _, t := range scanned {
				if !hasPhase3(t.Phases) {
					continue
				}
				status.add(t.Status)
				for _, country := range t.Countries {
					geo.add(country)
				}
				if len(matched) < limit {
					matched = append(matched, t)
				}
			}
			matchedTotal := status.total()
			view := phase3View{
				Query:           term,
				ScannedTrials:   len(scanned),
				MaxScanPages:    maxScanPages,
				Matched:         matchedTotal,
				Returned:        len(matched),
				StatusBreakdown: status.top(8),
				TopCountries:    geo.top(10),
				Trials:          matched,
			}
			if matchedTotal == 0 {
				view.Note = fmt.Sprintf("scanned %d trials across up to %d pages without finding a Phase 3 trial; raise --max-scan-pages to widen the search", len(scanned), maxScanPages)
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				if err := printJSONFiltered(cmd.OutOrStdout(), view, flags); err != nil {
					return err
				}
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "%s — %d Phase 3 trials matched (scanned %d), showing %d\n\n", term, matchedTotal, len(scanned), len(matched))
				for _, t := range matched {
					fmt.Fprintf(cmd.OutOrStdout(), "%s  %s\n    status=%s phase=%s\n", t.NCTID, truncate(t.Title, 80), t.Status, phaseDisplay(t.Phases))
				}
				if view.Note != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "\nnote: %s\n", view.Note)
				}
			}
			if matchedTotal == 0 {
				return noResultsErr(term)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "Max matching trials to return")
	cmd.Flags().IntVar(&maxScanPages, "max-scan-pages", 3, "Max ClinicalTrials.gov pages (200 trials each) to scan")
	return cmd
}

func hasPhase3(phases []string) bool {
	for _, p := range phases {
		if p == "PHASE3" {
			return true
		}
	}
	return false
}
