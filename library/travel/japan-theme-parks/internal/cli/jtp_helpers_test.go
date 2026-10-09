// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/japan-theme-parks/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/japan-theme-parks/internal/parks"
	"github.com/mvanhorn/printing-press-library/library/travel/japan-theme-parks/internal/sources"
)

func TestParseHourRange(t *testing.T) {
	cases := []struct {
		in      string
		lo, hi  int
		wantErr bool
	}{
		{"", 0, 23, false},
		{"10", 10, 10, false},
		{"9-12", 9, 12, false},
		{"12-9", 0, 0, true},
		{"24", 0, 0, true},
		{"x", 0, 0, true},
	}
	for _, tc := range cases {
		lo, hi, err := parseHourRange(tc.in)
		if (err != nil) != tc.wantErr || (!tc.wantErr && (lo != tc.lo || hi != tc.hi)) {
			t.Errorf("parseHourRange(%q) = %d,%d,%v", tc.in, lo, hi, err)
		}
	}
}

func TestResolveDateRange(t *testing.T) {
	today := time.Date(2026, 10, 9, 0, 0, 0, 0, parks.Tokyo)
	cases := []struct {
		name     string
		args     []string
		from, to string
		wantFrom string
		wantTo   string
		wantErr  bool
	}{
		{"single arg", []string{"2026-11-14"}, "", "", "2026-11-14", "2026-11-14", false},
		{"range", nil, "2026-11-13", "2026-11-15", "2026-11-13", "2026-11-15", false},
		{"relative", nil, "today", "+7d", "2026-10-09", "2026-10-16", false},
		{"tomorrow", []string{"tomorrow"}, "", "", "2026-10-10", "2026-10-10", false},
		{"past", []string{"2026-10-01"}, "", "", "", "", true},
		{"reversed", nil, "2026-11-15", "2026-11-13", "", "", true},
		{"too long", nil, "2026-11-01", "2027-03-01", "", "", true},
		{"92 days inclusive", nil, "2026-11-01", "2027-01-31", "2026-11-01", "2027-01-31", false},
		{"93 days inclusive", nil, "2026-11-01", "2027-02-01", "", "", true},
		{"both forms", []string{"2026-11-14"}, "2026-11-14", "", "", "", true},
		{"bad format", []string{"14/11/2026"}, "", "", "", "", true},
		{"nothing", nil, "", "", "", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f, to, err := resolveDateRange(tc.args, tc.from, tc.to, today)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("want error, got %s..%s", f.Format("2006-01-02"), to.Format("2006-01-02"))
				}
				return
			}
			if err != nil || f.Format("2006-01-02") != tc.wantFrom || to.Format("2006-01-02") != tc.wantTo {
				t.Fatalf("got %s..%s, %v", f.Format("2006-01-02"), to.Format("2006-01-02"), err)
			}
		})
	}
}

func TestTicketMatches(t *testing.T) {
	tk := parks.TDRTicket{ID: "T7", NameEN: "Early Evening Passport", NameJA: "アーリーイブニングパスポート"}
	cases := []struct {
		filter []string
		want   bool
	}{
		{nil, true},
		{[]string{"t7"}, true},
		{[]string{"evening"}, true},
		{[]string{"イブニング"}, true},
		{[]string{"T1"}, false},
	}
	for _, tc := range cases {
		if got := ticketMatches(tk, tc.filter); got != tc.want {
			t.Errorf("ticketMatches(%v) = %v", tc.filter, got)
		}
	}
}

func TestSortRides(t *testing.T) {
	w := func(n int) *int { return &n }
	d := func(f float64) *float64 { return &f }
	rides := []rideView{
		{Name: "closed"},
		{Name: "short", WaitMinutes: w(5), DeltaMinutes: d(-20)},
		{Name: "long", WaitMinutes: w(60), DeltaMinutes: d(10)},
	}
	sortRides(rides, "wait")
	if rides[0].Name != "long" || rides[2].Name != "closed" {
		t.Fatalf("wait sort = %v", rides)
	}
	sortRides(rides, "delta")
	if rides[0].Name != "short" || rides[2].Name != "closed" {
		t.Fatalf("delta sort = %v", rides)
	}
}

func TestMonthShouldBeOnSale(t *testing.T) {
	now := time.Date(2026, 10, 9, 12, 0, 0, 0, parks.Tokyo)
	cases := map[string]bool{"202610": true, "202612": true, "202701": false, "bad": false}
	for ym, want := range cases {
		if got := monthShouldBeOnSale(ym, now); got != want {
			t.Errorf("monthShouldBeOnSale(%s) = %v", ym, got)
		}
	}
}

func TestResolveDBPath(t *testing.T) {
	for _, bad := range []string{"x.db?mode=ro", "a#b.db"} {
		if _, err := resolveDBPath(bad); err == nil {
			t.Errorf("resolveDBPath(%q) accepted", bad)
		}
	}
	if p, err := resolveDBPath("/tmp/h.db"); err != nil || p != "/tmp/h.db" {
		t.Errorf("plain path: %q %v", p, err)
	}
}

