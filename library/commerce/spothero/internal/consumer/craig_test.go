package consumer

import (
	_ "embed"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

//go:embed testdata/search_transient_live_shape.json
var craigTransientFixture []byte

//go:embed testdata/search_transient_facility_live_shape.json
var craigTransientFacilityFixture []byte

//go:embed testdata/search_transient_facility_outside_hours_shape.json
var craigTransientFacilityOutsideHoursFixture []byte

func TestParseCraigTransientFixture(t *testing.T) {
	var resp craigSearchResponse
	if err := json.Unmarshal(craigTransientFixture, &resp); err != nil {
		t.Fatal(err)
	}
	spots, err := parseCraigSearchResults(resp.Results)
	if err != nil {
		t.Fatal(err)
	}
	if len(spots) != 1 {
		t.Fatalf("len=%d", len(spots))
	}
	s := spots[0]
	if s.FacilityID != 107496 {
		t.Fatalf("id=%d", s.FacilityID)
	}
	if s.PriceCents != 2968 || s.Price != "$29.68" {
		t.Fatalf("price=%q cents=%d", s.Price, s.PriceCents)
	}
	if !s.Available || s.DistanceMeters != 63 {
		t.Fatalf("avail=%v dist=%d", s.Available, s.DistanceMeters)
	}
}

func TestSearchUsesCraigTransientGET(t *testing.T) {
	var gotPath, gotAction, gotStarts string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/search-params/" {
			_, _ = w.Write([]byte(`{"meta":{},"data":{"google_place":{"place_id":"p"},"latitude":37.788,"longitude":-122.396,"starts":"2099-09-15T09:00","ends":"2099-09-15T17:00","distance_lt":1609,"sort":"distance","sort_order":"asc","page_info":{"setup":{"city":{"slug":"san-francisco"}}}}}`))
			return
		}
		if r.URL.Path == "/search/transient" {
			gotPath = r.URL.Path
			gotAction = r.URL.Query().Get("action")
			gotStarts = r.URL.Query().Get("starts")
			if r.URL.Query().Get("session_id") == "" || r.URL.Query().Get("search_id") == "" {
				t.Fatal("missing tracking UUIDs")
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(craigTransientFixture)
			return
		}
		t.Fatalf("unexpected %s", r.URL.Path)
	}))
	t.Cleanup(srv.Close)

	c := New(nil)
	c.BaseURL = srv.URL
	c.CraigBaseURL = srv.URL
	c.HTTP = srv.Client()

	res, err := c.Search(SearchQuery{
		Address: "500 Howard Street San Francisco",
		Starts:  "2099-09-15T09:00",
		Ends:    "2099-09-15T17:00",
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/search/transient" || gotAction != "LIST_SEARCH" {
		t.Fatalf("path=%q action=%q", gotPath, gotAction)
	}
	if !strings.Contains(gotStarts, "T") {
		t.Fatalf("starts not UTC: %q", gotStarts)
	}
	if len(res.Results) != 1 || res.Results[0].Price != "$29.68" {
		t.Fatalf("%+v", res)
	}
}

func TestParseCraigTransientFacilityFixture(t *testing.T) {
	var resp craigFacilityResponse
	if err := json.Unmarshal(craigTransientFacilityFixture, &resp); err != nil {
		t.Fatal(err)
	}
	rates, title, err := parseCraigFacilityRates(resp.Result, 6698, "2099-09-15T09:00", "2099-09-15T17:00")
	if err != nil {
		t.Fatal(err)
	}
	if len(rates) != 1 {
		t.Fatalf("len=%d", len(rates))
	}
	if rates[0].PriceCents != 2968 || rates[0].Price != "$29.68" {
		t.Fatalf("price=%q cents=%d", rates[0].Price, rates[0].PriceCents)
	}
	if rates[0].RateID != "113821" || rates[0].QuoteToken != "REDACTED-QUOTE-TOKEN" || rates[0].QuoteMAC != "113821" || !rates[0].Available {
		t.Fatalf("rate=%+v", rates[0])
	}
	if rates[0].ContextStarts != "2026-09-15T09:00:00-07:00" {
		t.Fatalf("context starts=%q", rates[0].ContextStarts)
	}
	if title != "Example Facility 6698" {
		t.Fatalf("title=%q", title)
	}
}

func TestParseCraigTransientFacilityOutsideHoursFixture(t *testing.T) {
	var resp craigFacilityResponse
	if err := json.Unmarshal(craigTransientFacilityOutsideHoursFixture, &resp); err != nil {
		t.Fatal(err)
	}
	rates, title, err := parseCraigFacilityRates(resp.Result, 144293, "2099-09-14T09:00", "2099-09-14T17:00")
	if err != nil {
		t.Fatal(err)
	}
	if len(rates) != 1 {
		t.Fatalf("len=%d", len(rates))
	}
	r := rates[0]
	if r.Available {
		t.Fatalf("expected unavailable, got %+v", r)
	}
	if r.QuoteToken != "" {
		t.Fatalf("quote_token=%q", r.QuoteToken)
	}
	if len(r.UnavailableReasons) != 1 || r.UnavailableReasons[0] != "Outside Hours" {
		t.Fatalf("reasons=%v", r.UnavailableReasons)
	}
	if r.PriceCents != 1500 || r.RateID != "144293" {
		t.Fatalf("rate=%+v", r)
	}
	if title != "Foundry 144293" {
		t.Fatalf("title=%q", title)
	}
}

func TestGetFacilityRatesUsesCraigTransientFacilityGET(t *testing.T) {
	var gotPath, gotStarts, gotEnds string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/search/transient/6698" {
			gotPath = r.URL.Path
			gotStarts = r.URL.Query().Get("starts")
			gotEnds = r.URL.Query().Get("ends")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(craigTransientFacilityFixture)
			return
		}
		t.Fatalf("unexpected %s", r.URL.Path)
	}))
	t.Cleanup(srv.Close)

	c := New(nil)
	c.CraigBaseURL = srv.URL
	c.HTTP = srv.Client()

	quote, err := c.GetFacilityRates(FacilityRateQuery{
		FacilityID: 6698,
		Starts:     "2099-09-15T09:00",
		Ends:       "2099-09-15T17:00",
		CitySlug:   "san-francisco",
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/search/transient/6698" {
		t.Fatalf("path=%q", gotPath)
	}
	if gotStarts != "2099-09-15T16:00:00Z" || gotEnds != "2099-09-16T00:00:00Z" {
		t.Fatalf("starts=%q ends=%q", gotStarts, gotEnds)
	}
	if len(quote.Rates) != 1 || quote.Rates[0].PriceCents != 2968 {
		t.Fatalf("%+v", quote)
	}
	if quote.Title != "Example Facility 6698" {
		t.Fatalf("title=%q", quote.Title)
	}
	if quote.TimezoneNote == "" {
		t.Fatal("expected timezone note")
	}
}
