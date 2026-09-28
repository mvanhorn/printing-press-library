package navitime

import (
	"context"
	"encoding/json"
	"errors"
	"golang.org/x/net/html"
	"os"
	"strings"
	"testing"
	"time"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, e := os.ReadFile("testdata/" + name)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func testQuery() Query {
	return Query{From: "station:00006668", To: "station:00001756", DepartAt: "2026-09-28T09:00"}
}
func assertJSON(t *testing.T, label string, v any) {
	t.Helper()
	b, e := json.Marshal(v)
	if e != nil {
		t.Fatal(e)
	}
	t.Logf("ASSERT %s %s", label, b)
}
func TestCapturedRoutesContent(t *testing.T) {
	for _, tc := range []struct {
		name, file string
		q          Query
		count      int
		dep, arr   string
	}{
		{"depart", "depart.html", testQuery(), 5, "2026-09-28T09:00:00+09:00", "2026-09-28T11:15:00+09:00"},
		{"arrive", "arrive.html", Query{From: "00006668", To: "00001756", ArriveBy: "2026-09-28T12:00"}, 5, "2026-09-28T09:48:00+09:00", "2026-09-28T12:00:00+09:00"},
		{"first", "first.html", Query{From: "00006668", To: "00001756", FirstOn: "2026-09-28"}, 4, "2026-09-28T05:45:00+09:00", "2026-09-28T08:02:00+09:00"},
		{"last", "last.html", Query{From: "00006668", To: "00001756", LastOn: "2026-09-28"}, 5, "2026-09-28T21:24:00+09:00", "2026-09-28T23:31:00+09:00"},
		{"pass", "pass.html", Query{From: "00006668", To: "00001756", DepartAt: "2026-09-28T09:00", Pass: "japan_rail_pass"}, 5, "2026-09-28T09:03:00+09:00", "2026-09-28T11:37:00+09:00"},
		{"overnight", "overnight.html", Query{From: "00006668", To: "00004254", DepartAt: "2026-09-28T23:58"}, 5, "2026-09-29T00:06:00+09:00", "2026-09-29T00:20:00+09:00"},
		{"poi", "poi.html", Query{From: "00006668", To: "spot:02301-1300539n", DepartAt: "2026-09-28T09:00"}, 5, "2026-09-28T09:02:00+09:00", "2026-09-28T09:27:00+09:00"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			routes, passes, e := parseRoutes(fixture(t, tc.file), tc.q, "https://japantravel.navitime.com/test")
			if e != nil {
				t.Fatal(e)
			}
			if len(routes) != tc.count || len(passes) != 53 {
				t.Fatalf("routes/passes: %d/%d", len(routes), len(passes))
			}
			r := routes[0]
			if r.DepartureAt == nil || *r.DepartureAt != tc.dep || r.ArrivalAt == nil || *r.ArrivalAt != tc.arr {
				t.Fatalf("calendar dates: %v %v", r.DepartureAt, r.ArrivalAt)
			}
			if !regexpLocalID.MatchString(r.ID) || r.SourceID != nil || len(r.Legs) == 0 || r.From.SourceID == nil || *r.From.SourceID != "00006668" {
				t.Fatal("identifier/detail provenance missing")
			}
			for _, l := range r.Legs {
				if l.DepartureAt != nil && l.ArrivalAt != nil {
					d, _ := time.Parse(time.RFC3339, *l.DepartureAt)
					a, _ := time.Parse(time.RFC3339, *l.ArrivalAt)
					if a.Before(d) {
						t.Fatal("negative leg duration")
					}
				}
			}
			if tc.name == "depart" || tc.name == "arrive" {
				if r.To.Kind != nil && *r.To.Kind != "station" {
					t.Fatal("a tourism link must not overwrite a station point type")
				}
				if r.Fare.TotalJPY == nil || *r.Fare.TotalJPY != 13320 || len(r.FareGroups) != 1 {
					t.Fatal("Nozomi total/through-fare grouping")
				}
				g := r.FareGroups[0]
				if g.BaseFareJPY == nil || *g.BaseFareJPY != 8360 || g.DefaultSeatFareJPY == nil || *g.DefaultSeatFareJPY != 4960 {
					t.Fatalf("base/seat: %+v", g)
				}
				if len(r.Legs[0].SeatOptions) != 3 || !r.Legs[0].SeatOptions[0].Selected || *r.Legs[0].SeatOptions[0].SupplementJPY != 4960 || *r.Legs[0].SeatOptions[1].SupplementJPY != 5610 || *r.Legs[0].SeatOptions[2].SupplementJPY != 10480 {
					t.Fatalf("seat choices: %+v", r.Legs[0].SeatOptions)
				}
			}
			if tc.name == "first" {
				if len(r.Legs) != 2 || len(r.FareGroups) != 1 || *r.FareGroups[0].BaseFareJPY != 8360 {
					t.Fatal("through fare duplicated across two connecting legs")
				}
			}
			if tc.name == "pass" {
				if *r.Fare.TotalJPY != 13320 || r.Fare.PassHolderCostJPY != nil || r.Pass.PassHolderCostJPY != nil || r.Pass.Coverage != "source_coverage_labels_present" || len(r.Pass.SourceTexts) == 0 {
					t.Fatal("pass cost or source coverage semantics corrupted")
				}
				if r.Legs[0].LineName == nil || *r.Legs[0].LineName != "Hikari" {
					t.Fatal("pass filter service not reflected")
				}
			}
			if tc.name == "poi" {
				if r.To.Kind == nil || *r.To.Kind != "spot" || r.To.SourceID == nil || *r.To.SourceID != "02301-1300539n" || r.Legs[len(r.Legs)-1].Kind != "walking" || r.WalkingMeters == nil || *r.WalkingMeters <= 0 || r.Fare.ICJPY == nil || *r.Fare.ICJPY != 209 {
					t.Fatalf("POI/walking/IC normalization: %+v", r)
				}
			}
			assertJSON(t, "captured_"+tc.name, map[string]any{"routes": len(routes), "passes": len(passes), "departure": r.DepartureAt, "arrival": r.ArrivalAt, "fare": r.Fare, "fare_group_count": len(r.FareGroups), "leg_count": len(r.Legs), "walking_meters": r.WalkingMeters})
		})
	}
}
func TestCalendarSecondsMinuteLegPrecisionAndRoadEstimates(t *testing.T) {
	q := Query{From: "00006668", To: "00004254", DepartAt: "2026-09-28T23:58"}
	routes, _, e := parseRoutes(fixture(t, "overnight.html"), q, "source")
	if e != nil {
		t.Fatal(e)
	}
	taxi := routes[4]
	if taxi.TimingBasis != "estimated_road_travel" || len(taxi.TransportKinds) != 1 || taxi.TransportKinds[0] != "car_taxi" || taxi.Fare.TotalJPY == nil || *taxi.Fare.TotalJPY != 4200 || taxi.Fare.Basis != "source_estimated_taxi_fare" || taxi.Fare.SourceCaveat == nil || *taxi.ArrivalAt != "2026-09-29T00:19:45+09:00" || *taxi.Legs[0].ArrivalAt != "2026-09-29T00:19:00+09:00" || *taxi.DurationSeconds != 1305 || taxi.DurationMinutesBasis != "whole_minutes_floor_from_calendar_seconds" {
		t.Fatalf("road timing precision: %+v", summary(taxi))
	}
	last := time.Date(2026, 9, 28, 10, 48, 49, 0, japan)
	end := last.Add(time.Hour)
	stamp, _, e := nextClock("10:48", last, end)
	if e != nil || *stamp != "2026-09-28T10:48:00+09:00" {
		t.Fatal("calendar seconds caused false midnight rollover")
	}
	if _, _, e = nextClock("25:99", last, end); e == nil {
		t.Fatal("invalid source minute accepted")
	}
	assertJSON(t, "calendar_road_precision", summary(taxi))
}
func TestPassWarningsExcludeAncestorPromotion(t *testing.T) {
	routes, _, e := parseRoutes(fixture(t, "depart.html"), testQuery(), "source")
	if e != nil {
		t.Fatal(e)
	}
	warning := false
	for _, r := range routes {
		for _, s := range r.Pass.SourceTexts {
			if strings.Contains(s, "Order Ticket Now") || strings.Contains(s, "Timetable") || strings.Contains(s, "Platform") {
				t.Fatalf("pass warning contains unrelated ancestor content: %q", s)
			}
			if s == "Ride Nozomi/Mizuho with an extra fare" {
				warning = true
			}
		}
	}
	if !warning {
		t.Fatal("specific Nozomi/Mizuho supplement warning lost")
	}
}
func TestNamedPassCoverageLabelsScopeImageAlternatives(t *testing.T) {
	q := testQuery()
	q.Pass = "japan_rail_pass"
	source := strings.ReplaceAll(string(fixture(t, "pass.html")), `<div class="special-pass-button covered">`, `<div class="special-pass-button covered"><img alt="BOOKING PROMOTION"><img alt="KLOOK">`)
	routes, _, err := parseRoutes([]byte(source), q, "source")
	if err != nil {
		t.Fatal(err)
	}
	named := false
	for _, label := range routes[0].Pass.SourceTexts {
		if label == "Covered by JAPAN RAIL PASS" {
			named = true
		}
		if strings.Contains(label, "KLOOK") || strings.Contains(label, "BOOKING PROMOTION") || strings.Contains(label, "Order Ticket Now") {
			t.Fatalf("unrelated promotional image label captured: %q", label)
		}
	}
	if !named {
		t.Fatalf("named source pass coverage label missing: %v", routes[0].Pass.SourceTexts)
	}
	assertJSON(t, "named_pass_label", routes[0].Pass)
}
func TestLookupContent(t *testing.T) {
	for _, tc := range []struct {
		file     string
		count    int
		id, name string
	}{{"places-ja.json", 7, "00004302", "Shin-Okubo"}, {"places-en.json", 10, "00006668", "Tokyo"}} {
		t.Run(tc.file, func(t *testing.T) {
			p, e := parsePlaces(fixture(t, tc.file), "https://japantravel.navitime.com/lookup")
			if e != nil {
				t.Fatal(e)
			}
			if len(p) != tc.count || p[0].ID != tc.id || p[0].Name["en"] != tc.name || p[0].Name["ja"] == "" || p[0].Coordinates == nil {
				t.Fatalf("candidate content: %+v", p)
			}
			if tc.file == "places-ja.json" {
				seen := map[string]bool{}
				for _, v := range p {
					seen[v.ID] = true
				}
				for _, id := range []string{"00005561", "00005559", "00005562", "00005560"} {
					if !seen[id] {
						t.Fatalf("missing region-distinct Okubo %s", id)
					}
				}
			}
			assertJSON(t, "lookup", map[string]any{"count": len(p), "leading_zero_id": p[0].ID, "bilingual": p[0].Name})
		})
	}
	for _, body := range []string{"<html>login</html>", "null", `[{"code":"bad","mainType":"station"}]`} {
		if _, e := parsePlaces([]byte(body), "source"); e == nil {
			t.Fatal("invalid lookup accepted")
		}
	}
	p, e := parsePlaces([]byte(`[{"code":"00006668","mainType":"station","name":{"en":"Tokyo"}}]`), "source")
	if e != nil || p[0].Coordinates != nil || p[0].Category != nil {
		t.Fatal("missing lookup values must remain null")
	}
}
func TestRejectSourceContradictions(t *testing.T) {
	arrive := string(fixture(t, "arrive.html"))
	poi := string(fixture(t, "poi.html"))
	cases := []struct {
		name, body string
		q          Query
	}{
		{"challenge", `<html><script src="https://x.token.awswaf.com/challenge.js"></script></html>`, testQuery()},
		{"login", "<html><title>Login</title></html>", testQuery()},
		{"ignored mode", arrive, testQuery()},
		{"ignored pass", string(fixture(t, "depart.html")), Query{From: "00006668", To: "00001756", DepartAt: "2026-09-28T09:00", Pass: "japan_rail_pass"}},
		{"wrong POI", poi, Query{From: "00006668", To: "spot:02301-0000000", DepartAt: "2026-09-28T09:00"}},
		{"late arrival", strings.ReplaceAll(arrive, "2026-09-28T12:00", "2026-09-28T09:30"), Query{From: "00006668", To: "00001756", ArriveBy: "2026-09-28T09:30"}},
		{"arrival seconds exceed exact deadline", strings.ReplaceAll(arrive, "20260928T004800Z/20260928T030000Z", "20260928T004800Z/20260928T030045Z"), Query{From: "00006668", To: "00001756", ArriveBy: "2026-09-28T12:00"}},
		{"early departure", strings.ReplaceAll(strings.ReplaceAll(arrive, "goal-time", "start-time"), "2026-09-28T12:00", "2026-09-28T11:00"), Query{From: "00006668", To: "00001756", DepartAt: "2026-09-28T11:00"}},
		{"reversed calendar", strings.ReplaceAll(arrive, "20260928T004800Z/20260928T030000Z", "20260928T030000Z/20260928T004800Z"), Query{From: "00006668", To: "00001756", ArriveBy: "2026-09-28T12:00"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := parseRoutes([]byte(tc.body), tc.q, "source")
			var source *SourceError
			if !errors.As(err, &source) {
				t.Fatalf("expected typed source contradiction, got %v", err)
			}
			assertJSON(t, "rejected_source", map[string]any{"case": tc.name, "message": err.Error()})
		})
	}
}
func TestSourceMetricsAndTransportEvidence(t *testing.T) {
	for _, tc := range []struct{ markup, line, want string }{{"<div></div>", "Kanagawa Express", "unknown"}, {"<div></div>", "Kanazawa Shuttle", "unknown"}, {"<div></div>", "Unverified Service", "unknown"}, {"<div></div>", "ANA", "flight"}, {"<div></div>", "Hikari", "rail"}, {`<div><img src="https://railroad-icon.common.navitime.jp/a.png"></div>`, "Kanazawa Local", "rail"}, {`<div><div class="walk-summary">Walk</div></div>`, "Walk", "walking"}} {
		root, _ := html.Parse(strings.NewReader(tc.markup))
		if got := classifyMove(root, tc.line); got != tc.want {
			t.Errorf("%s: %s want %s", tc.line, got, tc.want)
		}
	}
	for _, tc := range []struct {
		raw  string
		want *int
	}{{"", nil}, {"Unknown", nil}, {"JPY 0", ptr(0)}, {"JPY 13,320", ptr(13320)}} {
		got := money(tc.raw)
		if (got == nil) != (tc.want == nil) || got != nil && *got != *tc.want {
			t.Errorf("money %q: %v", tc.raw, got)
		}
	}
	for _, tc := range []struct {
		raw  string
		want *int
	}{{"", nil}, {"0m", ptr(0)}, {"49m", ptr(49)}, {"0.5km", ptr(500)}, {"2 miles", nil}} {
		got := distance(tc.raw)
		if (got == nil) != (tc.want == nil) || got != nil && *got != *tc.want {
			t.Errorf("distance %q: %v", tc.raw, got)
		}
	}
	root, _ := html.Parse(strings.NewReader(`<div class="route-section-point"><span class="point-name">Intermediate</span><input name="node_id" value="00004254"></div>`))
	if p := point(firstClass(root, "route-section-point")); p.SourceID == nil || *p.SourceID != "00004254" {
		t.Fatal("explicit intermediate point ID lost")
	}
	b, e := json.Marshal(Metadata{})
	if e != nil || !strings.Contains(string(b), `"fetched_at":null`) {
		t.Fatalf("unknown metadata time: %s", b)
	}
	client, e := NewClient(Options{NoCache: true})
	if e != nil {
		t.Fatal(e)
	}
	latest, e := client.Show(context.Background(), "latest")
	if e != nil || latest.Route != nil {
		t.Fatal("empty latest fabricated snapshot")
	}
}
