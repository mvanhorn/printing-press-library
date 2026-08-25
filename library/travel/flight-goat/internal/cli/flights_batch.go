// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
// PATCH(amend-2026-07-31): batch fare probes with built-in pacing.
//
// Dogfood origin: a real session needed ~40 origin x destination x date fare
// probes and ran them as parallel shell loops — which tripped Google's
// IP-level rate limit and cost the rest of the night in hand-rolled
// sleep-and-retry scripts. The CLI is the right place for that pacing:
// `flights --trip A>B@DATE --trip ...` runs the probes sequentially with a
// configurable gap, emits one envelope with per-trip results, and stops the
// moment Google rate-limits (continuing would deepen the block).

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/flight-goat/internal/gflights"

	"github.com/spf13/cobra"
)

// batchTrip is one parsed --trip value.
type batchTrip struct {
	Origin        string `json:"origin"`
	Destination   string `json:"destination"`
	DepartureDate string `json:"departure_date"`
	ReturnDate    string `json:"return_date,omitempty"`
}

// parseBatchTrips parses repeatable --trip values in the form
// "ORIG>DEST@YYYY-MM-DD" (one-way) or "ORIG>DEST@YYYY-MM-DD@YYYY-MM-DD"
// (round-trip). The syntax deliberately extends the existing --segment
// grammar ("ORIG>DEST@DATE") so users learn one shape.
func parseBatchTrips(values []string) ([]batchTrip, error) {
	trips := make([]batchTrip, 0, len(values))
	for _, raw := range values {
		v := strings.TrimSpace(raw)
		route, dates, ok := strings.Cut(v, "@")
		if !ok {
			return nil, fmt.Errorf("invalid --trip %q: expected ORIG>DEST@YYYY-MM-DD or ORIG>DEST@DEPART@RETURN", raw)
		}
		orig, dest, ok := strings.Cut(route, ">")
		orig, dest = strings.TrimSpace(orig), strings.TrimSpace(dest)
		if !ok || orig == "" || dest == "" {
			return nil, fmt.Errorf("invalid --trip %q: route must be ORIG>DEST (e.g. SEA>DEN)", raw)
		}
		// PATCH(greptile-1639): Cut splits at the FIRST '>', so orig can never
		// carry one — but "SEA>DEN>LAX" would leave "DEN>LAX" as the
		// destination and reach Google as a junk code. Fail in preflight.
		if strings.Contains(dest, ">") {
			return nil, fmt.Errorf("invalid --trip %q: route must be a single ORIG>DEST pair (extra '>' found; multi-leg itineraries use --segment)", raw)
		}
		depart, ret, _ := strings.Cut(dates, "@")
		depart, ret = strings.TrimSpace(depart), strings.TrimSpace(ret)
		if depart == "" {
			return nil, fmt.Errorf("invalid --trip %q: missing departure date after @", raw)
		}
		for _, d := range []string{depart, ret} {
			if d == "" {
				continue
			}
			if _, err := time.Parse("2006-01-02", d); err != nil {
				return nil, fmt.Errorf("invalid --trip %q: date %q must be YYYY-MM-DD", raw, d)
			}
		}
		// PATCH(greptile-1639): a reversed round trip is a deterministic
		// input error — catch it in preflight instead of spending network
		// budget. Same-day returns are legitimate. ISO dates compare
		// lexicographically, and both are format-validated above.
		if ret != "" && ret < depart {
			return nil, fmt.Errorf("invalid --trip %q: return date %s must be on or after departure %s", raw, ret, depart)
		}
		trips = append(trips, batchTrip{
			Origin:        strings.ToUpper(orig),
			Destination:   strings.ToUpper(dest),
			DepartureDate: depart,
			ReturnDate:    ret,
		})
	}
	return trips, nil
}

// batchTripResult is one row of the batch envelope.
type batchTripResult struct {
	Trip   batchTrip              `json:"trip"`
	Status string                 `json:"status"` // "ok", "error", or "skipped"
	Error  string                 `json:"error,omitempty"`
	Result *gflights.SearchResult `json:"result,omitempty"`
}

// batchEnvelope is the single JSON document a batch run emits, including on
// early stop — partial results are the whole point (the user keeps every
// fare fetched before the rate limit hit).
type batchEnvelope struct {
	Success     bool              `json:"success"`
	SearchType  string            `json:"search_type"` // "flights_batch"
	Pace        string            `json:"pace"`
	Count       int               `json:"count"`
	Completed   int               `json:"completed"`
	RateLimited bool              `json:"rate_limited,omitempty"`
	Results     []batchTripResult `json:"results"`
}

// Test seams.
var (
	batchSearch = gflights.Search
	batchSleep  = func(ctx context.Context, d time.Duration) error {
		t := time.NewTimer(d)
		defer t.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			return nil
		}
	}
)

