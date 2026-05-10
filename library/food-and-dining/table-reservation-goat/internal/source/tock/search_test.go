// Copyright 2026 pejman-pour-moezzi. Licensed under Apache-2.0. See LICENSE.

package tock

import (
	"context"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/enetx/surf"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/table-reservation-goat/internal/cliutil"
)

func TestBuildSearchPath(t *testing.T) {
	cases := []struct {
		name string
		p    SearchParams
		want []string // substrings that must appear in the result
	}{
		{
			name: "Seattle basic",
			p:    SearchParams{City: "Seattle", Date: "2026-05-10", Time: "19:00", PartySize: 4, Lat: 47.6062, Lng: -122.3321},
			want: []string{
				"/city/seattle/search?",
				"city=Seattle",
				"date=2026-05-10",
				"latlng=47.6062%2C-122.3321",
				"size=4",
				"time=19%3A00",
				"type=DINE_IN_EXPERIENCES",
			},
		},
		{
			name: "Multi-word city slugifies",
			p:    SearchParams{City: "New York", Date: "2026-05-10", Time: "19:00", PartySize: 2, Lat: 40.7589, Lng: -73.9851},
			want: []string{
				"/city/new-york/search?",
				"city=New+York",
				"latlng=40.7589%2C-73.9851",
			},
		},
		{
			name: "Zero party size omitted",
			p:    SearchParams{City: "Chicago", Date: "2026-05-10", Time: "19:00", PartySize: 0, Lat: 41.8781, Lng: -87.6298},
			want: []string{
				"/city/chicago/search?",
				"city=Chicago",
				"date=2026-05-10",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := buildSearchPath(tc.p)
			for _, sub := range tc.want {
				if !strings.Contains(got, sub) {
					t.Errorf("buildSearchPath = %q; missing %q", got, sub)
				}
			}
			if tc.p.PartySize == 0 && strings.Contains(got, "size=") {
				t.Errorf("buildSearchPath = %q; expected no size= when PartySize=0", got)
			}
		})
	}
}

func TestCitySlugFromName(t *testing.T) {
	cases := map[string]string{
		"Seattle":     "seattle",
		"New York":    "new-york",
		"Los Angeles": "los-angeles",
		"  Chicago  ": "chicago",
		"":            "",
	}
	for in, want := range cases {
		if got := citySlugFromName(in); got != want {
			t.Errorf("citySlugFromName(%q) = %q; want %q", in, got, want)
		}
	}
}

func TestDecodeCuisines(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{"single string", `"Indian"`, "Indian"},
		{"array of two", `["Indian","Vegetarian"]`, "Indian, Vegetarian"},
		{"empty array", `[]`, ""},
		{"null", `null`, ""},
		{"empty raw", ``, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := decodeCuisines([]byte(tc.raw))
			if got != tc.want {
				t.Errorf("decodeCuisines(%q) = %q; want %q", tc.raw, got, tc.want)
			}
		})
	}
}

func TestEntryToBusiness(t *testing.T) {
	e := offeringAvailEntry{}
	e.Business.ID = 25727
	e.Business.Name = "Kricket Club"
	e.Business.DomainName = "kricketclub"
	e.Business.BusinessType = "Restaurant"
	e.Business.Cuisines = []byte(`"Indian"`)
	e.Business.Neighborhood = "Ravenna"
	e.Business.Location.City = "Seattle"
	e.Business.Location.State = "WA"
	e.Business.Location.Lat = 47.6760166
	e.Business.Location.Lng = -122.3012957
	e.Ranking.DistanceMeters = 8097.4
	e.Ranking.RelevanceScore = 96.15

	got := entryToBusiness(e)
	if got.ID != 25727 || got.Slug != "kricketclub" || got.Name != "Kricket Club" {
		t.Errorf("entryToBusiness identity wrong: %+v", got)
	}
	if got.URL != "https://www.exploretock.com/kricketclub" {
		t.Errorf("entryToBusiness URL = %q; want https://www.exploretock.com/kricketclub", got.URL)
	}
	if got.Cuisine != "Indian" || got.Neighborhood != "Ravenna" || got.City != "Seattle" {
		t.Errorf("entryToBusiness metadata wrong: cuisine=%q neighborhood=%q city=%q", got.Cuisine, got.Neighborhood, got.City)
	}
	if got.Latitude == 0 || got.Longitude == 0 {
		t.Errorf("entryToBusiness lat/lng zero: %+v", got)
	}
}

// fixtureHTML returns a minimal Tock SSR HTML response containing
// `window.$REDUX_STATE` with a synthetic offeringAvailability list.
// Used by the integration tests below.
func fixtureHTML(jsonBody string) string {
	return `<!doctype html><html><head><title>fixture</title></head>
<body>
<script>window.$ENV = {};</script>
<script>window.$REDUX_STATE = ` + jsonBody + `;</script>
<script>window.$APOLLO_STATE = {};</script>
</body></html>`
}

// newTestClient builds a minimal Tock Client that points at an httptest server.
// We can't reuse New() because it constructs a Surf client targeting the real
// origin; tests need to redirect to an httptest URL.
func newTestClient(t *testing.T, server *httptest.Server) *Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar: %v", err)
	}
	surfClient := surf.NewClient().Builder().Impersonate().Chrome().Session().Build().Unwrap()
	std := surfClient.Std()
	std.Jar = jar
	// Redirect the test client to the httptest server by overriding Transport.
	std.Transport = &rewriteTransport{base: http.DefaultTransport, target: server.URL}
	return &Client{
		http:    std,
		limiter: cliutil.NewAdaptiveLimiter(1.0),
	}
}

