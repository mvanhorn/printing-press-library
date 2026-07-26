// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0. See LICENSE.

package crate

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func newStore(t *testing.T) (*Store, context.Context) {
	t.Helper()
	ctx := context.Background()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	s, err := Open(ctx, db)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	return s, ctx
}

func TestRecordsRoundTrip(t *testing.T) {
	s, ctx := newStore(t)
	in := shelf()
	if err := s.ReplaceRecords(ctx, "example", false, in); err != nil {
		t.Fatalf("replace: %v", err)
	}
	out, err := s.Records(ctx, "example", false)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(out) != len(in) {
		t.Fatalf("got %d records, want %d", len(out), len(in))
	}
	var blueTrain Record
	for _, r := range out {
		if r.ReleaseID == 1 {
			blueTrain = r
		}
	}
	if blueTrain.Title != "Blue Train" || blueTrain.Year != 1957 || blueTrain.Rating != 5 {
		t.Errorf("scalar fields lost: %+v", blueTrain)
	}
	if len(blueTrain.Genres) != 1 || blueTrain.Genres[0] != "Jazz" {
		t.Errorf("genres lost: %v", blueTrain.Genres)
	}
	if len(blueTrain.Formats) != 2 {
		t.Errorf("multi-value formats lost: %v", blueTrain.Formats)
	}
}

// A record sold or removed upstream must disappear locally. An upsert-only
// sync would keep it on the shelf forever.
func TestReplaceRemovesVanishedRecords(t *testing.T) {
	s, ctx := newStore(t)
	if err := s.ReplaceRecords(ctx, "example", false, shelf()); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceRecords(ctx, "example", false, shelf()[:2]); err != nil {
		t.Fatal(err)
	}
	out, _ := s.Records(ctx, "example", false)
	if len(out) != 2 {
		t.Errorf("after a shorter sync the store holds %d records, want 2", len(out))
	}
}

func TestCollectionAndWantlistAreSeparate(t *testing.T) {
	s, ctx := newStore(t)
	if err := s.ReplaceRecords(ctx, "example", false, shelf()[:2]); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceRecords(ctx, "example", true, shelf()[2:]); err != nil {
		t.Fatal(err)
	}
	owned, _ := s.Records(ctx, "example", false)
	wanted, _ := s.Records(ctx, "example", true)
	if len(owned) != 2 || len(wanted) != 2 {
		t.Errorf("owned=%d wanted=%d, want 2 and 2", len(owned), len(wanted))
	}
	// Replacing the wantlist must not touch the collection.
	if err := s.ReplaceRecords(ctx, "example", true, nil); err != nil {
		t.Fatal(err)
	}
	owned, _ = s.Records(ctx, "example", false)
	if len(owned) != 2 {
		t.Errorf("clearing the wantlist removed collection rows: %d left", len(owned))
	}
}

func TestUsersAreIsolated(t *testing.T) {
	s, ctx := newStore(t)
	_ = s.ReplaceRecords(ctx, "alice", false, shelf()[:2])
	_ = s.ReplaceRecords(ctx, "bob", false, shelf()[2:])
	a, _ := s.Records(ctx, "alice", false)
	b, _ := s.Records(ctx, "bob", false)
	if len(a) != 2 || len(b) != 2 {
		t.Errorf("alice=%d bob=%d, want 2 and 2", len(a), len(b))
	}
	_ = s.ReplaceRecords(ctx, "alice", false, nil)
	b, _ = s.Records(ctx, "bob", false)
	if len(b) != 2 {
		t.Errorf("clearing alice removed bob's records: %d left", len(b))
	}
}

func TestOwnedIDs(t *testing.T) {
	s, ctx := newStore(t)
	_ = s.ReplaceRecords(ctx, "example", false, shelf()[:2])
	_ = s.ReplaceRecords(ctx, "example", true, shelf()[2:])
	ids, err := s.OwnedIDs(ctx, "example")
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 || !ids[1] || !ids[2] {
		t.Errorf("owned ids = %v, want {1,2} only (wantlist must not count as owned)", ids)
	}
}

