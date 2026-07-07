// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

// playbooks_test.go covers the in-process Playbook types and
// ResolveSlots' entity-only candidate pool contract. Ported from ESPN
// PR #851 HEAD 9bb0a40a, adapted for prediction-goat's country-ISO
// canonical seeds (no NBA teams here).
//
// PATCH(learn-loop-backport U6): part of the ESPN learn-loop cascade
// backport. See docs/plans/2026-05-25-001-feat-prediction-goat-learn-
// loop-backport-plan.md.

package learn

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestParsePlaybook_HappyPath(t *testing.T) {
	t.Parallel()
	raw := []byte(`{
		"query_family_examples": ["what are the odds $COUNTRY wins"],
		"steps": [
			{"cmd": "kalshi series get KXMENWORLDCUP-26", "purpose": "series object"},
			{"client_side": "rank_by", "args": {"stats": "$STATS"}}
		],
		"entity_slots": ["$COUNTRY", "$STATS"],
		"expected_tool_calls": 3
	}`)
	p, err := ParsePlaybook(raw, "inline-test")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(p.Steps) != 2 {
		t.Errorf("want 2 steps, got %d", len(p.Steps))
	}
	if p.Steps[0].Cmd == "" {
		t.Errorf("first step should have cmd")
	}
	if p.Steps[1].ClientSide != "rank_by" {
		t.Errorf("second step should be client_side rank_by, got %q", p.Steps[1].ClientSide)
	}
	if p.ExpectedToolCalls != 3 {
		t.Errorf("expected_tool_calls = %d, want 3", p.ExpectedToolCalls)
	}
}

func TestParsePlaybookFile_HappyPath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "p.json")
	if err := os.WriteFile(path, []byte(`{"steps":[{"cmd":"x"}],"entity_slots":["$X"]}`), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	p, err := ParsePlaybookFile(path)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(p.Steps) != 1 || p.Steps[0].Cmd != "x" {
		t.Errorf("unexpected: %+v", p)
	}
}

