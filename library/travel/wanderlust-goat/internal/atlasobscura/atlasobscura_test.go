package atlasobscura

import (
	"context"
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

const cityHTML = `<!doctype html>
<html><head><title>Tokyo</title>
<script type="application/ld+json">
{
  "@context":"https://schema.org",
  "@type":"ItemList",
  "itemListElement":[
    {"@type":"ListItem","position":1,"item":{
      "@type":"Place",
      "name":"Nakano Broadway",
      "url":"https://www.atlasobscura.com/places/nakano-broadway",
      "description":"A geek paradise in Tokyo.",
      "geo":{"@type":"GeoCoordinates","latitude":35.7076,"longitude":139.6657},
      "image":"https://example.com/nakano.jpg"
    }},
    {"@type":"ListItem","position":2,"item":{
      "@type":"TouristAttraction",
      "name":"Golden Gai",
      "url":"https://www.atlasobscura.com/places/golden-gai",
      "shortDescription":"Cluster of tiny bars.",
      "geo":{"@type":"GeoCoordinates","latitude":"35.6938","longitude":"139.7036"}
    }}
  ]
}
</script></head><body>placeholder</body></html>`

func TestCity_ParsesJSONLDItemList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/things-to-do/tokyo-japan") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(cityHTML))
	}))
	defer srv.Close()

	c := fixtureClient(srv.URL)
	entries, err := c.City(context.Background(), "tokyo-japan")
	if err != nil {
		t.Fatalf("City: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Title != "Nakano Broadway" || entries[0].Lat != 35.7076 {
		t.Errorf("first entry: %+v", entries[0])
	}
	if entries[0].ImageURL == "" {
		t.Error("expected image url")
	}
	// String-encoded numbers should still parse.
	if entries[1].Lat == 0 || entries[1].Lng == 0 {
		t.Errorf("string-encoded geo should parse: %+v", entries[1])
	}
	if entries[1].Description != "Cluster of tiny bars." {
		t.Errorf("shortDescription should populate Description: %q", entries[1].Description)
	}
}

const linkCardHTML = `<!doctype html>
<html><body>
<a class="js-LinkCard__link Card_card__link" href="/places/sample-one">Sample One</a>
<a class="some-other-class" href="/places/should-skip">Skip</a>
<a class="js-LinkCard__link" href="https://www.atlasobscura.com/places/sample-two"><span>Sample Two</span></a>
</body></html>`

func TestCity_FallsBackToLinkCards(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(linkCardHTML))
	}))
	defer srv.Close()

	c := fixtureClient(srv.URL)
	entries, err := c.City(context.Background(), "nowhere")
	if err != nil {
		t.Fatalf("City: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 link-card entries, got %d", len(entries))
	}
	if entries[0].Title != "Sample One" {
		t.Errorf("first title: %q", entries[0].Title)
	}
	if entries[1].Title != "Sample Two" {
		t.Errorf("second title: %q", entries[1].Title)
	}
	if !strings.HasPrefix(entries[0].URL, "https://www.atlasobscura.com/places/") {
		t.Errorf("relative href should be absolutized: %q", entries[0].URL)
	}
}

const entryHTML = `<!doctype html>
<html><head>
<script type="application/ld+json">
{
  "@context":"https://schema.org",
  "@type":"Place",
  "name":"Robot Restaurant",
  "url":"https://www.atlasobscura.com/places/robot-restaurant",
  "description":"Neon spectacle.",
  "geo":{"latitude":35.6951,"longitude":139.7016},
  "image":["https://example.com/a.jpg","https://example.com/b.jpg"]
}
</script>
</head><body></body></html>`

func TestEntry_ParsesPlace(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(entryHTML))
	}))
	defer srv.Close()

	c := fixtureClient(srv.URL)
	e, err := c.Entry(context.Background(), srv.URL+"/places/robot-restaurant")
	if err != nil {
		t.Fatalf("Entry: %v", err)
	}
	if e.Title != "Robot Restaurant" {
		t.Errorf("title: %q", e.Title)
	}
	if e.Lat != 35.6951 {
		t.Errorf("lat: %v", e.Lat)
	}
	if e.ImageURL != "https://example.com/a.jpg" {
		t.Errorf("image: %q", e.ImageURL)
	}
}

func TestEntry_NoJSONLDReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body>plain</body></html>`))
	}))
	defer srv.Close()

	c := fixtureClient(srv.URL)
	_, err := c.Entry(context.Background(), srv.URL+"/places/none")
	if err == nil {
		t.Fatal("expected error when JSON-LD absent")
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
		t.Error("expected default base URL")
	}
}
