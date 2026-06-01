// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
package mcp

import (
	"strings"
	"testing"
)

func TestMCPIntArg(t *testing.T) {
	args := map[string]any{"offset": float64(7), "limit": float64(0)}
	if got := mcpIntArg(args, "offset"); got != 7 {
		t.Fatalf("offset = %d, want 7", got)
	}
	if got := mcpIntArg(args, "missing"); got != 0 {
		t.Fatalf("missing = %d, want 0", got)
	}
}

func TestBindingHasName(t *testing.T) {
	bindings := []mcpParamBinding{{PublicName: "limit", WireName: "limit", Location: "query"}}
	if !bindingHasName(bindings, "limit") {
		t.Fatalf("expected bindingHasName(limit)=true")
	}
	if bindingHasName(bindings, "offset") {
		t.Fatalf("expected bindingHasName(offset)=false")
	}
}

// TestClientCursorNames guards the wiring that stops a client-side pagination
// cursor from leaking into the upstream query string. A cursor name is consumed
// by the client pager (and so must be marked known, NOT forwarded) only when the
// tool has no native binding of that name. For an endpoint that pages
// server-side, the native binding owns the arg, so it is excluded here and still
// reaches the API.
func TestClientCursorNames(t *testing.T) {
	// No native offset/limit (e.g. get_comments as a list tool): both are
	// client cursors and must be consumed.
	noNative := clientCursorNames([]mcpParamBinding{{PublicName: "expense_id", WireName: "expense_id", Location: "query"}})
	if got := strings.Join(noNative, ","); got != "offset,limit" {
		t.Fatalf("clientCursorNames(no native) = %q, want \"offset,limit\"", got)
	}

	// Native offset AND limit (e.g. get_expenses): neither is a client cursor;
	// both must reach the upstream API, so the consumed set is empty.
	bothNative := clientCursorNames([]mcpParamBinding{
		{PublicName: "offset", WireName: "offset", Location: "query"},
		{PublicName: "limit", WireName: "limit", Location: "query"},
	})
	if len(bothNative) != 0 {
		t.Fatalf("clientCursorNames(both native) = %v, want empty", bothNative)
	}

	// Mixed (e.g. get_notifications declares limit but not offset): only the
	// undeclared cursor (offset) is consumed.
	mixed := clientCursorNames([]mcpParamBinding{{PublicName: "limit", WireName: "limit", Location: "query"}})
	if got := strings.Join(mixed, ","); got != "offset" {
		t.Fatalf("clientCursorNames(limit native) = %q, want \"offset\"", got)
	}
}
