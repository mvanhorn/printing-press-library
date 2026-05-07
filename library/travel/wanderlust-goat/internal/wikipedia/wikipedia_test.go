package wikipedia

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fixtureClient returns a Client whose ActionBase and RESTBase point at the
// given httptest server. We append the path components the production code
// would have appended to the real api.php / rest_v1 endpoints.
func fixtureClient(srvURL string) *Client {
	c := New("en", nil, "test-ua")
	c.ActionBase = srvURL + "/w/api.php"
	c.RESTBase = srvURL + "/api/rest_v1"
	return c
}

func TestGeoSearch_ParsesPages(t *testing.T) {
	var seenQuery, seenUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenQuery = r.URL.RawQuery
		seenUA = r.Header.Get("User-Agent")
		if !strings.HasSuffix(r.URL.Path, "/w/api.php") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"batchcomplete": true,
			"query": {
				"geosearch": [
					{"pageid":1,"ns":0,"title":"Tokyo Tower","lat":35.6586,"lon":139.7454,"dist":12.3,"primary":""},
					{"pageid":2,"ns":0,"title":"Zojoji","lat":35.6577,"lon":139.7488,"dist":456.7,"primary":""}
				]
			}
		}`))
	}))
	defer srv.Close()

	c := fixtureClient(srv.URL)
	resp, err := c.GeoSearch(context.Background(), 35.6586, 139.7454, 1000, 5)
	if err != nil {
		t.Fatalf("GeoSearch: %v", err)
	}
	if len(resp.Pages) != 2 {
		t.Fatalf("expected 2 pages, got %d", len(resp.Pages))
	}
	if resp.Pages[0].Title != "Tokyo Tower" || resp.Pages[0].PageID != 1 {
		t.Errorf("first page: %+v", resp.Pages[0])
	}
	if resp.Pages[1].Distance != 456.7 {
		t.Errorf("second page distance: got %v", resp.Pages[1].Distance)
	}

	for _, want := range []string{"action=query", "list=geosearch", "gslimit=5", "gsradius=1000"} {
		if !strings.Contains(seenQuery, want) {
			t.Errorf("query missing %q: %q", want, seenQuery)
		}
	}
	if seenUA == "" {
		t.Error("missing User-Agent header")
	}
}

func TestPageSummary_FlattensPageURL(t *testing.T) {
	var seenPath, seenRawPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.Path
		seenRawPath = r.URL.RawPath
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"title":"Tokyo Tower",
			"extract":"Tokyo Tower is a communications and observation tower in Tokyo, Japan.",
			"extract_html":"<p>Tokyo Tower is a communications and observation tower in Tokyo, Japan.</p>",
			"coordinates":{"lat":35.6586,"lon":139.7454},
			"content_urls":{
				"desktop":{"page":"https://en.wikipedia.org/wiki/Tokyo_Tower"},
				"mobile":{"page":"https://en.m.wikipedia.org/wiki/Tokyo_Tower"}
			}
		}`))
	}))
	defer srv.Close()

	c := fixtureClient(srv.URL)
	sum, err := c.PageSummary(context.Background(), "Tokyo Tower")
	if err != nil {
		t.Fatalf("PageSummary: %v", err)
	}
	if sum.Title != "Tokyo Tower" {
		t.Errorf("title: got %q", sum.Title)
	}
	if !strings.Contains(sum.Extract, "communications and observation tower") {
		t.Errorf("extract: %q", sum.Extract)
	}
	if sum.PageURL != "https://en.wikipedia.org/wiki/Tokyo_Tower" {
		t.Errorf("page URL not flattened: %q", sum.PageURL)
	}
	if sum.Coordinates == nil || sum.Coordinates.Lat != 35.6586 {
		t.Errorf("coords: %+v", sum.Coordinates)
	}
	// Decoded path should round-trip to "Tokyo Tower"; RawPath preserves the
	// escaped wire form for inspection.
	if !strings.HasSuffix(seenPath, "/page/summary/Tokyo Tower") {
		t.Errorf("path: got %q", seenPath)
	}
	if seenRawPath != "" && !strings.Contains(seenRawPath, "/page/summary/Tokyo%20Tower") {
		t.Errorf("raw path: got %q", seenRawPath)
	}
}

func TestPageSummary_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"detail":"not found"}`))
	}))
	defer srv.Close()

	c := fixtureClient(srv.URL)
	_, err := c.PageSummary(context.Background(), "Nonexistent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("error should mention 404: %v", err)
	}
}

func TestNew_DefaultsLocale(t *testing.T) {
	c := New("", nil, "")
	if c.Locale != "en" {
		t.Errorf("expected default locale en, got %q", c.Locale)
	}
	if !strings.Contains(c.ActionBase, "en.wikipedia.org") {
		t.Errorf("action base: %q", c.ActionBase)
	}
	if c.UserAgent == "" {
		t.Error("expected default user agent")
	}
}
