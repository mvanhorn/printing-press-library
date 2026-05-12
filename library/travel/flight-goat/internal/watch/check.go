// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

package watch

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/flight-goat/internal/gflights"
)

// DepartureTimeToleranceMinutes is how far the matched itinerary's
// departure time can drift from the user's recorded departure_time
// before the matcher rejects it. Catches the case where an airline
// reuses the same flight number for a later departure on the same day
// after a major schedule change.
const DepartureTimeToleranceMinutes = 30

// Searcher is the subset of internal/gflights we depend on. Tests
// substitute a fake to avoid hitting Google during unit tests.
type Searcher interface {
	Search(ctx context.Context, opts gflights.SearchOptions) (*gflights.SearchResult, error)
}

// gflightsSearcher is the production Searcher; it delegates to
// gflights.Search. Production callers don't need to depend on this — they
// call CheckWithSearcher(nil, ...) and the default kicks in.
type gflightsSearcher struct{}

func (gflightsSearcher) Search(ctx context.Context, opts gflights.SearchOptions) (*gflights.SearchResult, error) {
	return gflights.Search(ctx, opts)
}

// CheckOptions tunes the behavior of one Check call.
type CheckOptions struct {
	// ForceAlert dispatches the alert payload even if the threshold isn't
	// crossed and even if last_alerted_price is set. Used for `watch
	// alert-test`.
	ForceAlert bool

	// RepeatDelta is the minimum additional drop (in the watch's
	// currency) since last_alerted_price that re-triggers an alert.
	// Default is 0, which dedups any price equal to or above the last
	// alerted price.
	RepeatDelta float64

	// Now overrides time.Now for tests.
	Now func() time.Time

	// Searcher overrides the gflights backend. Nil means production.
	Searcher Searcher

	// Dispatcher overrides the alert dispatch surface. Nil means a
	// dispatcher built from the watch's Notify spec.
	Dispatcher Dispatcher
}

func (o *CheckOptions) now() time.Time {
	if o.Now != nil {
		return o.Now()
	}
	return time.Now().UTC()
}

func (o *CheckOptions) searcher() Searcher {
	if o.Searcher != nil {
		return o.Searcher
	}
	return gflightsSearcher{}
}

