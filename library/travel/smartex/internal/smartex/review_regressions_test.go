package smartex

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/smartex/internal/cliutil"
)

func TestWindowDoesNotPromiseAdvanceOrProcessingSeatMaps(t *testing.T) {
	for _, tc := range []struct{ date, now, class, product string }{
		{"2027-02-28", "2026-10-02T12:00:00+09:00", "reserved", "smart-ex"},
		{"2026-11-02", "2026-10-02T08:30:00+09:00", "reserved", "smart-ex"},
		{"2026-10-20", "2026-10-02T12:00:00+09:00", "unreserved", "smart-ex"},
		{"2026-10-08", "2026-10-02T12:00:00+09:00", "unreserved", "hayatoku1"},
	} {
		w, err := BookingWindow(tc.date, tc.product, "", mustNow(tc.now), 1, 0, false, tc.class)
		if err != nil || w.SeatMapAvailable {
			t.Fatalf("unavailable calendar/class map advertised: %+v %v", w, err)
		}
	}
}

func TestLeapDayUnknownAnnualOpeningUsesKnownMonthlyFallback(t *testing.T) {
	w, err := BookingWindow("2028-02-29", "smart-ex", "", mustNow("2027-03-01T12:00:00+09:00"), 1, 0, false, "reserved")
	if err != nil || w.Opens != nil || w.ReservationPermitted != nil || w.SeatMapAvailable {
		t.Fatalf("unknown annual opening should not deny or approve permission: %+v %v", w, err)
	}
	w, err = BookingWindow("2028-02-29", "smart-ex", "", mustNow("2028-02-01T12:00:00+09:00"), 1, 0, false, "reserved")
	if err != nil || w.Opens != nil || w.WindowStatus != "within_calendar_window" || w.ReservationPermitted != true || !w.SeatMapAvailable || w.StandardSeatSales != "2028-01-29T10:00:00+09:00" {
		t.Fatalf("known monthly fallback lost: %+v %v", w, err)
	}
}

func TestOversizedWindowPreservesSourceConflictsAndClassPartyBounds(t *testing.T) {
	for _, tc := range []struct {
		class          string
		party, maximum int
		fits           bool
	}{{"reserved", 5, 5, true}, {"reserved", 6, 5, false}, {"green", 4, 4, true}, {"green", 5, 4, false}} {
		w, err := BookingWindow("2026-10-20", "smart-ex", "", mustNow("2026-10-02T12:00:00+09:00"), tc.party, 0, true, tc.class)
		if err != nil || w.MaxPartyNow != tc.maximum || w.PartyFits != tc.fits || w.OneYearRequestEligible != nil || w.Opens != nil || len(w.OversizedSourceRules) != 2 || w.StandardSeatSales != "2026-09-20T10:00:00+09:00" {
			t.Fatalf("oversized limits/conflicts lost: %+v %v", w, err)
		}
	}
	w, _ := BookingWindow("2027-02-28", "smart-ex", "", mustNow("2026-10-02T12:00:00+09:00"), 1, 0, true, "reserved")
	if w.WindowStatus != "advance_eligibility_requires_confirmation" || w.ReservationPermitted != nil || w.SeatMapAvailable {
		t.Fatalf("advance eligibility falsely confirmed: %+v", w)
	}
	w, _ = BookingWindow("2026-10-20", "smart-ex", "", mustNow("2026-10-02T23:30:00+09:00"), 1, 0, true, "reserved")
	if w.PartyFits != nil || w.ReservationPermitted != nil || w.SeatMapAvailable || w.WindowStatus != "after_hours_eligibility_requires_confirmation" {
		t.Fatalf("overnight oversized booking falsely confirmed: %+v", w)
	}
	if _, err := BookingWindow("2026-10-20", "smart-ex", "", mustNow("2026-10-02T12:00:00+09:00"), 1, 0, true, "unreserved"); err == nil {
		t.Fatal("unreserved oversized area accepted")
	}
}

