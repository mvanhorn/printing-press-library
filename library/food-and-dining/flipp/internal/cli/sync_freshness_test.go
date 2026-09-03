// Copyright 2026 mlabrenz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/flipp/internal/store"
)

type stubSyncClient struct {
	body json.RawMessage
	err  error
}

func (s stubSyncClient) Get(context.Context, string, map[string]string) (json.RawMessage, error) {
	return s.body, s.err
}

func (s stubSyncClient) RateLimit() float64 { return 0 }

func seedLastSynced(t *testing.T, db *store.Store, resource string, ts time.Time, cursor string) {
	t.Helper()
	_, err := db.DB().Exec(
		`INSERT INTO sync_state(resource_type, last_cursor, last_synced_at, total_count)
		 VALUES (?, ?, ?, 1)
		 ON CONFLICT(resource_type) DO UPDATE SET
		   last_cursor = excluded.last_cursor,
		   last_synced_at = excluded.last_synced_at`,
		resource, cursor, ts.UTC().Format(time.RFC3339),
	)
	if err != nil {
		t.Fatalf("seed last_synced_at: %v", err)
	}
}

func lastSyncedAt(t *testing.T, db *store.Store, resource string) time.Time {
	t.Helper()
	_, ts, _, err := db.GetSyncState(resource)
	if err != nil {
		t.Fatalf("GetSyncState: %v", err)
	}
	return ts.UTC()
}

func TestShouldStampSuccessfulSync(t *testing.T) {
	t.Parallel()
	cases := []struct {
		complete bool
		stored   int
		want     bool
	}{
		{complete: true, stored: 0, want: true},
		{complete: true, stored: 3, want: true},
		{complete: false, stored: 4, want: true},
		{complete: false, stored: 0, want: false},
	}
	for _, tc := range cases {
		got := shouldStampSuccessfulSync(tc.complete, tc.stored)
		if got != tc.want {
			t.Errorf("shouldStampSuccessfulSync(complete=%v, stored=%d) = %v, want %v",
				tc.complete, tc.stored, got, tc.want)
		}
	}
}

func TestFullSyncFetchFailureDoesNotStampFreshness(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	old := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	seedLastSynced(t, db, "flyers", old, "resume-me")

	if err := db.ResetSyncCursor("flyers"); err != nil {
		t.Fatalf("ResetSyncCursor: %v", err)
	}
	cursor, _, _, err := db.GetSyncState("flyers")
	if err != nil {
		t.Fatalf("GetSyncState after reset: %v", err)
	}
	if cursor != "" {
		t.Fatalf("cursor after reset = %q, want empty", cursor)
	}
	if got := lastSyncedAt(t, db, "flyers"); !got.Equal(old) {
		t.Fatalf("ResetSyncCursor advanced last_synced_at: got %v want %v", got, old)
	}

	res := syncResource(context.Background(), stubSyncClient{err: fmt.Errorf("fetching flyers: 500")}, db, "flyers", "", true, 0, false, false, nil, io.Discard)
	if res.Err == nil {
		t.Fatal("expected fetch error")
	}
	if got := lastSyncedAt(t, db, "flyers"); !got.Equal(old) {
		t.Fatalf("failed --full sync advanced last_synced_at: got %v want %v", got, old)
	}
}

func TestIncompleteNonJSONSyncDoesNotStampFreshness(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	old := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	seedLastSynced(t, db, "flyers", old, "")

	res := syncResource(context.Background(), stubSyncClient{body: json.RawMessage(`<html>not json</html>`)}, db, "flyers", "", false, 0, false, false, nil, io.Discard)
	if res.Err != nil {
		t.Fatalf("non-JSON 200 should not fail the resource: %v", res.Err)
	}
	if res.Count != 0 {
		t.Fatalf("stored count = %d, want 0", res.Count)
	}
	if got := lastSyncedAt(t, db, "flyers"); !got.Equal(old) {
		t.Fatalf("incomplete non-JSON sync advanced last_synced_at: got %v want %v", got, old)
	}
}

func TestCompletedJSONSyncStampsFreshness(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	old := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	seedLastSynced(t, db, "flyers", old, "")
	before := time.Now().UTC().Add(-time.Second)

	res := syncResource(context.Background(), stubSyncClient{body: json.RawMessage(`[{"id":"f1","name":"Weekly"}]`)}, db, "flyers", "", false, 0, false, false, nil, io.Discard)
	if res.Err != nil {
		t.Fatalf("sync: %v", res.Err)
	}
	if res.Count != 1 {
		t.Fatalf("stored count = %d, want 1", res.Count)
	}
	got := lastSyncedAt(t, db, "flyers")
	if !got.After(before) {
		t.Fatalf("completed sync last_synced_at = %v, want after %v", got, before)
	}
	if got.Equal(old) {
		t.Fatal("completed sync left last_synced_at at the seeded timestamp")
	}
}
