// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

package watch

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/flight-goat/internal/gflights"
)

// fakeSearcher returns a canned SearchResult; tests build it inline.
type fakeSearcher struct {
	res *gflights.SearchResult
	err error
}

func (f fakeSearcher) Search(_ context.Context, _ gflights.SearchOptions) (*gflights.SearchResult, error) {
	return f.res, f.err
}

// recordDispatcher captures dispatches without sending anything.
type recordDispatcher struct {
	got []CheckResult
}

func (d *recordDispatcher) Name() string { return "test-dispatcher" }
func (d *recordDispatcher) Dispatch(_ context.Context, r CheckResult) error {
	d.got = append(d.got, r)
	return nil
}

func flightOpt(airline, fno string, price float64) gflights.Flight {
	return gflights.Flight{
		Price:    price,
		Currency: "USD",
		Stops:    0,
		Legs: []gflights.Leg{{
			DepartureAirport: gflights.Airport{Code: "SFO"},
			ArrivalAirport:   gflights.Airport{Code: "JFK"},
			Airline:          gflights.Airline{Code: airline},
			FlightNumber:     fno,
		}},
	}
}

func TestCheckExactMatchAndAlert(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	w := newSampleWatch()
	if _, err := s.Insert(ctx, w); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	fake := fakeSearcher{res: &gflights.SearchResult{
		Success:    true,
		Source:     "test",
		DataSource: "test",
		Flights: []gflights.Flight{
			flightOpt("UA", "200", 300), // route-cheaper, different airline
			flightOpt("DL", "669", 354), // exact match, below threshold-crossing price (428.20 - 354.10 = 74)
			flightOpt("DL", "700", 400), // same airline, different flight
		},
	}}
	rec := &recordDispatcher{}
	res, err := Check(ctx, s, w, CheckOptions{
		Searcher:   fake,
		Dispatcher: rec,
	})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if res.Confidence != MatchExact {
		t.Fatalf("want high confidence, got %q", res.Confidence)
	}
	if res.FoundPrice == nil || *res.FoundPrice != 354 {
		t.Fatalf("FoundPrice = %v, want 354", res.FoundPrice)
	}
	if res.RouteCheapestPrice == nil || *res.RouteCheapestPrice != 300 {
		t.Fatalf("RouteCheapestPrice = %v, want 300", res.RouteCheapestPrice)
	}
	if !res.ThresholdCrossed {
		t.Fatalf("ThresholdCrossed should be true: delta=%v threshold=%v", res.Delta, res.Threshold)
	}
	if !res.AlertDispatched {
		t.Fatalf("AlertDispatched should be true: %+v", res)
	}
	if len(rec.got) != 1 {
		t.Fatalf("dispatcher should have received exactly 1 alert, got %d", len(rec.got))
	}

	// Persisted last_alerted_price should reflect the alerted price.
	got, _ := s.Get(ctx, w.ID)
	if got.LastAlertedPrice == nil || *got.LastAlertedPrice != 354 {
		t.Fatalf("LastAlertedPrice = %v, want 354", got.LastAlertedPrice)
	}
}

func TestCheckBelowThresholdDoesNotAlert(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	w := newSampleWatch()
	if _, err := s.Insert(ctx, w); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	// Paid 428.20, threshold 50 -> need at least 50 off to alert.
	// Use 400 -> only 28.20 off, below threshold.
	fake := fakeSearcher{res: &gflights.SearchResult{
		Flights: []gflights.Flight{flightOpt("DL", "669", 400)},
	}}
	rec := &recordDispatcher{}
	res, err := Check(ctx, s, w, CheckOptions{Searcher: fake, Dispatcher: rec})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if res.Confidence != MatchExact {
		t.Fatalf("want high confidence, got %q", res.Confidence)
	}
	if res.ThresholdCrossed {
		t.Fatalf("ThresholdCrossed should be false")
	}
	if res.AlertDispatched {
		t.Fatalf("AlertDispatched should be false")
	}
	if !res.AlertSuppressed || !strings.Contains(res.AlertSuppressReason, "threshold") {
		t.Fatalf("expected suppression reason mentioning threshold, got %q", res.AlertSuppressReason)
	}
	if len(rec.got) != 0 {
		t.Fatalf("dispatcher should not have been called")
	}
}

