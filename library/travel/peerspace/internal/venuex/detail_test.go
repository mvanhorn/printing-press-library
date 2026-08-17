// Copyright 2026 nspage and contributors. Licensed under Apache-2.0. See LICENSE.

package venuex

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseListingDetailStudioFixture(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "listing_detail_studio.json"))
	if err != nil {
		t.Fatal(err)
	}
	l, ok := ParseListing(raw)
	if !ok {
		t.Fatal("ParseListing failed on studio fixture")
	}
	if l.ID != "68d468bb44492187e415d4a6" {
		t.Fatalf("id = %q", l.ID)
	}
	if l.PriceHourly != 85 {
		t.Fatalf("price = %v want 85", l.PriceHourly)
	}
	if l.Currency != "EUR" {
		t.Fatalf("currency = %q", l.Currency)
	}
	if l.Rules == "" {
		t.Fatal("expected rules")
	}
	if l.Parking == "" {
		t.Fatal("expected parking description")
	}
	if l.ParkingAvail == nil || !*l.ParkingAvail {
		t.Fatal("expected parking available")
	}
	if !l.Hydrated {
		t.Fatal("expected hydrated")
	}
	if l.SpaceID == "" {
		t.Fatal("expected space_id / parentSpaceId")
	}
	if l.FormatFit != "production" && l.FormatFit != "mixed" {
		// video_studio + photo keywords
		t.Fatalf("format_fit = %q want production|mixed", l.FormatFit)
	}
}

func TestParseListingDetailYogaFixture(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "listing_detail_yoga.json"))
	if err != nil {
		t.Fatal(err)
	}
	l, ok := ParseListing(raw)
	if !ok {
		t.Fatal("ParseListing failed on yoga fixture")
	}
	if l.SpaceType != "yoga_studio" {
		t.Fatalf("space_type = %q", l.SpaceType)
	}
	if l.FormatFit != "wellness" && l.FormatFit != "mixed" {
		t.Fatalf("format_fit = %q want wellness|mixed", l.FormatFit)
	}
	if l.About == "" && l.Description == "" {
		t.Fatal("expected about/description")
	}
}

func TestInferFormatFitTalkSignals(t *testing.T) {
	l := Listing{
		SpaceType:   "meeting_room",
		Description: "projector and wifi for presentations",
		Amenities:   []string{"projector", "wifi", "chairs"},
	}
	if got := InferFormatFit(l); got != "talk" {
		t.Fatalf("got %q want talk", got)
	}
}

func TestExportMarkdownIncludesSections(t *testing.T) {
	md := ExportMarkdown([]Listing{{
		ID: "1", Title: "Loft", City: "Paris", PriceHourly: 100, Guests: 40,
		SpaceType: "meeting_room", FormatFit: "talk",
		Amenities: []string{"wifi"}, About: "Great for meetups", Rules: "No smoking", Parking: "Street",
	}})
	for _, want := range []string{"About", "Host rules", "Parking", "Format fit", "meeting_room"} {
		if !contains(md, want) {
			t.Fatalf("markdown missing %q:\n%s", want, md)
		}
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		(func() bool {
			for i := 0; i+len(sub) <= len(s); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		})())
}
