// Copyright 2026 laci141 and contributors. Licensed under Apache-2.0. See LICENSE.

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
)

// velocityNow is a fixed clock: 25 Nov 2026, so the last COMPLETE year is 2025.
var velocityNow = time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)

func TestVelocityWindowsFor(t *testing.T) {
	cases := []struct {
		name                                     string
		now                                      time.Time
		years                                    int
		recentFrom, recentTo, priorFrom, priorTo int
	}{
		{"default 3 years", velocityNow, 3, 2023, 2025, 2020, 2022},
		{"one year", velocityNow, 1, 2025, 2025, 2024, 2024},
		{"five years", velocityNow, 5, 2021, 2025, 2016, 2020},
		{"1 Jan 00:00 UTC makes last year complete", time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC), 3, 2024, 2026, 2021, 2023},
		{"31 Dec 23:59 UTC is still the old year", time.Date(2026, 12, 31, 23, 59, 0, 0, time.UTC), 3, 2023, 2025, 2020, 2022},
		// 1 Jan 2027 03:00 in UTC+14 is 31 Dec 2026 13:00 UTC: the machine's zone must not matter.
		{"machine zone is ignored", time.Date(2027, 1, 1, 3, 0, 0, 0, time.FixedZone("ahead", 14*3600)), 3, 2023, 2025, 2020, 2022},
	}
	for _, tc := range cases {
		w := velocityWindowsFor(tc.now, tc.years)
		got := [4]int{w.RecentFrom, w.RecentTo, w.PriorFrom, w.PriorTo}
		want := [4]int{tc.recentFrom, tc.recentTo, tc.priorFrom, tc.priorTo}
		if got != want {
			t.Errorf("%s: windows = %v, want %v", tc.name, got, want)
		}
		if w.RecentTo-w.RecentFrom != tc.years-1 || w.PriorTo-w.PriorFrom != tc.years-1 {
			t.Errorf("%s: windows are not %d years long: %+v", tc.name, tc.years, w)
		}
		if w.PriorTo+1 != w.RecentFrom {
			t.Errorf("%s: windows are not adjacent: %+v", tc.name, w)
		}
	}
}

func TestYearSpan(t *testing.T) {
	if got := yearSpan(2023, 2025); got != "2023–2025" {
		t.Errorf("yearSpan(2023, 2025) = %q", got)
	}
	if got := yearSpan(2025, 2025); got != "2025" {
		t.Errorf("yearSpan(2025, 2025) = %q", got)
	}
}

func velocityTrials(start string, n, enroll int) []Trial {
	out := make([]Trial, n)
	for i := range out {
		out[i] = Trial{NCTID: fmt.Sprintf("NCT%s%05d", strings.ReplaceAll(start, "-", ""), i), StartDate: start, Enrollment: enroll}
	}
	return out
}

