// Copyright 2026 Farouk Umar and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// TestNovelAgentTraceHelpWires smoke-tests that the agent-trace command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelAgentTraceHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"agent-trace", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("agent-trace --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "agent-trace"} {
		if !strings.Contains(help, want) {
			t.Fatalf("agent-trace --help missing %q in output:\n%s", want, help)
		}
	}
}

// TestAgentTracePairsCallsWithResults is the core contract: the API returns a
// flat event array, and the whole point of this command is deriving how long
// each tool call took by pairing it with its result.
func TestAgentTracePairsCallsWithResults(t *testing.T) {
	events := []agentEvent{
		{
			Timestamp: "2026-07-31T12:00:00Z", Author: "agent",
			ToolCalls: []map[string]any{{"tool_call_id": "call_1", "tool_name": "search_docs"}},
		},
		{
			Timestamp: "2026-07-31T12:00:09Z", Author: "tool",
			ToolResults: []map[string]any{{"tool_call_id": "call_1", "tool_outcome": "success"}},
		},
		{Timestamp: "2026-07-31T12:00:10Z", Author: "agent", Content: "Here is the summary."},
	}
	got := buildAgentTrace("run-1", "agent-1", "sess-1", events)

	if got.ToolCalls != 1 {
		t.Fatalf("ToolCalls = %d, want 1", got.ToolCalls)
	}
	var result traceStep
	for _, s := range got.Steps {
		if s.Kind == "tool_result" {
			result = s
		}
	}
	if result.DurationSecs != 9 {
		t.Errorf("tool_result DurationSecs = %v, want 9 (derived from the call/result timestamp gap)", result.DurationSecs)
	}
	if result.ToolName != "search_docs" {
		t.Errorf("tool_result ToolName = %q, want %q (name must carry over from the call)", result.ToolName, "search_docs")
	}
	if got.TotalSecs != 10 {
		t.Errorf("TotalSecs = %v, want 10", got.TotalSecs)
	}
	if len(got.ToolStats) != 1 || got.ToolStats[0].TotalSecs != 9 {
		t.Errorf("ToolStats = %+v, want one entry totalling 9s", got.ToolStats)
	}
}

// TestAgentTraceHistogramRanksSlowest checks the histogram answers the question
// it exists for — which tool ate the wall clock — rather than listing tools in
// arrival order.
func TestAgentTraceHistogramRanksSlowest(t *testing.T) {
	events := []agentEvent{
		{Timestamp: "2026-07-31T12:00:00Z", ToolCalls: []map[string]any{{"tool_call_id": "a", "tool_name": "fast_tool"}}},
		{Timestamp: "2026-07-31T12:00:01Z", ToolResults: []map[string]any{{"tool_call_id": "a", "tool_outcome": "success"}}},
		{Timestamp: "2026-07-31T12:00:02Z", ToolCalls: []map[string]any{{"tool_call_id": "b", "tool_name": "slow_tool"}}},
		{Timestamp: "2026-07-31T12:00:20Z", ToolResults: []map[string]any{{"tool_call_id": "b", "tool_outcome": "error"}}},
	}
	got := buildAgentTrace("run-2", "agent-1", "sess-1", events)

	if len(got.ToolStats) != 2 {
		t.Fatalf("ToolStats = %d entries, want 2", len(got.ToolStats))
	}
	if got.ToolStats[0].ToolName != "slow_tool" {
		t.Errorf("ToolStats[0] = %q, want slow_tool first (slowest ranks first)", got.ToolStats[0].ToolName)
	}
	if got.ToolStats[0].TotalSecs != 18 {
		t.Errorf("slow_tool TotalSecs = %v, want 18", got.ToolStats[0].TotalSecs)
	}
	if got.ToolErrors != 1 || got.ToolStats[0].Errors != 1 {
		t.Errorf("error accounting wrong: ToolErrors=%d slow_tool.Errors=%d, want 1 and 1", got.ToolErrors, got.ToolStats[0].Errors)
	}
}

// TestAgentTraceEmptyIsHonest is the absence-of-correctness case: a run with no
// events must say so rather than rendering an empty timeline that reads as a
// clean, instant run.
func TestAgentTraceEmptyIsHonest(t *testing.T) {
	got := buildAgentTrace("run-3", "agent-1", "sess-1", nil)

	if len(got.Steps) != 0 {
		t.Fatalf("Steps = %d, want 0", len(got.Steps))
	}
	if got.Note == "" {
		t.Error("a run with no events must carry an explanatory note")
	}
	if got.TotalSecs != 0 {
		t.Errorf("TotalSecs = %v, want 0", got.TotalSecs)
	}
	blob, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, want := range []string{`"steps":[]`, `"tool_stats":[]`} {
		if !strings.Contains(string(blob), want) {
			t.Errorf("JSON missing %s (null slices break agent iteration): %s", want, blob)
		}
	}
}

// TestAgentTraceUnmatchedCallHasNoFabricatedDuration guards the case that
// matters when a run dies mid-flight: a tool call with no result must not be
// assigned an invented duration.
func TestAgentTraceUnmatchedCallHasNoFabricatedDuration(t *testing.T) {
	events := []agentEvent{
		{Timestamp: "2026-07-31T12:00:00Z", ToolCalls: []map[string]any{{"tool_call_id": "orphan", "tool_name": "hung_tool"}}},
		{Timestamp: "2026-07-31T12:00:30Z", ErrorMessage: "run terminated"},
	}
	got := buildAgentTrace("run-4", "agent-1", "sess-1", events)

	if got.ToolCalls != 1 {
		t.Fatalf("ToolCalls = %d, want 1", got.ToolCalls)
	}
	for _, s := range got.Steps {
		if s.Kind == "tool_result" {
			t.Fatalf("no tool_result should exist for an unanswered call, got %+v", s)
		}
	}
	if len(got.ToolStats) != 1 {
		t.Fatalf("ToolStats = %d, want 1", len(got.ToolStats))
	}
	if got.ToolStats[0].TotalSecs != 0 {
		t.Errorf("unanswered call TotalSecs = %v, want 0 (duration must not be invented)", got.ToolStats[0].TotalSecs)
	}
	if got.ToolStats[0].Calls != 1 {
		t.Errorf("unanswered call should still be counted: Calls = %d, want 1", got.ToolStats[0].Calls)
	}
	if got.TerminatedBy != "error" {
		t.Errorf("TerminatedBy = %q, want %q", got.TerminatedBy, "error")
	}
}

// TestAgentTraceLooseToolKeyNames covers the defensive decoding: the spec does
// not pin the tool-call item shape, so alternate key spellings must still pair.
func TestAgentTraceLooseToolKeyNames(t *testing.T) {
	events := []agentEvent{
		{Timestamp: "2026-07-31T12:00:00Z", ToolCalls: []map[string]any{{"id": "x1", "name": "alt_keys_tool"}}},
		{Timestamp: "2026-07-31T12:00:05Z", ToolResults: []map[string]any{{"id": "x1", "status": "success"}}},
	}
	got := buildAgentTrace("run-5", "agent-1", "sess-1", events)

	if len(got.ToolStats) != 1 {
		t.Fatalf("ToolStats = %+v, want a single paired tool", got.ToolStats)
	}
	if got.ToolStats[0].ToolName != "alt_keys_tool" {
		t.Errorf("ToolName = %q, want alt_keys_tool", got.ToolStats[0].ToolName)
	}
	if got.ToolStats[0].TotalSecs != 5 {
		t.Errorf("TotalSecs = %v, want 5 (pairing must work with id/name keys too)", got.ToolStats[0].TotalSecs)
	}
}
