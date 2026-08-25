// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.

package icp

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func mustMap(t *testing.T, s string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatalf("bad fixture: %v", err)
	}
	return m
}

// Real payload shapes captured live from app.iclasspro.com on 2026-08-05.
const classFixture = `{
  "id": 16010,
  "name": "Culver: Thursday 08/06 at 5:30am - Long Course",
  "minAgeYear": 18, "maxAgeYear": 100,
  "schedule": [{"dayNumber":5,"startTime":"5:30AM","endTime":"6:30AM","dayName":"Thu"}],
  "programId": 57, "levelId": null,
  "instructors": ["Margot Newcomer"],
  "showOpenings": true, "openings": 26, "futureOpenings": 0,
  "allowWaitlist": true, "autoApprove": true,
  "dates": {"start": null, "end": null, "regStart": null, "regEnd": null},
  "availableDates": ["2026-08-06"],
  "startDate": "2026-08-06", "endDate": ""
}`

const campFixture = `{
  "id": 2035,
  "name": "Open Gym | Ages 5-13 | Saturday September 19th",
  "programId": 601, "programName": "OPEN GYMS", "typeId": 5,
  "minAge": 5, "maxAge": 12,
  "startDate": "2026-09-19", "endDate": "2026-09-19",
  "registrationStartDate": "2025-01-30", "registrationEndDate": "2026-09-19",
  "schedule": [{"id":3911,"campId":2035,"dayInt":7,"startTime":"2:00PM","endTime":"5:00PM"}],
  "blocks": [{"bid":4803,"tsid":3911,"sqlDate":"2026-09-19 00:00:00"}],
  "roomName": "Main Gym", "instructors": [],
  "hasOpenings": true, "openings": 10, "showOpenings": true,
  "allowToRequestCampThatIsFull": true,
  "programIsDeleted": false, "campRegisterExpired": false,
  "image": "129/camps/abc.png",
  "description": "<p>Bring a friend</p>"
}`

func TestNormalizeClass(t *testing.T) {
	e := NormalizeClass(mustMap(t, classFixture), "scaq", 1)
	if e.Kind != KindClass || e.ID != 16010 {
		t.Fatalf("kind/id wrong: %+v", e)
	}
	if e.Openings != 26 || !e.HasOpenings {
		t.Errorf("openings = %d hasOpenings=%v, want 26/true", e.Openings, e.HasOpenings)
	}
	if !e.AllowWaitlist {
		t.Error("allowWaitlist should be true")
	}
	if e.LevelID != 0 {
		t.Errorf("null levelId should normalize to 0, got %d", e.LevelID)
	}
	if e.PortalURL != "https://portal.iclasspro.com/scaq/class-details/16010" {
		t.Errorf("portal url = %q", e.PortalURL)
	}
	if len(e.Slots) != 1 || e.Slots[0].Date != "2026-08-06" {
		t.Fatalf("expected one dated slot, got %+v", e.Slots)
	}
	if e.Slots[0].StartTime != "5:30AM" {
		t.Errorf("start time = %q", e.Slots[0].StartTime)
	}
}

func TestNormalizeCamp(t *testing.T) {
	e := NormalizeCamp(mustMap(t, campFixture), "scottsdalegymnastics", 1)
	if e.Kind != KindCamp || e.ID != 2035 {
		t.Fatalf("kind/id wrong: %+v", e)
	}
	if e.ProgramName != "OPEN GYMS" || e.RoomName != "Main Gym" {
		t.Errorf("program/room wrong: %+v", e)
	}
	if e.RegStart != "2025-01-30" || e.RegEnd != "2026-09-19" {
		t.Errorf("registration window wrong: %q..%q", e.RegStart, e.RegEnd)
	}
	// allowWaitlist on camps comes from allowToRequestCampThatIsFull.
	if !e.AllowWaitlist {
		t.Error("camp waitlist should map from allowToRequestCampThatIsFull")
	}
	if e.Image != MediaBase+"129/camps/abc.png" {
		t.Errorf("image not absolutized: %q", e.Image)
	}
	// blocks[].sqlDate carries a trailing time that must be trimmed.
	if len(e.Slots) != 1 || e.Slots[0].Date != "2026-09-19" {
		t.Fatalf("expected slot dated 2026-09-19, got %+v", e.Slots)
	}
}

