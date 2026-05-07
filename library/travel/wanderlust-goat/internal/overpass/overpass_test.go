package overpass

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNearbyByTags_BuildsQL(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/x-www-form-urlencoded") {
			t.Fatalf("unexpected Content-Type: %s", ct)
		}
		if ua := r.Header.Get("User-Agent"); ua == "" {
			t.Fatal("missing User-Agent")
		}
		body, _ := io.ReadAll(r.Body)
		got = string(body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"elements":[]}`))
	}))
	defer srv.Close()

	c := New(nil, "")
	c.Endpoint = srv.URL

	_, err := c.NearbyByTags(context.Background(), 35.6895, 139.6917, 800, []TagFilter{
		{Key: "amenity", Value: "cafe"},
		{Key: "tourism"},
	})
	if err != nil {
		t.Fatalf("NearbyByTags: %v", err)
	}

	// Form-encoded body has data=<urlencoded ql>; check that decoded substrings appear.
	if !strings.HasPrefix(got, "data=") {
		t.Fatalf("body did not start with data=: %q", got)
	}
	// The encoded body should contain (URL-encoded) markers from the QL.
	for _, want := range []string{
		"out%3Ajson", // [out:json]
		"timeout%3A25",
		"amenity%3Dcafe",                  // [amenity=cafe]
		"%5Btourism%5D",                   // [tourism]
		"around%3A800%2C35.689500",        // around:800,35.689500
		"out+center+tags+50",              // out center tags 50
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expected body to contain %q; full body: %s", want, got)
		}
	}
}

func TestNearbyByTags_ParsesResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"version": 0.6,
			"elements": [
				{"type":"node","id":1,"lat":35.6895,"lon":139.6917,"tags":{"amenity":"cafe","name":"Cafe One"}},
				{"type":"way","id":2,"center":{"lat":35.6900,"lon":139.6920},"tags":{"tourism":"museum","name":"Way Museum"}}
			]
		}`))
	}))
	defer srv.Close()

	c := New(nil, "test-ua")
	c.Endpoint = srv.URL

	resp, err := c.NearbyByTags(context.Background(), 35.6895, 139.6917, 500, []TagFilter{{Key: "amenity", Value: "cafe"}})
	if err != nil {
		t.Fatalf("NearbyByTags: %v", err)
	}
	if len(resp.Elements) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(resp.Elements))
	}

	node := resp.Elements[0]
	if node.Type != "node" || node.ID != 1 || node.Tags["name"] != "Cafe One" {
		t.Errorf("unexpected node: %+v", node)
	}
	if lat, lon := node.LatLng(); lat != 35.6895 || lon != 139.6917 {
		t.Errorf("node LatLng: got (%f,%f)", lat, lon)
	}

	way := resp.Elements[1]
	if way.Type != "way" || way.ID != 2 {
		t.Errorf("unexpected way: %+v", way)
	}
	if way.Center == nil {
		t.Fatal("way Center missing")
	}
	if lat, lon := way.LatLng(); lat != 35.6900 || lon != 139.6920 {
		t.Errorf("way LatLng: got (%f,%f), want (35.6900, 139.6920)", lat, lon)
	}
}

func TestQuery_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`rate limited`))
	}))
	defer srv.Close()

	c := New(nil, "")
	c.Endpoint = srv.URL

	_, err := c.Query(context.Background(), "[out:json];out;")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "429") {
		t.Errorf("error should mention 429: %v", err)
	}
	if !strings.Contains(err.Error(), srv.URL) {
		t.Errorf("error should include URL: %v", err)
	}
}

func TestBuildNearbyQL_Shape(t *testing.T) {
	q := buildNearbyQL(1.0, 2.0, 100, []TagFilter{{Key: "amenity", Value: "cafe"}})
	for _, want := range []string{
		"[out:json][timeout:25];",
		"node[amenity=cafe](around:100,1.000000,2.000000);",
		"way[amenity=cafe](around:100,1.000000,2.000000);",
		"out center tags 50;",
	} {
		if !strings.Contains(q, want) {
			t.Errorf("QL missing %q; got:\n%s", want, q)
		}
	}
}