// Check looks up the current price for the watch's exact flight, decides
// whether to dispatch an alert, persists the result, and returns the
// CheckResult envelope. It does NOT return an error for "no match" or
// "above threshold" — those are normal, expressed in the envelope. Errors
// are reserved for transport failures and bad inputs.
func Check(ctx context.Context, store *Store, w *Watch, opts CheckOptions) (*CheckResult, error) {
	now := opts.now()
	res := &CheckResult{
		Schema:        "flight-goat.watch.check.v1",
		WatchID:       w.ID,
		CheckedAt:     now,
		Origin:        w.Origin,
		Destination:   w.Destination,
		Date:          w.DepartureDate,
		DepartureTime: w.DepartureTime,
		Airline:       w.Airline,
		FlightNo:      w.FlightNumber,
		Cabin:         w.Cabin,
		FareBrand:     w.FareBrand,
		OriginalPrice: w.OriginalPrice,
		Threshold:     w.Threshold,
		Currency:      w.Currency,
		BookingURL:    BookingSearchURL(w),
		Confidence:    MatchRouteOnly,
		SafetyNotice:  SafetyNoticeText,
	}

	searchOpts := gflights.SearchOptions{
		Origin:        w.Origin,
		Destination:   w.Destination,
		DepartureDate: w.DepartureDate,
		CabinClass:    w.Cabin,
		Passengers:    w.Passengers,
		Currency:      w.Currency,
		// ExcludeBasic prevents a $300 basic-economy result from
		// "matching" a $700 main-cabin ticket. The default is set at
		// watch-add time (true unless the user opts out via
		// --include-basic).
		ExcludeBasic: w.ExcludeBasic,
		// We intentionally do not pre-filter by airline: the user's
		// flight could appear in a code-share alongside the operating
		// carrier, and the match logic below handles airline filtering
		// itself. Restricting via SearchOptions.Airlines would risk
		// dropping the user's flight when the response uses a different
		// marketing code.
	}
	result, err := opts.searcher().Search(ctx, searchOpts)
	if err != nil {
		return nil, err
	}

	matched, confidence, cheapest, mismatchReason := matchFlight(result, w)
	if mismatchReason != "" {
		res.MatchMismatchReason = mismatchReason
	}
	res.MatchReason = explainMatch(w, matched, confidence, cheapest, mismatchReason)
	if cheapest != nil {
		p := cheapest.Price
		res.RouteCheapestPrice = &p
	}
	res.Confidence = confidence
	if matched != nil {
		mf := flightToMatched(matched, w.Cabin, w.FareBrand)
		res.MatchedFlight = &mf
		fp := matched.Price
		res.FoundPrice = &fp
		delta := w.OriginalPrice - matched.Price
		res.Delta = &delta
		res.ThresholdCrossed = (w.OriginalPrice - matched.Price) >= w.Threshold
	}

	// Decide whether to dispatch.
	dispatch := false
	switch {
	case opts.ForceAlert:
		// PATCH(greptile P1): force-alert overrides threshold + dedup,
		// but it must NOT bypass the confidence filter. A MatchProbable
		// hit (same airline + cabin but Google didn't echo the flight
		// number) is informational only — alerting on it would update
		// last_alerted_price for a flight the user may not hold and
		// suppress the next legitimate high-confidence alert.
		dispatch = matched != nil && confidence == MatchExact
		if !dispatch {
			res.AlertSuppressed = true
			if matched == nil {
				res.AlertSuppressReason = "force-alert requested but no matching itinerary returned"
			} else {
				res.AlertSuppressReason = "force-alert requested but match confidence below high (refusing to alert on a flight the user may not hold)"
			}
		}
	case confidence != MatchExact:
		res.AlertSuppressed = true
		res.AlertSuppressReason = "match confidence below high (would alert on a flight the user does not hold)"
	case !res.ThresholdCrossed:
		res.AlertSuppressed = true
		res.AlertSuppressReason = "delta below threshold"
	case w.LastAlertedPrice != nil && matched != nil && matched.Price >= *w.LastAlertedPrice-opts.RepeatDelta:
		res.AlertSuppressed = true
		res.AlertSuppressReason = "price has not dropped by --repeat-delta since the last alert"
	default:
		dispatch = true
	}

	if dispatch {
		dispatcher := opts.Dispatcher
		if dispatcher == nil {
			dispatcher = DispatcherFor(w.Notify)
		}
		if dispatcher != nil {
			if err := dispatcher.Dispatch(ctx, *res); err != nil {
				return nil, err
			}
			res.AlertDispatched = true
			res.AlertDispatchedTo = dispatcher.Name()
		}
	}

	// Persist tracking fields. Skip when store is nil (alert-test in dry
	// mode + unit tests).
	if store != nil {
		if err := store.RecordCheck(ctx, w.ID, now, res.FoundPrice, res.AlertDispatched); err != nil {
			return nil, err
		}
	}
	return res, nil
}

