// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"context"
	"encoding/json"
	"testing"
)

func raws(ids ...string) []json.RawMessage {
	out := make([]json.RawMessage, len(ids))
	for i, id := range ids {
		out[i] = json.RawMessage(`{"id":"` + id + `"}`)
	}
	return out
}

func fakeFetcher(pages map[string]feedPage) feedFetcher {
	return func(ctx context.Context, cursor string, limit int) (feedPage, error) {
		return pages[cursor], nil
	}
}

func TestWalkFeed_DrainsAllPages(t *testing.T) {
	pages := map[string]feedPage{
		"":   {Clips: raws("a", "b"), NextCursor: "c1", HasMore: true},
		"c1": {Clips: raws("c", "d"), NextCursor: "c2", HasMore: true},
		"c2": {Clips: raws("e"), NextCursor: "", HasMore: false},
	}
	var got []json.RawMessage
	err := walkFeed(context.Background(), fakeFetcher(pages), 2, "", func(c []json.RawMessage) (bool, error) {
		got = append(got, c...)
		return true, nil
	})
	if err != nil {
		t.Fatalf("walkFeed: %v", err)
	}
	if len(got) != 5 {
		t.Errorf("collected %d clips, want 5 (all pages drained)", len(got))
	}
}

func TestWalkFeed_StopsEarlyWhenVisitReturnsFalse(t *testing.T) {
	pages := map[string]feedPage{
		"":   {Clips: raws("a", "b"), NextCursor: "c1", HasMore: true},
		"c1": {Clips: raws("c", "d"), NextCursor: "c2", HasMore: true},
	}
	var got []json.RawMessage
	err := walkFeed(context.Background(), fakeFetcher(pages), 2, "", func(c []json.RawMessage) (bool, error) {
		got = append(got, c...)
		return false, nil // stop after first page
	})
	if err != nil {
		t.Fatalf("walkFeed: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("collected %d clips, want 2 (stopped early)", len(got))
	}
}

func TestWalkFeed_StopsOnStickyCursor(t *testing.T) {
	// Always returns the same next cursor with a full page and has_more=true.
	fetch := func(ctx context.Context, cursor string, limit int) (feedPage, error) {
		return feedPage{Clips: raws("x", "y"), NextCursor: "stuck", HasMore: true}, nil
	}
	pages := 0
	err := walkFeed(context.Background(), fetch, 2, "", func(c []json.RawMessage) (bool, error) {
		pages++
		return true, nil
	})
	if err != nil {
		t.Fatalf("walkFeed: %v", err)
	}
	if pages != 2 {
		t.Errorf("visited %d pages, want exactly 2 (sticky-cursor fires on 2nd repeat)", pages)
	}
}

func TestWalkFeed_StopsOnShortPage(t *testing.T) {
	// A page shorter than limit ends the walk even if the API still claims
	// has_more with a cursor.
	pages := map[string]feedPage{
		"": {Clips: raws("a"), NextCursor: "c1", HasMore: true}, // len 1 < limit 2
	}
	n := 0
	fetch := func(ctx context.Context, cursor string, limit int) (feedPage, error) { n++; return pages[cursor], nil }
	if err := walkFeed(context.Background(), fetch, 2, "", func(c []json.RawMessage) (bool, error) { return true, nil }); err != nil {
		t.Fatalf("walkFeed: %v", err)
	}
	if n != 1 {
		t.Errorf("fetched %d pages, want 1 (short page stops the walk)", n)
	}
}

func TestWalkFeed_LimitZeroTerminatesOnHasMore(t *testing.T) {
	// With limit 0 the short-page guard is disabled; termination must still
	// happen via has_more=false rather than looping forever.
	pages := map[string]feedPage{
		"":   {Clips: raws("a"), NextCursor: "c1", HasMore: true},
		"c1": {Clips: raws("b"), NextCursor: "", HasMore: false},
	}
	n := 0
	fetch := func(ctx context.Context, cursor string, limit int) (feedPage, error) { n++; return pages[cursor], nil }
	if err := walkFeed(context.Background(), fetch, 0, "", func(c []json.RawMessage) (bool, error) { return true, nil }); err != nil {
		t.Fatalf("walkFeed: %v", err)
	}
	if n != 2 {
		t.Errorf("fetched %d pages, want 2 (terminate via has_more even with limit 0)", n)
	}
}
