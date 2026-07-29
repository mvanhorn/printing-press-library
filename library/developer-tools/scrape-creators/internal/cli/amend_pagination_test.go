// Copyright 2026 Adrian Horning and contributors. Licensed under Apache-2.0. See LICENSE.
// Tests for the amend-2026-07-29 pagination + usability patch: --all follows
// declared response cursors, lone positional args stand in for the required
// flag, and local-store errors point at a sync invocation that can work.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/spf13/cobra"
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

func TestPaginatedGetStopsWithoutDeclaredCursor(t *testing.T) {
	// With no nextCursorPath/hasMoreField declared, --all must return page 1
	// only (the pre-patch behavior for every endpoint).
	c := &fakePagedClient{pages: map[string]string{
		"": `{"items":[{"id":"a"}],"next_max_id":"CUR2","more_available":true}`,
	}}
	data, err := paginatedGet(context.Background(), c, "/v2/instagram/user/posts",
		map[string]string{"handle": "example"}, nil, true, "next_max_id", "cursor", "", "", "")
	if err != nil {
		t.Fatalf("paginatedGet: %v", err)
	}
	if got := len(c.calls); got != 1 {
		t.Fatalf("expected a single fetch without a declared cursor, got %d", got)
	}
	var items []map[string]any
	if err := json.Unmarshal(data, &items); err != nil {
		t.Fatalf("unmarshal items: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected page-1 items only, got %d", len(items))
	}
}

func TestAdoptLonePositionalArg(t *testing.T) {
	newCmd := func() (*cobra.Command, *string) {
		var handle string
		cmd := &cobra.Command{Use: "posts", RunE: func(*cobra.Command, []string) error { return nil }}
		cmd.Flags().StringVar(&handle, "handle", "", "handle")
		return cmd, &handle
	}

	t.Run("positional adopted", func(t *testing.T) {
		cmd, handle := newCmd()
		adoptLonePositionalArg(cmd, []string{"mrbeast"}, "handle", handle)
		if *handle != "mrbeast" {
			t.Fatalf("expected positional to fill the flag, got %q", *handle)
		}
		if !cmd.Flags().Changed("handle") {
			t.Fatal("flag should be marked as set after adoption")
		}
	})

	t.Run("explicit flag wins", func(t *testing.T) {
		cmd, handle := newCmd()
		if err := cmd.Flags().Set("handle", "explicit"); err != nil {
			t.Fatal(err)
		}
		adoptLonePositionalArg(cmd, []string{"positional"}, "handle", handle)
		if *handle != "explicit" {
			t.Fatalf("explicit flag must win over positional, got %q", *handle)
		}
	})

	t.Run("multiple args untouched", func(t *testing.T) {
		cmd, handle := newCmd()
		adoptLonePositionalArg(cmd, []string{"a", "b"}, "handle", handle)
		if *handle != "" {
			t.Fatalf("two positionals must not be adopted, got %q", *handle)
		}
	})
}

func TestLocalSyncHint(t *testing.T) {
	if !isSyncableResource("github") {
		t.Fatal("github should be syncable")
	}
	if isSyncableResource("definitely-not-a-resource") {
		t.Fatal("unknown resource types must not be reported as syncable")
	}
	if hint := localSyncHint("github"); strings.Contains(hint, "--resources") {
		t.Fatalf("default-synced resources should get the bare sync hint, got %q", hint)
	}
	// "instagram" is a known sync resource but not part of the default sync
	// set — the hint must name the explicit --resources invocation.
	if isDefaultSyncResource("instagram") {
		t.Skip("instagram joined the default sync set; hint distinction no longer applies")
	}
	if hint := localSyncHint("instagram"); !strings.Contains(hint, "sync --resources instagram") {
		t.Fatalf("non-default resources should get an explicit --resources hint, got %q", hint)
	}
}
