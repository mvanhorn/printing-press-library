package timeout

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
      "@type":"LocalBusiness",
      "name":"Borough Market",
      "url":"https://www.timeout.com/london/restaurants/borough-market",
      "description":"London's most-loved food market. Worth a wander.",
      "address":{"@type":"PostalAddress","streetAddress":"8 Southwark St","addressLocality":"London","postalCode":"SE1 1TL"},
      "geo":{"latitude":51.5055,"longitude":-0.0905}
    }},
    {"@type":"ListItem","position":2,"item":{
      "@type":"Restaurant",
      "name":"Padella",
      "address":"6 Southwark St, London",
      "geo":{"latitude":"51.5051","longitude":"-0.0901"}
    }}
  ]
}
</script>
</head></html>`

func TestBestOf_ItemList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(itemListHTML))
	}))
	defer srv.Close()

	c := New(nil, "test-ua")
	listURL := srv.URL + "/london/restaurants/best-restaurants-in-london"
	got, err := c.BestOf(context.Background(), listURL)
	if err != nil {
		t.Fatalf("BestOf: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2, got %d", len(got))
	}
	if got[0].Name != "Borough Market" {
		t.Errorf("first name: %q", got[0].Name)
	}
	if got[0].Lat == 0 {
		t.Errorf("first lat missing: %+v", got[0])
	}
	if !strings.Contains(got[0].Address, "Southwark") {
		t.Errorf("first address: %q", got[0].Address)
	}
	if got[0].Description != "London's most-loved food market." {
		t.Errorf("description should be first sentence: %q", got[0].Description)
	}
	if got[0].ListURL != listURL {
		t.Errorf("list url not propagated: %q", got[0].ListURL)
	}
	if got[1].Lat == 0 {
		t.Errorf("string-encoded geo should parse: %+v", got[1])
	}
}

const inlineHTML = `<!doctype html>
<html><body>
<script type="application/ld+json">
{"@type":"Restaurant","name":"Solo","address":"Addr A","geo":{"latitude":1,"longitude":2}}
</script>
<script type="application/ld+json">
{"@type":"BarOrPub","name":"Pub","address":"Addr B","geo":{"latitude":3,"longitude":4}}
</script>
</body></html>`

func TestBestOf_InlineBlocks(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(inlineHTML))
	}))
	defer srv.Close()

	c := New(nil, "test-ua")
	got, err := c.BestOf(context.Background(), srv.URL+"/x")
	if err != nil {
		t.Fatalf("BestOf: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2, got %d", len(got))
	}
}

func TestBestOf_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	c := New(nil, "test-ua")
	_, err := c.BestOf(context.Background(), srv.URL+"/x")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "502") {
		t.Errorf("error should mention 502: %v", err)
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
