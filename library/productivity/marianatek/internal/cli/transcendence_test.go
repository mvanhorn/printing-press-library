// Copyright 2026 salmonumbrella. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"testing"
	"time"
)

func TestExtractSpotsLeft(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want int
	}{
		{"remaining_spots float", `{"data":{"attributes":{"remaining_spots":3}}}`, 3},
		{"spots_remaining alt key", `{"data":{"attributes":{"spots_remaining":7}}}`, 7},
		{"available_spots alt", `{"data":{"attributes":{"available_spots":1}}}`, 1},
		{"absent → 0", `{"data":{"attributes":{}}}`, 0},
		{"malformed → 0", `not json`, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := extractSpotsLeft(json.RawMessage(c.raw))
			if got != c.want {
				t.Fatalf("got %d, want %d", got, c.want)
			}
		})
	}
}

func TestParseHHMM(t *testing.T) {
	cases := []struct {
		in           string
		wantHH, wantMM int
		wantOK       bool
	}{
		{"07:00", 7, 0, true},
		{"23:59", 23, 59, true},
		{"00:00", 0, 0, true},
		{"24:00", 0, 0, false},
		{"12:60", 0, 0, false},
		{"7am", 0, 0, false},
		{"", 0, 0, false},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			hh, mm, ok := parseHHMM(c.in)
			if ok != c.wantOK {
				t.Fatalf("ok: got %v, want %v", ok, c.wantOK)
			}
			if ok && (hh != c.wantHH || mm != c.wantMM) {
				t.Fatalf("got %d:%d, want %d:%d", hh, mm, c.wantHH, c.wantMM)
			}
		})
	}
}

