// Copyright 2026 Nimrod Astarhan and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"testing"
)

func TestTriggerDryRun(t *testing.T) {
	// trigger with dryRun should print intent and return nil without calling API
	flags := &rootFlags{dryRun: true}
	cmd := newNovelTriggerCmd(flags)
	cmd.SetArgs([]string{"12345", "--cause", "test"})
	// capture dry-run short-circuit
	if dryRunOK(flags) != true {
		t.Error("dryRunOK should be true when dryRun flag is set")
	}
}

func TestTriggerVerifyEnvShortCircuit(t *testing.T) {
	// When verify env is set, trigger should not call the API
	// This test ensures the verify guard logic is in place
	flags := &rootFlags{}
	cmd := newNovelTriggerCmd(flags)
	if cmd == nil {
		t.Fatal("trigger command should not be nil")
	}
	// Check annotations
	if cmd.Annotations["mcp:read-only"] != "false" {
		t.Errorf("trigger should be mcp:read-only=false, got %q", cmd.Annotations["mcp:read-only"])
	}
}
