// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package lookups

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

// openTestDB creates a fresh empty SQLite database with the
// entity_lookups table already created. It does NOT load seed data —
// individual tests control which rows are present so each scenario
// stays self-describing.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "lookups.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if _, err := db.Exec(`
		CREATE TABLE entity_lookups (
			kind TEXT NOT NULL,
			canonical TEXT NOT NULL,
			value TEXT NOT NULL,
			source TEXT NOT NULL DEFAULT 'seeded',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (kind, canonical, value)
		)
	`); err != nil {
		t.Fatalf("create entity_lookups: %v", err)
	}
	if _, err := db.Exec(`CREATE INDEX idx_entity_lookup_canonical ON entity_lookups(canonical)`); err != nil {
		t.Fatalf("create idx_entity_lookup_canonical: %v", err)
	}
	if _, err := db.Exec(`CREATE INDEX idx_entity_lookup_kind ON entity_lookups(kind)`); err != nil {
		t.Fatalf("create idx_entity_lookup_kind: %v", err)
	}
	return db
}

func TestIsComputedKind(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"lowercase", "uppercase", "kebab-case", "capitalize-first", "slug"} {
		if !IsComputedKind(kind) {
			t.Errorf("IsComputedKind(%q) = false, want true", kind)
		}
	}
	for _, kind := range []string{"country_iso2", "nfl_team_abbrev", "unknown-kind", ""} {
		if IsComputedKind(kind) {
			t.Errorf("IsComputedKind(%q) = true, want false", kind)
		}
	}
}

// TestLookup_ComputedKinds_BypassDB pins the contract that a
// computed-kind Lookup never touches SQLite. We pass a nil *sql.DB,
// which would otherwise error on any query, and the call must
// still succeed.
func TestLookup_ComputedKinds_BypassDB(t *testing.T) {
	t.Parallel()
	cases := []struct {
		kind, in, want string
	}{
		{"lowercase", "Portugal", "portugal"},
		{"lowercase", "USA", "usa"},
		{"uppercase", "portugal", "PORTUGAL"},
		{"kebab-case", "New Zealand", "new-zealand"},
		{"kebab-case", "Cote d'Ivoire", "cote-d'ivoire"},
		{"capitalize-first", "portugal", "Portugal"},
		{"capitalize-first", "PORTUGAL", "Portugal"},
		{"capitalize-first", "", ""},
		{"slug", "New Zealand", "new-zealand"},
		{"slug", "Cote d'Ivoire", "cote-divoire"},
	}
	for _, tc := range cases {
		got, ok, err := Lookup(nil, tc.kind, tc.in)
		if err != nil {
			t.Errorf("Lookup(nil, %q, %q): unexpected error %v", tc.kind, tc.in, err)
			continue
		}
		if !ok {
			t.Errorf("Lookup(nil, %q, %q): found=false, want true", tc.kind, tc.in)
			continue
		}
		if got != tc.want {
			t.Errorf("Lookup(nil, %q, %q) = %q, want %q", tc.kind, tc.in, got, tc.want)
		}
	}
}

// TestLookup_TableBacked_RoundTrip exercises the table-backed path:
// Upsert a row, Lookup returns the same value, case-insensitive
// canonical matches.
func TestLookup_TableBacked_RoundTrip(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	if err := Upsert(db, "country_iso2", "Portugal", "PT", "seeded"); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	got, ok, err := Lookup(db, "country_iso2", "Portugal")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if !ok || got != "PT" {
		t.Errorf("Lookup(country_iso2, Portugal) = (%q, %v), want (\"PT\", true)", got, ok)
	}
	// Case-insensitive canonical.
	got, ok, err = Lookup(db, "country_iso2", "portugal")
	if err != nil {
		t.Fatalf("Lookup lowercase canonical: %v", err)
	}
	if !ok || got != "PT" {
		t.Errorf("Lookup(country_iso2, portugal) = (%q, %v), want (\"PT\", true)", got, ok)
	}
	// ALL-CAPS canonical.
	got, ok, err = Lookup(db, "country_iso2", "PORTUGAL")
	if err != nil {
		t.Fatalf("Lookup uppercase canonical: %v", err)
	}
	if !ok || got != "PT" {
		t.Errorf("Lookup(country_iso2, PORTUGAL) = (%q, %v), want (\"PT\", true)", got, ok)
	}
}

func TestLookup_NotFound(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	got, ok, err := Lookup(db, "country_iso2", "Atlantis")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if ok {
		t.Errorf("Lookup(country_iso2, Atlantis) found = true, want false (got value %q)", got)
	}
	if got != "" {
		t.Errorf("Lookup(country_iso2, Atlantis) value = %q, want empty", got)
	}
}

// TestUpsert_IdempotentOnSamePK verifies that re-running Upsert with
// the same (kind, canonical, value) triple does not create a
// duplicate row. The table PK is (kind, canonical, value) and the
// INSERT uses OR IGNORE.
func TestUpsert_IdempotentOnSamePK(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	for i := 0; i < 3; i++ {
		if err := Upsert(db, "country_iso2", "Portugal", "PT", "taught"); err != nil {
			t.Fatalf("Upsert iter %d: %v", i, err)
		}
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM entity_lookups WHERE kind = ? AND canonical = ?`,
		"country_iso2", "Portugal").Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Errorf("re-Upserted same triple 3 times, got %d rows, want 1", count)
	}
}

