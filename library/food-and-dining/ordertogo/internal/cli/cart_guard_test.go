// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"testing"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/ordertogo/internal/store"
)

func TestValidateCartForPlacement(t *testing.T) {
	priced := []store.OrderItem{{ID: "19001", Price: 4.99, Quantity: 1}}
	cases := []struct {
		name    string
		restID  string
		items   []store.OrderItem
		wantErr bool
	}{
		{"valid", "72", priced, false},
		{"slug restid", "mixsushibarlin", priced, true},
		{"zero restid", "0", priced, true},
		{"empty restid", "", priced, true},
		{"no priced items", "72", []store.OrderItem{{ID: "19001", Price: 0, Quantity: 1}}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateCartForPlacement(tc.restID, tc.items)
			if (err != nil) != tc.wantErr {
				t.Fatalf("restID=%q items=%v: got err=%v, wantErr=%v", tc.restID, tc.items, err, tc.wantErr)
			}
		})
	}
}

func TestValidatePlaceCart(t *testing.T) {
	priced := []cartItem{{ItemID: 19001, Price: 4.99}}
	if err := validatePlaceCart("mixsushibarlin", 72, priced); err != nil {
		t.Fatalf("valid cart rejected: %v", err)
	}
	if err := validatePlaceCart("mixsushibarlin", 0, priced); err == nil {
		t.Fatal("zero restid should be rejected")
	}
	if err := validatePlaceCart("", 72, priced); err == nil {
		t.Fatal("empty restaurant slug should be rejected")
	}
	if err := validatePlaceCart("mixsushibarlin", 72, []cartItem{{ItemID: 19001, Price: 0}}); err == nil {
		t.Fatal("all-zero-price cart should be rejected")
	}
}
