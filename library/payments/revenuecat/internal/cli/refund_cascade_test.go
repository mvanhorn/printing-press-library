// Copyright 2026 Joseph Alvin Castillo and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestTraceFromSubscription(t *testing.T) {
	view := refundCascadeView{
		ProjectID:        "proj1",
		SubscriptionID:   "sub1",
		EntitlementsLost: []refundEntitlementLoss{},
	}
	sub := subscriptionDetail{
		ID:           "sub1",
		CustomerID:   "cust1",
		Status:       "active",
		GivesAccess:  true,
		TotalRevenue: map[string]any{"currency": "USD", "gross": 59.99},
		ProductID:    "prodA",
	}
	sub.Entitlements.Items = []struct {
		ID          string `json:"id"`
		DisplayName string `json:"display_name"`
		LookupKey   string `json:"lookup_key"`
	}{
		{ID: "ent_premium", DisplayName: "Premium", LookupKey: "premium"},
		{ID: "ent_pro", DisplayName: "Pro", LookupKey: "pro"},
	}

	traceFromSubscription(&view, sub)

	if view.CustomerID != "cust1" || view.Status != "active" || !view.GivesAccess {
		t.Fatalf("unexpected view scalars: %+v", view)
	}
	if view.TotalRevenueUSD != 59.99 {
		t.Fatalf("revenue = %v, want 59.99", view.TotalRevenueUSD)
	}
	if len(view.EntitlementsLost) != 2 {
		t.Fatalf("entitlements lost = %d, want 2", len(view.EntitlementsLost))
	}
	if view.EntitlementsLost[0].EntitlementID != "ent_premium" {
		t.Fatalf("first entitlement = %q, want ent_premium", view.EntitlementsLost[0].EntitlementID)
	}
}

func TestTraceFromSubscriptionNoEntitlements(t *testing.T) {
	view := refundCascadeView{EntitlementsLost: []refundEntitlementLoss{}}
	traceFromSubscription(&view, subscriptionDetail{ID: "s", Status: "expired"})
	if len(view.EntitlementsLost) != 0 {
		t.Fatalf("expected no entitlements, got %d", len(view.EntitlementsLost))
	}
	if view.TotalRevenueUSD != 0 {
		t.Fatalf("expected zero revenue, got %v", view.TotalRevenueUSD)
	}
}
