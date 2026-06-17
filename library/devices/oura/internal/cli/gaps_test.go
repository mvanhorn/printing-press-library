// Copyright 2026 slinsmaier and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestNewNovelGapsCmdDryRun(t *testing.T) {
	flags := &rootFlags{dryRun: true}
	cmd := newNovelGapsCmd(flags)
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Errorf("dry-run should succeed, got: %v", err)
	}
}

func TestNewNovelGapsCmdInvalidSince(t *testing.T) {
	flags := &rootFlags{}
	cmd := newNovelGapsCmd(flags)
	if err := cmd.Flags().Parse([]string{"--since", "not-a-date"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if err := cmd.RunE(cmd, []string{}); err == nil {
		t.Error("expected usage error for invalid --since")
	}
}
