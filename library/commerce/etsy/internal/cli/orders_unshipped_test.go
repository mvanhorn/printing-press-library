package cli

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/store"
)

func s2SetNow(t *testing.T, now time.Time) {
	old := s2Now
	s2Now = func() time.Time { return now }
	t.Cleanup(func() { s2Now = old })
}

var s2Epoch = time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)

func s2Receipt(id int, ageDays int, paid, shipped bool, status string) string {
	ts := s2Epoch.Add(-time.Duration(ageDays) * 24 * time.Hour).Unix()
	return fmt.Sprintf(`{"receipt_id":%d,"created_timestamp":%d,"is_paid":%v,"is_shipped":%v,"status":%q,
	 "name":"Jordan Example","buyer_email":"jordan@example.test","buyer_user_id":777000%d,"first_line":"12 Pine Lane","second_line":"Apt 4",
	 "city":"Springfield","state":"IL","zip":"62704","formatted_address":"Jordan Example 12 Pine Lane Springfield IL","country_iso":"US",
	 "message_from_buyer":"Please gift wrap","grandtotal":{"amount":%d,"divisor":100,"currency_code":"USD"},
	 "transactions":[{"transaction_id":1},{"transaction_id":2}]}`, id, ts, paid, shipped, status, id, 2000+id)
}

func s2ReceiptFixture(t *testing.T) string {
	return s2FixtureDB(t, func(db *store.Store) {
		for _, r := range []string{
			s2Receipt(3002, 3, true, false, "paid"),     // newer paid/unshipped
			s2Receipt(3001, 10, true, false, "paid"),    // oldest paid/unshipped
			s2Receipt(3003, 6, true, true, "completed"), // shipped
			s2Receipt(3004, 2, false, false, "open"),    // unpaid
			s2Receipt(3005, 8, true, false, "canceled"), // canceled
		} {
			s2Upsert(t, db.UpsertReceipts, r)
		}
	})
}

func TestOrdersUnshippedFiltersSortsAndMinimizesPII(t *testing.T) {
	s2SetNow(t, s2Epoch)
	db := s2ReceiptFixture(t)
	out, _, err := s2Run(t, "orders", "unshipped", "--db", db, "--json")
	s2Must(t, err)
	var rows []map[string]any
	shopxDecodeResults(t, out, &rows)
	if len(rows) != 2 {
		t.Fatalf("rows = %s", out)
	}
	if rows[0]["receipt_id"] != 3001.0 || rows[1]["receipt_id"] != 3002.0 {
		t.Fatalf("not oldest-first: %s", out)
	}
	if rows[0]["age_days"] != 10.0 || rows[1]["age_days"] != 3.0 || rows[0]["grand_total"] != 50.01 || rows[0]["currency"] != "USD" ||
		rows[0]["country_iso"] != "US" || rows[0]["item_count"] != 2.0 || rows[0]["has_buyer_message"] != true {
		t.Fatalf("row = %v", rows[0])
	}
	for _, banned := range []string{"name", "buyer_email", "buyer_user_id", "first_line", "second_line", "city", "state", "zip", "formatted_address", "message_from_buyer"} {
		for _, r := range rows {
			if _, present := r[banned]; present {
				t.Fatalf("default output leaks %q: %s", banned, out)
			}
		}
	}
	for _, pii := range []string{"Jordan", "jordan@example.test", "Pine Lane", "Springfield", "gift wrap"} {
		if strings.Contains(out, pii) {
			t.Fatalf("default output contains %q: %s", pii, out)
		}
	}
}

func TestOrdersUnshippedFullIncludesBuyerFields(t *testing.T) {
	s2SetNow(t, s2Epoch)
	out, _, err := s2Run(t, "orders", "unshipped", "--full", "--limit", "1", "--db", s2ReceiptFixture(t), "--json")
	s2Must(t, err)
	var rows []map[string]any
	shopxDecodeResults(t, out, &rows)
	if len(rows) != 1 || rows[0]["name"] != "Jordan Example" || rows[0]["buyer_email"] != "jordan@example.test" ||
		rows[0]["first_line"] != "12 Pine Lane" || rows[0]["message_from_buyer"] != "Please gift wrap" || rows[0]["receipt_id"] != 3001.0 {
		t.Fatalf("--full row = %s", out)
	}
}

func TestOrdersUnshippedEmptyAndMissingStore(t *testing.T) {
	out, _, err := s2Run(t, "orders", "unshipped", "--db", s2FixtureDB(t, nil), "--json")
	s2Must(t, err)
	if len(shopxMustList(t, out)) != 0 {
		t.Fatalf("empty = %q", out)
	}
	out, errOut, err := s2Run(t, "orders", "unshipped", "--db", t.TempDir()+"/absent.db", "--json")
	s2Must(t, err)
	if len(shopxMustList(t, out)) != 0 || !strings.Contains(errOut, "sync") {
		t.Fatalf("missing: out=%q err=%q", out, errOut)
	}
	out, _, err = s2Run(t, "orders", "unshipped", "--dry-run", "--json")
	s2Must(t, err)
	var env map[string]any
	s2Decode(t, out, &env)
	if env["dry_run"] != true {
		t.Fatalf("dry-run = %s", out)
	}
	if _, _, err := s2Run(t, "orders", "--help"); err != nil {
		t.Fatal(err)
	}
}

func TestOrdersUnshippedRefundsAgreeWithAttention(t *testing.T) {
	f := shopxNewFixture(t)
	rows := []map[string]any{}
	for i, status := range []string{"Fully Refunded", "fully_refunded", "fully-refunded", "Partially Refunded", "paid", "canceled", "cancelled"} {
		rows = append(rows, map[string]any{"receipt_id": i + 1, "is_paid": true, "is_shipped": false, "status": status})
	}
	rows = append(rows,
		map[string]any{"receipt_id": 8, "is_paid": false, "was_paid": true},
		map[string]any{"receipt_id": 9, "is_paid": true, "is_shipped": false, "was_shipped": true},
		map[string]any{"receipt_id": 10, "was_paid": true, "was_shipped": true},
		map[string]any{"receipt_id": 11, "was_paid": true})
	f.put("receipts", rows...)
	out, _, err := shopxRun(t, f, "orders", "unshipped")
	s2Must(t, err)
	got := shopxMustList(t, out)
	if len(got) != 4 {
		t.Fatalf("rows: %s", out)
	}
	ids := map[float64]bool{}
	for _, row := range got {
		ids[row["receipt_id"].(float64)] = true
	}
	for _, id := range []float64{4, 5, 9, 11} {
		if !ids[id] {
			t.Fatalf("missing receipt %v: %s", id, out)
		}
	}
	out, _, err = shopxRun(t, f, "shop", "attention")
	s2Must(t, err)
	if shopxMustObject(t, out)["unshipped_count"] != float64(len(got)) {
		t.Fatalf("counts disagree: %s", out)
	}
}

func TestOrdersUnshippedHelpNamesBuyerUserID(t *testing.T) {
	out, _, err := s2Run(t, "orders", "unshipped", "--help")
	s2Must(t, err)
	if !strings.Contains(out, "buyer_user_id") {
		t.Fatalf("help: %s", out)
	}
}
