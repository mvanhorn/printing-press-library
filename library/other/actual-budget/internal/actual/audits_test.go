// Copyright 2026 Matt Anderson and contributors. Licensed under Apache-2.0. See LICENSE.

package actual_test

import (
	"context"
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/other/actual-budget/internal/actual"
)

func TestNormalizePayeeName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Amazon", "amazon"},
		{"AMAZON MKTPLACE", "amazon"},
		{"Amazon.com", "amazon"},
		{"AMZN Mktp US*2K4", "amzn"},
		{"KROGER #123", "kroger"},
		{"Never Used Inc", "never used"},
		{"Oak Street Apartments", "oak street apartments"},
		{"www.Netflix.com", "netflix"},
		{"1234", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := actual.NormalizePayeeName(c.in); got != c.want {
			t.Errorf("NormalizePayeeName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestJaroWinkler(t *testing.T) {
	cases := []struct {
		a, b   string
		lo, hi float64
	}{
		{"martha", "marhta", 0.96, 0.962},
		{"dixon", "dicksonx", 0.81, 0.82},
		{"same", "same", 1, 1},
		{"", "x", 0, 0},
		{"abc", "xyz", 0, 0},
	}
	for _, c := range cases {
		got := actual.JaroWinkler(c.a, c.b)
		if got < c.lo || got > c.hi {
			t.Errorf("JaroWinkler(%q,%q) = %v, want in [%v,%v]", c.a, c.b, got, c.lo, c.hi)
		}
	}
}

func TestLevenshteinSimilarity(t *testing.T) {
	cases := []struct {
		a, b string
		want float64
	}{
		{"kitten", "sitting", 1 - 3.0/7},
		{"", "", 1},
		{"abc", "abc", 1},
		{"abc", "", 0},
	}
	for _, c := range cases {
		if got := actual.LevenshteinSimilarity(c.a, c.b); math.Abs(got-c.want) > 1e-9 {
			t.Errorf("LevenshteinSimilarity(%q,%q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestPayeeSimilarity(t *testing.T) {
	cases := []struct {
		a, b            string
		atLeast, atMost float64
	}{
		{"Amazon", "AMAZON MKTPLACE", 1, 1},
		{"Amazon", "Amazon.com", 1, 1},
		{"Kroger", "Kroger Fuel", 0.9, 0.9}, // whole-word prefix ranks below identical names
		{"Kroger", "Chipotle", 0, 0.6},
		{"Netflix", "Never Used Inc", 0, 0.7},
		{"123", "Amazon", 0, 0},
	}
	for _, c := range cases {
		got := actual.PayeeSimilarity(c.a, c.b)
		if got < c.atLeast || got > c.atMost {
			t.Errorf("PayeeSimilarity(%q,%q) = %v, want in [%v,%v]", c.a, c.b, got, c.atLeast, c.atMost)
		}
	}
}

func TestClusterPayees(t *testing.T) {
	cases := []struct {
		name     string
		in       []actual.Payee
		minSim   float64
		clusters int
		target   string
		merges   int
	}{
		{"variants cluster to most used", []actual.Payee{
			{ID: "a", Name: "Amazon", TxnCount: 5}, {ID: "b", Name: "AMAZON MKTPLACE", TxnCount: 1}, {ID: "c", Name: "Amazon.com", TxnCount: 2}, {ID: "k", Name: "Kroger", TxnCount: 9},
		}, 0.85, 1, "a", 2},
		{"transfer payees ignored", []actual.Payee{
			{ID: "a", Name: "Amazon"}, {ID: "x", Name: "Amazon", TransferAcct: "acct"},
		}, 0.85, 0, "", 0},
		{"distinct names", []actual.Payee{{ID: "k", Name: "Kroger"}, {ID: "c", Name: "Chipotle"}}, 0.85, 0, "", 0},
		{"threshold zero joins everything", []actual.Payee{{ID: "k", Name: "Kroger", TxnCount: 1}, {ID: "c", Name: "Chipotle", TxnCount: 3}}, 0, 1, "c", 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := actual.ClusterPayees(c.in, c.minSim)
			if len(got) != c.clusters {
				t.Fatalf("clusters = %d, want %d: %+v", len(got), c.clusters, got)
			}
			if c.clusters > 0 && (got[0].Target.ID != c.target || len(got[0].Merge) != c.merges) {
				t.Fatalf("cluster = %+v, want target %s with %d merges", got[0], c.target, c.merges)
			}
		})
	}
}

func raw(v any) json.RawMessage { b, _ := json.Marshal(v); return b }

func TestEvalCondition(t *testing.T) {
	tx := actual.Txn{PayeeID: "pay-kroger", ImportedDescription: "KROGER #123 CINCINNATI", Notes: "weekly shop", AccountID: "acct-card", CategoryID: "cat-groceries", Amount: -8450}
	cases := []struct {
		c              actual.RuleCondition
		match, support bool
	}{
		{actual.RuleCondition{Op: "is", Field: "payee", Value: raw("pay-kroger")}, true, true},
		{actual.RuleCondition{Op: "is", Field: "payee", Value: raw("PAY-KROGER")}, false, true}, // ids are exact
		{actual.RuleCondition{Op: "isNot", Field: "account", Value: raw("acct-card")}, false, true},
		{actual.RuleCondition{Op: "contains", Field: "imported_payee", Value: raw("kroger")}, true, true},
		{actual.RuleCondition{Op: "doesNotContain", Field: "notes", Value: raw("shop")}, false, true},
		{actual.RuleCondition{Op: "is", Field: "notes", Value: raw("WEEKLY SHOP")}, true, true},
		{actual.RuleCondition{Op: "oneOf", Field: "category", Value: raw([]string{"cat-dining", "cat-groceries"})}, true, true},
		{actual.RuleCondition{Op: "notOneOf", Field: "category", Value: raw([]string{"cat-groceries"})}, false, true},
		{actual.RuleCondition{Op: "matches", Field: "imported_payee", Value: raw("^kroger #\\d+")}, true, true},
		{actual.RuleCondition{Op: "matches", Field: "imported_payee", Value: raw("(")}, false, false},
		{actual.RuleCondition{Op: "lt", Field: "amount", Value: raw(-5000)}, true, true},
		{actual.RuleCondition{Op: "gte", Field: "amount", Value: raw(0)}, false, true},
		{actual.RuleCondition{Op: "isapprox", Field: "amount", Value: raw(-8000)}, true, true},
		{actual.RuleCondition{Op: "isbetween", Field: "amount", Value: raw(map[string]int{"num1": -9000, "num2": -8000})}, true, true},
		{actual.RuleCondition{Op: "is", Field: "date", Value: raw("2026-07-05")}, false, false},
		{actual.RuleCondition{Op: "hasTags", Field: "notes", Value: raw("x")}, false, false},
	}
	for i, c := range cases {
		m, s := actual.EvalCondition(c.c, tx)
		if m != c.match || s != c.support {
			t.Errorf("case %d %+v: got (%v,%v) want (%v,%v)", i, c.c, m, s, c.match, c.support)
		}
	}
}

func TestRuleMatches(t *testing.T) {
	tx := actual.Txn{PayeeID: "p1", Amount: -100}
	is := func(f string, v any) actual.RuleCondition {
		return actual.RuleCondition{Op: "is", Field: f, Value: raw(v)}
	}
	cases := []struct {
		r              actual.Rule
		match, support bool
	}{
		{actual.Rule{ConditionsOp: "and", Conditions: []actual.RuleCondition{is("payee", "p1"), is("amount", -100)}}, true, true},
		{actual.Rule{ConditionsOp: "and", Conditions: []actual.RuleCondition{is("payee", "p1"), is("amount", -1)}}, false, true},
		{actual.Rule{ConditionsOp: "or", Conditions: []actual.RuleCondition{is("payee", "p2"), is("amount", -100)}}, true, true},
		{actual.Rule{ConditionsOp: "and", Conditions: []actual.RuleCondition{is("payee", "p1"), is("date", "x")}}, false, false},
		{actual.Rule{}, false, false},
	}
	for i, c := range cases {
		m, s := actual.RuleMatches(c.r, tx)
		if m != c.match || s != c.support {
			t.Errorf("case %d: got (%v,%v) want (%v,%v)", i, m, s, c.match, c.support)
		}
	}
}

func TestRuleScheduleID(t *testing.T) {
	cases := []struct {
		r    actual.Rule
		want string
	}{
		{actual.Rule{Actions: []actual.RuleAction{{Op: "link-schedule", Value: raw("sched-1")}}}, "sched-1"},
		{actual.Rule{Actions: []actual.RuleAction{{Op: "set", Field: "category", Value: raw("c")}}}, ""},
	}
	for _, c := range cases {
		if got := c.r.ScheduleID(); got != c.want {
			t.Errorf("ScheduleID = %q, want %q", got, c.want)
		}
	}
}

func TestLedgerRulesAndPayeeClusters(t *testing.T) {
	l := fixtureLedger(t)
	rules, err := l.Rules(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 6 {
		t.Fatalf("rules = %d, want 6", len(rules))
	}
	payees, err := l.Payees(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	clusters := actual.ClusterPayees(payees, 0.85)
	if len(clusters) != 1 || clusters[0].Target.ID != "pay-amazon" || len(clusters[0].Merge) != 2 {
		t.Fatalf("clusters = %+v", clusters)
	}
}

func TestAuditRules(t *testing.T) {
	l := fixtureLedger(t)
	got, err := l.AuditRules(context.Background(), actual.AuditRulesOptions{})
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]actual.RuleAudit{}
	for _, a := range got {
		by[a.ID] = a
	}
	has := func(id, prefix string) bool {
		for _, f := range by[id].Findings {
			if strings.HasPrefix(f, prefix) {
				return true
			}
		}
		return false
	}
	cases := []struct {
		id, finding string
		want        bool
	}{
		{"rule-dead", "never-matches", true},
		{"rule-dangling", "dangling-reference", true},
		{"rule-kroger", "shadowed-by:rule-shadow", true},
		{"rule-shadow", "shadowed-by:rule-kroger", true},
		{"rule-kroger", "never-matches", false},
		{"rule-rent-sched", "never-matches", false},
		{"rule-netflix-sched", "never-matches", false},
		{"rule-rent-sched", "unevaluated", false},
	}
	for _, c := range cases {
		if has(c.id, c.finding) != c.want {
			t.Errorf("%s finding %q present=%v, want %v (%+v)", c.id, c.finding, !c.want, c.want, by[c.id])
		}
	}
	if by["rule-kroger"].MatchCount != 6 {
		t.Errorf("rule-kroger match_count = %d, want 6", by["rule-kroger"].MatchCount)
	}
	if len(by["rule-dangling"].Dangling) != 2 {
		t.Errorf("rule-dangling dangling = %v, want payee+category", by["rule-dangling"].Dangling)
	}
	if !strings.Contains(by["rule-kroger"].Summary, `"Kroger"`) || !strings.Contains(by["rule-kroger"].Summary, `"Groceries"`) {
		t.Errorf("summary not name-resolved: %q", by["rule-kroger"].Summary)
	}
	// Window excludes everything: kroger never matches.
	got, _ = l.AuditRules(context.Background(), actual.AuditRulesOptions{From: 20270101})
	for _, a := range got {
		if a.ID == "rule-kroger" && a.MatchCount != 0 {
			t.Errorf("windowed match_count = %d", a.MatchCount)
		}
	}
}

func TestAuditSchedules(t *testing.T) {
	l := fixtureLedger(t)
	cases := []struct {
		asOf     int
		tol      float64
		id       string
		findings []string
		notFound []string
	}{
		{20260930, 5, "sched-netflix", []string{"overdue", "never-posted"}, nil},
		{20260930, 5, "sched-rent", []string{"amount-drift"}, []string{"overdue"}},
		{20260930, 15, "sched-rent", nil, []string{"amount-drift", "overdue"}},
		{20260905, 5, "sched-netflix", []string{"never-posted"}, []string{"overdue"}},
	}
	for _, c := range cases {
		got, err := l.AuditSchedules(context.Background(), c.asOf, c.tol)
		if err != nil {
			t.Fatal(err)
		}
		var row *actual.ScheduleAudit
		for i := range got {
			if got[i].ID == c.id {
				row = &got[i]
			}
		}
		if row == nil {
			t.Fatalf("%s missing", c.id)
		}
		set := map[string]bool{}
		for _, f := range row.Findings {
			set[f] = true
		}
		for _, f := range c.findings {
			if !set[f] {
				t.Errorf("%s asOf=%d tol=%v: missing %q in %v", c.id, c.asOf, c.tol, f, row.Findings)
			}
		}
		for _, f := range c.notFound {
			if set[f] {
				t.Errorf("%s asOf=%d tol=%v: unexpected %q", c.id, c.asOf, c.tol, f)
			}
		}
		if c.id == "sched-rent" && (row.ExpectedAmount != -150000 || row.LastAmount == nil || *row.LastAmount != -165000 || row.NextDate != "2026-10-03") {
			t.Errorf("rent row = %+v", row)
		}
	}
}

func TestSuggestCategories(t *testing.T) {
	l := fixtureLedger(t)
	cases := []struct {
		minHist int
		minConf float64
		want    map[string]string // txn -> category id
	}{
		{2, 0.6, map[string]string{"txn-kr-6": "cat-groceries"}},
		{1, 0.6, map[string]string{"txn-kr-6": "cat-groceries", "txn-ch-2": "cat-dining", "txn-dup-a": "cat-dining", "txn-dup-b": "cat-dining"}},
		{2, 0.9, map[string]string{}},
	}
	for _, c := range cases {
		got, err := l.SuggestCategories(context.Background(), c.minHist, c.minConf)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != len(c.want) {
			t.Fatalf("minHist=%d minConf=%v: got %+v", c.minHist, c.minConf, got)
		}
		for _, s := range got {
			if c.want[s.TransactionID] != s.SuggestedCategoryID {
				t.Errorf("%s -> %s, want %s", s.TransactionID, s.SuggestedCategoryID, c.want[s.TransactionID])
			}
			if s.TransactionID == "txn-kr-6" && (s.Confidence != 0.8 || s.HistoryCount != 5) {
				t.Errorf("kr-6 confidence %v history %d, want 0.8/5", s.Confidence, s.HistoryCount)
			}
		}
	}
}
