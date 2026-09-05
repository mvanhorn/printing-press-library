package store

import (
	"context"
	"testing"
	"time"
)

func seedNCCPLSearchStore(t *testing.T) *Store {
	t.Helper()
	ctx := context.Background()
	s := newNCCPLTestStore(t)
	seed := []struct {
		resource string
		date     string
		rows     []NCCPLRow
	}{
		{"mts", "2026-09-03", []NCCPLRow{
			{Key: "HUBC", Payload: `{"symbol":"HUBC","open_position":1200}`},
			{Key: "OGDC", Payload: `{"symbol":"OGDC","open_position":900}`},
		}},
		{"mts", "2026-09-04", []NCCPLRow{
			{Key: "HUBC", Payload: `{"symbol":"HUBC","open_position":1350}`},
		}},
		{"slb", "2026-09-04", []NCCPLRow{
			{Key: "SLB|001", Payload: `{"symbol":"HUBC","net_open":40}`},
			{Key: "SLB|002", Payload: `{"symbol":"PSO","net_open":10}`},
		}},
	}
	for _, batch := range seed {
		if err := SaveNCCPLDate(ctx, s, batch.resource, batch.date, batch.rows, time.Now()); err != nil {
			t.Fatalf("seed %s %s: %v", batch.resource, batch.date, err)
		}
	}
	return s
}

func TestSearchNCCPLObservationsSpansResources(t *testing.T) {
	ctx := context.Background()
	s := seedNCCPLSearchStore(t)

	hits, err := SearchNCCPLObservations(ctx, s, "hubc", NCCPLSearchOptions{})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(hits) != 3 {
		t.Fatalf("got %d hits, want 3 across mts and slb", len(hits))
	}
	// Newest settlement date first, so a caller reading the head of the list
	// sees the most recent session rather than an arbitrary one.
	if hits[0].Date != "2026-09-04" {
		t.Errorf("first hit date = %q, want 2026-09-04", hits[0].Date)
	}
	for _, hit := range hits {
		if hit.ObservedAt == "" {
			t.Errorf("hit %s/%s/%s lost its observed_at vintage", hit.Resource, hit.Date, hit.Key)
		}
	}
}

// A hit on the row key is what makes a row about a symbol; a hit inside the
// payload only says the symbol is mentioned. Conflating them would let a search
// for a symbol claim rows that merely reference it.
func TestSearchNCCPLObservationsFlagsKeyMatches(t *testing.T) {
	ctx := context.Background()
	s := seedNCCPLSearchStore(t)

	hits, err := SearchNCCPLObservations(ctx, s, "HUBC", NCCPLSearchOptions{Resources: []string{"slb"}})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("got %d hits, want 1", len(hits))
	}
	if hits[0].MatchedKey {
		t.Error("slb row key SLB|001 does not contain HUBC; matched_key must be false")
	}

	hits, err = SearchNCCPLObservations(ctx, s, "HUBC", NCCPLSearchOptions{Resources: []string{"mts"}})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(hits) != 2 {
		t.Fatalf("got %d mts hits, want 2", len(hits))
	}
	for _, hit := range hits {
		if !hit.MatchedKey {
			t.Errorf("mts row keyed %q must report matched_key", hit.Key)
		}
	}
}

func TestSearchNCCPLObservationsHonoursBounds(t *testing.T) {
	ctx := context.Background()
	s := seedNCCPLSearchStore(t)

	hits, err := SearchNCCPLObservations(ctx, s, "hubc", NCCPLSearchOptions{From: "2026-09-04"})
	if err != nil {
		t.Fatalf("search from: %v", err)
	}
	if len(hits) != 2 {
		t.Fatalf("got %d hits from 2026-09-04, want 2", len(hits))
	}

	hits, err = SearchNCCPLObservations(ctx, s, "hubc", NCCPLSearchOptions{To: "2026-09-03"})
	if err != nil {
		t.Fatalf("search to: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("got %d hits to 2026-09-03, want 1", len(hits))
	}

	hits, err = SearchNCCPLObservations(ctx, s, "hubc", NCCPLSearchOptions{Limit: 1})
	if err != nil {
		t.Fatalf("search limit: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("got %d hits under --limit 1, want 1", len(hits))
	}
}

// A query that matches nothing must return nothing. Widening it into "closest
// match" would put a row under a date or a symbol it was never observed for.
func TestSearchNCCPLObservationsReturnsNothingForAMiss(t *testing.T) {
	ctx := context.Background()
	s := seedNCCPLSearchStore(t)

	hits, err := SearchNCCPLObservations(ctx, s, "NOSUCHSYMBOL", NCCPLSearchOptions{})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(hits) != 0 {
		t.Fatalf("got %d hits for an absent symbol, want 0", len(hits))
	}

	// A date the store never fetched is absent, not interpolated from its
	// neighbours.
	hits, err = SearchNCCPLObservations(ctx, s, "hubc", NCCPLSearchOptions{From: "2026-09-05", To: "2026-09-05"})
	if err != nil {
		t.Fatalf("search unfetched date: %v", err)
	}
	if len(hits) != 0 {
		t.Fatalf("got %d hits for an unfetched session, want 0", len(hits))
	}
}

// LIKE wildcards typed by a caller are literal text, not pattern syntax.
func TestSearchNCCPLObservationsEscapesLikeWildcards(t *testing.T) {
	ctx := context.Background()
	s := seedNCCPLSearchStore(t)

	hits, err := SearchNCCPLObservations(ctx, s, "%", NCCPLSearchOptions{})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(hits) != 0 {
		t.Fatalf("a literal %% matched %d rows; wildcards must be escaped", len(hits))
	}

	if _, err := SearchNCCPLObservations(ctx, s, "   ", NCCPLSearchOptions{}); err == nil {
		t.Error("a blank query must be rejected rather than matching every row")
	}
}
