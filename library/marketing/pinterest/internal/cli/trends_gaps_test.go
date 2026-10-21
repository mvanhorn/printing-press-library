// Copyright 2026 Elizabeth Stone and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/marketing/pinterest/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/marketing/pinterest/internal/store"
)

// TestNovelTrendsGapsHelpWires smoke-tests that the trends gaps command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelTrendsGapsHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"trends", "gaps", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("trends gaps --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "gaps"} {
		if !strings.Contains(help, want) {
			t.Fatalf("trends gaps --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestParseTrendingKeywordsAndRank(t *testing.T) {
	trends, err := parseTrendingKeywords(json.RawMessage(`{"trends":[{"keyword":"meal prep","pct_growth_wow":10},{"keyword":"baking","pct_growth_wow":5},{"keyword":"journal","pct_growth_wow":1}]}`))
	if err != nil || len(trends) != 3 || fieldString(trends[1], "keyword") != "baking" {
		t.Fatalf("parse: %v %v", trends, err)
	}
	empty, err := parseTrendingKeywords(json.RawMessage(`{}`))
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("empty response should give [] not nil: %v %v", empty, err)
	}
	gaps := rankTrendGaps([]trendGap{{Keyword: "a", MatchingPins: 2}, {Keyword: "b"}, {Keyword: "c", MatchingPins: 1}, {Keyword: "d"}})
	order := []string{}
	for _, g := range gaps {
		order = append(order, g.Keyword)
	}
	if strings.Join(order, ",") != "b,d,a,c" {
		t.Fatalf("rank order = %v, want uncovered first then trend order", order)
	}
}

func TestTrendsGapsCountsLocalCoverage(t *testing.T) {
	testenv.Isolate(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/trends/keywords/US/top/growing") {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"trends":[{"keyword":"meal prep","pct_growth_wow":10},{"keyword":"quantum origami","pct_growth_wow":3}]}`))
	}))
	defer srv.Close()
	t.Setenv("PINTEREST_BASE_URL", srv.URL)
	t.Setenv("PINTEREST_ACCESS_TOKEN", "test-token")
	db := seedPinterestStore(t, nil, []string{
		`{"id":"p1","board_id":"b1","title":"Meal prep basics","description":"d"}`,
		`{"id":"p2","board_id":"b1","title":"Morning journal","description":"meal prep habit"}`,
	})
	// A board Pin (stored as boards_pins) must count, and a duplicate of an
	// owned Pin must be counted once.
	func() {
		st, err := store.Open(db)
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		defer st.Close()
		for _, bp := range []string{
			`{"id":"p3","boards_id":"b2","board_id":"b2","title":"Shared board pin","alt_text":"a meal-prep journal"}`,
			`{"id":"p1","boards_id":"b1","board_id":"b1","title":"Meal prep basics","description":"d"}`,
		} {
			if err := st.UpsertBoardsPins(json.RawMessage(bp)); err != nil {
				t.Fatalf("seed board pin: %v", err)
			}
		}
	}()
	out, err := runPinterestCmd(t, "", "trends", "gaps", "--region", "US", "--db", db, "--json")
	if err != nil {
		t.Fatalf("gaps: %v\n%s", err, out)
	}
	var gaps []trendGap
	if err := json.Unmarshal([]byte(out), &gaps); err != nil {
		t.Fatalf("parse: %v\n%s", err, out)
	}
	if len(gaps) != 2 || gaps[0].Keyword != "quantum origami" || gaps[0].MatchingPins != 0 {
		t.Fatalf("uncovered keyword should come first: %+v", gaps)
	}
	if gaps[1].Keyword != "meal prep" || gaps[1].MatchingPins != 3 || len(gaps[1].SamplePinIDs) != 3 {
		t.Fatalf("meal prep coverage wrong: %+v", gaps[1])
	}
}

func TestKeywordCoverageMatchesAllWords(t *testing.T) {
	pins := []pinText{newPinText("a", "Meal prep basics"), newPinText("b", "prep letters"), newPinText("c", "meal-prep journal"), newPinText("d", "Party ideas")}
	n, ids := keywordCoverage("Meal Prep", pins)
	if n != 2 || len(ids) != 2 || ids[0] != "a" || ids[1] != "c" {
		t.Fatalf("coverage = %d %v", n, ids)
	}
	if n, _ := keywordCoverage("quantum origami", pins); n != 0 {
		t.Fatalf("unrelated keyword matched %d pins", n)
	}
	if n, _ := keywordCoverage("art", pins); n != 0 {
		t.Fatalf("word fragment matched %d pins (art must not match party)", n)
	}
}
