// Copyright 2026 Kevin Magnan and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestDiffToolSurfaces(t *testing.T) {
	older := []mcpTool{
		{Name: "get_accounts", InputSchema: json.RawMessage(`{"a":1}`)},
		{Name: "get_portfolio", InputSchema: json.RawMessage(`{"b":1}`)},
		{Name: "old_tool", InputSchema: json.RawMessage(`{}`)},
	}
	newer := []mcpTool{
		{Name: "get_accounts", InputSchema: json.RawMessage(`{"a":1}`)},  // unchanged
		{Name: "get_portfolio", InputSchema: json.RawMessage(`{"b":2}`)}, // changed schema
		{Name: "brand_new_tool", InputSchema: json.RawMessage(`{}`)},     // added
	}
	d := diffToolSurfaces(older, newer)
	if len(d.Added) != 1 || d.Added[0] != "brand_new_tool" {
		t.Errorf("Added = %v, want [brand_new_tool]", d.Added)
	}
	if len(d.Removed) != 1 || d.Removed[0] != "old_tool" {
		t.Errorf("Removed = %v, want [old_tool]", d.Removed)
	}
	if len(d.Changed) != 1 || d.Changed[0] != "get_portfolio" {
		t.Errorf("Changed = %v, want [get_portfolio]", d.Changed)
	}
}

// TestNovelSurfaceDiffHelpWires smoke-tests that the surface diff command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelSurfaceDiffHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"surface", "diff", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("surface diff --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "diff"} {
		if !strings.Contains(help, want) {
			t.Fatalf("surface diff --help missing %q in output:\n%s", want, help)
		}
	}
}
