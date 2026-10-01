package hiking

import (
	"encoding/json"
	"testing"
	"time"
)

func TestSourceIDs(t *testing.T) {
	for _, tc := range []struct {
		input, resource, want string
		valid                 bool
	}{{"108", "mountains", "108", true}, {"https://yamap.com/activities/51497803", "activities", "51497803", true}, {"https://evil.example/activities/1", "activities", "", false}, {"https://yamap.com/model-courses/1", "activities", "", false}, {"0", "maps", "", false}, {"-1", "maps", "", false}, {"1e4", "maps", "", false}, {"https://yamap.com/maps/1?foo=2", "maps", "", false}} {
		got, e := ParseID(tc.input, tc.resource)
		if (e == nil) != tc.valid || got != tc.want {
			t.Errorf("%s: got %s, %v", tc.input, got, e)
		}
	}
}
func TestRegularizedMetricsAndNull(t *testing.T) {
	m := Row{"id": json.Number("123"), "distance": json.Number("5000"), "duration": json.Number("3600"), "cumulative_up": json.Number("0"), "activity_whole_section": Row{"distance": json.Number("4900"), "total_time": json.Number("3500"), "active_time": json.Number("3100"), "rest_time": json.Number("400")}}
	for _, tc := range []struct {
		detail   bool
		distance float64
		elapsed  float64
		source   string
	}{{false, 5000, 3600, "legacy_activity_summary"}, {true, 4900, 3500, "activity_whole_section"}} {
		r := Summary("report", m, tc.detail)
		metrics := object(r["metrics"])
		if metrics["distance_m"] != tc.distance || metrics["elapsed_seconds"] != tc.elapsed || metrics["source"] != tc.source || metrics["moving_seconds"] != nil {
			t.Fatalf("metrics %#v", metrics)
		}
	}
	r := Summary("route", Row{"id": json.Number("1"), "cumulative_up": json.Number("0")}, true)
	metrics := object(r["metrics"])
	if metrics["ascent_m"] != float64(0) || metrics["distance_m"] != nil || r["trail_open"] != nil || r["official_closure_status"] != nil {
		t.Fatal(r)
	}
}
func TestRecentTripDate(t *testing.T) {
	now := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	since := now.AddDate(0, 0, -30)
	for _, tc := range []struct {
		at      time.Time
		planned bool
		want    bool
	}{{now.AddDate(0, 0, -1), false, true}, {now.AddDate(-1, 0, 0), false, false}, {now.Add(time.Hour), false, false}, {now.AddDate(0, 0, -1), true, false}} {
		m := Row{"start_at": float64(tc.at.Unix()), "public_at": float64(now.Unix()), "is_planned": tc.planned}
		if Recent(m, since, now) != tc.want {
			t.Fatal(tc)
		}
	}
	recent := now.AddDate(0, 0, -1)
	for _, unknown := range []Row{
		{"start_at": float64(recent.Unix())},
		{"start_at": float64(recent.Unix()), "is_planned": nil},
		{"start_at": float64(recent.Unix()), "is_planned": "false"},
		{"start_at": float64(recent.Unix()), "is_planned": float64(0)},
	} {
		if Recent(unknown, since, now) {
			t.Fatalf("unknown is_planned included: %#v", unknown)
		}
	}
	if _, e := Since("2026-99-10", 30, now); e == nil {
		t.Fatal("invalid date accepted")
	}
	if JapanDate(float64(time.Date(2026, 9, 30, 16, 0, 0, 0, time.UTC).Unix())) != "2026-10-01" {
		t.Fatal("Japan boundary")
	}
}
func TestProjectionAndUTF8(t *testing.T) {
	m := Row{"id": "1", "metrics": Row{"distance_m": nil, "ascent_m": float64(0)}}
	p, e := Project(m, "id,metrics.distance_m")
	if e != nil || len(p) != 2 || object(p["metrics"])["distance_m"] != nil {
		t.Fatalf("%#v %v", p, e)
	}
	if _, e := Project(m, "bogus"); e == nil {
		t.Fatal("unknown field")
	}
	s, cut := Excerpt("高尾山の記録", 3)
	if s != "高尾山" || !cut {
		t.Fatal(s, cut)
	}
}
func TestSchemaAndCompare(t *testing.T) {
	if _, e := Extract(Row{}, "activities"); e == nil {
		t.Fatal("missing array")
	}
	if _, e := Detail(Row{"activity": Row{"id": json.Number("2")}}, "activity", "1"); e == nil {
		t.Fatal("ID mismatch")
	}
	r, err := Compare(Row{"id": json.Number("1"), "distance": float64(10)}, Row{"id": json.Number("2"), "distance": float64(15), "is_planned": false})
	if err != nil {
		t.Fatal(err)
	}
	if object(r["recorded_minus_planned"])["distance_m"] != float64(5) || r["route_equivalence_verified"] != false {
		t.Fatal(r)
	}
}

func TestReviewPlannedAndNullEvidence(t *testing.T) {
	for _, value := range []any{nil, true} {
		if r, e := Compare(Row{"id": "1"}, Row{"id": "2", "is_planned": value}); e == nil || r != nil {
			t.Fatal("non-recorded comparison accepted", r, e)
		}
	}
	for _, tc := range []struct {
		value     any
		want      any
		truncated any
	}{{nil, nil, nil}, {"", "", false}, {"高尾山", "高尾山", false}} {
		r := Summary("report", Row{"id": "2", "description": tc.value}, true)
		if r["observation_text"] != tc.want || r["observation_truncated"] != tc.truncated {
			t.Fatal(r)
		}
	}
}
