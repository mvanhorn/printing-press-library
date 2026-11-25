// Copyright 2026 laci141 and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// scheduleNow is the fixed clock every scheduleFactor case is measured against,
// so no case depends on the day the suite runs.
var scheduleNow = time.Date(2026, time.November, 25, 12, 0, 0, 0, time.UTC)

// TestScheduleFactorBranches pins every branch of the schedule factor by points
// and by a fragment of its detail. Only an ongoing trial whose ESTIMATED primary
// completion period has fully ended scores; terminal, suspended and UNKNOWN
// trials are left to the status factor so the same staleness is not counted
// twice, and an ACTUAL or untyped date never scores.
func TestScheduleFactorBranches(t *testing.T) {
	cases := []struct {
		name       string
		status     string
		date       string
		dateType   string
		wantPoints int
		wantDetail string
	}{
		{name: "recruiting, estimated day passed a year ago", status: "RECRUITING", date: "2025-11-13", dateType: "ESTIMATED", wantPoints: 5, wantDetail: "passed 12 month(s) ago"},
		{name: "active not recruiting, estimated month passed", status: "ACTIVE_NOT_RECRUITING", date: "2025-11", dateType: "ESTIMATED", wantPoints: 5, wantDetail: "passed 11 month(s) ago"},
		{name: "estimated year passed", status: "NOT_YET_RECRUITING", date: "2025", dateType: "ESTIMATED", wantPoints: 5, wantDetail: "passed 10 month(s) ago"},
		{name: "estimated day ended yesterday", status: "ENROLLING_BY_INVITATION", date: "2026-11-24", dateType: "ESTIMATED", wantPoints: 5, wantDetail: "passed 0 month(s) ago"},
		{name: "lowercase status and type still match", status: "recruiting", date: "2025-01-15", dateType: "estimated", wantPoints: 5, wantDetail: "still listed as recruiting"},
		{name: "estimated day is today", status: "RECRUITING", date: "2026-11-25", dateType: "ESTIMATED", wantPoints: 0, wantDetail: "expected 2026-11-25"},
		{name: "estimated month is the current month", status: "RECRUITING", date: "2026-11", dateType: "ESTIMATED", wantPoints: 0, wantDetail: "expected 2026-11"},
		{name: "estimated date in the future", status: "RECRUITING", date: "2028-06", dateType: "ESTIMATED", wantPoints: 0, wantDetail: "expected 2028-06"},
		{name: "actual date passed while follow-up continues", status: "ACTIVE_NOT_RECRUITING", date: "2025-11-13", dateType: "ACTUAL", wantPoints: 0, wantDetail: "reached on 2025-11-13"},
		{name: "passed date with no type posted", status: "RECRUITING", date: "2024-03", dateType: "", wantPoints: 0, wantDetail: "date type not posted"},
		{name: "no date posted", status: "RECRUITING", date: "", dateType: "", wantPoints: 0, wantDetail: "not posted"},
		{name: "unparseable date", status: "RECRUITING", date: "soon", dateType: "ESTIMATED", wantPoints: 0, wantDetail: "not parseable"},
		{name: "completed trial is not checked", status: "COMPLETED", date: "2020-01", dateType: "ESTIMATED", wantPoints: 0, wantDetail: "only to ongoing trials"},
		{name: "unknown status is left to the status factor", status: "UNKNOWN", date: "2020-01", dateType: "ESTIMATED", wantPoints: 0, wantDetail: "only to ongoing trials"},
		{name: "suspended trial is left to the status factor", status: "SUSPENDED", date: "2020-01", dateType: "ESTIMATED", wantPoints: 0, wantDetail: "only to ongoing trials"},
		{name: "status not posted", status: "", date: "2020-01", dateType: "ESTIMATED", wantPoints: 0, wantDetail: "only to ongoing trials"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			trial := Trial{NCTID: "NCT0", Status: tc.status, PrimaryCompletionDate: tc.date, PrimaryCompletionType: tc.dateType}
			f := scheduleFactor(trial, scheduleNow)
			if f.Name != "schedule" {
				t.Errorf("factor name = %q, want %q", f.Name, "schedule")
			}
			if f.Points != tc.wantPoints {
				t.Errorf("points = %d, want %d (detail %q)", f.Points, tc.wantPoints, f.Detail)
			}
			if !strings.Contains(f.Detail, tc.wantDetail) {
				t.Errorf("detail = %q, want it to contain %q", f.Detail, tc.wantDetail)
			}
		})
	}
}

