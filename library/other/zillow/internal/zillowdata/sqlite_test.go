package zillowdata

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSaveTableAndReadOnlyQuery(t *testing.T) {
	table, err := Parse(Dataset{Key: "test", Unit: "usd"}, strings.NewReader(sampleCSV))
	if err != nil {
		t.Fatal(err)
	}
	table.FetchedAt = time.Now().UTC()
	table.SourceURL = "https://example.test/data.csv"
	table.SHA256 = "abc"
	db, err := OpenDatabase(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := SaveTable(context.Background(), db, table); err != nil {
		t.Fatal(err)
	}
	rows, err := QueryReadOnly(context.Background(), db, `SELECT COUNT(*) AS count FROM zillow_observations`, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0]["count"].(int64) != 6 {
		t.Fatalf("rows = %#v", rows)
	}
	if _, err := QueryReadOnly(context.Background(), db, `DELETE FROM zillow_observations`, 10); err == nil {
		t.Fatal("DELETE unexpectedly allowed")
	}
	if _, err := QueryReadOnly(context.Background(), db, `WITH doomed AS (SELECT 1) DELETE FROM zillow_observations`, 10); err == nil {
		t.Fatal("CTE DELETE unexpectedly allowed")
	}
}

func TestSaveTableRejectsEmptyReplacement(t *testing.T) {
	table, err := Parse(Dataset{Key: "test", Unit: "usd"}, strings.NewReader(sampleCSV))
	if err != nil {
		t.Fatal(err)
	}
	table.FetchedAt = time.Now().UTC()
	table.SourceURL = "https://example.test/data.csv"
	table.SHA256 = "abc"

	db, err := OpenDatabase(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := SaveTable(context.Background(), db, table); err != nil {
		t.Fatal(err)
	}

	empty := &Table{
		Dataset:   table.Dataset,
		FetchedAt: time.Now().UTC(),
		SourceURL: table.SourceURL,
		SHA256:    "empty",
	}
	if err := SaveTable(context.Background(), db, empty); err == nil {
		t.Fatal("empty replacement unexpectedly accepted")
	}

	rows, err := QueryReadOnly(context.Background(), db, `SELECT COUNT(*) AS count FROM zillow_observations WHERE metric = 'test'`, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0]["count"].(int64) != 6 {
		t.Fatalf("stored observations changed after empty replacement: %#v", rows)
	}
}
