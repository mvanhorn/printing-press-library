// Copyright 2026 laci141 and contributors. Licensed under Apache-2.0. See LICENSE.

// risk.go implements the `risk` command: an explainable, deterministic risk
// read on a single trial. It fetches one study by NCT id, then scores it across
// four observable signals — overall status / termination history, posted
// enrollment, geographic breadth, and the lead sponsor's track record — and
// returns each factor's contribution so the score is auditable, never a black box.
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// riskFactor is one scored contribution to the overall risk read.
type riskFactor struct {
	Name   string `json:"name"`
	Detail string `json:"detail"`
	Points int    `json:"points"`
}

// riskView is the JSON shape `risk` emits.
type riskView struct {
	NCTID         string       `json:"id"`
	Title         string       `json:"title,omitempty"`
	Status        string       `json:"status,omitempty"`
	Sponsor       string       `json:"sponsor,omitempty"`
	SponsorTrials int          `json:"sponsor_trials"`
	Score         int          `json:"score"`
	Level         string       `json:"level"`
	Factors       []riskFactor `json:"factors"`
	Note          string       `json:"note,omitempty"`
}

// pp:data-source live
func newNovelRiskCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "risk <nct-id>",
		Short: "Get an explainable risk read on a single trial from termination, enrollment, site count, sponsor track record, and phase factor (earlier phases = higher risk).",
		Long: "Score one trial's risk of failure across four observable signals — overall\n" +
			"status / termination history, posted enrollment, geographic breadth, and the\n" +
			"lead sponsor's track record. Each factor's point contribution is returned so\n" +
			"the 0-100 score is fully explainable rather than a black box.",
		Example: "  clinical-trials-pp-cli risk NCT07011732 --json\n" +
			"  clinical-trials-pp-cli risk NCT04280705",
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would score a single trial's risk from ClinicalTrials.gov")
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

			// Lead-sponsor track record: how many trials this sponsor has run.
			// A soft signal — failure here degrades the factor rather than the
			// whole command.
			sponsorTrials := 0
			if t.Sponsor != "" {
				if n, cErr := ctgovCount(ctx, c, ctgovParams("spons", t.Sponsor)); cErr == nil {
					sponsorTrials = n
				}
			}

			view := scoreRisk(t, sponsorTrials)
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			return renderRiskHuman(cmd, view)
		},
	}
	return cmd
}

