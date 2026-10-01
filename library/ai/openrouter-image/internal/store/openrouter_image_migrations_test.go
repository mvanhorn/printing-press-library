// Copyright 2026 neal-kyle and contributors. Licensed under Apache-2.0. See LICENSE.
// Behavior tests for the custom openrouter-image store tables: generation
// ledger round-trip, endpoint pricing cache, and catalog snapshot baseline.

package store

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.EnsureOpenRouterImageTables(context.Background()); err != nil {
		t.Fatalf("EnsureOpenRouterImageTables: %v", err)
	}
	return db
}

func TestGenerationLedgerRoundTrip(t *testing.T) {
	db := openTestStore(t)
	ctx := context.Background()

	entry := GenerationEntry{
		ID:         "gen-1",
		Model:      "openai/gpt-image-1",
		Prompt:     "a red panda",
		Params:     `{"model":"openai/gpt-image-1","prompt":"a red panda","n":1}`,
		CostUSD:    0.02,
		Tokens:     `{"total":100}`,
		OutputPath: "/tmp/panda.png",
		CreatedAt:  time.Now().UTC(),
	}
	if err := db.LedgerGeneration(ctx, entry); err != nil {
		t.Fatalf("LedgerGeneration: %v", err)
	}

	got, err := db.GetGeneration(ctx, "gen-1")
	if err != nil {
		t.Fatalf("GetGeneration: %v", err)
	}
	if got == nil {
		t.Fatal("GetGeneration returned nil")
	}
	if got.Model != entry.Model || got.CostUSD != entry.CostUSD || got.OutputPath != entry.OutputPath {
		t.Errorf("round-trip mismatch: %+v", got)
	}

	missing, err := db.GetGeneration(ctx, "gen-nope")
	if err != nil {
		t.Fatalf("GetGeneration missing: %v", err)
	}
	if missing != nil {
		t.Errorf("expected nil for missing id, got %+v", missing)
	}
}

func TestGenerationLedgerZeroTimestampUsesCurrentTime(t *testing.T) {
	db := openTestStore(t)
	ctx := context.Background()
	before := time.Now().UTC().Add(-time.Second)

	if err := db.LedgerGeneration(ctx, GenerationEntry{ID: "gen-zero-time", Model: "model"}); err != nil {
		t.Fatalf("LedgerGeneration: %v", err)
	}
	entries, err := db.ListGenerations(ctx, before, 10)
	if err != nil {
		t.Fatalf("ListGenerations: %v", err)
	}
	if len(entries) != 1 || entries[0].ID != "gen-zero-time" {
		t.Fatalf("recent entries = %+v, want gen-zero-time", entries)
	}
	if entries[0].CreatedAt.IsZero() || entries[0].CreatedAt.Year() == 1 {
		t.Fatalf("stored timestamp = %v, want current database timestamp", entries[0].CreatedAt)
	}
}

func TestListGenerationsOrdersMixedTimestampFormatsByTime(t *testing.T) {
	db := openTestStore(t)
	ctx := context.Background()
	for _, row := range []struct{ id, createdAt string }{
		{"legacy-early", "2026-10-01T09:00:00Z"},
		{"database-late", "2026-10-01 13:00:00"},
		{"database-fraction", "2026-10-01 13:00:00.100"},
		{"legacy-fraction", "2026-10-01T13:00:00.900Z"},
	} {
		if _, err := db.db.ExecContext(ctx,
			"INSERT INTO generation_ledger (id, model, created_at) VALUES (?, ?, ?)",
			row.id, "model", row.createdAt); err != nil {
			t.Fatalf("seed %s: %v", row.id, err)
		}
	}
	sameDayNoon := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	recent, err := db.ListGenerations(ctx, sameDayNoon, 10)
	if err != nil {
		t.Fatalf("ListGenerations recent: %v", err)
	}
	if len(recent) != 3 || recent[0].ID != "legacy-fraction" || recent[1].ID != "database-fraction" || recent[2].ID != "database-late" {
		t.Fatalf("recent generations = %+v, want fractional timestamps newest first", recent)
	}
	if recent[0].CreatedAt.Nanosecond() != 900000000 {
		t.Fatalf("fractional timestamp lost: %v", recent[0].CreatedAt)
	}
	latest, err := db.ListGenerations(ctx, sameDayNoon, 1)
	if err != nil {
		t.Fatalf("ListGenerations latest: %v", err)
	}
	if len(latest) != 1 || latest[0].ID != "legacy-fraction" {
		t.Fatalf("limited latest generation = %+v, want legacy-fraction", latest)
	}
	afterHalfSecond := time.Date(2026, 10, 1, 13, 0, 0, 500000000, time.UTC)
	filtered, err := db.ListGenerations(ctx, afterHalfSecond, 10)
	if err != nil {
		t.Fatalf("ListGenerations fractional filter: %v", err)
	}
	if len(filtered) != 1 || filtered[0].ID != "legacy-fraction" {
		t.Fatalf("fractional filter = %+v, want only legacy-fraction", filtered)
	}
	all, err := db.ListGenerations(ctx, sameDayNoon.Add(-4*time.Hour), 10)
	if err != nil {
		t.Fatalf("ListGenerations all: %v", err)
	}
	if len(all) != 4 || all[0].ID != "legacy-fraction" || all[1].ID != "database-fraction" || all[2].ID != "database-late" || all[3].ID != "legacy-early" {
		t.Fatalf("generation order = %+v, want timestamp order across both formats", all)
	}
}