func TestFillTDRRowUnknownMonth(t *testing.T) {
	p, _ := parks.Lookup("tds")
	d := time.Date(2026, 12, 20, 0, 0, 0, 0, parks.Tokyo)
	cases := []struct {
		name     string
		st       *tdrMonthState
		wantRule bool
	}{
		{"fetch failed", &tdrMonthState{err: errors.New("timeout"), reason: "TDR ticket calendar fetch failed: timeout"}, false},
		{"unpublished", &tdrMonthState{reason: "TDR has not published the ticket calendar for this month yet"}, true},
		{"404", &tdrMonthState{noPopup: true, reason: "TDR has no ticket calendar page for this month yet (HTTP 404)"}, true},
	}
	for _, tc := range cases {
		row := dateRow{}
		fillTDRRow(&row, p, d, tc.st, nil)
		if row.TicketsUnknownReason == "" || row.Hours.UnknownReason == "" || row.Tickets != nil {
			t.Errorf("%s: row = %+v", tc.name, row)
		}
		if (row.SaleOpensAt != nil) != tc.wantRule {
			t.Errorf("%s: sale_opens_at present = %v", tc.name, row.SaleOpensAt != nil)
		}
	}
}

func TestFillThirdPartyHours(t *testing.T) {
	s, err := parks.ParseSchedule([]byte(`{"schedule":[{"date":"2026-10-10","type":"OPERATING","openingTime":"2026-10-10T08:30:00+09:00","closingTime":"2026-10-10T21:00:00+09:00"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	f := sources.Fetch{FetchedAt: time.Date(2026, 10, 9, 1, 0, 0, 0, time.UTC)}
	cases := []struct {
		name, date, open, reason string
		res                      scheduleResult
	}{
		{"in window", "2026-10-10", "08:30", "", scheduleResult{sched: &s, fetch: f}},
		{"beyond horizon", "2026-11-20", "", "beyond third-party source horizon", scheduleResult{sched: &s, fetch: f}},
		{"fetch failed", "2026-10-10", "", "ThemeParks.wiki schedule fetch failed", scheduleResult{err: "HTTP 503"}},
	}
	for _, tc := range cases {
		row := dateRow{}
		fillThirdPartyHours(&row, tc.date, tc.res)
		gotOpen := ""
		if row.Hours.Open != nil {
			gotOpen = *row.Hours.Open
		}
		if gotOpen != tc.open || !strings.HasPrefix(row.Hours.UnknownReason, tc.reason) {
			t.Errorf("%s: open=%q reason=%q", tc.name, gotOpen, row.Hours.UnknownReason)
		}
		if tc.res.sched != nil && (row.Hours.SourceKind != "third-party aggregator (not official)" || row.Hours.FetchedAt == "" || row.Hours.Attribution == "") {
			t.Errorf("%s: missing third-party labels: %+v", tc.name, row.Hours)
		}
	}
}

func TestStaleHistoryHint(t *testing.T) {
	cases := []struct {
		name   string
		maxAge time.Duration
		age    time.Duration
		want   bool
	}{
		{"fresh", 30 * time.Minute, 5 * time.Minute, false},
		{"stale", 30 * time.Minute, 2 * time.Hour, true},
		{"disabled", 0, 48 * time.Hour, false},
	}
	for _, tc := range cases {
		var buf bytes.Buffer
		staleHistoryHint(&buf, &rootFlags{maxAge: tc.maxAge}, time.Now().Add(-tc.age))
		if (buf.Len() > 0) != tc.want {
			t.Errorf("%s: hint=%q", tc.name, buf.String())
		}
	}
}

func TestTypicalFilters(t *testing.T) {
	f := typicalFilters("", "", "", 0, 23, 6)
	if f.HourFrom != nil || f.Ride != nil || f.Weekdays != nil || f.MinSamples != 6 {
		t.Fatalf("empty filters = %+v", f)
	}
	f = typicalFilters("Mario", "sat", "9-12", 9, 12, 3)
	if *f.HourFrom != 9 || *f.HourTo != 12 || *f.Ride != "Mario" || *f.Weekdays != "sat" {
		t.Fatalf("filters = %+v", f)
	}
}

func TestFetchTallyExitCodes(t *testing.T) {
	rl := &sources.FetchError{URL: "https://example.test/a", Status: 429, Err: &cliutil.RateLimitError{URL: "https://example.test/a"}}
	other := &sources.FetchError{URL: "https://example.test/b", Status: 503, Err: errors.New("HTTP 503")}
	cases := []struct {
		name     string
		errs     []error
		ok       int
		wantNil  bool
		wantCode int
	}{
		{"one success", []error{other}, 1, true, 0},
		{"all 429", []error{rl, rl}, 0, false, 7},
		{"mixed", []error{rl, other}, 0, false, 5},
	}
	for _, tc := range cases {
		meta := newMeta()
		tally := fetchTally{meta: &meta}
		for _, e := range tc.errs {
			tally.fail("src", e)
		}
		for i := 0; i < tc.ok; i++ {
			tally.succeed()
		}
		err := tally.allFailed("x")
		if (err == nil) != tc.wantNil {
			t.Fatalf("%s: err = %v", tc.name, err)
		}
		if err != nil && ExitCode(err) != tc.wantCode {
			t.Errorf("%s: exit = %d", tc.name, ExitCode(err))
		}
		if len(meta.FetchFailures) != len(tc.errs) || meta.FetchFailures[0].URL == "" {
			t.Errorf("%s: failures = %+v", tc.name, meta.FetchFailures)
		}
	}
}