// matchFlight scans the result for the user's exact flight, falling back
// to confidence levels described in MatchConfidence. It returns:
//   - (matched, MatchExact, cheapest, "") when airline + flight number +
//     cabin all match a single itinerary AND the optional departure-time
//     window is satisfied;
//   - (matched, MatchProbable, cheapest, "") when airline + cabin match
//     and the provider didn't echo a flight number;
//   - (nil, MatchRouteOnly, cheapest, reason) when the cheapest result
//     has a different airline or flight number, or every airline+number
//     match failed a sanity check (departure-time drift). `reason` carries
//     the rejection so the alert envelope can explain why a near-match
//     was demoted.
func matchFlight(r *gflights.SearchResult, w *Watch) (matched *gflights.Flight, conf MatchConfidence, cheapest *gflights.Flight, mismatchReason string) {
	// gflights does not always return flights sorted by price, and some
	// entries come back with Price == 0 when Google didn't surface a fare
	// (operator-only, codeshare with missing fare, etc.). Skip those when
	// computing the route-cheapest so we don't surface a misleading $0.
	for i := range r.Flights {
		if r.Flights[i].Price <= 0 {
			continue
		}
		if cheapest == nil || r.Flights[i].Price < cheapest.Price {
			cheapest = &r.Flights[i]
		}
	}

	wantAirline := strings.ToUpper(strings.TrimSpace(w.Airline))
	wantNum := strings.ToUpper(strings.TrimSpace(w.FlightNumber))

	var probable *gflights.Flight
	var timeRejected bool
	for i := range r.Flights {
		f := &r.Flights[i]
		if len(f.Legs) == 0 {
			continue
		}
		// Only single-leg itineraries can be the user's flight by
		// flight number — connecting itineraries have multiple flight
		// numbers and would never match a single-flight watch.
		if len(f.Legs) > 1 {
			continue
		}
		leg := f.Legs[0]
		airline := strings.ToUpper(strings.TrimSpace(leg.Airline.Code))
		number := strings.ToUpper(strings.TrimSpace(leg.FlightNumber))
		if airline != wantAirline {
			continue
		}
		if number == wantNum {
			if w.DepartureTime != "" && !departureTimeMatches(leg.DepartureTime, w.DepartureTime) {
				timeRejected = true
				continue
			}
			return f, MatchExact, cheapest, ""
		}
		if number == "" && probable == nil {
			probable = f
		}
	}
	if timeRejected {
		mismatchReason = fmt.Sprintf("flight %s%s found but departure time differs from watched %s by more than %d min", wantAirline, wantNum, w.DepartureTime, DepartureTimeToleranceMinutes)
	}
	if probable != nil {
		return probable, MatchProbable, cheapest, mismatchReason
	}
	return nil, MatchRouteOnly, cheapest, mismatchReason
}

// departureTimeMatches returns true if the candidate ISO-8601 timestamp
// (e.g. "2026-06-21T07:25:00") falls within ±DepartureTimeToleranceMinutes
// of the user's HH:MM watch time. A parse failure on either side is
// treated as "match" — we don't want a malformed Google response to
// drop an otherwise-good exact match.
func departureTimeMatches(candidateISO, wantHHMM string) bool {
	if len(candidateISO) < 16 {
		return true
	}
	candHM := candidateISO[11:16]
	candT, err1 := time.Parse("15:04", candHM)
	wantT, err2 := time.Parse("15:04", wantHHMM)
	if err1 != nil || err2 != nil {
		return true
	}
	// PATCH(greptile P1): treat HH:MM as a point on a 24-hour clock so a
	// 23:50 watch matches a 00:10 candidate by 20 min, not 23h40m. Both
	// timestamps are anchored on the zero date by time.Parse, so a naive
	// Sub gives the linear distance; wrap to the shorter arc whenever the
	// raw diff exceeds 12h.
	diff := candT.Sub(wantT)
	if diff < 0 {
		diff = -diff
	}
	if diff > 12*time.Hour {
		diff = 24*time.Hour - diff
	}
	return diff <= time.Duration(DepartureTimeToleranceMinutes)*time.Minute
}

