// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package napcamp

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, e := os.ReadFile(filepath.Join("testdata", name+".json"))
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func rawObject(t *testing.T, name string) Object {
	t.Helper()
	var r Object
	if e := json.Unmarshal(fixture(t, name), &r); e != nil {
		t.Fatal(e)
	}
	return r
}
func TestInputValidation(t *testing.T) {
	for _, c := range []struct {
		id string
		ok bool
	}{{"11007", true}, {"20005062", true}, {"0", false}, {"-2", false}, {"../11007", false}, {"", false}} {
		t.Run("id/"+c.id, func(t *testing.T) {
			if (ValidateID(c.id) == nil) != c.ok {
				t.Fatalf("ID validation %q", c.id)
			}
		})
	}
	for _, c := range []struct {
		month string
		ok    bool
	}{{"2026-10", true}, {"2026-2", false}, {"2026-13", false}, {"garbage", false}} {
		t.Run("month/"+c.month, func(t *testing.T) {
			_, e := ValidateMonth(c.month)
			if (e == nil) != c.ok {
				t.Fatal(e)
			}
		})
	}
	for _, c := range []struct {
		in, out string
		ok      bool
	}{{"", "", true}, {"2026-10-05", "2026-10-06", true}, {"2026-10-05", "", false}, {"2026-10-05", "2026-10-04", false}, {"2026-02-30", "2026-03-01", false}, {"2026-10-01", "2026-12-01", false}} {
		if (ValidateDates(c.in, c.out) == nil) != c.ok {
			t.Fatalf("date validation %#v", c)
		}
	}
	if Now() == "" {
		t.Fatal("timestamp missing")
	}
}
func TestCampsiteNormalization(t *testing.T) {
	for _, c := range []struct {
		raw       Object
		name, url string
	}{{rawObject(t, "campsite-11007"), "長瀞オートキャンプ場", Origin + "/saitama/11007"}, {Object{}, "", ""}, {Object{"id": 11007, "prefecture_name_en": "../evil", "name": "test"}, "test", ""}} {
		got := NormalizeCampsite(c.raw)
		if got["name"] != c.name || got["canonical_url"] != c.url {
			t.Fatalf("unexpected campsite %#v", got)
		}
		if got["full_dated_total"] != nil {
			t.Fatal("invented full total")
		}
		if got["facility_labels"] == nil || got["vehicle_categories"] == nil {
			t.Fatal("nil lists")
		}
		if Canonical(c.raw) != c.url {
			t.Fatal("canonical validation failed")
		}
	}
}
func TestPitchNormalization(t *testing.T) {
	camp := NormalizeCampsite(rawObject(t, "campsite-11007"))
	for _, c := range []struct {
		raw            Object
		power, vehicle string
		capacity       any
	}{{rawObject(t, "plan-20005062"), "no", "yes", 5}, {Object{}, "unknown", "unknown", nil}} {
		p := NormalizePitch(c.raw, camp)
		if p["power"] != c.power || p["vehicle_entry"] != c.vehicle || p["capacity"] != c.capacity {
			t.Fatalf("unexpected basic data %#v", p)
		}
		d := object(p["vehicle_dimensions"])
		if d["length_m"] != nil || d["width_m"] != nil || d["height_m"] != nil {
			t.Fatal("invented vehicle clearance")
		}
	}
	p := NormalizePitch(rawObject(t, "plan-20005062"), camp)
	if p["vehicle_memo"] != "乗用車" || !strings.Contains(p["pitch_area_source"].(string), "150") {
		t.Fatal("lost Japanese restriction")
	}
	if p["canonical_url"] != Origin+"/saitama/11007/plans/20005062" {
		t.Fatal("plan handoff")
	}
}
func TestCalendarNormalizationAndBounds(t *testing.T) {
	for _, c := range []struct {
		raw       []Object
		want      string
		candidate bool
		bad       bool
	}{{[]Object{{"date": []any{2026, 10, 5}, "status": 1, "price": Object{"guideline": 6000}}}, "受付中", true, false}, {[]Object{{"date": []any{2026, 10, 6}, "status": 0, "price": Object{"guideline": 0}}}, "準備中", false, false}, {[]Object{{"date": []any{2026, 10, 7}, "status": 9}}, "unknown source status", false, false}, {[]Object{{"date": []any{2026, 2, 30}}}, "", false, true}} {
		days, e := NormalizeCalendar(c.raw)
		if (e != nil) != c.bad {
			t.Fatalf("calendar error %v", e)
		}
		if c.bad {
			continue
		}
		if days[0]["source_label"] != c.want || days[0]["acceptance_candidate"] != c.candidate {
			t.Fatal(days)
		}
		if !c.candidate && object(days[0]["price"])["guideline_jpy"] != nil {
			t.Fatal("zero price became free")
		}
	}
	var raw []Object
	if e := json.Unmarshal(fixture(t, "calendar-20005062"), &raw); e != nil {
		t.Fatal(e)
	}
	days, e := NormalizeCalendar(raw)
	if e != nil {
		t.Fatal(e)
	}
	view := CalendarView(days, "2026-10", 7)
	rows := view["days"].([]Object)
	if len(rows) != 7 || view["truncated"] != true {
		t.Fatal(view)
	}
	for _, d := range rows {
		if !strings.HasPrefix(d["date"].(string), "2026-10-") {
			t.Fatal("leaked next month")
		}
	}
	dup := []Object{{"date": []any{2026, 10, 5}}, {"date": []any{2026, 10, 5}}}
	if _, e := NormalizeCalendar(dup); e == nil {
		t.Fatal("duplicate dates accepted")
	}
	empty, e := NormalizeCalendar([]Object{})
	if e != nil || empty == nil {
		t.Fatal("empty must be []")
	}
}
func TestFitNeverCertifiesCategoryOrArea(t *testing.T) {
	c := NormalizeCampsite(rawObject(t, "campsite-11007"))
	p := NormalizePitch(rawObject(t, "plan-20005062"), c)
	for _, x := range []struct {
		req     Requirements
		verdict string
	}{{Requirements{People: 2}, "needs_confirmation"}, {Requirements{Power: true}, "contradiction"}, {Requirements{People: 6}, "contradiction"}, {Requirements{Length: 6, Width: 2.2, Height: 2.9, Pets: true}, "needs_confirmation"}} {
		r := Fit(c, p, x.req)
		if r["verdict"] != x.verdict {
			t.Fatal(r)
		}
		checks := r["checks"].([]Object)
		for _, v := range checks {
			if v["requirement"] == "vehicle_dimensions" && v["status"] != "unknown" {
				t.Fatal("area certified clearance")
			}
			if v["requirement"] == "campervan_entry" && v["status"] != "unknown" {
				t.Fatal("passenger cars certified campervans")
			}
		}
	}
}
func TestVehicleMemoNeverCreatesPositivePermission(t *testing.T) {
	for _, memo := range []string{"キャンピングカー", "キャンピングカー不可", "キャンピングカーは事前確認", "大型キャンピングカー禁止"} {
		got := Fit(Object{"id": "11007"}, Object{"id": "20005062", "vehicle_entry": "yes", "vehicle_memo": memo}, Requirements{})
		for _, check := range got["checks"].([]Object) {
			if check["requirement"] == "campervan_entry" && check["status"] != "unknown" {
				t.Fatal("free text became permission", memo, check)
			}
		}
	}
}

