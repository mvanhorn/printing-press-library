// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil/testenv"
)

// TestNovelMoneyReconcileHelpWires smoke-tests that the money reconcile command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelMoneyReconcileHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"money", "reconcile", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("money reconcile --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "reconcile"} {
		if !strings.Contains(help, want) {
			t.Fatalf("money reconcile --help missing %q in output:\n%s", want, help)
		}
	}
}

func shopxReconcileArray(t *testing.T, obj map[string]any, key string) []any {
	t.Helper()
	rows, ok := obj[key].([]any)
	if !ok {
		t.Fatalf("%s is not an array: %#v", key, obj[key])
	}
	return rows
}

func TestShopxMoneyReconcileMismatches(t *testing.T) {
	f := shopxNewFixture(t)
	for _, v := range []struct {
		id     int64
		paid   bool
		days   int64
		amount int64
	}{{55, true, 10, 4200}, {56, true, 9, 1500}, {57, false, 8, 100}, {58, true, 90, 200}} {
		f.put("receipts", map[string]any{"receipt_id": v.id, "is_paid": v.paid, "created_timestamp": shopxAgo(time.Duration(v.days) * shopxDay), "grandtotal": shopxMoneyObj(v.amount, "USD"), "name": "Jane Synthetic", "buyer_email": "jane@example.invalid", "first_line": "123 Synthetic Lane"})
	}
	f.put("payments", map[string]any{"payment_id": 3, "receipt_id": 55, "created_timestamp": shopxAgo(10 * shopxDay), "amount_gross": shopxMoneyObj(4200, "USD")}, map[string]any{"payment_id": 4, "receipt_id": 999, "create_timestamp": shopxAgo(shopxDay), "amount_gross": shopxMoneyObj(800, "USD")})
	for i, v := range []struct {
		amount, balance int64
		ref             string
	}{{4200, 4200, "55"}, {-210, 3990, "55"}, {-15, 3975, "55"}, {100, 4075, "777"}} {
		f.put("ledger_entries", map[string]any{"entry_id": i + 1, "ledger_id": 1, "sequence_number": i + 1, "amount": v.amount, "balance": v.balance, "currency": "USD", "created_timestamp": shopxAgo(shopxDay), "reference_type": " Receipt ", "reference_id": v.ref})
	}
	out, _, err := shopxRun(t, f, "money", "reconcile")
	if err != nil {
		t.Fatal(err)
	}
	obj := shopxMustObject(t, out)
	if obj["since"] != "30d" || obj["mismatch_count"] != float64(3) {
		t.Fatalf("unexpected result %s", out)
	}
	for key, want := range map[string]map[string]any{
		"receipts_without_payment": {"receipt_id": float64(56), "amount_minor": float64(1500), "currency": "USD"},
		"payments_without_receipt": {"payment_id": float64(4), "receipt_id": float64(999), "amount_minor": float64(800), "currency": "USD"},
		"ledger_missing_reference": {"entry_id": float64(4), "reference_id": "777", "amount_minor": float64(100), "currency": "USD"},
	} {
		rows := shopxReconcileArray(t, obj, key)
		if len(rows) != 1 {
			t.Fatalf("%s: %#v", key, rows)
		}
		row := rows[0].(map[string]any)
		for k, v := range want {
			if row[k] != v {
				t.Fatalf("%s %s: got %#v want %#v", key, k, row[k], v)
			}
		}
	}
	if len(shopxReconcileArray(t, obj, "balance_discontinuities")) != 0 {
		t.Fatal(out)
	}
	for _, pii := range []string{"Jane Synthetic", "jane@example.invalid", "123 Synthetic Lane"} {
		if strings.Contains(out, pii) {
			t.Fatalf("PII leaked: %s", pii)
		}
	}
}

func shopxAssertReconcileClean(t *testing.T, out string) {
	t.Helper()
	obj := shopxMustObject(t, out)
	if obj["mismatch_count"] != float64(0) {
		t.Fatal(out)
	}
	for _, key := range []string{"receipts_without_payment", "payments_without_receipt", "ledger_missing_reference", "balance_discontinuities"} {
		if len(shopxReconcileArray(t, obj, key)) != 0 {
			t.Fatalf("%s not empty: %s", key, out)
		}
	}
	shopxMustEnvelope(t, out)
}

func TestShopxMoneyReconcileEmpty(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(fmt.Sprint(existing), func(t *testing.T) {
			var f *shopxFixture
			if existing {
				f = shopxNewFixture(t)
			}
			out, _, err := shopxRun(t, f, "money", "reconcile")
			if err != nil {
				t.Fatal(err)
			}
			shopxAssertReconcileClean(t, out)
		})
	}
}

