// Copyright 2026 Elizabeth Stone and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"testing"
)

func TestPinterestBookmarkPagination(t *testing.T) {
	for _, resource := range []string{"boards", "pins", "boards/boards_pins"} {
		d := determinePaginationDefaults(resource)
		if d.cursorParam != "bookmark" || d.cursorType != "cursor" || d.nextCursorPath != "bookmark" {
			t.Fatalf("%s pagination = %+v, want bookmark cursor", resource, d)
		}
	}
	d := determinePaginationDefaults("boards")
	items, next, more := extractPageItemsWithPagination(json.RawMessage(`{"items":[{"id":"1"},{"id":"2"}],"bookmark":"bm-2"}`), d.cursorParam, d.nextCursorPath)
	if len(items) != 2 || next != "bm-2" || !more {
		t.Fatalf("page 1: items=%d next=%q more=%v", len(items), next, more)
	}
	_, next, more = extractPageItemsWithPagination(json.RawMessage(`{"items":[{"id":"3"}],"bookmark":null}`), d.cursorParam, d.nextCursorPath)
	if next != "" || more {
		t.Fatalf("last page should end pagination: next=%q more=%v", next, more)
	}
}

func TestPinterestBookmarkCursorForAll(t *testing.T) {
	c, typ, next := pinterestBookmarkCursor("", "offset", "page_size", "")
	if c != "bookmark" || typ != "cursor" || next != "bookmark" {
		t.Fatalf("got %q %q %q", c, typ, next)
	}
	if c, typ, _ := pinterestBookmarkCursor("cursor", "cursor", "limit", ""); c != "cursor" || typ != "cursor" {
		t.Fatalf("explicit cursor must be kept, got %q %q", c, typ)
	}
}
