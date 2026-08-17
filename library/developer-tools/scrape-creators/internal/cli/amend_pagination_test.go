// Copyright 2026 Adrian Horning and contributors. Licensed under Apache-2.0. See LICENSE.
// Tests for the amend-2026-07-29 pagination + usability patch: --all follows
// declared response cursors, lone positional args stand in for the required
// flag, and local-store errors point at a sync invocation that can work.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
)

// fakePagedClient serves object-shaped pages keyed by the cursor param value,
// mimicking Scrape Creators endpoints like /v2/instagram/user/posts that
// return {"items": [...], "next_max_id": "...", "more_available": true}.
type fakePagedClient struct {
	pages map[string]string // cursor value -> raw JSON page ("" = first page)
	calls []string
}

func (f *fakePagedClient) GetWithHeaders(_ context.Context, _ string, params map[string]string, _ map[string]string) (json.RawMessage, error) {
	cursor := params["next_max_id"]
	f.calls = append(f.calls, cursor)
	page, ok := f.pages[cursor]
	if !ok {
		return nil, fmt.Errorf("unexpected cursor %q", cursor)
	}
	return json.RawMessage(page), nil
}

func TestPaginatedGetFollowsDeclaredCursor(t *testing.T) {
	c := &fakePagedClient{pages: map[string]string{
		"":     `{"items":[{"id":"a"},{"id":"b"}],"next_max_id":"CUR2","more_available":true}`,
		"CUR2": `{"items":[{"id":"c"}],"next_max_id":"","more_available":false}`,
	}}

	data, err := paginatedGet(context.Background(), c, "/v2/instagram/user/posts",
		map[string]string{"handle": "example"}, nil,
		true,          // fetchAll
		"next_max_id", // cursorParam
		"cursor",      // paginationType
		"",            // limitParam
		0,             // defaultPageSize
		"next_max_id", // nextCursorPath
		"more_available")
	if err != nil {
		t.Fatalf("paginatedGet: %v", err)
	}
	if got, want := len(c.calls), 2; got != want {
		t.Fatalf("expected %d page fetches, got %d (%v)", want, got, c.calls)
	}
	if c.calls[1] != "CUR2" {
		t.Fatalf("second fetch should carry the response cursor, got %q", c.calls[1])
	}
	var items []map[string]any
	if err := json.Unmarshal(data, &items); err != nil {
		t.Fatalf("unmarshal merged items: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("expected 3 merged items across pages, got %d", len(items))
	}
}

func TestPaginatedGetAutoDetectsUndeclaredCursor(t *testing.T) {
	// With no nextCursorPath/hasMoreField declared, the 4.30 paginator
	// auto-detects well-known cursor keys (next_max_id) in the response and
	// follows them — stronger than the 4.27-era patch, which required an
	// explicit declaration. Pin the auto-detection so a regen can't regress
	// --all to a single silent page.
	c := &fakePagedClient{pages: map[string]string{
		"":     `{"items":[{"id":"a"}],"next_max_id":"CUR2","more_available":true}`,
		"CUR2": `{"items":[{"id":"b"}],"more_available":false}`,
	}}
	data, err := paginatedGet(context.Background(), c, "/v2/instagram/user/posts",
		map[string]string{"handle": "example"}, nil, true, "next_max_id", "cursor", "", 0, "", "")
	if err != nil {
		t.Fatalf("paginatedGet: %v", err)
	}
	if got := len(c.calls); got != 2 {
		t.Fatalf("expected cursor auto-detection to fetch both pages, got %d", got)
	}
	var items []map[string]any
	if err := json.Unmarshal(data, &items); err != nil {
		t.Fatalf("unmarshal items: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected merged items from both auto-detected pages, got %d", len(items))
	}
}
