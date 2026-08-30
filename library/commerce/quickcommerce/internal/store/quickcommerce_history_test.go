// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"
)

func TestEnsureQuickCommerceHistoryCreatesSchema(t *testing.T) {
	for _, tc := range []struct{ name string }{{"fresh"}, {"repeat"}} {
		t.Run(tc.name, func(t *testing.T) {
			db, err := OpenWithContext(context.Background(), filepath.Join(t.TempDir(), "data.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			if err := EnsureQuickCommerceHistory(context.Background(), db); err != nil {
				t.Fatal(err)
			}
			if err := EnsureQuickCommerceHistory(context.Background(), db); err != nil {
				t.Fatal(err)
			}
			var count int
			if err := db.DB().QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='quickcommerce_observations'`).Scan(&count); err != nil {
				t.Fatal(err)
			}
			if count != 1 {
				t.Fatalf("schema count = %d, want 1", count)
			}
		})
	}
}

func TestInsertQuickCommerceObservationPersistsFields(t *testing.T) {
	db, err := OpenWithContext(context.Background(), filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	want := QuickCommerceObservation{ID: "snapshot-1", Resource: "products", ItemID: "501346", Platform: "BlinkIt", Location: "12.902100,77.663900", Query: "milk", CapturedAt: time.Now().UTC().Truncate(time.Second), Data: json.RawMessage(`{"name":"Milk","offer_price":65,"quantity":"1 L"}`), Quantity: "1 L"}
	if err := InsertQuickCommerceObservation(context.Background(), db, want); err != nil {
		t.Fatal(err)
	}
	var itemID, platform, quantity string
	if err := db.DB().QueryRow(`SELECT item_id,platform,quantity FROM quickcommerce_observations WHERE id=?`, want.ID).Scan(&itemID, &platform, &quantity); err != nil {
		t.Fatal(err)
	}
	if itemID != want.ItemID || platform != want.Platform || quantity != want.Quantity {
		t.Fatalf("persisted fields = %q %q %q", itemID, platform, quantity)
	}
}