func TestListGenerationsUsesNormalizedTimestampIndex(t *testing.T) {
	db := openTestStore(t)
	rows, err := db.db.QueryContext(context.Background(),
		`EXPLAIN QUERY PLAN SELECT id FROM generation_ledger
		 WHERE replace(replace(created_at, 'T', ' '), 'Z', '') >= ?
		 ORDER BY replace(replace(created_at, 'T', ' '), 'Z', '') DESC LIMIT ?`,
		"2026-10-01 00:00:00", 1)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var id, parent, unused int
		var detail string
		if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(detail, "idx_generation_ledger_created_normalized") {
			return
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	t.Fatal("ledger query plan did not use the normalized timestamp index")
}

func TestListGenerationsNewestFirst(t *testing.T) {
	db := openTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()

	old := GenerationEntry{ID: "gen-old", Model: "m1", Prompt: "p", CreatedAt: now.Add(-48 * time.Hour)}
	new := GenerationEntry{ID: "gen-new", Model: "m2", Prompt: "q", CreatedAt: now}
	for _, e := range []GenerationEntry{old, new} {
		if err := db.LedgerGeneration(ctx, e); err != nil {
			t.Fatal(err)
		}
	}

	// Window of 7 days should return both, newest first.
	got, err := db.ListGenerations(ctx, now.Add(-7*24*time.Hour), 10)
	if err != nil {
		t.Fatalf("ListGenerations: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d rows, want 2", len(got))
	}
	if got[0].ID != "gen-new" || got[1].ID != "gen-old" {
		t.Errorf("ordering wrong: %s then %s", got[0].ID, got[1].ID)
	}
}

func TestEndpointCacheRoundTrip(t *testing.T) {
	db := openTestStore(t)
	ctx := context.Background()

	data := json.RawMessage(`{"id":"openai/gpt-image-1","endpoints":[]}`)
	if err := db.PutEndpointCache(ctx, "openai/gpt-image-1", data); err != nil {
		t.Fatalf("PutEndpointCache: %v", err)
	}
	got, err := db.GetEndpointCache(ctx, "openai/gpt-image-1")
	if err != nil {
		t.Fatalf("GetEndpointCache: %v", err)
	}
	if got == nil {
		t.Fatal("GetEndpointCache returned nil")
	}
	if string(got.Data) != string(data) {
		t.Errorf("data mismatch: %s vs %s", got.Data, data)
	}

	missing, err := db.GetEndpointCache(ctx, "nope/nope")
	if err != nil {
		t.Fatalf("GetEndpointCache missing: %v", err)
	}
	if missing != nil {
		t.Errorf("expected nil for missing, got %+v", missing)
	}
}

func TestCatalogSnapshotBaseline(t *testing.T) {
	db := openTestStore(t)
	ctx := context.Background()

	// No snapshot initially.
	prior, err := db.GetCatalogSnapshot(ctx)
	if err != nil {
		t.Fatalf("GetCatalogSnapshot: %v", err)
	}
	if prior != nil {
		t.Fatal("expected nil snapshot before first baseline")
	}

	snap := []map[string]any{
		{"id": "a/b", "name": "A"},
		{"id": "c/d", "name": "C"},
	}
	if err := db.PutCatalogSnapshot(ctx, snap); err != nil {
		t.Fatalf("PutCatalogSnapshot: %v", err)
	}

	got, err := db.GetCatalogSnapshot(ctx)
	if err != nil {
		t.Fatalf("GetCatalogSnapshot after write: %v", err)
	}
	if got == nil {
		t.Fatal("snapshot nil after write")
	}
	if len(got.Snapshot) != 2 {
		t.Errorf("snapshot len = %d, want 2", len(got.Snapshot))
	}
	if got.Snapshot[0]["id"] != "a/b" {
		t.Errorf("snapshot[0] = %v", got.Snapshot[0])
	}
}
