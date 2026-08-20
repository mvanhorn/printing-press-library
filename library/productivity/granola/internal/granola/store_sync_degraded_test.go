// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0.

package granola

import (
	"context"
	"database/sql"
	"testing"
)

// seedCacheOwnedRows puts the store in the state a healthy sync would leave
// behind: one meeting with a cache-owned transcript, and a cache-owned folder
// membership. These are exactly the rows a degraded run must not touch --
// once Granola's DEK moved out of reach they can never be re-synced.
func seedCacheOwnedRows(t *testing.T, db *sql.DB, meetingID string) {
	t.Helper()
	ctx := context.Background()
	if _, err := db.ExecContext(ctx,
		`INSERT INTO meetings(id,title,row_source) VALUES (?,?,?)`,
		meetingID, "Standup", RowSourceCache); err != nil {
		t.Fatalf("seed meeting: %v", err)
	}
	for i := 0; i < 3; i++ {
		if _, err := db.ExecContext(ctx,
			`INSERT INTO transcript_segments(meeting_id,idx,source,text,row_source) VALUES (?,?,?,?,?)`,
			meetingID, i, "microphone", "seeded segment", RowSourceCache); err != nil {
			t.Fatalf("seed segment %d: %v", i, err)
		}
	}
	if _, err := db.ExecContext(ctx,
		`INSERT INTO folders(id,title) VALUES (?,?)`, "fold_1", "Customers"); err != nil {
		t.Fatalf("seed folder: %v", err)
	}
	if _, err := db.ExecContext(ctx,
		`INSERT INTO folder_memberships(folder_id,meeting_id,row_source) VALUES (?,?,?)`,
		"fold_1", meetingID, RowSourceCache); err != nil {
		t.Fatalf("seed membership: %v", err)
	}
}

func countRows(t *testing.T, db *sql.DB, query string, args ...any) int {
	t.Helper()
	var n int
	if err := db.QueryRowContext(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatalf("count (%s): %v", query, err)
	}
	return n
}

// TestDegradedSyncPreservesCacheOwnedRows is the regression guard for the
// data-loss path: on a migrated install the cache cannot be decrypted, so
// documents are hydrated from the API into an otherwise-empty Cache. Running
// the normal sync against that Cache reads every absent cache-derived row as
// "deleted upstream" and retires it. Degraded mode must upsert only.
func TestDegradedSyncPreservesCacheOwnedRows(t *testing.T) {
	db := openTestDB(t)
	const meetingID = "not_seeded"
	seedCacheOwnedRows(t, db, meetingID)

	// What a degraded run actually holds: documents from /v2/get-documents,
	// and nothing else -- no transcripts, no document lists.
	cache := &Cache{
		Documents: map[string]Document{
			meetingID: {ID: meetingID, Title: "Standup"},
		},
	}

	res, err := SyncFromCacheWithOptions(context.Background(), db, cache, SyncOptions{Degraded: true})
	if err != nil {
		t.Fatalf("degraded SyncFromCache: %v", err)
	}

	if got := countRows(t, db,
		`SELECT COUNT(*) FROM transcript_segments WHERE meeting_id = ? AND row_source = ?`,
		meetingID, RowSourceCache); got != 3 {
		t.Errorf("cache-owned transcript segments: got %d, want 3 (degraded run destroyed them)", got)
	}
	if got := countRows(t, db,
		`SELECT COUNT(*) FROM folder_memberships WHERE row_source = ?`, RowSourceCache); got != 1 {
		t.Errorf("cache-owned folder memberships: got %d, want 1 (degraded run destroyed them)", got)
	}
	if res.Meetings != 1 {
		t.Errorf("meetings upserted: got %d, want 1", res.Meetings)
	}
}

// TestHealthySyncStillRetiresOwnRows pins the other half of the contract: the
// suppression is scoped to degraded runs. A healthy sync whose cache genuinely
// no longer lists a transcript or membership must still retire it, or a real
// upstream deletion would linger forever.
func TestHealthySyncStillRetiresOwnRows(t *testing.T) {
	db := openTestDB(t)
	const meetingID = "not_seeded"
	seedCacheOwnedRows(t, db, meetingID)

	// A readable cache that legitimately no longer carries the transcript or
	// the membership: the same shape as above, but not degraded.
	cache := &Cache{
		Documents: map[string]Document{
			meetingID: {ID: meetingID, Title: "Standup"},
		},
	}

	if _, err := SyncFromCache(context.Background(), db, cache); err != nil {
		t.Fatalf("healthy SyncFromCache: %v", err)
	}

	if got := countRows(t, db,
		`SELECT COUNT(*) FROM transcript_segments WHERE meeting_id = ? AND row_source = ?`,
		meetingID, RowSourceCache); got != 0 {
		t.Errorf("healthy sync should retire its own stale segments: got %d, want 0", got)
	}
	if got := countRows(t, db,
		`SELECT COUNT(*) FROM folder_memberships WHERE row_source = ?`, RowSourceCache); got != 0 {
		t.Errorf("healthy sync should retire its own stale memberships: got %d, want 0", got)
	}
}