// TestUpsert_DifferentValuesForSameCanonical pins the contract that
// the PK is (kind, canonical, VALUE) — different values for the same
// (kind, canonical) DO create separate rows. This is how "United
// States" can carry both "US" and "USA" aliases under different
// kinds without collision, and how a canonical can have multiple
// values under one kind if a future need arises.
func TestUpsert_DifferentValuesForSameCanonical(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	if err := Upsert(db, "country_alias", "United States", "US", "seeded"); err != nil {
		t.Fatalf("Upsert US: %v", err)
	}
	if err := Upsert(db, "country_alias", "United States", "USA", "seeded"); err != nil {
		t.Fatalf("Upsert USA: %v", err)
	}
	values, err := LookupAll(db, "country_alias", "United States")
	if err != nil {
		t.Fatalf("LookupAll: %v", err)
	}
	if len(values) != 2 {
		t.Errorf("LookupAll returned %d values, want 2: %v", len(values), values)
	}
}

func TestLookupAll_EmptyOnMiss(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	values, err := LookupAll(db, "country_iso2", "Atlantis")
	if err != nil {
		t.Fatalf("LookupAll: %v", err)
	}
	if values == nil {
		t.Errorf("LookupAll returned nil on miss; want non-nil empty slice")
	}
	if len(values) != 0 {
		t.Errorf("LookupAll returned %d values on miss, want 0: %v", len(values), values)
	}
}

func TestLookupAll_ComputedKind(t *testing.T) {
	t.Parallel()
	values, err := LookupAll(nil, "lowercase", "Portugal")
	if err != nil {
		t.Fatalf("LookupAll lowercase: %v", err)
	}
	if len(values) != 1 || values[0] != "portugal" {
		t.Errorf("LookupAll(lowercase, Portugal) = %v, want [\"portugal\"]", values)
	}
}

// TestLookup_SourcePriority verifies that when the same (kind,
// canonical) has both a 'seeded' and a 'taught' row pointing at
// different values, Lookup returns the taught one first. This
// matches the contract that a user override beats a seed.
func TestLookup_SourcePriority(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	if err := Upsert(db, "country_iso2", "Portugal", "PT", "seeded"); err != nil {
		t.Fatalf("Upsert seeded: %v", err)
	}
	if err := Upsert(db, "country_iso2", "Portugal", "PRT", "taught"); err != nil {
		t.Fatalf("Upsert taught: %v", err)
	}
	got, ok, err := Lookup(db, "country_iso2", "Portugal")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if !ok {
		t.Fatalf("Lookup found=false, want true")
	}
	if got != "PRT" {
		t.Errorf("Lookup returned %q, want %q (taught row should outrank seeded)", got, "PRT")
	}
}

// TestUpsert_NewKindWorksWithoutCodeChange pins the data-driven
// contract: a brand-new kind that the package has never seen before
// can be Upsert'd and Lookup'd without any code changes. This is the
// portability story for a future CLI that adds its own domain.
func TestUpsert_NewKindWorksWithoutCodeChange(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	if err := Upsert(db, "stock_ticker", "Apple Inc.", "AAPL", "taught"); err != nil {
		t.Fatalf("Upsert custom kind: %v", err)
	}
	got, ok, err := Lookup(db, "stock_ticker", "Apple Inc.")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if !ok || got != "AAPL" {
		t.Errorf("Lookup(stock_ticker, Apple Inc.) = (%q, %v), want (\"AAPL\", true)", got, ok)
	}
}

