// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"context"
	"encoding/json"
	"io"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/suno/internal/store"
)

// fakeClipsClient serves scripted feed pages and records each request body so
// tests can assert cursor behavior. After the scripted pages are exhausted it
// returns a terminal empty page (has_more=false).
type fakeClipsClient struct {
	pages  []sunoFeedResponse
	calls  int
	bodies []map[string]any
}

func (f *fakeClipsClient) PostWithParamsAndHeaders(_ context.Context, _ string, _ map[string]string, body any, _ map[string]string) (json.RawMessage, int, error) {
	if b, ok := body.(map[string]any); ok {
		f.bodies = append(f.bodies, b)
	}
	var page sunoFeedResponse
	if f.calls < len(f.pages) {
		page = f.pages[f.calls]
	} // else: terminal empty page (HasMore false, no clips)
	f.calls++
	raw, _ := json.Marshal(page)
	return raw, 200, nil
}

func cursorPtr(s string) *string { return &s }

func clip(id, createdAt string) json.RawMessage {
	return json.RawMessage(`{"id":"` + id + `","created_at":"` + createdAt + `","title":"t"}`)
}

func newClipsSyncTestStore(t *testing.T) *store.Store {
	t.Helper()
	db, err := store.OpenWithContext(context.Background(), filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// TestSyncSunoClipsEarlyStopOnSince proves that an explicit --since bounds the
// clips walk: the walker stops at the page whose oldest clip predates the
// boundary instead of draining the whole feed, and it starts at the head
// (ignoring any stale resume cursor) so the newest clips are never skipped.
func TestSyncSunoClipsEarlyStopOnSince(t *testing.T) {
	db := newClipsSyncTestStore(t)

	// A stale resume cursor that an incremental walk must ignore.
	if err := db.SaveSyncState(sunoClipsResource, "stale-mid-feed-cursor", 10); err != nil {
		t.Fatalf("seed sync_state: %v", err)
	}

	fc := &fakeClipsClient{pages: []sunoFeedResponse{
		// Page 1: all newer than the 2026-06-01 boundary.
		{Clips: raws2(string(clip("a", "2026-06-05T10:00:00Z")), string(clip("b", "2026-06-03T10:00:00Z"))), NextCursor: cursorPtr("c1"), HasMore: true},
		// Page 2: crossing page — oldest (d) predates the boundary.
		{Clips: raws2(string(clip("c", "2026-06-02T10:00:00Z")), string(clip("d", "2026-05-28T10:00:00Z"))), NextCursor: cursorPtr("c2"), HasMore: true},
		// Page 3: must never be fetched.
		{Clips: raws2(string(clip("e", "2026-05-20T10:00:00Z"))), NextCursor: cursorPtr("c3"), HasMore: true},
	}}

	total, err := syncSunoClips(context.Background(), fc, db, "dev", false, 0, "2026-06-01T00:00:00Z", io.Discard)
	if err != nil {
		t.Fatalf("syncSunoClips: %v", err)
	}

	// Stops after the crossing page: 2 requests, 4 clips (a,b,c,d), page 3 skipped.
	if fc.calls != 2 {
		t.Errorf("requests = %d, want 2 (early-stop after crossing page)", fc.calls)
	}
	if total != 4 {
		t.Errorf("clips upserted = %d, want 4", total)
	}
	// First request starts at the head — no cursor — despite the stale resume cursor.
	if len(fc.bodies) == 0 {
		t.Fatalf("no requests captured")
	}
	if _, hasCursor := fc.bodies[0]["cursor"]; hasCursor {
		t.Errorf("first incremental request sent a cursor %v; want head start (no cursor)", fc.bodies[0]["cursor"])
	}
}

// TestSyncSunoClipsFullDrainWithoutBoundary confirms that with no --since and no
// stored cursor (fresh library), the walk drains every page until the feed
// reports has_more=false — early-stop must not fire without a boundary.
func TestSyncSunoClipsFullDrainWithoutBoundary(t *testing.T) {
	db := newClipsSyncTestStore(t)

	fc := &fakeClipsClient{pages: []sunoFeedResponse{
		{Clips: raws2(string(clip("a", "2026-06-05T10:00:00Z"))), NextCursor: cursorPtr("c1"), HasMore: true},
		{Clips: raws2(string(clip("b", "2026-05-01T10:00:00Z"))), NextCursor: cursorPtr("c2"), HasMore: true},
		{Clips: raws2(string(clip("c", "2026-01-01T10:00:00Z"))), HasMore: false},
	}}

	total, err := syncSunoClips(context.Background(), fc, db, "dev", false, 0, "", io.Discard)
	if err != nil {
		t.Fatalf("syncSunoClips: %v", err)
	}
	if fc.calls != 3 {
		t.Errorf("requests = %d, want 3 (full drain to has_more=false)", fc.calls)
	}
	if total != 3 {
		t.Errorf("clips upserted = %d, want 3", total)
	}
}
