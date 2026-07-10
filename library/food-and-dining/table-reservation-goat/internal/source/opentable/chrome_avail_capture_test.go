// Copyright 2026 Pejman Pour-Moezzi and contributors. Licensed under Apache-2.0. See LICENSE.

package opentable

import (
	"strings"
	"testing"
)

// TestIsAvailabilityGraphQLRequest_ExactOperationMatch pins the request
// matcher to an exact (case-insensitive) allowlist. The old substring test
// ("availability") let a stray operation whose name merely contained the
// word poison the harvested persisted-query identity and the captured
// response.
func TestIsAvailabilityGraphQLRequest_ExactOperationMatch(t *testing.T) {
	cases := []struct {
		name string
		url  string
		want bool
	}{
		{
			"plural operation",
			"https://www.opentable.com/dapi/fe/gql?optype=query&opname=RestaurantsAvailability",
			true,
		},
		{
			"singular operation",
			"https://www.opentable.com/dapi/fe/gql?optype=query&opname=RestaurantAvailability",
			true,
		},
		{
			"plural lowercase",
			"https://www.opentable.com/dapi/fe/gql?opname=restaurantsavailability",
			true,
		},
		{
			"singular uppercase",
			"https://www.opentable.com/dapi/fe/gql?opname=RESTAURANTAVAILABILITY",
			true,
		},
		{
			"substring near-miss",
			"https://www.opentable.com/dapi/fe/gql?opname=FooAvailabilityBar",
			false,
		},
		{
			"suffixed variant",
			"https://www.opentable.com/dapi/fe/gql?opname=RestaurantsAvailabilityExperiment",
			false,
		},
		{
			"unrelated op containing substring",
			"https://www.opentable.com/dapi/fe/gql?opname=WaitlistAvailability",
			false,
		},
		{
			"bare availability",
			"https://www.opentable.com/dapi/fe/gql?opname=Availability",
			false,
		},
		{
			"missing opname",
			"https://www.opentable.com/dapi/fe/gql?optype=query",
			false,
		},
		{
			"wrong path",
			"https://www.opentable.com/other/path?opname=RestaurantsAvailability",
			false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isAvailabilityGraphQLRequest(tc.url); got != tc.want {
				t.Fatalf("isAvailabilityGraphQLRequest(%q) = %v, want %v", tc.url, got, tc.want)
			}
		})
	}
}

// TestSlotCapture_FirstIdentityWins guards against a later availability
// request replacing the persisted-query identity the first request sealed.
func TestSlotCapture_FirstIdentityWins(t *testing.T) {
	cap := newSlotCapture()
	first := availabilityQueryIdentity{
		Hash:          strings.Repeat("a", 64),
		OperationName: "RestaurantsAvailability",
	}
	second := availabilityQueryIdentity{
		Hash:          strings.Repeat("b", 64),
		OperationName: "RestaurantAvailability",
	}

	if !cap.trackRequest("req-1") {
		t.Fatal("first request must be tracked")
	}
	// A hashless identity (e.g. post-data fetch failed) must not seal anything.
	cap.recordIdentity(availabilityQueryIdentity{OperationName: "RestaurantsAvailability"})
	if cap.reqQuery.Hash != "" {
		t.Fatalf("hashless identity sealed capture: %+v", cap.reqQuery)
	}

	cap.recordIdentity(first)
	cap.recordIdentity(second)
	if cap.reqQuery != first {
		t.Fatalf("captured identity = %+v, want first capture %+v", cap.reqQuery, first)
	}

	// Requests arriving after the identity is sealed must not be tracked, so
	// their responses cannot displace the first captured body.
	if cap.trackRequest("req-2") {
		t.Fatal("request after sealed identity must be rejected")
	}
	if cap.isTracked("req-2") {
		t.Fatal("rejected request must not be response-matched")
	}
	if !cap.isTracked("req-1") {
		t.Fatal("first request must stay response-matched")
	}
}

// TestSlotCapture_FirstResponseWins guards against a later response
// overwriting the body/status the caller will read.
func TestSlotCapture_FirstResponseWins(t *testing.T) {
	cap := newSlotCapture()
	cap.recordResponse([]byte(`{"data":"first"}`), 200, nil)
	cap.recordResponse([]byte(`{"data":"second"}`), 403, nil)

	if got := string(cap.body); got != `{"data":"first"}` {
		t.Fatalf("captured body = %q, want first capture", got)
	}
	if cap.status != 200 {
		t.Fatalf("captured status = %d, want 200", cap.status)
	}
	select {
	case <-cap.done:
	default:
		t.Fatal("recordResponse must release done waiters")
	}
}
