package lefooding

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const fixtureSearchHTML = `<html><body>
<main>
  <article class="card">
    <a href="/en/restaurant/septime"><h3>Septime</h3><span>Paris</span> — <p>A classic of the rue de Charonne.</p></a>
  </article>
  <article class="card">
    <a href="/en/restaurant/chambelland"><h3>Chambelland</h3><span>Paris</span> — <p>Gluten-free bakery in the 11th.</p></a>
  </article>
  <article class="card">
    <a href="/en/category/cafes">All cafés</a>
  </article>
  <a href="/en/search?q=other">Search again</a>
</main>
</body></html>`

const fixtureRestaurantHTML = `<html><head>
<script type="application/ld+json">
{
  "@context": "https://schema.org",
  "@type": "Restaurant",
  "name": "Septime",
  "description": "Bertrand Grébaut's flagship in the 11th — set menus only.",
  "url": "https://lefooding.com/en/restaurant/septime",
  "address": {
    "@type": "PostalAddress",
    "streetAddress": "80 Rue de Charonne",
    "postalCode": "75011",
    "addressLocality": "Paris",
    "addressCountry": "FR"
  },
  "geo": {
    "@type": "GeoCoordinates",
    "latitude": 48.8527,
    "longitude": 2.3793
  }
}
</script>
<meta name="page:neighborhood" content="11ème arrondissement">
</head>
<body><h1>Septime</h1><p>Description fallback.</p></body></html>`

func newTestClient(srvURL string) *Client {
	c := New(nil, "test-ua")
	c.BaseURL = srvURL
	return c
}

func TestSearch_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.RawQuery, "q=septime") {
			t.Errorf("query missing q=septime: %q", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(fixtureSearchHTML))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	results, err := c.Search(context.Background(), "septime")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d (%+v)", len(results), results)
	}
	if results[0].Name != "Septime" {
		t.Errorf("name: %q", results[0].Name)
	}
	if results[0].City != "Paris" {
		t.Errorf("city: %q", results[0].City)
	}
	if !strings.Contains(results[0].Description, "rue de Charonne") {
		t.Errorf("description: %q", results[0].Description)
	}
	if !strings.HasSuffix(results[0].URL, "/en/restaurant/septime") {
		t.Errorf("url: %q", results[0].URL)
	}
}

func TestSearch_EmptyQuery(t *testing.T) {
	c := New(nil, "")
	if _, err := c.Search(context.Background(), "  "); err == nil {
		t.Fatal("expected error for empty query")
	}
}

func TestSearch_FallbackToQueryParam(t *testing.T) {
	var sawQ, sawQuery bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		if r.URL.Query().Get("q") != "" {
			sawQ = true
			// Empty body for the q= request → triggers fallback.
			_, _ = w.Write([]byte(`<html><body></body></html>`))
			return
		}
		if r.URL.Query().Get("query") != "" {
			sawQuery = true
			_, _ = w.Write([]byte(fixtureSearchHTML))
			return
		}
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	results, err := c.Search(context.Background(), "septime")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if !sawQ || !sawQuery {
		t.Errorf("expected both q and query params hit; q=%v query=%v", sawQ, sawQuery)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results from fallback, got %d", len(results))
	}
}

func TestRestaurant_ParsesJSONLD(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(fixtureRestaurantHTML))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	r, err := c.Restaurant(context.Background(), srv.URL+"/en/restaurant/septime")
	if err != nil {
		t.Fatalf("Restaurant: %v", err)
	}
	if r.Name != "Septime" {
		t.Errorf("name: %q", r.Name)
	}
	if !strings.Contains(r.Address, "80 Rue de Charonne") {
		t.Errorf("address: %q", r.Address)
	}
	if r.City != "Paris" {
		t.Errorf("city: %q", r.City)
	}
	if r.Lat == 0 || r.Lng == 0 {
		t.Errorf("geo missing: %+v", r)
	}
	if !strings.Contains(r.Description, "Bertrand") {
		t.Errorf("description: %q", r.Description)
	}
	if r.Neighborhood == "" {
		t.Errorf("neighborhood meta not picked up")
	}
}

func TestRestaurant_EmptyURL(t *testing.T) {
	c := New(nil, "")
	if _, err := c.Restaurant(context.Background(), ""); err == nil {
		t.Fatal("expected error for empty url")
	}
}

func TestRestaurant_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	if _, err := c.Restaurant(context.Background(), srv.URL+"/en/restaurant/missing"); err == nil {
		t.Fatal("expected 404 error")
	}
}
