package michelin

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func fixtureClient(srvURL string) *Client {
	c := New(nil, "test-ua")
	c.BaseURL = srvURL
	return c
}

const regionHTML = `<!doctype html>
<html><head>
<script type="application/ld+json">
{
  "@context":"https://schema.org",
  "@type":"ItemList",
  "itemListElement":[
    {"@type":"ListItem","position":1,"item":{
      "@type":"Restaurant",
      "name":"Sukiyabashi Jiro",
      "url":"https://guide.michelin.com/en/tokyo-region/tokyo/restaurant/sukiyabashi-jiro-honten",
      "description":"Three-star sushi.",
      "address":{"@type":"PostalAddress","streetAddress":"4-2-15 Ginza","addressLocality":"Tokyo"},
      "geo":{"latitude":35.6717,"longitude":139.7639}
    }},
    {"@type":"ListItem","position":2,"item":{
      "@type":"Restaurant",
      "name":"Narisawa",
      "address":"2-6-15 Minami-Aoyama, Tokyo",
      "geo":{"latitude":"35.6628","longitude":"139.7196"}
    }}
  ]
}
</script>
</head></html>`

func TestRestaurantsForRegion_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/en/tokyo-region/restaurants") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(regionHTML))
	}))
	defer srv.Close()

	c := fixtureClient(srv.URL)
	got, err := c.RestaurantsForRegion(context.Background(), "tokyo-region")
	if err != nil {
		t.Fatalf("RestaurantsForRegion: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2, got %d", len(got))
	}
	if got[0].Name != "Sukiyabashi Jiro" {
		t.Errorf("first name: %q", got[0].Name)
	}
	if got[0].Lat == 0 {
		t.Errorf("first lat: %v", got[0].Lat)
	}
	if !strings.Contains(got[0].Address, "Ginza") {
		t.Errorf("first address: %q", got[0].Address)
	}
	if got[1].Lat == 0 {
		t.Errorf("string-encoded geo should parse: %+v", got[1])
	}
}

func TestRestaurantsForRegion_AWSChallengeBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`<!doctype html><html><head><title>Action Required</title></head><body>aws-waf-token blocked</body></html>`))
	}))
	defer srv.Close()

	c := fixtureClient(srv.URL)
	_, err := c.RestaurantsForRegion(context.Background(), "tokyo-region")
	if !errors.Is(err, ErrChallenged) {
		t.Fatalf("expected ErrChallenged, got %v", err)
	}
}

func TestRestaurantsForRegion_AWSChallengeHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-amzn-waf-action", "challenge")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<html></html>`))
	}))
	defer srv.Close()

	c := fixtureClient(srv.URL)
	_, err := c.RestaurantsForRegion(context.Background(), "tokyo-region")
	if !errors.Is(err, ErrChallenged) {
		t.Fatalf("expected ErrChallenged, got %v", err)
	}
}

func TestRestaurant_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><head>
<script type="application/ld+json">
{"@type":"Restaurant","name":"Den","address":"Tokyo","geo":{"latitude":35.6694,"longitude":139.7099},"description":"Playful kaiseki."}
</script></head></html>`))
	}))
	defer srv.Close()

	c := fixtureClient(srv.URL)
	r, err := c.Restaurant(context.Background(), srv.URL+"/en/tokyo/restaurant/den")
	if err != nil {
		t.Fatalf("Restaurant: %v", err)
	}
	if r.Name != "Den" {
		t.Errorf("name: %q", r.Name)
	}
	if r.URL == "" {
		t.Error("URL should fall back to input URL")
	}
}

func TestRestaurant_NoJSONLD(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html></html>`))
	}))
	defer srv.Close()

	c := fixtureClient(srv.URL)
	_, err := c.Restaurant(context.Background(), srv.URL+"/x")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNew_Defaults(t *testing.T) {
	c := New(nil, "")
	if c.UserAgent == "" {
		t.Error("expected default UA")
	}
	if c.HTTPClient == nil {
		t.Error("expected default HTTP client")
	}
	if c.BaseURL == "" {
		t.Error("expected default BaseURL")
	}
}
