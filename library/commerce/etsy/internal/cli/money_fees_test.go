// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil/testenv"
)

// TestNovelMoneyFeesHelpWires smoke-tests that the money fees command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelMoneyFeesHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"money", "fees", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("money fees --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "fees"} {
		if !strings.Contains(help, want) {
			t.Fatalf("money fees --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestNovelMoneyFeesGroups(t *testing.T) {
	f := shopxNewFixture(t)
	f.put("ledger_entries",
		shopxFeeTestRow(1, "LISTING", "USD", -20, shopxAgo(5*shopxDay)),
		shopxFeeTestRow(2, "LISTING", "USD", -20, shopxAgo(5*shopxDay)),
		shopxFeeTestRow(3, "TRANSACTION", "USD", -300, shopxAgo(shopxDay)),
		shopxFeeTestRow(4, "LISTING", "EUR", -45, shopxAgo(shopxDay)),
		shopxFeeTestRow(5, "LISTING", "JPY", -500, shopxAgo(shopxDay)),
		shopxFeeTestRow(6, "RENEW", "USD", -99, shopxAgo(60*shopxDay)),
		shopxFeeTestRow(7, "LISTING", "USD", -99, nil),
		shopxFeeTestRow(8, "LISTING", "USD", -99, shopxIn(2*shopxDay)),
	)
	out, _, err := shopxRun(t, f, "money", "fees")
	if err != nil {
		t.Fatal(err)
	}
	want := []map[string]any{
		{"group": "LISTING", "currency": "EUR", "count": float64(1), "total_minor": float64(-45), "total": "-0.45", "group_by": "ledger_type"},
		{"group": "LISTING", "currency": "JPY", "count": float64(1), "total_minor": float64(-500), "total": "-500", "group_by": "ledger_type"},
		{"group": "LISTING", "currency": "USD", "count": float64(2), "total_minor": float64(-40), "total": "-0.40", "group_by": "ledger_type"},
		{"group": "TRANSACTION", "currency": "USD", "count": float64(1), "total_minor": float64(-300), "total": "-3.00", "group_by": "ledger_type"},
	}
	if got := shopxMustList(t, out); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
	out, _, err = shopxRun(t, f, "money", "fees", "--since", "90d")
	if err != nil {
		t.Fatal(err)
	}
	rows := shopxMustList(t, out)
	if len(rows) != 5 || rows[3]["group"] != "RENEW" || rows[3]["total_minor"] != float64(-99) {
		t.Fatalf("90d rows: %#v", rows)
	}
}

func shopxFeeTestRow(id int, group, currency string, amount int64, ts any) map[string]any {
	return map[string]any{"entry_id": id, "ledger_type": group, "currency": currency, "amount": amount, "created_timestamp": ts}
}

func TestNovelMoneyFeesMonths(t *testing.T) {
	f := shopxNewFixture(t)
	oct := shopxAgo(30 * shopxDay)
	nov := shopxAgo(shopxDay)
	row := shopxFeeTestRow(1, "LISTING", "USD", -20, nil)
	row["create_date"] = oct
	f.put("ledger_entries", row, shopxFeeTestRow(2, "LISTING", "USD", -45, nov))
	out, _, err := shopxRun(t, f, "money", "fees", "--since", "60d", "--group-by", "month")
	if err != nil {
		t.Fatal(err)
	}
	rows := shopxMustList(t, out)
	if len(rows) != 2 {
		t.Fatalf("rows: %#v", rows)
	}
	for i, ts := range []int64{oct, nov} {
		amount := []float64{-20, -45}[i]
		if rows[i]["group"] != time.Unix(ts, 0).UTC().Format("2006-01") || rows[i]["total_minor"] != amount || rows[i]["count"] != float64(1) || rows[i]["group_by"] != "month" {
			t.Fatalf("row: %#v", rows[i])
		}
	}
}

func TestNovelMoneyFeesEmptyAndInvalid(t *testing.T) {
	for _, f := range []*shopxFixture{nil, shopxNewFixture(t)} {
		out, _, err := shopxRun(t, f, "money", "fees")
		if err != nil {
			t.Fatal(err)
		}
		if rows := shopxMustList(t, out); len(rows) != 0 {
			t.Fatalf("rows: %#v", rows)
		}
	}
	for _, flag := range []string{"--since", "--group-by"} {
		if _, _, err := shopxRun(t, nil, "money", "fees", flag, "banana"); err == nil {
			t.Fatalf("%s accepted banana", flag)
		}
	}
}

func TestNovelMoneyFeesOnlyFees(t *testing.T) {
	f := shopxNewFixture(t)
	sale := shopxFeeTestRow(3, "PAYMENT", "USD", 5000, shopxAgo(shopxDay))
	sale["reference_type"], sale["description"] = "payment", "payment"
	payout := shopxFeeTestRow(5, "DISBURSE2", "USD", -4000, shopxAgo(shopxDay))
	payout["description"] = "disbursement"
	f.put("ledger_entries",
		shopxFeeTestRow(1, "LISTING", "USD", -20, shopxAgo(shopxDay)),
		shopxFeeTestRow(2, "transaction_fee", "USD", -300, shopxAgo(shopxDay)),
		sale, shopxFeeTestRow(4, "REFUND", "USD", -1500, shopxAgo(shopxDay)),
		payout, shopxFeeTestRow(6, "recoupment", "USD", -700, shopxAgo(shopxDay)),
		shopxFeeTestRow(7, "miscellaneous debit", "USD", -50, shopxAgo(shopxDay)))
	out, _, err := shopxRun(t, f, "money", "fees")
	if err != nil {
		t.Fatal(err)
	}
	rows := shopxMustList(t, out)
	if len(rows) != 2 {
		t.Fatalf("groups: %s", out)
	}
	for i, want := range []struct {
		group  string
		amount float64
	}{{"LISTING", -20}, {"transaction_fee", -300}} {
		if rows[i]["group"] != want.group || rows[i]["count"] != float64(1) || rows[i]["total_minor"] != want.amount {
			t.Fatalf("groups: %s", out)
		}
	}
	_, notes := shopxMustEnvelope(t, out)
	for _, want := range []string{
		"5 ledger entries in the window were not counted as fees (sales, refunds, payouts, credits or unrecognized types)",
		"1 in-window debit entries had an unrecognized ledger type and were excluded; fee classification is an allowlist (fee, listing, renew, processing, vat, tax, ads, label, bill, subscription, transaction)",
	} {
		found := false
		for _, note := range notes {
			if note == want {
				found = true
			}
		}
		if !found {
			t.Fatalf("missing %q: %v", want, notes)
		}
	}
}

func TestShopxLedgerIsFee(t *testing.T) {
	for _, tc := range []struct {
		typ, ref, desc string
		amount         int64
		want           bool
	}{
		{"LISTING", "", "", -20, true},
		{"transaction_fee", "", "", -20, true},
		{"etsy_ads", "", "", -20, true},
		{"shipping_label", "", "", -20, true},
		{"VAT_seller_services", "", "", -20, true},
		{"renew_expired", "", "", -20, true},
		{"", "", "bill payment", -20, true},
		{"", "processing", "", -20, true},
		{"", "", "tax", -20, true},
		{"loads", "", "", -20, false},
		{"taxi", "", "", -20, false},
		{"private", "", "", -20, false},
		{"coffee", "", "", -20, false},
		{"fee", "", "", 20, false},
		{"fee", "", "", 0, false},
		{"fee", "", "reversal of a failed refund", -20, false},
		{"fee", "sale", "", -20, false},
		{"fee", "", "disbursement", -20, false},
		{"fee", "", "recoupment", -20, false},
		{"fee", "", "chargeback dispute", -20, false},
		{"miscellaneous debit", "", "", -20, false},
	} {
		t.Run(tc.typ+"/"+tc.ref+"/"+tc.desc, func(t *testing.T) {
			if got := shopxLedgerIsFee(tc.typ, tc.ref, tc.desc, tc.amount); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}
