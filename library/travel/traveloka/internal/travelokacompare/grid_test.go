package travelokacompare

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/mvanhorn/printing-press-library/library/travel/traveloka/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/traveloka/internal/traveloka"
	"testing"
	"time"
)

// Planner and fetch callbacks use SIMULATED data only.
var simulatedNow = time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)

func validFlightQuery() traveloka.Query {
	return traveloka.Query{Kind: "flights", Market: "SG", Locale: "en-SG", Currency: "SGD", Origin: "SIN", Destination: "CGK", Cabin: "ECONOMY", Adults: 2, Children: 1, Infants: 1, Limit: 2, MaxCandidates: 2}
}
func TestFlightGridSimulated(t *testing.T) {
	cases := []struct {
		name, depart, returns string
		cap, count            int
		fail                  bool
	}{
		{"one_way", "2027-01-06,2027-01-07", "", 4, 2, false},
		{"return_cartesian", "2027-01-06,2027-01-07", "2027-01-10,2027-01-11", 4, 4, false},
		{"later_invalid_pair", "2027-01-06,2027-01-12", "2027-01-10", 4, 0, true},
		{"invalid_calendar", "2027-02-30", "", 4, 0, true},
		{"cap_exceeded", "2027-01-06,2027-01-07", "2027-01-10,2027-01-11", 3, 0, true},
		{"invalid_cap", "2027-01-06", "", 10, 0, true},
		{"missing_list", "", "", 4, 0, true},
		{"duplicate_date", "2027-01-06,2027-01-06", "", 4, 0, true},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			q := validFlightQuery()
			cells, e := FlightGrid(q, tt.depart, tt.returns, tt.cap, simulatedNow)
			if (e != nil) != tt.fail || len(cells) != tt.count {
				t.Fatalf("planner: %d cells %v", len(cells), e)
			}
			for _, cell := range cells {
				if cell.Adults != 2 || cell.Children != 1 || cell.Infants != 1 || cell.Currency != "SGD" || cell.Cabin != "ECONOMY" {
					t.Fatal("fixed query context changed")
				}
			}
		})
	}
}
func TestHotelGridSimulated(t *testing.T) {
	cases := []struct {
		name, stays string
		cap, count  int
		fail        bool
	}{
		{"equal_lengths", "2027-01-06:2027-01-08,2027-01-13:2027-01-15", 4, 2, false},
		{"unequal_lengths", "2027-01-06:2027-01-08,2027-01-13:2027-01-14", 4, 0, true},
		{"invalid_later_date", "2027-01-06:2027-01-08,2027-02-30:2027-03-02", 4, 0, true},
		{"same_day", "2027-01-06:2027-01-06", 4, 0, true},
		{"missing_separator", "2027-01-06", 4, 0, true},
		{"cap_exceeded", "2027-01-06:2027-01-08,2027-01-13:2027-01-15", 1, 0, true},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			q := roomSnapshot().Query
			q.Children = 1
			q.ChildAges = []int{8}
			q.Rooms = 2
			q.Limit = 2
			cells, e := HotelGrid(q, tt.stays, tt.cap, simulatedNow)
			if (e != nil) != tt.fail || len(cells) != tt.count {
				t.Fatalf("planner: %d cells %v", len(cells), e)
			}
			for _, cell := range cells {
				if cell.PropertyID != q.PropertyID || cell.Rooms != 2 || cell.Children != 1 || cell.ChildAges[0] != 8 {
					t.Fatal("party/property context changed")
				}
			}
		})
	}
	q := roomSnapshot().Query
	q.Children = 1
	if _, e := HotelGrid(q, "2027-01-06:2027-01-08", 4, simulatedNow); e == nil {
		t.Fatal("missing child ages inferred")
	}
}
func TestRunGridSimulatedPreservesFailuresAndCurtailment(t *testing.T) {
	queries, e := FlightGrid(validFlightQuery(), "2027-01-06,2027-01-07,2027-01-08", "", 4, simulatedNow)
	if e != nil {
		t.Fatal(e)
	}
	cases := []struct {
		name                                    string
		cap                                     int
		failCode                                string
		status                                  int
		wantAttempts, wantFailures, wantSkipped int
	}{
		{"protection_then_success", 3, "ACCESS_BLOCKED", 202, 3, 1, 0},
		{"auth_error_then_success", 3, "AUTH_REQUIRED", 403, 3, 1, 0},
		{"upstream_error_then_success", 3, "UPSTREAM_ERROR", 500, 3, 1, 0},
		{"bounded_attempts", 2, "ACCESS_BLOCKED", 202, 2, 1, 1},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			calls, saved := 0, 0
			fetch := func(ctx context.Context, q traveloka.Query) (*traveloka.Snapshot, error) {
				calls++
				if calls == 1 {
					return nil, &traveloka.APIError{Code: tt.failCode, Status: tt.status, Message: "SIMULATED-error"}
				}
				s := flightSnapshot(flight("SIMULATED-priced", "15900", 0, 100))
				s.Query = q
				s.Status = "success"
				return s, nil
			}
			save := func(context.Context, *traveloka.Snapshot) error { saved++; return nil }
			r := RunGrid(context.Background(), queries, tt.cap, fetch, save)
			if len(r.Cells) != 3 || r.Attempted != tt.wantAttempts || r.FetchFailureCount != tt.wantFailures || r.NotAttempted != tt.wantSkipped || calls != r.Attempted || saved != r.Attempted-r.FetchFailureCount {
				t.Fatalf("lost cells/counts %+v", r)
			}
			if r.Cells[0].LowestRetrievedTotal != nil || r.Cells[0].StatusCode != tt.status || r.Cells[0].Error.Code != tt.failCode || r.Cells[0].CompletedAt == "" {
				t.Fatal("failed cell became inventory or lost source status/time")
			}
			if len(r.FetchFailures) != 1 || r.FetchFailures[0].CellIndex != 0 || r.FetchFailures[0].Error.Message != "SIMULATED-error" || r.FetchFailures[0].Error.Status != tt.status || r.FetchFailures[0].At == "" || r.FetchFailures[0].Query.Depart != "2027-01-06" {
				t.Fatalf("structured fetch failures lost source error/query/time: %+v", r.FetchFailures)
			}
			if r.Cells[1].LowestRetrievedTotal.Amount != "159.00" || r.Cells[1].Snapshot.Query.Depart != "2027-01-07" {
				t.Fatal("source price/context lost")
			}
			if tt.wantSkipped > 0 && (r.Cells[2].Attempted || r.Cells[2].LowestRetrievedTotal != nil || r.Cells[2].Status != "not_attempted") {
				t.Fatal("curtailed cell invented a quote")
			}
		})
	}
}
func TestRunGridSimulatedEmptyDeadlineAndSaveFailure(t *testing.T) {
	queries, _ := FlightGrid(validFlightQuery(), "2027-01-06", "", 4, simulatedNow)
	cases := []struct {
		name                string
		fetch               Fetch
		save                Save
		status              string
		hasPrice            bool
		fetchFail, saveFail int
	}{
		{"empty_inventory", func(_ context.Context, q traveloka.Query) (*traveloka.Snapshot, error) {
			s := flightSnapshot()
			s.Query = q
			s.Status = "no_inventory"
			return s, nil
		}, nil, "no_inventory", false, 0, 0},
		{"nil_snapshot", func(context.Context, traveloka.Query) (*traveloka.Snapshot, error) { return nil, nil }, nil, "fetch_failed", false, 1, 0},
		{"save_error", func(_ context.Context, q traveloka.Query) (*traveloka.Snapshot, error) {
			s := flightSnapshot(flight("a", "100", 0, 100))
			s.Query = q
			s.Status = "success"
			return s, nil
		}, func(context.Context, *traveloka.Snapshot) error { return errors.New("SIMULATED-save-error") }, "snapshot_save_failed", true, 0, 1},
		{"wrong_context", func(_ context.Context, q traveloka.Query) (*traveloka.Snapshot, error) {
			s := flightSnapshot(flight("a", "100", 0, 100))
			s.Query = q
			s.Query.Adults++
			return s, nil
		}, nil, "fetch_failed", false, 1, 0},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			r := RunGrid(context.Background(), queries, 1, tt.fetch, tt.save)
			if r.Cells[0].Status != tt.status || (r.Cells[0].LowestRetrievedTotal != nil) != tt.hasPrice || r.FetchFailureCount != tt.fetchFail || r.SaveFailureCount != tt.saveFail || len(r.FetchFailures) != tt.fetchFail || len(r.SaveFailures) != tt.saveFail {
				t.Fatalf("wrong error/price handling %+v", r)
			}
			if tt.saveFail > 0 && r.SaveFailures[0].Error.Message != "SIMULATED-save-error" {
				t.Fatal("save failure message lost")
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	called := false
	r := RunGrid(ctx, queries, 1, func(context.Context, traveloka.Query) (*traveloka.Snapshot, error) { called = true; return nil, nil }, nil)
	if called || r.NotAttempted != 1 || r.Cells[0].Status != "not_attempted" {
		t.Fatal("cancelled grid performed IO")
	}
}
func TestRunGridSimulatedRateLimitPreservesTypedCause(t *testing.T) {
	queries, _ := FlightGrid(validFlightQuery(), "2027-01-06", "", 4, simulatedNow)
	original := &cliutil.RateLimitError{URL: "https://www.traveloka.com/api/v2/flight/search/poll", Body: "SIMULATED-throttle", Cause: &traveloka.APIError{Code: "UPSTREAM_ERROR", Message: "SIMULATED-upstream", Status: 429}}
	r := RunGrid(context.Background(), queries, 1, func(context.Context, traveloka.Query) (*traveloka.Snapshot, error) { return nil, original }, nil)
	if r.FetchFailureCount != 1 || r.Cells[0].Error.Code != "RATE_LIMITED" || r.Cells[0].StatusCode != 429 || r.Cells[0].Cause != original || r.Cells[0].LowestRetrievedTotal != nil {
		t.Fatalf("rate limit became generic error/price %+v", r)
	}
}

func TestRunGridHotelMinimumRequiresMatchingPartySimulated(t *testing.T) {
	cases := []struct {
		name  string
		edit  func(*traveloka.Offer)
		valid bool
	}{
		{"matching", func(*traveloka.Offer) {}, true},
		{"room_count_fallback", func(o *traveloka.Offer) { delete(object(o.Details["rate"]), "numChargedRooms") }, true},
		{"integer_count", func(o *traveloka.Offer) { object(o.Details["rate"])["numChargedRooms"] = json.Number("2") }, true},
		{"occupancy_mismatch", func(o *traveloka.Offer) { o.OccupancyMatch = ptr(false) }, false},
		{"occupancy_unknown", func(o *traveloka.Offer) { o.OccupancyMatch = nil }, false},
		{"rate_count_mismatch", func(o *traveloka.Offer) { object(o.Details["rate"])["numChargedRooms"] = "1" }, false},
		{"room_count_conflict", func(o *traveloka.Offer) { object(o.Details["room"])["numChargedRooms"] = "1" }, false},
		{"rate_null_stays_unknown", func(o *traveloka.Offer) { object(o.Details["rate"])["numChargedRooms"] = nil }, false},
		{"room_null_stays_unknown", func(o *traveloka.Offer) { object(o.Details["room"])["numChargedRooms"] = nil }, false},
		{"unknown_count", func(o *traveloka.Offer) { object(o.Details["rate"])["numChargedRooms"] = "UNKNOWN" }, false},
		{"no_count", func(o *traveloka.Offer) {
			delete(object(o.Details["rate"]), "numChargedRooms")
			delete(object(o.Details["room"]), "numChargedRooms")
		}, false},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			matching, alternative := room("SIMULATED-matching", true, "20000"), room("SIMULATED-alternative", true, "10000")
			object(matching.Details["rate"])["numChargedRooms"] = "2"
			object(alternative.Details["rate"])["numChargedRooms"] = "2"
			alternative.Details["room"] = map[string]any{"numChargedRooms": "2"}
			tt.edit(&alternative)
			s := roomSnapshot(matching, alternative)
			s.Query.Adults, s.Query.Rooms = 3, 2
			s.Status, s.SearchComplete = "success", true
			fetch := func(context.Context, traveloka.Query) (*traveloka.Snapshot, error) { return s, nil }
			r := RunGrid(context.Background(), []traveloka.Query{s.Query}, 1, fetch, nil)
			want := "20000"
			if tt.valid {
				want = "10000"
			}
			if r.Cells[0].LowestRetrievedTotal == nil || r.Cells[0].LowestRetrievedTotal.MinorUnits != want || r.Cells[0].Status != "success" || r.Cells[0].Snapshot != s || len(s.Offers) != 2 {
				t.Fatalf("fixed-party minimum or retained alternatives wrong: %+v", r)
			}
			if !tt.valid {
				s.Offers = []traveloka.Offer{alternative}
				r = RunGrid(context.Background(), []traveloka.Query{s.Query}, 1, fetch, nil)
				if r.Cells[0].LowestRetrievedTotal != nil || len(r.Cells[0].Snapshot.Offers) != 1 || r.Cells[0].Status != "success" {
					t.Fatal("unknown/mismatched-only inventory invented a matching price or lost its source outcome")
				}
			}
		})
	}
}

