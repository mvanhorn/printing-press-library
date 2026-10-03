package carstay

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/carstay/internal/cliutil"
)

func TestClientPublicReadContracts(t *testing.T) {
	var pages []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
			t.Error("not a public read")
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/ja/api/data/no-route/stations":
			fmt.Fprintf(w, "[%s]", sourceFixture(""))
		case "/ja/api/data/stations/kinki/station/" + fixtureID:
			fmt.Fprintf(w, `{"station":%s,"orders":[{"secret":"discard"}],"reviews":[{"name":"discard"}]}`, sourceFixture(`,"length":8`))
		case "/ja/api/data/stations":
			pages = append(pages, r.URL.Query().Get("page"))
			if r.URL.Query().Get("checkIn") != "2026-10-10" || r.URL.Query().Get("checkOut") != "2026-10-11" {
				t.Error("JST date wire changed")
			}
			fixture := string(sourceFixture(""))
			if r.URL.Query().Get("page") == "2" {
				fixture = strings.Replace(fixture, fixtureID, "000000000000000000000001", 1)
			}
			fmt.Fprintf(w, `{"total":2,"stations":[%s]}`, fixture)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	c := New(srv.URL, 2)
	c.limiter = nil
	dir, e := c.Directory(context.Background())
	if e != nil || len(dir) != 1 {
		t.Fatalf("directory %v", e)
	}
	d, e := c.Detail(context.Background(), dir[0])
	if e != nil || d.ParkingSpace.Length == nil || *d.ParkingSpace.Length != 8 {
		t.Fatalf("detail %v", e)
	}
	dates, e := c.DateCandidates(context.Background(), "ja", "2026-10-10", "2026-10-11", 2)
	if e != nil || len(dates.Spots) != 2 || dates.Pages != 2 || dates.Scanned != 2 || strings.Join(pages, ",") != "1,2" {
		t.Fatalf("date paging %+v %v %v", dates, pages, e)
	}
}
func TestClientFailureAndEmptyDistinctions(t *testing.T) {
	for _, tc := range []struct {
		name            string
		status          int
		content, body   string
		rate, wantError bool
	}{{"rate-limited", 429, "application/json", `[]`, true, true}, {"notfound", 404, "application/json", `[]`, false, true}, {"HTML-wall", 200, "text/html", `<html>login</html>`, false, true}, {"null-shape", 200, "application/json", `null`, false, true}, {"bad-shape", 200, "application/json", `{"error":"failed"}`, false, true}, {"genuine-empty-array", 200, "application/json", `[]`, false, false}} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", tc.content)
				w.Header().Set("Retry-After", "2")
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer srv.Close()
			c := New(srv.URL, 2)
			data, e := c.Directory(context.Background())
			if (e != nil) != tc.wantError {
				t.Fatalf("data%v err%v", data, e)
			}
			if tc.rate {
				var r *cliutil.RateLimitError
				if !errors.As(e, &r) || r.RetryAfter != 2*time.Second {
					t.Fatalf("typed rate limit lost: %v", e)
				}
			}
			if !tc.wantError && data == nil {
				t.Fatal("empty is null")
			}
		})
	}
}
func TestClientContextAndDatedShapeValidation(t *testing.T) {
	for _, tc := range []struct{ name, body string }{{"missing-stations", `{"total":2}`}, {"null-stations", `{"total":2,"stations":null}`}, {"duplicate-pages", `{"total":2,"stations":[` + string(sourceFixture("")) + `]}`}} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, tc.body)
			}))
			defer srv.Close()
			c := New(srv.URL, 2)
			c.limiter = nil
			if _, e := c.DateCandidates(context.Background(), "ja", "2026-10-10", "2026-10-11", 2); e == nil {
				t.Fatal("bad dated contract accepted")
			}
		})
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if _, e := New(srv.URL, 2).Directory(ctx); e == nil {
		t.Fatal("timeout ignored")
	}
	for _, tc := range []struct {
		lang, in, out string
		pages         int
	}{{"xx", "2026-10-10", "2026-10-11", 2}, {"ja", "", "", 2}, {"ja", "2026-10-10", "2026-10-11", 11}} {
		if _, e := New(srv.URL, 2).DateCandidates(context.Background(), tc.lang, tc.in, tc.out, tc.pages); e == nil {
			t.Fatal("invalid candidate input accepted")
		}
	}
}

func TestDatedLanguageProvenance(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/en/api/data/stations" {
			t.Errorf("wrong language route %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"total":1,"stations":[%s]}`, sourceFixture(""))
	}))
	defer srv.Close()
	c := New(srv.URL, 2)
	c.limiter = nil
	dates, err := c.DateCandidates(context.Background(), "en", "2026-10-10", "2026-10-11", 1)
	if err != nil || len(dates.Spots) != 1 {
		t.Fatalf("dated source: %+v %v", dates, err)
	}
	spot := dates.Spots[0]
	if spot.SourceLanguage != "en" || !strings.Contains(spot.SourceURL, "/ja/") || spot.Name == "" {
		t.Fatalf("language provenance changed names or canonical handoff: %+v", spot)
	}
}

func TestNonfiniteRateCannotDisablePublicPacing(t *testing.T) {
	for _, rate := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		var calls atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls.Add(1)
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, "[%s]", sourceFixture(""))
		}))
		c := New(srv.URL, rate)
		if _, err := c.Directory(context.Background()); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
		_, err := c.Directory(ctx)
		cancel()
		srv.Close()
		if !errors.Is(err, context.DeadlineExceeded) || calls.Load() != 1 {
			t.Fatalf("nonfinite rate bypassed pacing: rate%v calls%d err%v", rate, calls.Load(), err)
		}
	}
}
