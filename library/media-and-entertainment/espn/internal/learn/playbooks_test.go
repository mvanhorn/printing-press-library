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
		"query_family_examples": ["how did $TEAM end the season"],
		"steps": [
			{"cmd": "teams basketball nba {team.id}", "purpose": "team object"},
			{"client_side": "rank_by", "args": {"stats": "$STATS"}}
		],
		"entity_slots": ["$TEAM", "$STATS"],
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
		Steps:             []PlaybookStep{{Cmd: "teams basketball nba {team.id}"}},
		EntitySlots:       []string{"$TEAM"},
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
	if len(decoded.Steps) != 1 || decoded.Steps[0].Cmd != orig.Steps[0].Cmd {
		t.Errorf("roundtrip mismatch: %+v vs %+v", decoded, orig)
	}
}

func TestResolveSlots_SingleEntity(t *testing.T) {
	t.Parallel()
	db := openCanonicalTestDB(t)
	seedCanonical(t, db, "nba_team", "Detroit Pistons",
		[]string{"Detroit Pistons", "Pistons", "DET"})

	p := Playbook{
		EntitySlots: []string{"$TEAM"},
		Steps:       []PlaybookStep{{Cmd: "teams basketball nba {team.id}"}},
	}
	cfg := espnLikeConfig()
	normalized := Normalize("how did pistons end the season", cfg)
	resolver := NewCanonicalResolver(context.Background(), db)
	normalized = PromoteEntities(normalized, resolver)

	got := ResolveSlots(p, normalized, resolver)
	if got == nil {
		t.Fatal("ResolveSlots returned nil")
	}
	slot, ok := got["$TEAM"]
	if !ok {
		t.Fatalf("$TEAM slot missing; got %+v", got)
	}
	if slot["canonical"] != "Detroit Pistons" {
		t.Errorf("canonical = %v, want Detroit Pistons", slot["canonical"])
	}
}

func TestResolveSlots_MultiEntity(t *testing.T) {
	t.Parallel()
	db := openCanonicalTestDB(t)
	seedCanonical(t, db, "mlb_team", "Seattle Mariners", []string{"Mariners", "SEA"})
	seedCanonical(t, db, "mlb_team", "New York Mets", []string{"Mets", "NYM"})

	p := Playbook{
		EntitySlots: []string{"$HOME", "$AWAY"},
		Steps:       []PlaybookStep{{Cmd: "h2h {team.abbr.home} {team.abbr.away}"}},
	}
	cfg := espnLikeConfig()
	normalized := Normalize("Mariners vs Mets tonight", cfg)
	resolver := NewCanonicalResolver(context.Background(), db)
	normalized = PromoteEntities(normalized, resolver)

	got := ResolveSlots(p, normalized, resolver)
	if _, ok := got["$HOME"]; !ok {
		t.Errorf("$HOME slot missing; got %+v", got)
	}
	if _, ok := got["$AWAY"]; !ok {
		t.Errorf("$AWAY slot missing; got %+v", got)
	}
}

func TestResolveSlots_UnresolvableSkipped(t *testing.T) {
	t.Parallel()
	db := openCanonicalTestDB(t)
	// No seeds.

	p := Playbook{
		EntitySlots: []string{"$THING"},
		Steps:       []PlaybookStep{{Cmd: "x"}},
	}
	cfg := espnLikeConfig()
	normalized := Normalize("how is weatherman doing", cfg)
	resolver := NewCanonicalResolver(context.Background(), db)
	got := ResolveSlots(p, normalized, resolver)
	if _, ok := got["$THING"]; ok {
		t.Errorf("unresolvable slot should be absent; got %+v", got)
	}
}

func TestResolveSlots_EmptySlots(t *testing.T) {
	t.Parallel()
	db := openCanonicalTestDB(t)

	p := Playbook{Steps: []PlaybookStep{{Cmd: "x"}}}
	resolver := NewCanonicalResolver(context.Background(), db)
	cfg := espnLikeConfig()
	got := ResolveSlots(p, Normalize("any query", cfg), resolver)
	if got != nil {
		t.Errorf("empty entity_slots should yield nil map, got %+v", got)
	}
}

// TestResolveSlots_OnlyConsidersEntities guards the Greptile finding
// on PR #851 round 3: ResolveSlots used to include non-entity tokens
// in the candidate pool. If a token classified as non-entity happens
// to resolve via entity_lookups (a secondary-alias collision the
// extractor didn't promote), the old code would still let it win a
// slot binding meant for a real entity. The fix restricts the pool
// to normalized.Entities so the slot stays unbound rather than
// silently grabbing the wrong token.
//
// To exercise the contract without depending on PromoteEntities'
// classification logic, we construct the normalized query directly:
// "boston" is in Entities, "ppg" is only in NonEntityNormalized, and
// the resolver can resolve BOTH. Pre-fix: "ppg" would have won
// because the candidate pool included non-entity tokens. Post-fix:
// "boston" is the only candidate.
func TestResolveSlots_OnlyConsidersEntities(t *testing.T) {
	t.Parallel()
	db := openCanonicalTestDB(t)
	seedCanonical(t, db, "nba_team", "Boston Celtics",
		[]string{"Boston Celtics", "Boston", "Celtics", "BOS"})
	// Register "ppg" as a secondary alias of a different canonical so
	// the resolver returns a hit for it.
	seedCanonical(t, db, "stat_abbrev", "PointsPerGame",
		[]string{"PointsPerGame", "ppg"})

	p := Playbook{
		EntitySlots: []string{"$TEAM"},
		Steps:       []PlaybookStep{{Cmd: "leaders {team.abbr}"}},
	}
	// Hand-build the normalized query so "ppg" stays in
	// NonEntityNormalized (the test of the fix's contract).
	normalized := NormalizedQuery{
		Entities:            []string{"boston"},
		NonEntityNormalized: "leads ppg who",
	}
	resolver := NewCanonicalResolver(context.Background(), db)
	got := ResolveSlots(p, normalized, resolver)

	slot, ok := got["$TEAM"]
	if !ok {
		t.Fatalf("$TEAM slot missing; got %+v", got)
	}
	if slot["token"] == "ppg" || slot["canonical"] == "PointsPerGame" {
		t.Errorf("non-entity token 'ppg' wrongly won the $TEAM slot; slot=%+v", slot)
	}
	if slot["canonical"] != "Boston Celtics" {
		t.Errorf("$TEAM canonical = %v, want Boston Celtics", slot["canonical"])
	}
}

func TestResolveSlots_NilResolver(t *testing.T) {
	t.Parallel()
	p := Playbook{
		EntitySlots: []string{"$X"},
		Steps:       []PlaybookStep{{Cmd: "x"}},
	}
	cfg := espnLikeConfig()
	got := ResolveSlots(p, Normalize("anything", cfg), nil)
	if got != nil {
		t.Errorf("nil resolver should yield nil, got %+v", got)
	}
}