// rewriteTransport rewrites any request URL to point at the test server's
// host, preserving path and query. Lets the client think it's hitting
// www.exploretock.com while actually hitting httptest.
type rewriteTransport struct {
	base   http.RoundTripper
	target string
}

func (r *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Replace scheme + host with the target server's.
	target := strings.TrimSuffix(r.target, "/")
	newURL := target + req.URL.RequestURI()
	newReq := req.Clone(req.Context())
	parsed, err := http.NewRequest(req.Method, newURL, req.Body)
	if err != nil {
		return nil, err
	}
	parsed.Header = newReq.Header
	return r.base.RoundTrip(parsed)
}

func TestSearchCity_HappyPath(t *testing.T) {
	const stateJSON = `{
  "availability": {
    "result": {
      "count": 2,
      "offeringAvailability": [
        {
          "business": {
            "id": 25727,
            "name": "Kricket Club",
            "domainName": "kricketclub",
            "businessType": "Restaurant",
            "cuisines": "Indian",
            "neighborhood": "Ravenna",
            "location": {"address": "2404 NE 65th", "city": "Seattle", "state": "WA", "country": "US", "lat": 47.676, "lng": -122.301}
          },
          "ranking": {"distanceMeters": 8097.4, "relevanceScore": 96.15},
          "offering": []
        },
        {
          "business": {
            "id": 12345,
            "name": "Canlis",
            "domainName": "canlis",
            "businessType": "Restaurant",
            "cuisines": ["Pacific Northwest", "Tasting Menu"],
            "neighborhood": "Queen Anne",
            "location": {"address": "2576 Aurora N", "city": "Seattle", "state": "WA", "country": "US", "lat": 47.640, "lng": -122.347}
          },
          "ranking": {"distanceMeters": 4500.0, "relevanceScore": 200.0},
          "offering": []
        }
      ]
    }
  }
}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, fixtureHTML(stateJSON))
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	got, err := c.SearchCity(context.Background(), SearchParams{
		City: "Seattle", Date: "2026-05-10", Time: "19:00", PartySize: 4,
		Lat: 47.6062, Lng: -122.3321,
	})
	if err != nil {
		t.Fatalf("SearchCity error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("SearchCity len = %d; want 2", len(got))
	}
	if got[0].Slug != "kricketclub" || got[0].Cuisine != "Indian" {
		t.Errorf("entry[0] wrong: %+v", got[0])
	}
	if got[1].Slug != "canlis" || got[1].Cuisine != "Pacific Northwest, Tasting Menu" {
		t.Errorf("entry[1] cuisines: %+v", got[1])
	}
	for _, b := range got {
		if !strings.HasPrefix(b.URL, "https://www.exploretock.com/") {
			t.Errorf("URL not Tock-prefixed: %q", b.URL)
		}
	}
}

func TestSearchCity_NoResults(t *testing.T) {
	const stateJSON = `{"availability": {"result": {"count": 0, "offeringAvailability": []}}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, fixtureHTML(stateJSON))
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	got, err := c.SearchCity(context.Background(), SearchParams{City: "Mars", Date: "2026-05-10", Time: "19:00", PartySize: 2})
	if err != nil {
		t.Fatalf("SearchCity error on empty result: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty slice, got %d entries", len(got))
	}
}

func TestSearchCity_NoResultSubtree(t *testing.T) {
	// `availability` is present but has no `result` (e.g., request still in flight at SSR time).
	const stateJSON = `{"availability": {"isInitialized": false, "requestInProgress": false}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, fixtureHTML(stateJSON))
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	got, err := c.SearchCity(context.Background(), SearchParams{City: "Seattle", Date: "2026-05-10", Time: "19:00", PartySize: 2})
	if err != nil {
		t.Fatalf("SearchCity should treat missing result as zero rows, got error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty slice, got %d entries", len(got))
	}
}

func TestSearchCity_MissingAvailability(t *testing.T) {
	// SPA-refactor scenario: $REDUX_STATE present but `availability` slice is missing entirely.
	const stateJSON = `{"app": {"jwtToken": ""}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, fixtureHTML(stateJSON))
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	_, err := c.SearchCity(context.Background(), SearchParams{City: "Seattle", Date: "2026-05-10", Time: "19:00", PartySize: 2})
	if err == nil {
		t.Fatal("expected error when state.availability is missing; got nil")
	}
	if !strings.Contains(err.Error(), "Tock SPA") {
		t.Errorf("expected SPA-refactor sentinel, got: %v", err)
	}
}

func TestSearchCity_MissingReduxState(t *testing.T) {
	// Tock's HTML present but no $REDUX_STATE — extractor inside FetchReduxState fails first.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><body>no state here</body></html>`)
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	_, err := c.SearchCity(context.Background(), SearchParams{City: "Seattle", Date: "2026-05-10", Time: "19:00", PartySize: 2})
	if err == nil {
		t.Fatal("expected error when $REDUX_STATE missing; got nil")
	}
}

func TestSearchCity_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal", http.StatusInternalServerError)
	}))
	defer srv.Close()
	c := newTestClient(t, srv)
	_, err := c.SearchCity(context.Background(), SearchParams{City: "Seattle", Date: "2026-05-10", Time: "19:00", PartySize: 2})
	if err == nil {
		t.Fatal("expected error on HTTP 500; got nil")
	}
}
