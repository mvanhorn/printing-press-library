// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package seatsaero

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func sampleResponse() string {
	return `{
	  "data": [
	    {
	      "ID": "2QSaUXJ0ZuSVqgrRWqkSlXhnVbS",
	      "RouteID": "2HmSwbzAS9SnEdtIsf3nkjozpX1",
	      "Route": {
	        "OriginAirport": "SFO",
	        "DestinationAirport": "HND",
	        "Source": "united",
	        "NumDaysOut": 75,
	        "Distance": 5135
	      },
	      "Date": "2026-10-01",
	      "YAvailable": true,
	      "WAvailable": false,
	      "JAvailable": true,
	      "FAvailable": false,
	      "YMileageCost": "45000",
	      "WMileageCost": "0",
	      "JMileageCost": "90000",
	      "FMileageCost": "0",
	      "Source": "united"
	    }
	  ],
	  "count": 1,
	  "hasMore": false,
	  "cursor": 0
	}`
}

func newTestServer(t *testing.T, wantOrigin, wantDest string, status int, body string) (*httptest.Server, *http.Client) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if got := r.Header.Get("Partner-Authorization"); got != "test-key" {
			t.Errorf("Partner-Authorization = %q, want test-key", got)
		}
		if r.URL.Path != "/search" {
			t.Errorf("path = %s, want /search", r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("origin_airport") != wantOrigin {
			t.Errorf("origin_airport = %q, want %q", q.Get("origin_airport"), wantOrigin)
		}
		if q.Get("destination_airport") != wantDest {
			t.Errorf("destination_airport = %q, want %q", q.Get("destination_airport"), wantDest)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	return srv, srv.Client()
}

func TestSearch_UsesPartnerAuthAndParses(t *testing.T) {
	srv, hc := newTestServer(t, "SFO", "HND", 200, sampleResponse())
	defer srv.Close()

	c := NewClient("test-key")
	c.BaseURL = srv.URL
	c.HTTP = hc

	res, err := c.Search(context.Background(), SearchParams{
		OriginAirport:      "SFO",
		DestinationAirport: "HND",
		StartDate:          "2026-10-01",
		EndDate:            "2026-10-31",
		Cabin:              "business",
		OrderBy:            "lowest_mileage",
		Take:               100,
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if res.Count != 1 || len(res.Data) != 1 {
		t.Fatalf("count=%d len=%d, want 1/1", res.Count, len(res.Data))
	}
	entry := res.Data[0]
	if entry.Route.OriginAirport != "SFO" || entry.Route.DestinationAirport != "HND" {
		t.Fatalf("route = %s->%s", entry.Route.OriginAirport, entry.Route.DestinationAirport)
	}
	if entry.Route.Source != "united" {
		t.Fatalf("source = %q", entry.Route.Source)
	}
	if entry.JMileage != "90000" || !entry.JAvailable || entry.FAvailable {
		t.Fatalf("cabin parse wrong: J=%s/%v F=%v", entry.JMileage, entry.JAvailable, entry.FAvailable)
	}
	if !res.Cached {
		t.Error("Cached should be true — the Seats.aero cached-search endpoint returns cached, not live, data")
	}
}

func TestSearch_SendsEnvelopeParams(t *testing.T) {
	srv, hc := newTestServer(t, "SFO,LAX", "HND,NRT", 200, `{"data":[],"count":0,"hasMore":false,"cursor":0}`)
	defer srv.Close()
	c := NewClient("test-key")
	c.BaseURL = srv.URL
	c.HTTP = hc
	if _, err := c.Search(context.Background(), SearchParams{
		OriginAirport: "SFO,LAX", DestinationAirport: "HND,NRT",
		OnlyDirectFlights: true, Take: 250,
	}); err != nil {
		t.Fatalf("Search: %v", err)
	}
}

func TestSearch_HTTPError(t *testing.T) {
	srv, hc := newTestServer(t, "SFO", "HND", 401, `{"error":"unauthorized"}`)
	defer srv.Close()
	c := NewClient("test-key")
	c.BaseURL = srv.URL
	c.HTTP = hc
	_, err := c.Search(context.Background(), SearchParams{OriginAirport: "SFO", DestinationAirport: "HND"})
	if err == nil || !strings.Contains(err.Error(), "HTTP 401") {
		t.Fatalf("want HTTP 401 error, got %v", err)
	}
}

func TestSearch_NoAPIKey(t *testing.T) {
	c := NewClient("")
	if c.HasAPIKey() {
		t.Fatal("HasAPIKey should be false with no key")
	}
	_, err := c.Search(context.Background(), SearchParams{OriginAirport: "SFO", DestinationAirport: "HND"})
	if err == nil {
		t.Fatal("expected ErrNoAPIKey, got nil")
	}
	if _, ok := err.(ErrNoAPIKey); !ok {
		t.Fatalf("expected ErrNoAPIKey, got %T: %v", err, err)
	}
}

func TestNewClient_ReadsConfigFileKey(t *testing.T) {
	// Reset env so the config-file fallback is exercised.
	t.Setenv("SEATS_AERO_API_KEY", "")
	t.Setenv("SEATS_AERO_PARTNER_PARTNER_AUTHORIZATION", "")
	dir := t.TempDir()
	cfgPath := dir + "/config.toml"
	if err := os.WriteFile(cfgPath, []byte("aero_partner_partner_authorization = \"cfg-key-123\"\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("SEATS_AERO_CONFIG", cfgPath)
	c := NewClient("")
	if !c.HasAPIKey() {
		t.Fatal("expected config-file key to be loaded")
	}
	if c.APIKey != "cfg-key-123" {
		t.Fatalf("APIKey = %q, want cfg-key-123", c.APIKey)
	}
}

func TestResult_RoundTripsJSON(t *testing.T) {
	res := &SearchResult{Data: []AvailabilityEntry{
		{
			ID: "x", RouteID: "r",
			Source: "united", Date: "2026-10-01",
			YAvailable: true, JMileage: "90000",
		},
	}, Count: 1, Cached: true}
	b, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(b), `"cached":true`) {
		t.Fatalf("cached flag missing: %s", b)
	}
}
