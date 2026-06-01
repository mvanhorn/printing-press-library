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
