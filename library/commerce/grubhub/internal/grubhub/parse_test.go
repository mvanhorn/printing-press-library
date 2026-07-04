// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

package grubhub

import "testing"

func TestParseSearch(t *testing.T) {
	raw := []byte(`{"search_result":{"stats":{"total_results":42},"results":[
		{"restaurant_id":"1","name":"Sweetgreen","cuisines":["Salad"],"delivery_fee":{"price":0},"delivery_minimum":{"price":1200},"delivery_time_estimate":13,"distance_from_location":"0.22","ratings":{"actual_rating_value":4.8},"open":true,"coupons_available":false,"available_offers":[]},
		{"restaurant_id":"2","name":"NAYA","cuisines":["Mediterranean"],"delivery_fee":{"price":99},"delivery_minimum":{"price":0},"delivery_time_estimate":10,"distance_from_location":"0.04","ratings":{"rating_value":"4"},"open":true,"coupons_available":true,"available_offers":[{"title":"$5 off $15","amount":{"value":500,"order_minimum":1500}}]}
	]}}`)
	cards, total, err := ParseSearch(raw)
	if err != nil {
		t.Fatalf("ParseSearch error: %v", err)
	}
	if total != 42 {
		t.Errorf("total = %d, want 42", total)
	}
	if len(cards) != 2 {
		t.Fatalf("got %d cards, want 2", len(cards))
	}
	if cards[0].Name != "Sweetgreen" || cards[0].DeliveryFee.Price != 0 {
		t.Errorf("card0 = %+v", cards[0])
	}
	if got := cards[0].Rating(); got != 4.8 {
		t.Errorf("card0 rating = %v, want 4.8", got)
	}
	if got := cards[1].Rating(); got != 4 {
		t.Errorf("card1 rating (from string) = %v, want 4", got)
	}
	if got := cards[1].DealCount(); got != 2 { // 1 offer + coupons_available
		t.Errorf("card1 deal count = %d, want 2", got)
	}
	if got := cards[0].DistanceMiles(); got != 0.22 {
		t.Errorf("card0 distance = %v, want 0.22", got)
	}
}

func TestParseGeocode(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		raw := []byte(`[{"latitude":"40.7485","longitude":"-73.9857","address_locality":"New York","address_region":"NY","postal_code":"10001"}]`)
		c, err := ParseGeocode(raw)
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if c.Lat != 40.7485 || c.Lng != -73.9857 || c.Locality != "New York" {
			t.Errorf("coords = %+v", c)
		}
	})
	t.Run("empty", func(t *testing.T) {
		if _, err := ParseGeocode([]byte(`[]`)); err == nil {
			t.Error("expected error for empty geocode result")
		}
	})
	t.Run("zero coords", func(t *testing.T) {
		if _, err := ParseGeocode([]byte(`[{"latitude":"0","longitude":"0"}]`)); err == nil {
			t.Error("expected error for zero coordinates")
		}
	})
}

func TestParseMenu(t *testing.T) {
	raw := []byte(`{"restaurant":{"name":"Sweetgreen","menu_category_list":[
		{"name":"Salads","menu_item_list":[
			{"id":"a","name":"Caesar","price":{"amount":1200},"popular":true},
			{"id":"b","name":"Build Your Own","price":{"amount":0},"minimum_price_variation":{"amount":1535}}
		]}
	]}}`)
	name, items, err := ParseMenu(raw)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if name != "Sweetgreen" {
		t.Errorf("name = %q", name)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	if items[0].Category != "Salads" {
		t.Errorf("category = %q, want Salads", items[0].Category)
	}
	if got := items[0].PriceCents(); got != 1200 {
		t.Errorf("item0 price = %d, want 1200", got)
	}
	if got := items[1].PriceCents(); got != 1535 { // falls back to min_price_variation
		t.Errorf("build-your-own price = %d, want 1535 (variation fallback)", got)
	}
}

func TestParseItem(t *testing.T) {
	raw := []byte(`{"id":"x","name":"Bowl","price":{"amount":1570},"choice_category_list":[
		{"name":"Bases","min_choice_options":1,"max_choice_options":1,"choice_option_list":[
			{"description":"Romaine","price":{"amount":0}},
			{"description":"Spinach","price":{"amount":50}}
		]}
	]}`)
	item, err := ParseItem(raw)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if item.Name != "Bowl" || item.Price.Amount != 1570 {
		t.Errorf("item = %+v", item)
	}
	if len(item.ChoiceCategories) != 1 || len(item.ChoiceCategories[0].Options) != 2 {
		t.Fatalf("choices = %+v", item.ChoiceCategories)
	}
	if item.ChoiceCategories[0].Options[1].Price.Amount != 50 {
		t.Errorf("option price = %d, want 50", item.ChoiceCategories[0].Options[1].Price.Amount)
	}
}

func TestDollars(t *testing.T) {
	cases := map[int]string{0: "$0.00", 1200: "$12.00", 1570: "$15.70", 99: "$0.99"}
	for cents, want := range cases {
		if got := Dollars(cents); got != want {
			t.Errorf("Dollars(%d) = %q, want %q", cents, got, want)
		}
	}
}

func TestFormatPoint(t *testing.T) {
	if got := FormatPoint(-73.9857, 40.7484); got != "POINT(-73.9857 40.7484)" {
		t.Errorf("FormatPoint = %q (longitude must come first)", got)
	}
}
