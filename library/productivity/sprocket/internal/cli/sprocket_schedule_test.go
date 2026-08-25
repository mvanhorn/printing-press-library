// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	tm, ok := parseSprocketTime(s)
	if !ok {
		t.Fatalf("parseSprocketTime(%q) failed", s)
	}
	return tm
}

func TestParseSprocketTime(t *testing.T) {
	cases := []struct {
		in string
		ok bool
	}{
		{"2026-06-27T18:00:00", true},
		{"2026-06-27T18:00:00Z", true},
		{"2026-06-27T18:00:00.123", true},
		{"2026-06-27", true},
		{"", false},
		{"not-a-date", false},
	}
	for _, c := range cases {
		if _, ok := parseSprocketTime(c.in); ok != c.ok {
			t.Errorf("parseSprocketTime(%q) ok=%v, want %v", c.in, ok, c.ok)
		}
	}
}

func TestChunkRange(t *testing.T) {
	start := mustTime(t, "2026-06-01")
	end := mustTime(t, "2026-07-31") // 60 days span
	chunks := chunkRange(start, end, 30)
	if len(chunks) != 3 {
		t.Fatalf("expected 3 chunks for 61-day span at 30/chunk, got %d", len(chunks))
	}
	for _, c := range chunks {
		span := c[1].Sub(c[0]).Hours() / 24
		if span > 30 {
			t.Errorf("chunk %v..%v spans %.0f days, exceeds 30", c[0], c[1], span)
		}
	}
	if !chunks[0][0].Equal(start) {
		t.Errorf("first chunk should start at range start")
	}
	if !chunks[len(chunks)-1][1].Equal(end) {
		t.Errorf("last chunk should end at range end")
	}
}

func TestChunkRangeEmptyWhenEndBeforeStart(t *testing.T) {
	if got := chunkRange(mustTime(t, "2026-06-10"), mustTime(t, "2026-06-01"), 30); got != nil {
		t.Errorf("expected nil for inverted range, got %v", got)
	}
}

func TestWeekBounds(t *testing.T) {
	// 2026-06-18 is a Thursday.
	now := time.Date(2026, 6, 18, 14, 0, 0, 0, time.UTC)
	start, end := weekBounds(now)
	if start.Weekday() != time.Monday {
		t.Errorf("week start weekday = %v, want Monday", start.Weekday())
	}
	if end.Weekday() != time.Sunday {
		t.Errorf("week end weekday = %v, want Sunday", end.Weekday())
	}
	if got := start.Format("2006-01-02"); got != "2026-06-15" {
		t.Errorf("week start = %s, want 2026-06-15", got)
	}
	if got := end.Format("2006-01-02"); got != "2026-06-21" {
		t.Errorf("week end = %s, want 2026-06-21", got)
	}
}

func TestFlattenCalElement(t *testing.T) {
	raw := json.RawMessage(`{"id":123,"clubCalendarEvent":{"title":"League Game","startDate":"2026-06-27T18:00:00","endDate":"2026-06-27T19:30:00","opponent":"Rivals SC","awayGame":true,"clubEventTypeID":1,"teamID":456,"locationID":789,"isCancelled":false}}`)
	e := flattenCalElement(raw)
	if e.ID != 123 || e.Title != "League Game" || e.Opponent != "Rivals SC" {
		t.Fatalf("unexpected flatten: %+v", e)
	}
	if !e.AwayGame || e.TeamID != 456 || e.LocationID != 789 || e.EventTypeID != 1 {
		t.Fatalf("unexpected fields: %+v", e)
	}
	if !e.HasStart || e.Start.Format("2006-01-02 15:04") != "2026-06-27 18:00" {
		t.Fatalf("start not parsed: %+v", e)
	}
}

func TestFirstFutureEvent(t *testing.T) {
	now := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	events := []calEvent{
		{Title: "past", Start: now.Add(-time.Hour), HasStart: true},
		{Title: "soon", Start: now.Add(2 * time.Hour), HasStart: true},
		{Title: "later", Start: now.Add(48 * time.Hour), HasStart: true},
	}
	got := firstFutureEvent(events, now)
	if got == nil || got.Title != "soon" {
		t.Fatalf("firstFutureEvent = %+v, want 'soon'", got)
	}
	if firstFutureEvent(events[:1], now) != nil {
		t.Errorf("expected nil when all events are in the past")
	}
}

