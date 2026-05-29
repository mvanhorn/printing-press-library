// Copyright 2026 Joseph Alvin Castillo and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/revenuecat/internal/store"
)

// putInvoice inserts an invoice into both the resources and domain tables.
func putInvoice(t *testing.T, db *store.Store, id, customerID string, paidAt int64, gross float64) {
	t.Helper()
	obj := map[string]any{
		"id":           id,
		"customers_id": customerID, // domain-table linkage
		"customer_id":  customerID,
		"total_amount": map[string]any{"currency": "USD", "gross": gross},
	}
	if paidAt > 0 {
		obj["paid_at"] = paidAt
	}
	blob, _ := json.Marshal(obj)
	if err := db.UpsertInvoices(blob); err != nil {
		t.Fatalf("upsert invoice %s: %v", id, err)
	}
}

func TestBuildDunningAlert(t *testing.T) {
	db := newNovelTestStore(t)
	now := time.Now().UTC()

	// Recoverable subs.
	putResource(t, db, "subscriptions", "sub_grace", map[string]any{
		"id": "sub_grace", "customer_id": "cust1", "status": "in_grace_period",
		"product_id":           "prodA",
		"total_revenue_in_usd": map[string]any{"currency": "USD", "gross": 40.0},
	})
	putResource(t, db, "subscriptions", "sub_retry", map[string]any{
		"id": "sub_retry", "customer_id": "cust2", "status": "in_billing_retry",
		"total_revenue_in_usd": map[string]any{"currency": "USD", "gross": 10.0},
	})
	// Non-recoverable sub: excluded.
	putResource(t, db, "subscriptions", "sub_expired", map[string]any{
		"id": "sub_expired", "customer_id": "cust3", "status": "expired",
		"total_revenue_in_usd": map[string]any{"currency": "USD", "gross": 999.0},
	})

	// cust1 has one unpaid invoice (paid_at absent) of $40, one paid invoice.
	putInvoice(t, db, "inv_unpaid", "cust1", 0, 40.0)
	putInvoice(t, db, "inv_paid", "cust1", now.UnixMilli(), 40.0)

	view, err := buildDunningAlert(db, "proj1")
	if err != nil {
		t.Fatalf("buildDunningAlert: %v", err)
	}
	if view.Count != 2 {
		t.Fatalf("count = %d, want 2 (rows %+v)", view.Count, view.Rows)
	}
	// Top row should be the higher recoverable amount. cust1 has an unpaid
	// invoice of $40, so recoverable = 40; cust2 has no invoices so falls back
	// to its subscription revenue $10.
	if view.Rows[0].SubscriptionID != "sub_grace" {
		t.Fatalf("top row = %q, want sub_grace", view.Rows[0].SubscriptionID)
	}
	if view.Rows[0].RecoverableUSD != 40.0 {
		t.Fatalf("top recoverable = %v, want 40.0", view.Rows[0].RecoverableUSD)
	}
	if view.Rows[0].UnpaidInvoices != 1 {
		t.Fatalf("unpaid invoices = %d, want 1", view.Rows[0].UnpaidInvoices)
	}
}

func TestBuildDunningAlertEmpty(t *testing.T) {
	db := newNovelTestStore(t)
	view, err := buildDunningAlert(db, "proj1")
	if err != nil {
		t.Fatalf("buildDunningAlert: %v", err)
	}
	if view.Count != 0 || view.Note == "" {
		t.Fatalf("expected empty result with note, got count=%d note=%q", view.Count, view.Note)
	}
}
