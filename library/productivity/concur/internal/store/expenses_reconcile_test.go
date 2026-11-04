// Copyright 2026 Allen Lew and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// PATCH(amend-2026-11-04: never prune on a partially understood listing) --
// an item the sync could not read must not cause the rows it would have
// matched to be deleted.
func TestUpsertAndReconcileExpenses_PruneRules(t *testing.T) {
	exp := func(id, report string) json.RawMessage {
		b, _ := json.Marshal(map[string]any{"expenseId": id, "reportId": report})
		return b
	}
	tests := []struct {
		name       string
		second     []json.RawMessage
		prune      bool
		wantGone   []string
		wantKept   []string
		wantPruned int
	}{
		{
			name:       "missing expense is pruned",
			second:     []json.RawMessage{exp("A", "R1")},
			prune:      true,
			wantGone:   []string{"B"},
			wantKept:   []string{"A", "C"},
			wantPruned: 1,
		},
		{
			name:     "prune disabled keeps everything",
			second:   []json.RawMessage{exp("A", "R1")},
			prune:    false,
			wantKept: []string{"A", "B", "C"},
		},
		{
			name:     "listing with an unreadable item is not pruned",
			second:   []json.RawMessage{exp("A", "R1"), json.RawMessage(`"not an object"`)},
			prune:    true,
			wantKept: []string{"A", "B", "C"},
		},
		{
			name:       "empty listing prunes only that report",
			second:     nil,
			prune:      true,
			wantGone:   []string{"A", "B"},
			wantKept:   []string{"C"},
			wantPruned: 2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := Open(filepath.Join(t.TempDir(), "data.db"))
			if err != nil {
				t.Fatalf("open: %v", err)
			}
			defer s.Close()
			if _, _, _, err := s.UpsertAndReconcileExpenses([]json.RawMessage{exp("A", "R1"), exp("B", "R1")}, "R1", true); err != nil {
				t.Fatalf("first upsert: %v", err)
			}
			if _, _, _, err := s.UpsertAndReconcileExpenses([]json.RawMessage{exp("C", "R2")}, "R2", true); err != nil {
				t.Fatalf("other report upsert: %v", err)
			}
			_, pruned, _, err := s.UpsertAndReconcileExpenses(tt.second, "R1", tt.prune)
			if err != nil {
				t.Fatalf("second upsert: %v", err)
			}
			if pruned != tt.wantPruned {
				t.Errorf("pruned = %d, want %d", pruned, tt.wantPruned)
			}
			for _, id := range tt.wantGone {
				if _, err := s.Get("expenses", id); err == nil {
					t.Errorf("expense %s should have been pruned", id)
				}
			}
			for _, id := range tt.wantKept {
				if _, err := s.Get("expenses", id); err != nil {
					t.Errorf("expense %s should have been kept: %v", id, err)
				}
			}
		})
	}
}
