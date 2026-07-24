// Copyright 2026 Avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNormalizeAndFilterAMCShowtimes(t *testing.T) {
	raw := json.RawMessage(`{"_embedded":{"showtimes":[
		{"id":2,"movie":{"name":"Dune"},"theatre":{"name":"AMC Far"},"showDateTimeLocal":"2026-07-24T19:00","presentation":{"name":"Standard"},"distance":5.0},
		{"id":1,"movie":{"name":"Dune: Part Two"},"theatre":{"name":"AMC Near"},"showDateTimeLocal":"2026-07-24T20:15","presentation":{"name":"IMAX"},"distance":1.2}
	]}}`)
	rows, err := normalizeAMCShowtimes(raw)
	if err != nil {
		t.Fatal(err)
	}
	rows = filterAMCShowtimes(rows, "dune", 20*60, "imax")
	if len(rows) != 1 || rows[0].ID != "1" || rows[0].Theatre != "AMC Near" {
		t.Fatalf("filtered rows = %#v", rows)
	}
}

func TestMoviePlanCurrentLocationHTTPContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-AMC-Vendor-Key"); got != "vendor-key" {
			t.Errorf("X-AMC-Vendor-Key = %q", got)
		}
		if got := r.Header.Get("X-AMC-Auth-Token"); got != "viewer-token" {
			t.Errorf("X-AMC-Auth-Token = %q", got)
		}
		want := "/v2/showtimes/views/current-location/2026-07-24/40.7128/-74.006?page-size=100"
		if r.URL.RequestURI() != want {
			t.Errorf("RequestURI = %q, want %q", r.URL.RequestURI(), want)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"showtimes": []map[string]any{{
			"id": 7, "movieName": "Dune", "theatreName": "AMC 25",
			"showDateTimeLocal": "2026-07-24T20:30", "format": "IMAX", "distanceMiles": 1.5,
		}}})
	}))
	defer server.Close()

	t.Setenv("AMC_THEATRES_BASE_URL", server.URL)
	t.Setenv("AMC_THEATRES_VENDOR_KEY", "vendor-key")
	t.Setenv("AMC_THEATRES_AUTH_TOKEN", "viewer-token")
	t.Setenv("AMC_THEATRES_HOME", t.TempDir())
	root := RootCmd()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"movie-plan", "Dune", "--latitude", "40.7128", "--longitude", "-74.0060", "--date", "2026-07-24", "--after", "20:00", "--format", "IMAX", "--json", "--no-cache"})
	if err := root.Execute(); err != nil {
		t.Fatalf("movie-plan error = %v, stderr = %s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"theatre": "AMC 25"`) || !strings.Contains(stdout.String(), `"count": 1`) {
		t.Fatalf("unexpected output: %s", stdout.String())
	}
}

func TestMoviePlanDryRunAndUsageErrors(t *testing.T) {
	t.Setenv("AMC_THEATRES_HOME", t.TempDir())
	root := RootCmd()
	var stdout bytes.Buffer
	root.SetOut(&stdout)
	root.SetArgs([]string{"movie-plan", "--theatre", "123", "--date", "2026-07-24", "--dry-run", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("dry-run error = %v", err)
	}
	if !strings.Contains(stdout.String(), `/v2/theatres/123/showtimes/2026-07-24`) {
		t.Fatalf("dry-run output = %s", stdout.String())
	}

	root = RootCmd()
	root.SetArgs([]string{"movie-plan", "--theatre", "123", "--latitude", "1", "--longitude", "2"})
	if err := root.Execute(); err == nil {
		t.Fatal("conflicting location sources returned nil error")
	}
}
