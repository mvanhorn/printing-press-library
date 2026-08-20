// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

// TestList_NonPositiveLimitReturnsAllRows pins the contract that List(rt, 0)
// (and any negative limit) returns every synced row, not an arbitrary capped
// page. Regression guard for the analytics read-cap bug: a prior `limit = 200`
// default silently truncated spend/debts/ledger/balances to ~200 of N rows
// even when the local store was complete. We insert more than the old 200-row
// cap and assert all rows come back.
func TestList_NonPositiveLimitReturnsAllRows(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	const total = 638 // mirrors the real account size that exposed the bug
	items := make([]json.RawMessage, 0, total)
	for i := 0; i < total; i++ {
		items = append(items, json.RawMessage(fmt.Sprintf(`{"id": %d}`, i)))
	}
	if _, _, err := s.UpsertBatch("get-expenses", items); err != nil {
		t.Fatalf("UpsertBatch: %v", err)
	}

	for _, limit := range []int{0, -1} {
		rows, err := s.List("get-expenses", limit)
		if err != nil {
			t.Fatalf("List(limit=%d): %v", limit, err)
		}
		if len(rows) != total {
			t.Fatalf("List(limit=%d) returned %d rows, want %d (non-positive limit must return ALL synced rows, not a 200-row cap)", limit, len(rows), total)
		}
	}

	// A positive limit still caps the result for callers that want a page.
	rows, err := s.List("get-expenses", 50)
	if err != nil {
		t.Fatalf("List(limit=50): %v", err)
	}
	if len(rows) != 50 {
		t.Fatalf("List(limit=50) returned %d rows, want 50 (positive limit must still cap)", len(rows))
	}
}

func TestList_ExcludesEmptyRecordsAndFillsPageWithValidRows(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	resourceType := "get-expenses"
	entries := []string{
		`{"id":"v1","description":"one"}`,
		`[]`,
		`{"id":"v2","description":"two"}`,
		`null`,
		`{"id":"v3","description":"three"}`,
		`{}`,
		`{"id":"v4","description":"four"}`,
		`    `,
		`{"id":"v5","description":"five"}`,
		`[]`,
		`{"id":"v6","description":"six"}`,
		`{"id":"v7","description":"seven"}`,
	}
	for i, raw := range entries {
		if err := s.Upsert(resourceType, fmt.Sprintf("row-%d", i), json.RawMessage(raw)); err != nil {
			t.Fatalf("Upsert(%d): %v", i, err)
		}
	}

	page, err := s.List(resourceType, 5)
	if err != nil {
		t.Fatalf("List(limit=5): %v", err)
	}
	if len(page) != 5 {
		t.Fatalf("List(limit=5) returned %d rows, want 5 valid rows", len(page))
	}
	for i, row := range page {
		trimmed := strings.TrimSpace(string(row))
		if trimmed == "" || trimmed == "null" || trimmed == "[]" || trimmed == "{}" {
			t.Fatalf("List(limit=5) row %d is empty artifact %q", i, trimmed)
		}
	}

	all, err := s.List(resourceType, 0)
	if err != nil {
		t.Fatalf("List(limit=0): %v", err)
	}
	if len(all) != 7 {
		t.Fatalf("List(limit=0) returned %d rows, want 7 valid rows", len(all))
	}
	for i, row := range all {
		trimmed := strings.TrimSpace(string(row))
		if trimmed == "" || trimmed == "null" || trimmed == "[]" || trimmed == "{}" {
			t.Fatalf("List(limit=0) row %d is empty artifact %q", i, trimmed)
		}
	}
}
