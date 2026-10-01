// Copyright 2026 Matt Anderson and contributors. Licensed under Apache-2.0. See LICENSE.

package actual_test

import (
	"context"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/other/actual-budget/internal/actual"
)

func TestParseNoteTemplates(t *testing.T) {
	cases := []struct {
		note        string
		n           int
		typ         string
		target      int64
		unsupported bool
	}{
		{"Weekly food\n#template 400", 1, actual.TemplateSimple, 40000, false},
		{`Weekly food\n#template 400`, 1, actual.TemplateSimple, 40000, false},
		{"#template up to 1,250.50", 1, actual.TemplateUpTo, 125050, false},
		{"#goal $6000", 1, actual.TemplateGoal, 600000, false},
		{"#template-2 15", 1, actual.TemplateSimple, 1500, false},
		{"#template 500 by 2026-12", 1, actual.TemplateSimple, 0, true},
		{"#template 10% of Salary", 1, actual.TemplateSimple, 0, true},
		{"just a note", 0, "", 0, false},
		{"", 0, "", 0, false},
	}
	for _, c := range cases {
		got := actual.ParseNoteTemplates(c.note)
		if len(got) != c.n {
			t.Fatalf("%q: got %d templates (%+v), want %d", c.note, len(got), got, c.n)
		}
		if c.n == 0 {
			continue
		}
		g := got[0]
		if g.Type != c.typ || g.Target != c.target || (g.Unsupported != "") != c.unsupported {
			t.Errorf("%q: got %+v, want type %s target %d unsupported %v", c.note, g, c.typ, c.target, c.unsupported)
		}
	}
}

func TestParseGoalDef(t *testing.T) {
	cases := []struct {
		in          string
		n           int
		typ         string
		target      int64
		unsupported bool
		err         bool
	}{
		{`[{"type":"goal","amount":6000}]`, 1, actual.TemplateGoal, 600000, false, false},
		{`[{"type":"simple","monthly":150}]`, 1, actual.TemplateSimple, 15000, false, false},
		{`[{"type":"periodic","amount":"75.5"}]`, 1, actual.TemplateSimple, 7550, false, false},
		{`[{"type":"periodic","amount":40,"period":{"period":"month","amount":1}}]`, 1, actual.TemplateSimple, 4000, false, false},
		// Weekly / multi-month periods are not monthly amounts: never treated as one.
		{`[{"type":"periodic","amount":25,"period":{"period":"week","amount":1}}]`, 1, actual.TemplateSimple, 0, true, false},
		{`[{"type":"periodic","amount":300,"period":{"period":"month","amount":3}}]`, 1, actual.TemplateSimple, 0, true, false},
		{`[{"type":"periodic","amount":25,"period":"day"}]`, 1, actual.TemplateSimple, 0, true, false},
		{`[{"type":"simple","limit":{"amount":300}}]`, 1, actual.TemplateUpTo, 30000, false, false},
		{`[{"type":"percentage","percent":10}]`, 1, "percentage", 0, true, false},
		{`{"type":"goal","amount":10}`, 1, actual.TemplateGoal, 1000, false, false},
		{``, 0, "", 0, false, false},
		{`not json`, 0, "", 0, false, true},
	}
	for _, c := range cases {
		got, err := actual.ParseGoalDef(c.in)
		if (err != nil) != c.err {
			t.Fatalf("%q: err = %v", c.in, err)
		}
		if len(got) != c.n {
			t.Fatalf("%q: got %+v", c.in, got)
		}
		if c.n == 0 {
			continue
		}
		g := got[0]
		if g.Type != c.typ || g.Target != c.target || (g.Unsupported != "") != c.unsupported {
			t.Errorf("%q: got %+v", c.in, g)
		}
	}
}

func TestFundingStatus(t *testing.T) {
	cases := []struct {
		typ              string
		target, budgeted int64
		want             string
	}{
		{actual.TemplateSimple, 100, 100, "funded"},
		{actual.TemplateSimple, 100, 50, "underfunded"},
		{actual.TemplateUpTo, 100, 150, "overfunded"},
		{actual.TemplateGoal, 100, 0, "goal"},
	}
	for _, c := range cases {
		if got := actual.FundingStatus(c.typ, c.target, c.budgeted); got != c.want {
			t.Errorf("FundingStatus(%s,%d,%d) = %s, want %s", c.typ, c.target, c.budgeted, got, c.want)
		}
	}
}

func TestTemplateStatuses(t *testing.T) {
	l := fixtureLedger(t)
	got, err := l.TemplateStatuses(context.Background(), "2026-09")
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]actual.TemplateStatus{}
	for _, r := range got {
		by[r.CategoryID] = r
	}
	cases := []struct {
		cat              string
		target, budgeted int64
		status           string
	}{
		{"cat-groceries", 40000, 40000, "funded"},
		{"cat-dining", 15000, 10000, "underfunded"},
		{"cat-rent", 150000, 150000, "funded"},
		{"cat-car", 600000, 20000, "goal"},
	}
	for _, c := range cases {
		r, ok := by[c.cat]
		if !ok || r.Target != c.target || r.Budgeted != c.budgeted || r.Status != c.status {
			t.Errorf("%s = %+v, want target %d budgeted %d %s", c.cat, r, c.target, c.budgeted, c.status)
		}
	}
	if r := by["cat-rent"]; r.Spent != 165000 {
		t.Errorf("rent spent = %d, want 165000", r.Spent)
	}
	if _, err := l.TemplateStatuses(context.Background(), "bad"); err == nil {
		t.Error("expected error for invalid month")
	}
}
