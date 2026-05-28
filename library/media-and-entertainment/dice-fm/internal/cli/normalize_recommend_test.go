// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"context"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestRecommendShape(t *testing.T) {
	// low-cardinality controlled set -> vocab
	if s := recommendShape(profileStats{Distinct: 8, AliasCollisions: 0, StructuredFrac: 0.0}); s != "vocab" {
		t.Errorf("low-card -> %q, want vocab", s)
	}
	// high-cardinality free-text names with dedup variants -> alias
	if s := recommendShape(profileStats{Distinct: 700, AliasCollisions: 30, StructuredFrac: 0.05}); s != "alias" {
		t.Errorf("high-card names -> %q, want alias", s)
	}
	// structured labels -> attributes
	if s := recommendShape(profileStats{Distinct: 1100, AliasCollisions: 80, StructuredFrac: 0.7}); s != "attributes" {
		t.Errorf("structured -> %q, want attributes", s)
	}
}

// TestProfileField verifies that profileField correctly computes Distinct,
// AliasCollisions, and StructuredFrac from a seeded store.
//
// Fixture design (synthetic, no real tenant data):
//   - "Tier A" and "tier a " differ only by case and trailing whitespace →
//     they collapse to the same lower-trim key → AliasCollisions = 2.
//   - "Pass 2" contains a digit → StructuredFrac > 0.
func TestProfileField(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{
		"tickets": {
			"tk1": `{"id":"tk1","ticketType":{"name":"Tier A"}}`,
			"tk2": `{"id":"tk2","ticketType":{"name":"tier a "}}`, // same key after lower+trim → collision
			"tk3": `{"id":"tk3","ticketType":{"name":"Pass 2"}}`,  // digit → structured signal
		},
	})

	stats, err := profileField(context.Background(), s.DB(), "tickets.ticketType.name")
	if err != nil {
		t.Fatalf("profileField: %v", err)
	}

	if stats.Distinct != 3 {
		t.Errorf("Distinct = %d, want 3", stats.Distinct)
	}
	// "Tier A" and "tier a " share the lower-trim key "tier a" → 2 collisions.
	if stats.AliasCollisions != 2 {
		t.Errorf("AliasCollisions = %d, want 2", stats.AliasCollisions)
	}
	// "Pass 2" matches the structured signal (digit). 1/3 ≈ 0.333.
	if stats.StructuredFrac <= 0 {
		t.Errorf("StructuredFrac = %f, want > 0 (Pass 2 has a digit)", stats.StructuredFrac)
	}
	if stats.StructuredFrac > 0.5 {
		t.Errorf("StructuredFrac = %f, want ≤ 0.5 (only 1 of 3 values is structured)", stats.StructuredFrac)
	}
}

// TestProfileFieldArraySource verifies that array-path sources (events.venues[*].name)
// are also profiled correctly.
func TestProfileFieldArraySource(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{
		"events": {
			"e1": `{"id":"e1","venues":[{"name":"Northside Hall"}]}`,
			"e2": `{"id":"e2","venues":[{"name":"Southside Arena"}]}`,
		},
	})

	stats, err := profileField(context.Background(), s.DB(), "events.venues[*].name")
	if err != nil {
		t.Fatalf("profileField array: %v", err)
	}
	if stats.Distinct != 2 {
		t.Errorf("Distinct = %d, want 2", stats.Distinct)
	}
	if stats.AliasCollisions != 0 {
		t.Errorf("AliasCollisions = %d, want 0", stats.AliasCollisions)
	}
}

// TestDistinctSourceValuesScalarArray verifies that a scalar-array source
// (events.genres[*], where genres is a JSON array of strings) yields the
// individual array elements rather than the whole array as one string.
func TestDistinctSourceValuesScalarArray(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{
		"events": {
			"e1": `{"id":"e1","genres":["a","b"]}`,
			"e2": `{"id":"e2","genres":["b","c"]}`,
		},
	})

	vals, err := distinctSourceValues(context.Background(), s.DB(), "events.genres[*]")
	if err != nil {
		t.Fatalf("distinctSourceValues scalar array: %v", err)
	}
	sort.Strings(vals)
	if want := []string{"a", "b", "c"}; !reflect.DeepEqual(vals, want) {
		t.Errorf("distinct scalar-array values = %v, want %v (individual elements, not the array-as-one-string)", vals, want)
	}
}

