// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"strings"
	"testing"
	"time"
)

func TestPickJournalRecordEmptyPlan(t *testing.T) {
	_, _, err := pickJournalRecord(nil, "naertjcoixqrgrfc", "", "undo")
	if err == nil || !strings.Contains(err.Error(), "no applied journal entry") {
		t.Fatalf("empty undo: %v", err)
	}
	_, _, err = pickJournalRecord([]planEditJournalRecord{{TargetKey: "naertjcoixqrgrfc", Status: "applied"}}, "naertjcoixqrgrfc", "", "redo")
	if err == nil || !strings.Contains(err.Error(), "no undone journal entry") {
		t.Fatalf("empty redo: %v", err)
	}
}

func TestWanderlogAPIEnvelopeError(t *testing.T) {
	err := wanderlogAPIEnvelopeError([]byte(`{"success":false,"error":"ApplicationError: missing","messages":["We couldn’t find your trip plan"],"errTypes":["tripPlanNotFound"]}`))
	if err == nil || ExitCode(err) != 3 || !strings.Contains(err.Error(), "tripPlanNotFound") {
		t.Fatalf("failure envelope: %v", err)
	}
	if err := wanderlogAPIEnvelopeError([]byte(`{"success":true,"comments":[]}`)); err != nil {
		t.Fatalf("success envelope: %v", err)
	}
}

func TestSummarizeJournalRecordOmitsOpsUnlessIncludeOps(t *testing.T) {
	rec := planEditJournalRecord{
		ID:         "1-2",
		TargetKey:  "abcdefghijklmnop",
		Command:    "plan note add",
		CreatedAt:  time.Date(2026, 8, 23, 0, 0, 0, 0, time.UTC),
		Version:    9,
		Status:     "applied",
		ForwardOps: []map[string]any{{"p": []any{"itinerary", "sections", 0, "blocks", 0}, "li": map[string]any{"type": "note"}}},
		InverseOps: []map[string]any{{"p": []any{"itinerary", "sections", 0, "blocks", 0}, "ld": map[string]any{"type": "note"}}},
		OpPaths:    []string{"itinerary.sections.0.blocks.0"},
	}
	got := summarizeJournalRecord(rec, false)
	if _, ok := got["forward_ops"]; ok {
		t.Fatalf("default summary includes forward_ops: %#v", got)
	}
	if _, ok := got["inverse_ops"]; ok {
		t.Fatalf("default summary includes inverse_ops: %#v", got)
	}
	if got["id"] != rec.ID || got["status"] != "applied" {
		t.Fatalf("identity missing: %#v", got)
	}

	withOps := summarizeJournalRecord(rec, true)
	fwd, _ := withOps["forward_ops"].([]map[string]any)
	inv, _ := withOps["inverse_ops"].([]map[string]any)
	if len(fwd) != 1 || len(inv) != 1 {
		t.Fatalf("--include-ops missing ops: %#v", withOps)
	}
}