func TestWindowsStayNightsExcludeCheckoutAndMissingDates(t *testing.T) {
	for _, c := range []struct {
		nights int
		count  int
		bad    bool
	}{{1, 1, false}, {2, 0, false}, {0, 0, true}} {
		days, e := NormalizeCalendar([]Object{{"date": []any{2026, 10, 5}, "status": 1, "price": Object{"guideline": 6000}}, {"date": []any{2026, 10, 6}, "status": 0, "price": Object{"guideline": 0}}})
		if e != nil {
			t.Fatal(e)
		}
		r, e := Windows(days, "2026-10", c.nights, 5)
		if (e != nil) != c.bad {
			t.Fatal(e)
		}
		if c.bad {
			continue
		}
		rows := r["candidate_windows"].([]Object)
		if len(rows) != c.count {
			t.Fatal(r)
		}
		if c.count == 1 {
			if rows[0]["check_out"] != "2026-10-06" || rows[0]["full_total"] != nil || rows[0]["vacancy"] != "unknown" {
				t.Fatal(rows)
			}
		}
		if r["unknown_dates"] == nil {
			t.Fatal("unknown dates list nil")
		}
	}
	days, _ := NormalizeCalendar([]Object{{"date": []any{2026, 10, 31}, "status": 1}, {"date": []any{2026, 11, 1}, "status": 2}})
	r, e := Windows(days, "2026-10", 2, 5)
	if e != nil || len(r["candidate_windows"].([]Object)) != 1 {
		t.Fatal("source-covered next-month night lost", r, e)
	}
}
func TestSnapshotsAndComparableDrift(t *testing.T) {
	before := Snapshot{SchemaVersion: 1, ObservedAt: "2026-10-02T16:00:00Z", Campsite: Object{"id": "11007", "name": "長瀞オートキャンプ場"}, Pitch: Object{"id": "20005062", "power": "no"}, Calendar: []Object{}, SourceURLs: []string{}}
	for _, c := range []struct {
		name   string
		change bool
	}{{"unchanged", false}, {"source-power-changed", true}} {
		after := before
		after.ObservedAt = "2026-10-03T16:00:00Z"
		after.Pitch = Object{"id": "20005062", "power": "no"}
		if c.change {
			after.Pitch["power"] = "yes"
		}
		r, e := Diff(before, after)
		if e != nil {
			t.Fatal(e)
		}
		rows := r["changes"].([]Object)
		if (len(rows) == 1) != c.change {
			t.Fatal(r)
		}
		if !c.change && rows == nil {
			t.Fatal("unchanged must emit []")
		}
	}
	after := before
	after.Pitch = Object{"id": "999", "power": "no"}
	if _, e := Diff(before, after); e == nil {
		t.Fatal("mixed entity comparison")
	}
	p := filepath.Join(t.TempDir(), "observation.json")
	if e := SaveSnapshot(p, before); e != nil {
		t.Fatal(e)
	}
	loaded, e := LoadSnapshot(p)
	if e != nil || loaded.Campsite["id"] != "11007" || loaded.Calendar == nil {
		t.Fatal(loaded, e)
	}
	if e := SaveSnapshot(p, before); e == nil {
		t.Fatal("existing observation overwritten")
	}
	only := before
	only.Calendar = []Object{{"date": "2026-10-05", "source_status": 1}}
	r, e := Diff(before, only)
	if e != nil || r["coverage_changed"] != true || len(r["changes"].([]Object)) != 0 {
		t.Fatal("coverage fabricated status drift", r, e)
	}
	if _, e := LoadSnapshot(filepath.Join(t.TempDir(), "missing.json")); e == nil {
		t.Fatal("missing file accepted")
	}
}

