// Copyright 2026 slinsmaier and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"math"
	"testing"
)

func TestRollingSum(t *testing.T) {
	byDay := map[string]float64{"2026-06-01": 100, "2026-06-02": 200, "2026-06-04": 50}
	got := rollingSum(byDay, "2026-06-01", "2026-06-04")
	if got != 350 {
		t.Errorf("rollingSum = %v, want 350", got)
	}
}

func TestPearsonCorrelation(t *testing.T) {
	x := []float64{1, 2, 3, 4, 5}
	y := []float64{2, 4, 6, 8, 10}
	got := pearsonCorrelation(x, y)
	if math.Abs(got-1.0) > 1e-9 {
		t.Errorf("perfect positive correlation = %v, want 1.0", got)
	}

	yNeg := []float64{10, 8, 6, 4, 2}
	got = pearsonCorrelation(x, yNeg)
	if math.Abs(got+1.0) > 1e-9 {
		t.Errorf("perfect negative correlation = %v, want -1.0", got)
	}

	if got := pearsonCorrelation(nil, nil); got != 0 {
		t.Errorf("empty input = %v, want 0", got)
	}
}

func TestNewNovelTrainingLoadCmdDryRun(t *testing.T) {
	flags := &rootFlags{dryRun: true}
	cmd := newNovelTrainingLoadCmd(flags)
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Errorf("dry-run should succeed, got: %v", err)
	}
}
