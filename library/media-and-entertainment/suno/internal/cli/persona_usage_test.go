// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"context"
	"encoding/json"
	"testing"
)

func TestPersonaUsageRows(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ctx := context.Background()
	s, err := openDefaultStore(ctx)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer s.Close()

	// Two personas; p1 is referenced by two clips (one via metadata.persona_id,
	// one via top-level persona_id), p2 by none (orphan). UpsertBatch routes
	// rows into the typed personas/clips tables that personaUsageRows reads.
	personas := []json.RawMessage{
		mustJSON(map[string]any{"id": "p1", "name": "Velvet"}),
		mustJSON(map[string]any{"id": "p2", "name": "Gravel"}),
	}
	if _, _, err := s.UpsertBatch("personas", personas); err != nil {
		t.Fatalf("seed personas: %v", err)
	}
	clips := []json.RawMessage{
		mustJSON(map[string]any{"id": "c1", "metadata": map[string]any{"persona_id": "p1"}}),
		mustJSON(map[string]any{"id": "c2", "persona_id": "p1"}),
		mustJSON(map[string]any{"id": "c3"}), // no persona
	}
	if _, _, err := s.UpsertBatch("clips", clips); err != nil {
		t.Fatalf("seed clips: %v", err)
	}

	rows, err := personaUsageRows(s)
	if err != nil {
		t.Fatalf("personaUsageRows: %v", err)
	}
	got := map[string]personaUsageRow{}
	for _, r := range rows {
		got[r.PersonaID] = r
	}
	if r := got["p1"]; r.UsageCount != 2 || r.Orphan {
		t.Errorf("p1 = %+v, want usage_count=2 orphan=false", r)
	}
	if r := got["p2"]; r.UsageCount != 0 || !r.Orphan {
		t.Errorf("p2 = %+v, want usage_count=0 orphan=true", r)
	}
}