// runFlightsBatch executes the parsed trips sequentially with `pace` between
// consecutive searches. Every trip shares the caller's filter options
// (currency, stops, class, airlines, time window, bags, passengers, sort).
// On ErrRateLimited the batch stops: the failed trip is recorded, the rest
// are marked "skipped", the partial envelope is still emitted, and the
// command exits with the rate-limit code so scripts can tell "done" from
// "cut short".
func runFlightsBatch(cmd *cobra.Command, flags *rootFlags, trips []batchTrip, base gflights.SearchOptions, pace time.Duration) error {
	// PATCH(review-2026-07-31): honor the command's context so a future
	// ExecuteContext/SIGINT wiring can stop the batch and its pacing sleeps;
	// falls back to Background exactly like the single-search path today.
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	// interactive mirrors the rendering condition below: progress goes to
	// stderr only when a human is watching (a paced batch would otherwise
	// look hung for its whole duration); agent/JSON consumers stay silent
	// until the envelope.
	interactive := !flags.asJSON && isTerminal(cmd.OutOrStdout())
	env := batchEnvelope{
		SearchType: "flights_batch",
		Pace:       pace.String(),
		Count:      len(trips),
		Results:    make([]batchTripResult, 0, len(trips)),
	}
	var otherErrors int
	for i, trip := range trips {
		if i > 0 && pace > 0 {
			if err := batchSleep(ctx, pace); err != nil {
				return err
			}
		}
		opts := base
		opts.Origin = trip.Origin
		opts.Destination = trip.Destination
		opts.DepartureDate = trip.DepartureDate
		opts.ReturnDate = trip.ReturnDate
		result, err := batchSearch(ctx, opts)
		if interactive {
			status := "ok"
			if err != nil {
				status = "error"
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "batch %d/%d %s>%s@%s: %s\n", i+1, len(trips), trip.Origin, trip.Destination, trip.DepartureDate, status)
		}
		switch {
		case err == nil:
			env.Results = append(env.Results, batchTripResult{Trip: trip, Status: "ok", Result: result})
			env.Completed++
		case errors.Is(err, gflights.ErrRateLimited):
			env.RateLimited = true
			env.Results = append(env.Results, batchTripResult{Trip: trip, Status: "error", Error: err.Error()})
			for _, rest := range trips[i+1:] {
				env.Results = append(env.Results, batchTripResult{Trip: rest, Status: "skipped", Error: "batch stopped: google flights rate limited"})
			}
		case err != nil:
			otherErrors++
			env.Results = append(env.Results, batchTripResult{Trip: trip, Status: "error", Error: err.Error()})
		}
		if env.RateLimited {
			break
		}
	}
	env.Success = !env.RateLimited && otherErrors == 0

	if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
		bts, _ := json.MarshalIndent(env, "", "  ")
		fmt.Fprintln(cmd.OutOrStdout(), string(bts))
	} else {
		renderBatchTable(cmd.OutOrStdout(), cmd.ErrOrStderr(), env)
	}

	if env.RateLimited {
		return classifyGoogleFlightsErr(fmt.Errorf("batch stopped after %d/%d trips: %w", env.Completed, env.Count, gflights.ErrRateLimited))
	}
	if otherErrors > 0 {
		return apiErr(fmt.Errorf("batch finished with %d/%d trips failed (see per-trip errors in the envelope)", otherErrors, env.Count))
	}
	return nil
}

// renderBatchTable writes the human-readable batch summary: a stderr
// completion line plus a stdout table with one row per trip. Extracted from
// runFlightsBatch so the rendering is unit-testable without a real terminal.
func renderBatchTable(stdout, stderr io.Writer, env batchEnvelope) {
	fmt.Fprintf(stderr, "batch: %d/%d trips completed (pace %s)\n", env.Completed, env.Count, env.Pace)
	tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "TRIP\tSTATUS\tCHEAPEST\tFLIGHTS")
	for _, r := range env.Results {
		label := fmt.Sprintf("%s>%s@%s", r.Trip.Origin, r.Trip.Destination, r.Trip.DepartureDate)
		if r.Trip.ReturnDate != "" {
			label += "@" + r.Trip.ReturnDate
		}
		cheapest, count := "-", 0
		if r.Result != nil && len(r.Result.Flights) > 0 {
			count = r.Result.Count
			flights := make([]gflights.Flight, len(r.Result.Flights))
			copy(flights, r.Result.Flights)
			sort.SliceStable(flights, func(a, b int) bool { return flights[a].Price < flights[b].Price })
			cheapest = formatPrice(flights[0].Currency, flights[0].Price)
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%d\n", label, r.Status, cheapest, count)
	}
	tw.Flush()
}
