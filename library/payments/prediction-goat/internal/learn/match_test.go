// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package learn

import (
	"encoding/json"
	"testing"
)

func TestClassifyEntityMatch_Exact_QueryAndResourceShareEntity(t *testing.T) {
	got := ClassifyEntityMatch([]string{"USA"}, []string{"USA", "hosting"})
	if got != EntityMatchExact {
		t.Errorf("want exact, got %q", got)
	}
}

func TestClassifyEntityMatch_Partial_EmptyQueryEntities(t *testing.T) {
	got := ClassifyEntityMatch(nil, []string{"Portugal"})
	if got != EntityMatchPartial {
		t.Errorf("empty-query side: want partial, got %q", got)
	}
}

func TestClassifyEntityMatch_Partial_EmptyResourceEntities(t *testing.T) {
	got := ClassifyEntityMatch([]string{"USA"}, nil)
	if got != EntityMatchPartial {
		t.Errorf("empty-resource side: want partial, got %q", got)
	}
}

func TestClassifyEntityMatch_Mismatch_QueryAndResourceDisjoint(t *testing.T) {
	got := ClassifyEntityMatch([]string{"England"}, []string{"Portugal"})
	if got != EntityMatchMismatch {
		t.Errorf("disjoint entities: want mismatch, got %q", got)
	}
}

func TestClassifyEntityMatch_ExactWhenOneOfManyEntitiesShared(t *testing.T) {
	got := ClassifyEntityMatch([]string{"USA", "World", "Cup"}, []string{"USA", "FIFA"})
	if got != EntityMatchExact {
		t.Errorf("USA shared across both sides: want exact, got %q", got)
	}
}

func TestClassifyEntityMatch_BothEmpty(t *testing.T) {
	got := ClassifyEntityMatch(nil, nil)
	if got != EntityMatchPartial {
		t.Errorf("both empty: want partial (categorical), got %q", got)
	}
}

func TestClassifyEntityMatch_CaseInsensitive(t *testing.T) {
	got := ClassifyEntityMatch([]string{"usa"}, []string{"USA"})
	if got != EntityMatchExact {
		t.Errorf("case-insensitive match: want exact, got %q", got)
	}
}

func TestClassifyEntityMatch_SubstringPermissiveOnResourceSide(t *testing.T) {
	// Resource entity is a compound phrase containing the query entity.
	got := ClassifyEntityMatch([]string{"USA"}, []string{"USA-Mexico-Canada"})
	if got != EntityMatchExact {
		t.Errorf("substring match on resource: want exact, got %q", got)
	}
}

func TestJaccard_IdenticalSetsReturnOne(t *testing.T) {
	got := Jaccard([]string{"world", "cup"}, []string{"world", "cup"})
	if got != 1.0 {
		t.Errorf("identical sets: want 1.0, got %v", got)
	}
}

func TestJaccard_DisjointSetsReturnZero(t *testing.T) {
	got := Jaccard([]string{"world", "cup"}, []string{"super", "bowl"})
	if got != 0.0 {
		t.Errorf("disjoint: want 0, got %v", got)
	}
}

func TestJaccard_PartialOverlap(t *testing.T) {
	// Three tokens each, two shared -> 2 / (3+3-2) = 0.5.
	got := Jaccard([]string{"a", "b", "c"}, []string{"b", "c", "d"})
	if got < 0.499 || got > 0.501 {
		t.Errorf("partial overlap: want ~0.5, got %v", got)
	}
}

func TestJaccard_CaseInsensitive(t *testing.T) {
	got := Jaccard([]string{"Portugal"}, []string{"portugal"})
	if got != 1.0 {
		t.Errorf("case-insensitive: want 1.0, got %v", got)
	}
}

func TestJaccard_EmptyOnEitherSideReturnsZero(t *testing.T) {
	if Jaccard(nil, []string{"a"}) != 0 {
		t.Errorf("empty a side should be 0")
	}
	if Jaccard([]string{"a"}, nil) != 0 {
		t.Errorf("empty b side should be 0")
	}
}

