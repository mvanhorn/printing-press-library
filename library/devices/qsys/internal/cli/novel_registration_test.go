// Copyright 2026 drummerms and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestNovelLeafCommandsResolveAsLeaves is the regression test for the wiring
// bug this tree shipped with: a novel subcommand whose parent was built twice
// lost the race in addNovelCommandIfAbsent and fell through to the parent, so
// `<binary> bom risks --help` silently printed the `bom` group's help instead.
//
// A leaf's Usage block is one line ending in "[flags]". A parent's ends in
// "[command]", which is what fall-through looks like from the outside.
func TestNovelLeafCommandsResolveAsLeaves(t *testing.T) {
	for _, path := range [][]string{
		{"product", "get"},
		{"bom", "verify"},
		{"bom", "risks"},
		{"compat", "check"},
		{"qds"},
		{"fault"},
		{"connect"},
		{"coverage"},
	} {
		name := strings.Join(path, " ")
		t.Run(name, func(t *testing.T) {
			root := RootCmd()
			var out bytes.Buffer
			root.SetOut(&out)
			root.SetErr(&out)
			root.SetArgs(append(append([]string{}, path...), "--help"))
			if err := root.Execute(); err != nil {
				t.Fatalf("%s --help error = %v", name, err)
			}
			usage := usageSpec(t, out.String())
			wantSuffix := "qsys-pp-cli " + name
			if !strings.HasPrefix(usage, wantSuffix) {
				t.Fatalf("%s --help usage line = %q, want it to start with %q", name, usage, wantSuffix)
			}
			if !strings.HasSuffix(usage, "[flags]") {
				t.Fatalf("%s --help usage line = %q, want it to end in [flags]", name, usage)
			}
			if strings.Contains(out.String(), "Available Commands:") {
				t.Fatalf("%s resolved to a parent group, not a leaf:\n%s", name, out.String())
			}
		})
	}
}

// usageSpec returns the first line of the Usage: block.
func usageSpec(t *testing.T, help string) string {
	t.Helper()
	lines := strings.Split(help, "\n")
	for i, l := range lines {
		if strings.TrimSpace(l) == "Usage:" && i+1 < len(lines) {
			return strings.TrimSpace(lines[i+1])
		}
	}
	t.Fatalf("no Usage: block in help output:\n%s", help)
	return ""
}

// TestBomParentCarriesBothChildren pins the reconciliation: one bom parent,
// both subcommands. Building the parent in either child's file is what let one
// of them go missing.
func TestBomParentCarriesBothChildren(t *testing.T) {
	root := RootCmd()
	bom, _, err := root.Find([]string{"bom"})
	if err != nil {
		t.Fatalf("root.Find(bom) error = %v", err)
	}
	got := map[string]bool{}
	for _, c := range bom.Commands() {
		got[c.Name()] = true
	}
	for _, want := range []string{"verify", "risks"} {
		if !got[want] {
			t.Fatalf("bom is missing subcommand %q; has %v", want, got)
		}
	}
}

// TestDroppedCommandsAreGone guards the absorb manifest: `integrations` and
// `compat deprecated` were dropped, while the generated `compat deprecations`
// endpoint is a different feature that stays.
func TestDroppedCommandsAreGone(t *testing.T) {
	root := RootCmd()
	for _, path := range [][]string{{"integrations"}, {"compat", "deprecated"}} {
		cmd, _, err := root.Find(path)
		if err == nil && cmd != nil && cmd.Name() == path[len(path)-1] {
			t.Fatalf("%v still resolves; it was dropped by the absorb manifest", path)
		}
	}
	if cmd, _, err := root.Find([]string{"compat", "deprecations"}); err != nil || cmd.Name() != "deprecations" {
		t.Fatalf("compat deprecations must stay registered; find err = %v", err)
	}
}
