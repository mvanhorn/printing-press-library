// Copyright 2026 dstevens. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/granola/internal/granola"
	"github.com/mvanhorn/printing-press-library/library/productivity/granola/internal/store"

	_ "modernc.org/sqlite"
)

// TestAttendeeTimeline_StoreFiltering builds 3 meetings (2 with Trevin,
// 1 without) in a temp store and asserts the SQL filtering returns the
// 2 in ASC order.
func TestAttendeeTimeline_StoreFiltering(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "data.db")
	t.Setenv("HOME", tmp) // openGranolaStoreRead reads HOME for defaultDBPath
	// Override path: defaultDBPath uses ~/.local/share/granola-pp-cli/data.db
	_ = os.MkdirAll(filepath.Join(tmp, ".local", "share", "granola-pp-cli"), 0o755)
	dbPath = filepath.Join(tmp, ".local", "share", "granola-pp-cli", "data.db")

	s, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := granola.EnsureSchema(context.Background(), s.DB()); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	for i, row := range []struct {
		ID, Title, Started string
		With               []string
	}{
		{"m1", "Meeting 1", "2026-04-01T10:00:00Z", []string{"alice@example.com", "PII_EMAIL_EXAMPLE_OTHER"}},
		{"m2", "Meeting 2", "2026-04-02T10:00:00Z", []string{"PII_EMAIL_EXAMPLE_SOMEONE"}},
		{"m3", "Meeting 3", "2026-04-03T10:00:00Z", []string{"PII_EMAIL_ALICE_SECOND"}},
	} {
		_, err := s.DB().Exec(`INSERT INTO meetings(id,title,started_at,transcript_available) VALUES (?,?,?,?)`, row.ID, row.Title, row.Started, 1)
		if err != nil {
			t.Fatalf("row %d: %v", i, err)
		}
		for _, e := range row.With {
			_, err := s.DB().Exec(`INSERT INTO attendees(meeting_id,email,name) VALUES (?,?,?)`, row.ID, e, "")
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	rows, err := s.DB().Query(`SELECT m.id FROM meetings m JOIN attendees a ON a.meeting_id = m.id WHERE a.email LIKE ? ORDER BY m.started_at ASC`, "%alice%")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		_ = rows.Scan(&id)
		ids = append(ids, id)
	}
	if len(ids) != 2 || ids[0] != "m1" || ids[1] != "m3" {
		t.Errorf("expected [m1 m3] ASC, got %v", ids)
	}
}

// Ensure sql.ErrNoRows reachable to satisfy import linter on environments
// where the package would otherwise be considered unused.
var _ = sql.ErrNoRows