func TestCheckRouteOnlyDoesNotAlert(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	w := newSampleWatch()
	if _, err := s.Insert(ctx, w); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	// Only a different airline / different flight number is cheaper.
	fake := fakeSearcher{res: &gflights.SearchResult{
		Flights: []gflights.Flight{
			flightOpt("B6", "100", 250),
			flightOpt("AA", "200", 260),
		},
	}}
	rec := &recordDispatcher{}
	res, err := Check(ctx, s, w, CheckOptions{Searcher: fake, Dispatcher: rec})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if res.Confidence != MatchRouteOnly {
		t.Fatalf("want route-only, got %q", res.Confidence)
	}
	if res.FoundPrice != nil {
		t.Fatalf("FoundPrice should be nil when no flight matches: %v", res.FoundPrice)
	}
	if res.RouteCheapestPrice == nil || *res.RouteCheapestPrice != 250 {
		t.Fatalf("RouteCheapestPrice = %v, want 250", res.RouteCheapestPrice)
	}
	if res.AlertDispatched {
		t.Fatalf("should not alert on route-only match")
	}
	if len(rec.got) != 0 {
		t.Fatalf("dispatcher should not have been called for route-only match")
	}
}

func TestCheckMissingFlightNumberIsProbable(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	w := newSampleWatch()
	if _, err := s.Insert(ctx, w); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	// Provider omitted the flight number on the same-airline result.
	fake := fakeSearcher{res: &gflights.SearchResult{
		Flights: []gflights.Flight{flightOpt("DL", "", 300)},
	}}
	rec := &recordDispatcher{}
	res, err := Check(ctx, s, w, CheckOptions{Searcher: fake, Dispatcher: rec})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if res.Confidence != MatchProbable {
		t.Fatalf("want medium confidence, got %q", res.Confidence)
	}
	if res.AlertDispatched {
		t.Fatalf("medium confidence should not alert: %+v", res)
	}
}

func TestCheckDedupsRepeatAlertViaRepeatDelta(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	w := newSampleWatch()
	if _, err := s.Insert(ctx, w); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	// First check alerts at 354.
	fake1 := fakeSearcher{res: &gflights.SearchResult{
		Flights: []gflights.Flight{flightOpt("DL", "669", 354)},
	}}
	if _, err := Check(ctx, s, w, CheckOptions{Searcher: fake1, Dispatcher: &recordDispatcher{}}); err != nil {
		t.Fatalf("first Check: %v", err)
	}
	// Reload watch from store so LastAlertedPrice is populated.
	fresh, _ := s.Get(ctx, w.ID)

	// Second check at the same price (no further drop) should be suppressed.
	fake2 := fakeSearcher{res: &gflights.SearchResult{
		Flights: []gflights.Flight{flightOpt("DL", "669", 354)},
	}}
	rec := &recordDispatcher{}
	res, err := Check(ctx, s, fresh, CheckOptions{Searcher: fake2, Dispatcher: rec, RepeatDelta: 10})
	if err != nil {
		t.Fatalf("second Check: %v", err)
	}
	if res.AlertDispatched {
		t.Fatalf("repeat at same price should be suppressed: %+v", res)
	}
	if !res.AlertSuppressed || !strings.Contains(res.AlertSuppressReason, "repeat-delta") {
		t.Fatalf("expected repeat-delta suppression, got %q", res.AlertSuppressReason)
	}
	if len(rec.got) != 0 {
		t.Fatalf("dispatcher should not have been called on dedup")
	}

	// A further drop of more than RepeatDelta should re-alert.
	fresh2, _ := s.Get(ctx, w.ID)
	fake3 := fakeSearcher{res: &gflights.SearchResult{
		Flights: []gflights.Flight{flightOpt("DL", "669", 340)}, // 14 below last alerted
	}}
	rec2 := &recordDispatcher{}
	res2, err := Check(ctx, s, fresh2, CheckOptions{Searcher: fake3, Dispatcher: rec2, RepeatDelta: 10})
	if err != nil {
		t.Fatalf("third Check: %v", err)
	}
	if !res2.AlertDispatched {
		t.Fatalf("further drop past repeat-delta should re-alert: %+v", res2)
	}
	if len(rec2.got) != 1 {
		t.Fatalf("dispatcher should have been called exactly once on the re-alert, got %d", len(rec2.got))
	}
}

