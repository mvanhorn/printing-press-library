package cli

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func setupDividendsDB(t *testing.T) (string, *sql.DB) {
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

func TestPortfolioDividendsIncomeMath(t *testing.T) {
	dbPath, db := setupDividendsDB(t)
	// 100 AAPL @ $175 cost basis ($17500 total)
	if _, err := db.Exec(`INSERT INTO portfolio_lots(symbol, shares, cost_basis, purchased_on)
		VALUES('AAPL', 100, 175, '2024-01-01')`); err != nil {
		t.Fatalf("insert lot: %v", err)
	}
	// Two dividend rows, $1/share each in 2026.
	for i, dt := range []string{"2026-02-09", "2026-05-09"} {
		payload := map[string]any{"date": dt, "amount": 1.0}
		raw, _ := json.Marshal(payload)
		id := "AAPL:" + dt
		_ = i
		if _, err := db.Exec(`INSERT INTO resources(id, resource_type, data) VALUES(?, 'dividends', ?)`, id, string(raw)); err != nil {
			t.Fatalf("insert div: %v", err)
		}
	}
	db.Close()

	flags := &rootFlags{asJSON: true}
	cmd := newPortfolioDividendsCmd(flags)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--db", dbPath, "--year", "2026"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	var got []dividendRow
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v (raw=%s)", err, buf.String())
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 row, got %d (%+v)", len(got), got)
	}
	r := got[0]
	if r.Symbol != "AAPL" {
		t.Errorf("symbol: %v", r.Symbol)
	}
	if r.TotalDividendInc != 200 {
		t.Errorf("TotalDividendInc = %v, want 200", r.TotalDividendInc)
	}
	wantYoC := 200.0 / 17500.0
	if abs(r.YieldOnCostPct-wantYoC) > 1e-6 {
		t.Errorf("YoC = %v, want %v", r.YieldOnCostPct, wantYoC)
	}
}

func TestPortfolioDividendsNoData(t *testing.T) {
	dbPath, db := setupDividendsDB(t)
	if _, err := db.Exec(`INSERT INTO portfolio_lots(symbol, shares, cost_basis, purchased_on)
		VALUES('TSLA', 10, 200, '2024-01-01')`); err != nil {
		t.Fatalf("insert lot: %v", err)
	}
	db.Close()

	flags := &rootFlags{asJSON: true}
	cmd := newPortfolioDividendsCmd(flags)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--db", dbPath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	var got []dividendRow
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 row, got %d", len(got))
	}
	if got[0].Note == "" {
		t.Errorf("expected a 'note' for unsynced symbol, got %+v", got[0])
	}
	if got[0].TotalDividendInc != 0 {
		t.Errorf("expected 0 income for unsynced symbol, got %v", got[0].TotalDividendInc)
	}
}

func TestPortfolioDividendsSymbolFilter(t *testing.T) {
	dbPath, db := setupDividendsDB(t)
	for _, sym := range []string{"AAPL", "MSFT"} {
		if _, err := db.Exec(`INSERT INTO portfolio_lots(symbol, shares, cost_basis, purchased_on)
			VALUES(?, 10, 100, '2024-01-01')`, sym); err != nil {
			t.Fatalf("insert lot: %v", err)
		}
	}
	db.Close()
	flags := &rootFlags{asJSON: true}
	cmd := newPortfolioDividendsCmd(flags)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--db", dbPath, "--symbol", "AAPL"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	var got []dividendRow
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got) != 1 || got[0].Symbol != "AAPL" {
		t.Errorf("expected 1 AAPL row, got %+v", got)
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
