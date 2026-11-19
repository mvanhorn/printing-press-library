// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil/testenv"
)

// TestNovelMarketTrendHelpWires smoke-tests that the market trend command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelMarketTrendHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"market", "trend", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("market trend --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "trend"} {
		if !strings.Contains(help, want) {
			t.Fatalf("market trend --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestMarketTrendOneSnapshotReturnsEmptyPlusNote(t *testing.T) {
	mktTestEnv(t)
	db := mktOpenFixtureStore(t)
	mktFxSnapshot(t, db.DB(), time.Now(), []string{"linen apron"}, 1, "complete", mktFxProg{})
	db.Close()
	out, err := mktRunCLI(t, "market", "trend")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%s", out)
	m := mktDecode(t, out)
	res, ok := m["results"].([]any)
	if !ok || len(res) != 0 {
		t.Fatalf("results = %v, want []", m["results"])
	}
	if !strings.Contains(m["note"].(string), "at least two") {
		t.Errorf("note = %v", m["note"])
	}
}

func TestMarketTrendEmptyStoreExitsZero(t *testing.T) {
	mktTestEnv(t)
	out, err := mktRunCLI(t, "market", "trend")
	if err != nil {
		t.Fatalf("empty store should exit 0: %v", err)
	}
	if m := mktDecode(t, out); len(m["results"].([]any)) != 0 || m["note"] == "" {
		t.Errorf("got %v", m)
	}
}

func TestMarketTrendFixturePair(t *testing.T) {
	mktTestEnv(t)
	db := mktOpenFixtureStore(t)
	d := db.DB()
	a := mktFxSnapshot(t, d, time.Now().Add(-48*time.Hour), []string{"linen apron", "ceramic mug"}, 1, "complete", mktFxProg{})
	b := mktFxSnapshot(t, d, time.Now(), []string{"linen apron", "ceramic mug", "wool scarf"}, 1, "complete", mktFxProg{})
	for _, l := range []mktFxListing{
		{term: "linen apron", pos: 1, id: 1000000001, shop: 5001, title: "Linen Apron A", price: 2400, favs: 10},
		{term: "linen apron", pos: 2, id: 1000000002, shop: 5002, title: "Linen Apron B", price: 3000, favs: 5},
		{term: "linen apron", pos: 3, id: 1000000003, shop: 5003, title: "Linen Apron C", price: 2800, favs: 5},
		{term: "ceramic mug", pos: 1, id: 1000000010, shop: 5004, title: "Mug M", price: 1800, favs: 3},
	} {
		mktFxResult(t, d, a, l)
	}
	for _, l := range []mktFxListing{
		{term: "linen apron", pos: 1, id: 1000000002, shop: 5002, title: "Linen Apron B", price: 3000, favs: 5},
		{term: "linen apron", pos: 2, id: 1000000001, shop: 5001, title: "Linen Apron A", price: 2600, favs: 14},
		{term: "linen apron", pos: 3, id: 1000000004, shop: 5009, title: "Linen Apron D", price: 2000, favs: 1},
		{term: "ceramic mug", pos: 1, id: 1000000010, shop: 5004, title: "Mug M", price: 1800, favs: 3},
		{term: "wool scarf", pos: 1, id: 1000000099, shop: 5099, title: "Scarf only in newer", price: 4000, favs: 1},
	} {
		mktFxResult(t, d, b, l)
	}
	db.Close()

	out, err := mktRunCLI(t, "market", "trend")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%s", out)
	m := mktDecode(t, out)
	ids := func(key string) []int64 {
		var r []int64
		for _, x := range m["results"].(map[string]any)[key].([]any) {
			r = append(r, int64(x.(map[string]any)["listing_id"].(float64)))
		}
		return r
	}
	if got := ids("new_listings"); len(got) != 1 || got[0] != 1000000004 {
		t.Errorf("new_listings = %v, want exactly [1000000004] (wool scarf term is not comparable)", got)
	}
	if got := ids("exited_listings"); len(got) != 1 || got[0] != 1000000003 {
		t.Errorf("exited_listings = %v, want exactly [1000000003]", got)
	}
	if ns := m["results"].(map[string]any)["new_shops"].([]any); len(ns) != 1 || ns[0].(float64) != 5009 {
		t.Errorf("new_shops = %v", ns)
	}
	if es := m["results"].(map[string]any)["exited_shops"].([]any); len(es) != 1 || es[0].(float64) != 5003 {
		t.Errorf("exited_shops = %v", es)
	}
	moves := m["results"].(map[string]any)["position_moves"].([]any)
	if len(moves) != 2 {
		t.Fatalf("position_moves = %v, want 2", moves)
	}
	pc := m["results"].(map[string]any)["price_changes"].([]any)
	if len(pc) != 1 || pc[0].(map[string]any)["from"].(float64) != 24 || pc[0].(map[string]any)["to"].(float64) != 26 {
		t.Errorf("price_changes = %v", pc)
	}
	fd := m["results"].(map[string]any)["favorites_deltas"].([]any)
	found := false
	for _, x := range fd {
		if x.(map[string]any)["listing_id"].(float64) == 1000000001 && x.(map[string]any)["delta"].(float64) == 4 {
			found = true
		}
	}
	if !found {
		t.Errorf("favorites_deltas = %v, want +4 for 1000000001", fd)
	}
	if !strings.Contains(m["note"].(string), "wool scarf") {
		t.Errorf("note should name the skipped term: %v", m["note"])
	}
}

func TestMarketTrendQuotedSkippedTerms(t *testing.T) {
	for _, tc := range []struct {
		from, to []string
		want     string
	}{
		{[]string{"linen apron", "Juniper Ceramics"}, []string{"ceramic mug"}, `compared 1 term(s) present in both snapshots; skipped terms only in snapshot 1: "linen apron", "Juniper Ceramics"; only in snapshot 2: "ceramic mug"`},
		{[]string{"linen apron"}, nil, `compared 1 term(s) present in both snapshots; skipped terms only in snapshot 1: "linen apron"`},
		{nil, []string{"ceramic mug"}, `compared 1 term(s) present in both snapshots; skipped terms only in snapshot 2: "ceramic mug"`},
	} {
		got := mktCompareSnapshots(&mktSnapshot{ID: 1}, &mktSnapshot{ID: 2}, nil, nil, []string{"common"}, tc.from, tc.to)
		if got["note"] != tc.want {
			t.Errorf("note = %q, want %q", got["note"], tc.want)
		}
		skipped := got["skipped_terms"].(map[string]any)
		if len(skipped["only_in_from"].([]string)) != len(tc.from) || len(skipped["only_in_to"].([]string)) != len(tc.to) {
			t.Fatal(skipped)
		}
	}
}

func TestMarketEmptyStoreExactHint(t *testing.T) {
	for _, initialized := range []bool{false, true} {
		for _, name := range []string{"demand", "price", "gaps", "shops", "tags", "trend"} {
			t.Run(fmt.Sprintf("%s_store_%t", name, initialized), func(t *testing.T) {
				mktTestEnv(t)
				if initialized {
					db := mktOpenFixtureStore(t)
					db.Close()
				}
				cmd := RootCmd()
				args := []string{"market", name, "--json"}
				if name == "gaps" {
					args = append(args, "--rules=-")
					cmd.SetIn(strings.NewReader(mktGapsHappyStdin))
				}
				cmd.SetArgs(args)
				var out bytes.Buffer
				cmd.SetOut(&out)
				cmd.SetErr(&out)
				if err := cmd.Execute(); err != nil {
					t.Fatal(err)
				}
				const want = `no market snapshots yet; run 'etsy-pp-cli market snapshot --term "SEARCH TERM"' first`
				if got := mktDecode(t, out.String())["note"]; got != want {
					t.Errorf("note=%q, want %q", got, want)
				}
			})
		}
	}
}

func TestMarketTrendOnlyPartialSnapshots(t *testing.T) {
	mktTestEnv(t)
	db := mktOpenFixtureStore(t)
	mktFxSnapshot(t, db.DB(), time.Now(), []string{"linen apron"}, 1, "partial", mktFxProg{})
	db.Close()
	out, err := mktRunCLI(t, "market", "trend")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "need at least two complete snapshots to compare; found 0") || !strings.Contains(out, "resume the partial snapshot with etsy-pp-cli market snapshot --resume 1") {
		t.Fatalf("TREND_HINT_PARTIAL_MISSING: %s", out)
	}
}
