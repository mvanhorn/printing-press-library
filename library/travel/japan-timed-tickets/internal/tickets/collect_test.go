// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.

package tickets

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// fakeTransport answers by host+path without network access.
type fakeTransport map[string]func() (int, string)

func (ft fakeTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	h, ok := ft[r.URL.Host+r.URL.Path]
	if !ok {
		h, ok = ft["*"]
	}
	code, body := http.StatusNotFound, "not found"
	if ok {
		code, body = h()
	}
	return &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"Retry-After": {"0"}}, Request: r}, nil
}

func fakeFetcher(ft fakeTransport) *Fetcher {
	f := NewFetcher(5*time.Second, MaxPerHostRate, 200)
	f.HTTP.Transport = ft
	f.backoff = func(int) time.Duration { return time.Millisecond }
	f.noPacing = true
	f.maxRetryWait = time.Millisecond
	return f
}

var fixedNow = func() time.Time { return time.Date(2026, 10, 9, 11, 0, 0, 0, JST) }

func TestCollectClassifiesFailures(t *testing.T) {
	kyoto, _ := Lookup("teamlab-kyoto")
	sky, _ := Lookup("shibuya-sky")
	ft := fakeTransport{"*": func() (int, string) { return http.StatusTooManyRequests, "slow down" }}
	from := Date{2026, 11, 2}
	data, failures := Collect(context.Background(), fakeFetcher(ft), []Sight{kyoto, sky}, from, from, nil, fixedNow)
	if len(failures) != 2 || !failures[0].RateLimited || !IsRateLimited(failures) {
		t.Fatalf("failures = %+v, want 2 rate-limited", failures)
	}
	if data["teamlab-kyoto"] == nil || data["shibuya-sky"] == nil {
		t.Fatal("throttled sights must keep their data so rows fall back to stored rules")
	}
	rows := BuildOnsale(data, []Sight{kyoto, sky}, from, from, fixedNow(), nil)
	if len(rows) != 2 {
		t.Errorf("rows = %d, want 2 (one per sight, not dropped)", len(rows))
	}
	for _, f := range failures {
		if strings.Contains(f.Error, "no longer shows") {
			t.Errorf("a fetch failure must not be reported as rule drift: %q", f.Error)
		}
	}

	// Non-throttle failure: not rate limited.
	ft = fakeTransport{"*": func() (int, string) { return http.StatusForbidden, "no" }}
	_, failures = Collect(context.Background(), fakeFetcher(ft), []Sight{kyoto}, from, from, nil, fixedNow)
	if len(failures) != 1 || failures[0].RateLimited || IsRateLimited(failures) {
		t.Errorf("403 failures = %+v, want not rate limited", failures)
	}
}

func TestCollectPartialThrottleKeepsData(t *testing.T) {
	kyoto, _ := Lookup("teamlab-kyoto")
	host := kyoto.Host
	ft := fakeTransport{
		host + "/api/v1/configurations": func() (int, string) {
			return 200, `[{"key":"Calendar_Period","value":"2026-12"}]`
		},
		host + "/api/v1/texts": func() (int, string) {
			return 200, `[{"key":"unpublished_calendar_notice","text":"11月2日 12時00分発売予定"}]`
		},
		host + "/api/v1/products/stocks-statuses": func() (int, string) {
			return 200, `{"records":[{"date":"2026-11-03T00:00:00+09:00","stock_status":"few"}]}`
		},
		"*": func() (int, string) { return http.StatusTooManyRequests, "" },
	}
	d1, d2 := Date{2026, 11, 3}, Date{2026, 11, 4}
	data, failures := Collect(context.Background(), fakeFetcher(ft), []Sight{kyoto}, d1, d2, []Date{d1}, fixedNow)
	if len(failures) != 0 {
		t.Fatalf("partial throttle must not fail the sight: %+v", failures)
	}
	sd := data["teamlab-kyoto"]
	if !sd.RateLimited || sd.Days["2026-11-03"].Status != DayFew {
		t.Errorf("sd = rate_limited %v, days %+v", sd.RateLimited, sd.Days)
	}
	if st := sd.Days["2026-11-04"].Status; st != DayUnknown {
		t.Errorf("date omitted by the calendar = %s, want unknown", st)
	}
	if sd.Release == nil || sd.Release.Covers != "2027-01" || sd.Release.Confidence != ConfAnnounced {
		t.Errorf("release = %+v, want covers 2027-01 announced", sd.Release)
	}
	found := false
	for _, w := range sd.Warnings {
		found = found || strings.Contains(w, "rate limited")
	}
	if !found {
		t.Errorf("warnings = %v, want a rate-limit note", sd.Warnings)
	}
}

func TestRequestBudgetCoversSlots(t *testing.T) {
	all := All()
	if got := RequestBudget(all, 7); got < 3+4+8+4*(3+3*7) {
		t.Errorf("budget %d too small for --slots over 7 dates", got)
	}
}

func TestInferYear(t *testing.T) {
	cases := []struct{ month, refY, refM, want int }{
		{1, 2026, 10, 2027},  // "1月のチケット" read in October
		{10, 2026, 10, 2026}, // same month
		{9, 2026, 10, 2026},  // last month
		{12, 2027, 1, 2026},  // December sale for January entry
		{4, 2026, 10, 2026},  // six months back stays
		{3, 2026, 10, 2027},  // seven months back rolls over
	}
	for _, c := range cases {
		if got := InferYear(c.month, c.refY, c.refM); got != c.want {
			t.Errorf("InferYear(%d, %d, %d) = %d, want %d", c.month, c.refY, c.refM, got, c.want)
		}
	}
}

