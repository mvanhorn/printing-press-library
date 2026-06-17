// Copyright 2026 slinsmaier and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestNewNovelAlertCmdValidation(t *testing.T) {
	flags := &rootFlags{}
	cmd := newNovelAlertCmd(flags)
	if err := cmd.Flags().Parse([]string{"--metric", "readiness", "--direction", "sideways"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if err := cmd.RunE(cmd, []string{}); err == nil {
		t.Error("expected usage error for invalid --direction")
	}
}

func TestNewNovelAlertCmdDryRun(t *testing.T) {
	flags := &rootFlags{dryRun: true}
	cmd := newNovelAlertCmd(flags)
	if err := cmd.Flags().Parse([]string{"--metric", "readiness"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if err := cmd.RunE(cmd, []string{}); err != nil {
		t.Errorf("dry-run should succeed, got: %v", err)
	}
}

func TestNewNovelAlertCmdBareHelp(t *testing.T) {
	flags := &rootFlags{}
	cmd := newNovelAlertCmd(flags)
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Errorf("bare invocation should show help, got error: %v", err)
	}
}
