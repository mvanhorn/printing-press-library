// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

// PATCH(library): tests for the airport-alias-table patch (Kayak side).
// See kayak.go for the production code.

package kayak

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestDirectRemapsRetiredCode verifies that a request for a retired IATA
// code (PNH, REP) hits the current-code path (/direct/KTI, /direct/SAI).
// Without this remap, Kayak returns no routes because its index is keyed
// on current codes only.
func TestDirectRemapsRetiredCode(t *testing.T) {
	cases := []struct {
		input    string
		wantPath string
	}{
		{"PNH", "/direct/KTI"},
		{"pnh", "/direct/KTI"},
		{"REP", "/direct/SAI"},
		{"SEA", "/direct/SEA"},
		{"  PNH  ", "/direct/KTI"},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			var gotPath string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				// Minimal valid response containing a routes array.
				_, _ = w.Write([]byte(`<html><script>{"routes":[{"code":"AAA","localizedDisplay":"Test","displayLocation":"Test","fullName":"Test","countryCode":"US","airlineCodes":["AS"],"distance":100,"localizedTravelTime":"1h","duration":60,"flightsCount":1,"routeSeoLink":""}]}</script></html>`))
			}))
			defer srv.Close()

			c := New()
			c.BaseURL = srv.URL + "/direct"
			_, err := c.Direct(tc.input)
			if err != nil {
				t.Fatalf("Direct(%q): %v", tc.input, err)
			}
			if !strings.HasSuffix(gotPath, tc.wantPath) {
				t.Errorf("Direct(%q) hit path %q, want suffix %q", tc.input, gotPath, tc.wantPath)
			}
		})
	}
}