func TestICSStripsInjectedText(t *testing.T) {
	now := fixedNow()
	at := "2026-11-02T12:00:00+09:00"
	rows := []OnsaleRow{{Sight: "teamlab-kyoto", NameEN: "teamLab Biovortex Kyoto", VisitDate: "2027-01-04", State: StateOpensAt, OnSaleJST: &at,
		SaleBasis: "ticket site notice: x\rURL:https://evil\x1b[2J\ttab\u202eRLO", HandoffURL: "https://kyoto.tickets.teamlab.art/#/?lang=en"}}
	cal, n := BuildICS(rows, now)
	if n != 1 {
		t.Fatalf("events = %d", n)
	}
	unfolded := strings.ReplaceAll(cal, "\r\n ", "")
	for _, line := range strings.Split(strings.TrimSuffix(unfolded, "\r\n"), "\r\n") {
		if strings.ContainsAny(line, "\r\x1b\t\u202e") {
			t.Errorf("control character survived: %q", line)
		}
		if strings.HasPrefix(line, "URL:https://evil") {
			t.Errorf("injected property: %q", line)
		}
	}
	if !strings.Contains(unfolded, "xURL:https://evil[2J tabRLO") {
		t.Errorf("description text not kept: %s", unfolded)
	}
	if _, n := BuildICS(nil, now); n != 0 {
		t.Error("no rows must give zero events")
	}
}

func TestFoldICSMultiByte(t *testing.T) {
	line := "SUMMARY:" + strings.Repeat("チームラボ", 10)
	for _, part := range strings.Split(foldICS(line), "\r\n") {
		if len(part) > 75 {
			t.Errorf("folded part has %d octets", len(part))
		}
		if !strings.HasPrefix(part, "SUMMARY") && !strings.HasPrefix(part, " ") {
			t.Errorf("continuation must start with a space: %q", part)
		}
	}
	if got := strings.ReplaceAll(foldICS(line), "\r\n ", ""); got != line {
		t.Error("unfolding must restore the line")
	}
}

func TestStripControlFormatChars(t *testing.T) {
	in := "a\u202eb\u2066c\u200bd\ufeffe\u200df\r"
	if got := StripControl(in); got != "abcde\u200df" {
		t.Errorf("StripControl = %q", got)
	}
}

func TestSkyPricesAndProducts(t *testing.T) {
	if !CheckSkyPrices(`"2,700円" "3,400円" "15:00以降の入場"`) {
		t.Error("prices present must pass")
	}
	if CheckSkyPrices(`"2,500円" "3,400円" "15:00以降の入場"`) {
		t.Error("changed price must fail")
	}
	ps, err := ParseTeamLabProducts([]byte(`{"products":[{"id":1,"uuid":"u-1","name":"Admission","sale_type":"time","is_few_stock":true},{"id":2,"uuid":"u-2","name":"Day pass","sale_type":"date"}]}`))
	if err != nil || len(ps) != 2 || ps[0].UUID != "u-1" || !ps[0].Few || ps[1].SaleType != "date" {
		t.Errorf("products = %+v, %v", ps, err)
	}
}

func TestSlotWithoutStockIsUnknown(t *testing.T) {
	slots, err := ParseTeamLabSlots([]byte(`{"time_spans":[{"start_time":"2026-11-24T09:00:00+09:00","start_str":"09:00","end_str":"09:30"}]}`), "2026-11-24", "Admission")
	if err != nil || slots[0].Status != SlotUnknown {
		t.Errorf("slot = %+v, %v; want unknown", slots, err)
	}
	slots, _ = ParseDMMSlots([]byte(`{"ticket_types":{"1":{"ticket_type_id":1,"entry_times":[{"start_time":"08:00","end_time":"08:30"}]}}}`), Date{2026, 11, 24})
	if slots[0].Status != SlotUnknown {
		t.Errorf("DMM slot without instock = %s", slots[0].Status)
	}
}

func TestPassedEstimatedWindowIsUnknown(t *testing.T) {
	planets, _ := Lookup("teamlab-planets")
	sd := newSightData(planets)
	until := Date{2026, 12, 31}
	sd.CalendarUntil = &until
	sd.Release = &Release{Covers: "2027-01", EarliestDay: "2026-10-21", LatestDay: "2026-10-31", Confidence: ConfEstimated}
	late := time.Date(2026, 11, 3, 9, 0, 0, 0, JST)
	rows := BuildOnsale(map[string]*SightData{planets.ID: sd}, []Sight{planets}, Date{2027, 1, 5}, Date{2027, 1, 5}, late, nil)
	if rows[0].State != StateUnknown {
		t.Errorf("state = %s, want unknown after the estimated window", rows[0].State)
	}
	if _, n := BuildICS(rows, late); n != 0 {
		t.Error("a passed window must not become a calendar event")
	}
}

func TestTeamLabStatusDayUsesJST(t *testing.T) {
	st, err := ParseTeamLabStatuses([]byte(`{"records":[{"date":"2026-11-02T15:00:00Z","stock_status":"few"},{"date":"2026-11-05T00:00:00+09:00","stock_status":""}]}`))
	if err != nil || st["2026-11-03"] != DayFew || st["2026-11-05"] != DayAvailable {
		t.Errorf("statuses = %v, %v; want the UTC record on 2026-11-03 JST", st, err)
	}
}

func TestRemoteTextIsCleanedAtParse(t *testing.T) {
	lines := htmlText("<p>a&#27;[2Jb</p>")
	if len(lines) != 1 || lines[0] != "a[2Jb" {
		t.Errorf("htmlText = %q", lines)
	}
}