func TestUpsert_ValidationErrors(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	cases := []struct {
		name                       string
		kind, canonical, value     string
		expectErrSubstr            string
	}{
		{"empty kind", "", "Portugal", "PT", "kind is required"},
		{"whitespace kind", "  ", "Portugal", "PT", "kind is required"},
		{"empty canonical", "country_iso2", "", "PT", "canonical is required"},
		{"empty value", "country_iso2", "Portugal", "", "value is required"},
		{"computed kind", "lowercase", "Portugal", "portugal", "computed kind"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := Upsert(db, tc.kind, tc.canonical, tc.value, "taught")
			if err == nil {
				t.Fatalf("Upsert(%q,%q,%q): expected error containing %q, got nil",
					tc.kind, tc.canonical, tc.value, tc.expectErrSubstr)
			}
			if !contains(err.Error(), tc.expectErrSubstr) {
				t.Errorf("Upsert(%q,%q,%q) error = %v, want substring %q",
					tc.kind, tc.canonical, tc.value, err, tc.expectErrSubstr)
			}
		})
	}
}

func TestUpsert_DefaultSource(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	// Empty source should default to "taught" (matches teach-lookup
	// CLI behavior).
	if err := Upsert(db, "country_iso2", "Portugal", "PT", ""); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	var source string
	if err := db.QueryRow(`SELECT source FROM entity_lookups WHERE kind = ? AND canonical = ?`,
		"country_iso2", "Portugal").Scan(&source); err != nil {
		t.Fatalf("select source: %v", err)
	}
	if source != "taught" {
		t.Errorf("default source = %q, want %q", source, "taught")
	}
}

// TestSeedBatch_InsertsAndIsIdempotent checks that the migration-time
// batch insert lands every row on a fresh table and that re-running
// it on the same table inserts zero new rows (everything's a PK
// conflict, silenced by OR IGNORE).
func TestSeedBatch_InsertsAndIsIdempotent(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	seeds := []LookupRow{
		{Kind: "country_iso2", Canonical: "Portugal", Value: "PT", Source: "seeded"},
		{Kind: "country_iso2", Canonical: "England", Value: "GB", Source: "seeded"},
		{Kind: "country_iso3", Canonical: "Portugal", Value: "PRT", Source: "seeded"},
	}

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	inserted, err := SeedBatch(tx, seeds)
	if err != nil {
		tx.Rollback()
		t.Fatalf("first SeedBatch: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit first: %v", err)
	}
	if inserted != 3 {
		t.Errorf("first SeedBatch inserted = %d, want 3", inserted)
	}

	// Re-run: every triple is a PK conflict, so OR IGNORE skips
	// every row and inserted should be 0.
	tx2, err := db.Begin()
	if err != nil {
		t.Fatalf("begin tx2: %v", err)
	}
	inserted2, err := SeedBatch(tx2, seeds)
	if err != nil {
		tx2.Rollback()
		t.Fatalf("second SeedBatch: %v", err)
	}
	if err := tx2.Commit(); err != nil {
		t.Fatalf("commit second: %v", err)
	}
	if inserted2 != 0 {
		t.Errorf("second SeedBatch inserted = %d, want 0 (idempotent)", inserted2)
	}

	var total int
	if err := db.QueryRow(`SELECT COUNT(*) FROM entity_lookups`).Scan(&total); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if total != 3 {
		t.Errorf("table row count = %d, want 3", total)
	}
}

func TestSeedBatch_DefaultsSource(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	seeds := []LookupRow{
		{Kind: "country_iso2", Canonical: "Portugal", Value: "PT"}, // Source empty
	}
	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if _, err := SeedBatch(tx, seeds); err != nil {
		tx.Rollback()
		t.Fatalf("SeedBatch: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	var source string
	if err := db.QueryRow(`SELECT source FROM entity_lookups WHERE kind = ? AND canonical = ?`,
		"country_iso2", "Portugal").Scan(&source); err != nil {
		t.Fatalf("select: %v", err)
	}
	if source != "seeded" {
		t.Errorf("default seed source = %q, want %q", source, "seeded")
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
