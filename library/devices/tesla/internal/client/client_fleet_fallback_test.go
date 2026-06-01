package client

import (
	"bytes"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/tesla/internal/config"
)

// routingRoundTripper answers per-host: the owner-api host 404s vehicle reads
// (a 2021+/non-NA car it can't serve), the Fleet host serves a nested
// vehicle_data envelope. It records every (host, authorization) pair so tests
// can assert the fallback switched base URL and bearer.
type routingRoundTripper struct {
	ownerHost, fleetHost string
	hits                 []string // "host auth"
}

func (r *routingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	r.hits = append(r.hits, req.URL.Host+" "+req.Header.Get("Authorization"))
	mk := func(code int, body string) (*http.Response, error) {
		return &http.Response{
			StatusCode: code,
			Body:       io.NopCloser(bytes.NewReader([]byte(body))),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	}
	switch req.URL.Host {
	case r.fleetHost:
		return mk(200, `{"response":{"charge_state":{"battery_level":71},"vehicle_id":1}}`)
	default:
		return mk(404, `{"error":"Not Found"}`)
	}
}

func newFallbackClient(t *testing.T, rt *routingRoundTripper) *Client {
	t.Helper()
	cfg := &config.Config{
		BaseURL:     "http://" + rt.ownerHost,
		AccessToken: "owner-tok",
		TokenExpiry: time.Now().Add(time.Hour), // a *usable* owner token
	}
	cfg.Fleet.AccessToken = "fleet-tok"
	c := New(cfg, time.Second, 0)
	c.HTTPClient = &http.Client{Transport: rt}
	c.NoCache = true
	c.FleetFallback = true
	c.FleetBaseURL = "http://" + rt.fleetHost
	return c
}

// TestFleetFallback_OwnerVehicleRead404 pins the reactive owner-api-404 -> Fleet
// retry for a vehicle read: owner-api 404s, the client switches to the Fleet
// base + Fleet bearer, rewrites the subset path, and unwraps the nested Fleet
// response back to the owner-api shape.
func TestFleetFallback_OwnerVehicleRead404(t *testing.T) {
	rt := &routingRoundTripper{ownerHost: "owner.test", fleetHost: "fleet.test"}
	c := newFallbackClient(t, rt)

	body, status, err := c.do("GET", "/api/1/vehicles/VIN123/data_request/charge_state", nil, nil, nil)
	if err != nil {
		t.Fatalf("expected fallback success, got error: %v", err)
	}
	if status != 200 {
		t.Fatalf("status = %d, want 200", status)
	}
	if got, want := string(body), `{"response":{"battery_level":71}}`; got != want {
		t.Errorf("unwrapped body = %s, want %s", got, want)
	}
	if len(rt.hits) != 2 {
		t.Fatalf("expected owner attempt then fleet retry, got hits: %v", rt.hits)
	}
	if rt.hits[0] != "owner.test Bearer owner-tok" {
		t.Errorf("first hit = %q, want owner host with owner bearer", rt.hits[0])
	}
	if rt.hits[1] != "fleet.test Bearer fleet-tok" {
		t.Errorf("second hit = %q, want fleet host with fleet bearer", rt.hits[1])
	}
}

// TestFleetFallback_NotArmed confirms a 404 surfaces as an error when no Fleet
// fallback is armed (unchanged owner-api behavior).
func TestFleetFallback_NotArmed(t *testing.T) {
	rt := &routingRoundTripper{ownerHost: "owner.test", fleetHost: "fleet.test"}
	c := newFallbackClient(t, rt)
	c.FleetFallback = false

	if _, status, err := c.do("GET", "/api/1/vehicles/VIN123/data_request/charge_state", nil, nil, nil); err == nil || status != 404 {
		t.Fatalf("expected 404 error without fallback, got status=%d err=%v", status, err)
	}
	if len(rt.hits) != 1 {
		t.Errorf("expected exactly one (owner) attempt, got: %v", rt.hits)
	}
}

// TestFleetFallback_NonVehicle404 confirms the fallback is scoped to vehicle
// reads: a 404 on another resource must not retry through Fleet.
func TestFleetFallback_NonVehicle404(t *testing.T) {
	rt := &routingRoundTripper{ownerHost: "owner.test", fleetHost: "fleet.test"}
	c := newFallbackClient(t, rt)

	if _, status, err := c.do("GET", "/api/1/products", nil, nil, nil); err == nil || status != 404 {
		t.Fatalf("expected 404 error for non-vehicle read, got status=%d err=%v", status, err)
	}
	if len(rt.hits) != 1 {
		t.Errorf("expected no Fleet retry for /api/1/products, got: %v", rt.hits)
	}
}

// TestFleetFallback_FullVehicleData confirms the fallback also fires for the
// full-snapshot path (/vehicle_data), not just the data_request subsets. The
// Fleet response is not rewritten by rewriteFleetSubsetPath, so it returns
// as-is.
func TestFleetFallback_FullVehicleData(t *testing.T) {
	rt := &routingRoundTripper{ownerHost: "owner.test", fleetHost: "fleet.test"}
	c := newFallbackClient(t, rt)

	body, status, err := c.do("GET", "/api/1/vehicles/VIN123/vehicle_data", nil, nil, nil)
	if err != nil || status != 200 {
		t.Fatalf("expected fallback success, got status=%d err=%v", status, err)
	}
	if got := string(body); got != `{"response":{"charge_state":{"battery_level":71},"vehicle_id":1}}` {
		t.Errorf("full-snapshot body not passed through: %s", got)
	}
	if len(rt.hits) != 2 || rt.hits[1] != "fleet.test Bearer fleet-tok" {
		t.Errorf("expected owner attempt then fleet retry, got hits: %v", rt.hits)
	}
}

// TestFleetFallback_ScopedRestore pins the P1 fix: a fallback must not
// permanently redirect a reused client. After a 404-driven Fleet retry, the
// client's base URL, FleetMode, and bearer routing are restored, so a later
// read on the same client (e.g. a pre-2021 car in a multi-vehicle command)
// still goes to owner-api.
func TestFleetFallback_ScopedRestore(t *testing.T) {
	rt := &routingRoundTripper{ownerHost: "owner.test", fleetHost: "fleet.test"}
	c := newFallbackClient(t, rt)

	if _, _, err := c.do("GET", "/api/1/vehicles/VIN123/vehicle_data", nil, nil, nil); err != nil {
		t.Fatalf("fallback read errored: %v", err)
	}
	if c.FleetMode {
		t.Error("FleetMode left true after a scoped fallback retry")
	}
	if c.BaseURL != "http://owner.test" {
		t.Errorf("BaseURL = %q, want it restored to the owner host", c.BaseURL)
	}
	if c.Config != nil && c.Config.UseFleetBearer {
		t.Error("UseFleetBearer left true after a scoped fallback retry")
	}
}

func TestIsVehicleReadPath(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"/api/1/vehicles/VIN123/vehicle_data", true},
		{"/api/1/vehicles/VIN123/data_request/charge_state", true},
		{"/api/1/vehicles/VIN123/vehicle_data?endpoints=charge_state", true},
		{"/api/1/products", false},
		{"/api/1/vehicles/VIN123/command/door_lock", false},
		{"/api/1/vehicles", false},
	}
	for _, tc := range cases {
		if got := isVehicleReadPath(tc.path); got != tc.want {
			t.Errorf("isVehicleReadPath(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}
