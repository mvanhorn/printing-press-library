// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// The wiring smoke test is KEPT verbatim from the scaffold; the behaviour
// cases live in nepra_capacity_test.go.

package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestNovelCapacityHelpWires smoke-tests that the capacity command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelCapacityHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"capacity", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("capacity --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "capacity"} {
		if !strings.Contains(help, want) {
			t.Fatalf("capacity --help missing %q in output:\n%s", want, help)
		}
	}
	// The command is IMPLEMENTED, so --help must document its real flags and
	// the scaffold annotation must be gone: root.go's
	// preferImplementedNovelCommands keys on "pp:novel-scaffold" and would
	// treat an implemented command carrying it as a stub.
	for _, want := range []string{"--as-of", "--by", "--strict", "status", "technology", "no in-year"} {
		if !strings.Contains(help, want) {
			t.Errorf("capacity --help missing %q in output:\n%s", want, help)
		}
	}
	var found bool
	for _, c := range RootCmd().Commands() {
		if c.Name() != "capacity" {
			continue
		}
		found = true
		if _, scaffold := c.Annotations[novelScaffoldAnnotation]; scaffold {
			t.Error("capacity still carries the pp:novel-scaffold annotation")
		}
		for k, v := range map[string]string{
			"mcp:read-only":       "true",
			"pp:happy-args":       "--as-of=2024-06-30",
			"pp:typed-exit-codes": "true",
			"pp:novel-hand-coded": "true",
		} {
			if c.Annotations[k] != v {
				t.Errorf("annotation %q = %q, want %q", k, c.Annotations[k], v)
			}
		}
	}
	if !found {
		t.Fatal("capacity is not registered on the root command")
	}
}