func TestFilterAway(t *testing.T) {
	events := []calEvent{{AwayGame: true, Title: "a"}, {AwayGame: false, Title: "h"}, {AwayGame: true, Title: "b"}}
	got := filterAway(events)
	if len(got) != 2 || got[0].Title != "a" || got[1].Title != "b" {
		t.Fatalf("filterAway = %+v", got)
	}
}

func TestDetectConflictsOverlap(t *testing.T) {
	events := []calEvent{
		{ID: 1, Title: "A", Start: mustTime(t, "2026-06-20T10:00:00"), End: mustTime(t, "2026-06-20T11:00:00"), HasStart: true},
		{ID: 2, Title: "B", Start: mustTime(t, "2026-06-20T10:30:00"), End: mustTime(t, "2026-06-20T11:30:00"), HasStart: true},
		{ID: 3, Title: "C", Start: mustTime(t, "2026-06-20T18:00:00"), End: mustTime(t, "2026-06-20T19:00:00"), HasStart: true},
	}
	got := detectConflicts(events, 60*time.Minute)
	if len(got) != 1 || got[0].Kind != "overlap" {
		t.Fatalf("expected exactly 1 overlap, got %+v", got)
	}
}

func TestDetectConflictsTightGap(t *testing.T) {
	events := []calEvent{
		{ID: 1, Title: "A", Start: mustTime(t, "2026-06-20T10:00:00"), End: mustTime(t, "2026-06-20T11:00:00"), HasStart: true, LocationID: 1},
		{ID: 2, Title: "B", Start: mustTime(t, "2026-06-20T11:15:00"), End: mustTime(t, "2026-06-20T12:00:00"), HasStart: true, LocationID: 2},
	}
	got := detectConflicts(events, 60*time.Minute)
	if len(got) != 1 || got[0].Kind != "tight-gap" || got[0].GapMinutes != 15 {
		t.Fatalf("expected 1 tight-gap of 15min, got %+v", got)
	}
}

func TestDetectConflictsSameLocationNoTightGap(t *testing.T) {
	events := []calEvent{
		{ID: 1, Title: "A", Start: mustTime(t, "2026-06-20T10:00:00"), End: mustTime(t, "2026-06-20T11:00:00"), HasStart: true, LocationID: 7},
		{ID: 2, Title: "B", Start: mustTime(t, "2026-06-20T11:15:00"), End: mustTime(t, "2026-06-20T12:00:00"), HasStart: true, LocationID: 7},
	}
	if got := detectConflicts(events, 60*time.Minute); len(got) != 0 {
		t.Fatalf("same-location tight gap should not conflict, got %+v", got)
	}
}

func TestBuildICS(t *testing.T) {
	now := time.Date(2026, 6, 18, 0, 0, 0, 0, time.UTC)
	events := []calEvent{
		{ID: 5, Title: "Game; vs, A\\B", Start: mustTime(t, "2026-06-27T18:00:00"), End: mustTime(t, "2026-06-27T19:30:00"), HasStart: true, Opponent: "Rivals", Location: "Field 1"},
		{ID: 6, Title: "No start", HasStart: false},
	}
	ics := buildICS(events, now)
	if !strings.HasPrefix(ics, "BEGIN:VCALENDAR\r\n") || !strings.Contains(ics, "END:VCALENDAR") {
		t.Fatalf("missing VCALENDAR envelope:\n%s", ics)
	}
	if strings.Count(ics, "BEGIN:VEVENT") != 1 {
		t.Errorf("expected exactly 1 VEVENT (undated event skipped), got %d", strings.Count(ics, "BEGIN:VEVENT"))
	}
	if !strings.Contains(ics, "SUMMARY:Game\\; vs\\, A\\\\B vs Rivals") {
		t.Errorf("summary not escaped correctly:\n%s", ics)
	}
	if !strings.Contains(ics, "DTSTART:20260627T180000") {
		t.Errorf("DTSTART missing:\n%s", ics)
	}
}

