// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMissingSavedStoreIsReadOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "facts.sqlite")
	xs, n, e := Cached(context.Background(), path, "", 10)
	if e != nil || xs == nil || n != 0 {
		t.Fatal(xs, n, e)
	}
	if _, e = os.Stat(filepath.Dir(path)); !os.IsNotExist(e) {
		t.Fatal("read created directories")
	}
}
func TestNewestObservationWinsInstantAware(t *testing.T) {
	path := filepath.Join(t.TempDir(), "facts.sqlite")
	ctx := context.Background()
	newer := Service{ID: tsumago, NameJA: "newest", ObservedAt: "2026-10-04T12:00:00.01Z"}
	if e := Save(ctx, path, newer); e != nil {
		t.Fatal(e)
	}
	for _, stamp := range []string{"2026-10-04T12:00:00Z", "2026-10-04T21:00:00+09:00", "2026-10-04T12:00:00.001Z"} {
		older := newer
		older.NameJA = "older"
		older.ObservedAt = stamp
		if e := Save(ctx, path, older); e != nil {
			t.Fatal(e)
		}
	}
	xs, n, e := Cached(ctx, path, "", 10)
	if e != nil || n != 1 || len(xs) != 1 || xs[0].NameJA != "newest" {
		t.Fatal(xs, n, e)
	}
}
func TestCacheBoundsAndQualifiedFacts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "facts.sqlite")
	ctx := context.Background()
	base := time.Now()
	for i := 0; i < 205; i++ {
		s := Service{ID: fmt.Sprintf("%08x-ef99-4115-95e5-be5227cdc74e", i), NameJA: "test", ObservedAt: base.Add(time.Duration(i) * time.Second).Format(time.RFC3339Nano), Price: ParsePrice("実費負担", "")}
		if e := Save(ctx, path, s); e != nil {
			t.Fatal(e)
		}
	}
	xs, n, e := Cached(ctx, path, "", 3)
	if e != nil || n != Capacity || len(xs) != 3 || xs[0].Price.Status != "expenses" || xs[0].Transport != "local" {
		t.Fatal(xs, n, e)
	}
	if _, _, e = Cached(ctx, path, "", 51); e == nil {
		t.Fatal("limit overflow")
	}
}

func TestRuntimeSQLiteNormalTransaction(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "normal.sqlite")
	db, e := openDB(ctx, path, true)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	var version string
	if e = db.QueryRowContext(ctx, "SELECT sqlite_version()").Scan(&version); e != nil {
		t.Fatal(e)
	}
	if version != "3.53.4" {
		t.Fatalf("unexpected SQLite runtime %s", version)
	}
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = tx.ExecContext(ctx, "CREATE TABLE transaction_probe(value INTEGER)"); e != nil {
		t.Fatal(e)
	}
	if _, e = tx.ExecContext(ctx, "INSERT INTO transaction_probe VALUES(47)"); e != nil {
		t.Fatal(e)
	}
	if e = tx.Commit(); e != nil {
		t.Fatal(e)
	}
	var value int
	if e = db.QueryRowContext(ctx, "SELECT value FROM transaction_probe").Scan(&value); e != nil || value != 47 {
		t.Fatal(value, e)
	}
	t.Logf("SQLite %s normal transaction verified", version)
}
