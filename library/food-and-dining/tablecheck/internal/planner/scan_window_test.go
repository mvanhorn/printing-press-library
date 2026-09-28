package planner_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/tablecheck/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/tablecheck/internal/planner"
)

func calendarWindow(t *testing.T, days map[string]any, closed ...string) []byte {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"availability_calendar": map[string]any{
		"type": "success", "time_zone": "Asia/Tokyo", "data": days, "closed_dates": closed,
	}})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func calendarSlot(date string, available bool) map[string]any {
	// 08:30Z is 17:30 in the venue's verified Asia/Tokyo timezone.
	return map[string]any{date + "T08:30:00Z": map[string]any{"is_available": available}}
}

func windowTransport(t *testing.T, answer func(string) (int, []byte)) *fixtureTransport {
	t.Helper()
	f := defaultTransport(t, "menu-items.json", "calendar-party2.json")
	ordinary := f.respond
	f.respond = func(r *http.Request, request recordedRequest, n int) (int, []byte) {
		if request.Path != "/v2/hub/availability_calendar_v2" {
			return ordinary(r, request, n)
		}
		stamp, _ := request.Body["start_at"].(string)
		if len(stamp) < len("2006-01-02") {
			t.Errorf("calendar request has no UTC start_at: %v", request.Body)
			return 400, []byte(`{"error":"missing anchor"}`)
		}
		return answer(stamp[:len("2006-01-02")])
	}
	return f
}

func scanRows(t *testing.T, result planner.Result) []map[string]any {
	t.Helper()
	raw := array(t, result["checks"])
	rows := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		rows = append(rows, object(t, item))
	}
	return rows
}

func calendarAnchors(t *testing.T, f *fixtureTransport) []string {
	t.Helper()
	var anchors []string
	for _, request := range f.entries() {
		if request.Path != "/v2/hub/availability_calendar_v2" {
			continue
		}
		stamp, ok := request.Body["start_at"].(string)
		if !ok {
			t.Fatalf("calendar request lost start_at: %v", request.Body)
		}
		anchors = append(anchors, stamp)
	}
	return anchors
}