func TestDiffSnapshots(t *testing.T) {
	prev := map[string]snapEvent{
		"1": {Start: "2026-06-20T10:00:00", Title: "Keeps"},
		"2": {Start: "2026-06-21T10:00:00", Title: "Moves"},
		"3": {Start: "2026-06-22T10:00:00", Title: "GetsCancelled", Cancelled: false},
	}
	cur := []calEvent{
		{ID: 1, Title: "Keeps", StartRaw: "2026-06-20T10:00:00"},
		{ID: 2, Title: "Moves", StartRaw: "2026-06-21T14:00:00"},                          // moved
		{ID: 3, Title: "GetsCancelled", StartRaw: "2026-06-22T10:00:00", Cancelled: true}, // cancelled
		{ID: 4, Title: "BrandNew", StartRaw: "2026-06-25T10:00:00"},                       // added
	}
	rep := diffSnapshots(prev, cur)
	if len(rep.Added) != 1 || rep.Added[0].Title != "BrandNew" {
		t.Errorf("added = %+v", rep.Added)
	}
	if len(rep.Moved) != 1 || rep.Moved[0].Title != "Moves" || rep.Moved[0].OldStart != "2026-06-21T10:00:00" {
		t.Errorf("moved = %+v", rep.Moved)
	}
	if len(rep.Cancelled) != 1 || rep.Cancelled[0].Title != "GetsCancelled" {
		t.Errorf("cancelled = %+v", rep.Cancelled)
	}
}

func TestBuildOwedReport(t *testing.T) {
	reg := json.RawMessage(`[{"remainingBalance":100.5},{"remainingBalance":0},{"amountDue":25}]`)
	inv := json.RawMessage(`[{"amount":30}]`)
	rep, err := buildOwedReport(reg, inv)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rep.RegistrationsWithBalance != 2 {
		t.Errorf("registrationsWithBalance = %d, want 2", rep.RegistrationsWithBalance)
	}
	if rep.RegistrationBalance != 125.5 {
		t.Errorf("registrationBalance = %v, want 125.5", rep.RegistrationBalance)
	}
	if rep.OverdueInvoices != 30 || rep.OverdueInvoiceCount != 1 {
		t.Errorf("invoices = %v / %d", rep.OverdueInvoices, rep.OverdueInvoiceCount)
	}
	if rep.TotalOwed != 155.5 {
		t.Errorf("totalOwed = %v, want 155.5", rep.TotalOwed)
	}
}