// scoreRisk computes the explainable risk view. Points accumulate toward risk
// (0 = no observed risk signal); the total is capped at 100 and bucketed into
// low / moderate / high. Every branch records a factor so the score is auditable.
func scoreRisk(t Trial, sponsorTrials int) riskView {
	v := riskView{
		NCTID:         t.NCTID,
		Title:         t.Title,
		Status:        t.Status,
		Sponsor:       t.Sponsor,
		SponsorTrials: sponsorTrials,
	}

	// 1. Status / termination history.
	switch strings.ToUpper(t.Status) {
	case "TERMINATED", "WITHDRAWN", "SUSPENDED":
		detail := fmt.Sprintf("trial is %s", strings.ToLower(t.Status))
		if t.WhyStopped != "" {
			detail += ": " + truncate(t.WhyStopped, 120)
		}
		v.Factors = append(v.Factors, riskFactor{Name: "status", Detail: detail, Points: 50})
	case "COMPLETED":
		v.Factors = append(v.Factors, riskFactor{Name: "status", Detail: "trial already completed; low forward risk", Points: 0})
	case "":
		v.Factors = append(v.Factors, riskFactor{Name: "status", Detail: "overall status not posted", Points: 10})
	default:
		v.Factors = append(v.Factors, riskFactor{Name: "status", Detail: "trial " + strings.ToLower(strings.ReplaceAll(t.Status, "_", " ")), Points: 0})
	}

	// 2. Posted enrollment.
	switch {
	case t.Enrollment <= 0:
		v.Factors = append(v.Factors, riskFactor{Name: "enrollment", Detail: "enrollment target not posted", Points: 15})
	case t.Enrollment < 30:
		v.Factors = append(v.Factors, riskFactor{Name: "enrollment", Detail: fmt.Sprintf("small enrollment target (%d)", t.Enrollment), Points: 15})
	case t.Enrollment < 100:
		v.Factors = append(v.Factors, riskFactor{Name: "enrollment", Detail: fmt.Sprintf("modest enrollment target (%d)", t.Enrollment), Points: 8})
	default:
		v.Factors = append(v.Factors, riskFactor{Name: "enrollment", Detail: fmt.Sprintf("healthy enrollment target (%d)", t.Enrollment), Points: 0})
	}

	// 3. Geographic breadth (country count as a site-diversity proxy).
	switch n := len(t.Countries); {
	case n == 0:
		v.Factors = append(v.Factors, riskFactor{Name: "geographic_breadth", Detail: "no trial locations posted", Points: 15})
	case n == 1:
		v.Factors = append(v.Factors, riskFactor{Name: "geographic_breadth", Detail: fmt.Sprintf("single-country trial (%s)", t.Countries[0]), Points: 8})
	default:
		v.Factors = append(v.Factors, riskFactor{Name: "geographic_breadth", Detail: fmt.Sprintf("%d countries", n), Points: 0})
	}

	// 4. Lead-sponsor track record.
	switch {
	case t.Sponsor == "":
		v.Factors = append(v.Factors, riskFactor{Name: "sponsor_track_record", Detail: "lead sponsor not posted", Points: 15})
	case sponsorTrials <= 1:
		v.Factors = append(v.Factors, riskFactor{Name: "sponsor_track_record", Detail: "sponsor has little or no prior trial history", Points: 20})
	case sponsorTrials < 5:
		v.Factors = append(v.Factors, riskFactor{Name: "sponsor_track_record", Detail: fmt.Sprintf("sponsor has run %d trials (limited record)", sponsorTrials), Points: 10})
	default:
		v.Factors = append(v.Factors, riskFactor{Name: "sponsor_track_record", Detail: fmt.Sprintf("experienced sponsor (%d trials)", sponsorTrials), Points: 0})
	}

	// 5. Phase attrition (early-phase trials fail more often). Every branch
	// — including Phase 2 and unknown/unposted phase — records an explicit
	// factor so the score stays auditable; a missing entry should never be
	// the way a reviewer learns "no phase signal was applied."
	switch {
	case hasAnyPhase(t.Phases, "EARLY_PHASE1", "PHASE1"):
		v.Factors = append(v.Factors, riskFactor{Name: "phase", Detail: "early-phase trial (higher historical attrition)", Points: 10})
	case hasAnyPhase(t.Phases, "PHASE2"):
		v.Factors = append(v.Factors, riskFactor{Name: "phase", Detail: "phase 2 trial (most common attrition point)", Points: 5})
	case hasAnyPhase(t.Phases, "PHASE3", "PHASE4"):
		v.Factors = append(v.Factors, riskFactor{Name: "phase", Detail: "late-phase trial", Points: 0})
	default:
		v.Factors = append(v.Factors, riskFactor{Name: "phase", Detail: "trial phase not posted or not applicable", Points: 5})
	}

	score := 0
	for _, f := range v.Factors {
		score += f.Points
	}
	if score > 100 {
		score = 100
	}
	v.Score = score
	switch {
	case score < 25:
		v.Level = "low"
	case score < 55:
		v.Level = "moderate"
	default:
		v.Level = "high"
	}
	return v
}

// hasAnyPhase reports whether phases contains any of the given phase enums.
func hasAnyPhase(phases []string, want ...string) bool {
	for _, p := range phases {
		for _, w := range want {
			if p == w {
				return true
			}
		}
	}
	return false
}

func renderRiskHuman(cmd *cobra.Command, v riskView) error {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "%s  %s\n", v.NCTID, truncate(v.Title, 80))
	fmt.Fprintf(w, "  status=%s sponsor=%s\n\n", v.Status, truncate(v.Sponsor, 40))
	fmt.Fprintf(w, "Risk score: %d/100 (%s)\n\n", v.Score, strings.ToUpper(v.Level))
	fmt.Fprintln(w, "Contributing factors:")
	for _, f := range v.Factors {
		fmt.Fprintf(w, "  %-22s +%-3d %s\n", f.Name, f.Points, f.Detail)
	}
	if v.Note != "" {
		fmt.Fprintf(w, "\nnote: %s\n", v.Note)
	}
	return nil
}
