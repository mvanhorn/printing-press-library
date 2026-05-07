package navermap

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const fixtureSuccessJSON = `{
  "place": [
    {
      "id": "1234567",
      "name": "광장시장",
      "address": "서울 종로구 창경궁로 88",
      "category": "전통시장",
      "x": "126.999846",
      "y": "37.570316"
    },
    {
      "id": "9876543",
      "name": "Tosokchon Samgyetang",
      "address": "5 Jahamun-ro 5-gil, Jongno-gu, Seoul",
      "category": "Korean restaurant",
      "x": 126.97,
      "y": 37.578
    }
  ]
}`

func newTestClient(srvURL string) *Client {
	c := New(nil, "test-ua")
	c.BaseURL = srvURL
	return c
}

func TestSearch_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("query") != "광장시장" {
			t.Errorf("query: %q", r.URL.Query().Get("query"))
		}
		if r.URL.Query().Get("type") != "all" {
			t.Errorf("type: %q", r.URL.Query().Get("type"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fixtureSuccessJSON))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	places, err := c.Search(context.Background(), "광장시장")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(places) != 2 {
		t.Fatalf("expected 2 places, got %d (%+v)", len(places), places)
	}
	if places[0].ID != "1234567" || places[0].Name != "광장시장" {
		t.Errorf("first: %+v", places[0])
	}
	if places[0].Lat <= 37.0 || places[0].Lat >= 38.0 {
		t.Errorf("first lat (string-coord): %v", places[0].Lat)
	}
	if places[0].Lng <= 126.0 || places[0].Lng >= 128.0 {
		t.Errorf("first lng (string-coord): %v", places[0].Lng)
	}
	if places[1].Lat == 0 || places[1].Lng == 0 {
		t.Errorf("second numeric coord lost: %+v", places[1])
	}
}

func TestSearch_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"login required"}`))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	_, err := c.Search(context.Background(), "광장시장")
	if !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("expected ErrUnauthenticated, got %v", err)
	}
}

func TestSearch_Forbidden(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	_, err := c.Search(context.Background(), "광장시장")
	if !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("expected ErrUnauthenticated for 403, got %v", err)
	}
}

func TestSearch_EmptyQuery(t *testing.T) {
	c := New(nil, "")
	if _, err := c.Search(context.Background(), "  "); err == nil {
		t.Fatal("expected error for empty query (no HTTP call)")
	}
}

func TestSearch_PassesCookieWhenSet(t *testing.T) {
	var seenCookie string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenCookie = r.Header.Get("Cookie")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"place":[]}`))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	c.Cookie = "NID_AUT=abc; NID_SES=def"
	if _, err := c.Search(context.Background(), "x"); err != nil {
		t.Fatalf("Search: %v", err)
	}
	if !strings.Contains(seenCookie, "NID_AUT=abc") {
		t.Errorf("cookie not forwarded: %q", seenCookie)
	}
}

func TestSearch_OtherError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	_, err := c.Search(context.Background(), "x")
	if err == nil {
		t.Fatal("expected error")
	}
	if errors.Is(err, ErrUnauthenticated) {
		t.Errorf("500 should not be ErrUnauthenticated: %v", err)
	}
}