func TestResourceEntities_KalshiMarket_TitleSubtitleTicker(t *testing.T) {
	data, _ := json.Marshal(map[string]any{
		"title":         "FIFA Men's World Cup 2026",
		"yes_sub_title": "USA",
		"ticker":        "KXMENWORLDCUP-26-US",
	})
	got := ResourceEntities("kalshi_markets", data)
	// We expect both the entity USA and the ticker KXMENWORLDCUP-26-US.
	if !contains(got, "USA") {
		t.Errorf("expected USA in resource entities; got %v", got)
	}
	if !contains(got, "KXMENWORLDCUP-26-US") {
		t.Errorf("expected ticker in resource entities; got %v", got)
	}
}

func TestResourceEntities_PolymarketMarket_QuestionAndSlug(t *testing.T) {
	data, _ := json.Marshal(map[string]any{
		"question": "Will Portugal win the 2026 FIFA World Cup?",
		"slug":     "will-portugal-win-the-2026-fifa-world-cup-912",
	})
	got := ResourceEntities("markets", data)
	if !contains(got, "Portugal") {
		t.Errorf("expected Portugal entity from question; got %v", got)
	}
	if !contains(got, "will-portugal-win-the-2026-fifa-world-cup-912") {
		t.Errorf("expected slug treated as ticker; got %v", got)
	}
}

func TestResourceEntities_KalshiEvent_TitleAndTicker(t *testing.T) {
	data, _ := json.Marshal(map[string]any{
		"title":         "2026 Men's World Cup Winner",
		"event_ticker":  "KXMENWORLDCUP-26",
		"series_ticker": "KXMENWORLDCUP",
	})
	got := ResourceEntities("kalshi_events", data)
	if !contains(got, "KXMENWORLDCUP-26") {
		t.Errorf("expected event_ticker as ticker; got %v", got)
	}
}

func TestResourceEntities_EmptyOnUnknownType(t *testing.T) {
	got := ResourceEntities("unknown_type", []byte(`{"title":"X"}`))
	if got != nil {
		t.Errorf("unknown type should produce nil; got %v", got)
	}
}

func TestResourceEntities_EmptyDataReturnsNil(t *testing.T) {
	if got := ResourceEntities("markets", nil); got != nil {
		t.Errorf("nil data should return nil; got %v", got)
	}
	if got := ResourceEntities("markets", []byte{}); got != nil {
		t.Errorf("empty data should return nil; got %v", got)
	}
}

func TestIsKalshiParentTicker(t *testing.T) {
	cases := []struct {
		ticker string
		parent bool
	}{
		{"KXMENWORLDCUP-26", true},        // event-level (one hyphen)
		{"KXMENWORLDCUP", true},           // series-level (zero hyphens)
		{"KXMENWORLDCUP-26-US", false},    // market-level (two hyphens)
		{"KXMENWORLDCUP-26-PT", false},    // market-level
		{"KXPRES24-DJT", true},            // single-hyphen ticker
		{"will-portugal-win-...", false}, // not a Kalshi ticker
		{"", false},
	}
	for _, tc := range cases {
		got := IsKalshiParentTicker(tc.ticker)
		if got != tc.parent {
			t.Errorf("IsKalshiParentTicker(%q) = %v, want %v", tc.ticker, got, tc.parent)
		}
	}
}

func TestParseStoredEntities(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"null", nil},
		{"[]", nil},
		{`["Portugal"]`, []string{"Portugal"}},
		{`["USA","Mexico"]`, []string{"USA", "Mexico"}},
	}
	for _, tc := range cases {
		got, err := ParseStoredEntities(tc.in)
		if err != nil {
			t.Errorf("ParseStoredEntities(%q): %v", tc.in, err)
			continue
		}
		if len(got) != len(tc.want) {
			t.Errorf("ParseStoredEntities(%q) len = %d, want %d (got=%v)", tc.in, len(got), len(tc.want), got)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("ParseStoredEntities(%q)[%d] = %q, want %q", tc.in, i, got[i], tc.want[i])
			}
		}
	}
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