func TestFareQuoteFailuresRetainThrottleAcrossFailureOrder(t *testing.T) {
	for _, codes := range [][]int{{429, 429, 429}, {429, 503, 503}, {503, 429, 503}, {200, 429, 503}} {
		t.Run(strings.Trim(strings.ReplaceAll(http.StatusText(codes[0]), " ", "_"), "_"), func(t *testing.T) {
			request := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				request++
				if request == 1 {
					_, _ = w.Write(liveFixture(t, "fare-step1"))
					return
				}
				code := codes[request-2]
				w.WriteHeader(code)
				if code == 200 {
					_, _ = w.Write(liveFixture(t, "fare-reserved"))
				}
			}))
			defer srv.Close()
			c := NewClient()
			c.HTTP = srv.Client()
			c.endpoint = srv.URL
			c.limiter = cliutil.NewAdaptiveLimiter(0)
			got, err := c.Fares(context.Background(), "Tokyo", "Shin-Osaka", "2026-10-02", "all", 1, 0, mustNow("2026-10-02T12:00:00+09:00"))
			if codes[0] == 200 {
				if err != nil || len(got.Quotes) != 1 || len(got.FetchFailures) != 2 {
					t.Fatalf("partial quotes lost: %+v %v", got, err)
				}
			} else {
				var rate *cliutil.RateLimitError
				if !errors.As(err, &rate) || len(got.FetchFailures) != 3 {
					t.Fatalf("typed aggregate throttle lost: %+v %v", got, err)
				}
			}
		})
	}
}

type reviewTransport func(*http.Request) (*http.Response, error)

func (f reviewTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestTimetableReturnsCurrentLinksAndLeavesOfflineComparisonUnknown(t *testing.T) {
	c := NewClient()
	c.limiter = cliutil.NewAdaptiveLimiter(0)
	west := "https://global.jr-central.co.jp/en/info/timetable/_pdf/shinkansen_west_bound2703.pdf"
	east := "https://global.jr-central.co.jp/en/info/timetable/_pdf/shinkansen_east_bound2703.pdf"
	c.HTTP = &http.Client{Transport: reviewTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("<html><body><a href='" + west + "'>West</a><a href='" + east + "'>East</a></body></html>")), Request: r}, nil
	})}
	got, err := c.Timetable(context.Background(), "Tokyo", "Shin-Osaka", "2026-10-02", "", "", 5, false, false)
	if err != nil || got["snapshot_matches_current_pdf_links"] != false || len(got["services"].([]TimetableMatch)) != 0 || got["publications"].([]string)[0] != west {
		t.Fatalf("publication drift misstated: %+v %v", got, err)
	}
	got, err = c.Timetable(context.Background(), "Tokyo", "Shin-Osaka", "2026-10-02", "", "", 5, false, true)
	if err != nil || got["snapshot_matches_current_pdf_links"] != nil || len(got["publications"].([]string)) != 0 || len(got["snapshot_publications"].([]string)) != 2 {
		t.Fatalf("offline comparison falsely verified: %+v %v", got, err)
	}
}

func TestSourceFailuresPreserveTypedThrottle(t *testing.T) {
	c := NewClient()
	c.limiter = cliutil.NewAdaptiveLimiter(0)
	c.HTTP = &http.Client{Transport: reviewTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 429, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("")), Request: r}, nil
	})}
	check := c.Source(context.Background(), Sources[4], true)
	var rate *cliutil.RateLimitError
	if !errors.As(check.Err, &rate) || !errors.As(AllSourceFailures([]SourceCheck{{Err: context.DeadlineExceeded}, check}), &rate) {
		t.Fatal("source aggregation lost typed throttle")
	}
	_, err := c.Timetable(context.Background(), "Tokyo", "Shin-Osaka", "", "", "", 5, false, false)
	if !errors.As(err, &rate) {
		t.Fatalf("timetable lost source throttle: %v", err)
	}
}
