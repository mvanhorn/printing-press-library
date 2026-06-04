// Copyright 2026 kothari-nikunj and contributors. Licensed under Apache-2.0. See LICENSE.

package parser

import (
	"testing"
)

// TestExtractInitDataBlobs verifies we can find the bracketed JSON payload
// inside an AF_initDataCallback envelope without being defeated by quoted
// brackets inside strings.
func TestExtractInitDataBlobs(t *testing.T) {
	html := []byte(`<script>AF_initDataCallback({key: 'ds:0', hash: 'x', data:[[1,"["],[2,"]"]] , sideChannel:{}})</script>`)
	blobs := ExtractInitDataBlobs(html)
	if len(blobs) != 1 {
		t.Fatalf("expected 1 blob, got %d", len(blobs))
	}
	got, ok := blobs["ds:0"]
	if !ok {
		t.Fatalf("missing ds:0")
	}
	want := `[[1,"["],[2,"]"]]`
	if string(got) != want {
		t.Fatalf("blob mismatch:\n got=%s\nwant=%s", string(got), want)
	}
}

// TestTryParseHotelHappyPath feeds a synthetic record matching Google's
// type-34 shape and asserts the parser extracts the public fields.
func TestTryParseHotelHappyPath(t *testing.T) {
	// Build a minimal payload mirroring the real shape from the sniff:
	// payload = [[null, name, meta, [class_label, class_int], _, images, prices, [[rating,reviews]], ...]]
	payload := []any{
		[]any{
			nil,
			"The Westin St. Francis",
			[]any{
				[]any{37.78, -122.41}, // [0] lat/lng
			},
			[]any{"4-star hotel", 4}, // [3]
			nil,                      // [4]
			[]any{},                  // [5] images
			[]any{ // [6] pricing
				nil,
				[]any{[]any{300, 0}, nil, nil, "USD"}, // [1][3]=currency
				[]any{ // [2]
					nil,
					[]any{"$184", "$217", 184.02, nil, 184}, // [2][1]=price
				},
			},
			[]any{ // [7] rating
				[]any{4.6, 1234.0},
			},
			nil,                           // [8]
			"0x5487:0xde3b",               // [9]
			nil,                           // [10]
			[]any{"Upscale grande dame."}, // [11] description
			[]any{"https://lh4.googleusercontent.com/proxy/x"}, // [12] thumb
			nil, nil, nil, nil, nil, nil, nil,
			"ChcIyIDJ4cf-7J3eARoKL20vMDJycDRobBAB", // [20] token
		},
	}
	h, ok := tryParseHotel(payload)
	if !ok {
		t.Fatal("expected parse ok")
	}
	if h.Name != "The Westin St. Francis" {
		t.Errorf("name = %q", h.Name)
	}
	if h.HotelClass != 4 {
		t.Errorf("hotel_class = %d", h.HotelClass)
	}
	if h.Rating != 4.6 {
		t.Errorf("rating = %v", h.Rating)
	}
	if h.Reviews != 1234 {
		t.Errorf("reviews = %d", h.Reviews)
	}
	if h.PricePerNight != 184.02 {
		t.Errorf("price = %v", h.PricePerNight)
	}
	if h.Currency != "USD" {
		t.Errorf("currency = %q", h.Currency)
	}
	if h.Latitude == 0 || h.Longitude == 0 {
		t.Errorf("lat/lng missing: %v %v", h.Latitude, h.Longitude)
	}
	if h.PropertyToken == "" {
		t.Errorf("property_token missing")
	}
	if h.BookingURLs.GoogleURL == "" {
		t.Errorf("google_url should be derived from token")
	}
	if h.Brand != "Westin" {
		t.Errorf("brand inferred = %q, want Westin", h.Brand)
	}
	if h.Description == "" {
		t.Errorf("description missing")
	}
}

// TestTryParseHotelDegradesOnReshuffle checks that a record missing
// optional indices still parses (name-only) instead of panicking.
func TestTryParseHotelDegradesOnReshuffle(t *testing.T) {
	payload := []any{[]any{nil, "Some Hotel"}}
	h, ok := tryParseHotel(payload)
	if !ok {
		t.Fatal("expected parse ok with name-only record")
	}
	if h.Name != "Some Hotel" {
		t.Errorf("name = %q", h.Name)
	}
}

func TestParseDollarString(t *testing.T) {
	cases := map[string]float64{
		"$184":    184,
		"$1,234":  1234,
		"$0":      0,
		"":        0,
		"garbage": 0,
	}
	for in, want := range cases {
		if got := parseDollarString(in); got != want {
			t.Errorf("parseDollarString(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestProgramForBrand(t *testing.T) {
	cases := map[string]string{
		"Park Hyatt":          "hyatt",
		"Westin":              "marriott",
		"DoubleTree":          "hilton",
		"Holiday Inn Express": "ihg",
		"Sofitel":             "accor",
		"Joe's Random Inn":    "",
	}
	for sub, want := range cases {
		if got := ProgramForBrand(sub); got != want {
			t.Errorf("ProgramForBrand(%q) = %q, want %q", sub, got, want)
		}
	}
}
