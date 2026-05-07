package wikivoyage

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func fixtureClient(srvURL string) *Client {
	c := New("en", nil, "test-ua")
	c.RESTBase = srvURL + "/api/rest_v1"
	return c
}

func TestPageSummary_FlattensPageURL(t *testing.T) {
	var seenPath, seenUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.Path
		seenUA = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"title":"Tokyo",
			"extract":"Tokyo is the capital of Japan.",
			"extract_html":"<p>Tokyo is the capital of Japan.</p>",
			"coordinates":{"lat":35.6895,"lon":139.6917},
			"content_urls":{
				"desktop":{"page":"https://en.wikivoyage.org/wiki/Tokyo"},
				"mobile":{"page":"https://en.m.wikivoyage.org/wiki/Tokyo"}
			}
		}`))
	}))
	defer srv.Close()

	c := fixtureClient(srv.URL)
	sum, err := c.PageSummary(context.Background(), "Tokyo")
	if err != nil {
		t.Fatalf("PageSummary: %v", err)
	}
	if sum.Title != "Tokyo" {
		t.Errorf("title: got %q", sum.Title)
	}
	if !strings.Contains(sum.Extract, "capital of Japan") {
		t.Errorf("extract: %q", sum.Extract)
	}
	if sum.PageURL != "https://en.wikivoyage.org/wiki/Tokyo" {
		t.Errorf("page URL not flattened: %q", sum.PageURL)
	}
	if sum.Coordinates == nil || sum.Coordinates.Lat != 35.6895 {
		t.Errorf("coords: %+v", sum.Coordinates)
	}
	if !strings.HasSuffix(seenPath, "/page/summary/Tokyo") {
		t.Errorf("path: got %q", seenPath)
	}
	if seenUA == "" {
		t.Error("missing User-Agent header")
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
	if !strings.Contains(c.RESTBase, "en.wikivoyage.org") {
		t.Errorf("REST base: %q", c.RESTBase)
	}
	if c.UserAgent == "" {
		t.Error("expected default user agent")
	}
}

func TestNew_HonorsLocale(t *testing.T) {
	for _, loc := range []string{"en", "ja", "fr"} {
		c := New(loc, nil, "ua")
		want := "https://" + loc + ".wikivoyage.org/api/rest_v1"
		if c.RESTBase != want {
			t.Errorf("locale %q: got REST base %q, want %q", loc, c.RESTBase, want)
		}
	}
}
