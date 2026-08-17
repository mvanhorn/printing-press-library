// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0.

package granola

import (
	"context"
	"testing"
)

func seedDocs(cache *Cache, ids ...string) {
	if cache.Documents == nil {
		cache.Documents = map[string]Document{}
	}
	for i, id := range ids {
		// Descending timestamps so the first id is newest.
		cache.Documents[id] = Document{
			ID:        id,
			CreatedAt: []string{"2026-08-03", "2026-08-02", "2026-08-01", "2026-07-31"}[i%4],
		}
	}
}

func TestTranscriptCandidatesSkipAttemptedAndDeleted(t *testing.T) {
	deleted := "gone"
	cache := &Cache{Documents: map[string]Document{
		"new":     {ID: "new", CreatedAt: "2026-08-03"},
		"old":     {ID: "old", CreatedAt: "2026-07-01"},
		"done":    {ID: "done", CreatedAt: "2026-08-02"},
		"deleted": {ID: "deleted", CreatedAt: "2026-08-03", DeletedAt: &deleted},
	}}
	got := transcriptCandidates(cache, map[string]bool{"done": true})

	if len(got) != 2 {
		t.Fatalf("want 2 candidates (attempted and deleted excluded), got %v", got)
	}
	// Newest first: a bounded run should spend its budget on recent meetings.
	if got[0] != "new" || got[1] != "old" {
		t.Errorf("candidates not ordered newest-first: %v", got)
	}
}

func TestHydrateTranscriptsRecordsAttemptForEmptyResult(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	cache := &Cache{}
	seedDocs(cache, "m1")

	client := newFakeTranscriptClient(map[string][]TranscriptSegment{"m1": nil})
	res, err := HydrateTranscriptsFromAPI(ctx, db, cache, client, 0)
	if err != nil {
		t.Fatalf("HydrateTranscriptsFromAPI: %v", err)
	}
	if res.Fetched != 1 || res.WithSegments != 0 {
		t.Errorf("fetched=%d withSegments=%d, want 1/0", res.Fetched, res.WithSegments)
	}

	// The point of the state table: a meeting that genuinely has no transcript
	// must stop being asked about, or every sync re-pays for it forever.
	attempted, err := transcriptAttemptedIDs(ctx, db)
	if err != nil {
		t.Fatalf("transcriptAttemptedIDs: %v", err)
	}
	if !attempted["m1"] {
		t.Fatal("an empty result did not record an attempt")
	}
	res2, err := HydrateTranscriptsFromAPI(ctx, db, cache, client, 0)
	if err != nil {
		t.Fatalf("second pass: %v", err)
	}
	if res2.Fetched != 0 {
		t.Errorf("second pass re-fetched %d already-attempted documents", res2.Fetched)
	}
}

func TestHydrateTranscriptsFillsCacheAndRespectsBudget(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	cache := &Cache{}
	seedDocs(cache, "a", "b", "c")

	segs := []TranscriptSegment{{Text: "hello", Source: "microphone"}}
	client := newFakeTranscriptClient(map[string][]TranscriptSegment{"a": segs, "b": segs, "c": segs})

	res, err := HydrateTranscriptsFromAPI(ctx, db, cache, client, 2)
	if err != nil {
		t.Fatalf("HydrateTranscriptsFromAPI: %v", err)
	}
	if res.Fetched != 2 {
		t.Errorf("budget not honored: fetched %d, want 2", res.Fetched)
	}
	if res.Remaining != 1 {
		t.Errorf("remaining=%d, want 1 so the caller can tell the user to re-run", res.Remaining)
	}
	if len(cache.Transcripts) != 2 {
		t.Errorf("cache.Transcripts has %d entries, want 2", len(cache.Transcripts))
	}
	if res.Segments != 2 {
		t.Errorf("segment total=%d, want 2", res.Segments)
	}

	// A second pass picks up exactly the one left behind.
	res2, err := HydrateTranscriptsFromAPI(ctx, db, cache, client, 2)
	if err != nil {
		t.Fatalf("second pass: %v", err)
	}
	if res2.Fetched != 1 || res2.Remaining != 0 {
		t.Errorf("resume pass fetched=%d remaining=%d, want 1/0", res2.Fetched, res2.Remaining)
	}
}

func TestHydrateTranscriptsSkipsPerDocumentFailures(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	cache := &Cache{}
	seedDocs(cache, "ok1", "missing", "ok2")

	client := newFakeTranscriptClient(map[string][]TranscriptSegment{
		"ok1": {{Text: "one"}},
		"ok2": {{Text: "two"}},
	})
	client.errs = map[string]error{"missing": ErrNoteNotFound}

	res, err := HydrateTranscriptsFromAPI(ctx, db, cache, client, 0)
	if err != nil {
		t.Fatalf("one unreadable document must not fail the run: %v", err)
	}
	if res.WithSegments != 2 {
		t.Errorf("withSegments=%d, want 2", res.WithSegments)
	}
	if len(res.Skipped) != 1 || res.Skipped[0] != "missing" {
		t.Errorf("skipped=%v, want [missing]", res.Skipped)
	}
	// The skipped document is still recorded, so it is not retried forever.
	attempted, _ := transcriptAttemptedIDs(ctx, db)
	if !attempted["missing"] {
		t.Error("a skippable failure did not record an attempt")
	}
}

func TestHydrateTranscriptsStopsOnRunWideFailure(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	cache := &Cache{}
	seedDocs(cache, "a", "b", "c")

	client := newFakeTranscriptClient(nil)
	// An auth failure will hit every remaining document identically; grinding
	// through hundreds of them helps nobody.
	client.errs = map[string]error{"a": ErrAPIUnauthorized, "b": ErrAPIUnauthorized, "c": ErrAPIUnauthorized}

	if _, err := HydrateTranscriptsFromAPI(ctx, db, cache, client, 0); err == nil {
		t.Fatal("a non-skippable failure should stop the run")
	}
}

// fakeTranscriptClient stands in for the internal API.
type fakeTranscriptClient struct {
	segs map[string][]TranscriptSegment
	errs map[string]error
}

func newFakeTranscriptClient(segs map[string][]TranscriptSegment) *fakeTranscriptClient {
	if segs == nil {
		segs = map[string][]TranscriptSegment{}
	}
	return &fakeTranscriptClient{segs: segs}
}

func (f *fakeTranscriptClient) GetDocumentTranscript(id string) ([]TranscriptSegment, error) {
	if err, ok := f.errs[id]; ok {
		return nil, err
	}
	return f.segs[id], nil
}
