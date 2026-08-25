// Copyright 2026 laci141 and contributors. Licensed under Apache-2.0. See LICENSE.

// enrollment_check.go implements the `enrollment-check` command: a heuristic
// plausibility check on a trial's posted enrollment target relative to its
// phase. It reuses scoreRisk to obtain the enrollment factor rather than
// duplicating that logic here.
//
// IMPORTANT: the verdict is a heuristic derived from observable registry
// signals only. It is NOT a clinical judgment or a prediction of whether a
// trial will successfully enroll.
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// enrollmentVerdict bundles the plausibility verdict with its reasoning.
type enrollmentVerdict struct {
	Target     int    `json:"target"`
	Phase      string `json:"phase,omitempty"`
	Verdict    string `json:"verdict"`  // "not_posted" | "modest" | "typical" | "ambitious"
	Label      string `json:"label"`    // human-readable one-liner
	RiskPoints int    `json:"risk_signal_points"`
	Caveat     string `json:"caveat"`
}

// enrollmentCheckView is the JSON shape `enrollment-check` emits.
type enrollmentCheckView struct {
	NCTID   string            `json:"id"`
	Title   string            `json:"title,omitempty"`
	Status  string            `json:"status,omitempty"`
	Sponsor string            `json:"sponsor,omitempty"`
	Check   enrollmentVerdict `json:"enrollment_check"`
}

// enrollmentVerdictFromRisk derives a plausibility verdict by reusing the
// enrollment riskFactor that scoreRisk already computed. The factor's points
// and detail encode the same thresholds — we read them back rather than
// re-implementing the switch.
func enrollmentVerdictFromRisk(v riskView, phase string) enrollmentVerdict {
	const caveat = "Heuristic plausibility check from observable registry signals only — not a clinical judgment or enrollment prediction."

	// Find the enrollment factor scoreRisk already computed.
	var enrollFactor riskFactor
	for _, f := range v.Factors {
		if f.Name == "enrollment" {
			enrollFactor = f
			break
		}
	}

	// phaseContext produces a short phrase for use in verdict labels, reusing
	// the existing phaseLabel helper from intel.go (takes a plain string).
	phaseContext := phaseLabel(phase)
	if phaseContext == "" {
		phaseContext = "this trial"
	}

	// Map the risk-factor points back to a plausibility verdict.
	// Mirrors the scoreRisk enrollment switch:
	//   <=0 or <30  → 15 pts  (not posted / small)
	//   <100        →  8 pts  (modest)
	//   >=100       →  0 pts  (typical)
	verdict, label := "", ""
	switch enrollFactor.Points {
	case 0:
		verdict = "typical"
		label = fmt.Sprintf("enrollment target looks typical for %s", phaseContext)
	case 8:
		verdict = "modest"
		label = fmt.Sprintf("enrollment target is modest for %s (may be appropriate for early-phase or rare-disease trials)", phaseContext)
	default: // 15 = not posted or very small
		if strings.Contains(enrollFactor.Detail, "not posted") {
			verdict = "not_posted"
			label = "enrollment target not posted — plausibility cannot be assessed"
		} else {
			verdict = "modest"
			label = fmt.Sprintf("enrollment target is small for %s (common in early-phase or feasibility trials)", phaseContext)
		}
	}

	return enrollmentVerdict{
		Verdict:    verdict,
		Label:      label,
		RiskPoints: enrollFactor.Points,
		Caveat:     caveat,
	}
}

// pp:data-source live
func newNovelEnrollmentCheckCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "enrollment-check <nct-id>",
		Short: "Heuristic plausibility check on a trial's posted enrollment target relative to its phase (registry signals only — not a clinical judgment).",
		Long: "Fetches a single trial and assesses whether its posted enrollment target\n" +
			"looks modest, typical, or ambitious given the trial's phase — reusing the\n" +
			"same enrollment-signal logic as the `risk` command.\n\n" +
			"CAVEAT: the verdict is a heuristic derived from observable ClinicalTrials.gov\n" +
			"registry signals. It is NOT a clinical judgment or a prediction of whether a\n" +
			"trial will successfully enroll.",
		Example: "  clinical-trials-pp-cli enrollment-check NCT04280705\n" +
			"  clinical-trials-pp-cli enrollment-check NCT04280705 --json",
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would check enrollment plausibility from ClinicalTrials.gov")
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

			// Reuse scoreRisk — all enrollment-factor logic lives there.
			risk := scoreRisk(t, sponsorTrials)

			// Build the verdict, then fill in the raw values scoreRisk doesn't re-expose.
			check := enrollmentVerdictFromRisk(risk, t.Phase)
			check.Target = t.Enrollment
			check.Phase = t.Phase

			view := enrollmentCheckView{
				NCTID:   t.NCTID,
				Title:   t.Title,
				Status:  t.Status,
				Sponsor: t.Sponsor,
				Check:   check,
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			return renderEnrollmentCheckHuman(cmd, view)
		},
	}
	return cmd
}

func renderEnrollmentCheckHuman(cmd *cobra.Command, v enrollmentCheckView) error {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "%s  %s\n", v.NCTID, truncate(v.Title, 80))
	fmt.Fprintf(w, "  status=%s phase=%s sponsor=%s\n\n", v.Status, v.Check.Phase, truncate(v.Sponsor, 40))

	targetStr := fmt.Sprintf("%d", v.Check.Target)
	if v.Check.Target <= 0 {
		targetStr = "not posted"
	}
	fmt.Fprintf(w, "Enrollment target:  %s\n", targetStr)
	fmt.Fprintf(w, "Verdict:            %s\n", strings.ToUpper(v.Check.Verdict))
	fmt.Fprintf(w, "Assessment:         %s\n", v.Check.Label)
	fmt.Fprintf(w, "Risk signal points: %d\n", v.Check.RiskPoints)
	fmt.Fprintf(w, "\nnote: %s\n", v.Check.Caveat)
	return nil
}