func TestMediaURL(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},
		{"129/a.png", MediaBase + "129/a.png"},
		{"/129/a.png", MediaBase + "129/a.png"},
		{"https://cdn.example/x.png", "https://cdn.example/x.png"},
	}
	for _, c := range cases {
		if got := MediaURL(c.in); got != c.want {
			t.Errorf("MediaURL(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestStatus(t *testing.T) {
	cases := []struct {
		e    Entity
		want string
	}{
		{Entity{Openings: 3}, "open (3)"},
		{Entity{Openings: 0, FutureOpenings: 2}, "future (2)"},
		{Entity{Openings: 0, AllowWaitlist: true}, "waitlist"},
		{Entity{Openings: 0}, "full"},
	}
	for _, c := range cases {
		if got := c.e.Status(); got != c.want {
			t.Errorf("Status() = %q, want %q", got, c.want)
		}
	}
}

func TestOpensSoon(t *testing.T) {
	now := time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC)
	ents := []Entity{
		{Kind: KindCamp, Account: "a", ID: 1, Name: "opens in 3", RegStart: "2026-08-08"},
		{Kind: KindCamp, Account: "a", ID: 2, Name: "opens in 40", RegStart: "2026-09-14"},
		{Kind: KindCamp, Account: "a", ID: 3, Name: "closes in 2", RegStart: "2026-01-01", RegEnd: "2026-08-07"},
		{Kind: KindCamp, Account: "a", ID: 4, Name: "no window"},
		{Kind: KindCamp, Account: "a", ID: 5, Name: "deleted", RegStart: "2026-08-06", ProgramDeleted: true},
	}
	got := OpensSoon(ents, now, 14)
	if len(got) != 2 {
		t.Fatalf("want 2 findings inside 14d, got %d: %+v", len(got), got)
	}
	// Sorted by days away: the one closing in 2 days precedes the one opening in 3.
	if got[0].Entity.ID != 3 || got[0].State != WindowClosing || got[0].DaysAway != 2 {
		t.Errorf("first finding wrong: %+v", got[0])
	}
	if got[1].Entity.ID != 1 || got[1].State != WindowUpcoming || got[1].DaysAway != 3 {
		t.Errorf("second finding wrong: %+v", got[1])
	}
}

func TestOpensSoonSkipsEntitiesWithoutWindows(t *testing.T) {
	now := time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC)
	got := OpensSoon([]Entity{{Kind: KindClass, ID: 9, Name: "always open"}}, now, 30)
	if len(got) != 0 {
		t.Fatalf("entities with no registration dates must not be reported, got %+v", got)
	}
}

func TestDiff(t *testing.T) {
	prev := []Entity{
		{Kind: KindClass, Account: "a", ID: 1, Name: "Level 1", Openings: 5},
		{Kind: KindClass, Account: "a", ID: 2, Name: "Level 2", Openings: 0},
		{Kind: KindClass, Account: "a", ID: 3, Name: "Gone", Openings: 1},
	}
	cur := []Entity{
		{Kind: KindClass, Account: "a", ID: 1, Name: "Level 1", Openings: 2},
		{Kind: KindClass, Account: "a", ID: 2, Name: "Level Two", Openings: 0},
		{Kind: KindClass, Account: "a", ID: 4, Name: "Brand New", Openings: 8},
	}
	kinds := map[ChangeKind]int{}
	for _, c := range Diff(prev, cur) {
		kinds[c.Kind]++
	}
	if kinds[ChangeOpenings] != 1 {
		t.Errorf("want 1 openings change, got %d", kinds[ChangeOpenings])
	}
	if kinds[ChangeRenamed] != 1 {
		t.Errorf("want 1 rename, got %d", kinds[ChangeRenamed])
	}
	if kinds[ChangeAdded] != 1 {
		t.Errorf("want 1 addition, got %d", kinds[ChangeAdded])
	}
	if kinds[ChangeRemoved] != 1 {
		t.Errorf("want 1 removal, got %d", kinds[ChangeRemoved])
	}
}