func TestParsePlaybookFile_NonExistent(t *testing.T) {
	t.Parallel()
	_, err := ParsePlaybookFile("/definitely/does/not/exist.json")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestParsePlaybook_Malformed(t *testing.T) {
	t.Parallel()
	_, err := ParsePlaybook([]byte(`{not json`), "bad")
	if err == nil {
		t.Fatal("expected parse error on malformed JSON")
	}
}

func TestParsePlaybook_Empty(t *testing.T) {
	t.Parallel()
	_, err := ParsePlaybook([]byte(``), "empty")
	if err == nil {
		t.Fatal("expected error for empty input")
	}
}

func TestMarshalPlaybook_RoundTrip(t *testing.T) {
	t.Parallel()
	orig := Playbook{
		QueryFamilyExamples: []string{"odds $COUNTRY wins"},
		Steps: []PlaybookStep{
			{Cmd: "kalshi markets list --series KXMENWORLDCUP-26", Purpose: "list markets"},
			{ClientSide: "filter", Args: map[string]any{"by": "country"}},
		},
		EntitySlots:       []string{"$COUNTRY"},
		ExpectedToolCalls: 3,
	}
	enc, err := MarshalPlaybook(orig)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	decoded, err := ParsePlaybook([]byte(enc), "roundtrip")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(decoded.Steps) != len(orig.Steps) {
		t.Fatalf("step count: got %d want %d", len(decoded.Steps), len(orig.Steps))
	}
	if decoded.Steps[0].Cmd != orig.Steps[0].Cmd {
		t.Errorf("step[0].Cmd: got %q want %q", decoded.Steps[0].Cmd, orig.Steps[0].Cmd)
	}
	if decoded.Steps[1].ClientSide != orig.Steps[1].ClientSide {
		t.Errorf("step[1].ClientSide: got %q want %q", decoded.Steps[1].ClientSide, orig.Steps[1].ClientSide)
	}
	if decoded.EntitySlots[0] != orig.EntitySlots[0] {
		t.Errorf("entity_slots[0]: got %q want %q", decoded.EntitySlots[0], orig.EntitySlots[0])
	}
	if decoded.ExpectedToolCalls != orig.ExpectedToolCalls {
		t.Errorf("expected_tool_calls: got %d want %d", decoded.ExpectedToolCalls, orig.ExpectedToolCalls)
	}
	if decoded.QueryFamilyExamples[0] != orig.QueryFamilyExamples[0] {
		t.Errorf("query_family_examples[0]: got %q want %q",
			decoded.QueryFamilyExamples[0], orig.QueryFamilyExamples[0])
	}
}

func TestQueryFamily_UsesNonEntityNormalized(t *testing.T) {
	t.Parallel()
	// QueryFamily is defined as the NonEntityNormalized string. Verify
	// that contract holds across a non-trivial NormalizedQuery built by
	// hand (so we don't depend on the extractor's stopword choices).
	normalized := NormalizedQuery{
		Entities:            []string{"portugal"},
		NonEntityNormalized: "cup portugal world",
	}
	got := QueryFamily(normalized)
	if got != normalized.NonEntityNormalized {
		t.Errorf("QueryFamily = %q, want NonEntityNormalized %q", got, normalized.NonEntityNormalized)
	}
}

func TestResolveSlots_SingleEntity(t *testing.T) {
	t.Parallel()
	db := openRecallCanonicalDB(t)
	seedCanonical(t, db, "country", "Portugal", []string{"Portugal", "PT", "portugal"})

	p := Playbook{
		EntitySlots: []string{"$COUNTRY"},
		Steps:       []PlaybookStep{{Cmd: "polymarket siblings will-{country.canonical}-win"}},
	}
	cfg := DefaultPredictionGoatConfig()
	normalized := Normalize("odds Portugal wins the world cup", cfg)
	resolver := NewCanonicalResolver(context.Background(), db)
	normalized = PromoteEntities(normalized, resolver)

	got := ResolveSlots(p, normalized, resolver)
	if got == nil {
		t.Fatal("ResolveSlots returned nil")
	}
	slot, ok := got["$COUNTRY"]
	if !ok {
		t.Fatalf("$COUNTRY slot missing; got %+v", got)
	}
	if slot["canonical"] != "Portugal" {
		t.Errorf("canonical = %v, want Portugal", slot["canonical"])
	}
}

func TestResolveSlots_MultiEntity(t *testing.T) {
	t.Parallel()
	db := openRecallCanonicalDB(t)
	seedCanonical(t, db, "country", "Portugal", []string{"Portugal", "PT", "portugal"})
	seedCanonical(t, db, "country", "Brazil", []string{"Brazil", "BR", "brazil"})

	p := Playbook{
		EntitySlots: []string{"$A", "$B"},
		Steps:       []PlaybookStep{{Cmd: "compare {country.canonical.a} {country.canonical.b}"}},
	}
	cfg := DefaultPredictionGoatConfig()
	normalized := Normalize("Portugal vs Brazil tonight", cfg)
	resolver := NewCanonicalResolver(context.Background(), db)
	normalized = PromoteEntities(normalized, resolver)

	got := ResolveSlots(p, normalized, resolver)
	if got == nil {
		t.Fatal("ResolveSlots returned nil")
	}
	if _, ok := got["$A"]; !ok {
		t.Errorf("$A slot missing; got %+v", got)
	}
	if _, ok := got["$B"]; !ok {
		t.Errorf("$B slot missing; got %+v", got)
	}
	// Both must have bound to distinct tokens.
	if got["$A"]["token"] == got["$B"]["token"] {
		t.Errorf("$A and $B bound to same token: %v", got["$A"]["token"])
	}
}

func TestResolveSlots_UnresolvableSkipped(t *testing.T) {
	t.Parallel()
	db := openRecallCanonicalDB(t)
	// No seeds.

	p := Playbook{
		EntitySlots: []string{"$THING"},
		Steps:       []PlaybookStep{{Cmd: "x"}},
	}
	cfg := DefaultPredictionGoatConfig()
	normalized := Normalize("how is weatherman doing", cfg)
	resolver := NewCanonicalResolver(context.Background(), db)
	got := ResolveSlots(p, normalized, resolver)
	if _, ok := got["$THING"]; ok {
		t.Errorf("unresolvable slot should be absent; got %+v", got)
	}
}

func TestResolveSlots_EmptySlots(t *testing.T) {
	t.Parallel()
	db := openRecallCanonicalDB(t)

	p := Playbook{Steps: []PlaybookStep{{Cmd: "x"}}}
	resolver := NewCanonicalResolver(context.Background(), db)
	cfg := DefaultPredictionGoatConfig()
	got := ResolveSlots(p, Normalize("any query", cfg), resolver)
	if got != nil {
		t.Errorf("empty entity_slots should yield nil map, got %+v", got)
	}
}

// TestResolveSlots_OnlyConsidersEntities guards the Greptile finding
// on PR #851 round 3 (R7): ResolveSlots used to include non-entity
// tokens in the candidate pool. If a token classified as non-entity
// happens to resolve via entity_lookups (a secondary-alias collision
// the extractor didn't promote), the old code would still let it win
// a slot binding meant for a real entity. The fix restricts the pool
// to normalized.Entities so the slot stays bound to the real entity
// rather than the spurious alias.
//
// To exercise the contract without depending on PromoteEntities'
// classification logic, we construct the normalized query directly:
// "portugal" is in Entities, "ppg" is only in NonEntityNormalized,
// and the resolver can resolve BOTH. Pre-fix: "ppg" would have won
// because the candidate pool included non-entity tokens. Post-fix:
// "portugal" is the only candidate and wins the $COUNTRY slot.
func TestResolveSlots_OnlyConsidersEntities(t *testing.T) {
	t.Parallel()
	db := openRecallCanonicalDB(t)
	seedCanonical(t, db, "country", "Portugal", []string{"Portugal", "PT", "portugal"})
	// Register "ppg" as a secondary alias of an unrelated canonical
	// so the resolver returns a hit for it.
	seedCanonical(t, db, "stat_abbrev", "PointsPerGame",
		[]string{"PointsPerGame", "ppg"})

	p := Playbook{
		EntitySlots: []string{"$COUNTRY"},
		Steps:       []PlaybookStep{{Cmd: "polymarket siblings {country.canonical}"}},
	}
	// Hand-build the normalized query so "ppg" stays in
	// NonEntityNormalized (the test of the fix's contract).
	normalized := NormalizedQuery{
		Entities:            []string{"portugal"},
		NonEntityNormalized: "leads ppg who",
	}
	resolver := NewCanonicalResolver(context.Background(), db)
	got := ResolveSlots(p, normalized, resolver)

	slot, ok := got["$COUNTRY"]
	if !ok {
		t.Fatalf("$COUNTRY slot missing; got %+v", got)
	}
	if slot["token"] == "ppg" || slot["canonical"] == "PointsPerGame" {
		t.Errorf("non-entity token 'ppg' wrongly won the $COUNTRY slot; slot=%+v", slot)
	}
	if slot["canonical"] != "Portugal" {
		t.Errorf("$COUNTRY canonical = %v, want Portugal", slot["canonical"])
	}
}

func TestResolveSlots_NilResolver(t *testing.T) {
	t.Parallel()
	p := Playbook{
		EntitySlots: []string{"$X"},
		Steps:       []PlaybookStep{{Cmd: "x"}},
	}
	cfg := DefaultPredictionGoatConfig()
	got := ResolveSlots(p, Normalize("anything", cfg), nil)
	if got != nil {
		t.Errorf("nil resolver should yield nil, got %+v", got)
	}
}
