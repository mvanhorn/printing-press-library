// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
// PATCH(amend-2026-07-31): tests for the batch fare-probe runner and the
// Google 429 exit-code classification. See flights_batch.go.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/flight-goat/internal/gflights"

	"github.com/spf13/cobra"
)

func TestParseBatchTrips(t *testing.T) {
	trips, err := parseBatchTrips([]string{"sea>den@2026-09-14", "PDX>DEN@2026-09-15@2026-09-17"})
	if err != nil {
		t.Fatalf("parseBatchTrips: %v", err)
	}
	if len(trips) != 2 {
		t.Fatalf("trips = %d, want 2", len(trips))
	}
	if trips[0].Origin != "SEA" || trips[0].Destination != "DEN" || trips[0].DepartureDate != "2026-09-14" || trips[0].ReturnDate != "" {
		t.Errorf("trip[0] mismatch: %+v", trips[0])
	}
	if trips[1].ReturnDate != "2026-09-17" {
		t.Errorf("trip[1] return = %q, want 2026-09-17", trips[1].ReturnDate)
	}
}

func TestParseBatchTripsRejectsMalformed(t *testing.T) {
	// PATCH(greptile-1639): the last three cases are the reported P1s — an
	// extra route delimiter left a junk destination, and a reversed round
	// trip passed preflight; both reached Google and burned rate-limit
	// budget on a deterministically invalid search.
	for _, bad := range []string{
		"SEADEN@2026-09-14", "SEA>DEN", "SEA>@2026-09-14", "SEA>DEN@notadate", "SEA>DEN@2026-09-14@notadate",
		"SEA>DEN>LAX@2026-09-14",
		"SEA>DEN@2026-09-17@2026-09-15",
		"SEA>DEN>LAX@2026-09-15@2026-09-17",
	} {
		if _, err := parseBatchTrips([]string{bad}); err == nil {
			t.Errorf("parseBatchTrips(%q) = nil error, want parse failure", bad)
		}
	}
}

// PATCH(greptile-1639): a same-day return is a legitimate itinerary — the
// reversed-date guard must not reject it.
func TestParseBatchTripsAcceptsSameDayReturn(t *testing.T) {
	trips, err := parseBatchTrips([]string{"SEA>DEN@2026-09-15@2026-09-15"})
	if err != nil {
		t.Fatalf("parseBatchTrips same-day return: %v", err)
	}
	if trips[0].DepartureDate != "2026-09-15" || trips[0].ReturnDate != "2026-09-15" {
		t.Errorf("trip mismatch: %+v", trips[0])
	}
}

// PATCH(greptile-1639): both new preflight rejections must exit 2 (usage),
// not 5 (API) — the whole point is failing before any network call.
func TestFlightsCmdRejectsMalformedTripWithExit2(t *testing.T) {
	for _, tc := range []struct{ name, trip string }{
		{"extra-route-delimiter", "SEA>DEN>LAX@2026-09-14"},
		{"reversed-round-trip", "SEA>DEN@2026-09-17@2026-09-15"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newGfFlightsCmd(&rootFlags{})
			cmd.SetArgs([]string{"--trip", tc.trip})
			cmd.SetOut(&bytes.Buffer{})
			cmd.SetErr(&bytes.Buffer{})
			err := cmd.Execute()
			if err == nil {
				t.Fatalf("--trip %q accepted, want usage error", tc.trip)
			}
			if ExitCode(err) != 2 {
				t.Fatalf("ExitCode = %d, want 2 (usage error); err = %v", ExitCode(err), err)
			}
		})
	}
}

func TestRunFlightsBatchStopsOnRateLimitAndEmitsPartialEnvelope(t *testing.T) {
	origSearch, origSleep := batchSearch, batchSleep
	defer func() { batchSearch, batchSleep = origSearch, origSleep }()

	var paced int
	batchSleep = func(context.Context, time.Duration) error { paced++; return nil }
	calls := 0
	batchSearch = func(_ context.Context, opts gflights.SearchOptions) (*gflights.SearchResult, error) {
		calls++
		if calls == 2 {
			return nil, fmt.Errorf("shopping endpoint: %w", gflights.ErrRateLimited)
		}
		return &gflights.SearchResult{Success: true, Count: 1, Flights: []gflights.Flight{{Price: 81, Currency: "EUR"}}}, nil
	}

	trips := []batchTrip{
		{Origin: "PDX", Destination: "DEN", DepartureDate: "2026-09-15"},
		{Origin: "SEA", Destination: "DEN", DepartureDate: "2026-09-15"},
		{Origin: "SFO", Destination: "DEN", DepartureDate: "2026-09-15"},
	}
	cmd := &cobra.Command{}
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(&bytes.Buffer{})
	flags := &rootFlags{asJSON: true}

	err := runFlightsBatch(cmd, flags, trips, gflights.SearchOptions{Currency: "EUR"}, 2*time.Second)
	if !errors.Is(err, gflights.ErrRateLimited) {
		t.Fatalf("err = %v, want ErrRateLimited", err)
	}
	if ExitCode(err) != 7 {
		t.Fatalf("ExitCode = %d, want 7 (rate limit)", ExitCode(err))
	}
	if calls != 2 {
		t.Fatalf("search calls = %d, want 2 (stop at first 429)", calls)
	}
	if paced != 1 {
		t.Fatalf("pacing sleeps = %d, want 1 (only between attempted trips)", paced)
	}

	var env batchEnvelope
	if uerr := json.Unmarshal(out.Bytes(), &env); uerr != nil {
		t.Fatalf("envelope not valid JSON: %v\n%s", uerr, out.String())
	}
	if env.Success || !env.RateLimited || env.Completed != 1 || len(env.Results) != 3 {
		t.Fatalf("envelope mismatch: %+v", env)
	}
	if env.Results[0].Status != "ok" || env.Results[1].Status != "error" || env.Results[2].Status != "skipped" {
		t.Fatalf("statuses = %s/%s/%s, want ok/error/skipped", env.Results[0].Status, env.Results[1].Status, env.Results[2].Status)
	}
}