type fakeReader struct {
	t *testing.T
	q map[string]string
}

func (f *fakeReader) GetNoCache(_ context.Context, p string, q map[string]string) (json.RawMessage, error) {
	f.q = q
	name := map[string]string{"/api/campsite/11007": "campsite-11007", "/api/campsite/11007/plans": "plans-11007", "/api/campsite/11007/plans/20005062": "plan-20005062", "/api/campsite/11007/plans/20005062/reservation": "calendar-20005062", "/api/search": "search-kanto", "/api/master": "master", "/api/locations": "locations"}[p]
	if name == "" {
		return nil, errors.New("unexpected path " + p)
	}
	return fixture(f.t, name), nil
}
func TestPublicReadContracts(t *testing.T) {
	for _, kind := range []string{"campsite", "pitch", "plans", "discover", "calendar", "snapshot", "regions", "prefectures", "filters"} {
		t.Run(kind, func(t *testing.T) {
			f := &fakeReader{t: t}
			a := API{Reader: f}
			ctx := context.Background()
			c := NormalizeCampsite(rawObject(t, "campsite-11007"))
			switch kind {
			case "campsite":
				if _, e := a.Campsite(ctx, "11007"); e != nil {
					t.Fatal(e)
				}
			case "pitch":
				if _, e := a.Pitch(ctx, "11007", "20005062", c); e != nil {
					t.Fatal(e)
				}
			case "plans":
				v, e := a.Plans(ctx, "11007", c, 2)
				if e != nil || len(v["plans"].([]Object)) != 2 || v["truncated"] != true {
					t.Fatal(v, e)
				}
			case "discover":
				v, e := a.Discover(ctx, Query{RegionID: 2, Page: 1, Limit: 2, Filters: []int{43, 26}, CheckIn: "2026-10-05", CheckOut: "2026-10-06"})
				if e != nil || len(v["results"].([]Object)) != 2 {
					t.Fatal(v, e)
				}
				if f.q["other"] != "26,43" || f.q["check_in"] != "2026-10-05" || f.q["count"] != "2" {
					t.Fatal("dropped query", f.q)
				}
			case "calendar":
				if _, e := a.Calendar(ctx, "11007", "20005062", "2026-10"); e != nil || f.q["month"] != "2026-10" {
					t.Fatal(f.q, e)
				}
			case "snapshot":
				s, e := a.Snapshot(ctx, "11007", "20005062", "2026-10")
				if e != nil || len(s.Calendar) != 31 {
					t.Fatal(len(s.Calendar), e)
				}
			default:
				v, e := a.Catalog(ctx, kind, 3)
				if e != nil || len(v["results"].([]Object)) != 3 {
					t.Fatal(v, e)
				}
			}
		})
	}
}

