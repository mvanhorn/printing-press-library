package adsanalytics

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestLoadSellerRevenueFromOrdersTable(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "store.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE orders (id TEXT PRIMARY KEY, data JSON NOT NULL)`); err != nil {
		t.Fatalf("create orders: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO orders (id, data) VALUES (?, ?), (?, ?)`,
		"one", `{"AmazonOrderId":"1","OrderTotal":{"CurrencyCode":"USD","Amount":"120.50"},"OrderItems":[{"ASIN":"B0A"}]}`,
		"two", `{"AmazonOrderId":"2","OrderTotal":{"CurrencyCode":"USD","Amount":"80.00"},"OrderItems":[{"ASIN":"B0B"}]}`,
	); err != nil {
		t.Fatalf("insert orders: %v", err)
	}

	all, err := LoadSellerRevenue(path, "")
	if err != nil {
		t.Fatalf("LoadSellerRevenue all returned error: %v", err)
	}
	if all.Revenue != 200.50 || all.MatchedRecords != 2 || all.Source != "orders" {
		t.Fatalf("all revenue = %+v", all)
	}

	filtered, err := LoadSellerRevenue(path, "B0A")
	if err != nil {
		t.Fatalf("LoadSellerRevenue filtered returned error: %v", err)
	}
	if filtered.Revenue != 120.50 || filtered.MatchedRecords != 1 {
		t.Fatalf("filtered revenue = %+v", filtered)
	}
}

func TestLoadSellerRevenueMissingStoreDegradesGracefully(t *testing.T) {
	t.Parallel()
	got, err := LoadSellerRevenue(filepath.Join(t.TempDir(), "missing.db"), "")
	if err != nil {
		t.Fatalf("LoadSellerRevenue missing returned error: %v", err)
	}
	if got.Revenue != 0 || len(got.Notes) == 0 {
		t.Fatalf("missing store summary = %+v", got)
	}
}

func TestLoadSellerRevenueSchemaMismatchSurfacesNote(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "store.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE orders (id TEXT PRIMARY KEY, payload JSON NOT NULL)`); err != nil {
		t.Fatalf("create incompatible orders: %v", err)
	}

	got, err := LoadSellerRevenue(path, "")
	if err != nil {
		t.Fatalf("LoadSellerRevenue schema mismatch returned error: %v", err)
	}
	if !notesContain(got.Notes, "orders does not include a data column") {
		t.Fatalf("schema mismatch notes = %#v", got.Notes)
	}
}

func TestLoadSellerRevenueMalformedJSONSurfacesNote(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "store.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE orders (id TEXT PRIMARY KEY, data JSON NOT NULL)`); err != nil {
		t.Fatalf("create orders: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO orders (id, data) VALUES (?, ?), (?, ?)`,
		"bad", `{not-json`,
		"good", `{"AmazonOrderId":"1","OrderTotal":{"Amount":"42.00"}}`,
	); err != nil {
		t.Fatalf("insert orders: %v", err)
	}

	got, err := LoadSellerRevenue(path, "")
	if err != nil {
		t.Fatalf("LoadSellerRevenue malformed row returned error: %v", err)
	}
	if got.Revenue != 42 || got.MatchedRecords != 1 {
		t.Fatalf("malformed row revenue = %+v", got)
	}
	if !notesContain(got.Notes, "skipped 1 malformed JSON row") {
		t.Fatalf("malformed row notes = %#v", got.Notes)
	}
}

func TestValidateSellerStorePrerequisites(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "store.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE metadata (key TEXT PRIMARY KEY, value TEXT);
CREATE TABLE orders (id TEXT PRIMARY KEY, updated_at TEXT, data JSON NOT NULL);
INSERT INTO metadata (key, value) VALUES ('profile_id', 'profile-1'), ('marketplace', 'ATVPDKIKX0DER');
INSERT INTO orders (id, updated_at, data) VALUES ('one', '2026-06-03T12:00:00Z', '{"PurchaseDate":"2026-06-02","OrderTotal":{"Amount":"42.00"}}');`); err != nil {
		t.Fatalf("seed store: %v", err)
	}
	got, err := ValidateSellerStore(path, "ATVPDKIKX0DER", "profile-1", "2026-06-01", "2026-06-04")
	if err != nil {
		t.Fatalf("ValidateSellerStore returned error: %v", err)
	}
	if !got.Exists || got.Freshness == "" || got.AccountMatch == nil || !*got.AccountMatch || got.DateOverlap == nil || !*got.DateOverlap {
		t.Fatalf("validation = %+v", got)
	}
	if _, err := ValidateSellerStore(path, "ATVPDKIKX0DER", "other-profile", "2026-06-01", "2026-06-04"); err == nil {
		t.Fatalf("expected account mismatch error")
	}
}

func TestSellerStoreDateOverlapUsesNestedJSONDates(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "store.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE resources (id TEXT PRIMARY KEY, data JSON NOT NULL);
INSERT INTO resources (id, data) VALUES
('one', '{"payload":{"order":{"PurchaseDate":"2026-06-02"}}}'),
('two', '{"payload":{"order":{"lastUpdateDate":"2026-06-06T10:00:00Z"}}}'),
('bad', '{not-json');`); err != nil {
		t.Fatalf("seed resources: %v", err)
	}
	overlap, known := sellerStoreDateOverlap(db, "2026-06-03", "2026-06-04")
	if !known || !overlap {
		t.Fatalf("overlap, known = %v, %v; want true, true", overlap, known)
	}
	overlap, known = sellerStoreDateOverlap(db, "2026-05-01", "2026-05-02")
	if !known || overlap {
		t.Fatalf("overlap, known = %v, %v; want false, true", overlap, known)
	}
}

func notesContain(notes []string, needle string) bool {
	for _, note := range notes {
		if strings.Contains(note, needle) {
			return true
		}
	}
	return false
}
