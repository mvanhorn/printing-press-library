package cli

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/payments/lunch-money/internal/store"
)

// PATCH: Local transaction reads should apply common list filters instead of returning all cached rows.
func TestResolveLocalTransactionsAppliesCommonFilters(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dbPath := filepath.Join(home, ".local", "share", "lunch-money-pp-cli", "data.db")
	s, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()
	if _, _, err := s.UpsertBatch("transactions", []json.RawMessage{
		json.RawMessage(`{"id":1,"date":"2024-01-01","category_id":10,"status":"reviewed","payee":"old"}`),
		json.RawMessage(`{"id":2,"date":"2024-02-01","category_id":20,"status":"reviewed","payee":"wrong category"}`),
		json.RawMessage(`{"id":3,"date":"2024-03-01","category_id":10,"status":"unreviewed","payee":"match"}`),
	}); err != nil {
		t.Fatalf("UpsertBatch: %v", err)
	}

	data, _, err := resolveLocal(context.Background(), "transactions", true, "/transactions", map[string]string{
		"start_date":  "2024-02-01",
		"category_id": "10",
		"limit":       "10",
	}, "user_requested")
	if err != nil {
		t.Fatalf("resolveLocal: %v", err)
	}
	var rows []map[string]any
	if err := json.Unmarshal(data, &rows); err != nil {
		t.Fatalf("unmarshal %s: %v", data, err)
	}
	if len(rows) != 1 || rows[0]["id"] != float64(3) {
		t.Fatalf("rows = %#v, want only transaction 3", rows)
	}
}

// PATCH: Doctor cache rows should report actual stored rows, not last sync batch count.
func TestCollectCacheReportUsesActualStoredRows(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dbPath := filepath.Join(home, ".local", "share", "lunch-money-pp-cli", "data.db")
	s, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, _, err := s.UpsertBatch("transactions", []json.RawMessage{
		json.RawMessage(`{"id":1,"payee":"one"}`),
		json.RawMessage(`{"id":2,"payee":"two"}`),
	}); err != nil {
		t.Fatalf("UpsertBatch: %v", err)
	}
	if err := s.SaveSyncState("transactions", "", 999); err != nil {
		t.Fatalf("SaveSyncState: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	report := collectCacheReport(context.Background(), "24h")
	resources, ok := report["resources"].([]map[string]any)
	if !ok {
		t.Fatalf("resources = %#v", report["resources"])
	}
	for _, r := range resources {
		if r["type"] == "transactions" {
			if r["rows"] != int64(2) {
				t.Fatalf("transactions rows = %#v, want 2", r["rows"])
			}
			return
		}
	}
	t.Fatalf("transactions resource missing from %#v", resources)
}

// PATCH: `search --type` must restrict local FTS results to that resource type.
func TestSearchCommandLocalTypeFilter(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	s, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, _, err := s.UpsertBatch("transactions", []json.RawMessage{
		json.RawMessage(`{"id":"tx-1","payee":"Example Brand Outlet"}`),
	}); err != nil {
		t.Fatalf("UpsertBatch transactions: %v", err)
	}
	if _, _, err := s.UpsertBatch("categories", []json.RawMessage{
		json.RawMessage(`{"id":"cat-1","name":"Example Brand category"}`),
	}); err != nil {
		t.Fatalf("UpsertBatch categories: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	stdout, stderr, err := executeForTest(t, "search", "Example Brand", "--data-source", "local", "--type", "transactions", "--db", dbPath, "--json")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q", stderr)
	}
	var env struct {
		Results []map[string]any `json:"results"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("unmarshal %q: %v", stdout, err)
	}
	if len(env.Results) != 1 || env.Results[0]["id"] != "tx-1" {
		t.Fatalf("results = %#v, want only transaction", env.Results)
	}
}
