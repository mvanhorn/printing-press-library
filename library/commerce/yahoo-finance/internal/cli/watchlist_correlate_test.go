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

func setupCorrelateDB(t *testing.T) (string, *sql.DB) {
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

// insertHistory inserts a series of closes for a symbol, dated daysAgo[i]
// days ago. Each row goes in as a single history object.
func insertHistory(t *testing.T, db *sql.DB, symbol string, closes []float64) {
	t.Helper()
	for i, c := range closes {
		date := time.Now().Add(-time.Duration(len(closes)-i) * 24 * time.Hour).Format("2006-01-02")
		raw, _ := json.Marshal(map[string]any{"date": date, "close": c})
		id := fmt.Sprintf("%s:%s", symbol, date)
		if _, err := db.Exec(`INSERT INTO resources(id, resource_type, data) VALUES(?, 'history', ?)`,
			id, string(raw)); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}
}

func TestWatchlistCorrelatePerfectPositive(t *testing.T) {
	dbPath, db := setupCorrelateDB(t)
	if _, err := db.Exec(`INSERT INTO watchlists(name) VALUES('twin')`); err != nil {
		t.Fatalf("wl: %v", err)
	}
	for _, sym := range []string{"AAA", "BBB"} {
		if _, err := db.Exec(`INSERT INTO watchlist_members(watchlist, symbol) VALUES('twin', ?)`, sym); err != nil {
			t.Fatalf("wlm: %v", err)
		}
	}
	closes := []float64{100, 110, 121, 133.1, 146.41}
	insertHistory(t, db, "AAA", closes)
	insertHistory(t, db, "BBB", closes) // identical → perfect correlation
	db.Close()

	flags := &rootFlags{asJSON: true}
	cmd := newWatchlistCorrelateCmd(flags)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"twin", "--db", dbPath, "--range", "180d"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	var got correlationPayload
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v (raw=%s)", err, buf.String())
	}
	if len(got.Triples) != 1 {
		t.Fatalf("expected 1 pair, got %d", len(got.Triples))
	}
	if abs(got.Triples[0].Correlation-1.0) > 1e-4 {
		t.Errorf("correlation = %v, want 1.0", got.Triples[0].Correlation)
	}
}

func TestWatchlistCorrelateAnticorrelated(t *testing.T) {
	dbPath, db := setupCorrelateDB(t)
	if _, err := db.Exec(`INSERT INTO watchlists(name) VALUES('opp')`); err != nil {
		t.Fatalf("wl: %v", err)
	}
	for _, sym := range []string{"UP", "DN"} {
		if _, err := db.Exec(`INSERT INTO watchlist_members(watchlist, symbol) VALUES('opp', ?)`, sym); err != nil {
			t.Fatalf("wlm: %v", err)
		}
	}
	// Construct UP and DN whose daily returns are exact sign-mirrors:
	//   UP returns: +0.10, -0.10, +0.05  (prices: 100, 110, 99, 103.95)
	//   DN returns: -0.10, +0.10, -0.05  (prices: 100, 90, 99, 94.05)
	// Pearson on (r, -r) is exactly -1.
	insertHistory(t, db, "UP", []float64{100, 110, 99, 103.95})
	insertHistory(t, db, "DN", []float64{100, 90, 99, 94.05})
	db.Close()

	flags := &rootFlags{asJSON: true}
	cmd := newWatchlistCorrelateCmd(flags)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"opp", "--db", dbPath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	var got correlationPayload
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got.Triples) != 1 {
		t.Fatalf("expected 1 pair, got %d", len(got.Triples))
	}
	if abs(got.Triples[0].Correlation-(-1.0)) > 1e-4 {
		t.Errorf("correlation = %v, want -1.0", got.Triples[0].Correlation)
	}
}

func TestWatchlistCorrelateEmptyWatchlist(t *testing.T) {
	dbPath, db := setupCorrelateDB(t)
	if _, err := db.Exec(`INSERT INTO watchlists(name) VALUES('solo')`); err != nil {
		t.Fatalf("wl: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO watchlist_members(watchlist, symbol) VALUES('solo', 'AAA')`); err != nil {
		t.Fatalf("wlm: %v", err)
	}
	db.Close()
	flags := &rootFlags{asJSON: true}
	cmd := newWatchlistCorrelateCmd(flags)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"solo", "--db", dbPath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	var got correlationPayload
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got.Triples) != 0 {
		t.Errorf("expected 0 triples for solo watchlist, got %+v", got.Triples)
	}
	if got.Note == "" {
		t.Errorf("expected a note for too-small watchlist")
	}
}
