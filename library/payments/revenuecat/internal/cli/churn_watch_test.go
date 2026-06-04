// Copyright 2026 Joseph Alvin Castillo and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/revenuecat/internal/store"
)

// newNovelTestStore opens a fresh on-disk store in a temp dir for novel-command
// tests. Shared across the novel-command test files in this package.
func newNovelTestStore(t *testing.T) *store.Store {
	t.Helper()
	db, err := store.OpenWithContext(context.Background(), filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// putResource inserts a raw resource row of the given type.
func putResource(t *testing.T, db *store.Store, resourceType, id string, obj map[string]any) {
	t.Helper()
	blob, err := json.Marshal(obj)
	if err != nil {
		t.Fatalf("marshal resource: %v", err)
	}
	if err := db.Upsert(resourceType, id, blob); err != nil {
		t.Fatalf("upsert %s/%s: %v", resourceType, id, err)
	}
}

func epochMS(t time.Time) int64 { return t.UnixMilli() }

func TestBuildChurnWatch(t *testing.T) {
	db := newNovelTestStore(t)
	now := time.Now().UTC()

	// In-window expired sub: should appear.
	putResource(t, db, "subscriptions", "sub_expired", map[string]any{
		"id": "sub_expired", "customer_id": "cust1", "status": "expired",
		"product_id":             "prodA",
		"current_period_ends_at": epochMS(now.Add(-2 * 24 * time.Hour)),
		"total_revenue_in_usd":   map[string]any{"currency": "USD", "gross": 50.0},
	})
	// In-window grace sub: should appear.
	putResource(t, db, "subscriptions", "sub_grace", map[string]any{
		"id": "sub_grace", "customer_id": "cust2", "status": "in_grace_period",
		"current_period_ends_at": epochMS(now.Add(-1 * 24 * time.Hour)),
		"total_revenue_in_usd":   map[string]any{"currency": "USD", "gross": 25.0},
	})
	// Active sub: not churned, excluded.
	putResource(t, db, "subscriptions", "sub_active", map[string]any{
		"id": "sub_active", "customer_id": "cust3", "status": "active",
		"current_period_ends_at": epochMS(now.Add(-1 * 24 * time.Hour)),
		"total_revenue_in_usd":   map[string]any{"currency": "USD", "gross": 99.0},
	})
	// Expired but OUT of window: excluded.
	putResource(t, db, "subscriptions", "sub_old", map[string]any{
		"id": "sub_old", "customer_id": "cust4", "status": "expired",
		"current_period_ends_at": epochMS(now.Add(-90 * 24 * time.Hour)),
		"total_revenue_in_usd":   map[string]any{"currency": "USD", "gross": 10.0},
	})

	view, err := buildChurnWatch(db, "proj1", 30*24*time.Hour, "30d")
	if err != nil {
		t.Fatalf("buildChurnWatch: %v", err)
	}
	if view.Count != 2 {
		t.Fatalf("count = %d, want 2 (rows: %+v)", view.Count, view.Rows)
	}
	if view.DollarExposure != 75.0 {
		t.Fatalf("dollar exposure = %v, want 75.0", view.DollarExposure)
	}
	// Sorted by period-end desc: sub_grace (-1d) before sub_expired (-2d).
	if view.Rows[0].SubscriptionID != "sub_grace" {
		t.Fatalf("first row = %q, want sub_grace", view.Rows[0].SubscriptionID)
	}
}

func TestBuildChurnWatchWillNotRenew(t *testing.T) {
	db := newNovelTestStore(t)
	now := time.Now().UTC()

	// Active subscription the customer cancelled (auto-renew off): this is a
	// voluntary churn and MUST be flagged even though status is "active".
	putResource(t, db, "subscriptions", "sub_cancelled", map[string]any{
		"id": "sub_cancelled", "customer_id": "cust1", "status": "active",
		"auto_renewal_status":    "will_not_renew",
		"current_period_ends_at": epochMS(now.Add(-1 * 24 * time.Hour)),
		"total_revenue_in_usd":   map[string]any{"currency": "USD", "gross": 40.0},
	})
	// Active sub still auto-renewing: excluded.
	putResource(t, db, "subscriptions", "sub_active", map[string]any{
		"id": "sub_active", "customer_id": "cust2", "status": "active",
		"auto_renewal_status":    "will_renew",
		"current_period_ends_at": epochMS(now.Add(-1 * 24 * time.Hour)),
		"total_revenue_in_usd":   map[string]any{"currency": "USD", "gross": 99.0},
	})

	view, err := buildChurnWatch(db, "proj1", 30*24*time.Hour, "30d")
	if err != nil {
		t.Fatalf("buildChurnWatch: %v", err)
	}
	if view.Count != 1 {
		t.Fatalf("count = %d, want 1 (rows: %+v)", view.Count, view.Rows)
	}
	if view.Rows[0].SubscriptionID != "sub_cancelled" {
		t.Fatalf("row = %q, want sub_cancelled", view.Rows[0].SubscriptionID)
	}
	if view.Rows[0].AutoRenewalStatus != "will_not_renew" {
		t.Fatalf("auto_renewal_status = %q, want will_not_renew", view.Rows[0].AutoRenewalStatus)
	}
	if view.DollarExposure != 40.0 {
		t.Fatalf("dollar exposure = %v, want 40.0", view.DollarExposure)
	}
}

func TestBuildChurnWatchEmpty(t *testing.T) {
	db := newNovelTestStore(t)
	view, err := buildChurnWatch(db, "proj1", 30*24*time.Hour, "30d")
	if err != nil {
		t.Fatalf("buildChurnWatch: %v", err)
	}
	if view.Count != 0 {
		t.Fatalf("count = %d, want 0", view.Count)
	}
	if view.Note == "" {
		t.Fatal("expected a note when no churned subs found")
	}
	// Ensure rows marshals as [] not null.
	blob, _ := json.Marshal(view)
	if !strings.Contains(string(blob), `"rows":[]`) {
		t.Fatalf("expected empty rows array in JSON, got %s", blob)
	}
}
