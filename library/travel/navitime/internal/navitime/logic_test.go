package navitime

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestValidateQueryCases(t *testing.T) {
	for _, tc := range []struct {
		name  string
		q     Query
		valid bool
	}{{"valid", testQuery(), true}, {"raw leading zero", Query{From: "00006668", To: "00001756", FirstOn: "2026-09-28"}, true}, {"spot", Query{From: "spot:02301-1300539n", To: "station:00004254", LastOn: "2026-09-28"}, true}, {"name", Query{From: "Tokyo", To: "00001756", DepartAt: "2026-09-28T09:00"}, false}, {"short ID", Query{From: "6668", To: "00001756", DepartAt: "2026-09-28T09:00"}, false}, {"missing time", Query{From: "00006668", To: "00001756"}, false}, {"two modes", Query{From: "00006668", To: "00001756", DepartAt: "2026-09-28T09:00", FirstOn: "2026-09-28"}, false}, {"invalid date", Query{From: "00006668", To: "00001756", FirstOn: "2026-02-30"}, false}, {"two passes", Query{From: "00006668", To: "00001756", DepartAt: "2026-09-28T09:00", Pass: "japan_rail_pass.hakone"}, false}, {"seconds", Query{From: "00006668", To: "00001756", DepartAt: "2026-09-28T09:00:01+09:00"}, false}} {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateQuery(tc.q); (err == nil) != tc.valid {
				t.Fatalf("valid=%v err=%v", tc.valid, err)
			}
		})
	}
	q := testQuery()
	q.DepartAt = "2026-09-28T00:00:00Z"
	p, _, e := queryParams(q)
	if e != nil || p.Get("date_time") != "2026-09-28T09:00" {
		t.Fatal("offset normalization failed")
	}
}
func TestSummariesLimitsAndNulls(t *testing.T) {
	r := RouteResult{Routes: []Route{{ID: "a"}, {ID: "b"}, {ID: "c"}, {ID: "d"}}}
	for _, tc := range []struct{ limit, want int }{{0, 3}, {1, 1}, {3, 3}, {99, 4}} {
		out := Summaries(r, tc.limit).(SummaryResult)
		if len(out.Routes) != tc.want || out.ReturnedAlternatives != 4 {
			t.Fatalf("limit %d %+v", tc.limit, out)
		}
		b, e := json.Marshal(out)
		if e != nil || strings.Contains(string(b), `"legs"`) || !strings.Contains(string(b), `"duration_minutes":null`) {
			t.Fatalf("summary nulls/detail bound: %s", b)
		}
	}
}
func TestCompareRoutesUnknownsAndCaps(t *testing.T) {
	input := RouteResult{Routes: []Route{{ID: "unknown", SourceIndex: 1}, {ID: "slow", SourceIndex: 2, DurationMinutes: ptr(20), WalkingMeters: ptr(10), Transfers: ptr(1), Fare: Fare{TotalJPY: ptr(500)}}, {ID: "fast", SourceIndex: 3, DurationMinutes: ptr(10), WalkingMeters: ptr(20), Transfers: ptr(0), Fare: Fare{TotalJPY: ptr(1000)}}}}
	for _, tc := range []struct {
		sort  string
		first string
	}{{"duration", "fast"}, {"fare", "slow"}, {"walk", "slow"}, {"transfers", "fast"}, {"source", "unknown"}, {"", "fast"}} {
		o, e := CompareRoutes(input, CompareOptions{tc.sort, -1, -1, -1, -1})
		if e != nil {
			t.Fatal(e)
		}
		r := o.(CompareResult)
		if r.Routes[0].ID != tc.first {
			t.Fatalf("sort %s: %+v", tc.sort, r.Routes)
		}
		if tc.sort != "source" && r.Routes[2].ID != "unknown" {
			t.Fatal("unknown metric ranked as best")
		}
	}
	for _, tc := range []struct {
		opts  CompareOptions
		count int
	}{{CompareOptions{"duration", 15, -1, -1, -1}, 1}, {CompareOptions{"fare", -1, 600, -1, -1}, 1}, {CompareOptions{"walk", -1, -1, 15, -1}, 1}, {CompareOptions{"transfers", -1, -1, -1, 0}, 1}, {CompareOptions{"duration", 0, 0, 0, 0}, 0}} {
		o, e := CompareRoutes(input, tc.opts)
		if e != nil {
			t.Fatal(e)
		}
		r := o.(CompareResult)
		if len(r.Routes) != tc.count || len(r.Notes) == 0 {
			t.Fatalf("caps: %+v", r)
		}
		assertJSON(t, "compare_caps", r)
	}
	for _, opts := range []CompareOptions{{"bogus", -1, -1, -1, -1}, {"duration", -2, -1, -1, -1}} {
		if _, e := CompareRoutes(input, opts); e == nil {
			t.Fatal("invalid compare options accepted")
		}
	}
	precise := RouteResult{Routes: []Route{{ID: "over_one_minute", DurationMinutes: ptr(1), DurationSeconds: ptr(61)}}}
	out, e := CompareRoutes(precise, CompareOptions{"duration", 1, -1, -1, -1})
	if e != nil || len(out.(CompareResult).Routes) != 0 {
		t.Fatal("whole-minute rounding hid a strict duration cap violation")
	}
}