// Regression for greptile P1: departureTimeMatches must wrap around
// midnight on the 24-hour clock so a red-eye watch at 23:50 matches a
// 00:10 candidate by 20 min, not by 23h40m. Without the wrap the alert
// is silently demoted to MatchRouteOnly and the price drop never fires.
func TestDepartureTimeMatchesWrapsMidnight(t *testing.T) {
	cases := []struct {
		watch     string
		candidate string
		want      bool
	}{
		{"23:50", "2026-06-21T00:10:00", true},  // 20 min after midnight
		{"00:10", "2026-06-21T23:50:00", true},  // 20 min before midnight
		{"23:30", "2026-06-21T00:01:00", true},  // 31 min apart -> just over tolerance? actually within
		{"23:00", "2026-06-21T00:35:00", false}, // 95 min apart, outside tolerance
		{"12:00", "2026-06-21T11:45:00", true},  // boring non-wrap case
		{"12:00", "2026-06-21T13:00:00", false}, // 60 min, outside tolerance
	}
	// 23:30 -> 00:01 is actually 31 min; tolerance is 30 — fix the
	// expected case to be within tolerance only at 23:30 vs 23:59.
	cases[2] = struct {
		watch, candidate string
		want             bool
	}{"23:30", "2026-06-22T00:00:00", true} // 30 min apart, on the boundary
	for _, tc := range cases {
		got := departureTimeMatches(tc.candidate, tc.watch)
		if got != tc.want {
			t.Errorf("departureTimeMatches(%q, %q) = %v, want %v",
				tc.candidate, tc.watch, got, tc.want)
		}
	}
}

// Regression for greptile P1: ForceAlert must not dispatch on
// MatchProbable. A probable match (same airline + cabin but no flight
// number returned) is informational only; alerting would update
// last_alerted_price for a flight the user may not hold and suppress
// the next legitimate high-confidence alert.
func TestCheckForceAlertSuppressesMatchProbable(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	w := newSampleWatch()
	if _, err := s.Insert(ctx, w); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	// Same airline + cabin, but Google omitted the flight number ->
	// MatchProbable. With --force-alert, the old code would still
	// dispatch; the fixed code suppresses with a confidence reason.
	fake := fakeSearcher{res: &gflights.SearchResult{
		Flights: []gflights.Flight{flightOpt("DL", "", 300)},
	}}
	rec := &recordDispatcher{}
	res, err := Check(ctx, s, w, CheckOptions{
		Searcher: fake, Dispatcher: rec, ForceAlert: true,
	})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if res.Confidence != MatchProbable {
		t.Fatalf("expected MatchProbable, got %q", res.Confidence)
	}
	if res.AlertDispatched {
		t.Fatalf("force-alert must not dispatch on MatchProbable: %+v", res)
	}
	if !res.AlertSuppressed || !strings.Contains(res.AlertSuppressReason, "confidence") {
		t.Fatalf("expected confidence-based suppression, got %q", res.AlertSuppressReason)
	}
	if len(rec.got) != 0 {
		t.Fatalf("dispatcher must not be called on MatchProbable force-alert")
	}
	// And critically: last_alerted_price must NOT be updated, otherwise
	// the next legitimate high-confidence alert gets dedup-suppressed.
	got, _ := s.Get(ctx, w.ID)
	if got.LastAlertedPrice != nil {
		t.Fatalf("LastAlertedPrice must remain nil after suppressed MatchProbable force-alert, got %v", *got.LastAlertedPrice)
	}
}

func TestCheckForceAlertOverridesDedupAndThreshold(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	w := newSampleWatch()
	if _, err := s.Insert(ctx, w); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	// Price slightly below paid, far above the threshold-crossing line.
	fake := fakeSearcher{res: &gflights.SearchResult{
		Flights: []gflights.Flight{flightOpt("DL", "669", 425)},
	}}
	rec := &recordDispatcher{}
	res, err := Check(ctx, s, w, CheckOptions{Searcher: fake, Dispatcher: rec, ForceAlert: true})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !res.AlertDispatched {
		t.Fatalf("force-alert should dispatch despite threshold not crossed")
	}
	if len(rec.got) != 1 {
		t.Fatalf("dispatcher should have received the forced alert")
	}
}

