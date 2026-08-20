package cli

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/tesla/internal/client"
	"github.com/mvanhorn/printing-press-library/library/devices/tesla/internal/config"
)

// reachRoundTripper routes by host+path: the owner-api host serves /products
// and 404s the numeric-id vehicle_state read (a 2021+ car it can't serve); the
// Fleet host serves vehicle_data by VIN. Toggle ownerStateStatus to model a
// REST car (200) or an asleep car (the Fleet host also 404s when fleetDown).
type reachRoundTripper struct {
	ownerHost, fleetHost string
	ownerStateStatus     int  // status for owner-api data_request/vehicle_state
	fleetDown            bool // when true, Fleet vehicle_data also 404s
}

func (rt *reachRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	mk := func(code int, body string) (*http.Response, error) {
		return &http.Response{
			StatusCode: code,
			Body:       io.NopCloser(bytes.NewReader([]byte(body))),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	}
	p := req.URL.Path
	switch {
	case req.URL.Host == rt.ownerHost && p == "/api/1/products":
		return mk(200, `{"response":[{"vin":"5YJ3E1EA6PF404972","vehicle_id":123}]}`)
	case req.URL.Host == rt.ownerHost && strings.HasSuffix(p, "/data_request/vehicle_state"):
		return mk(rt.ownerStateStatus, `{"response":null,"error":"not_found"}`)
	case req.URL.Host == rt.fleetHost && strings.HasSuffix(p, "/vehicle_data"):
		if rt.fleetDown {
			return mk(408, `{"error":"vehicle offline"}`)
		}
		return mk(200, `{"response":{"vehicle_state":{"locked":true},"vehicle_id":123}}`)
	default:
		return mk(404, `{"error":"Not Found"}`)
	}
}

func newReachClient(t *testing.T, rt *reachRoundTripper) *client.Client {
	t.Helper()
	cfg := &config.Config{
		BaseURL:     "http://" + rt.ownerHost,
		AccessToken: "owner-tok",
		TokenExpiry: time.Now().Add(time.Hour), // mixed account: a usable owner token
	}
	cfg.Fleet.AccessToken = "fleet-tok"
	c := client.New(cfg, time.Second, 0)
	c.HTTPClient = &http.Client{Transport: rt}
	c.NoCache = true
	c.FleetFallback = true
	c.FleetBaseURL = "http://" + rt.fleetHost
	return c
}

// TestReachability_MixedAccount_SignedViaFleet is the regression for the bug
// this PR fixes: on a mixed account (valid owner token) the owner-api 404s the
// 2021+ car by numeric id, and reachability must probe Fleet by VIN and
// classify SIGNED_COMMAND_REQ rather than the misleading
// VEHICLE_ASLEEP_OR_OFFLINE the unpatched code returned.
func TestReachability_MixedAccount_SignedViaFleet(t *testing.T) {
	rt := &reachRoundTripper{ownerHost: "owner.test", fleetHost: "fleet.test", ownerStateStatus: 404}
	c := newReachClient(t, rt)

	r := probeReachability(context.Background(), c, "")
	if r.Classification != "SIGNED_COMMAND_REQ" {
		t.Fatalf("classification = %q, want SIGNED_COMMAND_REQ (checks: %+v)", r.Classification, r.Checks)
	}
	if r.VIN != "5YJ3E1EA6PF404972" {
		t.Errorf("vin = %q, want the products VIN", r.VIN)
	}
	var sawFleetCheck bool
	for _, ck := range r.Checks {
		if ck.Name == "vehicle_state_fleet" && ck.OK {
			sawFleetCheck = true
		}
	}
	if !sawFleetCheck {
		t.Errorf("expected an OK vehicle_state_fleet check, got %+v", r.Checks)
	}
}

// TestReachability_RESTCar_StaysREST confirms a pre-2021 car that answers
// owner-api is still classified REST_OK and never forced onto the Fleet path,
// even with Fleet configured.
func TestReachability_RESTCar_StaysREST(t *testing.T) {
	rt := &reachRoundTripper{ownerHost: "owner.test", fleetHost: "fleet.test", ownerStateStatus: 200}
	c := newReachClient(t, rt)

	r := probeReachability(context.Background(), c, "")
	if r.Classification != "REST_OK" {
		t.Fatalf("classification = %q, want REST_OK (checks: %+v)", r.Classification, r.Checks)
	}
	for _, ck := range r.Checks {
		if ck.Name == "vehicle_state_fleet" {
			t.Errorf("REST car must not probe Fleet: %+v", r.Checks)
		}
	}
}

// TestReachability_AsleepBothPaths: owner-api 404 and Fleet can't reach it
// either -> asleep/offline, not a false signed-command classification.
func TestReachability_AsleepBothPaths(t *testing.T) {
	rt := &reachRoundTripper{ownerHost: "owner.test", fleetHost: "fleet.test", ownerStateStatus: 404, fleetDown: true}
	c := newReachClient(t, rt)

	r := probeReachability(context.Background(), c, "")
	if r.Classification != "VEHICLE_ASLEEP_OR_OFFLINE" {
		t.Fatalf("classification = %q, want VEHICLE_ASLEEP_OR_OFFLINE (checks: %+v)", r.Classification, r.Checks)
	}
}

func TestLooksLikeVIN(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"5YJ3E1EA6PF404972", true},
		{"2252164336719276", false}, // 16-digit numeric vehicle_id
		{"12345678901234567", false}, // 17 digits, no letter
		{"5YJ3E1EA6PF40497", false},   // 16 chars
		{"", false},
	}
	for _, tc := range cases {
		if got := looksLikeVIN(tc.in); got != tc.want {
			t.Errorf("looksLikeVIN(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
