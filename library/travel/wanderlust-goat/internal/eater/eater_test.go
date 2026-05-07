package eater

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const itemListHTML = `<!doctype html>
<html><head>
<script type="application/ld+json">
{
  "@context":"https://schema.org",
  "@type":"ItemList",
  "itemListElement":[
    {"@type":"ListItem","position":1,"item":{
      "@type":"Restaurant",
      "name":"Sushi Saito",
      "url":"https://www.eater.com/places/sushi-saito",
      "description":"World-class omakase. Worth the splurge.",
      "address":{"@type":"PostalAddress","streetAddress":"1-4-5 Akasaka","addressLocality":"Tokyo","addressCountry":"JP"},
      "geo":{"@type":"GeoCoordinates","latitude":35.6735,"longitude":139.7378}
    }},
    {"@type":"ListItem","position":2,"item":{
      "@type":"Restaurant",
      "name":"Den",
      "address":"2-3-18 Jingumae, Shibuya, Tokyo",
      "geo":{"latitude":"35.6694","longitude":"139.7099"},
      "description":"Playful kaiseki."
    }}
  ]
}
</script>
</head><body></body></html>`

func TestBestOf_ItemListShape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(itemListHTML))
	}))
	defer srv.Close()

	c := New(nil, "test-ua")
	listURL := srv.URL + "/maps/best-restaurants-tokyo"
	rs, err := c.BestOf(context.Background(), listURL)
	if err != nil {
		t.Fatalf("BestOf: %v", err)
	}
	if len(rs) != 2 {
		t.Fatalf("expected 2 restaurants, got %d", len(rs))
	}
	if rs[0].Name != "Sushi Saito" {
		t.Errorf("first name: %q", rs[0].Name)
	}
	if rs[0].Lat != 35.6735 {
		t.Errorf("first lat: %v", rs[0].Lat)
	}
	if !strings.Contains(rs[0].Address, "Akasaka") {
		t.Errorf("first address: %q", rs[0].Address)
	}
	if rs[0].Description != "World-class omakase." {
		t.Errorf("description should be first sentence: %q", rs[0].Description)
	}
	if rs[0].ListURL != listURL {
		t.Errorf("list url not propagated: %q", rs[0].ListURL)
	}
	if rs[0].ItemURL == "" {
		t.Error("item url missing")
	}
	if rs[1].Lat == 0 || rs[1].Lng == 0 {
		t.Errorf("string-encoded geo should parse: %+v", rs[1])
	}
}

const inlineRestaurantsHTML = `<!doctype html>
<html><head>
<script type="application/ld+json">
{"@type":"Restaurant","name":"Restaurant A","address":"Addr A","geo":{"latitude":1.0,"longitude":2.0}}
</script>
<script type="application/ld+json">
[
 {"@type":"Restaurant","name":"Restaurant B","address":"Addr B","geo":{"latitude":3.0,"longitude":4.0}},
 {"@type":"Restaurant","name":"Restaurant C","address":"Addr C","geo":{"latitude":5.0,"longitude":6.0}}
]
</script>
</head></html>`

func TestBestOf_InlineRestaurants(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(inlineRestaurantsHTML))
	}))
	defer srv.Close()

	c := New(nil, "test-ua")
	rs, err := c.BestOf(context.Background(), srv.URL+"/maps/best-x")
	if err != nil {
		t.Fatalf("BestOf: %v", err)
	}
	if len(rs) != 3 {
		t.Fatalf("expected 3 restaurants, got %d", len(rs))
	}
	names := []string{rs[0].Name, rs[1].Name, rs[2].Name}
	wantNames := []string{"Restaurant A", "Restaurant B", "Restaurant C"}
	for i := range names {
		if names[i] != wantNames[i] {
			t.Errorf("name[%d]: got %q, want %q", i, names[i], wantNames[i])
		}
	}
}

func TestBestOf_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`forbidden`))
	}))
	defer srv.Close()

	c := New(nil, "test-ua")
	_, err := c.BestOf(context.Background(), srv.URL+"/maps/x")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Errorf("error should mention 403: %v", err)
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
}
