// Copyright 2026 laci141 and contributors. Licensed under Apache-2.0. See LICENSE.

// risk.go implements the `risk` command: an explainable, deterministic risk
// read on a single trial. It fetches one study by NCT id, then scores it across
// six observable signals — overall status / termination history, posted
// enrollment, geographic breadth, the lead sponsor's track record, trial
// phase, and whether an ongoing trial has passed its own estimated primary
// completion date — and returns each factor's contribution so the score is
// auditable, never a black box.
package cli

import (
	"fmt"
	"strings"
	"time"

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
		Long: "Score one trial's risk of failure across six observable signals — overall\n" +
			"status / termination history, posted enrollment, geographic breadth, the\n" +
			"lead sponsor's track record, trial phase, and an ongoing trial that has\n" +
			"passed its own estimated primary completion date. Each factor's point\n" +
			"contribution is returned so the 0-100 score is fully explainable rather\n" +
			"than a black box.",
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
	case "UNKNOWN":
		// The registry assigns UNKNOWN when a trial was last posted as
		// recruiting and has passed its own estimated completion date by more
		// than two years with no sponsor update. That is a different absence
		// from the "" case below: there a status was never posted, here one was
		// posted and has since gone stale, so the trial's present state cannot
		// be verified from the record. Scored at the same 10 points because
		// both describe an unknown current state; a higher figure would assert
		// that stale trials terminate more often than unposted ones, which is
		// not measured. Without this case UNKNOWN reached the default arm and
		// scored 0 — less than a status that was never posted at all, which
		// contradicts this table's own treatment of missing information.
		v.Factors = append(v.Factors, riskFactor{Name: "status", Detail: "status not updated for over two years; current state unverifiable", Points: 10})
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

	// 6. Schedule: an ongoing trial past its own estimated primary completion.
	v.Factors = append(v.Factors, scheduleFactor(t, time.Now()))

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

// ongoingStatuses are the overall statuses under which a trial is still
// expected to reach its primary completion. Terminal statuses (COMPLETED,
// TERMINATED, WITHDRAWN), SUSPENDED and UNKNOWN are scored by the status factor
// already; checking their dates here would count the same staleness twice.
var ongoingStatuses = map[string]bool{
	"NOT_YET_RECRUITING":      true,
	"RECRUITING":              true,
	"ENROLLING_BY_INVITATION": true,
	"ACTIVE_NOT_RECRUITING":   true,
}

// scheduleOverduePoints is what an ongoing trial scores once its ESTIMATED
// primary completion date has passed. It stays below the 10 points the status
// factor gives UNKNOWN: the registry assigns UNKNOWN only after a trial is more
// than two years past its completion date with no update, so a trial that is
// still listed as ongoing is at most that stale. A figure at or above 10 would
// rank a shorter delay as riskier than a longer one.
const scheduleOverduePoints = 5

// scheduleFactor reports whether an ongoing trial has passed its own estimated
// primary completion date. Only an ESTIMATED date that has passed scores: an
// ACTUAL date means the primary outcome data are in (ACTIVE_NOT_RECRUITING is
// then the normal state while follow-up continues), and a date with no type
// posted cannot be told apart from an actual one. Every branch returns a factor
// so the schedule signal is always visible in the output. now is a parameter so
// the date rule is testable without a clock.
func scheduleFactor(t Trial, now time.Time) riskFactor {
	f := riskFactor{Name: "schedule"}
	status := strings.ToUpper(t.Status)
	date := strings.TrimSpace(t.PrimaryCompletionDate)
	dateType := strings.ToUpper(strings.TrimSpace(t.PrimaryCompletionType))
	if !ongoingStatuses[status] {
		f.Detail = "schedule check applies only to ongoing trials"
		return f
	}
	if date == "" {
		f.Detail = "primary completion date not posted"
		return f
	}
	if dateType == "ACTUAL" {
		f.Detail = "primary completion reached on " + date
		return f
	}
	deadline, ok := partialDateEnd(date)
	if !ok {
		f.Detail = fmt.Sprintf("primary completion date %q not parseable", truncate(date, 20))
		return f
	}
	if now.Before(deadline) {
		f.Detail = "primary completion expected " + date
		return f
	}
	if dateType != "ESTIMATED" {
		f.Detail = "primary completion date " + date + " has passed; date type not posted"
		return f
	}
	months := monthsBetween(deadline, now)
	f.Detail = fmt.Sprintf("estimated primary completion %s passed %d month(s) ago; trial still listed as %s", date, months, strings.ToLower(strings.ReplaceAll(status, "_", " ")))
	f.Points = scheduleOverduePoints
	return f
}

// partialDateEnd returns the first instant after the period a CT.gov partial
// date names: the next day for YYYY-MM-DD, the first of the next month for
// YYYY-MM, and January 1 of the next year for YYYY. A date is overdue only
// once its whole period has ended, so "2025-11" is not late during November.
func partialDateEnd(date string) (time.Time, bool) {
	if d, err := time.Parse("2006-01-02", date); err == nil {
		return d.AddDate(0, 0, 1), true
	}
	if d, err := time.Parse("2006-01", date); err == nil {
		return d.AddDate(0, 1, 0), true
	}
	if d, err := time.Parse("2006", date); err == nil {
		return d.AddDate(1, 0, 0), true
	}
	return time.Time{}, false
}

// monthsBetween counts whole calendar months from start to end (0 when end is
// less than a month after start).
func monthsBetween(start, end time.Time) int {
	m := (end.Year()-start.Year())*12 + int(end.Month()) - int(start.Month())
	if end.Day() < start.Day() {
		m--
	}
	if m < 0 {
		return 0
	}
	return m
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
