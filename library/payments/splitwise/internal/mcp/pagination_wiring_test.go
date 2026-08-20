// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
package mcp

import "testing"

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

// TestPaginatesNatively guards the gate that decides whether a GET tool's
// response is left untouched (the API already paged it) or re-wrapped by the
// client byte-budget pager. A tool pages natively when it declares an offset or
// limit query binding; for those the pager is disabled so the native response
// schema and full-collection total are preserved. Tools with no such binding are
// client-paged, so offset/limit are consumed locally and not forwarded upstream.
func TestPaginatesNatively(t *testing.T) {
	// No native offset/limit (e.g. get_comments as a list tool): client-paged.
	if paginatesNatively([]mcpParamBinding{{PublicName: "expense_id", WireName: "expense_id", Location: "query"}}) {
		t.Fatalf("expense_id-only tool must not be treated as natively paged")
	}
	// No bindings at all (e.g. get_groups): client-paged.
	if paginatesNatively(nil) {
		t.Fatalf("binding-less list tool must not be treated as natively paged")
	}
	// Native offset AND limit (e.g. get_expenses): natively paged.
	if !paginatesNatively([]mcpParamBinding{
		{PublicName: "offset", WireName: "offset", Location: "query"},
		{PublicName: "limit", WireName: "limit", Location: "query"},
	}) {
		t.Fatalf("offset+limit tool must be treated as natively paged")
	}
	// limit-only (e.g. get_notifications): natively paged.
	if !paginatesNatively([]mcpParamBinding{{PublicName: "limit", WireName: "limit", Location: "query"}}) {
		t.Fatalf("limit-only tool must be treated as natively paged")
	}
}