// TestScheduleOverduePointsStayBelowUnknown guards the ordering the point value
// rests on: the registry marks a trial UNKNOWN only after it is more than two
// years past its completion date, so a trial still listed as ongoing past its
// estimate must never outscore an UNKNOWN one.
func TestScheduleOverduePointsStayBelowUnknown(t *testing.T) {
	unknown := scoreRisk(Trial{NCTID: "NCT0", Status: "UNKNOWN", Enrollment: 500,
		Countries: []string{"United States", "Germany"}, Phases: []string{"PHASE3"}, Sponsor: "Acme"}, 100)
	if scheduleOverduePoints <= 0 || scheduleOverduePoints >= unknown.Score {
		t.Errorf("scheduleOverduePoints = %d, want 0 < points < %d (UNKNOWN status score)", scheduleOverduePoints, unknown.Score)
	}
}

// TestScoreRiskCarriesScheduleFactor checks the call site, which the branch
// test above cannot reach: scoreRisk must always list a schedule factor and add
// its points to the total. The dates sit far from today so the real clock
// scoreRisk uses cannot move them across the line.
func TestScoreRiskCarriesScheduleFactor(t *testing.T) {
	cases := []struct {
		name      string
		date      string
		dateType  string
		wantScore int
	}{
		{name: "long overdue estimate", date: "2000-01", dateType: "ESTIMATED", wantScore: scheduleOverduePoints},
		{name: "estimate far in the future", date: "2999-12", dateType: "ESTIMATED", wantScore: 0},
		{name: "no date posted", date: "", dateType: "", wantScore: 0},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// Every other factor scores 0, so the total is the schedule factor.
			trial := Trial{
				NCTID: "NCT0", Status: "RECRUITING", Enrollment: 500,
				Countries: []string{"United States", "Germany"}, Phases: []string{"PHASE3"},
				Sponsor: "Acme", PrimaryCompletionDate: tc.date, PrimaryCompletionType: tc.dateType,
			}
			v := scoreRisk(trial, 100)
			found := 0
			for _, f := range v.Factors {
				if f.Name == "schedule" {
					found++
				}
			}
			if found != 1 {
				t.Errorf("schedule factor entries = %d, want exactly 1 in %+v", found, v.Factors)
			}
			if v.Score != tc.wantScore {
				t.Errorf("score = %d, want %d (factors %+v)", v.Score, tc.wantScore, v.Factors)
			}
		})
	}
}

// TestNormalizedFieldsRequestsPrimaryCompletionType guards the live defect: CT.gov
// returns primaryCompletionDateStruct.type only when PrimaryCompletionDateType is
// named in the fields list, so without it every overdue estimate reads as "date
// type not posted" and never scores. A fixture-based test cannot see this, because
// a fixture carries the type whether or not it was requested.
func TestNormalizedFieldsRequestsPrimaryCompletionType(t *testing.T) {
	if !strings.Contains(","+normalizedFields+",", ",PrimaryCompletionDateType,") {
		t.Errorf("normalizedFields must request PrimaryCompletionDateType, got %q", normalizedFields)
	}
}

