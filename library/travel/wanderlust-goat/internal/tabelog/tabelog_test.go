package tabelog

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const fixtureListingHTML = `<html><body>
<ul class="list-rst-list">
  <li class="list-rst">
    <a class="list-rst__rst-name-target" href="https://tabelog.com/en/tokyo/A1301/A130101/13000001/">
      Sushi Saito
    </a>
    <p class="list-rst__area-genre">Roppongi / Sushi</p>
    <span class="list-rst__rating-val">4.31</span>
  </li>
  <li class="list-rst">
    <a class="list-rst__rst-name-target" href="https://tabelog.com/en/tokyo/A1301/A130101/13000002/">
      Casual Diner
    </a>
    <p class="list-rst__area-genre">Tokyo / Diner</p>
    <span class="list-rst__rating-val">3.20</span>
  </li>
  <li class="list-rst">
    <a class="list-rst__rst-name-target" href="https://tabelog.com/en/tokyo/A1301/A130101/13000003/">
      Mid Tier Coffee
    </a>
    <p class="list-rst__area-genre">Shibuya / Cafe</p>
    <span class="list-rst__rating-val">3.55</span>
  </li>
</ul>
</body></html>`

const fixtureDetailHTML = `<html><head>
<script type="application/ld+json">
{
  "@context":"http://schema.org",
  "@type":"Restaurant",
  "name":"Sushi Saito",
  "alternateName":"鮨 さいとう",
  "url":"https://tabelog.com/en/tokyo/A1301/A130101/13000001/",
  "address":{
    "@type":"PostalAddress",
    "streetAddress":"1-4-5 Roppongi",
    "addressLocality":"Minato-ku",
    "addressRegion":"Tokyo",
    "postalCode":"106-0032"
  },
  "priceRange":"¥¥¥¥",
  "servesCuisine":["Sushi"],
  "aggregateRating":{
    "@type":"AggregateRating",
    "ratingValue":"4.31",
    "reviewCount":"412"
  },
  "geo":{
    "@type":"GeoCoordinates",
    "latitude":35.6627,
    "longitude":139.7314
  }
}
</script>
</head><body><h1>Sushi Saito</h1></body></html>`

func newTestClient(srvURL string) *Client {
	c := New(nil, "test-ua")
	c.BaseURL = srvURL
	return c
}

func TestRestaurantsByPrefecture_FiltersUnderHighQualityBar(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/en/tokyo/cuisine/sushi") {
			t.Errorf("path: %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(fixtureListingHTML))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	results, err := c.RestaurantsByPrefecture(context.Background(), "tokyo", "sushi")
	if err != nil {
		t.Fatalf("RestaurantsByPrefecture: %v", err)
	}
	// Saito (4.31) and Mid Tier Coffee (3.55) should pass; Casual Diner (3.20) drops.
	if len(results) != 2 {
		t.Fatalf("expected 2 high-quality, got %d (%+v)", len(results), results)
	}
	for _, r := range results {
		if r.Rating < MinHighQualityRating {
			t.Errorf("rating below bar slipped through: %+v", r)
		}
	}
	if results[0].Cuisine != "Sushi" {
		t.Errorf("cuisine: %q", results[0].Cuisine)
	}
	if results[0].Address != "Roppongi" {
		t.Errorf("address: %q", results[0].Address)
	}
}

func TestRestaurantsByPrefecture_NoCuisineSlug(t *testing.T) {
	var seenPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.Path
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body></body></html>`))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	if _, err := c.RestaurantsByPrefecture(context.Background(), "tokyo", ""); err != nil {
		t.Fatalf("RestaurantsByPrefecture: %v", err)
	}
	if seenPath != "/en/tokyo/" {
		t.Errorf("path: %q", seenPath)
	}
}

func TestRestaurantsByPrefecture_EmptySlug(t *testing.T) {
	c := New(nil, "")
	if _, err := c.RestaurantsByPrefecture(context.Background(), "  ", "sushi"); err == nil {
		t.Fatal("expected error for empty prefecture")
	}
}

func TestRestaurant_ParsesJSONLD(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(fixtureDetailHTML))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	r, err := c.Restaurant(context.Background(), srv.URL+"/en/tokyo/A1301/A130101/13000001/")
	if err != nil {
		t.Fatalf("Restaurant: %v", err)
	}
	if r.Name != "Sushi Saito" {
		t.Errorf("name: %q", r.Name)
	}
	if r.NameLocal != "鮨 さいとう" {
		t.Errorf("name local: %q", r.NameLocal)
	}
	if !strings.Contains(r.Address, "Roppongi") {
		t.Errorf("address: %q", r.Address)
	}
	if r.Rating != 4.31 {
		t.Errorf("rating: %v", r.Rating)
	}
	if r.RatingCount != 412 {
		t.Errorf("rating count: %v", r.RatingCount)
	}
	if r.Lat == 0 || r.Lng == 0 {
		t.Errorf("geo: %+v", r)
	}
	if r.Cuisine != "Sushi" {
		t.Errorf("cuisine: %q", r.Cuisine)
	}
	if r.PriceRange != "¥¥¥¥" {
		t.Errorf("price range: %q", r.PriceRange)
	}
}

func TestRestaurant_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	if _, err := c.Restaurant(context.Background(), srv.URL+"/en/tokyo/x"); err == nil {
		t.Fatal("expected error")
	}
}

func TestRestaurant_EmptyURL(t *testing.T) {
	c := New(nil, "")
	if _, err := c.Restaurant(context.Background(), ""); err == nil {
		t.Fatal("expected error")
	}
}
