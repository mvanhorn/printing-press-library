// Copyright 2026 Joseph Alvin Castillo and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/revenuecat/internal/store"
)

func putActiveEntitlement(t *testing.T, db *store.Store, id, customerID, entitlementID string, expiresAt int64) {
	t.Helper()
	obj := map[string]any{
		"id":             id,
		"customers_id":   customerID,
		"customer_id":    customerID,
		"entitlement_id": entitlementID,
	}
	if expiresAt > 0 {
		obj["expires_at"] = expiresAt
	}
	blob, _ := json.Marshal(obj)
	if err := db.UpsertActiveEntitlements(blob); err != nil {
		t.Fatalf("upsert active entitlement %s: %v", id, err)
	}
}

func TestBuildEntitlementRollup(t *testing.T) {
	db := newNovelTestStore(t)
	now := time.Now().UTC()

	// Project entitlements.
	putResource(t, db, "entitlements", "ent_premium", map[string]any{
		"id": "ent_premium", "display_name": "Premium", "state": "active", "lookup_key": "premium",
	})
	putResource(t, db, "entitlements", "ent_pro", map[string]any{
		"id": "ent_pro", "display_name": "Pro", "state": "active", "lookup_key": "pro",
	})

	// Active grants: 2 customers on premium, 1 expired grant (excluded).
	putActiveEntitlement(t, db, "ae1", "cust1", "ent_premium", now.Add(24*time.Hour).UnixMilli())
	putActiveEntitlement(t, db, "ae2", "cust2", "ent_premium", 0) // no expiry
	putActiveEntitlement(t, db, "ae3", "cust3", "ent_premium", now.Add(-24*time.Hour).UnixMilli())

	// Subscriptions: cust1 active (no disagreement); cust2 all expired
	// (disagreement: holds active entitlement but no access-granting sub).
	putResource(t, db, "subscriptions", "s1", map[string]any{
		"id": "s1", "customer_id": "cust1", "status": "active",
	})
	putResource(t, db, "subscriptions", "s2", map[string]any{
		"id": "s2", "customer_id": "cust2", "status": "expired",
	})

	view, err := buildEntitlementRollup(db, "proj1", true)
	if err != nil {
		t.Fatalf("buildEntitlementRollup: %v", err)
	}
	if view.TotalEntitlement != 2 {
		t.Fatalf("total entitlements = %d, want 2", view.TotalEntitlement)
	}
	// premium should be first (2 active customers, expired grant excluded).
	premium := view.Entitlements[0]
	if premium.EntitlementID != "ent_premium" {
		t.Fatalf("first entitlement = %q, want ent_premium", premium.EntitlementID)
	}
	if premium.ActiveCustomers != 2 {
		t.Fatalf("premium active customers = %d, want 2", premium.ActiveCustomers)
	}
	if premium.Disagreements != 1 {
		t.Fatalf("premium disagreements = %d, want 1 (cust2 expired)", premium.Disagreements)
	}
}

func TestSubscriptionsAllNonAccess(t *testing.T) {
	cases := []struct {
		name   string
		states []string
		want   bool
	}{
		{"empty not flagged", nil, false},
		{"all expired", []string{"expired", "in_billing_retry"}, true},
		{"one active blocks", []string{"expired", "active"}, false},
		{"grace counts as access", []string{"in_grace_period"}, false},
		{"single expired", []string{"expired"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := subscriptionsAllNonAccess(tc.states); got != tc.want {
				t.Fatalf("subscriptionsAllNonAccess(%v) = %v, want %v", tc.states, got, tc.want)
			}
		})
	}
}
