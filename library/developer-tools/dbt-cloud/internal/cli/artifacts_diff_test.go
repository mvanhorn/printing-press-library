// Copyright 2026 Nimrod Astarhan and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"math"
	"testing"
)

func TestDiffIsFailStatus(t *testing.T) {
	cases := []struct{ status string; want bool }{
		{"error", true},
		{"fail", true},
		{"success", false},
		{"pass", false},
		{"skipped", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := diffIsFailStatus(tc.status); got != tc.want {
			t.Errorf("diffIsFailStatus(%q) = %v, want %v", tc.status, got, tc.want)
		}
	}
}

func TestDiffIsPassStatus(t *testing.T) {
	cases := []struct{ status string; want bool }{
		{"success", true},
		{"pass", true},
		{"error", false},
		{"fail", false},
		{"skipped", false},
	}
	for _, tc := range cases {
		if got := diffIsPassStatus(tc.status); got != tc.want {
			t.Errorf("diffIsPassStatus(%q) = %v, want %v", tc.status, got, tc.want)
		}
	}
}

func TestComputeArtifactsDiff_NewlyFailed(t *testing.T) {
	modelsA := map[string]ModelResult{
		"model.foo.bar": {UniqueID: "model.foo.bar", Status: "success", ExecutionTime: 10},
	}
	modelsB := map[string]ModelResult{
		"model.foo.bar": {UniqueID: "model.foo.bar", Status: "error", ExecutionTime: 5},
	}
	diff := computeArtifactsDiff("1", "2", modelsA, modelsB, 999)
	if diff.Summary.NewlyFailedCount != 1 {
		t.Errorf("expected 1 newly failed, got %d", diff.Summary.NewlyFailedCount)
	}
	if diff.Summary.NewlyPassedCount != 0 {
		t.Errorf("expected 0 newly passed, got %d", diff.Summary.NewlyPassedCount)
	}
}

func TestComputeArtifactsDiff_NewlyPassed(t *testing.T) {
	modelsA := map[string]ModelResult{
		"model.foo.baz": {UniqueID: "model.foo.baz", Status: "error", ExecutionTime: 15},
	}
	modelsB := map[string]ModelResult{
		"model.foo.baz": {UniqueID: "model.foo.baz", Status: "success", ExecutionTime: 12},
	}
	diff := computeArtifactsDiff("1", "2", modelsA, modelsB, 999)
	if diff.Summary.NewlyPassedCount != 1 {
		t.Errorf("expected 1 newly passed, got %d", diff.Summary.NewlyPassedCount)
	}
}

func TestComputeArtifactsDiff_TimingDelta(t *testing.T) {
	modelsA := map[string]ModelResult{
		"model.foo.slow": {UniqueID: "model.foo.slow", Status: "success", ExecutionTime: 10},
	}
	modelsB := map[string]ModelResult{
		"model.foo.slow": {UniqueID: "model.foo.slow", Status: "success", ExecutionTime: 50},
	}
	diff := computeArtifactsDiff("1", "2", modelsA, modelsB, 10.0)
	if diff.Summary.TimingDeltaCount != 1 {
		t.Errorf("expected 1 timing delta, got %d", diff.Summary.TimingDeltaCount)
	}
	delta := diff.TimingDeltas[0].DeltaSec
	if math.Abs(delta-40.0) > 0.01 {
		t.Errorf("expected delta 40s, got %.2f", delta)
	}
}

func TestComputeArtifactsDiff_NoChange(t *testing.T) {
	modelsA := map[string]ModelResult{
		"model.a": {UniqueID: "model.a", Status: "success", ExecutionTime: 20},
		"model.b": {UniqueID: "model.b", Status: "error", ExecutionTime: 5},
	}
	modelsB := map[string]ModelResult{
		"model.a": {UniqueID: "model.a", Status: "success", ExecutionTime: 22},
		"model.b": {UniqueID: "model.b", Status: "error", ExecutionTime: 5},
	}
	diff := computeArtifactsDiff("1", "2", modelsA, modelsB, 10.0)
	if diff.Summary.NewlyFailedCount != 0 {
		t.Errorf("expected 0 newly failed, got %d", diff.Summary.NewlyFailedCount)
	}
	if diff.Summary.NewlyPassedCount != 0 {
		t.Errorf("expected 0 newly passed, got %d", diff.Summary.NewlyPassedCount)
	}
	// 2s delta is below 10s threshold
	if diff.Summary.TimingDeltaCount != 0 {
		t.Errorf("expected 0 timing deltas, got %d", diff.Summary.TimingDeltaCount)
	}
}

func TestArtifactsDiffCommandAnnotations(t *testing.T) {
	flags := &rootFlags{}
	cmd := newNovelArtifactsDiffCmd(flags)
	if cmd == nil {
		t.Fatal("artifacts diff command should not be nil")
	}
	if cmd.Annotations["mcp:read-only"] != "true" {
		t.Errorf("artifacts diff should be mcp:read-only=true, got %q", cmd.Annotations["mcp:read-only"])
	}
}