type literalReader struct{ body string }

func (f literalReader) GetNoCache(context.Context, string, map[string]string) (json.RawMessage, error) {
	return json.RawMessage(f.body), nil
}
func TestMalformedShapesAreNotEmptyEvidence(t *testing.T) {
	for _, c := range []struct {
		kind, body string
		bad        bool
	}{{"calendar", "null", true}, {"calendar", "[]", false}, {"calendar", "{}", true}, {"filters", "null", true}, {"filters", "[]", false}, {"filters", "[{}]", true}, {"regions", "null", true}, {"regions", "{}", true}, {"regions", "{\"list_r\":null}", true}, {"regions", "{\"list_r\":[]}", false}} {
		a := API{Reader: literalReader{body: c.body}}
		var err error
		if c.kind == "calendar" {
			_, err = a.Calendar(context.Background(), "11007", "20005062", "2026-10")
		} else {
			_, err = a.Catalog(context.Background(), c.kind, 5)
		}
		if (err != nil) != c.bad {
			t.Fatalf("%s body=%s bad=%v got=%v", c.kind, c.body, c.bad, err)
		}
	}
}
func TestFractionalCalendarValuesNeverBecomeAcceptance(t *testing.T) {
	for _, status := range []any{1.5, 2.1, 9.0, nil, "bogus"} {
		days, err := NormalizeCalendar([]Object{{"date": []any{2026, 10, 5}, "status": status}})
		if err != nil || days[0]["acceptance_candidate"] != false {
			t.Fatal("status promoted", status, days, err)
		}
	}
	for _, date := range [][]any{{2026.5, 10, 5}, {2026, 10.5, 5}, {2026, 10, 5.5}} {
		if _, err := NormalizeCalendar([]Object{{"date": date, "status": 1}}); err == nil {
			t.Fatal("fractional date truncated", date)
		}
	}
}

func TestRichTextRulesKeepJapaneseEvidenceWithoutMarkup(t *testing.T) {
	cases := []struct{ raw, want string }{
		{"<!DOCTYPE html><html><head></head><body><p>3月～12月<br />※1月〜2月は縮小営業</p></body></html>", "3月～12月\n※1月〜2月は縮小営業"},
		{"<p>犬&amp;猫</p><p>車は1台まで</p>", "犬&猫\n車は1台まで"},
		{"<style>body{color:red}</style><script>ignore()</script><p>乗用車</p>", "乗用車"},
		{"利用条件 2 < 5 人", "利用条件 2 < 5 人"},
	}
	for _, c := range cases {
		if got := text(c.raw); got != c.want {
			t.Fatalf("got %q, want %q", got, c.want)
		}
	}
	camp := NormalizeCampsite(rawObject(t, "campsite-11007"))
	if strings.Contains(camp["season"].(string), "<") || !strings.Contains(camp["season"].(string), "3月") {
		t.Fatal("source season text lost or markup retained")
	}
}
