package cli

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/store"
)

func s2Ledger(id int, daysAgo int, amount int, ledgerType, refType, desc string) string {
	ts := s2Epoch.Add(-time.Duration(daysAgo) * 24 * time.Hour).Unix()
	return fmt.Sprintf(`{"entry_id":%d,"ledger_id":1,"sequence_number":%d,"amount":%d,"currency":"USD","description":%q,"balance":0,
	 "create_date":%d,"created_timestamp":%d,"ledger_type":%q,"reference_type":%q,"reference_id":"r%d"}`,
		id, id, amount, desc, ts, ts, ledgerType, refType, id)
}

func s2LedgerFixture(t *testing.T) string {
	return s2FixtureDB(t, func(db *store.Store) {
		for _, e := range []string{
			s2Ledger(1, 2, 5000, "PAYMENT", "payment", "Payment for order"),              // Nov 14 sale
			s2Ledger(2, 5, 3000, "PAYMENT", "payment", "Payment for order"),              // Nov 11 sale
			s2Ledger(3, 27, 2000, "PAYMENT", "payment", "Payment for order"),             // Oct 20 sale
			s2Ledger(4, 2, -325, "transaction", "transaction", "Transaction fee"),        // Nov 14 fee
			s2Ledger(5, 27, -20, "listing", "listing", "Listing fee"),                    // Oct 20 fee
			s2Ledger(6, 4, -1500, "refund", "payment", "Refund for order"),               // Nov 12 refund
			s2Ledger(7, 1, -6000, "DISBURSE", "payment_account", "Disbursement to bank"), // Nov 15 payout
			s2Ledger(8, 47, 9999, "PAYMENT", "payment", "Payment for order"),             // outside 30d window
			s2Ledger(9, 3, 250, "misc", "adjustment", "Miscellaneous credit"),            // Nov 13 other
		} {
			s2Upsert(t, db.UpsertLedgerEntries, e)
		}
	})
}

func TestMoneySummaryTotalsMatchHandComputedAndExcludeOldEntries(t *testing.T) {
	s2SetNow(t, s2Epoch)
	out, _, err := s2Run(t, "money", "summary", "--since", "30d", "--db", s2LedgerFixture(t), "--json")
	s2Must(t, err)
	var res s2MoneySummary
	shopxDecodeResults(t, out, &res)
	// in window: sales 5000+3000+2000, fees 325+20, refund 1500, payout 6000, other +250 (the 9999 is 47d old)
	if res.Entries != 8 || len(res.Totals) != 1 {
		t.Fatalf("summary = %s", out)
	}
	got := res.Totals[0]
	want := s2MoneyTotals{Currency: "USD", Divisor: 100, GrossMinor: 10000, FeesMinor: 345, RefundsMinor: 1500, NetMinor: 8155, PayoutsMinor: 6000, OtherMinor: 250, Entries: 8}
	if got != want {
		t.Fatalf("totals = %+v\nwant     %+v", got, want)
	}
	if len(res.Periods) != 2 || res.Periods[0].Period != "2026-10" || res.Periods[1].Period != "2026-11" {
		t.Fatalf("periods = %+v", res.Periods)
	}
	oct, nov := res.Periods[0], res.Periods[1]
	if oct.GrossMinor != 2000 || oct.FeesMinor != 20 || oct.NetMinor != 1980 ||
		nov.GrossMinor != 8000 || nov.FeesMinor != 325 || nov.RefundsMinor != 1500 || nov.NetMinor != 6175 || nov.PayoutsMinor != 6000 {
		t.Fatalf("period split oct=%+v nov=%+v", oct, nov)
	}
	// a wider window includes the old sale
	out, _, _ = s2Run(t, "money", "summary", "--since", "60d", "--period", "none", "--db", s2LedgerFixture(t), "--json")
	shopxDecodeResults(t, out, &res)
	if res.Totals[0].GrossMinor != 19999 || len(res.Periods) != 0 {
		t.Fatalf("60d = %s", out)
	}
	// a narrow window excludes the older entries
	out, _, _ = s2Run(t, "money", "summary", "--since", "3d", "--period", "none", "--db", s2LedgerFixture(t), "--json")
	shopxDecodeResults(t, out, &res)
	if res.Totals[0].GrossMinor != 5000 || res.Totals[0].FeesMinor != 325 || res.Totals[0].PayoutsMinor != 6000 || res.Totals[0].OtherMinor != 250 || res.Entries != 4 /* window start is inclusive */ {
		t.Fatalf("3d = %s", out)
	}
}

func TestMoneySummaryEmptyMissingAndBadFlags(t *testing.T) {
	s2SetNow(t, s2Epoch)
	out, _, err := s2Run(t, "money", "summary", "--db", s2FixtureDB(t, nil), "--json")
	s2Must(t, err)
	var res s2MoneySummary
	shopxDecodeResults(t, out, &res)
	if res.Entries != 0 || len(res.Totals) != 0 || !strings.Contains(out, "no ledger entries in this window") || !strings.Contains(out, `"totals": []`) {
		t.Fatalf("empty = %s", out)
	}
	out, errOut, err := s2Run(t, "money", "summary", "--db", t.TempDir()+"/absent.db", "--json")
	s2Must(t, err)
	shopxDecodeResults(t, out, &res)
	if res.Entries != 0 || !strings.Contains(errOut, "sync") {
		t.Fatalf("missing: out=%q err=%q", out, errOut)
	}
	for _, args := range [][]string{{"--since", "abc"}, {"--since", "0d"}, {"--period", "year"}} {
		if _, _, err := s2Run(t, append([]string{"money", "summary", "--db", t.TempDir() + "/x.db"}, args...)...); err == nil || ExitCode(err) != 2 {
			t.Fatalf("%v: err = %v", args, err)
		}
	}
	out, _, err = s2Run(t, "money", "summary", "--dry-run", "--json")
	s2Must(t, err)
	var env map[string]any
	s2Decode(t, out, &env)
	if env["dry_run"] != true {
		t.Fatalf("dry-run = %s", out)
	}
}

func TestLedgerCategoryClassification(t *testing.T) {
	for _, tc := range []struct{ lt, rt, desc, want string }{
		{"PAYMENT", "payment", "Payment for order", s2CatSale},
		{"transaction", "transaction", "Transaction fee", s2CatFee},
		{"listing", "listing", "Listing fee", s2CatFee},
		{"BILL_PAYMENT", "bill", "Bill payment", s2CatFee},
		{"refund", "payment", "Refund", s2CatRefund},
		{"DISBURSE", "payment_account", "Disbursement", s2CatPayout},
		{"returned_disbursement", "", "", s2CatPayout},
		{"misc", "adjustment", "Misc credit", s2CatOther},
	} {
		if got := s2LedgerCategory(tc.lt, tc.rt, tc.desc); got != tc.want {
			t.Errorf("%q/%q/%q = %s, want %s", tc.lt, tc.rt, tc.desc, got, tc.want)
		}
	}
}