// A failed or empty sync must never be reported as a mass deletion.
func TestDiffEmptyCurrentReportsNoRemovals(t *testing.T) {
	prev := []Entity{{Kind: KindClass, Account: "a", ID: 1, Name: "x"}}
	for _, c := range Diff(prev, nil) {
		if c.Kind == ChangeRemoved {
			t.Fatalf("empty current snapshot must not produce removals, got %+v", c)
		}
	}
}

func TestDiffNoChangeIsEmpty(t *testing.T) {
	same := []Entity{{Kind: KindClass, Account: "a", ID: 1, Name: "x", Openings: 3}}
	if got := Diff(same, same); len(got) != 0 {
		t.Fatalf("identical snapshots must produce no drift, got %+v", got)
	}
}

func TestFillRates(t *testing.T) {
	t0 := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	samples := []Sample{
		{Key: "a/class/1", Account: "a", Kind: KindClass, EntityID: 1, Name: "Filling", Openings: 10, ObservedAt: t0},
		{Key: "a/class/1", Account: "a", Kind: KindClass, EntityID: 1, Name: "Filling", Openings: 6, ObservedAt: t0.Add(48 * time.Hour)},
		{Key: "a/class/2", Account: "a", Kind: KindClass, EntityID: 2, Name: "Flat", Openings: 4, ObservedAt: t0},
		{Key: "a/class/2", Account: "a", Kind: KindClass, EntityID: 2, Name: "Flat", Openings: 4, ObservedAt: t0.Add(24 * time.Hour)},
		{Key: "a/class/3", Account: "a", Kind: KindClass, EntityID: 3, Name: "Single", Openings: 1, ObservedAt: t0},
	}
	got := FillRates(samples)
	if len(got) != 2 {
		t.Fatalf("single-sample entities must be skipped; want 2 trends, got %d", len(got))
	}
	if got[0].EntityID != 1 || got[0].Direction != "filling" {
		t.Fatalf("expected the filling class first: %+v", got[0])
	}
	// 4 seats over 2 days = 2/day.
	if got[0].PerDay != 2 {
		t.Errorf("PerDay = %v, want 2", got[0].PerDay)
	}
	if got[0].ProjectedETA == "" {
		t.Error("a filling class with remaining openings should project a full date")
	}
	if got[1].Direction != "flat" || got[1].PerDay != 0 {
		t.Errorf("flat trend wrong: %+v", got[1])
	}
}

func TestLint(t *testing.T) {
	now := time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC)
	ents := []Entity{
		{Kind: KindCamp, Account: "a", ID: 1, Name: "Deleted", Detailed: true, ProgramDeleted: true, Description: "d", Image: "i", Slots: []Slot{{Date: "2026-08-06"}}},
		{Kind: KindCamp, Account: "a", ID: 2, Name: "No desc", Detailed: true, Image: "i", Slots: []Slot{{Date: "2026-08-06"}}},
		{Kind: KindCamp, Account: "a", ID: 3, Name: "Stale window", Detailed: true, Description: "d", Image: "i", RegEnd: "2026-07-01", Slots: []Slot{{Date: "2026-08-06"}}},
		{Kind: KindClass, Account: "a", ID: 4, Name: "Dead end", Openings: 0, AllowWaitlist: false, Slots: []Slot{{Date: "2026-08-06"}}},
	}
	rules := map[string]int{}
	for _, f := range Lint(ents, now) {
		rules[f.Rule]++
	}
	for _, want := range []string{"deleted_but_listed", "missing_description", "stale_registration_window", "full_without_waitlist"} {
		if rules[want] == 0 {
			t.Errorf("expected rule %q to fire", want)
		}
	}
	// Errors must sort ahead of warnings and info.
	all := Lint(ents, now)
	if all[0].Severity != "error" {
		t.Errorf("findings must be severity-ordered, first = %q", all[0].Severity)
	}
}

func TestLintCleanCatalogIsEmpty(t *testing.T) {
	now := time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC)
	clean := []Entity{{
		Kind: KindCamp, Account: "a", ID: 1, Name: "Good", Detailed: true, Description: "d", Image: "i",
		Openings: 4, RegEnd: "2026-12-01", Slots: []Slot{{Date: "2026-09-01"}},
	}}
	if got := Lint(clean, now); len(got) != 0 {
		t.Fatalf("a clean catalog must produce no findings, got %+v", got)
	}
}