func TestShopxMoneyReconcileClean(t *testing.T) {
	f := shopxNewFixture(t)
	f.put("receipts", map[string]any{"receipt_id": 55, "is_paid": true, "created_timestamp": shopxAgo(90 * shopxDay), "grandtotal": shopxMoneyObj(4200, "USD")})
	f.put("payments", map[string]any{"payment_id": 3, "receipt_id": 55, "created_timestamp": shopxAgo(shopxDay), "amount_gross": shopxMoneyObj(4200, "USD")})
	for i, v := range []int64{4200, 3990, 3975} {
		amount := int64(-15)
		if i == 0 {
			amount = 4200
		}
		if i == 1 {
			amount = -210
		}
		f.put("ledger_entries", map[string]any{"entry_id": i + 1, "ledger_id": 1, "sequence_number": i + 1, "amount": amount, "balance": v, "currency": "USD", "reference_type": "receipt", "reference_id": "55", "created_timestamp": shopxAgo(shopxDay)})
	}
	out, _, err := shopxRun(t, f, "money", "reconcile")
	if err != nil {
		t.Fatal(err)
	}
	shopxAssertReconcileClean(t, out)
}

func TestShopxMoneyReconcileBalanceGap(t *testing.T) {
	f := shopxNewFixture(t)
	for i, v := range []int64{100, 200, 350, 450} {
		f.put("ledger_entries", map[string]any{"entry_id": i + 1, "ledger_id": 1, "sequence_number": i + 1, "amount": 100, "balance": v, "currency": "USD", "created_timestamp": shopxAgo(time.Duration(40-i*10) * shopxDay)})
	}
	out, _, err := shopxRun(t, f, "money", "reconcile")
	if err != nil {
		t.Fatal(err)
	}
	obj := shopxMustObject(t, out)
	rows := shopxReconcileArray(t, obj, "balance_discontinuities")
	if len(rows) != 1 || obj["mismatch_count"] != float64(1) {
		t.Fatal(out)
	}
	row := rows[0].(map[string]any)
	for k, want := range map[string]any{"entry_id": float64(3), "ledger_id": float64(1), "sequence_number": float64(3), "expected_balance_minor": float64(300), "actual_balance_minor": float64(350), "gap_minor": float64(50), "currency": "USD"} {
		if row[k] != want {
			t.Fatalf("%s: %s", k, out)
		}
	}
}

func TestShopxMoneyReconcileNotes(t *testing.T) {
	f := shopxNewFixture(t)
	f.put("receipts", map[string]any{"receipt_id": 56, "was_paid": true, "created_timestamp": shopxAgo(shopxDay), "grandtotal": shopxMoneyObj(1500, "USD")})
	out, stderr, err := shopxRun(t, f, "money", "reconcile")
	if err != nil {
		t.Fatal(err)
	}
	shopxAssertReconcileClean(t, out)
	note := "payments not synced; receipts_without_payment skipped"
	if !strings.Contains(out, note) || !strings.Contains(stderr, note) {
		t.Fatalf("missing note: %s / %s", out, stderr)
	}
}

func TestShopxMoneyReconcileTimestampAndReferenceTargets(t *testing.T) {
	f := shopxNewFixture(t)
	f.put("receipts", map[string]any{"receipt_id": 55, "was_paid": true, "created_timestamp": shopxAgo(shopxDay)}, map[string]any{"receipt_id": 56, "is_paid": true})
	p := map[string]any{"payment_id": 3, "receipt_id": 55, "created_timestamp": shopxAgo(90 * shopxDay)}
	f.put("payments", p)
	f.put("shops_receipts_payments", p)
	f.put("transactions", map[string]any{"transaction_id": 8, "listing_id": 1000000001})
	f.put("listings", map[string]any{"listing_id": 1000000001, "title": "Linen apron"})
	for i, v := range []struct {
		kind, id string
		stamp    any
	}{{"PAYMENT", "3", shopxAgo(shopxDay)}, {"transaction", "8", shopxAgo(shopxDay)}, {"listing", "1000000001", shopxAgo(shopxDay)}, {"other", "999", shopxAgo(shopxDay)}, {"payment", "999", shopxAgo(shopxDay)}, {"receipt", "999", nil}, {"receipt", "999", shopxAgo(90 * shopxDay)}} {
		f.put("ledger_entries", map[string]any{"entry_id": i + 1, "reference_type": v.kind, "reference_id": v.id, "created_timestamp": v.stamp})
	}
	out, _, err := shopxRun(t, f, "money", "reconcile")
	if err != nil {
		t.Fatal(err)
	}
	obj := shopxMustObject(t, out)
	rows := shopxReconcileArray(t, obj, "ledger_missing_reference")
	if len(rows) != 1 || rows[0].(map[string]any)["entry_id"] != float64(5) || obj["mismatch_count"] != float64(1) {
		t.Fatal(out)
	}
}

func TestShopxMoneyReconcileInvalidWindow(t *testing.T) {
	_, _, err := shopxRun(t, nil, "money", "reconcile", "--since", "bad")
	if err == nil {
		t.Fatal("invalid window accepted")
	}
}
