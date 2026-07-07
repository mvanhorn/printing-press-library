// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strings"
	"testing"
)

// TestYesPercent covers the JSON apples-to-apples helper that pairs
// the canonical 0-1 yesProbability float with a rounded 0-100 percent
// companion for cross-venue display. The same input shape produces the
// same percent regardless of venue.
func TestYesPercent(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   float64
		want float64
	}{
		{"zero stays zero", 0, 0},
		{"polymarket-style 0.062", 0.062, 6.2},
		{"kalshi-style 0.78", 0.78, 78},
		{"full 1.0", 1.0, 100},
		{"rounds half up at 0.0625", 0.0625, 6.3},
		{"rounds half down at 0.0624", 0.0624, 6.2},
		{"tiny 0.001", 0.001, 0.1},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := yesPercent(tc.in)
			if got != tc.want {
				t.Errorf("yesPercent(%v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

// TestTopicFTSQueryOrJoin locks the OR-mode behavior. Multi-token queries
// must OR-join so a row matching either term is a candidate; BM25 then
// favors rows matching more terms. Single-token queries are unchanged.
func TestTopicFTSQueryOrJoin(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want string
	}{
		{"kanye", `"kanye"`},
		{"kanye west", `"kanye" OR "west"`},
		{"kanye-west", `"kanye" OR "west"`},
		{"NBA Western Conference Finals", `"NBA" OR "Western" OR "Conference" OR "Finals"`},
		{"", `""`},     // Empty input returns a sentinel that matches nothing rather than triggering an FTS5 parse error.
		{"---", `""`}, // All-separator input collapses to the same sentinel.
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			got := topicFTSQuery(tc.in)
			if got != tc.want {
				t.Errorf("topicFTSQuery(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestTopicQueryTokens covers the helper that drives force-include for
// query-named outcomes. Tokens drop single-letter noise, lowercase
// everything, and split on the same separators FTS does.
func TestTopicQueryTokens(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want []string
	}{
		{"2026 World Cup", []string{"2026", "world", "cup"}},
		{"OKC Spurs", []string{"okc", "spurs"}},
		{"a vs b", []string{"vs"}}, // single-letter dropped
		{"", nil},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			got := topicQueryTokens(tc.in)
			if len(got) != len(tc.want) {
				t.Errorf("topicQueryTokens(%q) = %v, want %v", tc.in, got, tc.want)
				return
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("topicQueryTokens(%q)[%d] = %q, want %q", tc.in, i, got[i], tc.want[i])
				}
			}
		})
	}
}

// TestContainsWord locks the whole-word match used by force-include.
// "USA" should match "Will USA win" but not "Causes" or "USAir". Lowercase
// input is the contract; the helper does not re-normalize.
func TestContainsWord(t *testing.T) {
	t.Parallel()
	cases := []struct {
		s, tok string
		want   bool
	}{
		{"will usa win the 2026 fifa world cup", "usa", true},
		{"causes problems", "usa", false},
		{"usair flight 232", "usa", false},
		{"the united states", "states", true},
		{"the united states.", "states", true}, // trailing punctuation
		{"", "usa", false},
		{"will", "", false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.s+"/"+tc.tok, func(t *testing.T) {
			t.Parallel()
			got := containsWord(tc.s, tc.tok)
			if got != tc.want {
				t.Errorf("containsWord(%q, %q) = %v, want %v", tc.s, tc.tok, got, tc.want)
			}
		})
	}
}

// TestForceIncludeNamedOutcomes verifies that a hit whose title contains
// a query token but didn't make the truncated result set gets appended.
// This is the USA-in-World-Cup case: the FTS+vol rerank dropped the host
// nation's market but it is in the search pool, so force-include must add
// it back.
func TestForceIncludeNamedOutcomes(t *testing.T) {
	t.Parallel()
	results := []topicHit{
		{Source: "polymarket", Kind: "market", ID: "will-france-win-the-2026-fifa-world-cup", Title: "Will France win the 2026 FIFA World Cup?"},
		{Source: "polymarket", Kind: "market", ID: "will-spain-win-the-2026-fifa-world-cup", Title: "Will Spain win the 2026 FIFA World Cup?"},
	}
	pool := []topicHit{
		{Source: "polymarket", Kind: "market", ID: "will-usa-win-the-2026-fifa-world-cup-467", Title: "Will USA win the 2026 FIFA World Cup?"},
		{Source: "polymarket", Kind: "market", ID: "will-curacao-win-the-2026-fifa-world-cup", Title: "Will Curacao win the 2026 FIFA World Cup?"},
	}
	tokens := []string{"2026", "world", "cup", "usa"}
	got := forceIncludeNamedOutcomes(results, pool, nil, tokens, 100)
	hasUSA := false
	for _, h := range got {
		if strings.Contains(strings.ToLower(h.Title), "usa") {
			hasUSA = true
		}
	}
	if !hasUSA {
		t.Errorf("expected USA hit to be force-included, got titles: %v", titlesOf(got))
	}
}

// TestForceIncludeNamedOutcomes_NoDuplicate verifies the function never
// appends a hit already present in the result set, even when multiple
// tokens match the same title.
func TestForceIncludeNamedOutcomes_NoDuplicate(t *testing.T) {
	t.Parallel()
	results := []topicHit{
		{Source: "polymarket", Kind: "market", ID: "will-usa-win-the-2026-fifa-world-cup-467", Title: "Will USA win the 2026 FIFA World Cup?"},
	}
	pool := []topicHit{
		{Source: "polymarket", Kind: "market", ID: "will-usa-win-the-2026-fifa-world-cup-467", Title: "Will USA win the 2026 FIFA World Cup?"},
	}
	tokens := []string{"usa", "world", "cup"}
	got := forceIncludeNamedOutcomes(results, pool, nil, tokens, 100)
	if len(got) != 1 {
		t.Errorf("expected 1 result (no duplicate), got %d: %v", len(got), titlesOf(got))
	}
}

// TestSortHitsByScore confirms higher rankScore comes first and SortStable
// preserves the SQL order on ties.
func TestSortHitsByScore(t *testing.T) {
	t.Parallel()
	hits := []topicHit{
		{ID: "a", rankScore: 1.0},
		{ID: "b", rankScore: 5.0},
		{ID: "c", rankScore: 5.0},
		{ID: "d", rankScore: 2.0},
	}
	sortHitsByScore(hits)
	wantOrder := []string{"b", "c", "d", "a"}
	for i, h := range hits {
		if h.ID != wantOrder[i] {
			t.Errorf("sortHitsByScore[%d] = %q, want %q (full: %v)", i, h.ID, wantOrder[i], idsOf(hits))
		}
	}
}

// TestFilterPolyActiveOnly ensures closed/inactive Polymarket hits are
// dropped while active and Kalshi hits pass through untouched.
func TestFilterPolyActiveOnly(t *testing.T) {
	t.Parallel()
	hits := []topicHit{
		{Source: "polymarket", Kind: "market", ID: "active", Status: "active"},
		{Source: "polymarket", Kind: "market", ID: "closed", Status: "closed"},
		{Source: "polymarket", Kind: "market", ID: "inactive", Status: "inactive"},
		{Source: "polymarket", Kind: "market", ID: "noStatus"}, // empty status = pass
		{Source: "kalshi", Kind: "series", ID: "KXNBAWEST"},   // kalshi unchanged
	}
	got := filterPolyActiveOnly(hits)
	want := []string{"active", "noStatus", "KXNBAWEST"}
	if len(got) != len(want) {
		t.Fatalf("filterPolyActiveOnly len = %d, want %d (got: %v)", len(got), len(want), idsOf(got))
	}
	for i, h := range got {
		if h.ID != want[i] {
			t.Errorf("filterPolyActiveOnly[%d] = %q, want %q", i, h.ID, want[i])
		}
	}
}

// TestIsUntradedKalshi locks the three-signal untraded check: no last
// price, no 24h volume, and a yes-ask + no-ask that overshoots $1.00 by
// more than 10c. All three must be true.
func TestIsUntradedKalshi(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name                              string
		yesAsk, noAsk, lastPrice, volume24h float64
		want                              bool
	}{
		{"untraded default 17c ask", 0.17, 1.00, 0, 0, true},
		{"liquid OKC market", 0.78, 0.23, 0.78, 371000, false},
		{"has volume = traded", 0.17, 1.00, 0, 100, false},
		{"has last price = traded", 0.17, 1.00, 0.10, 0, false},
		{"tight book (1c spread) = traded", 0.50, 0.51, 0, 0, false},
		{"both asks zero = traded (no quote)", 0, 0, 0, 0, false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := isUntradedKalshi(tc.yesAsk, tc.noAsk, tc.lastPrice, tc.volume24h)
			if got != tc.want {
				t.Errorf("isUntradedKalshi(%v, %v, %v, %v) = %v, want %v", tc.yesAsk, tc.noAsk, tc.lastPrice, tc.volume24h, got, tc.want)
			}
		})
	}
}

