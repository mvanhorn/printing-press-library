package cli

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func setupInsidersDB(t *testing.T) (string, *sql.DB) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := sql.Open("sqlite", dbPath+"?_foreign_keys=ON")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := db.Exec(transcendenceSchema); err != nil {
		t.Fatalf("schema: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS resources (
		id TEXT NOT NULL, resource_type TEXT NOT NULL,
		data TEXT, synced_at DATETIME, updated_at DATETIME,
		PRIMARY KEY (resource_type, id)
	)`); err != nil {
		t.Fatalf("resources: %v", err)
	}
	return dbPath, db
}

func insertInsider(t *testing.T, db *sql.DB, symbol, txnType string, shares int64, daysAgo int, id string) {
	t.Helper()
	date := time.Now().Add(-time.Duration(daysAgo) * 24 * time.Hour).Format("2006-01-02")
	raw, _ := json.Marshal(map[string]any{
		"date":            date,
		"transactionType": txnType,
		"shares":          shares,
	})
	if _, err := db.Exec(`INSERT INTO resources(id, resource_type, data) VALUES(?, 'insider_transactions', ?)`,
		fmt.Sprintf("%s:%s", symbol, id), string(raw)); err != nil {
		t.Fatalf("insert: %v", err)
	}
}

func TestInsidersNetBuyingPositive(t *testing.T) {
	dbPath, db := setupInsidersDB(t)
	insertInsider(t, db, "AAPL", "Buy", 1000, 5, "1")
	insertInsider(t, db, "AAPL", "Buy", 1000, 10, "2")
	insertInsider(t, db, "AAPL", "Sell", 500, 7, "3")
	db.Close()

	flags := &rootFlags{asJSON: true}
	cmd := newInsidersNetBuyingLeafCmd(flags)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--db", dbPath, "--recent", "30d", "--all"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	var got []insiderNetRow
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v (raw=%s)", err, buf.String())
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 row, got %d: %+v", len(got), got)
	}
	if got[0].Symbol != "AAPL" || got[0].NetShares != 1500 {
		t.Errorf("expected AAPL/1500 net shares, got %+v", got[0])
	}
}

func TestInsidersNetBuyingAllSells(t *testing.T) {
	dbPath, db := setupInsidersDB(t)
	insertInsider(t, db, "AAPL", "Sell", 1000, 5, "1")
	insertInsider(t, db, "AAPL", "Sell", 500, 10, "2")
	db.Close()

	flags := &rootFlags{asJSON: true}
	cmd := newInsidersNetBuyingLeafCmd(flags)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--db", dbPath, "--recent", "30d", "--all"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	var got []insiderNetRow
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, r := range got {
		if r.Symbol == "AAPL" {
			t.Errorf("AAPL should NOT appear (only sells); got %+v", r)
		}
	}
}

func TestInsidersNetBuyingOutsideWindow(t *testing.T) {
	dbPath, db := setupInsidersDB(t)
	// Activity 90 days ago, but window is 30 days.
	insertInsider(t, db, "MSFT", "Buy", 5000, 90, "old")
	db.Close()

	flags := &rootFlags{asJSON: true}
	cmd := newInsidersNetBuyingLeafCmd(flags)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--db", dbPath, "--recent", "30d", "--all"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	var got []insiderNetRow
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected nothing in window, got %+v", got)
	}
}
