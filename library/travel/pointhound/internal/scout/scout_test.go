package scout

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSearch_RequiresQuery(t *testing.T) {
	c := New("")
	_, err := c.Search(context.Background(), SearchOptions{})
	if err == nil {
		t.Fatal("expected error for empty query")
	}
	if !strings.Contains(err.Error(), "query is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSearch_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/places/search" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if q := r.URL.Query().Get("q"); q != "lis" {
			t.Errorf("unexpected query: %s", q)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"searchStatus": "ok",
			"results": [
				{"code":"LIS","name":"Lisbon Portela","city":"Lisbon","dealRating":"high","isTracked":true}
			]
		}`))
	}))
	defer srv.Close()

	c := New(srv.URL)
	resp, err := c.Search(context.Background(), SearchOptions{Query: "lis", Limit: 5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(resp.Results))
	}
	if resp.Results[0].Code != "LIS" {
		t.Errorf("unexpected code: %s", resp.Results[0].Code)
	}
	if !resp.Results[0].IsTracked {
		t.Errorf("expected IsTracked=true")
	}
}

func TestSearch_RateLimited(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	c := New(srv.URL)
	_, err := c.Search(context.Background(), SearchOptions{Query: "lis"})
	if err == nil {
		t.Fatal("expected rate-limit error")
	}
	if !strings.Contains(err.Error(), "rate limited") {
		t.Fatalf("error should mention rate limited, got: %v", err)
	}
	if !strings.Contains(err.Error(), "30") {
		t.Fatalf("error should include Retry-After, got: %v", err)
	}
}

func TestSearch_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := New(srv.URL)
	_, err := c.Search(context.Background(), SearchOptions{Query: "lis"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "HTTP 500") {
		t.Fatalf("error should mention HTTP 500, got: %v", err)
	}
}

func TestNew_DefaultBaseURL(t *testing.T) {
	c := New("")
	if c.BaseURL != "https://scout.pointhound.com" {
		t.Errorf("unexpected default base URL: %s", c.BaseURL)
	}
	cCustom := New("https://example.test")
	if cCustom.BaseURL != "https://example.test" {
		t.Errorf("custom base URL not preserved: %s", cCustom.BaseURL)
	}
}