func TestScanReusesOnlyExplicitCalendarDatesAndClosedDates(t *testing.T) {
	first := calendarWindow(t, map[string]any{
		"2026-09-30": calendarSlot("2026-09-30", true),
		"2026-10-01": map[string]any{}, // Explicitly covered, with no proven inventory.
	}, "2026-10-02")
	last := calendarWindow(t, map[string]any{"2026-10-03": calendarSlot("2026-10-03", false)})
	f := windowTransport(t, func(date string) (int, []byte) {
		if date == "2026-09-30" {
			return 200, first
		}
		if date == "2026-10-03" {
			return 200, last
		}
		t.Errorf("scan fetched already-covered date %s", date)
		return 500, []byte(`{"error":"duplicate fetch"}`)
	})
	clock := 0
	c := newFixtureClient(t, f, func(o *planner.Options) {
		o.Now = func() time.Time {
			clock++
			return time.Date(2026, 9, 27, 14, 0, 0, clock*int(time.Millisecond), time.UTC)
		}
	})
	result, err := c.Scan(context.Background(), planner.ScanOptions{
		Venues: []string{fixtureVenue}, From: "2026-09-30", To: "2026-10-03", Party: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	rows := scanRows(t, result)
	want := []string{"available", "unknown", "closed", "unavailable"}
	if len(rows) != len(want) {
		t.Fatalf("scan rows=%d, want %d", len(rows), len(want))
	}
	for i, status := range want {
		if rows[i]["status"] != status {
			t.Errorf("date %v status=%v, want %s", rows[i]["date"], rows[i]["status"], status)
		}
	}
	if coverage := object(t, rows[1]["coverage"]); coverage["date_present"] != true || coverage["timeslots"] != float64(0) {
		t.Fatalf("empty explicit date was not treated as covered unknown: %v", coverage)
	}
	if coverage := object(t, rows[2]["coverage"]); coverage["closed"] != true {
		t.Fatalf("closed date lost its machine evidence: %v", coverage)
	}
	firstFetched := object(t, rows[0]["freshness"])["fetched_at"]
	if object(t, rows[1]["freshness"])["fetched_at"] != firstFetched || object(t, rows[2]["freshness"])["fetched_at"] != firstFetched {
		t.Fatal("reused dates did not retain their first response's observation time")
	}
	if object(t, rows[3]["freshness"])["fetched_at"] == firstFetched {
		t.Fatal("new calendar window borrowed the earlier response's observation time")
	}
	anchors := calendarAnchors(t, f)
	if len(anchors) != 2 || anchors[0] != "2026-09-30T09:00:00Z" || anchors[1] != "2026-10-03T09:00:00Z" || metaRequests(t, result) != 3 {
		t.Fatalf("calendar reuse or request budget wrong: anchors=%v meta=%v", anchors, result["meta"])
	}
}

func TestScanEmptyWindowsAdvanceOncePerUncoveredDateWithoutInventingInventory(t *testing.T) {
	empty := calendarWindow(t, map[string]any{})
	f := windowTransport(t, func(string) (int, []byte) { return 200, empty })
	result, err := newFixtureClient(t, f, nil).Scan(context.Background(), planner.ScanOptions{
		Venues: []string{fixtureVenue}, From: "2026-09-30", To: "2026-10-02", Party: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	rows := scanRows(t, result)
	if len(rows) != 3 {
		t.Fatalf("scan dropped missing dates: %v", result)
	}
	for _, row := range rows {
		if row["status"] != "unknown" || row["error"] != nil {
			t.Fatalf("missing date became sold-out/failed inventory: %v", row)
		}
		if coverage := object(t, row["coverage"]); coverage["date_present"] != false || coverage["full_day"] != false {
			t.Fatalf("empty source incorrectly claimed date/full-day coverage: %v", coverage)
		}
	}
	anchors := calendarAnchors(t, f)
	if len(anchors) != 3 || anchors[0] != "2026-09-30T09:00:00Z" || anchors[1] != "2026-10-01T09:00:00Z" || anchors[2] != "2026-10-02T09:00:00Z" || metaRequests(t, result) != 4 {
		t.Fatalf("no-progress source was refetched or stopped early: anchors=%v meta=%v", anchors, result["meta"])
	}
}

func TestScanBudgetExhaustionFailsOnlyUncoveredRemainingDates(t *testing.T) {
	f := windowTransport(t, func(date string) (int, []byte) {
		return 200, calendarWindow(t, map[string]any{date: calendarSlot(date, true)})
	})
	c := newFixtureClient(t, f, func(o *planner.Options) { o.MaxRequests = 3 })
	result, err := c.Scan(context.Background(), planner.ScanOptions{
		Venues: []string{fixtureVenue}, From: "2026-09-30", To: "2026-10-03", Party: 2,
	})
	var partial *planner.PartialError
	if !errors.As(err, &partial) || partial.Failed != 1 {
		t.Fatalf("budget exhaustion must be a partial failure: %T %v", err, err)
	}
	rows := scanRows(t, result)
	if len(rows) != 4 || rows[0]["status"] != "available" || rows[1]["status"] != "available" {
		t.Fatalf("budget failure lost successful rows: %v", rows)
	}
	for _, row := range rows[2:] {
		if row["status"] != "failed" || !strings.Contains(row["error"].(string), "request budget exhausted") || row["coverage"] != nil {
			t.Fatalf("unattempted date was presented as unknown inventory: %v", row)
		}
	}
	if len(calendarAnchors(t, f)) != 2 || metaRequests(t, result) != 3 {
		t.Fatalf("HTTP attempts exceeded shared budget: calls=%v meta=%v", f.entries(), result["meta"])
	}
}

func TestScanLaterWindowFailureKeepsEarlierAndLaterSuccesses(t *testing.T) {
	f := windowTransport(t, func(date string) (int, []byte) {
		if date == "2026-10-01" {
			return 503, []byte(`{"error":"later source window unavailable"}`)
		}
		return 200, calendarWindow(t, map[string]any{date: calendarSlot(date, true)})
	})
	clock := 0
	c := newFixtureClient(t, f, func(o *planner.Options) {
		o.Now = func() time.Time {
			clock++
			return time.Date(2026, 9, 27, 14, 0, 0, clock*int(time.Millisecond), time.UTC)
		}
	})
	result, err := c.Scan(context.Background(), planner.ScanOptions{
		Venues: []string{fixtureVenue}, From: "2026-09-30", To: "2026-10-02", Party: 2,
	})
	var partial *planner.PartialError
	if !errors.As(err, &partial) {
		t.Fatalf("later window failure must be visible alongside success: %T %v", err, err)
	}
	rows := scanRows(t, result)
	if len(rows) != 3 || rows[0]["status"] != "available" || rows[1]["status"] != "failed" || rows[2]["status"] != "available" {
		t.Fatalf("later source failure clobbered successful inventory: %v", rows)
	}
	if rows[1]["coverage"] != nil || rows[1]["error"] == nil || object(t, rows[0]["freshness"])["fetched_at"] == object(t, rows[2]["freshness"])["fetched_at"] {
		t.Fatalf("partial rows lost failure/freshness evidence: %v", rows)
	}
	failures := array(t, result["fetch_failures"])
	if len(failures) != 1 || object(t, failures[0])["date"] != "2026-10-01" || metaRequests(t, result) != 4 {
		t.Fatalf("failed date or actual request count omitted: failures=%v meta=%v", failures, result["meta"])
	}
}

func TestScanThrottleStopsFutureWindowsAndSurfacesTypedError(t *testing.T) {
	f := windowTransport(t, func(date string) (int, []byte) {
		if date == "2026-10-01" {
			return 429, []byte(`{"error":"synthetic throttle"}`)
		}
		return 200, calendarWindow(t, map[string]any{date: calendarSlot(date, true)})
	})
	result, err := newFixtureClient(t, f, nil).Scan(context.Background(), planner.ScanOptions{
		Venues: []string{fixtureVenue}, From: "2026-09-30", To: "2026-10-02", Party: 2,
	})
	var throttle *cliutil.RateLimitError
	if !errors.As(err, &throttle) {
		t.Fatalf("429 must remain typed through partial scan: %T %v", err, err)
	}
	rows := scanRows(t, result)
	if len(rows) != 3 || rows[0]["status"] != "available" || rows[1]["status"] != "failed" || rows[2]["status"] != "failed" {
		t.Fatalf("throttle silently dropped or invented inventory: %v", rows)
	}
	if len(calendarAnchors(t, f)) != 2 || metaRequests(t, result) != 3 {
		t.Fatalf("throttle did not cancel subsequent windows: calls=%v meta=%v", f.entries(), result["meta"])
	}
}

func TestScanLaterExplicitCoverageRecoversEarlierFailedDate(t *testing.T) {
	f := windowTransport(t, func(date string) (int, []byte) {
		if date == "2026-09-30" {
			return 503, []byte(`{"error":"first window failed"}`)
		}
		return 200, calendarWindow(t, map[string]any{
			"2026-09-30": calendarSlot("2026-09-30", true),
			"2026-10-01": calendarSlot("2026-10-01", true),
		})
	})
	result, err := newFixtureClient(t, f, nil).Scan(context.Background(), planner.ScanOptions{
		Venues: []string{fixtureVenue}, From: "2026-09-30", To: "2026-10-01", Party: 2,
	})
	if err != nil {
		t.Fatalf("recovered date still caused partial failure: %v", err)
	}
	rows := scanRows(t, result)
	if len(rows) != 2 {
		t.Fatalf("recovered scan dropped dates: %v", rows)
	}
	for _, row := range rows {
		if row["status"] != "available" || row["error"] != nil || row["source_status"] != "success" {
			t.Fatalf("later explicit availability did not recover date: %v", row)
		}
		if object(t, row["freshness"])["fetched_at"] == nil {
			t.Fatalf("recovered row lost supplier observation: %v", row)
		}
	}
	if object(t, rows[0]["freshness"])["fetched_at"] != object(t, rows[1]["freshness"])["fetched_at"] {
		t.Fatalf("recovered date did not use second window's observation: %v", rows)
	}
	if failures := array(t, result["fetch_failures"]); len(failures) != 0 {
		t.Fatalf("recovered error survived in fetch_failures: %v", failures)
	}
	meta := object(t, result["meta"])
	if meta["partial_failure"] != false || meta["failed_venues"] != float64(0) || metaRequests(t, result) != 3 {
		t.Fatalf("recovered error survived in result metadata: %v", meta)
	}
}

func TestScanRecoveryKeepsOnlyUnrecoveredFailedDate(t *testing.T) {
	f := windowTransport(t, func(date string) (int, []byte) {
		switch date {
		case "2026-09-30", "2026-10-02":
			return 503, []byte(`{"error":"one window failed"}`)
		default:
			return 200, calendarWindow(t, map[string]any{
				"2026-09-30": calendarSlot("2026-09-30", true),
				"2026-10-01": calendarSlot("2026-10-01", true),
			})
		}
	})
	result, err := newFixtureClient(t, f, nil).Scan(context.Background(), planner.ScanOptions{
		Venues: []string{fixtureVenue}, From: "2026-09-30", To: "2026-10-02", Party: 2,
	})
	var partial *planner.PartialError
	if !errors.As(err, &partial) || partial.Failed != 1 {
		t.Fatalf("unrecovered date should cause one failed venue: %T %v", err, err)
	}
	rows := scanRows(t, result)
	if len(rows) != 3 || rows[0]["status"] != "available" || rows[1]["status"] != "available" || rows[2]["status"] != "failed" {
		t.Fatalf("recovered and unrecovered statuses mixed: %v", rows)
	}
	failures := array(t, result["fetch_failures"])
	if len(failures) != 1 || object(t, failures[0])["date"] != "2026-10-02" || metaRequests(t, result) != 4 {
		t.Fatalf("only unrecovered date should be reported: failures=%v meta=%v", failures, result["meta"])
	}
}

func TestScanLaterExplicitCoverageReplacesEarlierMissingAnchorUnknown(t *testing.T) {
	f := windowTransport(t, func(date string) (int, []byte) {
		if date == "2026-09-30" {
			return 200, calendarWindow(t, map[string]any{})
		}
		return 200, calendarWindow(t, map[string]any{
			"2026-09-30": calendarSlot("2026-09-30", true),
			"2026-10-01": calendarSlot("2026-10-01", true),
		})
	})
	result, err := newFixtureClient(t, f, nil).Scan(context.Background(), planner.ScanOptions{
		Venues: []string{fixtureVenue}, From: "2026-09-30", To: "2026-10-01", Party: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	rows := scanRows(t, result)
	if len(rows) != 2 || rows[0]["status"] != "available" || rows[1]["status"] != "available" {
		t.Fatalf("later evidence did not replace missing-anchor unknown: %v", rows)
	}
	if first := object(t, rows[0]["coverage"]); first["date_present"] != true {
		t.Fatalf("recovered date still points at first empty response: %v", first)
	}
	if len(array(t, result["fetch_failures"])) != 0 || metaRequests(t, result) != 3 {
		t.Fatalf("later coverage changed request/error count: %v", result)
	}
}

func TestScanFirstExplicitCoverageWinsOverLaterConflictingWindow(t *testing.T) {
	f := windowTransport(t, func(date string) (int, []byte) {
		if date == "2026-09-30" {
			return 200, calendarWindow(t, map[string]any{"2026-09-30": calendarSlot("2026-09-30", false)})
		}
		return 200, calendarWindow(t, map[string]any{
			"2026-09-30": calendarSlot("2026-09-30", true),
			"2026-10-01": calendarSlot("2026-10-01", true),
		})
	})
	result, err := newFixtureClient(t, f, nil).Scan(context.Background(), planner.ScanOptions{
		Venues: []string{fixtureVenue}, From: "2026-09-30", To: "2026-10-01", Party: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	rows := scanRows(t, result)
	if len(rows) != 2 || rows[0]["status"] != "unavailable" || rows[1]["status"] != "available" {
		t.Fatalf("later conflicting evidence overwrote the first explicit observation: %v", rows)
	}
	if len(calendarAnchors(t, f)) != 2 {
		t.Fatalf("window overlap triggered an extra fetch: %v", f.entries())
	}
}
