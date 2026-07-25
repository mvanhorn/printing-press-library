// Copyright 2026 Olivier and contributors. Licensed under Apache-2.0. See LICENSE.
// Behaviour tests for the hand-authored irail logic.
//
// These cover the pure functions behind the novel commands: date/time
// reasoning, observation diffing, transfer-risk maths, disruption matching and
// occupancy payload derivation.

package cli

import (
	"strconv"
	"testing"
	"time"
)

// reference instant: Friday 2026-07-24 14:30 Belgian time.
func refTime(t *testing.T) time.Time {
	t.Helper()
	loc := belgiumTZ()
	return time.Date(2026, 7, 24, 14, 30, 0, 0, loc)
}

func TestParseHumanDate(t *testing.T) {
	now := refTime(t)
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"empty means unset", "", ""},
		{"today", "today", "240726"},
		{"tomorrow english", "tomorrow", "250726"},
		{"tomorrow dutch", "morgen", "250726"},
		{"tomorrow french", "demain", "250726"},
		{"day after tomorrow dutch", "overmorgen", "260726"},
		{"iso date", "2026-07-30", "300726"},
		{"slashed date", "30/07/2026", "300726"},
		{"relative days", "+3d", "270726"},
		{"relative weeks", "+1w", "310726"},
		{"raw ddmmyy passthrough", "300726", "300726"},
		// 24 Jul 2026 is a Friday, so "monday" is the 27th.
		{"weekday english", "monday", "270726"},
		{"weekday dutch", "maandag", "270726"},
		{"weekday french", "lundi", "270726"},
		{"weekday abbrev", "mon", "270726"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseHumanDate(tc.in, now)
			if err != nil {
				t.Fatalf("parseHumanDate(%q) error = %v", tc.in, err)
			}
			if got != tc.want {
				t.Fatalf("parseHumanDate(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}

	if _, err := parseHumanDate("not-a-date", now); err == nil {
		t.Fatal("expected an error for an unparseable date")
	}
	// A malformed ddmmyy must be rejected rather than forwarded to the API,
	// which answers far-out dates with HTTP 500.
	if _, err := parseHumanDate("999999", now); err == nil {
		t.Fatal("expected an error for an invalid ddmmyy value")
	}
}

func TestParseHumanTime(t *testing.T) {
	now := refTime(t)
	cases := []struct{ in, want string }{
		{"", ""},
		{"08:12", "0812"},
		{"0812", "0812"},
		{"now", "1430"},
		{"+30m", "1500"},
		{"+2h", "1630"},
	}
	for _, tc := range cases {
		got, err := parseHumanTime(tc.in, now)
		if err != nil {
			t.Fatalf("parseHumanTime(%q) error = %v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("parseHumanTime(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
	if _, err := parseHumanTime("half past nine", now); err == nil {
		t.Fatal("expected an error for an unparseable time")
	}
}

// TestResolveWhenRollsPastTimeToTomorrow covers the bug clirail documents in
// its own README: a bare time that has already passed must not silently mean
// "earlier today".
func TestResolveWhenRollsPastTimeToTomorrow(t *testing.T) {
	now := refTime(t) // 14:30

	date, hhmm, rolled, err := resolveWhen("", "07:00", now)
	if err != nil {
		t.Fatalf("resolveWhen error = %v", err)
	}
	if !rolled {
		t.Fatal("07:00 requested at 14:30 should roll to tomorrow")
	}
	if date != "250726" {
		t.Fatalf("rolled date = %q, want 250726 (tomorrow)", date)
	}
	if hhmm != "0700" {
		t.Fatalf("time = %q, want 0700", hhmm)
	}

	// A later time today must NOT roll.
	date, _, rolled, err = resolveWhen("", "18:00", now)
	if err != nil {
		t.Fatalf("resolveWhen error = %v", err)
	}
	if rolled {
		t.Fatal("18:00 requested at 14:30 must stay today")
	}
	if date != "" {
		t.Fatalf("date = %q, want empty so the API uses today", date)
	}

	// An explicit date must always win over the roll heuristic.
	date, _, rolled, err = resolveWhen("2026-07-30", "07:00", now)
	if err != nil {
		t.Fatalf("resolveWhen error = %v", err)
	}
	if rolled {
		t.Fatal("an explicit date must not be rolled")
	}
	if date != "300726" {
		t.Fatalf("date = %q, want 300726", date)
	}
}

func TestIrailScalarCoercion(t *testing.T) {
	// iRail sends every scalar as a JSON string.
	if got := irailInt("120"); got != 120 {
		t.Fatalf("irailInt(\"120\") = %d, want 120", got)
	}
	if got := irailInt("-60"); got != -60 {
		t.Fatalf("irailInt(\"-60\") = %d, want -60 (trains can run early)", got)
	}
	if got := irailInt(""); got != 0 {
		t.Fatalf("irailInt(\"\") = %d, want 0", got)
	}
	if !irailBool("1") {
		t.Fatal("irailBool(\"1\") should be true")
	}
	if irailBool("0") {
		t.Fatal("irailBool(\"0\") should be false")
	}
	if irailBool("") {
		t.Fatal("irailBool(\"\") should be false")
	}
	if got := irailInt64("1784922960"); got != 1784922960 {
		t.Fatalf("irailInt64 = %d", got)
	}
}

// TestSliceAtHandlesCollapsedArray guards the iRail habit of returning a bare
// object where a single-element array is expected.
func TestSliceAtHandlesCollapsedArray(t *testing.T) {
	asArray := map[string]any{
		"departures": map[string]any{
			"departure": []any{map[string]any{"vehicle": "IC1"}, map[string]any{"vehicle": "IC2"}},
		},
	}
	if got := sliceAt(asArray, "departures", "departure"); len(got) != 2 {
		t.Fatalf("array form returned %d rows, want 2", len(got))
	}

	collapsed := map[string]any{
		"departures": map[string]any{
			"departure": map[string]any{"vehicle": "IC1"},
		},
	}
	if got := sliceAt(collapsed, "departures", "departure"); len(got) != 1 {
		t.Fatalf("collapsed single object returned %d rows, want 1", len(got))
	}

	if got := sliceAt(map[string]any{}, "missing", "key"); len(got) != 0 {
		t.Fatalf("missing path returned %d rows, want 0", len(got))
	}
}

func TestDiffObservationsDetectsRealChanges(t *testing.T) {
	const sched = int64(1784922960)
	key := "BE.NMBS.IC1|" + strconv.FormatInt(sched, 10)

	base := observationSnapshot{
		vehicle: "BE.NMBS.IC1", vehicleShort: "IC 1", direction: "Ghent",
		scheduledAt: sched, delay: 0, canceled: false, platform: "4", normal: true,
	}

	t.Run("no change yields nothing", func(t *testing.T) {
		got := diffObservations(map[string]observationSnapshot{key: base}, map[string]observationSnapshot{key: base})
		if len(got) != 0 {
			t.Fatalf("unchanged board produced %d change(s): %+v", len(got), got)
		}
	})

	t.Run("delay increase", func(t *testing.T) {
		later := base
		later.delay = 300
		got := diffObservations(map[string]observationSnapshot{key: base}, map[string]observationSnapshot{key: later})
		if !hasKind(got, "delay-increased") {
			t.Fatalf("expected delay-increased, got %+v", got)
		}
	})

	t.Run("delay decrease is not reported as an increase", func(t *testing.T) {
		before := base
		before.delay = 300
		got := diffObservations(map[string]observationSnapshot{key: before}, map[string]observationSnapshot{key: base})
		if hasKind(got, "delay-increased") {
			t.Fatalf("recovering delay must not report an increase: %+v", got)
		}
	})

	t.Run("cancellation", func(t *testing.T) {
		later := base
		later.canceled = true
		got := diffObservations(map[string]observationSnapshot{key: base}, map[string]observationSnapshot{key: later})
		if !hasKind(got, "canceled") {
			t.Fatalf("expected canceled, got %+v", got)
		}
	})

	t.Run("platform change", func(t *testing.T) {
		later := base
		later.platform = "9"
		got := diffObservations(map[string]observationSnapshot{key: base}, map[string]observationSnapshot{key: later})
		if !hasKind(got, "platform-changed") {
			t.Fatalf("expected platform-changed, got %+v", got)
		}
	})

	t.Run("platform flagged not normal", func(t *testing.T) {
		later := base
		later.normal = false
		got := diffObservations(map[string]observationSnapshot{key: base}, map[string]observationSnapshot{key: later})
		if !hasKind(got, "platform-not-normal") {
			t.Fatalf("expected platform-not-normal, got %+v", got)
		}
	})

	t.Run("new departure", func(t *testing.T) {
		got := diffObservations(map[string]observationSnapshot{}, map[string]observationSnapshot{key: base})
		if !hasKind(got, "new-departure") {
			t.Fatalf("expected new-departure, got %+v", got)
		}
	})
}

func TestAnalyseViaVerdicts(t *testing.T) {
	// Leuven publishes a 300s official minimum transfer time.
	build := func(gapSec, arrivalDelay, departDelay int) map[string]any {
		const arrive = int64(1784922960)
		return map[string]any{
			"station": "Leuven",
			"arrival": map[string]any{
				"time":  strconv.FormatInt(arrive, 10),
				"delay": strconv.Itoa(arrivalDelay),
			},
			"departure": map[string]any{
				"time":  strconv.FormatInt(arrive+int64(gapSec), 10),
				"delay": strconv.Itoa(departDelay),
			},
		}
	}

	cases := []struct {
		name   string
		gap    int
		arrDly int
		depDly int
		want   string
	}{
		{"comfortable gap is ok", 1200, 0, 0, transferOK},
		{"gap just above minimum is tight", 400, 0, 0, transferTight},
		{"gap below official minimum is broken", 200, 0, 0, transferBroken},
		{"inbound delay eats the gap", 1200, 1100, 0, transferBroken},
		{"outbound delay gives the gap back", 400, 300, 600, transferOK},
		{"connection already missed", 300, 900, 0, transferBroken},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			leg := analyseVia(build(tc.gap, tc.arrDly, tc.depDly), 1.5)
			if leg.Verdict != tc.want {
				t.Fatalf("verdict = %q, want %q (scheduled gap %ds, actual %ds, required %ds)",
					leg.Verdict, tc.want, leg.ScheduledGapSec, leg.ActualGapSec, leg.RequiredSec)
			}
			if leg.Explanation == "" {
				t.Fatal("every verdict must carry an explanation")
			}
		})
	}

	t.Run("unknown station reports unknown, never ok", func(t *testing.T) {
		via := build(1200, 0, 0)
		via["station"] = "Nowhere-Junction"
		leg := analyseVia(via, 1.5)
		if leg.Verdict != transferUnknown {
			t.Fatalf("verdict = %q, want %q", leg.Verdict, transferUnknown)
		}
		if leg.HasRequired {
			t.Fatal("an unknown station must not claim a known transfer time")
		}
	})

	t.Run("missing times do not fabricate a verdict", func(t *testing.T) {
		leg := analyseVia(map[string]any{"station": "Leuven"}, 1.5)
		if leg.Verdict != transferUnknown {
			t.Fatalf("verdict = %q, want %q", leg.Verdict, transferUnknown)
		}
	})
}

func TestWorseVerdictOrdering(t *testing.T) {
	if got := worseVerdict(transferOK, transferBroken); got != transferBroken {
		t.Fatalf("worseVerdict(ok, broken) = %q", got)
	}
	if got := worseVerdict(transferBroken, transferOK); got != transferBroken {
		t.Fatalf("worseVerdict(broken, ok) = %q", got)
	}
	if got := worseVerdict(transferTight, transferUnknown); got != transferTight {
		t.Fatalf("worseVerdict(tight, unknown) = %q", got)
	}
}

func TestContainsWordRespectsBoundaries(t *testing.T) {
	if containsWord("tussen halle en brussel", "hal") {
		t.Fatal(`"hal" must not match inside "halle"`)
	}
	if !containsWord("tussen halle en brussel", "halle") {
		t.Fatal(`"halle" should match "halle"`)
	}
	if !containsWord("aat - lessen: vertragingen", "aat") {
		t.Fatal(`"aat" should match at a word boundary`)
	}
	if containsWord("", "hal") {
		t.Fatal("empty haystack must not match")
	}
	if containsWord("halle", "") {
		t.Fatal("empty needle must not match")
	}
}

func TestMatchDisruptionToStations(t *testing.T) {
	d := map[string]any{
		"title":       "Lessen - Papegem: Vertragingen en afschaffingen",
		"description": "Tussen Aat en Lessen: treinverkeer over een spoor.",
	}
	got := matchDisruptionToStations(d, []string{"Lessen", "Gent-Sint-Pieters"})
	if len(got) != 1 || got[0] != "Lessen" {
		t.Fatalf("matched = %v, want [Lessen]", got)
	}

	// A disruption naming no route station must not match at all.
	none := matchDisruptionToStations(d, []string{"Oostende", "Brugge"})
	if len(none) != 0 {
		t.Fatalf("unrelated route matched %v, want none", none)
	}
}

func TestOccupancyPayloadFrom(t *testing.T) {
	p, err := occupancyPayloadFrom("http://irail.be/connections/8813003/20260724/IC2344", "high")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.From != "http://irail.be/stations/NMBS/008813003" {
		t.Fatalf("station URI = %q; the 7-digit id must be padded to 9", p.From)
	}
	if p.Vehicle != "http://irail.be/vehicle/IC2344" {
		t.Fatalf("vehicle URI = %q", p.Vehicle)
	}
	if p.Date != "20260724" {
		t.Fatalf("date = %q, want yyyymmdd as the feedback endpoint expects", p.Date)
	}
	if p.Occupancy != "http://api.irail.be/terms/high" {
		t.Fatalf("occupancy term = %q", p.Occupancy)
	}

	if _, err := occupancyPayloadFrom("nonsense", "low"); err == nil {
		t.Fatal("expected an error for a malformed connection URI")
	}
	if _, err := occupancyPayloadFrom("http://irail.be/connections/8813003/BAD/IC2344", "low"); err == nil {
		t.Fatal("expected an error for a malformed date segment")
	}
}

func TestLeaveByOptionViability(t *testing.T) {
	deadline := time.Date(2026, 7, 24, 9, 0, 0, 0, belgiumTZ())
	arriveAt := deadline.Add(-10 * time.Minute)

	conn := map[string]any{
		"duration": "1200",
		"departure": map[string]any{
			"time": strconv.FormatInt(arriveAt.Add(-20*time.Minute).Unix(), 10), "delay": "0",
			"platform": "3", "vehicle": "BE.NMBS.IC1", "canceled": "0",
		},
		"arrival": map[string]any{
			"time": strconv.FormatInt(arriveAt.Unix(), 10), "delay": "0", "canceled": "0",
		},
	}

	t.Run("comfortable arrival is viable", func(t *testing.T) {
		opt := leaveByOptionFrom(conn, deadline, 0)
		if !opt.Viable {
			t.Fatalf("expected viable, got %+v", opt)
		}
		if opt.SlackSec != 600 {
			t.Fatalf("slack = %d, want 600", opt.SlackSec)
		}
	})

	t.Run("margin can rule an option out", func(t *testing.T) {
		opt := leaveByOptionFrom(conn, deadline, 900) // need 15m, only 10m exists
		if opt.Viable {
			t.Fatal("option should fail the margin requirement")
		}
	})

	t.Run("delay pushes arrival past the deadline", func(t *testing.T) {
		late := map[string]any{
			"duration":  "1200",
			"departure": conn["departure"],
			"arrival": map[string]any{
				"time": strconv.FormatInt(arriveAt.Unix(), 10), "delay": "1800", "canceled": "0",
			},
		}
		opt := leaveByOptionFrom(late, deadline, 0)
		if opt.Viable {
			t.Fatal("a 30m delay must make this option non-viable")
		}
		if opt.SlackSec >= 0 {
			t.Fatalf("slack = %d, want negative", opt.SlackSec)
		}
	})

	t.Run("cancelled option is never viable", func(t *testing.T) {
		cancelled := map[string]any{
			"duration":  "1200",
			"departure": conn["departure"],
			"arrival": map[string]any{
				"time": strconv.FormatInt(arriveAt.Unix(), 10), "delay": "0", "canceled": "1",
			},
		}
		opt := leaveByOptionFrom(cancelled, deadline, 0)
		if opt.Viable {
			t.Fatal("a cancelled option must not be viable")
		}
		if opt.Reason != "cancelled" {
			t.Fatalf("reason = %q, want cancelled", opt.Reason)
		}
	})
}

func TestHumanDuration(t *testing.T) {
	cases := []struct {
		in   int
		want string
	}{
		{0, "0m"},
		{-30, "0m"},
		{60, "1m"},
		{300, "5m"},
		{3600, "1h00"},
		{3660, "1h01"},
		{9720, "2h42"},
	}
	for _, tc := range cases {
		if got := humanDuration(tc.in); got != tc.want {
			t.Fatalf("humanDuration(%d) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestResolveStationNameFallsBackToInput(t *testing.T) {
	if got := resolveStationName("FGSP"); got != "Gent-Sint-Pieters" {
		t.Fatalf("telegraph code resolved to %q", got)
	}
	// Unknown input passes through so a brand-new station still reaches the API.
	if got := resolveStationName("Totally-New-Halt"); got != "Totally-New-Halt" {
		t.Fatalf("unknown station was rewritten to %q", got)
	}
}

// --- small helpers so tests read clearly ---------------------------------

func hasKind(rows []changeRow, kind string) bool {
	for _, r := range rows {
		if r.Kind == kind {
			return true
		}
	}
	return false
}

// TestClockOfSurvivesMissingTimes guards a real panic: unixToLocal returns ""
// when iRail omits a time field, and the human render slices at a fixed offset.
func TestClockOfSurvivesMissingTimes(t *testing.T) {
	cases := []struct{ in, want string }{
		{"2026-07-24T22:25:41+02:00", "22:25"},
		{"", "--:--"},
		{"short", "--:--"},
		{"2026-07-24T", "--:--"},
	}
	for _, tc := range cases {
		if got := clockOf(tc.in); got != tc.want {
			t.Fatalf("clockOf(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
