// Copyright 2026 Nimrod Astarhan and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestFailuresCommandAnnotations(t *testing.T) {
	flags := &rootFlags{}
	cmd := newNovelFailuresCmd(flags)
	if cmd == nil {
		t.Fatal("failures command should not be nil")
	}
	if cmd.Annotations["mcp:read-only"] != "true" {
		t.Errorf("failures should be mcp:read-only=true, got %q", cmd.Annotations["mcp:read-only"])
	}
}

func TestFailuresDryRun(t *testing.T) {
	flags := &rootFlags{dryRun: true}
	if dryRunOK(flags) != true {
		t.Error("dryRunOK should be true when dryRun is set")
	}
}