// TestNormalizeStudyReadsPrimaryCompletionType checks that the ACTUAL /
// ESTIMATED marker reaches the Trial — without it every overdue estimate would
// fall into the "date type not posted" branch and never score — and that the
// marker stays out of the JSON shape all commands share.
func TestNormalizeStudyReadsPrimaryCompletionType(t *testing.T) {
	raw, _ := json.Marshal(map[string]any{
		"protocolSection": map[string]any{
			"identificationModule": map[string]any{"nctId": "NCT05711940", "briefTitle": "An ongoing trial"},
			"statusModule": map[string]any{
				"overallStatus":               "ACTIVE_NOT_RECRUITING",
				"primaryCompletionDateStruct": map[string]any{"date": "2025-11-13", "type": "ESTIMATED"},
			},
		},
	})
	tr, ok := normalizeStudy(raw)
	if !ok {
		t.Fatal("normalizeStudy rejected a study with an NCT id")
	}
	if tr.PrimaryCompletionDate != "2025-11-13" || tr.PrimaryCompletionType != "ESTIMATED" {
		t.Errorf("primary completion = %q / %q, want %q / %q", tr.PrimaryCompletionDate, tr.PrimaryCompletionType, "2025-11-13", "ESTIMATED")
	}
	out, err := json.Marshal(tr)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "ESTIMATED") {
		t.Errorf("date type leaked into the shared Trial JSON: %s", out)
	}
}

// TestPartialDateEnd pins the period ends, including the year and month
// rollovers a naive day count would get wrong.
func TestPartialDateEnd(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{in: "2025-11-13", want: "2025-11-14"},
		{in: "2025-12-31", want: "2026-01-01"},
		{in: "2025-02", want: "2025-03-01"},
		{in: "2024-12", want: "2025-01-01"},
		{in: "2025", want: "2026-01-01"},
	}
	for _, tc := range cases {
		got, ok := partialDateEnd(tc.in)
		if !ok {
			t.Errorf("partialDateEnd(%q) not parsed", tc.in)
			continue
		}
		if got.Format("2006-01-02") != tc.want {
			t.Errorf("partialDateEnd(%q) = %s, want %s", tc.in, got.Format("2006-01-02"), tc.want)
		}
	}
	if _, ok := partialDateEnd("November 2025"); ok {
		t.Error("partialDateEnd accepted a free-text date")
	}
}

// TestMonthsBetweenIgnoresMachineZone pins the displayed delay to UTC: the
// deadline is parsed as UTC while scoreRisk passes the local clock, so the same
// instant must give the same month count in every zone. 2025-12-14 00:30 UTC is
// still 2025-12-13 in a UTC-5 zone, where a local calendar read would borrow a
// month and report 0.
func TestMonthsBetweenIgnoresMachineZone(t *testing.T) {
	start := time.Date(2025, time.November, 14, 0, 0, 0, 0, time.UTC)
	instant := time.Date(2025, time.December, 14, 0, 30, 0, 0, time.UTC)
	for _, zone := range []*time.Location{time.UTC, time.FixedZone("UTC-5", -5*3600), time.FixedZone("UTC+9", 9*3600)} {
		if got := monthsBetween(start, instant.In(zone)); got != 1 {
			t.Errorf("monthsBetween in %s = %d, want 1", zone, got)
		}
	}
}

// TestMonthsBetween covers the day-of-month borrow and the clamp at zero.
func TestMonthsBetween(t *testing.T) {
	d := func(s string) time.Time { v, _ := time.Parse("2006-01-02", s); return v }
	cases := []struct {
		start, end string
		want       int
	}{
		{start: "2025-01-31", end: "2025-02-28", want: 0},
		{start: "2025-01-15", end: "2025-02-15", want: 1},
		{start: "2025-11-14", end: "2026-11-25", want: 12},
		{start: "2026-12-01", end: "2026-11-25", want: 0},
	}
	for _, tc := range cases {
		if got := monthsBetween(d(tc.start), d(tc.end)); got != tc.want {
			t.Errorf("monthsBetween(%s, %s) = %d, want %d", tc.start, tc.end, got, tc.want)
		}
	}
}
