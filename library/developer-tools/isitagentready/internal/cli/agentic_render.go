// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/isitagentready/internal/store"
)

// validAgenticTiers are the only --category/--tier values that make sense for
// the is-agentic source. is-agentic has two tiers (essential, recommended),
// not isitagentready's five categories.
var validAgenticTiers = map[string]string{
	"essential":   "essential",
	"recommended": "recommended",
}

// filterAgenticReport narrows a raw is-agentic report's issues[] by optional
// tier, failing-only and check id while preserving every other top-level key
// (score, score_breakdown, scanned_at, ...) so machine output stays complete
// and honest. When no filter applies, the raw report is returned unchanged.
func filterAgenticReport(raw json.RawMessage, tier string, onlyFailing bool, checkID string) (json.RawMessage, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		return nil, fmt.Errorf("parsing is-agentic report: %w", err)
	}
	issuesRaw, ok := top["issues"]
	if !ok || string(issuesRaw) == "null" {
		return raw, nil
	}
	var issues []store.AgenticIssue
	if err := json.Unmarshal(issuesRaw, &issues); err != nil {
		return raw, nil
	}
	filtered := make([]store.AgenticIssue, 0, len(issues))
	for _, iss := range issues {
		if tier != "" && iss.Tier != tier {
			continue
		}
		if checkID != "" && iss.ID != checkID {
			continue
		}
		if onlyFailing && iss.Result == "pass" {
			continue
		}
		filtered = append(filtered, iss)
	}
	b, err := json.Marshal(filtered)
	if err != nil {
		return nil, err
	}
	top["issues"] = b
	out, err := json.Marshal(top)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// renderAgenticReport prints the is-agentic report natively for a terminal:
// score / score_label / eligible_checks, the essential and recommended tier
// lines (passing/total, earned/available), bonus points, then the non-passing
// issues grouped by tier. scanned_at is always shown because these reports are
// cached upstream and can be days old. The issues[] is already filtered by the
// caller; tier only controls which tier headline lines (and section headers)
// are shown.
func renderAgenticReport(cmd *cobra.Command, raw json.RawMessage, tier string) error {
	out := cmd.OutOrStdout()
	rep, err := store.ParseAgenticReport(raw)
	if err != nil {
		return printOutput(cmd.OutOrStdout(), raw, false)
	}
	fmt.Fprintln(out, bold(rep.Target))
	if rep.ScannedAt != "" {
		fmt.Fprintf(out, "  scanned %s\n", rep.ScannedAt)
	}
	fmt.Fprintf(out, "  Score: %d — %s\n", rep.Score, rep.ScoreLabel)
	fmt.Fprintf(out, "  Eligible checks: %d\n", rep.EligibleChecks)

	if tier == "" || tier == "essential" {
		ess := rep.ScoreBreakdown.Essential
		fmt.Fprintf(out, "  Essential: %d/%d passing, %.1f/%.1f earned\n", ess.Passing, ess.Total, ess.Earned, ess.Available)
	}
	if tier == "" || tier == "recommended" {
		rec := rep.ScoreBreakdown.Recommended
		fmt.Fprintf(out, "  Recommended: %d/%d passing, %.1f/%.1f earned\n", rec.Passing, rec.Total, rec.Earned, rec.Available)
	}
	if tier == "" {
		fmt.Fprintf(out, "  Bonus: %.1f points, %d positive signals\n", rep.ScoreBreakdown.Bonus.Points, rep.ScoreBreakdown.Bonus.PositiveSignals)
	}

	// Group the report's (already-filtered) issues by tier, essential first.
	issues := rep.FailingIssues()
	byTier := map[string][]store.AgenticIssue{}
	var order []string
	for _, iss := range issues {
		if _, ok := byTier[iss.Tier]; !ok {
			order = append(order, iss.Tier)
		}
		byTier[iss.Tier] = append(byTier[iss.Tier], iss)
	}
	// essential always precedes recommended when both are present. The
	// comparator must be a strict weak ordering: `order[i] == "essential"`
	// alone returns true for both (i,j) and (j,i) when both are essential,
	// which violates sort's contract.
	sort.SliceStable(order, func(i, j int) bool {
		return order[i] == "essential" && order[j] != "essential"
	})
	if len(issues) == 0 {
		fmt.Fprintln(out, "  No non-passing issues"+(func() string {
			if tier != "" {
				return " in this tier"
			}
			return ""
		})())
		return nil
	}
	for _, tr := range order {
		fmt.Fprintf(out, "\n  %s\n", bold(tierLabel(tr)))
		for _, iss := range byTier[tr] {
			mark := red("failed")
			if iss.Result == "partial" {
				mark = yellow("partial")
			}
			fmt.Fprintf(out, "    %s\t%s\n", mark, iss.ID)
			desc := iss.Name
			if iss.Details != "" {
				desc = desc + " — " + iss.Details
			}
			if desc != "" {
				fmt.Fprintf(out, "        %s\n", desc)
			}
			if iss.Recommendation != "" {
				fmt.Fprintf(out, "        fix: %s\n", iss.Recommendation)
			}
		}
	}
	return nil
}

