package lefooding

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/sourcetypes"
)

func TestSlug(t *testing.T) {
	c := NewClient()
	if c.Slug() != "lefooding" || c.Locale() != "fr" || c.IsStub() {
		t.Errorf("unexpected")
	}
	var _ sourcetypes.Client = c
}

func TestExtract(t *testing.T) {
	html := `<a href="/fr/paris/restaurant-le-bistrot">Le Bistrot</a>`
	hits := extractLeFoodingHits(html, "https://www.lefooding.com", 5)
	if len(hits) != 1 || hits[0].Title != "Le Bistrot" {
		t.Errorf("got %+v", hits)
	}
}

func TestHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<a href="/fr/paris/le-test">Le Test</a>`))
	}))
	defer srv.Close()
	c := NewClientWithBase(srv.URL, time.Millisecond)
	hits, err := c.LookupByName(context.Background(), "Le Test", "Paris", 5)
	if err != nil || len(hits) != 1 {
		t.Fatalf("err=%v hits=%v", err, hits)
	}
}

func TestCheckClosed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<p>Fermé définitivement depuis 2024.</p>`))
	}))
	defer srv.Close()
	c := NewClientWithBase(srv.URL, time.Millisecond)
	v := c.CheckClosed(context.Background(), sourcetypes.Hit{URL: srv.URL + "/fr/paris/test"})
	if !v.Closed {
		t.Errorf("expected Closed, got %+v", v)
	}
}
