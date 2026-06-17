// Copyright 2026 slinsmaier and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestValuesInRange(t *testing.T) {
	series := map[string]float64{
		"2026-06-01": 1,
		"2026-06-05": 2,
		"2026-06-10": 3,
	}
	got := valuesInRange(series, "2026-06-02", "2026-06-09")
	if len(got) != 1 || got[0] != 2 {
		t.Errorf("got %v, want [2]", got)
	}
}

func TestNewNovelHrvTrendCmdDryRun(t *testing.T) {
	flags := &rootFlags{dryRun: true}
	cmd := newNovelHrvTrendCmd(flags)
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Errorf("dry-run should succeed, got: %v", err)
	}
}
