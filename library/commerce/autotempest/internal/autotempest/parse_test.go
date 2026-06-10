// Copyright 2026 richardadonnell and contributors. Licensed under Apache-2.0. See LICENSE.

package autotempest

import (
	"encoding/json"
	"testing"
)

func TestParsePriceCents(t *testing.T) {
	cases := []struct {
		in   string
		want int64
	}{
		{"$30,497", 3049700},
		{"$30,497.50", 3049750},
		{"$1,200", 120000},
		{"7500", 750000},
		{"", -1},
		{"N/A", -1},
		{"$0", 0},
		{"Call", -1},
	}
	for _, c := range cases {
		if got := ParsePriceCents(c.in); got != c.want {
			t.Errorf("ParsePriceCents(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestParseMileage(t *testing.T) {
	cases := []struct {
		in   string
		want int64
	}{
		{"24,755", 24755},
		{"100000", 100000},
		{"", -1},
		{"unknown", -1},
		{"0", 0},
	}
	for _, c := range cases {
		if got := ParseMileage(c.in); got != c.want {
			t.Errorf("ParseMileage(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestParseYear(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"2016", 2016},
		{"", 0},
		{"abc", 0},
	}
	for _, c := range cases {
		if got := ParseYear(c.in); got != c.want {
			t.Errorf("ParseYear(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestParseListing(t *testing.T) {
	raw := json.RawMessage(`{
		"id":"ll-2HGFE1E51RH470912",
		"vin":"2HGFE1E51RH470912",
		"title":"2016 Honda Civic EX",
		"make":"honda","model":"civic","year":"2016","trim":"EX",
		"price":"$18,995","mileage":"42,100",
		"location":"Tampa, FL","locationCode":"33601","countryCode":"US",
		"distance":12.4,"dealerName":"Test Motors","sellerType":"Dealer",
		"url":"https://example.com/x","img":"https://example.com/i.jpg",
		"sitecode":"ll","vehicleTitle":"Clean","listingType":"regular",
		"currentBid":null,"bids":null
	}`)
	l, ok := ParseListing(raw, "cs")
	if !ok {
		t.Fatal("ParseListing returned ok=false")
	}
	if l.ID != "ll-2HGFE1E51RH470912" {
		t.Errorf("ID = %q", l.ID)
	}
	if l.Year != 2016 {
		t.Errorf("Year = %d, want 2016", l.Year)
	}
	if l.PriceCents != 1899500 {
		t.Errorf("PriceCents = %d, want 1899500", l.PriceCents)
	}
	if l.Mileage != 42100 {
		t.Errorf("Mileage = %d, want 42100", l.Mileage)
	}
	if l.Source != "cs" {
		t.Errorf("Source = %q, want cs", l.Source)
	}
	if l.Sitecode != "ll" {
		t.Errorf("Sitecode = %q, want ll", l.Sitecode)
	}
	if l.CurrentBid != -1 {
		t.Errorf("CurrentBid = %d, want -1", l.CurrentBid)
	}
}

func TestParseListingNoID(t *testing.T) {
	if _, ok := ParseListing(json.RawMessage(`{"title":"x"}`), "cs"); ok {
		t.Error("expected ok=false for listing with no id")
	}
}

func TestParseListingAuctionBid(t *testing.T) {
	raw := json.RawMessage(`{"id":"eb-1","title":"x","listingType":"auction","currentBid":"$5,250","bids":14}`)
	l, ok := ParseListing(raw, "eb")
	if !ok {
		t.Fatal("ok=false")
	}
	if l.CurrentBid != 525000 {
		t.Errorf("CurrentBid = %d, want 525000", l.CurrentBid)
	}
	if l.Bids != 14 {
		t.Errorf("Bids = %d, want 14", l.Bids)
	}
}

func TestFormatCents(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{3049700, "$30,497"},
		{120000, "$1,200"},
		{99900, "$999"},
		{100000, "$1,000"},
		{0, "$0"},
		{123456700, "$1,234,567"},
		{-1, ""},
		{-100, ""},
	}
	for _, c := range cases {
		if got := FormatCents(c.in); got != c.want {
			t.Errorf("FormatCents(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}
