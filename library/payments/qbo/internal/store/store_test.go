// Copyright 2026 Martin Kessler and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"path/filepath"
	"testing"
	"time"
)

func TestStore(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	s, err := OpenWithPath(dbPath)
	if err != nil {
		t.Fatalf("failed to open store with path %s: %v", dbPath, err)
	}
	defer s.Close()

	// Verify tables are created
	tables := []string{"customers", "vendors", "accounts", "invoices", "payments", "bills", "purchases", "journal_entries", "sync_state"}
	for _, table := range tables {
		var name string
		err := s.db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
		if err != nil {
			t.Errorf("table %s was not created: %v", table, err)
		}
	}

	// Test UpsertEntity
	err = s.UpsertEntity("customers", "123", "Alice Smith", "", "2026-06-04T12:00:00Z", `{"Id":"123","DisplayName":"Alice Smith"}`)
	if err != nil {
		t.Errorf("failed to upsert customer: %v", err)
	}

	// Verify record exists
	var name, rawJSON string
	err = s.db.QueryRow("SELECT name, raw_json FROM customers WHERE id=?", "123").Scan(&name, &rawJSON)
	if err != nil {
		t.Fatalf("failed to find upserted customer: %v", err)
	}
	if name != "Alice Smith" {
		t.Errorf("expected name 'Alice Smith', got '%s'", name)
	}

	// Test Sync State Time
	now := time.Now().Truncate(time.Second)
	err = s.SetLastSyncTime(now)
	if err != nil {
		t.Errorf("failed to set last sync time: %v", err)
	}

	retrieved, err := s.GetLastSyncTime()
	if err != nil {
		t.Errorf("failed to get last sync time: %v", err)
	}
	if !retrieved.Equal(now) {
		t.Errorf("expected sync time %v, got %v", now, retrieved)
	}

	// Test duplicate query logic on Purchases
	// Insert two purchases with same vendor, same amount, within 2 days (less than 3 days window)
	err = s.UpsertEntity("purchases", "p1", "", "", "2026-06-01T10:00:00Z", `{"Id":"p1","EntityRef":{"value":"vend1","name":"Vendor One"},"TotalAmt":"100.50","TxnDate":"2026-06-01"}`)
	if err != nil {
		t.Fatalf("failed to insert purchase p1: %v", err)
	}
	err = s.UpsertEntity("purchases", "p2", "", "", "2026-06-02T10:00:00Z", `{"Id":"p2","EntityRef":{"value":"vend1","name":"Vendor One"},"TotalAmt":"100.50","TxnDate":"2026-06-02"}`)
	if err != nil {
		t.Fatalf("failed to insert purchase p2: %v", err)
	}

	// Find duplicate purchases (using exact SQL query from duplicates subcommand)
	duplicateQuery := `
		SELECT 
			p1.id, p2.id
		FROM purchases p1
		JOIN purchases p2 ON p1.id < p2.id
		  AND json_extract(p1.raw_json, '$.EntityRef.value') = json_extract(p2.raw_json, '$.EntityRef.value')
		  AND abs(CAST(json_extract(p1.raw_json, '$.TotalAmt') AS REAL) - CAST(json_extract(p2.raw_json, '$.TotalAmt') AS REAL)) < 0.01
		  AND abs(strftime('%s', json_extract(p1.raw_json, '$.TxnDate')) - strftime('%s', json_extract(p2.raw_json, '$.TxnDate'))) <= ?
	`
	var id1, id2 string
	err = s.db.QueryRow(duplicateQuery, 3*24*3600).Scan(&id1, &id2)
	if err != nil {
		t.Errorf("failed to find duplicate purchases via SQL: %v", err)
	}
	if (id1 != "p1" || id2 != "p2") && (id1 != "p2" || id2 != "p1") {
		t.Errorf("expected duplicate ids p1 and p2, got %s and %s", id1, id2)
	}
}

func TestNormalizeToUTC(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"2026-06-03T23:45:00-08:00", "2026-06-04T07:45:00Z"},
		{"2026-06-04T07:45:00Z", "2026-06-04T07:45:00Z"},
		{"2026-06-03T23:45:00.123-08:00", "2026-06-04T07:45:00Z"}, // normalizes fractional part out or preserves UTC RFC3339
		{"invalid-date", "invalid-date"},
	}

	for _, tc := range tests {
		got := normalizeToUTC(tc.input)
		if got != tc.expected {
			t.Errorf("normalizeToUTC(%q) = %q; expected %q", tc.input, got, tc.expected)
		}
	}
}

func TestSchemaMigration(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "migration_test.db")

	s, err := OpenWithPath(dbPath)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}

	// Insert record directly with timezone-offset via raw SQL
	_, err = s.db.Exec(`
		INSERT INTO customers (id, name, doc_number, last_updated, raw_json)
		VALUES (?, ?, ?, ?, ?)
	`, "999", "Old Offset Record", "D999", "2026-06-03T23:45:00-08:00", `{"Id":"999"}`)
	if err != nil {
		s.Close()
		t.Fatalf("failed to insert raw test record: %v", err)
	}

	s.Close()

	// Reopen store (which runs initSchema and applies migrations)
	s2, err := OpenWithPath(dbPath)
	if err != nil {
		t.Fatalf("failed to reopen store: %v", err)
	}
	defer s2.Close()

	var lastUpdated string
	err = s2.db.QueryRow("SELECT last_updated FROM customers WHERE id = ?", "999").Scan(&lastUpdated)
	if err != nil {
		t.Fatalf("failed to query migrated record: %v", err)
	}

	expectedUTC := "2026-06-04T07:45:00Z"
	if lastUpdated != expectedUTC {
		t.Errorf("expected migrated last_updated to be %q, got %q", expectedUTC, lastUpdated)
	}
}