func TestParseClock(t *testing.T) {
	cases := []struct {
		in   string
		h, m int
		ok   bool
	}{
		{"5:30AM", 5, 30, true},
		{"2:00PM", 14, 0, true},
		{"12:15 PM", 12, 15, true},
		{"14:45", 14, 45, true},
		{"", 0, 0, false},
		{"nonsense", 0, 0, false},
	}
	for _, c := range cases {
		h, m, ok := parseClock(c.in)
		if ok != c.ok || (ok && (h != c.h || m != c.m)) {
			t.Errorf("parseClock(%q) = %d:%d,%v want %d:%d,%v", c.in, h, m, ok, c.h, c.m, c.ok)
		}
	}
}

func TestRenderICS(t *testing.T) {
	stamp := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	ents := []Entity{
		NormalizeCamp(mustMap(t, campFixture), "scottsdalegymnastics", 1),
		{Kind: KindClass, Account: "a", ID: 99, Name: "No schedule"},
	}
	out, skipped := RenderICS(ents, stamp)
	if skipped != 1 {
		t.Errorf("entity without dated slots should be counted as skipped, got %d", skipped)
	}
	if !strings.HasPrefix(out, "BEGIN:VCALENDAR\r\n") {
		t.Error("calendar must start with BEGIN:VCALENDAR and CRLF")
	}
	if !strings.HasSuffix(out, "END:VCALENDAR\r\n") {
		t.Error("calendar must end with END:VCALENDAR")
	}
	if strings.Count(out, "BEGIN:VEVENT") != 1 {
		t.Errorf("want exactly 1 event, got %d", strings.Count(out, "BEGIN:VEVENT"))
	}
	if !strings.Contains(out, "DTSTART:20260919T140000") {
		t.Errorf("2:00PM on 2026-09-19 should render as 20260919T140000:\n%s", out)
	}
	if !strings.Contains(out, "DTEND:20260919T170000") {
		t.Error("end time should render from the slot's endTime")
	}
	if !strings.Contains(out, "LOCATION:Main Gym") {
		t.Error("room name should become LOCATION")
	}
	if !strings.Contains(out, "URL:https://portal.iclasspro.com/scottsdalegymnastics/camp-details/2035") {
		t.Error("portal deep link should become URL")
	}
}

func TestICSEscape(t *testing.T) {
	e := Entity{
		Kind: KindCamp, Account: "a", ID: 1,
		Name:  "Camp; with, commas",
		Slots: []Slot{{Date: "2026-09-19"}},
	}
	out, _ := RenderICS([]Entity{e}, time.Unix(0, 0).UTC())
	if !strings.Contains(out, `SUMMARY:Camp\; with\, commas`) {
		t.Errorf("semicolons and commas must be escaped:\n%s", out)
	}
}

func TestCompare(t *testing.T) {
	ents := []Entity{
		{Account: "a", Kind: KindCamp, ProgramName: "OPEN GYMS", Openings: 4, MinAge: 5, MaxAge: 12},
		{Account: "a", Kind: KindCamp, ProgramName: "OPEN GYMS", Openings: 0, MinAge: 6, MaxAge: 14},
		{Account: "b", Kind: KindCamp, ProgramName: "OPEN GYMS", Openings: 2, MinAge: 4, MaxAge: 10},
	}
	rows := Compare(ents)
	if len(rows) != 2 {
		t.Fatalf("want one row per account/bucket, got %d: %+v", len(rows), rows)
	}
	if rows[0].Account != "a" || rows[0].Entities != 2 || rows[0].WithOpenings != 1 || rows[0].Full != 1 {
		t.Errorf("account a aggregate wrong: %+v", rows[0])
	}
	if rows[0].AvgOpenings != 2 {
		t.Errorf("avg openings = %v, want 2", rows[0].AvgOpenings)
	}
	if rows[0].MinAge != 5 || rows[0].MaxAge != 14 {
		t.Errorf("age band should widen across entities: %+v", rows[0])
	}
}