func TestTallyByWindow(t *testing.T) {
	w := velocityWindowsFor(velocityNow, 3) // recent 2023–2025, prior 2020–2022
	var trials []Trial
	trials = append(trials, velocityTrials("2019-06-01", 1, 10)...)  // older
	trials = append(trials, velocityTrials("2020", 2, 100)...)       // prior, year-only date
	trials = append(trials, velocityTrials("2022-12-31", 1, 50)...)  // prior, last day of the window
	trials = append(trials, velocityTrials("2023-01-01", 1, 200)...) // recent, first day of the window
	trials = append(trials, velocityTrials("2024-05", 1, 300)...)    // recent, month-only date
	trials = append(trials, velocityTrials("2025-12-31", 2, 400)...) // recent
	trials = append(trials, velocityTrials("2026-02-10", 4, 500)...) // the current year: later
	trials = append(trials, velocityTrials("2027-03", 1, 600)...)    // not yet started: later
	trials = append(trials, velocityTrials("", 3, 700)...)           // no start date: skipped
	trials = append(trials, velocityTrials("2024-07-01", 1, -5)...)  // negative enrollment counts as 0

	got := tallyByWindow(trials, w)
	if got.RecentTrials != 5 || got.PriorTrials != 3 || got.LaterTrials != 5 || got.OlderTrials != 1 {
		t.Errorf("counts recent/prior/later/older = %d/%d/%d/%d, want 5/3/5/1", got.RecentTrials, got.PriorTrials, got.LaterTrials, got.OlderTrials)
	}
	if got.RecentEnroll != 200+300+800+0 || got.PriorEnroll != 250 {
		t.Errorf("enrollment recent/prior = %d/%d, want 1300/250", got.RecentEnroll, got.PriorEnroll)
	}
	years := make([]int, len(got.ByYear))
	for i, b := range got.ByYear {
		years[i] = b.Year
	}
	if fmt.Sprint(years) != "[2019 2020 2022 2023 2024 2025 2026 2027]" {
		t.Errorf("by_year = %v: must be sorted, hold the current and future year, and skip undated trials", years)
	}
	for _, b := range got.ByYear {
		if b.Year == 2026 && (b.Trials != 4 || b.Enrollment != 2000) {
			t.Errorf("2026 bucket = %+v", b)
		}
		if b.Year == 2024 && (b.Trials != 2 || b.Enrollment != 300) {
			t.Errorf("2024 bucket = %+v: negative enrollment must count as 0", b)
		}
	}
}

// The Long COVID registry counts measured on 2026-11-25 (exact totalCount per
// start year). The old rule read this series as +13% and, from a 400-trial
// sample, as +24%; two equal complete windows say it is flat, because the field
// peaked in 2023 and halved in 2024.
func TestVelocityLongCovidRegistryShape(t *testing.T) {
	counts := map[string]int{"2019": 2, "2020": 50, "2021": 131, "2022": 162, "2023": 179, "2024": 85, "2025": 82, "2026": 49}
	var trials []Trial
	for y, n := range counts {
		trials = append(trials, velocityTrials(y+"-06-15", n, 0)...)
	}
	got := tallyByWindow(trials, velocityWindowsFor(velocityNow, 3))
	if got.RecentTrials != 346 || got.PriorTrials != 343 || got.LaterTrials != 49 || got.OlderTrials != 2 {
		t.Fatalf("recent/prior/later/older = %d/%d/%d/%d, want 346/343/49/2", got.RecentTrials, got.PriorTrials, got.LaterTrials, got.OlderTrials)
	}
	if g := growthPct(got.RecentTrials, got.PriorTrials); g != 1 {
		t.Errorf("growth = %+d%%, want +1%%", g)
	}
}

func TestVelocityNotes(t *testing.T) {
	w := velocityWindowsFor(velocityNow, 3)
	year := []yearBucket{{Year: 2024, Trials: 1}}
	full := windowTally{RecentTrials: 5, PriorTrials: 5, ByYear: year}
	cases := []struct {
		name           string
		sampled, total int
		tally          windowTally
		want           []string
		notWant        []string
	}{
		{"sample smaller than registry", 400, 745, full, []string{"sample covers 400 of 745 trials"}, nil},
		{"sample equals registry", 400, 400, full, nil, []string{"sample covers"}},
		{"registry total unknown", 400, 0, full, nil, []string{"sample covers"}},
		{"registry total below sample", 400, 399, full, nil, []string{"sample covers"}},
		{"current-year trials are explained", 400, 400, windowTally{RecentTrials: 5, PriorTrials: 5, LaterTrials: 49, ByYear: year},
			[]string{"49 trial(s) started in 2026 or later"}, nil},
		{"no later trials, no later note", 400, 400, full, nil, []string{"started in"}},
		{"empty prior window names its years", 400, 400, windowTally{RecentTrials: 5, ByYear: year},
			[]string{"no trials in the prior window (2020–2022)"}, nil},
		{"nothing sampled", 0, 745, windowTally{}, []string{"no trials matched"}, []string{"sample covers"}},
		{"no parseable dates", 10, 10, windowTally{}, []string{"no parseable start dates"}, nil},
	}
	for _, tc := range cases {
		got := velocityNotes(tc.sampled, tc.total, w, tc.tally)
		for _, s := range tc.want {
			if !strings.Contains(got, s) {
				t.Errorf("%s: note %q lacks %q", tc.name, got, s)
			}
		}
		for _, s := range tc.notWant {
			if strings.Contains(got, s) {
				t.Errorf("%s: note %q must not contain %q", tc.name, got, s)
			}
		}
	}
	both := velocityNotes(400, 745, w, windowTally{RecentTrials: 5, PriorTrials: 5, LaterTrials: 2, ByYear: year})
	if !strings.Contains(both, "closer match. 2 trial(s)") {
		t.Errorf("two notes must be joined by one space, got %q", both)
	}
}

