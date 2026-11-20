// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil/testenv"
)

// TestNovelMarketTagsHelpWires smoke-tests that the market tags command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelMarketTagsHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"market", "tags", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("market tags --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "tags"} {
		if !strings.Contains(help, want) {
			t.Fatalf("market tags --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestMarketTagsContrastByPosition(t *testing.T) {
	mktTestEnv(t)
	db := mktOpenFixtureStore(t)
	d := db.DB()
	snap := mktFxSnapshot(t, d, time.Now(), []string{"linen apron"}, 1, "complete", mktFxProg{})
	// 12 listings; the top quartile (positions 1-3) all carry "stonewash"
	// and the word "stonewashed"; the rest rarely do.
	for i := 1; i <= 12; i++ {
		l := mktFxListing{term: "linen apron", pos: i, id: int64(1000000000 + i), shop: int64(5000 + i), price: 2400}
		switch {
		case i <= 3:
			l.tags, l.title = []string{"stonewash", "kitchen"}, "Stonewashed Linen Apron"
		case i == 4:
			l.tags, l.title = []string{"stonewash", "kitchen"}, "Plain Apron"
		default:
			l.tags, l.title = []string{"kitchen"}, "Plain Apron"
		}
		mktFxResult(t, d, snap, l)
	}
	db.Close()
	out, err := mktRunCLI(t, "market", "tags", "--min-support", "3")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%s", out)
	m := mktDecode(t, out)
	if m["results"].(map[string]any)["min_support"] != float64(3) {
		t.Fatalf("TAGS_MIN_SUPPORT_BROKEN: %v", m)
	}
	if m["results"].(map[string]any)["basis"] != "position" || m["results"].(map[string]any)["top_group_size"].(float64) != 3 || m["results"].(map[string]any)["rest_group_size"].(float64) != 9 {
		t.Fatalf("grouping wrong: %v", m)
	}
	tags := m["results"].(map[string]any)["tags"].([]any)
	first := tags[0].(map[string]any)
	if first["term"] != "stonewash" || first["top_count"].(float64) != 3 || first["rest_count"].(float64) != 1 {
		t.Errorf("top tag = %v, want stonewash 3 vs 1", first)
	}
	if first["top_rate"].(float64) != 1.0 || first["diff"].(float64) <= 0.8 {
		t.Errorf("rates = %v", first)
	}
	for _, x := range tags {
		if x.(map[string]any)["term"] == "kitchen" && x.(map[string]any)["diff"].(float64) != 0 {
			t.Errorf("kitchen is in every listing, diff must be 0: %v", x)
		}
	}
	words := map[string]map[string]any{}
	for _, w := range m["results"].(map[string]any)["title_words"].([]any) {
		words[w.(map[string]any)["term"].(string)] = w.(map[string]any)
	}
	if w := words["stonewashed"]; w == nil || w["diff"].(float64) != 1.0 || w["top_count"].(float64) != 3 {
		t.Errorf("stonewashed = %v, want diff 1.0 with top_count 3", w)
	}
	if w := words["plain"]; w == nil || w["diff"].(float64) != -1.0 {
		t.Errorf("plain = %v, want diff -1.0 (only in the rest)", w)
	}
	if w := words["apron"]; w == nil || w["diff"].(float64) != 0 {
		t.Errorf("apron = %v, want diff 0", w)
	}
	// A high min-support drops rare terms.
	out, _ = mktRunCLI(t, "market", "tags", "--min-support", "5")
	for _, x := range mktDecode(t, out)["results"].(map[string]any)["tags"].([]any) {
		if x.(map[string]any)["term"] == "stonewash" {
			t.Error("stonewash has support 4 and must be dropped at --min-support 5")
		}
	}
}

func TestMarketTagsUsesDemandWhenAvailable(t *testing.T) {
	mktTestEnv(t)
	db := mktOpenFixtureStore(t)
	d := db.DB()
	taken := time.Now()
	snap := mktFxSnapshot(t, d, taken, []string{"linen apron"}, 1, "complete", mktFxProg{reviewsDone: []int64{1000000001, 1000000002, 1000000003, 1000000004}})
	for i := 1; i <= 4; i++ {
		tag := "plain"
		if i == 4 {
			tag = "bestseller" // worst position but highest demand
		}
		mktFxResult(t, d, snap, mktFxListing{term: "linen apron", pos: i, id: int64(1000000000 + i), shop: int64(5000 + i), title: "Apron", price: 2400, tags: []string{tag}})
	}
	mktFxReviews(t, d, snap, 1000000004, 9, taken.Add(-time.Hour))
	db.Close()
	out, err := mktRunCLI(t, "market", "tags", "--min-support", "1")
	if err != nil {
		t.Fatal(err)
	}
	m := mktDecode(t, out)
	if m["results"].(map[string]any)["basis"] != "demand" {
		t.Fatalf("basis = %v, want demand", m["results"].(map[string]any)["basis"])
	}
	if tags := m["results"].(map[string]any)["tags"].([]any); tags[0].(map[string]any)["term"] != "bestseller" {
		t.Errorf("top tag = %v, want bestseller (demand beats position)", tags[0])
	}
}

func TestMarketTagsEmptyStoreExitsZero(t *testing.T) {
	mktTestEnv(t)
	out, err := mktRunCLI(t, "market", "tags")
	if err != nil {
		t.Fatal(err)
	}
	if m := mktDecode(t, out); m["note"] == nil {
		t.Errorf("want a note: %v", m)
	}
}
