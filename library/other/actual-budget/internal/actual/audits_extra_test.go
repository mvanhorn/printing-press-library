// Copyright 2026 Matt Anderson and contributors. Licensed under Apache-2.0. See LICENSE.

package actual_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/mvanhorn/printing-press-library/library/other/actual-budget/internal/actual"
	"github.com/mvanhorn/printing-press-library/library/other/actual-budget/internal/actual/actualtest"
)

func TestNormalizePayeeNameProcessorPrefixes(t *testing.T) {
	cases := []struct{ in, want string }{
		{"SQ *JOES PIZZA", "joes pizza"},
		{"Sq *Blue Bottle", "blue bottle"},
		{"TST* BLUE BOTTLE COFFEE", "blue bottle coffee"},
		{"PAYPAL *NETFLIX", "netflix"},
		{"PP*SPOTIFY", "spotify"},
		{"SP * Allbirds", "allbirds"},
		{"AMZN Mktp US*2K4", "amzn"}, // not a processor: suffix still dropped
		{"Amazon.com*AB12", "amazon"},
	}
	for _, c := range cases {
		if got := actual.NormalizePayeeName(c.in); got != c.want {
			t.Errorf("NormalizePayeeName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestPayeeSimilarityWordBoundedPrefix(t *testing.T) {
	cases := []struct {
		a, b   string
		lo, hi float64
	}{
		{"Kroger", "Kroger Fuel", 0.9, 0.9}, // whole-word prefix
		{"Transfer to Savings", "Transfer from Savings", 0, 0},
		{"Venmo Sent", "Venmo Received", 0, 0},
		{"Apple", "Applebee's", 0, 0.84},    // mid-word prefix: distinct merchant
		{"Shell", "Shellfish Bar", 0, 0.84}, // mid-word prefix: distinct merchant
		{"SQ *JOES PIZZA", "SQ *BLUE BOTTLE", 0, 0.84},
	}
	for _, c := range cases {
		got := actual.PayeeSimilarity(c.a, c.b)
		if got < c.lo || got > c.hi {
			t.Errorf("PayeeSimilarity(%q,%q) = %v, want in [%v,%v]", c.a, c.b, got, c.lo, c.hi)
		}
	}
}

func TestClusterPayeesFalsePositives(t *testing.T) {
	cases := []struct {
		name string
		in   []actual.Payee
	}{
		{"apple vs applebees", []actual.Payee{{ID: "a", Name: "Apple", TxnCount: 3}, {ID: "b", Name: "Applebee's", TxnCount: 2}}},
		{"shell vs shellfish bar", []actual.Payee{{ID: "s", Name: "Shell", TxnCount: 3}, {ID: "f", Name: "Shellfish Bar", TxnCount: 1}}},
		{"square merchants", []actual.Payee{{ID: "j", Name: "SQ *JOES PIZZA"}, {ID: "b", Name: "SQ *BLUE BOTTLE"}}},
		{"toast merchants", []actual.Payee{{ID: "j", Name: "TST* JOES DINER"}, {ID: "b", Name: "TST* BLUE BOTTLE"}}},
		{"paypal merchants", []actual.Payee{{ID: "n", Name: "PAYPAL *NETFLIX"}, {ID: "s", Name: "PAYPAL *STEAM GAMES"}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := actual.ClusterPayees(c.in, 0.85); len(got) != 0 {
				t.Fatalf("expected no clusters, got %+v", got)
			}
		})
	}
	// Positive control: the same processor + merchant still clusters.
	got := actual.ClusterPayees([]actual.Payee{{ID: "a", Name: "SQ *JOES PIZZA", TxnCount: 2}, {ID: "b", Name: "Joe's Pizza", TxnCount: 1}}, 0.85)
	if len(got) != 1 || got[0].Target.ID != "a" {
		t.Fatalf("expected SQ *JOES PIZZA / Joe's Pizza to cluster, got %+v", got)
	}
}

// TestClusterPayeesPruningMatchesBruteForce checks the length-ratio pruning
// and Jaro-Winkler short-circuit never change which pairs link (compared
// with an unpruned all-pairs reference using the same target split).
func TestClusterPayeesPruningMatchesBruteForce(t *testing.T) {
	names := []string{"Amazon", "AMAZON MKTPLACE", "Amazon.com", "Kroger", "Kroger Fuel", "Krogers", "Chipotle",
		"Chipotle Mexican Grill", "Netflix", "Netflx", "Apple", "Applebee's", "Shell", "Shellfish Bar", "Target", "Targt",
		"Walmart", "Wal-Mart", "Home Depot", "The Home Depot", "Costco", "Costco Gas", "Uber", "Uber Eats", "Lyft"}
	payees := make([]actual.Payee, len(names))
	for i, n := range names {
		payees[i] = actual.Payee{ID: fmt.Sprintf("p%02d", i), Name: n, TxnCount: len(names) - i}
	}
	for _, minSim := range []float64{0, 0.5, 0.7, 0.85, 0.9, 0.95, 1} {
		got := actual.ClusterPayees(payees, minSim)
		want := bruteClusters(payees, minSim)
		if fmt.Sprint(clusterKeys(got)) != fmt.Sprint(want) {
			t.Errorf("minSim=%v: got %v, want %v", minSim, clusterKeys(got), want)
		}
	}
}

func bruteClusters(p []actual.Payee, minSim float64) map[string]string {
	parent := make([]int, len(p))
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(i int) int {
		if parent[i] != i {
			parent[i] = find(parent[i])
		}
		return parent[i]
	}
	for i := range p {
		for j := i + 1; j < len(p); j++ {
			if actual.PayeeSimilarity(p[i].Name, p[j].Name) >= minSim {
				parent[find(i)] = find(j)
			}
		}
	}
	// Components in index order (TxnCount descending, as ClusterPayees
	// sorts), then the target-anchored split: each cluster keeps only the
	// members that match its target directly; the rest form the next one.
	comps := map[int][]int{}
	var roots []int
	for i := range p {
		r := find(i)
		if _, ok := comps[r]; !ok {
			roots = append(roots, r)
		}
		comps[r] = append(comps[r], i)
	}
	out := map[string]string{}
	for _, r := range roots {
		g := comps[r]
		for len(g) >= 2 {
			t := g[0]
			members := []int{t}
			var rest []int
			for _, k := range g[1:] {
				if actual.PayeeSimilarity(p[t].Name, p[k].Name) >= minSim {
					members = append(members, k)
				} else {
					rest = append(rest, k)
				}
			}
			if len(members) > 1 {
				m := p[members[0]].ID
				for _, k := range members {
					if p[k].ID < m {
						m = p[k].ID
					}
				}
				for _, k := range members {
					out[p[k].ID] = m
				}
			}
			g = rest
		}
	}
	return out
}

func clusterKeys(cs []actual.PayeeCluster) map[string]string {
	out := map[string]string{}
	for _, c := range cs {
		all := append([]actual.Payee{c.Target}, c.Merge...)
		m := all[0].ID
		for _, p := range all {
			if p.ID < m {
				m = p.ID
			}
		}
		for _, p := range all {
			out[p.ID] = m
		}
	}
	return out
}

func TestClusterPayeesContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := actual.ClusterPayeesContext(ctx, []actual.Payee{{ID: "a", Name: "Amazon"}, {ID: "b", Name: "Amazon.com"}}, 0.85)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func TestEvalConditionInflowOutflow(t *testing.T) {
	out := actual.Txn{Amount: -5000} // $50 spent
	in := actual.Txn{Amount: 5000}   // $50 received
	outflow := actual.RuleConditionOptions{Outflow: true}
	inflow := actual.RuleConditionOptions{Inflow: true}
	cond := func(op string, v any, o actual.RuleConditionOptions) actual.RuleCondition {
		return actual.RuleCondition{Op: op, Field: "amount", Value: raw(v), Type: "number", Options: o}
	}
	cases := []struct {
		name  string
		c     actual.RuleCondition
		t     actual.Txn
		match bool
	}{
		{"outflow is matches spend", cond("is", 5000, outflow), out, true},
		{"outflow is ignores income", cond("is", 5000, outflow), in, false},
		{"inflow is matches income", cond("is", 5000, inflow), in, true},
		{"inflow is ignores spend", cond("is", 5000, inflow), out, false},
		{"outflow gt compares magnitude", cond("gt", 4000, outflow), out, true},
		{"outflow gt larger threshold", cond("gt", 6000, outflow), out, false},
		{"outflow lt compares magnitude", cond("lt", 6000, outflow), out, true},
		{"outflow lt never matches income", cond("lt", 6000, outflow), in, false},
		{"inflow gte", cond("gte", 5000, inflow), in, true},
		{"outflow isbetween", cond("isbetween", map[string]int{"num1": 4000, "num2": 6000}, outflow), out, true},
		{"outflow isbetween income", cond("isbetween", map[string]int{"num1": 4000, "num2": 6000}, outflow), in, false},
		{"inflow isbetween", cond("isbetween", map[string]int{"num1": 4000, "num2": 6000}, inflow), in, true},
		{"outflow isapprox", cond("isapprox", 5200, outflow), out, true},
		{"no options signed is", cond("is", -5000, actual.RuleConditionOptions{}), out, true},
		{"no options positive value misses spend", cond("is", 5000, actual.RuleConditionOptions{}), out, false},
	}
	for _, c := range cases {
		m, s := actual.EvalCondition(c.c, c.t)
		if !s || m != c.match {
			t.Errorf("%s: got (%v,%v), want (%v,true)", c.name, m, s, c.match)
		}
	}
	// Options decode from Actual's stored JSON.
	var rc actual.RuleCondition
	if err := json.Unmarshal([]byte(`{"op":"is","field":"amount","value":5000,"options":{"outflow":true}}`), &rc); err != nil || !rc.Options.Outflow || rc.Options.Inflow {
		t.Fatalf("decoded options = %+v (err %v)", rc.Options, err)
	}
}

func TestAuditRulesContextCancelled(t *testing.T) {
	l := fixtureLedger(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := l.AuditRules(ctx, actual.AuditRulesOptions{}); err == nil {
		t.Fatal("expected an error from a cancelled context")
	}
}

func TestAuditSchedulesPostingsUnavailable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db.sqlite")
	if err := actualtest.Build(path); err != nil {
		t.Fatal(err)
	}
	rw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rw.Exec(`ALTER TABLE transactions DROP COLUMN schedule`); err != nil {
		rw.Close()
		t.Fatal(err)
	}
	rw.Close()
	db, err := actual.OpenReadOnly(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	got, err := actual.NewLedger(db).AuditSchedules(context.Background(), 20260930, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) == 0 {
		t.Fatal("no schedules returned")
	}
	for _, a := range got {
		if len(a.Findings) != 1 || a.Findings[0] != "postings-unavailable" {
			t.Errorf("%s findings = %v, want only postings-unavailable", a.ID, a.Findings)
		}
	}
}

// Single linkage must not chain payees the pairwise checks reject: every
// merged member matches its target directly at minSim.
func TestClusterPayeesNoChaining(t *testing.T) {
	cases := [][]string{
		{"Transfer to Savings", "Transfer Savings", "Transfer from Savings"},
		{"Zelle to Jane Doe", "Zelle Jane Doe", "Zelle from Jane Doe"},
		{"Shell", "Shell Gas", "Shell Oil"},
	}
	for _, names := range cases {
		payees := make([]actual.Payee, len(names))
		for i, n := range names {
			payees[i] = actual.Payee{ID: fmt.Sprintf("p%d", i), Name: n, TxnCount: 10 - i}
		}
		for _, c := range actual.ClusterPayees(payees, 0.85) {
			if c.Similarity < 0.85 {
				t.Errorf("%v: cluster %q at similarity %.3f below threshold", names, c.Target.Name, c.Similarity)
			}
			for _, m := range c.Merge {
				if s := actual.PayeeSimilarity(c.Target.Name, m.Name); s < 0.85 {
					t.Errorf("%v: %q merged into %q at %.3f", names, m.Name, c.Target.Name, s)
				}
			}
		}
	}
}
