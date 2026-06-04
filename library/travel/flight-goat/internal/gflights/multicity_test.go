// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package gflights

import (
	"encoding/base64"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestMultiCityBookingURL_ShapeAndDecode(t *testing.T) {
	segments := []Segment{
		{Origin: "SFO", Destination: "NRT", DepartureDate: "2026-08-15"},
		{Origin: "NRT", Destination: "ICN", DepartureDate: "2026-08-28"},
		{Origin: "ICN", Destination: "SFO", DepartureDate: "2026-09-05"},
	}
	u, err := MultiCityBookingURL(segments)
	if err != nil {
		t.Fatalf("build URL: %v", err)
	}
	parsed, err := url.Parse(u)
	if err != nil {
		t.Fatalf("parse URL: %v", err)
	}
	if parsed.Host != "www.google.com" {
		t.Errorf("host = %q, want www.google.com", parsed.Host)
	}
	if parsed.Path != "/travel/flights" {
		t.Errorf("path = %q, want /travel/flights", parsed.Path)
	}
	q := parsed.Query()
	if q.Get("tfu") != "KgIIAw" {
		t.Errorf("tfu = %q, want KgIIAw (trip_type=3 marker)", q.Get("tfu"))
	}
	tfs := q.Get("tfs")
	if tfs == "" {
		t.Fatal("tfs missing")
	}
	raw, err := base64.RawURLEncoding.DecodeString(tfs)
	if err != nil {
		t.Fatalf("base64 decode: %v", err)
	}
	// The encoded protobuf must contain each segment's IATA codes and dates.
	for _, s := range segments {
		if !strings.Contains(string(raw), s.Origin) {
			t.Errorf("encoded tfs missing %q", s.Origin)
		}
		if !strings.Contains(string(raw), s.Destination) {
			t.Errorf("encoded tfs missing %q", s.Destination)
		}
		if !strings.Contains(string(raw), s.DepartureDate) {
			t.Errorf("encoded tfs missing %q", s.DepartureDate)
		}
	}
	// Tail must end with `mAED` (98 01 03) → field 19 varint = 3.
	if !strings.HasSuffix(tfs, "mAED") {
		t.Errorf("tfs does not end with multi-city marker mAED (field 19 = 3); got %q", tfs[len(tfs)-8:])
	}
}

func TestMultiCityBookingURL_RejectsSingleSegment(t *testing.T) {
	_, err := MultiCityBookingURL([]Segment{
		{Origin: "SFO", Destination: "NRT", DepartureDate: "2026-08-15"},
	})
	if err == nil || !strings.Contains(err.Error(), ">= 2") {
		t.Errorf("expected >=2 error, got %v", err)
	}
}

func TestMultiCityBookingURL_RejectsBadDate(t *testing.T) {
	_, err := MultiCityBookingURL([]Segment{
		{Origin: "SFO", Destination: "NRT", DepartureDate: "08-15-2026"},
		{Origin: "NRT", Destination: "SFO", DepartureDate: "2026-09-05"},
	})
	if err == nil || !strings.Contains(err.Error(), "YYYY-MM-DD") {
		t.Errorf("expected date format error, got %v", err)
	}
}

func TestBuildOffersPayload_OneWayUnchanged(t *testing.T) {
	// Regression: one-way payload must still build with the new signature
	// (sessionTok="") — the multi-city addition must not break callers.
	opts := SearchOptions{
		Origin:        "SFO",
		Destination:   "NRT",
		DepartureDate: "2026-08-15",
		Passengers:    1,
	}
	d, _ := time.Parse("2006-01-02", opts.DepartureDate)
	payload, err := buildOffersPayload(opts, d, time.Time{}, tripTypeOneWay, "")
	if err != nil {
		t.Fatalf("buildOffersPayload one-way: %v", err)
	}
	if payload == "" {
		t.Fatal("empty payload")
	}
}