// TestDistinctSourceValuesObjectArrayUnchanged is a regression guard ensuring
// the object-array path (events.venues[*].name) still extracts the leaf field
// from each element after scalar-array support was added.
func TestDistinctSourceValuesObjectArrayUnchanged(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{
		"events": {
			"e1": `{"id":"e1","venues":[{"name":"Northside Hall"},{"name":"Southside Arena"}]}`,
			"e2": `{"id":"e2","venues":[{"name":"Southside Arena"}]}`,
		},
	})

	vals, err := distinctSourceValues(context.Background(), s.DB(), "events.venues[*].name")
	if err != nil {
		t.Fatalf("distinctSourceValues object array: %v", err)
	}
	sort.Strings(vals)
	if want := []string{"Northside Hall", "Southside Arena"}; !reflect.DeepEqual(vals, want) {
		t.Errorf("distinct object-array leaf values = %v, want %v", vals, want)
	}
}

// TestValidateSourcePathScalarArray verifies that a scalar-array source with a
// trailing [*] marker (no leaf) passes validation, while an injection-style
// value still errors.
func TestValidateSourcePathScalarArray(t *testing.T) {
	if err := validateSourcePath("events.genres[*]"); err != nil {
		t.Errorf("validateSourcePath(%q): want nil, got %v", "events.genres[*]", err)
	}
	if err := validateSourcePath("events.genres[*]'); DROP TABLE x;--"); err == nil {
		t.Errorf("validateSourcePath: want error for injection value, got nil")
	}
}

// TestProfileFieldEmptySource verifies that a source path with no data returns
// zero stats (not an error).
func TestProfileFieldEmptySource(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{})

	stats, err := profileField(context.Background(), s.DB(), "tickets.ticketType.name")
	if err != nil {
		t.Fatalf("profileField empty store: %v", err)
	}
	if stats.Distinct != 0 {
		t.Errorf("Distinct = %d on empty store, want 0", stats.Distinct)
	}
}

// TestProfileFieldRejectsUnsafePath verifies that injection-style source paths
// return an error and never reach the SQL layer.
func TestProfileFieldRejectsUnsafePath(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{})

	injectionCases := []string{
		"tickets.name'); DROP TABLE x;--",
		"tickets.name' OR '1'='1",
		"tick ets.name",
		"tickets.name\x00evil",
	}
	for _, src := range injectionCases {
		_, err := profileField(context.Background(), s.DB(), src)
		if err == nil {
			t.Errorf("profileField(%q): want error for unsafe path, got nil", src)
		}
		if !strings.Contains(err.Error(), "unsafe source path") {
			t.Errorf("profileField(%q): error %q does not mention 'unsafe source path'", src, err.Error())
		}
	}
}

// TestRunRecommend verifies end-to-end: seeded store → runRecommend → Config.
//
// Assertions:
//   - each profiled entity has a non-empty Shape and Source that matches the
//     candidateSources entry.
//   - Rules, Attributes, Vocab are all empty/zero-length (starter config).
//   - an entity whose source has zero distinct values is absent from the result.
func TestRunRecommend(t *testing.T) {
	s := seedStore(t, map[string]map[string]string{
		"tickets": {
			"tk1": `{"id":"tk1","ticketType":{"name":"Tier A"}}`,
			"tk2": `{"id":"tk2","ticketType":{"name":"tier a "}}`,
			"tk3": `{"id":"tk3","ticketType":{"name":"Pass 2"}}`,
		},
		// Note: no "events" rows → venue/genre/artist sources will have 0 distinct
		// values and must be absent from the result.
	})

	sources := map[string]string{
		"ticket_type": "tickets.ticketType.name",
		"venue":       "events.venues[*].name", // no data → should be skipped
		"genre":       "events.genres",         // no data → should be skipped
	}

	cfg, err := runRecommend(context.Background(), s, sources)
	if err != nil {
		t.Fatalf("runRecommend: %v", err)
	}

	// ticket_type has data → must be present with a non-empty shape.
	tt, ok := cfg.Entities["ticket_type"]
	if !ok {
		t.Fatalf("ticket_type entity missing from result; entities=%v", cfg.Entities)
	}
	if tt.Shape == "" {
		t.Errorf("ticket_type Shape is empty")
	}
	if tt.Source != "tickets.ticketType.name" {
		t.Errorf("ticket_type Source = %q, want %q", tt.Source, "tickets.ticketType.name")
	}
	// Starter config: Rules, Attributes, Vocab must all be empty.
	if len(tt.Rules) != 0 {
		t.Errorf("ticket_type Rules = %v, want empty (starter config)", tt.Rules)
	}
	if len(tt.Attributes) != 0 {
		t.Errorf("ticket_type Attributes = %v, want empty (starter config)", tt.Attributes)
	}
	if len(tt.Vocab) != 0 {
		t.Errorf("ticket_type Vocab = %v, want empty (starter config)", tt.Vocab)
	}

	// venue and genre have no data → must be absent.
	if _, ok := cfg.Entities["venue"]; ok {
		t.Errorf("venue entity present despite no data — should be skipped")
	}
	if _, ok := cfg.Entities["genre"]; ok {
		t.Errorf("genre entity present despite no data — should be skipped")
	}
}
