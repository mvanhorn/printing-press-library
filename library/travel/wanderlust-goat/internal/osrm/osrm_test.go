package osrm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const canonicalOSRMResponse = `{
	"code":"Ok",
	"routes":[{
		"distance":1234.5,
		"duration":600.0,
		"geometry":{"type":"LineString","coordinates":[[139.6917,35.6895],[139.7,35.69]]}
	}],
	"waypoints":[]
}`

func TestWalkingRoute_HappyPath(t *testing.T) {
	var seenPath, seenQuery, seenUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.Path
		seenQuery = r.URL.RawQuery
		seenUA = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(canonicalOSRMResponse))
	}))
	defer srv.Close()

	c := New(srv.URL, nil, "")
	res, err := c.WalkingRoute(context.Background(), 35.6895, 139.6917, 35.69, 139.7)
	if err != nil {
		t.Fatalf("WalkingRoute: %v", err)
	}

	if res.DistanceMeters != 1234.5 {
		t.Errorf("distance: got %v", res.DistanceMeters)
	}
	if res.DurationSeconds != 600.0 {
		t.Errorf("duration: got %v", res.DurationSeconds)
	}
	if res.WalkingMinutes != 10.0 {
		t.Errorf("minutes: got %v, want 10", res.WalkingMinutes)
	}

	// URL path is /route/v1/foot/<from_lng>,<from_lat>;<to_lng>,<to_lat>
	wantPathPrefix := "/route/v1/foot/139.691700,35.689500;139.700000,35.690000"
	if seenPath != wantPathPrefix {
		t.Errorf("path: got %q, want %q", seenPath, wantPathPrefix)
	}
	if !strings.Contains(seenQuery, "overview=false") || !strings.Contains(seenQuery, "geometries=geojson") {
		t.Errorf("query missing expected params: %q", seenQuery)
	}
	if seenUA == "" {
		t.Error("missing User-Agent")
	}
}

func TestWalkingPolyline_ReturnsCoords(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.RawQuery, "overview=full") {
			t.Errorf("polyline call should request overview=full; got %q", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(canonicalOSRMResponse))
	}))
	defer srv.Close()

	c := New(srv.URL, nil, "ua")
	res, err := c.WalkingPolyline(context.Background(), 35.6895, 139.6917, 35.69, 139.7)
	if err != nil {
		t.Fatalf("WalkingPolyline: %v", err)
	}
	if len(res.Coords) != 2 {
		t.Fatalf("coords: want 2, got %d", len(res.Coords))
	}
	if res.Coords[0][0] != 139.6917 || res.Coords[0][1] != 35.6895 {
		t.Errorf("first coord: got %v", res.Coords[0])
	}
	if res.WalkingMinutes != 10.0 {
		t.Errorf("minutes: got %v", res.WalkingMinutes)
	}
}

func TestWalkingRoute_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`upstream error`))
	}))
	defer srv.Close()

	c := New(srv.URL, nil, "")
	_, err := c.WalkingRoute(context.Background(), 0, 0, 0, 0)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "502") {
		t.Errorf("error should mention 502: %v", err)
	}
}

func TestWalkingRoute_NonOkCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":"NoRoute","message":"no path","routes":[]}`))
	}))
	defer srv.Close()

	c := New(srv.URL, nil, "")
	_, err := c.WalkingRoute(context.Background(), 0, 0, 0, 0)
	if err == nil {
		t.Fatal("expected error for code != Ok")
	}
	if !strings.Contains(err.Error(), "NoRoute") {
		t.Errorf("error should include code: %v", err)
	}
}
