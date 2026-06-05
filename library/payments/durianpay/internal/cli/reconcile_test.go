// Copyright 2026 ardihanan and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/durianpay/internal/store"
)

// TestReconcileSQLPath inserts synthetic order/payment rows into a real
// SQLite store and exercises the json_extract/COALESCE/window query path.
func TestReconcileSQLPath(t *testing.T) {
	db, err := store.OpenWithContext(context.Background(), filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()

	now := time.Now().UTC()
	insert := func(rt, id string, obj map[string]any) {
		raw, _ := json.Marshal(obj)
		if err := db.Upsert(rt, id, raw); err != nil {
			t.Fatalf("upsert %s/%s: %v", rt, id, err)
		}
	}
	// In-window paid order with a matching payment.
	insert("orders", "ord_1", map[string]any{"id": "ord_1", "status": "paid", "amount": "100", "created_at": now.Format(time.RFC3339)})
	insert("payments", "pay_1", map[string]any{"id": "pay_1", "order_id": "ord_1", "status": "completed", "amount": "100", "created": now.Format(time.RFC3339)})
	// In-window paid order with NO payment.
	insert("orders", "ord_2", map[string]any{"id": "ord_2", "status": "completed", "amount": "50", "created_at": now.Format(time.RFC3339)})
	// Out-of-window order (created 100 days ago) should be filtered out.
	insert("orders", "ord_old", map[string]any{"id": "ord_old", "status": "paid", "amount": "9", "created_at": now.Add(-100 * 24 * time.Hour).Format(time.RFC3339)})

	cutoff := now.Add(-7 * 24 * time.Hour)
	orders, err := queryLocalOrders(db, cutoff)
	if err != nil {
		t.Fatalf("queryLocalOrders: %v", err)
	}
	if len(orders) != 2 {
		t.Fatalf("queryLocalOrders returned %d orders, want 2 (old one filtered)", len(orders))
	}
	payments, err := queryLocalPayments(db, cutoff)
	if err != nil {
		t.Fatalf("queryLocalPayments: %v", err)
	}
	if len(payments) != 1 {
		t.Fatalf("queryLocalPayments returned %d, want 1", len(payments))
	}

	res := classifyReconcile(orders, payments, "7d")
	if res.MatchedCount != 1 {
		t.Errorf("matched = %d, want 1", res.MatchedCount)
	}
	if len(res.OrdersWithoutPayment) != 1 || res.OrdersWithoutPayment[0].ID != "ord_2" {
		t.Errorf("orders_without_payment = %+v, want [ord_2]", res.OrdersWithoutPayment)
	}
}

func TestClassifyReconcile(t *testing.T) {
	orders := []localOrder{
		{ID: "ord_match", Status: "paid", Amount: 100},
		{ID: "ord_mismatch", Status: "completed", Amount: 200},
		{ID: "ord_unpaid_no_pay", Status: "paid", Amount: 50}, // paid but no payment -> flagged
		{ID: "ord_pending", Status: "created", Amount: 10},    // not paid, no payment -> ignored
	}
	payments := []localPayment{
		{ID: "pay_match", OrderID: "ord_match", Status: "completed", Amount: 100},
		{ID: "pay_mismatch", OrderID: "ord_mismatch", Status: "completed", Amount: 199},
		{ID: "pay_orphan", OrderID: "ord_missing", Status: "completed", Amount: 5}, // order absent
		{ID: "pay_no_order", OrderID: "", Status: "completed", Amount: 7},          // no order_id
	}

	res := classifyReconcile(orders, payments, "7d")

	if res.MatchedCount != 2 {
		t.Errorf("matched_count = %d, want 2", res.MatchedCount)
	}
	if len(res.OrdersWithoutPayment) != 1 || res.OrdersWithoutPayment[0].ID != "ord_unpaid_no_pay" {
		t.Errorf("orders_without_payment = %+v, want [ord_unpaid_no_pay]", res.OrdersWithoutPayment)
	}
	if len(res.PaymentsWithoutOrder) != 2 {
		t.Errorf("payments_without_order = %d, want 2 (orphan + no_order)", len(res.PaymentsWithoutOrder))
	}
	if len(res.AmountMismatches) != 1 || res.AmountMismatches[0].OrderID != "ord_mismatch" {
		t.Errorf("amount_mismatches = %+v, want [ord_mismatch]", res.AmountMismatches)
	}
	if res.ScannedOrders != 4 || res.ScannedPayments != 4 {
		t.Errorf("scanned = %d/%d, want 4/4", res.ScannedOrders, res.ScannedPayments)
	}
	if res.Window != "7d" {
		t.Errorf("window = %q, want 7d", res.Window)
	}
}

func TestClassifyReconcileEmptySlicesNotNil(t *testing.T) {
	res := classifyReconcile(nil, nil, "7d")
	if res.OrdersWithoutPayment == nil || res.PaymentsWithoutOrder == nil || res.AmountMismatches == nil {
		t.Fatalf("empty result slices must be non-nil so they marshal as []")
	}
}

func TestOrderLooksPaid(t *testing.T) {
	for _, s := range []string{"paid", "Completed", "SETTLED", "captured", "success"} {
		if !orderLooksPaid(s) {
			t.Errorf("orderLooksPaid(%q) = false, want true", s)
		}
	}
	for _, s := range []string{"created", "pending", "", "failed"} {
		if orderLooksPaid(s) {
			t.Errorf("orderLooksPaid(%q) = true, want false", s)
		}
	}
}
