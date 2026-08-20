package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/facebook-marketplace/internal/store"
	_ "modernc.org/sqlite"
)

type watchRow struct {
	ID               int64  `json:"id"`
	Name             string `json:"name"`
	Query            string `json:"query"`
	MustHaveKeywords string `json:"must_have_keywords,omitempty"`
	RejectKeywords   string `json:"reject_keywords,omitempty"`
	MinPriceCents    int64  `json:"min_price_cents,omitempty"`
	MaxPriceCents    int64  `json:"max_price_cents,omitempty"`
	RadiusMiles      int    `json:"radius_miles,omitempty"`
	Enabled          bool   `json:"enabled"`
	CreatedAt        string `json:"created_at"`
	UpdatedAt        string `json:"updated_at"`
}

type listingRow struct {
	ID              string `json:"id"`
	Title           string `json:"title"`
	PriceCents      int64  `json:"price_cents,omitempty"`
	DistanceMiles   int    `json:"distance_miles,omitempty"`
	URL             string `json:"url,omitempty"`
	SellerName      string `json:"seller_name,omitempty"`
	PublicLocation  string `json:"public_location,omitempty"`
	ListedAt        string `json:"listed_at,omitempty"`
	UpdatedAt       string `json:"updated_at,omitempty"`
	EngagementCount int    `json:"engagement_count,omitempty"`
	RawJSON         string `json:"-"`
}

type matchRow struct {
	ID              int64  `json:"id"`
	WatchID         int64  `json:"watch_id"`
	ListingID       string `json:"listing_id"`
	Title           string `json:"title"`
	PriceCents      int64  `json:"price_cents,omitempty"`
	DeterministicOK bool   `json:"deterministic_ok"`
	LLMRelevant     bool   `json:"llm_relevant"`
	Reason          string `json:"reason,omitempty"`
	IsNew           bool   `json:"is_new"`
	CreatedAt       string `json:"created_at"`
}

func openLocalDB() (*sql.DB, error) {
	dir, err := ensureAppDataDir()
	if err != nil {
		return nil, err
	}
	db, err := store.Open(filepath.Join(dir, "local.sqlite"))
	if err != nil {
		return nil, err
	}
	if err := initLocalDB(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func initLocalDB(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS watches (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			query TEXT NOT NULL,
			must_have_keywords TEXT NOT NULL DEFAULT '',
			reject_keywords TEXT NOT NULL DEFAULT '',
			min_price_cents INTEGER NOT NULL DEFAULT 0,
			max_price_cents INTEGER NOT NULL DEFAULT 0,
			radius_miles INTEGER NOT NULL DEFAULT 0,
			enabled INTEGER NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS listings (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			price_cents INTEGER NOT NULL DEFAULT 0,
			distance_miles INTEGER NOT NULL DEFAULT 0,
			url TEXT NOT NULL DEFAULT '',
			seller_name TEXT NOT NULL DEFAULT '',
			public_location TEXT NOT NULL DEFAULT '',
			listed_at TEXT NOT NULL DEFAULT '',
			updated_at TEXT NOT NULL DEFAULT '',
			engagement_count INTEGER NOT NULL DEFAULT 0,
			raw_json TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS matches (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			watch_id INTEGER NOT NULL,
			listing_id TEXT NOT NULL,
			deterministic_ok INTEGER NOT NULL,
			llm_relevant INTEGER NOT NULL,
			reason TEXT NOT NULL DEFAULT '',
			is_new INTEGER NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL,
			UNIQUE(watch_id, listing_id)
		)`,
		`CREATE TABLE IF NOT EXISTS write_actions (
			idempotency_key TEXT PRIMARY KEY,
			state TEXT NOT NULL,
			detail TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS eval_items (
			id TEXT PRIMARY KEY,
			payload_json TEXT NOT NULL,
			created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS eval_pairs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			watch_id INTEGER NOT NULL,
			listing_id TEXT NOT NULL,
			label TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			UNIQUE(watch_id, listing_id)
		)`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

func recordWriteState(key, state, detail string) error {
	db, err := openLocalDB()
	if err != nil {
		return err
	}
	defer db.Close()
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = db.Exec(`INSERT INTO write_actions (idempotency_key, state, detail, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(idempotency_key) DO UPDATE SET state = excluded.state, detail = excluded.detail, updated_at = excluded.updated_at`,
		key, state, detail, now, now)
	return err
}

func centsFromDollars(v float64) int64 {
	return int64(v*100 + 0.5)
}

func splitCSV(v string) []string {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.ToLower(strings.TrimSpace(part))
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func deterministicMatch(w watchRow, l listingRow) (bool, string) {
	title := strings.ToLower(l.Title)
	for _, term := range splitCSV(w.MustHaveKeywords) {
		if !strings.Contains(title, term) {
			return false, "missing required keyword: " + term
		}
	}
	for _, term := range splitCSV(w.RejectKeywords) {
		if strings.Contains(title, term) {
			return false, "rejected keyword: " + term
		}
	}
	if w.MinPriceCents > 0 && l.PriceCents < w.MinPriceCents {
		return false, "below minimum price"
	}
	if w.MaxPriceCents > 0 && l.PriceCents > 0 && l.PriceCents > w.MaxPriceCents {
		return false, "above maximum price"
	}
	if w.RadiusMiles > 0 && l.DistanceMiles > 0 && l.DistanceMiles > w.RadiusMiles {
		return false, "outside radius"
	}
	return true, "deterministic filters passed"
}

func loadListingsFile(path string) ([]listingRow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var rows []listingRow
	if err := json.Unmarshal(data, &rows); err == nil {
		return rows, nil
	}
	var wrapped struct {
		Listings []listingRow `json:"listings"`
		Results  []listingRow `json:"results"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return nil, fmt.Errorf("parsing listings JSON: %w", err)
	}
	if len(wrapped.Listings) > 0 {
		return wrapped.Listings, nil
	}
	return wrapped.Results, nil
}

func upsertListing(db *sql.DB, l listingRow) error {
	if l.ID == "" {
		return fmt.Errorf("listing id is required")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if l.UpdatedAt == "" {
		l.UpdatedAt = now
	}
	raw, _ := json.Marshal(l)
	_, err := db.Exec(`INSERT INTO listings (id, title, price_cents, distance_miles, url, seller_name, public_location, listed_at, updated_at, engagement_count, raw_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET title = excluded.title, price_cents = excluded.price_cents, distance_miles = excluded.distance_miles,
			url = excluded.url, seller_name = excluded.seller_name, public_location = excluded.public_location, listed_at = excluded.listed_at,
			updated_at = excluded.updated_at, engagement_count = excluded.engagement_count, raw_json = excluded.raw_json`,
		l.ID, l.Title, l.PriceCents, l.DistanceMiles, l.URL, l.SellerName, l.PublicLocation, l.ListedAt, l.UpdatedAt, l.EngagementCount, string(raw))
	return err
}
