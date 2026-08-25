// Copyright 2026 laci141 and contributors. Licensed under Apache-2.0. See LICENSE.

// forecast.go implements the `forecast` command: a forward-looking completion
// outlook derived from the same deterministic scoring logic as `risk`. It
// reframes the 0-100 risk score as a descriptive outlook band and surfaces the
// top risk-driving factors as actionable "primary concerns".
//
// IMPORTANT: the outlook percentages and bands are heuristics derived from
// observable ClinicalTrials.gov registry signals. They are NOT a clinical or
// statistical prediction of trial outcome. Do not use this output as a medical,
// financial, or regulatory determination.
package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// forecastOutlook holds the interpreted completion outlook for a trial.
type forecastOutlook struct {
	Band            string `json:"band"`             // "favorable" | "moderate" | "guarded" | "high-concern"
	DescriptiveRange string `json:"descriptive_range"` // e.g. "75–85%"
	Caveat          string `json:"caveat"`
}

// forecastView is the JSON shape `forecast` emits.
type forecastView struct {
	NCTID           string        `json:"id"`
	Title           string        `json:"title,omitempty"`
	Status          string        `json:"status,omitempty"`
	Sponsor         string        `json:"sponsor,omitempty"`
	Score           int           `json:"score"`
	Outlook         forecastOutlook `json:"outlook"`
	PrimaryConcerns []riskFactor  `json:"primary_concerns"`
	AllFactors      []riskFactor  `json:"all_factors"`
}

// outlookFromScore converts a 0-100 risk score into a completion-outlook band.
// Percentages are descriptive heuristic ranges, not statistical predictions.
func outlookFromScore(score int) forecastOutlook {
	const caveat = "Heuristic outlook derived from observable registry signals only — not a clinical or statistical prediction of trial outcome."
	switch {
	case score < 25:
		return forecastOutlook{Band: "favorable", DescriptiveRange: "75–85%", Caveat: caveat}
	case score < 55:
		return forecastOutlook{Band: "moderate", DescriptiveRange: "50–70%", Caveat: caveat}
	case score < 80:
		return forecastOutlook{Band: "guarded", DescriptiveRange: "30–50%", Caveat: caveat}
	default:
		return forecastOutlook{Band: "high-concern", DescriptiveRange: "15–30%", Caveat: caveat}
	}
}

// primaryConcerns returns the top n factors by points descending, keeping only
// those with points > 0 (zero-point factors contribute no observed risk signal).
func primaryConcerns(factors []riskFactor, n int) []riskFactor {
	var nonzero []riskFactor
	for _, f := range factors {
		if f.Points > 0 {
			nonzero = append(nonzero, f)
		}
	}
	sort.Slice(nonzero, func(i, j int) bool {
		return nonzero[i].Points > nonzero[j].Points
	})
	if len(nonzero) > n {
		nonzero = nonzero[:n]
	}
	return nonzero
}

// pp:data-source live
func newNovelForecastCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "forecast <nct-id>",
		Short: "Forward-looking completion outlook for a trial, derived from the same deterministic signals as `risk` (heuristic only — not a clinical prediction).",
		Long: "Reuses the same observable-signal scoring as `risk` (status, enrollment,\n" +
			"geographic breadth, sponsor track record, phase) and reframes the result as\n" +
			"a forward-looking completion outlook: a descriptive band (favorable / moderate /\n" +
			"guarded / high-concern), a heuristic percentage range, and the top risk-driving\n" +
			"factors as primary concerns so the output is actionable.\n\n" +
			"CAVEAT: outlook bands are heuristics derived from observable ClinicalTrials.gov\n" +
			"registry signals. They are NOT a clinical or statistical prediction of trial\n" +
			"outcome and must not be used as a medical, financial, or regulatory determination.",
		Example: "  clinical-trials-pp-cli forecast NCT07011732 --json\n" +
			"  clinical-trials-pp-cli forecast NCT04280705",
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would forecast completion outlook for a single trial from ClinicalTrials.gov")
				return nil
			}
			nctID := strings.TrimSpace(args[0])
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			t, ok, err := ctgovFetchOne(ctx, c, nctID)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if !ok {
				return usageErr(fmt.Errorf("no trial found for %q (check the NCT id)", nctID))
			}

			sponsorTrials := 0
			if t.Sponsor != "" {
				if n, cErr := ctgovCount(ctx, c, ctgovParams("spons", t.Sponsor)); cErr == nil {
					sponsorTrials = n
				}
			}

			// Reuse scoreRisk directly — all factor logic lives there.
			risk := scoreRisk(t, sponsorTrials)
			view := forecastView{
				NCTID:           risk.NCTID,
				Title:           risk.Title,
				Status:          risk.Status,
				Sponsor:         risk.Sponsor,
				Score:           risk.Score,
				Outlook:         outlookFromScore(risk.Score),
				PrimaryConcerns: primaryConcerns(risk.Factors, 3),
				AllFactors:      risk.Factors,
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			return renderForecastHuman(cmd, view)
		},
	}
	return cmd
}

func renderForecastHuman(cmd *cobra.Command, v forecastView) error {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "%s  %s\n", v.NCTID, truncate(v.Title, 80))
	fmt.Fprintf(w, "  status=%s sponsor=%s\n\n", v.Status, truncate(v.Sponsor, 40))
	fmt.Fprintf(w, "Completion outlook: %s (%s descriptive range)\n", strings.ToUpper(v.Outlook.Band), v.Outlook.DescriptiveRange)
	fmt.Fprintf(w, "Risk score:         %d/100\n\n", v.Score)
	if len(v.PrimaryConcerns) > 0 {
		fmt.Fprintln(w, "Primary concerns (top risk drivers):")
		for _, f := range v.PrimaryConcerns {
			fmt.Fprintf(w, "  %-22s +%-3d %s\n", f.Name, f.Points, f.Detail)
		}
		fmt.Fprintln(w)
	}
	fmt.Fprintln(w, "All factors:")
	for _, f := range v.AllFactors {
		fmt.Fprintf(w, "  %-22s +%-3d %s\n", f.Name, f.Points, f.Detail)
	}
	fmt.Fprintf(w, "\nnote: %s\n", v.Outlook.Caveat)
	return nil
}
