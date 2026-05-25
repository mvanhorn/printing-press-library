// Copyright 2026 justinwfu. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestJobsListEmpty verifies jobs list on a fresh store renders the empty-
// state hint and exits cleanly.
func TestJobsListEmpty(t *testing.T) {
	_, cleanup := withTempStore(t)
	defer cleanup()
	out, err := runCmd(t, "jobs", "list")
	if err != nil {
		t.Fatalf("jobs list: %v\nout=%s", err, out)
	}
	if !strings.Contains(out, "No sync history") {
		t.Errorf("expected empty-state hint, got: %s", out)
	}
}

// TestJobsListAfterSyncState verifies that recording a sync via
// SaveCoffeeSyncState surfaces in jobs list with the right shape.
func TestJobsListAfterSyncState(t *testing.T) {
	s, cleanup := withTempStore(t)
	defer cleanup()

	if err := s.SaveCoffeeSyncState("shopify", "ok", 142); err != nil {
		t.Fatalf("SaveCoffeeSyncState: %v", err)
	}
	if err := s.SaveCoffeeSyncState("coffee-review", "ok", 30); err != nil {
		t.Fatalf("SaveCoffeeSyncState: %v", err)
	}

	out, err := runCmd(t, "jobs", "list", "--json")
	if err != nil {
		t.Fatalf("jobs list --json: %v\nout=%s", err, out)
	}
	var resp struct {
		Jobs []struct {
			Source       string `json:"source"`
			LastStatus   string `json:"last_status"`
			ItemCount    int    `json:"item_count"`
			LastSyncedAt string `json:"last_synced_at,omitempty"`
		} `json:"jobs"`
		Count int `json:"count"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &resp); err != nil {
		t.Fatalf("decode: %v\nout=%s", err, out)
	}
	if resp.Count < 2 {
		t.Fatalf("expected at least 2 jobs, got %d (%+v)", resp.Count, resp.Jobs)
	}
	bySource := map[string]int{}
	for _, j := range resp.Jobs {
		bySource[j.Source] = j.ItemCount
	}
	if bySource["shopify"] != 142 {
		t.Errorf("shopify: expected item_count=142, got %d", bySource["shopify"])
	}
	if bySource["coffee-review"] != 30 {
		t.Errorf("coffee-review: expected item_count=30, got %d", bySource["coffee-review"])
	}
}

// TestJobsShow verifies jobs show <source> returns one record and errors
// on an unknown source.
func TestJobsShow(t *testing.T) {
	s, cleanup := withTempStore(t)
	defer cleanup()
	if err := s.SaveCoffeeSyncState("youtube", "ok", 7); err != nil {
		t.Fatalf("SaveCoffeeSyncState: %v", err)
	}

	out, err := runCmd(t, "jobs", "show", "youtube", "--json")
	if err != nil {
		t.Fatalf("jobs show youtube: %v\nout=%s", err, out)
	}
	var rec struct {
		Source     string `json:"source"`
		LastStatus string `json:"last_status"`
		ItemCount  int    `json:"item_count"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &rec); err != nil {
		t.Fatalf("decode: %v\nout=%s", err, out)
	}
	if rec.Source != "youtube" || rec.ItemCount != 7 {
		t.Errorf("unexpected record: %+v", rec)
	}

	if _, err := runCmd(t, "jobs", "show", "definitely-not-a-source"); err == nil {
		t.Error("expected error for unknown source, got success")
	}
}
