// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

//go:build capture

// PATCH(library): build-tagged capture helpers for the google-flights-parser-audit
// patch. Refreshes the testdata/*.json fixtures from live Google responses.
//
// Build-tagged helpers that hit Google live to refresh testdata fixtures.
// Not run in normal CI — invoke explicitly:
//
//   go test -tags capture -run TestCaptureSeaKtiResponse ./internal/gflights/...
//
// The captured payload feeds the parser regression test in flights_native_test.go.

package gflights

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCaptureSeaKtiResponse(t *testing.T) {
	captureFixture(t, SearchOptions{
		Origin:        "SEA",
		Destination:   "KTI",
		DepartureDate: "2026-12-24",
		ReturnDate:    "2027-01-01",
		Passengers:    4,
		Currency:      "USD",
	}, "sea_kti_2026-12-24_response.json")
}

func TestCaptureSeaBkkResponse(t *testing.T) {
	captureFixture(t, SearchOptions{
		Origin:        "SEA",
		Destination:   "BKK",
		DepartureDate: "2026-12-24",
		ReturnDate:    "2027-01-01",
		Passengers:    4,
		Currency:      "USD",
	}, "sea_bkk_2026-12-24_response.json")
}

func captureFixture(t *testing.T, opts SearchOptions, outName string) {
	depDate, _ := time.Parse("2006-01-02", opts.DepartureDate)
	retDate, _ := time.Parse("2006-01-02", opts.ReturnDate)

	payload, err := buildOffersPayload(opts, depDate, retDate, tripTypeRoundTrip)
	if err != nil {
		t.Fatalf("buildOffersPayload: %v", err)
	}
	body := "f.req=" + payload

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, offersEndpoint, strings.NewReader(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=UTF-8")
	req.Header.Set("User-Agent", chromeUserAgent)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("x-goog-ext-259736195-jspb", googleFlightsCurrencyHeader("USD"))

	resp, err := utlsClient().Do(req)
	if err != nil {
		t.Fatalf("calling endpoint: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	if err := os.MkdirAll("testdata", 0o755); err != nil {
		t.Fatalf("mkdir testdata: %v", err)
	}
	out := filepath.Join("testdata", outName)
	if err := os.WriteFile(out, raw, 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	t.Logf("wrote %d bytes to %s", len(raw), out)
}