func TestCompareFallsBackToKindWhenProgramNameMissing(t *testing.T) {
	rows := Compare([]Entity{{Account: "a", Kind: KindClass, Openings: 1}})
	if len(rows) != 1 || rows[0].Bucket != KindClass {
		t.Fatalf("bucket should fall back to entity kind, got %+v", rows)
	}
}

func TestNormalizeDate(t *testing.T) {
	cases := []struct{ in, want string }{
		{"2026-09-19 00:00:00", "2026-09-19"},
		{"2026-09-19T14:00:00Z", "2026-09-19"},
		{"2026-09-19", "2026-09-19"},
		{"", ""},
	}
	for _, c := range cases {
		if got := normalizeDate(c.in); got != c.want {
			t.Errorf("normalizeDate(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// A camp synced from the list endpoint has no description field in its payload
// at all. Flagging it as "missing description" would mark an entire catalog
// defective for data the response never carried.
func TestLintDoesNotFlagListSourcedCampsForDetailOnlyFields(t *testing.T) {
	now := time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC)
	listCamp := Entity{
		Kind: KindCamp, Account: "a", ID: 7, Name: "From list endpoint",
		Openings: 4, RegEnd: "2026-12-01", Slots: []Slot{{Date: "2026-09-01"}},
		// Detailed is false: description and image were never in the payload.
	}
	for _, f := range Lint([]Entity{listCamp}, now) {
		if f.Rule == "missing_description" || f.Rule == "missing_image" {
			t.Fatalf("detail-only rule %q must not fire on a list-sourced camp", f.Rule)
		}
	}
}

func TestNormalizeCampMarksDetailPayloads(t *testing.T) {
	detail := NormalizeCamp(mustMap(t, campFixture), "a", 1)
	if !detail.Detailed {
		t.Error("a payload carrying description/blocks should be marked detailed")
	}
	list := NormalizeCamp(mustMap(t, `{"id":1,"name":"x","startDate":"2026-09-19"}`), "a", 1)
	if list.Detailed {
		t.Error("a list payload without description or blocks must not be marked detailed")
	}
}

// Camps returned by the list endpoint carry startDate/endDate but no blocks and
// no availableDates. Leaving them undated silently dropped every camp from the
// calendar export.
func TestSpanDatesFallbackKeepsListCampsInTheCalendar(t *testing.T) {
	e := NormalizeCamp(mustMap(t, `{
		"id": 11, "name": "Week camp",
		"startDate": "2026-08-03", "endDate": "2026-08-07",
		"schedule": [{"dayInt":2,"startTime":"9:00AM","endTime":"12:00PM"}]
	}`), "a", 1)
	if len(e.Slots) == 0 {
		t.Fatal("a camp with a start/end span must produce dated slots")
	}
	// dayInt 2 is Monday; only 2026-08-03 falls on a Monday inside the span.
	if len(e.Slots) != 1 || e.Slots[0].Date != "2026-08-03" {
		t.Fatalf("weekday filter should yield only Monday 2026-08-03, got %+v", e.Slots)
	}
	out, skipped := RenderICS([]Entity{e}, time.Unix(0, 0).UTC())
	if skipped != 0 {
		t.Errorf("span-dated camp must not be skipped, skipped=%d", skipped)
	}
	if !strings.Contains(out, "DTSTART:20260803T090000") {
		t.Errorf("expected a 9am event on 2026-08-03:\n%s", out)
	}
}

func TestSpanDatesSingleDayCamp(t *testing.T) {
	e := NormalizeCamp(mustMap(t, `{"id":12,"name":"One day","startDate":"2026-09-19","endDate":"2026-09-19"}`), "a", 1)
	if len(e.Slots) != 1 || e.Slots[0].Date != "2026-09-19" {
		t.Fatalf("single-day camp should yield exactly one slot, got %+v", e.Slots)
	}
}

func TestSpanDatesIgnoredWhenExplicitDatesExist(t *testing.T) {
	// blocks[] wins over the start/end span so detail payloads keep their real
	// session dates.
	e := NormalizeCamp(mustMap(t, campFixture), "a", 1)
	if len(e.Slots) != 1 || e.Slots[0].Date != "2026-09-19" {
		t.Fatalf("explicit block dates must win over the span fallback, got %+v", e.Slots)
	}
}
