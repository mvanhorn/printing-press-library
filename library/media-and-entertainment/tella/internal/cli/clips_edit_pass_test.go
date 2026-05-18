// Copyright 2026 gregce. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"testing"
)

// stubPagedGetter returns a scripted sequence of envelope responses on
// successive c.Get calls. Pages are indexed by cursor; the first call has no
// cursor query param and returns the page at "".
type stubPagedGetter struct {
	pages       map[string]json.RawMessage
	cursorParam string
	calls       int
	lastCursor  string
}

func (s *stubPagedGetter) Get(_ string, params map[string]string) (json.RawMessage, error) {
	s.calls++
	cursor := params[s.cursorParam]
	s.lastCursor = cursor
	page, ok := s.pages[cursor]
	if !ok {
		return nil, fmt.Errorf("unexpected cursor %q in test stub", cursor)
	}
	return page, nil
}

// TestPaginatedListIDs_FollowsCursorAcrossPages pins the round-7 fix: before
// the helper landed, listPlaylistVideoIDs / listClipIDs issued a single
// c.Get, so any list that needed more than one page silently dropped
// everything past the first page. The stub here advertises hasMore=true with
// a non-empty nextCursor on the first two pages and terminates on the third.
func TestPaginatedListIDs_FollowsCursorAcrossPages(t *testing.T) {
	stub := &stubPagedGetter{
		cursorParam: "cursor",
		pages: map[string]json.RawMessage{
			"": json.RawMessage(`{
                "videos": [{"id":"v1"},{"id":"v2"}],
                "pagination": {"nextCursor":"cur-1","hasMore":true}
            }`),
			"cur-1": json.RawMessage(`{
                "videos": [{"id":"v3"},{"id":"v4"}],
                "pagination": {"nextCursor":"cur-2","hasMore":true}
            }`),
			"cur-2": json.RawMessage(`{
                "videos": [{"id":"v5"}],
                "pagination": {"nextCursor":null,"hasMore":false}
            }`),
		},
	}
	got, err := paginatedListIDs(stub, "/v1/videos", nil, "videos")
	if err != nil {
		t.Fatalf("paginatedListIDs: %v", err)
	}
	want := []string{"v1", "v2", "v3", "v4", "v5"}
	if len(got) != len(want) {
		t.Fatalf("got %v (%d ids), want %v (%d ids); only %d calls made — pagination not followed",
			got, len(got), want, len(want), stub.calls)
	}
	for i, id := range want {
		if got[i] != id {
			t.Fatalf("got[%d] = %q, want %q", i, got[i], id)
		}
	}
	if stub.calls != 3 {
		t.Fatalf("stub called %d times, want 3 (one per page)", stub.calls)
	}
}

// TestPaginatedListIDs_StickyCursorTerminates pins the sticky-cursor guard.
// If the API echoes the same cursor across two calls, the helper must break
// out instead of looping forever — otherwise a misbehaving endpoint would
// burn the full 100-page cap.
func TestPaginatedListIDs_StickyCursorTerminates(t *testing.T) {
	stickyPage := json.RawMessage(`{
        "videos": [{"id":"v1"}],
        "pagination": {"nextCursor":"stuck","hasMore":true}
    }`)
	stub := &stubPagedGetter{
		cursorParam: "cursor",
		pages: map[string]json.RawMessage{
			"":      stickyPage,
			"stuck": stickyPage,
		},
	}
	got, err := paginatedListIDs(stub, "/v1/videos", nil, "videos")
	if err != nil {
		t.Fatalf("paginatedListIDs: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d ids, want 2 (one per page before sticky-cursor break)", len(got))
	}
	if stub.calls != 2 {
		t.Fatalf("stub called %d times, want 2 (sticky-cursor must break after the second call)", stub.calls)
	}
}

// TestPaginatedListIDs_SinglePageNoCursorBreaksImmediately pins the
// no-regression contract for small workspaces: when hasMore=false on the
// first page, exactly one c.Get is issued and every id is returned.
func TestPaginatedListIDs_SinglePageNoCursorBreaksImmediately(t *testing.T) {
	stub := &stubPagedGetter{
		cursorParam: "cursor",
		pages: map[string]json.RawMessage{
			"": json.RawMessage(`{
                "videos": [{"id":"v1"},{"id":"v2"},{"id":"v3"}],
                "pagination": {"nextCursor":null,"hasMore":false}
            }`),
		},
	}
	got, err := paginatedListIDs(stub, "/v1/videos", nil, "videos")
	if err != nil {
		t.Fatalf("paginatedListIDs: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d ids, want 3", len(got))
	}
	if stub.calls != 1 {
		t.Fatalf("stub called %d times, want 1 (small workspace must not paginate)", stub.calls)
	}
}
