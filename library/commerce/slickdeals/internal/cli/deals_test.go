// Copyright 2026 david. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/slickdeals/internal/store"
)

// dealsTestStore seeds a tmp DB with a handful of snapshots and returns the
// path so the CLI command can be invoked via --db. Returns the open Store
// alongside in case the test wants to assert against it directly; tests
// should Close it before they finish.
func dealsTestStore(t *testing.T) (string, *store.Store) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "snap.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	now := time.Now().UTC()
	seeds := []store.DealSnapshot{
		{DealID: "1", CapturedAt: now.Add(-3 * time.Hour), Merchant: "costco", Category: "tech", Thumbs: 100, Title: "A", Link: "l1"},
		{DealID: "1", CapturedAt: now.Add(-30 * time.Minute), Merchant: "costco", Category: "tech", Thumbs: 120, Title: "A", Link: "l1"},
		{DealID: "2", CapturedAt: now.Add(-2 * time.Hour), Merchant: "amazon", Category: "home", Thumbs: 50, Title: "B", Link: "l2"},
		{DealID: "3", CapturedAt: now.Add(-30 * 24 * time.Hour), Merchant: "stale", Category: "tech", Thumbs: 5, Title: "C", Link: "l3"},
	}
	for i := range seeds {
		if err := s.InsertSnapshot(&seeds[i]); err != nil {
			t.Fatalf("insert seed %d: %v", i, err)
		}
	}
	return dbPath, s
}

// runDeals invokes the deals command in-process via the cobra tree, capturing
// stdout. Returns the parsed envelope ({results,meta}) for assertions.
func runDeals(t *testing.T, dbPath string, extraArgs ...string) (any, []byte) {
	t.Helper()
	flags := &rootFlags{asJSON: true}
	cmd := newDealsCmd(flags)
	var buf, stderrBuf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&stderrBuf) // separate so JSON envelope isn't polluted by empty-result hint
	cmd.SetContext(context.Background())
	// flags.asJSON is set directly above; --json is a root persistent flag
	// that isn't visible on the isolated sub-command in tests.
	args := append([]string{"--db", dbPath}, extraArgs...)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	var env map[string]any
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatalf("decode envelope: %v\n%s", err, buf.String())
	}
	return env, buf.Bytes()
}

func resultsSlice(t *testing.T, env any) []any {
	t.Helper()
	m, ok := env.(map[string]any)
	if !ok {
		t.Fatalf("envelope not a map: %T", env)
	}
	results, ok := m["results"].([]any)
	if !ok {
		t.Fatalf("results not a slice: %T", m["results"])
	}
	return results
}

func TestDealsCmd_StoreFilter(t *testing.T) {
	dbPath, s := dealsTestStore(t)
	defer s.Close()

	env, _ := runDeals(t, dbPath, "--store", "costco")
	results := resultsSlice(t, env)
	if len(results) != 1 {
		// Latest=true by default: 2 costco snapshots collapse to 1 deal.
		t.Fatalf("len=%d want 1 (latest=true dedupes 2 costco snapshots)", len(results))
	}
	row := results[0].(map[string]any)
	if row["merchant"] != "costco" || int(row["thumbs"].(float64)) != 120 {
		t.Fatalf("expected latest costco snapshot with thumbs=120, got %+v", row)
	}
}

func TestDealsCmd_LatestFalseReturnsAllObservations(t *testing.T) {
	dbPath, s := dealsTestStore(t)
	defer s.Close()

	env, _ := runDeals(t, dbPath, "--store", "costco", "--latest=false")
	results := resultsSlice(t, env)
	if len(results) != 2 {
		t.Fatalf("len=%d want 2 (latest=false should keep both costco snapshots)", len(results))
	}
}

func TestDealsCmd_CategoryFilter(t *testing.T) {
	dbPath, s := dealsTestStore(t)
	defer s.Close()

	env, _ := runDeals(t, dbPath, "--category", "tech")
	results := resultsSlice(t, env)
	// Latest=true: deals 1 (costco) and 3 (stale) both tech → 2 deduped rows.
	if len(results) != 2 {
		t.Fatalf("len=%d want 2", len(results))
	}
}

func TestDealsCmd_MinThumbsFilter(t *testing.T) {
	dbPath, s := dealsTestStore(t)
	defer s.Close()

	env, _ := runDeals(t, dbPath, "--min-thumbs", "100")
	results := resultsSlice(t, env)
	if len(results) != 1 {
		t.Fatalf("len=%d want 1 (only deal 1 latest >= 100)", len(results))
	}
}

func TestDealsCmd_DealIDFilter(t *testing.T) {
	dbPath, s := dealsTestStore(t)
	defer s.Close()

	env, _ := runDeals(t, dbPath, "--deal-id", "2")
	results := resultsSlice(t, env)
	if len(results) != 1 {
		t.Fatalf("len=%d want 1", len(results))
	}
	if got := results[0].(map[string]any)["deal_id"]; got != "2" {
		t.Fatalf("deal_id=%v want 2", got)
	}
}

func TestDealsCmd_SinceFilter(t *testing.T) {
	dbPath, s := dealsTestStore(t)
	defer s.Close()

	env, _ := runDeals(t, dbPath, "--since", "24h")
	results := resultsSlice(t, env)
	// Within 24h: deal 1 (latest 30 min ago) + deal 2 (2h ago). Deal 3 is 30d.
	if len(results) != 2 {
		t.Fatalf("len=%d want 2", len(results))
	}
}

func TestDealsCmd_EmptyResultIsNotError(t *testing.T) {
	dbPath, s := dealsTestStore(t)
	defer s.Close()

	env, raw := runDeals(t, dbPath, "--deal-id", "nonexistent")
	results := resultsSlice(t, env)
	if len(results) != 0 {
		t.Fatalf("expected empty results, got %d\n%s", len(results), string(raw))
	}
}

func TestDealsCmd_DryRunShortCircuits(t *testing.T) {
	flags := &rootFlags{asJSON: true, dryRun: true}
	cmd := newDealsCmd(flags)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetContext(context.Background())
	cmd.SetArgs([]string{"--store", "costco"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("dry-run should succeed: %v", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("dry-run should produce no output, got %q", buf.String())
	}
}

func TestDealsCmd_ProvenanceMeta(t *testing.T) {
	dbPath, s := dealsTestStore(t)
	defer s.Close()

	env, _ := runDeals(t, dbPath, "--store", "costco")
	m := env.(map[string]any)
	meta, ok := m["meta"].(map[string]any)
	if !ok {
		t.Fatalf("missing meta: %+v", env)
	}
	if meta["source"] != "local" {
		t.Fatalf("source=%v want local", meta["source"])
	}
	if meta["resource_type"] != "deals" {
		t.Fatalf("resource_type=%v want deals", meta["resource_type"])
	}
	if _, ok := meta["synced_at"]; !ok {
		t.Fatalf("expected synced_at in meta: %+v", meta)
	}
}