func TestBuildOwedReportEmpty(t *testing.T) {
	rep, err := buildOwedReport(json.RawMessage(`[]`), json.RawMessage(`[]`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rep.TotalOwed != 0 || rep.RegistrationsWithBalance != 0 {
		t.Errorf("empty owed report should be zero, got %+v", rep)
	}
}

func TestBuildOwedReportStringMoney(t *testing.T) {
	// Some finance APIs encode money as strings; must still sum.
	reg := json.RawMessage(`[{"remainingBalance":"100.50"}]`)
	inv := json.RawMessage(`[{"amount":"30"}]`)
	rep, err := buildOwedReport(reg, inv)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rep.TotalOwed != 130.5 {
		t.Errorf("totalOwed = %v, want 130.5", rep.TotalOwed)
	}
}

func TestBuildOwedReportUninterpretable(t *testing.T) {
	// A non-empty body that is neither an array nor an object must error rather
	// than silently report $0.
	_, err := buildOwedReport(json.RawMessage(`"oops not json data"`), json.RawMessage(`[]`))
	if err == nil {
		t.Fatalf("expected error for uninterpretable registrations response")
	}
}

func TestAsObjectsWrapper(t *testing.T) {
	objs, ok := asObjects(json.RawMessage(`{"results":[{"a":1},{"a":2}]}`))
	if !ok || len(objs) != 2 {
		t.Fatalf("asObjects wrapper: ok=%v len=%d", ok, len(objs))
	}
	if _, ok := asObjects(json.RawMessage(``)); !ok {
		t.Errorf("empty body should be ok with no objects")
	}
}

func TestDetectConflictsGapWindowScan(t *testing.T) {
	// A same-location neighbor must not mask a later cross-location event that
	// is also within the travel-gap window. (Regression for the early-break.)
	events := []calEvent{
		{ID: 1, Title: "A", Start: mustTime(t, "2026-06-20T10:00:00"), End: mustTime(t, "2026-06-20T11:00:00"), HasStart: true, LocationID: 1},
		{ID: 2, Title: "B", Start: mustTime(t, "2026-06-20T11:05:00"), End: mustTime(t, "2026-06-20T11:10:00"), HasStart: true, LocationID: 1},
		{ID: 3, Title: "C", Start: mustTime(t, "2026-06-20T11:20:00"), End: mustTime(t, "2026-06-20T12:00:00"), HasStart: true, LocationID: 2},
	}
	got := detectConflicts(events, 60*time.Minute)
	foundAC := false
	for _, p := range got {
		if p.Kind == "tight-gap" && p.A.Title == "A" && p.B.Title == "C" {
			foundAC = true
		}
	}
	if !foundAC {
		t.Fatalf("expected a tight-gap between A and C across locations, got %+v", got)
	}
}

func TestBuildDeadlines(t *testing.T) {
	now := time.Date(2026, 6, 18, 0, 0, 0, 0, time.UTC)
	data := json.RawMessage(`[
		{"name":"Fall Rec","registrationEndDate":"2026-07-01"},
		{"name":"Spring Travel","registrationEndDate":"2026-06-20"},
		{"name":"Open Always"}
	]`)
	views, err := buildDeadlines(data, now, 14)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(views) != 3 {
		t.Fatalf("expected 3 views, got %d", len(views))
	}
	// Dated, sorted ascending by days-until-close; undated last.
	if views[0].Name != "Spring Travel" || views[0].DaysUntilClose != 2 || !views[0].ClosingSoon {
		t.Errorf("views[0] = %+v", views[0])
	}
	if views[1].Name != "Fall Rec" || views[1].DaysUntilClose != 13 || !views[1].ClosingSoon {
		t.Errorf("views[1] = %+v", views[1])
	}
	if views[2].Name != "Open Always" || views[2].ClosesAt != "" || views[2].ClosingSoon {
		t.Errorf("views[2] = %+v", views[2])
	}
}

func TestBuildDeadlinesUninterpretable(t *testing.T) {
	// A non-empty body that is neither an array nor an object must error rather
	// than silently report "No open programs" (parity with buildOwedReport).
	if _, err := buildDeadlines(json.RawMessage(`"unexpected"`), time.Now(), 14); err == nil {
		t.Fatalf("expected error for uninterpretable open-programs response")
	}
	// Empty/array bodies stay non-error.
	if _, err := buildDeadlines(json.RawMessage(`[]`), time.Now(), 14); err != nil {
		t.Errorf("empty array should not error: %v", err)
	}
}

func TestIcalUIDZeroIDDistinct(t *testing.T) {
	// Distinct zero-ID events that share a start timestamp must get distinct
	// UIDs, or calendar apps drop one of the merged events.
	start := mustTime(t, "2026-06-20T18:00:00")
	a := calEvent{ID: 0, Start: start, StartRaw: "2026-06-20T18:00:00", Title: "U10 Practice", LocationID: 1}
	b := calEvent{ID: 0, Start: start, StartRaw: "2026-06-20T18:00:00", Title: "U12 Game", LocationID: 2}
	if icalUID(a) == icalUID(b) {
		t.Fatalf("distinct zero-ID events at the same start must not share a UID: %q", icalUID(a))
	}
	// Same event content -> stable UID (re-import updates, not duplicates).
	if icalUID(a) != icalUID(a) {
		t.Errorf("UID must be stable for identical content")
	}
	// Non-zero ID still keys by ID.
	if icalUID(calEvent{ID: 99}) != "sprocket-99@sprocketsports.com" {
		t.Errorf("non-zero ID UID changed")
	}
}

func TestBuildICSDistinctUIDsForZeroIDEvents(t *testing.T) {
	now := time.Date(2026, 6, 18, 0, 0, 0, 0, time.UTC)
	start := mustTime(t, "2026-06-20T18:00:00")
	events := []calEvent{
		{ID: 0, Start: start, StartRaw: "2026-06-20T18:00:00", HasStart: true, Title: "Kid A practice"},
		{ID: 0, Start: start, StartRaw: "2026-06-20T18:00:00", HasStart: true, Title: "Kid B game"},
	}
	ics := buildICS(events, now)
	if c := strings.Count(ics, "BEGIN:VEVENT"); c != 2 {
		t.Fatalf("expected 2 VEVENTs, got %d", c)
	}
	// Collect UID lines; they must differ.
	var uids []string
	for _, line := range strings.Split(ics, "\r\n") {
		if strings.HasPrefix(line, "UID:") {
			uids = append(uids, line)
		}
	}
	if len(uids) != 2 || uids[0] == uids[1] {
		t.Fatalf("expected 2 distinct UIDs, got %v", uids)
	}
}

func TestFoldICSLine(t *testing.T) {
	short := "SUMMARY:short line"
	if foldICSLine(short) != short {
		t.Errorf("short line should not fold")
	}
	long := "SUMMARY:" + strings.Repeat("x", 200)
	folded := foldICSLine(long)
	for i, seg := range strings.Split(folded, "\r\n") {
		// First segment <=75 octets; continuations start with a space and are
		// <=75 octets including that leading space.
		if len(seg) > 75 {
			t.Errorf("segment %d exceeds 75 octets: %d", i, len(seg))
		}
		if i > 0 && (len(seg) == 0 || seg[0] != ' ') {
			t.Errorf("continuation segment %d must start with a space: %q", i, seg)
		}
	}
	// Unfolding (strip CRLF+space) must recover the original.
	if unfolded := strings.ReplaceAll(folded, "\r\n ", ""); unfolded != long {
		t.Errorf("unfold mismatch:\n got %q\nwant %q", unfolded, long)
	}
}

func TestSnapKeyZeroIDNoCollision(t *testing.T) {
	a := calEvent{ID: 0, StartRaw: "2026-06-20T10:00:00", Title: "Practice A"}
	b := calEvent{ID: 0, StartRaw: "2026-06-20T12:00:00", Title: "Practice B"}
	if snapKey(a) == snapKey(b) {
		t.Errorf("distinct zero-ID events must not share a snapshot key: %q", snapKey(a))
	}
	if snapKey(calEvent{ID: 42}) != "42" {
		t.Errorf("non-zero ID should key by ID")
	}
}

func TestAsObjectsDeterministicWrapper(t *testing.T) {
	// Two array-valued fields: the priority key ("data") must win regardless of
	// Go's randomized map iteration order. Run repeatedly to surface flakiness.
	raw := json.RawMessage(`{"zzz":[{"a":1}],"data":[{"b":2},{"b":3}]}`)
	for i := 0; i < 20; i++ {
		objs, ok := asObjects(raw)
		if !ok || len(objs) != 2 {
			t.Fatalf("iter %d: expected the 2-element 'data' array, got ok=%v len=%d", i, ok, len(objs))
		}
	}
}

func TestAwayRowMarksCancelled(t *testing.T) {
	live := awayRow(calEvent{Title: "vs Rivals", Opponent: "Rivals", HasStart: false})
	if strings.Contains(live[4], "CANCELLED") {
		t.Errorf("live game should not be marked cancelled: %q", live[4])
	}
	off := awayRow(calEvent{Title: "vs Rivals", Opponent: "Rivals", Cancelled: true})
	if !strings.HasPrefix(off[4], "[CANCELLED] ") {
		t.Errorf("cancelled away game must be marked: %q", off[4])
	}
}

func TestClubKeyFromBaseURL(t *testing.T) {
	cases := map[string]string{
		"https://jfcsoccer.sprocketsports.com":      "jfcsoccer-sprocketsports-com",
		"https://otherclub.sprocketsports.com/":     "otherclub-sprocketsports-com",
		"jfcsoccer.sprocketsports.com":              "jfcsoccer-sprocketsports-com",
		"":                                          "default",
	}
	for in, want := range cases {
		if got := clubKeyFromBaseURL(in); got != want {
			t.Errorf("clubKeyFromBaseURL(%q) = %q, want %q", in, got, want)
		}
	}
	// Distinct clubs must yield distinct snapshot paths.
	a, _ := sinceSnapshotPath(clubKeyFromBaseURL("https://jfcsoccer.sprocketsports.com"))
	b, _ := sinceSnapshotPath(clubKeyFromBaseURL("https://otherclub.sprocketsports.com"))
	if a == b {
		t.Errorf("distinct clubs must use distinct snapshot files: %q", a)
	}
}