// TestRoundDelta verifies the signed percent helper used by mispriced to
// emit deltaPercent alongside the canonical 0-1 delta float.
func TestRoundDelta(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, want float64
	}{
		{0.012, 1.2},
		{-0.012, -1.2},
		{0.0625, 6.3},
		{-0.0625, -6.3},
		{0, 0},
		{0.10, 10.0},
	}
	for _, tc := range cases {
		tc := tc
		t.Run("", func(t *testing.T) {
			t.Parallel()
			got := roundDelta(tc.in)
			if got != tc.want {
				t.Errorf("roundDelta(%v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

// TestTopicRowsUntradedDisplay locks the text-mode display: untraded
// markets show "untraded" in the YES column instead of a misleading
// platform-default percent.
func TestTopicRowsUntradedDisplay(t *testing.T) {
	t.Parallel()
	hits := []topicHit{
		{Source: "kalshi", Kind: "market", ID: "KXNBADRAFTPICK-26-14-X", Title: "Pick 14 default", YesProbability: 0.17, Untraded: true},
		{Source: "kalshi", Kind: "market", ID: "KXNBAWEST-26-OKC", Title: "OKC WCF", YesProbability: 0.78, Untraded: false},
	}
	rows := topicRows(hits)
	if rows[0][3] != "untraded" {
		t.Errorf("untraded row YES cell = %q, want %q", rows[0][3], "untraded")
	}
	if rows[1][3] != "78.0%" {
		t.Errorf("traded row YES cell = %q, want %q", rows[1][3], "78.0%")
	}
}

func titlesOf(hits []topicHit) []string {
	out := make([]string, len(hits))
	for i, h := range hits {
		out[i] = h.Title
	}
	return out
}

func idsOf(hits []topicHit) []string {
	out := make([]string, len(hits))
	for i, h := range hits {
		out[i] = h.ID
	}
	return out
}