// A release with nothing for sale has no price. Storing that as zero would
// silently drag any floor total down.
func TestPriceDistinguishesAbsentFromZero(t *testing.T) {
	s, ctx := newStore(t)
	now := time.Now()
	_ = s.PutPrice(ctx, Price{ReleaseID: 1, Lowest: 12.5, Currency: "USD", NumForSale: 3, HasPrice: true, FetchedAt: now})
	_ = s.PutPrice(ctx, Price{ReleaseID: 2, HasPrice: false, FetchedAt: now})

	got, err := s.Prices(ctx, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if !got[1].HasPrice || got[1].Lowest != 12.5 {
		t.Errorf("priced release lost its price: %+v", got[1])
	}
	if got[2].HasPrice {
		t.Errorf("release with nothing for sale should report HasPrice false: %+v", got[2])
	}
}

func TestPricesRespectMaxAge(t *testing.T) {
	s, ctx := newStore(t)
	_ = s.PutPrice(ctx, Price{ReleaseID: 1, Lowest: 5, HasPrice: true, FetchedAt: time.Now().Add(-48 * time.Hour)})
	_ = s.PutPrice(ctx, Price{ReleaseID: 2, Lowest: 6, HasPrice: true, FetchedAt: time.Now()})

	fresh, _ := s.Prices(ctx, time.Hour, "")
	if _, ok := fresh[1]; ok {
		t.Error("a 48-hour-old price should be excluded by a 1-hour max age")
	}
	if _, ok := fresh[2]; !ok {
		t.Error("a fresh price should be included")
	}
	all, _ := s.Prices(ctx, 0, "")
	if len(all) != 2 {
		t.Errorf("maxAge 0 should return every cached price, got %d", len(all))
	}
}

func TestPutPriceUpserts(t *testing.T) {
	s, ctx := newStore(t)
	_ = s.PutPrice(ctx, Price{ReleaseID: 1, Lowest: 5, HasPrice: true, FetchedAt: time.Now()})
	_ = s.PutPrice(ctx, Price{ReleaseID: 1, Lowest: 9, HasPrice: true, FetchedAt: time.Now()})
	got, _ := s.Prices(ctx, 0, "")
	if len(got) != 1 || got[1].Lowest != 9 {
		t.Errorf("re-pricing should overwrite, got %+v", got)
	}
}

func TestSyncInfo(t *testing.T) {
	s, ctx := newStore(t)
	if _, _, ok := s.SyncInfo(ctx, "example", "collection"); ok {
		t.Error("expected no sync record before syncing")
	}
	_ = s.ReplaceRecords(ctx, "example", false, shelf())
	at, n, ok := s.SyncInfo(ctx, "example", "collection")
	if !ok || n != 4 {
		t.Errorf("sync info = (%v, %d, %v), want 4 items", at, n, ok)
	}
	if time.Since(at) > time.Minute {
		t.Errorf("sync timestamp looks wrong: %v", at)
	}
}

func TestOpenIsIdempotent(t *testing.T) {
	s, ctx := newStore(t)
	_ = s.ReplaceRecords(ctx, "example", false, shelf())
	again, err := Open(ctx, s.db)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	out, _ := again.Records(ctx, "example", false)
	if len(out) != 4 {
		t.Errorf("reopening dropped data: %d rows", len(out))
	}
}

// Prices are cached per requested currency. Keyed on release_id alone, a GBP
// price is served to a USD request and --currency appears to do nothing.
func TestPricesAreScopedToRequestedCurrency(t *testing.T) {
	s, ctx := newStore(t)
	now := time.Now()
	_ = s.PutPrice(ctx, Price{ReleaseID: 1, ReqCurrency: "GBP", Lowest: 10, Currency: "GBP", HasPrice: true, NumForSale: 1, FetchedAt: now})
	_ = s.PutPrice(ctx, Price{ReleaseID: 1, ReqCurrency: "USD", Lowest: 13, Currency: "USD", HasPrice: true, NumForSale: 1, FetchedAt: now})

	gbp, err := s.Prices(ctx, 0, "GBP")
	if err != nil {
		t.Fatal(err)
	}
	if gbp[1].Lowest != 10 || gbp[1].Currency != "GBP" {
		t.Errorf("GBP request got %+v", gbp[1])
	}
	usd, _ := s.Prices(ctx, 0, "USD")
	if usd[1].Lowest != 13 || usd[1].Currency != "USD" {
		t.Errorf("USD request got %+v", usd[1])
	}
	none, _ := s.Prices(ctx, 0, "")
	if _, ok := none[1]; ok {
		t.Error("a default-currency request must not be served a GBP or USD cache entry")
	}
}

// A database created before crate_prices gained req_currency keeps the old
// single-column-key shape, because CREATE TABLE IF NOT EXISTS is a no-op
// against an existing table. Every price read then fails with
// "no such column: req_currency", which takes out floor and deals entirely.
// Open must repair the table instead of inheriting it.
func TestOpenMigratesLegacyPricesTable(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "legacy.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	// The pre-req_currency shape, verbatim.
	if _, err := db.ExecContext(ctx, `
CREATE TABLE crate_prices (
	release_id   INTEGER PRIMARY KEY,
	lowest       REAL NOT NULL DEFAULT 0,
	currency     TEXT NOT NULL DEFAULT '',
	num_for_sale INTEGER NOT NULL DEFAULT 0,
	has_price    INTEGER NOT NULL DEFAULT 0,
	fetched_at   TEXT NOT NULL
);
INSERT INTO crate_prices VALUES (1, 9.99, 'USD', 3, 1, '2026-01-01T00:00:00Z');`); err != nil {
		t.Fatalf("seeding the legacy table: %v", err)
	}

	s, err := Open(ctx, db)
	if err != nil {
		t.Fatalf("open on a legacy database: %v", err)
	}
	// The read that used to fail.
	if _, err := s.Prices(ctx, 0, "USD"); err != nil {
		t.Fatalf("reading prices after migration: %v", err)
	}
	now := time.Now()
	if err := s.PutPrice(ctx, Price{ReleaseID: 2, ReqCurrency: "USD", Lowest: 5, Currency: "USD", HasPrice: true, NumForSale: 1, FetchedAt: now}); err != nil {
		t.Fatalf("writing a price after migration: %v", err)
	}
	got, err := s.Prices(ctx, 0, "USD")
	if err != nil {
		t.Fatal(err)
	}
	if got[2].Lowest != 5 {
		t.Errorf("post-migration write not readable: %+v", got[2])
	}

	// Re-opening must not drop the rebuilt table a second time.
	again, err := Open(ctx, db)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	after, _ := again.Prices(ctx, 0, "USD")
	if _, ok := after[2]; !ok {
		t.Error("a second Open re-ran the migration and dropped the cache")
	}
}