// explainMatch returns a one-sentence chain-of-evidence string covering
// every constraint the matcher actually checked. Users see this in the
// alert so they can decide whether to trust the match without
// reverse-engineering the code. Kept short on purpose — anything that
// doesn't change the human's "is this really my flight?" decision stays
// out.
func explainMatch(w *Watch, matched *gflights.Flight, conf MatchConfidence, cheapest *gflights.Flight, mismatchReason string) string {
	switch conf {
	case MatchExact:
		parts := []string{
			fmt.Sprintf("same airline %s, flight %s", w.Airline, w.FlightNumber),
			fmt.Sprintf("date %s", w.DepartureDate),
			fmt.Sprintf("route %s→%s", w.Origin, w.Destination),
		}
		if w.Cabin != "" {
			parts = append(parts, fmt.Sprintf("cabin %s", strings.ReplaceAll(w.Cabin, "_", " ")))
		}
		if w.ExcludeBasic {
			parts = append(parts, "basic-economy excluded")
		} else {
			parts = append(parts, "basic-economy included (per your watch)")
		}
		if w.DepartureTime != "" && matched != nil && len(matched.Legs) > 0 {
			candHM := trimDepartureHM(matched.Legs[0].DepartureTime)
			parts = append(parts, fmt.Sprintf("departure %s within ±%d min of your %s", candHM, DepartureTimeToleranceMinutes, w.DepartureTime))
		}
		return "exact match: " + strings.Join(parts, "; ")
	case MatchProbable:
		parts := []string{
			fmt.Sprintf("same airline %s", w.Airline),
			fmt.Sprintf("date %s", w.DepartureDate),
			fmt.Sprintf("route %s→%s", w.Origin, w.Destination),
		}
		if w.Cabin != "" {
			parts = append(parts, fmt.Sprintf("cabin %s", strings.ReplaceAll(w.Cabin, "_", " ")))
		}
		parts = append(parts, "flight number not returned by Google Flights for this itinerary")
		return "probable match: " + strings.Join(parts, "; ")
	default: // MatchRouteOnly
		if mismatchReason != "" {
			return "no exact match — " + mismatchReason
		}
		if cheapest != nil && len(cheapest.Legs) > 0 {
			leg := cheapest.Legs[0]
			return fmt.Sprintf("no exact match: your %s%s did not appear in the response; the cheapest result on this route is %s%s at %s %.2f (different from your flight)",
				w.Airline, w.FlightNumber, leg.Airline.Code, leg.FlightNumber, cheapest.Currency, cheapest.Price)
		}
		return fmt.Sprintf("no exact match: your %s%s did not appear in the live Google Flights response for %s→%s on %s",
			w.Airline, w.FlightNumber, w.Origin, w.Destination, w.DepartureDate)
	}
}

// trimDepartureHM pulls HH:MM out of an ISO timestamp; returns the
// original on parse miss.
func trimDepartureHM(iso string) string {
	if len(iso) >= 16 {
		return iso[11:16]
	}
	return iso
}

// BookingSearchURL returns a Google Flights search URL pre-filled with
// the watch's route, date, and cabin so users can open the cheaper
// itinerary in one tap. Google Flights' canonical search URL takes a
// natural-language `q` parameter and applies the right filters — much
// more durable than building the encoded `tfs` protobuf.
func BookingSearchURL(w *Watch) string {
	if w == nil {
		return ""
	}
	q := fmt.Sprintf("Flights from %s to %s on %s", w.Origin, w.Destination, w.DepartureDate)
	if w.Cabin != "" {
		q += " " + strings.ReplaceAll(w.Cabin, "_", " ")
	}
	u := url.URL{
		Scheme: "https",
		Host:   "www.google.com",
		Path:   "/travel/flights",
	}
	qs := u.Query()
	qs.Set("q", q)
	qs.Set("curr", strings.ToUpper(strings.TrimSpace(w.Currency)))
	u.RawQuery = qs.Encode()
	return u.String()
}

func flightToMatched(f *gflights.Flight, cabin, fareBrand string) MatchedFlight {
	mf := MatchedFlight{
		Price:           f.Price,
		Currency:        f.Currency,
		Cabin:           cabin,
		FareBrand:       fareBrand,
		Stops:           f.Stops,
		DurationMinutes: f.DurationMinutes,
	}
	if len(f.Legs) > 0 {
		mf.Airline = f.Legs[0].Airline.Code
		mf.FlightNumber = f.Legs[0].FlightNumber
		mf.DepartureTime = f.Legs[0].DepartureTime
		mf.ArrivalTime = f.Legs[len(f.Legs)-1].ArrivalTime
	}
	return mf
}
