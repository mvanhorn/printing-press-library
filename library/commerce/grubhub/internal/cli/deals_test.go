// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/grubhub/internal/grubhub"
)

func TestDealRowsFromCards(t *testing.T) {
	o1 := grubhub.Offer{Title: "$5 off $15"}
	o1.Amount.Value = 500
	o1.Amount.OrderMinimum = 1500
	o2 := grubhub.Offer{Title: "$20 off $50"}
	o2.Amount.Value = 2000
	o2.Amount.OrderMinimum = 5000

	cards := []grubhub.Card{
		{ID: "1", Name: "NoDeals", AvailableOffers: nil, CouponsAvailable: false},
		{ID: "2", Name: "SmallDeal", AvailableOffers: []grubhub.Offer{o1}},
		{ID: "3", Name: "BigDeal", AvailableOffers: []grubhub.Offer{o1, o2}},
	}
	rows := dealRowsFromCards(cards)
	// NoDeals is excluded (DealCount 0).
	if len(rows) != 2 {
		t.Fatalf("got %d deal rows, want 2 (NoDeals excluded)", len(rows))
	}
	sortDeals(rows, "value")
	if rows[0].RestaurantName != "BigDeal" {
		t.Errorf("value sort top = %s, want BigDeal", rows[0].RestaurantName)
	}
	if rows[0].BestOffer != "$20 off $50" || rows[0].BestValueCents != 2000 {
		t.Errorf("best offer = %q (%d cents)", rows[0].BestOffer, rows[0].BestValueCents)
	}
	if rows[0].OrderMinimum != "$50.00" {
		t.Errorf("order minimum = %q, want $50.00", rows[0].OrderMinimum)
	}
}

func TestDealsSortByCount(t *testing.T) {
	o := grubhub.Offer{Title: "x"}
	o.Amount.Value = 100
	rows := []dealRow{
		{RestaurantName: "One", Offers: []string{"a"}, BestValueCents: 900},
		{RestaurantName: "Three", Offers: []string{"a", "b", "c"}, BestValueCents: 100},
	}
	sortDeals(rows, "count")
	if rows[0].RestaurantName != "Three" {
		t.Errorf("count sort top = %s, want Three", rows[0].RestaurantName)
	}
}
