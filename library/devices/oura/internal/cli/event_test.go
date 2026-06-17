// Copyright 2026 slinsmaier and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestNewNovelEventCmdRequiresDate(t *testing.T) {
	flags := &rootFlags{}
	cmd := newNovelEventCmd(flags)
	if err := cmd.Flags().Parse([]string{"--window", "2"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if err := cmd.RunE(cmd, []string{}); err == nil {
		t.Error("expected usage error when --date is missing")
	}
}

func TestNewNovelEventCmdDryRun(t *testing.T) {
	flags := &rootFlags{dryRun: true}
	cmd := newNovelEventCmd(flags)
	if err := cmd.Flags().Parse([]string{"--date", "2026-06-01"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if err := cmd.RunE(cmd, []string{}); err != nil {
		t.Errorf("dry-run should succeed, got: %v", err)
	}
}
