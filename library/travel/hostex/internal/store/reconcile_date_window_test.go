// Copyright 2026 bust011r and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// Hostex filters /transactions by the operator-local date and stamps action_at
// with that local offset, so the first 10 characters are the date the API
// windows on. Rows near midnight must land on the operator's side of the
// window boundary, not the UTC side.
func TestReconcileDateWindowUsesOperatorLocalDate(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	rows := map[string]string{
		"local-in-utc-out": "2026-09-30T01:30:00+02:00", // UTC date 09-29, operator date 09-30: in the window
		"local-out-utc-in": "2026-09-29T23:30:00-05:00", // UTC date 09-30, operator date 09-29: outside the window
		"no-date":          "",
	}
	for id, at := range rows {
		data := `{"id":"` + id + `"}`
		if at != "" {
			data = `{"id":"` + id + `","action_at":"` + at + `"}`
		}
		if err := db.Upsert("transactions", id, json.RawMessage(data)); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	if _, err := db.ReconcileDateWindow("transactions", "$.action_at", "2026-09-30", "2026-09-30", nil, "", nil); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	left, _ := db.List("transactions", 10)
	got := map[string]bool{}
	for _, r := range left {
		var o struct {
			ID string `json:"id"`
		}
		_ = json.Unmarshal(r, &o)
		got[o.ID] = true
	}
	if got["local-in-utc-out"] {
		t.Errorf("row dated 09-30 in the operator's timezone should be pruned: %v", got)
	}
	if !got["local-out-utc-in"] || !got["no-date"] {
		t.Errorf("rows outside the window or without a date must survive: %v", got)
	}
}
