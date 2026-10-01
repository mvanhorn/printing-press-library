// Copyright 2026 Matt Anderson and contributors. Licensed under Apache-2.0. See LICENSE.

package actual

// Envelope template status: parses Actual's budget templates (the legacy
// "#template"/"#goal" lines in category notes and the newer goal_def JSON
// column) and compares them with what was budgeted for a month.

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Template kinds.
const (
	TemplateSimple = "template"
	TemplateUpTo   = "template up to"
	TemplateGoal   = "goal"
)

// TemplateTarget is one parsed template directive. Target is in minor units.
type TemplateTarget struct {
	Type   string `json:"template_type"`
	Target int64  `json:"target"`
	Source string `json:"source"` // "note" or "goal_def"
	Raw    string `json:"raw,omitempty"`
	// Unsupported is set (with a reason) when the directive was recognised
	// but its form is not modelled; Target is then 0.
	Unsupported string `json:"unsupported,omitempty"`
}

var (
	templateLine = regexp.MustCompile(`(?i)^\s*#(template|goal)(?:-\d+)?\b\s*(.*)$`)
	upToPrefix   = regexp.MustCompile(`(?i)^up\s+to\s+`)
	amountToken  = regexp.MustCompile(`^\$?-?[\d,]+(?:\.\d+)?$`)
)

// ParseNoteTemplates extracts "#template <amount>", "#template up to <amount>"
// and "#goal <amount>" lines from a category note. Amounts are decimal major
// units. Other template forms (by-date, percentages, schedules, repeats) are
// returned with Unsupported set so callers can surface them.
func ParseNoteTemplates(note string) []TemplateTarget {
	var out []TemplateTarget
	// Notes written through some API clients/exports carry escaped "\n"
	// sequences instead of real newlines; treat both as line breaks.
	note = strings.ReplaceAll(strings.ReplaceAll(note, "\r\n", "\n"), `\n`, "\n")
	for _, line := range strings.Split(note, "\n") {
		m := templateLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		kind, rest := strings.ToLower(m[1]), strings.TrimSpace(m[2])
		t := TemplateTarget{Source: "note", Raw: strings.TrimSpace(line)}
		if kind == "goal" {
			t.Type = TemplateGoal
		} else {
			t.Type = TemplateSimple
			if upToPrefix.MatchString(rest) {
				t.Type = TemplateUpTo
				rest = upToPrefix.ReplaceAllString(rest, "")
			}
		}
		fields := strings.Fields(rest)
		if len(fields) == 0 || !amountToken.MatchString(fields[0]) {
			t.Unsupported = "no leading amount"
			out = append(out, t)
			continue
		}
		v, err := ParseAmount(fields[0])
		if err != nil {
			t.Unsupported = err.Error()
			out = append(out, t)
			continue
		}
		if len(fields) > 1 {
			t.Unsupported = "unsupported template form: " + strings.Join(fields[1:], " ")
			out = append(out, t)
			continue
		}
		t.Target = v
		out = append(out, t)
	}
	return out
}

// ParseGoalDef parses the goal_def JSON column (an array of template
// objects). The "amount" (or "monthly" for simple templates) field is in major
// units. Types other than simple/goal/periodic, and periodic templates whose
// period is not one month, are returned Unsupported.
func ParseGoalDef(goalDef string) ([]TemplateTarget, error) {
	goalDef = strings.TrimSpace(goalDef)
	if goalDef == "" || goalDef == "null" {
		return nil, nil
	}
	var items []map[string]any
	if err := json.Unmarshal([]byte(goalDef), &items); err != nil {
		var one map[string]any
		if err2 := json.Unmarshal([]byte(goalDef), &one); err2 != nil {
			return nil, fmt.Errorf("parsing goal_def: %w", err)
		}
		items = []map[string]any{one}
	}
	var out []TemplateTarget
	for _, it := range items {
		typ, _ := it["type"].(string)
		raw, _ := json.Marshal(it)
		t := TemplateTarget{Source: "goal_def", Raw: string(raw)}
		switch strings.ToLower(typ) {
		case "goal":
			t.Type = TemplateGoal
		case "simple", "periodic":
			t.Type = TemplateSimple
			if strings.EqualFold(typ, "periodic") {
				if why := periodicUnsupported(it["period"]); why != "" {
					t.Unsupported = why
					out = append(out, t)
					continue
				}
			}
			if lim, ok := it["limit"].(map[string]any); ok && it["amount"] == nil && it["monthly"] == nil {
				if v, ok := numeric(lim["amount"]); ok {
					t.Type = TemplateUpTo
					t.Target = majorToMinor(v)
					out = append(out, t)
					continue
				}
			}
		default:
			t.Type = typ
			t.Unsupported = fmt.Sprintf("template type %q not modelled", typ)
			out = append(out, t)
			continue
		}
		v, ok := numeric(it["amount"])
		if !ok {
			v, ok = numeric(it["monthly"])
		}
		if !ok {
			t.Unsupported = "no amount"
			out = append(out, t)
			continue
		}
		t.Target = majorToMinor(v)
		out = append(out, t)
	}
	return out, nil
}

