// Copyright 2026 mlabrenz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
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
	stateKey := flippSyncStateKey("flyers", "85001", "en-us")
	seedLastSynced(t, db, stateKey, old, "resume-me")

	if err := db.ResetSyncCursor(stateKey); err != nil {
		t.Fatalf("ResetSyncCursor: %v", err)
	}
	cursor, _, _, err := db.GetSyncState(stateKey)
	if err != nil {
		t.Fatalf("GetSyncState after reset: %v", err)
	}
	if cursor != "" {
		t.Fatalf("cursor after reset = %q, want empty", cursor)
	}
	if got := lastSyncedAt(t, db, stateKey); !got.Equal(old) {
		t.Fatalf("ResetSyncCursor advanced last_synced_at: got %v want %v", got, old)
	}

	res := syncResource(context.Background(), stubSyncClient{err: fmt.Errorf("fetching flyers: 500")}, db, "flyers", "", true, 0, false, false, flippSyncLocationParams("85001", "en-us"), io.Discard)
	if res.Err == nil {
		t.Fatal("expected fetch error")
	}
	if got := lastSyncedAt(t, db, stateKey); !got.Equal(old) {
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
	stateKey := flippSyncStateKey("flyers", "85001", "en-us")
	seedLastSynced(t, db, stateKey, old, "")

	res := syncResource(context.Background(), stubSyncClient{body: json.RawMessage(`<html>not json</html>`)}, db, "flyers", "", false, 0, false, false, flippSyncLocationParams("85001", "en-us"), io.Discard)
	if res.Err != nil {
		t.Fatalf("non-JSON 200 should not fail the resource: %v", res.Err)
	}
	if res.Count != 0 {
		t.Fatalf("stored count = %d, want 0", res.Count)
	}
	if res.Complete {
		t.Fatal("non-JSON response must not count as a complete archive enumeration")
	}
	if got := lastSyncedAt(t, db, stateKey); !got.Equal(old) {
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
	stateKey := flippSyncStateKey("flyers", "85001", "en-us")
	seedLastSynced(t, db, stateKey, old, "")
	before := time.Now().UTC().Add(-time.Second)

	res := syncResource(context.Background(), stubSyncClient{body: json.RawMessage(`[{"id":"f1","name":"Weekly"}]`)}, db, "flyers", "", false, 0, false, false, flippSyncLocationParams("85001", "en-us"), io.Discard)
	if res.Err != nil {
		t.Fatalf("sync: %v", res.Err)
	}
	if res.Count != 1 {
		t.Fatalf("stored count = %d, want 1", res.Count)
	}
	if !res.Complete {
		t.Fatalf("completed resource should be archive-complete, reason %q", res.IncompleteReason)
	}
	got := lastSyncedAt(t, db, stateKey)
	if !got.After(before) {
		t.Fatalf("completed sync last_synced_at = %v, want after %v", got, before)
	}
	if got.Equal(old) {
		t.Fatal("completed sync left last_synced_at at the seeded timestamp")
	}
	var raw string
	if err := db.DB().QueryRow(`SELECT data FROM resources WHERE resource_type = 'flyers'`).Scan(&raw); err != nil {
		t.Fatalf("read stored flyer: %v", err)
	}
	var stored map[string]any
	if err := json.Unmarshal([]byte(raw), &stored); err != nil {
		t.Fatalf("decode stored flyer: %v", err)
	}
	if stored["_sync_postal_code"] != "85001" || stored["_sync_locale"] != "en-us" {
		t.Fatalf("stored location provenance = %#v", stored)
	}
}

func TestSyncResourceRejectsMissingLocation(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	res := syncResource(context.Background(), stubSyncClient{body: json.RawMessage(`[]`)}, db, "flyers", "", false, 0, false, false, nil, io.Discard)
	if res.Err == nil || !strings.Contains(res.Err.Error(), "postal code") {
		t.Fatalf("missing-location result = %#v, want postal-code error", res)
	}
}

func TestEmptyPageAdvertisingAnotherPageIsIncomplete(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
	}{
		{name: "outer", body: `{"items":[],"has_more":true,"next_cursor":"page-2"}`},
		{name: "nested", body: `{"data":{"items":[],"has_more":true,"next_cursor":"page-2"}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			res := syncResource(context.Background(), stubSyncClient{body: json.RawMessage(tc.body)}, db, "flyers", "", false, 0, false, false, flippSyncLocationParams("10001", "en-us"), io.Discard)
			if res.Err != nil || res.Complete || res.IncompleteReason != "pagination_unhandled" {
				t.Fatalf("empty advertised next page = %#v, want incomplete", res)
			}
		})
	}
}
