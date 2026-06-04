// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestSessions_GapBoundary(t *testing.T) {
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	seedClips(t, []map[string]any{
		{"id": "a", "created_at": base.Format(time.RFC3339)},
		{"id": "b", "created_at": base.Add(29 * time.Minute).Format(time.RFC3339)},
		{"id": "c", "created_at": base.Add(120 * time.Minute).Format(time.RFC3339)},
	})
	cmd := newSessionsCmd(&rootFlags{asJSON: true})
	var out strings.Builder
	cmd.SetOut(&out)
	cmd.SetArgs(nil)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("sessions: %v", err)
	}
	var rows []sessionRow
	if err := json.Unmarshal([]byte(out.String()), &rows); err != nil {
		t.Fatalf("parse: %v\n%s", err, out.String())
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 sessions, got %d: %s", len(rows), out.String())
	}
	if rows[0].GenerationsCount != 2 {
		t.Fatalf("expected first session to hold 2 generations, got %d", rows[0].GenerationsCount)
	}
}