func TestRunFlightsBatchAllOK(t *testing.T) {
	origSearch, origSleep := batchSearch, batchSleep
	defer func() { batchSearch, batchSleep = origSearch, origSleep }()
	batchSleep = func(context.Context, time.Duration) error { return nil }
	batchSearch = func(context.Context, gflights.SearchOptions) (*gflights.SearchResult, error) {
		return &gflights.SearchResult{Success: true}, nil
	}

	cmd := &cobra.Command{}
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(&bytes.Buffer{})
	err := runFlightsBatch(cmd, &rootFlags{asJSON: true}, []batchTrip{
		{Origin: "SEA", Destination: "LAX", DepartureDate: "2026-09-20"},
		{Origin: "SEA", Destination: "SAN", DepartureDate: "2026-09-21"},
	}, gflights.SearchOptions{}, 0)
	if err != nil {
		t.Fatalf("runFlightsBatch: %v", err)
	}
	var env batchEnvelope
	if uerr := json.Unmarshal(out.Bytes(), &env); uerr != nil {
		t.Fatalf("envelope not valid JSON: %v", uerr)
	}
	if !env.Success || env.Completed != 2 {
		t.Fatalf("envelope mismatch: %+v", env)
	}
}

// PATCH(review-2026-07-31): the human-readable rendering path was untested.
func TestRenderBatchTable(t *testing.T) {
	env := batchEnvelope{
		SearchType: "flights_batch",
		Pace:       "2s",
		Count:      3,
		Completed:  1,
		Results: []batchTripResult{
			{Trip: batchTrip{Origin: "PDX", Destination: "DEN", DepartureDate: "2026-09-15"}, Status: "ok",
				Result: &gflights.SearchResult{Count: 2, Flights: []gflights.Flight{{Price: 151, Currency: "EUR"}, {Price: 81, Currency: "EUR"}}}},
			{Trip: batchTrip{Origin: "SEA", Destination: "DEN", DepartureDate: "2026-09-15", ReturnDate: "2026-09-17"}, Status: "error", Error: "rate limited"},
			{Trip: batchTrip{Origin: "SFO", Destination: "DEN", DepartureDate: "2026-09-15"}, Status: "skipped"},
		},
	}
	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	renderBatchTable(out, errOut, env)
	if !strings.Contains(errOut.String(), "batch: 1/3 trips completed") {
		t.Errorf("stderr summary missing: %q", errOut.String())
	}
	table := out.String()
	for _, want := range []string{"TRIP", "STATUS", "CHEAPEST", "PDX>DEN@2026-09-15", "SEA>DEN@2026-09-15@2026-09-17", "ok", "error", "skipped"} {
		if !strings.Contains(table, want) {
			t.Errorf("table missing %q:\n%s", want, table)
		}
	}
	// Cheapest column sorts a copy: cheapest of {151, 81} is 81.
	if !strings.Contains(table, "81") {
		t.Errorf("expected cheapest price 81 in table:\n%s", table)
	}
}

// PATCH(review-2026-07-31): the Args-validator conflict branches were
// untested, and >=2 --segment used to silently win over --trip.
func TestFlightsCmdModeConflicts(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"trip+positional", []string{"SEA", "DEN", "2026-09-14", "--trip", "SEA>DEN@2026-09-14"}, "--trip replaces the positional"},
		{"trip+one-segment", []string{"--trip", "SEA>DEN@2026-09-14", "--segment", "SFO>NRT@2026-08-15"}, "cannot be combined with --segment"},
		{"trip+two-segments", []string{"--trip", "SEA>DEN@2026-09-14", "--segment", "SFO>NRT@2026-08-15", "--segment", "NRT>SFO@2026-09-05"}, "cannot be combined with --segment"},
		{"trip+return", []string{"--trip", "SEA>DEN@2026-09-14", "--return", "2026-09-17"}, "--return does not apply to --trip batches"},
		{"negative-pace", []string{"--trip", "SEA>DEN@2026-09-14", "--pace", "-1s"}, "--pace must be >= 0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			flags := &rootFlags{}
			cmd := newGfFlightsCmd(flags)
			cmd.SetArgs(tc.args)
			cmd.SetOut(&bytes.Buffer{})
			cmd.SetErr(&bytes.Buffer{})
			err := cmd.Execute()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want substring %q", err, tc.want)
			}
			if ExitCode(err) != 2 {
				t.Fatalf("ExitCode = %d, want 2 (usage error)", ExitCode(err))
			}
		})
	}
}

func TestClassifyGoogleFlightsErrMapsRateLimit(t *testing.T) {
	wrapped := fmt.Errorf("shopping endpoint: %w", gflights.ErrRateLimited)
	err := classifyGoogleFlightsErr(wrapped)
	if ExitCode(err) != 7 {
		t.Fatalf("ExitCode = %d, want 7", ExitCode(err))
	}
	if !strings.Contains(err.Error(), "hint:") {
		t.Fatalf("expected actionable hint in %q", err.Error())
	}

	other := errors.New("parse drift")
	if got := classifyGoogleFlightsErr(other); got != other {
		t.Fatalf("non-429 error must pass through unchanged; got %v", got)
	}
	if classifyGoogleFlightsErr(nil) != nil {
		t.Fatal("nil must stay nil")
	}
}
