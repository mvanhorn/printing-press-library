package client

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newMockServer(t *testing.T, capture *capturedAjax) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc(reservationPath, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		io.WriteString(w, `<html><script>window._wpnonce = "test-nonce-123"; window.NP_PLUGIN_DATA.ajaxUrl = "/x";</script></html>`)
	})
	mux.HandleFunc(locationsPath, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `window.NP_PLUGIN_DATA.location = { 'locations': [{'name':'MasterPark Lot B','codeID':'2515-1-889'},{'name':'MasterPark Lot G','codeID':'2525-1-893'}] };`)
	})
	mux.HandleFunc(ajaxPath, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		capture.headers = r.Header.Clone()
		capture.body = body
		_ = json.Unmarshal(body, &capture.parsed)
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"errors":[],"data":[{"quote":1,"name":"Standard","rate":"19.99","total":"49.98"}]}`)
	})
	return httptest.NewServer(mux)
}

type capturedAjax struct {
	headers http.Header
	body    []byte
	parsed  map[string]interface{}
}

func TestGetQuotesRequestShape(t *testing.T) {
	cap := &capturedAjax{}
	srv := newMockServer(t, cap)
	defer srv.Close()

	c := New(srv.URL, 5*time.Second)
	req := QuoteRequest{
		Location: "2515-1-889",
		Reservation: Reservation{
			StartDate: "2026-06-11 07:00",
			EndDate:   "2026-06-13 18:30",
			Source:    "website",
			Quote:     -2,
			Services:  []interface{}{},
			Vehicle:   Vehicle{Type: "standard"},
		},
	}
	data, err := c.GetQuotes(context.Background(), req)
	if err != nil {
		t.Fatalf("GetQuotes: %v", err)
	}

	// Headers
	if got := cap.headers.Get("X-CSRF-TOKEN"); got != "test-nonce-123" {
		t.Errorf("X-CSRF-TOKEN = %q, want test-nonce-123", got)
	}
	if got := cap.headers.Get("X-Requested-With"); got != "XMLHttpRequest" {
		t.Errorf("X-Requested-With = %q", got)
	}
	if got := cap.headers.Get("Content-Type"); got != "application/json;charset=UTF-8" {
		t.Errorf("Content-Type = %q", got)
	}
	if got := cap.headers.Get("Accept"); got != "application/json, text/plain, */*" {
		t.Errorf("Accept = %q", got)
	}

	// Body shape
	p := cap.parsed
	if p["action"] != "np_ajax" {
		t.Errorf("action = %v", p["action"])
	}
	if p["method"] != "getQuotes" {
		t.Errorf("method = %v", p["method"])
	}
	if p["location"] != "2515-1-889" {
		t.Errorf("location = %v", p["location"])
	}
	if _, ok := p["multi_locations"]; !ok {
		t.Errorf("multi_locations missing")
	}
	if p["resRate"] != false {
		t.Errorf("resRate = %v", p["resRate"])
	}
	res, ok := p["reservation"].(map[string]interface{})
	if !ok {
		t.Fatalf("reservation not an object: %v", p["reservation"])
	}
	if res["start_date"] != "2026-06-11 07:00" {
		t.Errorf("start_date = %v", res["start_date"])
	}
	if res["end_date"] != "2026-06-13 18:30" {
		t.Errorf("end_date = %v", res["end_date"])
	}
	if res["quote"] != float64(-2) {
		t.Errorf("quote = %v", res["quote"])
	}
	if res["source"] != "website" {
		t.Errorf("source = %v", res["source"])
	}
	veh, ok := res["vehicle"].(map[string]interface{})
	if !ok || veh["type"] != "standard" {
		t.Errorf("vehicle = %v", res["vehicle"])
	}

	// Response data passes through
	var quotes []map[string]interface{}
	if err := json.Unmarshal(data, &quotes); err != nil || len(quotes) != 1 {
		t.Fatalf("quotes data = %s err=%v", data, err)
	}
}

func TestLocationsParse(t *testing.T) {
	cap := &capturedAjax{}
	srv := newMockServer(t, cap)
	defer srv.Close()

	c := New(srv.URL, 5*time.Second)
	locs, err := c.Locations(context.Background())
	if err != nil {
		t.Fatalf("Locations: %v", err)
	}
	if len(locs) != 2 {
		t.Fatalf("want 2 locations, got %d: %v", len(locs), locs)
	}
	if locs[0].CodeID != "2515-1-889" || locs[1].CodeID != "2525-1-893" {
		t.Errorf("unexpected codeIDs: %v", locs)
	}
}

func TestResolveLot(t *testing.T) {
	cases := map[string]string{
		"B":          "2515-1-889",
		"g":          "2525-1-893",
		"2515-1-889": "2515-1-889",
	}
	for in, want := range cases {
		got, err := ResolveLot(in)
		if err != nil {
			t.Errorf("ResolveLot(%q): %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("ResolveLot(%q) = %q, want %q", in, got, want)
		}
	}
	if _, err := ResolveLot("Z"); err == nil {
		t.Errorf("expected error for unknown lot Z")
	}
}

// TestParseLocationsLiveShape covers the real locations.php payload, which is a
// JS object with a single-quoted "locations" key wrapping the array. The old
// non-greedy parser masked this shape by stopping at the first '}'.
func TestParseLocationsLiveShape(t *testing.T) {
	body := []byte(`window.NP_PLUGIN_DATA.location = { 'locations': [` +
		`{ 'name': 'MasterPark Lot B', 'codeID': '2515-1-889' },` +
		`{ 'name': 'MasterPark Lot G', 'codeID': '2525-1-893' }` +
		`] };`)
	locs, err := parseLocations(body)
	if err != nil {
		t.Fatalf("parseLocations live shape: %v", err)
	}
	if len(locs) != 2 {
		t.Fatalf("want 2 locations, got %d: %v", len(locs), locs)
	}
	if locs[0].Name != "MasterPark Lot B" || locs[0].CodeID != "2515-1-889" {
		t.Errorf("loc[0] = %+v", locs[0])
	}
	if locs[1].CodeID != "2525-1-893" {
		t.Errorf("loc[1] = %+v", locs[1])
	}
}

func TestParseLocationsSingleQuotedEscapedApostrophe(t *testing.T) {
	body := []byte(`window.NP_PLUGIN_DATA.location = { 'locations': [` +
		`{ 'name': 'Traveler\'s Choice Lot', 'codeID': '9999-1-111' }` +
		`] };`)
	locs, err := parseLocations(body)
	if err != nil {
		t.Fatalf("parseLocations escaped apostrophe: %v", err)
	}
	if len(locs) != 1 {
		t.Fatalf("want 1 location, got %d: %v", len(locs), locs)
	}
	if locs[0].Name != "Traveler's Choice Lot" || locs[0].CodeID != "9999-1-111" {
		t.Errorf("loc = %+v", locs[0])
	}
}

// TestParseLocationsBareArray keeps the alternate (double-quoted array) shape
// working.
func TestParseLocationsBareArray(t *testing.T) {
	body := []byte(`window.NP_PLUGIN_DATA.location = [{"name":"Lot B","codeID":"2515-1-889"}];`)
	locs, err := parseLocations(body)
	if err != nil {
		t.Fatalf("parseLocations bare array: %v", err)
	}
	if len(locs) != 1 || locs[0].CodeID != "2515-1-889" {
		t.Errorf("locs = %+v", locs)
	}
}