// velocityFake answers the two kinds of /studies request the command makes: the
// paged fetch and the countTotal probe.
type velocityFake struct {
	studies  []string
	total    int
	countErr error
	fetchErr error
	calls    []map[string]string
}

func (f *velocityFake) Get(_ context.Context, _ string, params map[string]string) (json.RawMessage, error) {
	cp := map[string]string{}
	for k, v := range params {
		cp[k] = v
	}
	f.calls = append(f.calls, cp)
	if params["countTotal"] == "true" {
		if f.countErr != nil {
			return nil, f.countErr
		}
		return json.RawMessage(fmt.Sprintf(`{"totalCount":%d}`, f.total)), nil
	}
	if f.fetchErr != nil {
		return nil, f.fetchErr
	}
	return json.RawMessage(`{"studies":[` + strings.Join(f.studies, ",") + `]}`), nil
}

func velocityStudy(i int, start string, enroll int) string {
	return fmt.Sprintf(`{"protocolSection":{"identificationModule":{"nctId":"NCT%08d"},"statusModule":{"startDateStruct":{"date":%q}},"designModule":{"enrollmentInfo":{"count":%d}}}}`, i, start, enroll)
}

func velocityFakeWith(total int, years ...string) *velocityFake {
	f := &velocityFake{total: total}
	for i, y := range years {
		f.studies = append(f.studies, velocityStudy(i+1, y, 100))
	}
	return f
}

func TestBuildVelocityView(t *testing.T) {
	ctx := context.Background()
	f := velocityFakeWith(745, "2021-01", "2022-01", "2023-01", "2024-01", "2024-02", "2025-01", "2026-01")
	v, err := buildVelocityView(ctx, f, "long covid", 400, 3, 3, velocityNow)
	if err != nil {
		t.Fatal(err)
	}
	if v.SampleSize != 7 || v.RegistryTotal != 745 {
		t.Errorf("sample/registry = %d/%d, want 7/745", v.SampleSize, v.RegistryTotal)
	}
	if v.RecentFrom != 2023 || v.RecentTo != 2025 || v.PriorFrom != 2020 || v.PriorTo != 2022 {
		t.Errorf("windows = %d–%d / %d–%d", v.RecentFrom, v.RecentTo, v.PriorFrom, v.PriorTo)
	}
	if v.RecentTrials != 4 || v.PriorTrials != 2 || v.TrialGrowthPct != 100 || v.LaterTrials != 1 || v.OlderTrials != 0 {
		t.Errorf("view = %+v", v)
	}
	if v.RecentEnroll != 400 || v.PriorEnroll != 200 || v.EnrollGrowthPct != 100 {
		t.Errorf("enrollment = %d/%d (%+d%%)", v.RecentEnroll, v.PriorEnroll, v.EnrollGrowthPct)
	}
	if !strings.Contains(v.Note, "sample covers 7 of 745") || !strings.Contains(v.Note, "started in 2026 or later") {
		t.Errorf("note = %q", v.Note)
	}
	if len(f.calls) != 2 || f.calls[1]["countTotal"] != "true" || f.calls[1]["query.cond"] != "long covid" {
		t.Errorf("expected one fetch and one count probe for the same condition, got %v", f.calls)
	}

	one, err := buildVelocityView(ctx, velocityFakeWith(2, "2025-01", "2024-01"), "x", 400, 3, 1, velocityNow)
	if err != nil {
		t.Fatal(err)
	}
	if one.RecentFrom != 2025 || one.PriorTo != 2024 || one.RecentTrials != 1 || one.PriorTrials != 1 {
		t.Errorf("one-year windows = %+v", one)
	}
	if strings.Contains(one.Note, "sample covers") {
		t.Errorf("sample equals registry total, got note %q", one.Note)
	}
}

