// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.
// pp:novel-static-reference: CVSS 3.1 base-score arithmetic is a fixed FIRST.org formula
// (https://www.first.org/cvss/v3.1/specification-document); rule-based, no API call.
// PATCH: Add deterministic CVSS 3.1 base-score validation for report drafts.

package cli

import (
	"fmt"
	"math"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func newReportCVSSCheckCmd(flags *rootFlags) *cobra.Command {
	var stepsPath string
	cmd := &cobra.Command{
		Use:     "cvss-check <vector>",
		Short:   "Parse a CVSS 3.1 vector and flag contradictions in reproduction steps",
		Example: "  yeswehack-pp-cli report cvss-check CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H\n  yeswehack-pp-cli report cvss-check CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H --steps repro.md",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			metrics, score, err := parseCVSS31(args[0])
			if err != nil {
				return usageErr(err)
			}
			var contradictions []string
			if stepsPath != "" {
				data, err := os.ReadFile(stepsPath)
				if err != nil {
					return err
				}
				steps := strings.ToLower(string(data))
				if metrics["AV"] == "N" && (strings.Contains(steps, "no remote access") || strings.Contains(steps, "physical access required")) {
					contradictions = append(contradictions, "AV:N contradicts steps that state no remote access or physical access required")
				}
				if metrics["UI"] == "N" && (strings.Contains(steps, "user clicks") || strings.Contains(steps, "user opens") || strings.Contains(steps, "user navigates to")) {
					contradictions = append(contradictions, "UI:N contradicts steps requiring user clicks, opens, or navigation")
				}
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"parsed_metrics":   metrics,
				"base_score":       score,
				"severity_label":   cvssSeverity(score),
				"contradictions":   contradictions,
				"contradict_count": len(contradictions),
			}, flags)
		},
	}
	cmd.Flags().StringVar(&stepsPath, "steps", "", "Optional file containing reproduction steps to scan for contradictions")
	return cmd
}

func parseCVSS31(vector string) (map[string]string, float64, error) {
	parts := strings.Split(vector, "/")
	if len(parts) < 9 || parts[0] != "CVSS:3.1" {
		return nil, 0, fmt.Errorf("expected CVSS:3.1 vector")
	}
	metrics := map[string]string{}
	for _, part := range parts[1:] {
		kv := strings.SplitN(part, ":", 2)
		if len(kv) != 2 {
			return nil, 0, fmt.Errorf("invalid metric %q", part)
		}
		metrics[kv[0]] = kv[1]
	}
	av := map[string]float64{"N": 0.85, "A": 0.62, "L": 0.55, "P": 0.2}[metrics["AV"]]
	ac := map[string]float64{"L": 0.77, "H": 0.44}[metrics["AC"]]
	ui := map[string]float64{"N": 0.85, "R": 0.62}[metrics["UI"]]
	scope := metrics["S"]
	prU := map[string]float64{"N": 0.85, "L": 0.62, "H": 0.27}
	prC := map[string]float64{"N": 0.85, "L": 0.68, "H": 0.5}
	pr := prU[metrics["PR"]]
	if scope == "C" {
		pr = prC[metrics["PR"]]
	}
	// PATCH(cvss-validate-cia): "N" is a valid CIA value that maps to 0, so the
	// av/ac/ui/pr zero-check below cannot validate C/I/A. Without a separate
	// validity check, a vector like .../C:GARBAGE/I:H/A:H scores as if C:N
	// (lower than reality) with no error. Greptile P1 on PR #459.
	impactMetric := map[string]float64{"N": 0, "L": 0.22, "H": 0.56}
	validImpact := map[string]bool{"N": true, "L": true, "H": true}
	if !validImpact[metrics["C"]] || !validImpact[metrics["I"]] || !validImpact[metrics["A"]] {
		return nil, 0, fmt.Errorf("missing or invalid required CVSS metric")
	}
	c, i, a := impactMetric[metrics["C"]], impactMetric[metrics["I"]], impactMetric[metrics["A"]]
	if av == 0 || ac == 0 || ui == 0 || pr == 0 || (scope != "U" && scope != "C") {
		return nil, 0, fmt.Errorf("missing or invalid required CVSS metric")
	}
	iss := 1 - (1-c)*(1-i)*(1-a)
	var impact float64
	if scope == "U" {
		impact = 6.42 * iss
	} else {
		impact = 7.52*(iss-0.029) - 3.25*math.Pow(iss-0.02, 15)
	}
	exploitability := 8.22 * av * ac * pr * ui
	var base float64
	if impact <= 0 {
		base = 0
	} else if scope == "U" {
		base = roundUpCVSS(math.Min(impact+exploitability, 10))
	} else {
		base = roundUpCVSS(math.Min(1.08*(impact+exploitability), 10))
	}
	return metrics, base, nil
}

func roundUpCVSS(x float64) float64 {
	return math.Ceil(x*10) / 10
}

func cvssSeverity(score float64) string {
	switch {
	case score == 0:
		return "None"
	case score < 4:
		return "Low"
	case score < 7:
		return "Medium"
	case score < 9:
		return "High"
	default:
		return "Critical"
	}
}
