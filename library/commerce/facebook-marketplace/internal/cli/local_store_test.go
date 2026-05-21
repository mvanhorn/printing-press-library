package cli

import (
	"database/sql"
	"testing"
	"time"
)

func TestDeterministicMatchRejectsZeroPriceBelowMinimum(t *testing.T) {
	t.Parallel()

	ok, reason := deterministicMatch(
		watchRow{MinPriceCents: 5000},
		listingRow{Title: "Free chair", PriceCents: 0},
	)
	if ok {
		t.Fatalf("deterministicMatch accepted zero-price listing below minimum")
	}
	if reason != "below minimum price" {
		t.Fatalf("reason = %q, want below minimum price", reason)
	}
}

func TestDeterministicMatchAllowsZeroPriceUnderMaximum(t *testing.T) {
	t.Parallel()

	ok, reason := deterministicMatch(
		watchRow{MaxPriceCents: 5000},
		listingRow{Title: "Free chair", PriceCents: 0},
	)
	if !ok {
		t.Fatalf("deterministicMatch rejected zero-price listing under maximum: %s", reason)
	}
}

func TestMarkMatchesSeenClearsNewFlag(t *testing.T) {
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
	if _, err := db.Exec(`INSERT INTO matches (id, watch_id, listing_id, deterministic_ok, llm_relevant, is_new, created_at) VALUES (1, 1, 'listing-1', 1, 0, 1, ?)`, now); err != nil {
		t.Fatalf("insert match: %v", err)
	}

	if err := markMatchesSeen(db, []matchRow{{ID: 1}}); err != nil {
		t.Fatalf("markMatchesSeen: %v", err)
	}
	var isNew int
	if err := db.QueryRow(`SELECT is_new FROM matches WHERE id = 1`).Scan(&isNew); err != nil {
		t.Fatalf("select is_new: %v", err)
	}
	if isNew != 0 {
		t.Fatalf("is_new = %d, want 0", isNew)
	}
}
