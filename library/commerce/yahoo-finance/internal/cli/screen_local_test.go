package cli

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func setupScreenDB(t *testing.T, rows []map[string]any) string {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := sql.Open("sqlite", dbPath+"?_foreign_keys=ON")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS resources (
		id TEXT NOT NULL, resource_type TEXT NOT NULL,
		data TEXT, synced_at DATETIME, updated_at DATETIME,
		PRIMARY KEY (resource_type, id)
	)`); err != nil {
		t.Fatalf("schema: %v", err)
	}
	for _, r := range rows {
		raw, _ := json.Marshal(r)
		sym := r["symbol"].(string)
		if _, err := db.Exec(`INSERT INTO resources(id, resource_type, data) VALUES(?, 'stats', ?)`, sym, string(raw)); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}
	db.Close()
	return dbPath
}

func TestScreenLocalPEMax(t *testing.T) {
	dbPath := setupScreenDB(t, []map[string]any{
		{"symbol": "A", "trailingPE": 10.0, "marketCap": 1e9},
		{"symbol": "B", "trailingPE": 20.0, "marketCap": 2e9},
		{"symbol": "C", "trailingPE": 30.0, "marketCap": 3e9},
	})
	flags := &rootFlags{asJSON: true}
	cmd := newScreenLocalCmd(flags)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--db", dbPath, "--pe-max", "15"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	var got []screenRow
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got) != 1 || got[0].Symbol != "A" {
		t.Errorf("expected only A, got %+v", got)
	}
}

func TestScreenLocalPEMaxNoMatch(t *testing.T) {
	dbPath := setupScreenDB(t, []map[string]any{
		{"symbol": "A", "trailingPE": 10.0},
	})
	flags := &rootFlags{asJSON: true}
	cmd := newScreenLocalCmd(flags)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--db", dbPath, "--pe-max", "5"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	var got []screenRow
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty result, got %+v", got)
	}
}

func TestScreenLocalROEAndDebtEquity(t *testing.T) {
	dbPath := setupScreenDB(t, []map[string]any{
		{"symbol": "A", "returnOnEquity": 0.20, "debtToEquity": 0.5},
		{"symbol": "B", "returnOnEquity": 0.10, "debtToEquity": 2.0},
		{"symbol": "C", "returnOnEquity": 0.30, "debtToEquity": 0.3},
	})
	flags := &rootFlags{asJSON: true}
	cmd := newScreenLocalCmd(flags)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--db", dbPath, "--roe-min", "0.15", "--debt-equity-max", "1"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	var got []screenRow
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 rows, got %d (%+v)", len(got), got)
	}
}