func TestBuildVelocityViewSurvivesFailedCount(t *testing.T) {
	f := velocityFakeWith(0, "2024-01", "2022-01")
	f.countErr = errors.New("rate limited")
	v, err := buildVelocityView(context.Background(), f, "x", 400, 3, 3, velocityNow)
	if err != nil {
		t.Fatalf("a failed total must not fail the command: %v", err)
	}
	if v.RegistryTotal != 0 || strings.Contains(v.Note, "sample covers") || v.SampleSize != 2 {
		t.Errorf("view = %+v", v)
	}
}

func TestBuildVelocityViewEmptyAndErrors(t *testing.T) {
	f := &velocityFake{total: 5}
	v, err := buildVelocityView(context.Background(), f, "nothing", 400, 3, 3, velocityNow)
	if err != nil {
		t.Fatal(err)
	}
	if v.SampleSize != 0 || len(f.calls) != 1 || !strings.Contains(v.Note, "no trials matched") {
		t.Errorf("empty result: view %+v, %d calls (the count probe is skipped when nothing matched)", v, len(f.calls))
	}
	bad := &velocityFake{fetchErr: errors.New("boom")}
	if _, err := buildVelocityView(context.Background(), bad, "x", 400, 3, 3, velocityNow); err == nil {
		t.Error("a failed fetch must be returned")
	}
}

func TestVelocityViewJSONShape(t *testing.T) {
	v, err := buildVelocityView(context.Background(), velocityFakeWith(0, "2024-01"), "x", 400, 3, 3, velocityNow)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(v)
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	for _, k := range []string{"recent_from", "recent_to", "prior_from", "prior_to", "later_trials", "older_trials", "window_years"} {
		if _, ok := m[k]; !ok {
			t.Errorf("JSON lacks %q: %s", k, raw)
		}
	}
	if _, ok := m["registry_total"]; ok {
		t.Errorf("registry_total must be omitted when unknown: %s", raw)
	}
}

func TestWriteVelocityHuman(t *testing.T) {
	v := velocityView{Query: "long covid", SampleSize: 400, RegistryTotal: 745, WindowYears: 3,
		RecentFrom: 2023, RecentTo: 2025, PriorFrom: 2020, PriorTo: 2022,
		RecentTrials: 346, PriorTrials: 343, TrialGrowthPct: 1, RecentEnroll: 10, PriorEnroll: 5, EnrollGrowthPct: 100,
		ByYear: []yearBucket{{Year: 2024, Trials: 85, Enrollment: 10}}, Note: "a note"}
	var buf bytes.Buffer
	if err := writeVelocityHuman(&buf, v); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		"(sample 400 of 745)",
		"Recent 2023–2025: 346 trials (+1%)",
		"vs prior 2020–2022: 343 trials",
		"  2024    85 trials",
		"note: a note",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	v.RegistryTotal = 400
	v.RecentFrom, v.PriorFrom, v.PriorTo = 2025, 2024, 2024
	buf.Reset()
	_ = writeVelocityHuman(&buf, v)
	out = buf.String()
	if !strings.Contains(out, "(sample 400)") || strings.Contains(out, "of 400") {
		t.Errorf("complete sample must print plain count:\n%s", out)
	}
	if !strings.Contains(out, "Recent 2025:") || !strings.Contains(out, "vs prior 2024:") {
		t.Errorf("one-year windows print a single year:\n%s", out)
	}
}
