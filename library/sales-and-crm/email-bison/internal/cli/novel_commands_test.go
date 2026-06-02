// Copyright 2026 riccardovandra and contributors. Licensed under Apache-2.0. See LICENSE.
// Tests that the novel commands are wired under the correct parents and honor --dry-run.

package cli

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
)

func findSub(parent *cobra.Command, name string) *cobra.Command {
	for _, c := range parent.Commands() {
		if c.Name() == name {
			return c
		}
	}
	return nil
}

func TestNovelCommandWiring(t *testing.T) {
	root := newRootCmd(&rootFlags{})

	checks := []struct {
		parent string
		child  string
	}{
		{"campaigns", "headroom"},
		{"campaigns", "preflight"},
		{"campaigns", "variants"},
		{"leads", "stale"},
		{"replies", "interested"},
		{"replies", "triage"},
		{"sender-emails", "health"},
	}
	for _, c := range checks {
		parent := findSub(root, c.parent)
		if parent == nil {
			t.Errorf("parent command %q not found under root", c.parent)
			continue
		}
		if findSub(parent, c.child) == nil {
			t.Errorf("novel command %q not wired under %q", c.child, c.parent)
		}
	}

	// There must be exactly one top-level "replies" command (no duplicate stub).
	count := 0
	for _, cmd := range root.Commands() {
		if cmd.Name() == "replies" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly one top-level replies command, found %d", count)
	}

	// The orphan top-level "senders" stub must be gone.
	if findSub(root, "senders") != nil {
		t.Errorf("orphan top-level 'senders' command should have been removed")
	}
}

func TestNovelCommandsDryRun(t *testing.T) {
	invocations := [][]string{
		{"campaigns", "headroom", "--dry-run"},
		{"sender-emails", "health", "--dry-run"},
		{"replies", "interested", "--dry-run"},
		{"replies", "triage", "--dry-run"},
		{"leads", "stale", "--dry-run"},
		{"campaigns", "preflight", "6", "--dry-run"},
		{"campaigns", "variants", "6", "--dry-run"},
	}
	for _, args := range invocations {
		root := newRootCmd(&rootFlags{})
		var buf bytes.Buffer
		root.SetOut(&buf)
		root.SetErr(&buf)
		root.SetArgs(args)
		if err := root.Execute(); err != nil {
			t.Errorf("dry-run %v returned error: %v (output: %s)", args, err, buf.String())
		}
	}
}