func TestCheckResultJSONSchemaShape(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	w := newSampleWatch()
	if _, err := s.Insert(ctx, w); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	fake := fakeSearcher{res: &gflights.SearchResult{
		Flights: []gflights.Flight{flightOpt("DL", "669", 354)},
	}}
	res, err := Check(ctx, s, w, CheckOptions{
		Searcher:   fake,
		Dispatcher: &recordDispatcher{},
		Now:        func() time.Time { return time.Date(2026, 5, 11, 12, 0, 0, 0, time.UTC) },
	})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	buf, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s2 := string(buf)
	wantFields := []string{
		`"schema":"flight-goat.watch.check.v1"`,
		`"watch_id":`,
		`"origin":"SFO"`,
		`"destination":"JFK"`,
		`"airline":"DL"`,
		`"flight_number":"669"`,
		`"confidence":"high"`,
		`"threshold_crossed":true`,
		`"alert_dispatched":true`,
		`"safety_notice":"Same flight appears cheaper`,
		`"matched_flight":`,
	}
	for _, f := range wantFields {
		if !strings.Contains(s2, f) {
			t.Fatalf("JSON missing %q\n%s", f, s2)
		}
	}
	if !strings.Contains(s2, "Verify fare rules") {
		t.Fatalf("safety notice missing rebooking warning: %s", s2)
	}
}

func TestSampleResultCarriesSafetyNotice(t *testing.T) {
	w := newSampleWatch()
	w.ID = "watch_test"
	r := SampleResult(w, time.Date(2026, 5, 11, 0, 0, 0, 0, time.UTC))
	if r.SafetyNotice != SafetyNoticeText {
		t.Fatalf("SampleResult dropped safety notice: %q", r.SafetyNotice)
	}
	text := FormatAlertText(r)
	if !strings.Contains(text, "Verify fare rules") {
		t.Fatalf("FormatAlertText missing safety notice: %s", text)
	}
	if !strings.Contains(text, "Book:") || !strings.Contains(text, "google.com/travel/flights") {
		t.Fatalf("FormatAlertText missing Google Flights booking URL: %s", text)
	}
	if !strings.Contains(text, "Why:") || !strings.Contains(text, "exact match") {
		t.Fatalf("FormatAlertText missing MatchReason line: %s", text)
	}
}

func TestCheckRejectsDepartureTimeDrift(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	w := newSampleWatch()
	w.DepartureTime = "07:30" // user holds the morning departure
	if _, err := s.Insert(ctx, w); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	// Same airline + flight number, but Google returned an 11pm departure
	// (e.g., DL reused the flight number for an evening reschedule).
	fake := fakeSearcher{res: &gflights.SearchResult{
		Flights: []gflights.Flight{{
			Price:    300,
			Currency: "USD",
			Legs: []gflights.Leg{{
				DepartureAirport: gflights.Airport{Code: "SFO"},
				ArrivalAirport:   gflights.Airport{Code: "JFK"},
				DepartureTime:    "2026-06-21T23:00:00",
				Airline:          gflights.Airline{Code: "DL"},
				FlightNumber:     "669",
			}},
		}},
	}}
	rec := &recordDispatcher{}
	res, err := Check(ctx, s, w, CheckOptions{Searcher: fake, Dispatcher: rec})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if res.Confidence == MatchExact {
		t.Fatalf("departure-time drift should NOT yield high confidence: %+v", res)
	}
	if res.MatchMismatchReason == "" {
		t.Fatalf("expected MatchMismatchReason populated when time drifts; got empty")
	}
	if res.AlertDispatched {
		t.Fatalf("alert should NOT dispatch on time-drift rejection")
	}
}

func TestCheckAcceptsDepartureTimeWithinTolerance(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	w := newSampleWatch()
	w.DepartureTime = "07:30"
	if _, err := s.Insert(ctx, w); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	// 07:50 is 20 min from user's 07:30 -> within ±30 min.
	fake := fakeSearcher{res: &gflights.SearchResult{
		Flights: []gflights.Flight{{
			Price:    300,
			Currency: "USD",
			Legs: []gflights.Leg{{
				DepartureTime: "2026-06-21T07:50:00",
				Airline:       gflights.Airline{Code: "DL"},
				FlightNumber:  "669",
			}},
		}},
	}}
	res, err := Check(ctx, s, w, CheckOptions{Searcher: fake, Dispatcher: &recordDispatcher{}})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if res.Confidence != MatchExact {
		t.Fatalf("within-tolerance time should match exact: %q", res.Confidence)
	}
}