func TestNormalizeRegularsDim(t *testing.T) {
	cases := []struct {
		in   string
		want string
		err  bool
	}{
		{"instructor", "instructor", false},
		{"Instructors", "instructor", false},
		{"type", "type", false},
		{"class-type", "type", false},
		{"time", "time", false},
		{"time-of-day", "time", false},
		{"day", "day", false},
		{"location", "location", false},
		{"studio", "location", false},
		{"bogus", "", true},
		{"", "", true},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got, err := normalizeRegularsDim(c.in)
			if (err != nil) != c.err {
				t.Fatalf("err mismatch: got %v, wantErr=%v", err, c.err)
			}
			if got != c.want {
				t.Fatalf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestParseSlotKey(t *testing.T) {
	cases := []struct {
		in       string
		wantWD   time.Weekday
		wantHour int
		wantType string
		wantErr  bool
	}{
		{"tue-7am-vinyasa", time.Tuesday, 7, "vinyasa", false},
		{"wed-12pm-hot-yoga", time.Wednesday, 12, "hot-yoga", false},
		{"mon-12am-sauna", time.Monday, 0, "sauna", false},
		{"sat-17-cycling", time.Saturday, 17, "cycling", false},
		{"sun-5pm-cold-plunge", time.Sunday, 17, "cold-plunge", false},
		{"xyz-7am-foo", 0, 0, "", true},
		{"tue-99am-foo", 0, 0, "", true},
		{"tue", 0, 0, "", true},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			wd, hour, typ, err := parseSlotKey(c.in)
			if (err != nil) != c.wantErr {
				t.Fatalf("err mismatch: got %v, wantErr=%v", err, c.wantErr)
			}
			if c.wantErr {
				return
			}
			if wd != c.wantWD || hour != c.wantHour || typ != c.wantType {
				t.Fatalf("got (%v, %d, %q), want (%v, %d, %q)", wd, hour, typ, c.wantWD, c.wantHour, c.wantType)
			}
		})
	}
}

func TestDetectConflicts(t *testing.T) {
	base := time.Date(2026, 5, 15, 9, 0, 0, 0, time.UTC)
	events := []conflictEvent{
		{Source: "marianatek", Title: "Vinyasa @ A", Start: base, End: base.Add(60 * time.Minute)},
		{Source: "marianatek", Title: "Sauna @ B", Start: base.Add(45 * time.Minute), End: base.Add(90 * time.Minute)},
		{Source: "marianatek", Title: "Cold Plunge @ C", Start: base.Add(100 * time.Minute), End: base.Add(130 * time.Minute)},
		{Source: "ics", Title: "Lunch", Start: base.Add(180 * time.Minute), End: base.Add(240 * time.Minute)},
	}
	conflicts := detectConflicts(events, 30*time.Minute)
	// Expect 2 conflicts:
	//   (Vinyasa, Sauna) overlap
	//   (Sauna, Cold Plunge) tight_gap (10min < 30min buffer)
	if len(conflicts) != 2 {
		t.Fatalf("expected 2 conflicts, got %d: %+v", len(conflicts), conflicts)
	}
	if conflicts[0].Kind != "overlap" {
		t.Errorf("first conflict kind: got %q, want overlap", conflicts[0].Kind)
	}
	if conflicts[1].Kind != "tight_gap" {
		t.Errorf("second conflict kind: got %q, want tight_gap", conflicts[1].Kind)
	}
}

func TestRankRegularsBasic(t *testing.T) {
	reservations := []json.RawMessage{
		json.RawMessage(`{"data":{"id":"r1","attributes":{"class_session_id":"c1","created_at":"2026-05-01T07:00:00Z"}}}`),
		json.RawMessage(`{"data":{"id":"r2","attributes":{"class_session_id":"c2","created_at":"2026-05-08T07:00:00Z"}}}`),
		json.RawMessage(`{"data":{"id":"r3","attributes":{"class_session_id":"c1","created_at":"2026-05-15T07:00:00Z"}}}`),
	}
	classes := []json.RawMessage{
		json.RawMessage(`{"data":{"id":"c1","attributes":{"instructor_name":"Lauren K","start_datetime":"2026-05-15T07:00:00Z"}}}`),
		json.RawMessage(`{"data":{"id":"c2","attributes":{"instructor_name":"Sam P","start_datetime":"2026-05-08T07:00:00Z"}}}`),
	}
	classByID := indexClassesByID(classes)
	ranking := rankRegulars(reservations, classByID, "instructor")
	if len(ranking) != 2 {
		t.Fatalf("expected 2 dimension values, got %d: %+v", len(ranking), ranking)
	}
	if ranking[0].Value != "Lauren K" || ranking[0].Count != 2 {
		t.Errorf("top rank: got %+v, want Lauren K count=2", ranking[0])
	}
	if ranking[1].Value != "Sam P" || ranking[1].Count != 1 {
		t.Errorf("second rank: got %+v, want Sam P count=1", ranking[1])
	}
}

func TestParseWeekday(t *testing.T) {
	cases := []struct {
		in    string
		want  time.Weekday
		wantOK bool
	}{
		{"mon", time.Monday, true},
		{"monday", time.Monday, true},
		{"tue", time.Tuesday, true},
		{"weds", time.Wednesday, true},
		{"thursday", time.Thursday, true},
		{"sat", time.Saturday, true},
		{"sunday", time.Sunday, true},
		{"funday", 0, false},
		{"", 0, false},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got, ok := parseWeekday(c.in)
			if ok != c.wantOK {
				t.Fatalf("ok: got %v, want %v", ok, c.wantOK)
			}
			if ok && got != c.want {
				t.Fatalf("got %v, want %v", got, c.want)
			}
		})
	}
}

func TestParseHourAMPM(t *testing.T) {
	cases := []struct {
		in     string
		want   int
		wantOK bool
	}{
		{"7am", 7, true},
		{"12am", 0, true},
		{"12pm", 12, true},
		{"5pm", 17, true},
		{"17", 17, true},
		{"0", 0, true},
		{"23", 23, true},
		{"13pm", 0, false},
		{"25", 0, false},
		{"foo", 0, false},
		{"", 0, false},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got, ok := parseHourAMPM(c.in)
			if ok != c.wantOK {
				t.Fatalf("ok: got %v, want %v", ok, c.wantOK)
			}
			if ok && got != c.want {
				t.Fatalf("got %d, want %d", got, c.want)
			}
		})
	}
}
