// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

package watch

import (
	"strings"
	"testing"
)

func TestValidateAcceptsHappyPath(t *testing.T) {
	w := &Watch{
		Origin:        "sfo",
		Destination:   "jfk",
		DepartureDate: "2026-06-21",
		Airline:       "dl",
		FlightNumber:  "669",
		Cabin:         "economy",
		OriginalPrice: 428.20,
		Threshold:     50,
		Currency:      "usd",
	}
	if err := w.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	// Validate is expected to normalize casing.
	if w.Origin != "SFO" || w.Destination != "JFK" || w.Airline != "DL" || w.Currency != "USD" {
		t.Fatalf("Validate did not normalize casing: %+v", w)
	}
	if w.Status != StatusActive {
		t.Fatalf("Validate did not default status to active: %q", w.Status)
	}
	if w.Passengers != 1 {
		t.Fatalf("Validate did not default passengers to 1: %d", w.Passengers)
	}
}

func TestValidateRejectsBadInput(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*Watch)
		want string
	}{
		{"bad origin", func(w *Watch) { w.Origin = "S F" }, "origin"},
		{"bad date", func(w *Watch) { w.DepartureDate = "2026/06/21" }, "departure date"},
		{"same airport", func(w *Watch) { w.Destination = w.Origin }, "origin and destination"},
		{"bad airline", func(w *Watch) { w.Airline = "Delta" }, "airline"},
		{"bad flight number", func(w *Watch) { w.FlightNumber = "DL669" }, "flight number"},
		{"bad cabin", func(w *Watch) { w.Cabin = "first-class" }, "cabin"},
		{"zero paid", func(w *Watch) { w.OriginalPrice = 0 }, "paid price"},
		{"negative threshold", func(w *Watch) { w.Threshold = -1 }, "threshold"},
		{"bad notify", func(w *Watch) { w.Notify = "slack://foo" }, "notify"},
		{"bad webhook scheme", func(w *Watch) { w.Notify = "webhook:ftp://nope" }, "http or https"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := &Watch{
				Origin: "SFO", Destination: "JFK",
				DepartureDate: "2026-06-21",
				Airline:       "DL", FlightNumber: "669",
				OriginalPrice: 100, Threshold: 5, Currency: "USD",
			}
			tc.mut(w)
			err := w.Validate()
			if err == nil {
				t.Fatalf("expected error mentioning %q, got nil", tc.want)
			}
			if !strings.Contains(strings.ToLower(err.Error()), tc.want) {
				t.Fatalf("expected error to mention %q, got %v", tc.want, err)
			}
		})
	}
}

func TestValidateAcceptsThreeLetterAirlineCode(t *testing.T) {
	w := newSampleWatch()
	w.Airline = "DAL" // ICAO 3-letter
	if err := w.Validate(); err != nil {
		t.Fatalf("3-letter airline should validate: %v", err)
	}
}

func newSampleWatch() *Watch {
	return &Watch{
		Origin:        "SFO",
		Destination:   "JFK",
		DepartureDate: "2026-06-21",
		Airline:       "DL",
		FlightNumber:  "669",
		Cabin:         "economy",
		OriginalPrice: 428.20,
		Threshold:     50,
		Currency:      "USD",
	}
}