// agenticAdviceJSON is the machine-shape of advice for an is-agentic report: a
// native score headline plus the fix prompts as OpenItems (their Check/Prompt
// fields carry is-agentic issue ids / recommendations).
type agenticAdviceJSON struct {
	URL        string           `json:"url"`
	Score      int              `json:"score"`
	ScoreLabel string           `json:"scoreLabel"`
	ScannedAt  string           `json:"scannedAt"`
	Fixes      []store.OpenItem `json:"fixes"`
}

// renderAgenticAdvice prints is-agentic advice for a terminal: the score
// headline then one fix per non-passing issue (recommendation as the fix
// prompt), filtered to the requested check/limit by the caller.
func renderAgenticAdvice(cmd *cobra.Command, rep *store.AgenticReport, items []store.OpenItem) error {
	out := cmd.OutOrStdout()
	fmt.Fprintln(out, bold(rep.Target))
	if rep.ScannedAt != "" {
		fmt.Fprintf(out, "  scanned %s\n", rep.ScannedAt)
	}
	fmt.Fprintf(out, "  Score %d — %s\n", rep.Score, rep.ScoreLabel)
	if len(items) == 0 {
		fmt.Fprintln(out, "  No fixes listed: no non-passing issues in the current filter.")
		return nil
	}
	for i, it := range items {
		fmt.Fprintf(out, "\n  %d. [%s] %s\n", i+1, it.Check, it.Description)
		if it.Prompt != "" {
			fmt.Fprintf(out, "     %s\n", it.Prompt)
		}
	}
	return nil
}

// renderAgenticAdviceCopy prints is-agentic advice as a plain string ready to
// paste into a coding agent, mirroring renderAdviceCopy.
func renderAgenticAdviceCopy(cmd *cobra.Command, rep *store.AgenticReport, items []store.OpenItem) error {
	out := cmd.OutOrStdout()
	if len(items) == 0 {
		fmt.Fprintf(out, "No outstanding agent-readiness fixes for %s (score %d, %s).\n", rep.Target, rep.Score, rep.ScoreLabel)
		return nil
	}
	fmt.Fprintf(out, "Make %s more AI-agent-ready. It is currently score %d (%s); apply these fixes to improve it:\n",
		rep.Target, rep.Score, rep.ScoreLabel)
	for i, it := range items {
		fmt.Fprintf(out, "\n%d. %s\n%s\n", i+1, it.Description, it.Prompt)
	}
	return nil
}

// tierLabel capitalizes an is-agentic tier name for display. strings.Title is
// deprecated (and Unicode-word-aware in ways we do not want here), and the tier
// vocabulary is a closed set of two values.
func tierLabel(tier string) string {
	switch tier {
	case "essential":
		return "Essential"
	case "recommended":
		return "Recommended"
	case "":
		return ""
	default:
		return strings.ToUpper(tier[:1]) + tier[1:]
	}
}
