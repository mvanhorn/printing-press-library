// Copyright 2026 sidduHERE and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestHasArgMatchesExactAndEqualsForms(t *testing.T) {
	args := []string{"--json=true", "--privacy=false"}

	if !hasArg(args, "--json") {
		t.Fatal("expected --json=true to satisfy --json")
	}
	if !hasArg(args, "--privacy") {
		t.Fatal("expected --privacy=false to satisfy --privacy")
	}
}

func TestMcpConfigDelegatesWithoutInjectingClient(t *testing.T) {
	got := mcpConfigArgs(nil)
	want := []string{"mcp-config"}

	if len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("mcpConfigArgs(nil) = %v, want %v", got, want)
	}
}
