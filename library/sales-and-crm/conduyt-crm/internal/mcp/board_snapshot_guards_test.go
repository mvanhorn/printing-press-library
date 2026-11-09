package mcp

import (
	"net/url"
	"testing"
)

// Board snapshot (board #116 for app #111): the executor's deals.list refuses the snapshot's conflicts as the CLI does
// (hand-written; a reprint must keep it) and sends a valid snapshot as query parameters.
const (
	bsPipeID  = "9a0c6b8e-3f1d-4c2a-8e7b-5d4c3b2a1f00"
	bsStageID = "44444444-4444-4444-8444-444444444441"
)

func TestCodeOrchDealsListBoardSnapshotIsCheckedThenSent(t *testing.T) {
	base := func() map[string]any {
		return map[string]any{"board_snapshot": true, "pipeline": bsPipeID, "board_columns": bsStageID + ":25"}
	}
	for name, mutate := range map[string]func(map[string]any){
		"page 1":           func(p map[string]any) { p["page"] = float64(1) },
		"stage":            func(p map[string]any) { p["stage"] = bsStageID },
		"stageName":        func(p map[string]any) { p["stageName"] = "Won" },
		"rows_only":        func(p map[string]any) { p["rows_only"] = true },
		"summary":          func(p map[string]any) { p["summary"] = true },
		"health_summary":   func(p map[string]any) { p["health_summary"] = true },
		"blank pipeline":   func(p map[string]any) { p["pipeline"] = "" },
		"spaces pipeline":  func(p map[string]any) { delete(p, "pipeline"); p["pipeline_name"] = "  " },
		"number pipeline":  func(p map[string]any) { p["pipeline"] = float64(7) },
		"no pipeline":      func(p map[string]any) { delete(p, "pipeline") },
		"no columns":       func(p map[string]any) { delete(p, "board_columns") },
		"bad columns":      func(p map[string]any) { p["board_columns"] = bsStageID + ":0" },
		"string snapshot":  func(p map[string]any) { p["board_snapshot"] = "true" },
		"columns, no mode": func(p map[string]any) { delete(p, "board_snapshot") },
	} {
		p := base()
		mutate(p)
		hits, _, _, res := execGuard(t, "deals.list", p)
		if hits != 0 || !res.IsError {
			t.Errorf("%s: hits=%d isError=%v", name, hits, res.IsError)
		}
	}
	hits, _, rawQuery, res := execGuard(t, "deals.list", base())
	if hits != 1 || res.IsError {
		t.Fatalf("valid snapshot: hits=%d isError=%v", hits, res.IsError)
	}
	q, _ := url.ParseQuery(rawQuery)
	if q.Get("board_snapshot") != "true" || q.Get("pipeline") != bsPipeID || q.Get("board_columns") != bsStageID+":25" || len(q) != 3 {
		t.Fatalf("query = %s", rawQuery)
	}
	// the plain list is untouched
	hits, _, rawQuery, res = execGuard(t, "deals.list", map[string]any{"page": float64(2), "stage": bsStageID})
	if hits != 1 || res.IsError {
		t.Fatalf("plain list: hits=%d isError=%v query=%s", hits, res.IsError, rawQuery)
	}
}
