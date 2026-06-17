// Copyright 2026 slinsmaier and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestNewNovelAnomaliesCmdRequiresMetric(t *testing.T) {
	flags := &rootFlags{}
	cmd := newNovelAnomaliesCmd(flags)
	if err := cmd.Flags().Parse([]string{"--sigma", "3"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if err := cmd.RunE(cmd, []string{}); err == nil {
		t.Error("expected usage error when --metric is missing")
	}
}

func TestNewNovelAnomaliesCmdDryRun(t *testing.T) {
	flags := &rootFlags{dryRun: true}
	cmd := newNovelAnomaliesCmd(flags)
	if err := cmd.Flags().Parse([]string{"--metric", "sleep"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if err := cmd.RunE(cmd, []string{}); err != nil {
		t.Errorf("dry-run should succeed, got: %v", err)
	}
}
