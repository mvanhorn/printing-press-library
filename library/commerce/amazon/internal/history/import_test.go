package history

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/amazon/internal/store"
)

const sampleJSONL = `
{"order_id":"112-0000001-0000001","placed_at":"2026-04-15T00:00:00Z","total":"$42.31","items":[{"asin":"B07AAA0001","title":"Charmin 24 Mega","quantity":1,"unit_price":"$28.49"},{"asin":"B07BBB0002","title":"AA Batteries 48-pk","quantity":2,"unit_price":"$6.91"}]}
{"order_id":"112-0000002-0000002","placed_at":"January 5, 2026","total":"USD 19.99","items":[{"asin":"B07AAA0001","title":"Charmin 24 Mega","quantity":1}]}
# this is a comment line, should be skipped
{"order_id":"112-0000003-0000003","placed_at":"not a real date","items":[]}
`

func TestImportReaderUpsertsAndDedupes(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "h.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	res, err := ImportReader(context.Background(), s, strings.NewReader(sampleJSONL))
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if res.OrdersWritten != 2 {
		t.Errorf("OrdersWritten=%d want 2", res.OrdersWritten)
	}
	if res.ItemsWritten != 3 {
		t.Errorf("ItemsWritten=%d want 3", res.ItemsWritten)
	}
	if res.Skipped != 1 {
		t.Errorf("Skipped=%d want 1 (bad date)", res.Skipped)
	}
	// Check the most-frequent rollup
	rows, _, err := s.SearchPurchasedItems("charmin", 5)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 search hit, got %d", len(rows))
	}
	if rows[0].PurchaseCount != 2 {
		t.Errorf("Charmin purchase_count=%d want 2 (1+1)", rows[0].PurchaseCount)
	}
}

func TestParsePriceCents(t *testing.T) {
	cases := []struct {
		in   string
		want int64
	}{
		{"$42.31", 4231},
		{"USD 19.99", 1999},
		{"$1,234.56", 123456},
		{"42", 4200},
		{"", 0},
		{"---", 0},
	}
	for _, c := range cases {
		if got := parsePriceCents(c.in); got != c.want {
			t.Errorf("parsePriceCents(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestParseTimestamp(t *testing.T) {
	cases := []string{
		"2026-04-15T00:00:00Z",
		"2026-04-15",
		"April 15, 2026",
		"Apr 15, 2026",
	}
	for _, in := range cases {
		if _, err := parseTimestamp(in); err != nil {
			t.Errorf("parseTimestamp(%q) failed: %v", in, err)
		}
	}
	if _, err := parseTimestamp("totally bogus"); err == nil {
		t.Error("expected error for bogus timestamp")
	}
}
