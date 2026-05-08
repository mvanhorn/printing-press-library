package googleplaces

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNearbySearch_DropsPermanentlyClosed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Goog-Api-Key"); got != "test-key" {
			t.Errorf("X-Goog-Api-Key = %q, want test-key", got)
		}
		if got := r.Header.Get("X-Goog-FieldMask"); !strings.Contains(got, "places.businessStatus") {
			t.Errorf("X-Goog-FieldMask missing businessStatus: %q", got)
		}
		body, _ := io.ReadAll(r.Body)
		var req nearbyRequest
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("bad request body: %v", err)
		}
		if req.LocationRestriction.Circle.Radius != 500 {
			t.Errorf("radius = %v, want 500", req.LocationRestriction.Circle.Radius)
		}
		_, _ = w.Write([]byte(`{
			"places":[
				{"id":"a","displayName":{"text":"Open Spot"},"location":{"latitude":35.0,"longitude":139.0},"businessStatus":"OPERATIONAL"},
				{"id":"b","displayName":{"text":"Dead Spot"},"location":{"latitude":35.1,"longitude":139.1},"businessStatus":"CLOSED_PERMANENTLY"},
				{"id":"c","displayName":{"text":"On Vacation"},"location":{"latitude":35.2,"longitude":139.2},"businessStatus":"CLOSED_TEMPORARILY"}
			]
		}`))
	}))
	defer srv.Close()

	c := NewClientWithBase("test-key", srv.URL)
	out, err := c.NearbySearch(context.Background(), 35.0, 139.0, 500, []string{"cafe"}, 10, "en")
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Fatalf("got %d places, want 2 (CLOSED_PERMANENTLY filtered)", len(out))
	}
	if out[0].DisplayName != "Open Spot" {
		t.Errorf("first = %q, want Open Spot", out[0].DisplayName)
	}
	if out[1].BusinessStatus != "CLOSED_TEMPORARILY" {
		t.Errorf("second BusinessStatus = %q, want CLOSED_TEMPORARILY", out[1].BusinessStatus)
	}
}

func TestSearchText_RejectsEmpty(t *testing.T) {
	c := NewClientWithBase("k", "http://localhost")
	_, err := c.SearchText(context.Background(), "  ", 0, 0, 0, 10, "en")
	if err == nil {
		t.Error("empty query should error")
	}
}

func TestSearchText_PassesBias(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req textRequest
		_ = json.Unmarshal(body, &req)
		if req.LocationBias == nil {
			t.Error("LocationBias should be set when biasRadius>0")
		} else if req.LocationBias.Circle.Radius != 250 {
			t.Errorf("LocationBias radius = %v, want 250", req.LocationBias.Circle.Radius)
		}
		_, _ = w.Write([]byte(`{"places":[]}`))
	}))
	defer srv.Close()

	c := NewClientWithBase("k", srv.URL)
	_, err := c.SearchText(context.Background(), "kissaten", 35.0, 139.0, 250, 10, "ja")
	if err != nil {
		t.Fatal(err)
	}
}

func TestNewClient_RequiresKey(t *testing.T) {
	t.Setenv(EnvVar, "")
	if _, err := NewClient(); err == nil {
		t.Error("NewClient should error when env var is unset")
	}
	t.Setenv(EnvVar, "k")
	if _, err := NewClient(); err != nil {
		t.Errorf("NewClient with key set should succeed: %v", err)
	}
}

func TestPost_AuthFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"PERMISSION_DENIED"}`))
	}))
	defer srv.Close()

	c := NewClientWithBase("k", srv.URL)
	_, err := c.NearbySearch(context.Background(), 35.0, 139.0, 500, nil, 10, "en")
	if err == nil || !strings.Contains(err.Error(), "auth failed") {
		t.Errorf("expected auth-failed error, got %v", err)
	}
}