func TestCheckPassesExcludeBasicToSearch(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	w := newSampleWatch()
	w.ExcludeBasic = true
	if _, err := s.Insert(ctx, w); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	// Capture the SearchOptions to verify ExcludeBasic propagated.
	var seen gflights.SearchOptions
	fake := capturingSearcher{
		seen: &seen,
		res:  &gflights.SearchResult{Flights: []gflights.Flight{flightOpt("DL", "669", 300)}},
	}
	if _, err := Check(ctx, s, w, CheckOptions{Searcher: fake, Dispatcher: &recordDispatcher{}}); err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !seen.ExcludeBasic {
		t.Fatalf("SearchOptions.ExcludeBasic should be true when w.ExcludeBasic is set")
	}
}

func TestCheckMatchReasonExplainsExactMatch(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	w := newSampleWatch()
	w.DepartureTime = "07:30"
	w.ExcludeBasic = true
	w.FareBrand = "Main Cabin"
	if _, err := s.Insert(ctx, w); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	fake := fakeSearcher{res: &gflights.SearchResult{
		Flights: []gflights.Flight{{
			Price: 300, Currency: "USD",
			Legs: []gflights.Leg{{
				DepartureTime: "2026-06-21T07:25:00",
				Airline:       gflights.Airline{Code: "DL"},
				FlightNumber:  "669",
			}},
		}},
	}}
	res, err := Check(ctx, s, w, CheckOptions{Searcher: fake, Dispatcher: &recordDispatcher{}})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	for _, want := range []string{"exact match", "same airline DL", "flight 669", "economy", "basic-economy excluded", "departure 07:25"} {
		if !strings.Contains(res.MatchReason, want) {
			t.Fatalf("MatchReason should mention %q; got %q", want, res.MatchReason)
		}
	}
}

func TestCheckBookingURLContainsRouteAndDate(t *testing.T) {
	w := newSampleWatch()
	url := BookingSearchURL(w)
	for _, want := range []string{"google.com/travel/flights", "SFO", "JFK", "2026-06-21", "curr=USD"} {
		if !strings.Contains(url, want) {
			t.Fatalf("BookingURL should contain %q; got %q", want, url)
		}
	}
}

// capturingSearcher records the SearchOptions it received and returns a
// canned result. Used to assert that flags propagate from Watch into the
// gflights call.
type capturingSearcher struct {
	seen *gflights.SearchOptions
	res  *gflights.SearchResult
	err  error
}

func (c capturingSearcher) Search(_ context.Context, opts gflights.SearchOptions) (*gflights.SearchResult, error) {
	*c.seen = opts
	return c.res, c.err
}

// Regression for greptile P2: DispatcherFor("") returns a stdout
// dispatcher whose Dispatch must NOT silently drop the alert when the
// embedded writer hasn't been wired via SetStdoutWriter. The CLI path
// always wires one, but library callers (and the eventual
// flight-watch-check cron path that calls Check with Dispatcher=nil)
// would otherwise lose alerts.
func TestStdoutDispatcherFallsBackToOsStdout(t *testing.T) {
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()

	d := DispatcherFor("") // intentionally no SetStdoutWriter
	res := CheckResult{
		Schema:        "flight-goat.watch.check.v1",
		WatchID:       "watch_test",
		Airline:       "DL",
		FlightNo:      "668",
		Date:          "2026-06-21",
		Origin:        "SFO",
		Destination:   "JFK",
		OriginalPrice: 700,
		Currency:      "USD",
		Confidence:    MatchExact,
		SafetyNotice:  SafetyNoticeText,
	}
	if err := d.Dispatch(context.Background(), res); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	_ = w.Close()
	buf, _ := io.ReadAll(r)
	if len(buf) == 0 {
		t.Fatalf("DispatcherFor(\"\").Dispatch wrote nothing; the nil-writer fallback dropped the alert")
	}
	if !strings.Contains(string(buf), "watch_test") || !strings.Contains(string(buf), "Verify fare rules") {
		t.Fatalf("fallback dispatcher output missing expected fields: %q", string(buf))
	}
}
