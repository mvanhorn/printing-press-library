package cli

import (
	"database/sql"
	"testing"
	"time"
)

func TestInsertWatchMatchReportsOnlyNewRows(t *testing.T) {
	t.Parallel()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()
	if err := initLocalDB(db); err != nil {
		t.Fatalf("initLocalDB: %v", err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := db.Exec(`INSERT INTO watches (id, name, query, created_at, updated_at) VALUES (1, 'chairs', 'chair', ?, ?)`, now, now); err != nil {
		t.Fatalf("insert watch: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO listings (id, title) VALUES ('listing-1', 'Chair')`); err != nil {
		t.Fatalf("insert listing: %v", err)
	}

	inserted, err := insertWatchMatch(db, 1, "listing-1", "query match", now)
	if err != nil {
		t.Fatalf("insertWatchMatch first call: %v", err)
	}
	if !inserted {
		t.Fatalf("first insert reported duplicate")
	}

	inserted, err = insertWatchMatch(db, 1, "listing-1", "query match", now)
	if err != nil {
		t.Fatalf("insertWatchMatch duplicate call: %v", err)
	}
	if inserted {
		t.Fatalf("duplicate insert reported new row")
	}
}