func TestRunGridPreservesContextCancellationSimulated(t *testing.T) {
	queries, e := FlightGrid(validFlightQuery(), "2027-01-06,2027-01-07", "", 4, simulatedNow)
	if e != nil {
		t.Fatal(e)
	}
	for _, deadline := range []bool{false, true} {
		name := "cancelled"
		ctx, cancel := context.WithCancel(context.Background())
		want := context.Canceled
		if deadline {
			cancel()
			ctx, cancel = context.WithDeadline(context.Background(), time.Unix(0, 0))
			name, want = "deadline", context.DeadlineExceeded
		} else {
			cancel()
		}
		t.Run(name, func(t *testing.T) {
			defer cancel()
			calls := 0
			r := RunGrid(ctx, queries, len(queries), func(context.Context, traveloka.Query) (*traveloka.Snapshot, error) { calls++; return nil, nil }, nil)
			if calls != 0 || r.Status != "failed" || r.Attempted != 0 || r.NotAttempted != len(queries) || len(r.Cells) != len(queries) || !errors.Is(r.ContextErr, want) {
				t.Fatalf("zero-attempt context failure lost: %+v", r)
			}
			for _, cell := range r.Cells {
				if cell.Status != "not_attempted" || cell.Attempted || cell.LowestRetrievedTotal != nil || !errors.Is(cell.Cause, want) {
					t.Fatal("curtailed cell lost its context cause or became a retrieval")
				}
			}
			b, e := json.Marshal(r)
			if e != nil {
				t.Fatal(e)
			}
			var view map[string]any
			if e = json.Unmarshal(b, &view); e != nil {
				t.Fatal(e)
			}
			if _, leaked := view["ContextErr"]; leaked {
				t.Fatal("internal context cause leaked into grid JSON")
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r := RunGrid(ctx, queries, len(queries), func(_ context.Context, q traveloka.Query) (*traveloka.Snapshot, error) {
		s := flightSnapshot(flight("SIMULATED-priced", "15900", 0, 100))
		s.Query, s.Status = q, "success"
		cancel()
		return s, nil
	}, nil)
	if r.Status != "partial" || r.Attempted != 1 || r.NotAttempted != 1 || !errors.Is(r.ContextErr, context.Canceled) || r.Cells[0].LowestRetrievedTotal == nil || r.Cells[1].LowestRetrievedTotal != nil {
		t.Fatalf("partially retrieved cancellation lost cells/price/context: %+v", r)
	}
	r = RunGrid(context.Background(), queries, 0, nil, nil)
	if r.Status != "partial" || r.ContextErr != nil || r.Attempted != 0 || r.NotAttempted != len(queries) {
		t.Fatal("harness cap was mistaken for a cancelled context")
	}
}
