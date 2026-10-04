// Copyright 2026 bust011r and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/hostex/internal/store"
)

type fixedPageClient struct{ body string }

func (c *fixedPageClient) Get(_ context.Context, _ string, _ map[string]string) (json.RawMessage, error) {
	return json.RawMessage(c.body), nil
}

func (c *fixedPageClient) RateLimit() float64 { return 0 }

func ids(t *testing.T, db *store.Store, resource string) map[string]bool {
	t.Helper()
	rows, err := db.List(resource, 50)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	got := map[string]bool{}
	for _, r := range rows {
		var o struct {
			ID string `json:"id"`
		}
		_ = json.Unmarshal(r, &o)
		got[o.ID] = true
	}
	return got
}

// PATCH(hostex-sync-reconcile-skips-partial-windows): /transactions is fetched
// through a date window, so a full sync prunes only rows inside that window and
// leaves older backfilled history alone.
func TestFullSyncPrunesTransactionsInsideWindowOnly(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	recent := time.Now().Format("2006-01-02") + "T10:00:00+02:00"
	seed := map[string]string{
		"old-1":  "2020-01-01T10:00:00+02:00", // backfilled, outside the window
		"gone-1": recent,                      // inside the window, deleted upstream
		"kept-1": recent,                      // inside the window, still returned
	}
	for id, at := range seed {
		if err := db.Upsert("transactions", id, json.RawMessage(`{"id":"`+id+`","action_at":"`+at+`"}`)); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	body := `{"error_code":200,"data":{"transactions":[{"id":"kept-1","action_at":"` + recent + `"},{"id":"new-1","action_at":"` + recent + `"}]}}`
	var events bytes.Buffer
	syncResource(context.Background(), &fixedPageClient{body: body}, db, "transactions", "", true, 1, false, true, nil, &events)
	got := ids(t, db, "transactions")
	for _, id := range []string{"old-1", "kept-1", "new-1"} {
		if !got[id] {
			t.Errorf("transactions %q missing after full sync: %v", id, got)
		}
	}
	if got["gone-1"] {
		t.Errorf("in-window transaction deleted upstream was not pruned: %v", got)
	}
}

// A --since or --param filter cannot be bounded, so reconcile is skipped with
// exactly one skip event.
func TestFullSyncSkipsReconcileForFilteredFetch(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	if err := db.Upsert("conversations", "old-1", json.RawMessage(`{"id":"old-1"}`)); err != nil {
		t.Fatalf("seed: %v", err)
	}
	body := `{"error_code":200,"data":{"conversations":[{"id":"new-1"}]}}`
	var events bytes.Buffer
	syncResource(context.Background(), &fixedPageClient{body: body}, db, "conversations", "2026-01-01T00:00:00Z", true, 1, false, true, nil, &events)
	if got := ids(t, db, "conversations"); !got["old-1"] || !got["new-1"] {
		t.Fatalf("filtered full sync pruned rows: %v", got)
	}
	if n := strings.Count(events.String(), `"event":"reconcile_skipped"`); n != 1 {
		t.Fatalf("want exactly one reconcile_skipped event, got %d:\n%s", n, events.String())
	}
	if !strings.Contains(events.String(), "partial_window") || strings.Contains(events.String(), "unsupported-resource-shape") {
		t.Fatalf("wrong skip reason:\n%s", events.String())
	}
}

// A whole-table fetch still prunes rows the API no longer returns.
func TestFullSyncPrunesWholeTableFetch(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	if err := db.Upsert("conversations", "old-1", json.RawMessage(`{"id":"old-1"}`)); err != nil {
		t.Fatalf("seed: %v", err)
	}
	body := `{"error_code":200,"data":{"conversations":[{"id":"new-1"}]}}`
	syncResource(context.Background(), &fixedPageClient{body: body}, db, "conversations", "", true, 1, false, true, nil, nil)
	if got := ids(t, db, "conversations"); got["old-1"] || !got["new-1"] {
		t.Fatalf("whole-table full sync rows = %v, want only new-1", got)
	}
}
