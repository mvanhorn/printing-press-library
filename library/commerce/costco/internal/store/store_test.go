package store

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
)

func openTemp(t *testing.T) *Archive {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	a, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = a.Close() })
	return a
}

func sampleRows() []ReceiptRow {
	return []ReceiptRow{
		{
			ID: "M1|BC1", MembershipNumber: "M1", TransactionDate: "2025-01-10",
			Channel: "warehouse", WarehouseName: "Seattle", Total: 100, ItemCount: 2,
			Raw: json.RawMessage(`{"x":1}`),
			Items: []ItemRow{
				{ItemNumber: "111", UPC: "0001", Description: "ROTISSERIE CHICKEN", UnitPrice: 4.99, Amount: 4.99},
				{ItemNumber: "222", UPC: "0002", Description: "PAPER TOWELS", UnitPrice: 19.99, Amount: 19.99},
			},
		},
		{
			ID: "M1|BC2", MembershipNumber: "M1", TransactionDate: "2025-02-01",
			Channel: "gas", WarehouseName: "Seattle", Total: 50, ItemCount: 1,
			Raw:   json.RawMessage(`{"x":2}`),
			Items: []ItemRow{{ItemNumber: "333", UPC: "0003", Description: "REGULAR FUEL", Amount: 50}},
		},
	}
}

func TestUpsertIdempotent(t *testing.T) {
	a := openTemp(t)
	ctx := context.Background()

	res, err := a.Upsert(ctx, sampleRows())
	if err != nil {
		t.Fatal(err)
	}
	if res.Added != 2 || res.Updated != 0 {
		t.Fatalf("first upsert = %+v, want Added 2 Updated 0", res)
	}

	// Re-syncing the same receipts updates, never duplicates.
	res2, err := a.Upsert(ctx, sampleRows())
	if err != nil {
		t.Fatal(err)
	}
	if res2.Added != 0 || res2.Updated != 2 {
		t.Fatalf("second upsert = %+v, want Added 0 Updated 2", res2)
	}

	count, err := a.Count(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("count = %d, want 2 (dedup by id)", count)
	}
}

func TestSearchItems(t *testing.T) {
	a := openTemp(t)
	ctx := context.Background()
	if _, err := a.Upsert(ctx, sampleRows()); err != nil {
		t.Fatal(err)
	}
	rows, err := a.SearchItems(ctx, "chicken", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("search 'chicken' = %d rows, want 1", len(rows))
	}
	if rows[0]["description"] != "ROTISSERIE CHICKEN" {
		t.Fatalf("unexpected match: %v", rows[0])
	}
	// Negative: a non-matching term returns nothing, not everything.
	none, err := a.SearchItems(ctx, "lawnmower", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(none) != 0 {
		t.Fatalf("non-matching search returned %d rows, want 0", len(none))
	}
}

func TestQueryJSONReadOnly(t *testing.T) {
	a := openTemp(t)
	ctx := context.Background()
	if _, err := a.Upsert(ctx, sampleRows()); err != nil {
		t.Fatal(err)
	}
	rows, err := a.QueryJSON(ctx, "SELECT channel, COUNT(*) AS c FROM receipts GROUP BY channel", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("group-by returned %d rows, want 2", len(rows))
	}

	// Mutating statements must be rejected — both leading-verb and chained forms.
	for _, q := range []string{
		"DELETE FROM receipts",
		"DROP TABLE items",
		"UPDATE receipts SET total=0",
		"SELECT 1; DROP TABLE receipts;", // chained, trailing ;
		"SELECT 1; DELETE FROM receipts", // chained, no trailing ;
	} {
		if _, err := a.QueryJSON(ctx, q, 0); err == nil {
			t.Fatalf("expected %q to be rejected", q)
		}
	}
}

func TestOpenReadOnlyBlocksWrites(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "ro.db")
	// Seed with a read-write open.
	rw, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rw.Upsert(ctx, sampleRows()); err != nil {
		t.Fatal(err)
	}
	_ = rw.Close()

	ro, err := OpenReadOnly(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ro.Close() })

	// Reads work.
	if n, err := ro.Count(ctx); err != nil || n != 2 {
		t.Fatalf("read-only Count = %d err=%v, want 2", n, err)
	}
	// Writes are blocked at the engine level (query_only pragma).
	if _, err := ro.db.ExecContext(ctx, "DELETE FROM receipts"); err == nil {
		t.Fatal("expected DELETE on a read-only connection to fail")
	}
}