// periodicUnsupported returns a reason when a periodic template's period is
// not exactly one month (e.g. weekly, every 2 months), which would need
// occurrence expansion within the budget month; "" when it is monthly or the
// period is absent (treated as monthly).
func periodicUnsupported(period any) string {
	if period == nil {
		return ""
	}
	unit, n := "", 1.0
	switch p := period.(type) {
	case string:
		unit = p
	case map[string]any:
		unit, _ = p["period"].(string)
		if v, ok := numeric(p["amount"]); ok {
			n = v
		} else if p["amount"] != nil {
			return "unsupported periodic template: unreadable period amount"
		}
	default:
		return "unsupported periodic template: unreadable period"
	}
	unit = strings.ToLower(strings.TrimSpace(unit))
	if (unit == "month" || unit == "months" || unit == "monthly") && n == 1 {
		return ""
	}
	if unit == "" {
		return "unsupported periodic template: unreadable period"
	}
	return fmt.Sprintf("unsupported periodic template: every %v %s (only monthly is modelled)", n, unit)
}

func numeric(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(n), 64)
		return f, err == nil
	}
	return 0, false
}

func majorToMinor(v float64) int64 { return int64(math.Round(v * 100)) }

// TemplateStatus is one category's funding state against its template.
type TemplateStatus struct {
	CategoryID   string `json:"category_id"`
	Category     string `json:"category"`
	Group        string `json:"group"`
	TemplateType string `json:"template_type"`
	Source       string `json:"source"`
	Target       int64  `json:"target"`
	Budgeted     int64  `json:"budgeted"`
	Spent        int64  `json:"spent"`
	BalanceHint  int64  `json:"balance_hint"`
	Status       string `json:"status"`
	Note         string `json:"note,omitempty"`
}

// FundingStatus classifies budgeted against a template target.
func FundingStatus(templateType string, target, budgeted int64) string {
	switch {
	case templateType == TemplateGoal:
		return "goal"
	case budgeted < target:
		return "underfunded"
	case budgeted > target:
		return "overfunded"
	}
	return "funded"
}

// TemplateStatuses compares every category's templates with the month's
// budgeted amount and activity. month is YYYY-MM. goal_def templates take
// precedence over note templates for the same category (Actual migrates
// note templates into goal_def). "#template" amounts in one category are
// summed; "up to" caps and goals get their own rows.
func (l *Ledger) TemplateStatuses(ctx context.Context, month string) ([]TemplateStatus, error) {
	_, _, ym, err := MonthRange(month)
	if err != nil {
		return nil, err
	}
	cats, err := l.Categories(ctx)
	if err != nil {
		return nil, err
	}
	notes, err := l.Notes(ctx)
	if err != nil {
		return nil, err
	}
	budgeted, err := l.BudgetedAmounts(ctx, ym)
	if err != nil {
		return nil, err
	}
	// Same activity definition as BudgetVsActual: on-budget accounts only,
	// starting balances excluded.
	activity, err := l.categoryActivity(ctx, ym, "")
	if err != nil {
		return nil, err
	}
	out := make([]TemplateStatus, 0)
	for _, c := range cats {
		var targets []TemplateTarget
		var note string
		if c.GoalDef != "" {
			targets, err = ParseGoalDef(c.GoalDef)
			if err != nil {
				note = err.Error()
			}
		}
		if len(targets) == 0 {
			targets = ParseNoteTemplates(notes[c.ID])
		}
		if len(targets) == 0 && note == "" {
			continue
		}
		base := TemplateStatus{CategoryID: c.ID, Category: c.Name, Group: c.Group, Budgeted: budgeted[c.ID], Spent: -activity[c.ID]}
		base.BalanceHint = base.Budgeted + activity[c.ID]
		agg := map[string]*TemplateStatus{}
		var order []string
		var skipped []string
		for _, t := range targets {
			if t.Unsupported != "" {
				skipped = append(skipped, t.Raw+" ("+t.Unsupported+")")
				continue
			}
			row, ok := agg[t.Type]
			if !ok {
				r := base
				r.TemplateType, r.Source = t.Type, t.Source
				row = &r
				agg[t.Type] = row
				order = append(order, t.Type)
			}
			if t.Type == TemplateSimple {
				row.Target += t.Target
			} else {
				row.Target = t.Target
			}
		}
		if len(order) == 0 {
			r := base
			r.Status = "unsupported"
			r.Note = strings.TrimPrefix(strings.Join(append([]string{note}, skipped...), "; "), "; ")
			out = append(out, r)
			continue
		}
		sort.Strings(order)
		for _, k := range order {
			r := *agg[k]
			r.Status = FundingStatus(r.TemplateType, r.Target, r.Budgeted)
			if len(skipped) > 0 {
				r.Note = "skipped: " + strings.Join(skipped, "; ")
			}
			out = append(out, r)
		}
	}
	return out, nil
}
