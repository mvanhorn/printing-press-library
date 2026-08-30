// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.

// Hand-authored shared helpers for the schedule-based novel commands
// (week, next, agenda, conflicts, away, ical, since). The Sprocket calendar
// is a read-only POST to /api/public/calendar with a {start,end} body capped
// at ~31 days, returning {responses, events}. These helpers fetch and flatten
// the deeply-nested event shape into a calEvent for date math, filtering, and
// rendering. Pure functions here are unit-tested in sprocket_schedule_test.go.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/sprocket/internal/client"
)

// calMaxWindowDays is the largest date window the calendar endpoint accepts in
// a single request. Observed empirically: 30 days succeeds, 60 returns
// "Start date is out of valid range." Stay under 31 to be safe.
const calMaxWindowDays = 30

// calEvent is a flattened Sprocket calendar event. Raw preserves the full
// events[] element so --json/--select can reach any nested field.
type calEvent struct {
	Raw         json.RawMessage `json:"-"`
	ID          int             `json:"id"`
	Title       string          `json:"title"`
	Start       time.Time       `json:"-"`
	End         time.Time       `json:"-"`
	StartRaw    string          `json:"start"`
	EndRaw      string          `json:"end,omitempty"`
	HasStart    bool            `json:"-"`
	EventTypeID int             `json:"eventTypeId,omitempty"`
	TeamID      int             `json:"teamId,omitempty"`
	Opponent    string          `json:"opponent,omitempty"`
	AwayGame    bool            `json:"awayGame"`
	LocationID  int             `json:"locationId,omitempty"`
	Location    string          `json:"location,omitempty"`
	Cancelled   bool            `json:"cancelled"`
}

// rawCalElement mirrors one element of the calendar response events[] array.
type rawCalElement struct {
	ID                int          `json:"id"`
	ClubCalendarEvent rawClubEvent `json:"clubCalendarEvent"`
}

type rawClubEvent struct {
	Title           string `json:"title"`
	StartDate       string `json:"startDate"`
	EndDate         string `json:"endDate"`
	Opponent        string `json:"opponent"`
	AwayGame        bool   `json:"awayGame"`
	ClubEventTypeID int    `json:"clubEventTypeID"`
	TeamID          *int   `json:"teamID"`
	LocationID      *int   `json:"locationID"`
	IsCancelled     bool   `json:"isCancelled"`
	// Best-effort location name fields; present on some clubs/events.
	LocationName string `json:"locationName"`
	Location     string `json:"location"`
	FacilityName string `json:"facilityName"`
}

type calResponse struct {
	Events []json.RawMessage `json:"events"`
}

// parseSprocketTime parses the calendar's date/datetime strings. The API emits
// ISO-8601 datetimes (sometimes without a zone or with fractional seconds) and
// occasionally plain dates. Zoned strings (with Z or an offset) are parsed to
// their true instant; zone-less strings are interpreted in the local timezone
// — the club's events are local wall-clock, and the user runs the CLI in (or
// near) the club's region, so this keeps "next"/"week" comparisons against a
// local now correct rather than UTC-skewed. Returns ok=false for an
// empty/unparseable string.
func parseSprocketTime(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	// Zoned layouts carry their own offset; parse to the exact instant.
	for _, l := range []string{time.RFC3339Nano, time.RFC3339} {
		if t, err := time.Parse(l, s); err == nil {
			return t, true
		}
	}
	// Zone-less layouts: interpret as local wall-clock.
	for _, l := range []string{
		"2006-01-02T15:04:05.999999999",
		"2006-01-02T15:04:05",
		"2006-01-02T15:04",
		"2006-01-02 15:04:05",
		"2006-01-02",
	} {
		if t, err := time.ParseInLocation(l, s, time.Local); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// truncateToDay returns midnight of t's calendar date in t's own location.
// Unlike time.Truncate(24h) — which rounds to a UTC boundary and can shift a
// local date — this stays on the intended calendar day.
func truncateToDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// chunkRange splits [start, end] into windows no wider than maxDays so each
// fits the calendar endpoint's single-request limit. Windows are inclusive of
// start and exclusive of the next chunk's start; the final chunk ends at end.
func chunkRange(start, end time.Time, maxDays int) [][2]time.Time {
	if maxDays < 1 {
		maxDays = 1
	}
	start = truncateToDay(start)
	end = truncateToDay(end)
	if end.Before(start) {
		return nil
	}
	var out [][2]time.Time
	cur := start
	for !cur.After(end) {
		win := cur.AddDate(0, 0, maxDays-1)
		if win.After(end) {
			win = end
		}
		out = append(out, [2]time.Time{cur, win})
		cur = win.AddDate(0, 0, 1)
	}
	return out
}

// weekBounds returns the Monday 00:00 and the following Sunday 23:59:59 that
// bracket the week containing now (local time).
func weekBounds(now time.Time) (time.Time, time.Time) {
	y, m, d := now.Date()
	day := time.Date(y, m, d, 0, 0, 0, 0, now.Location())
	// Go: Sunday=0..Saturday=6. Shift to Monday-based offset.
	offset := (int(day.Weekday()) + 6) % 7
	monday := day.AddDate(0, 0, -offset)
	sunday := monday.AddDate(0, 0, 6)
	endOfSunday := time.Date(sunday.Year(), sunday.Month(), sunday.Day(), 23, 59, 59, 0, now.Location())
	return monday, endOfSunday
}

// flattenCalElement converts one raw events[] element into a calEvent.
func flattenCalElement(raw json.RawMessage) calEvent {
	var el rawCalElement
	_ = json.Unmarshal(raw, &el)
	ev := el.ClubCalendarEvent
	out := calEvent{
		Raw:         raw,
		ID:          el.ID,
		Title:       ev.Title,
		StartRaw:    ev.StartDate,
		EndRaw:      ev.EndDate,
		EventTypeID: ev.ClubEventTypeID,
		Opponent:    ev.Opponent,
		AwayGame:    ev.AwayGame,
		Cancelled:   ev.IsCancelled,
	}
	if ev.TeamID != nil {
		out.TeamID = *ev.TeamID
	}
	if ev.LocationID != nil {
		out.LocationID = *ev.LocationID
	}
	// Best-effort location label from whichever name field the club populates.
	for _, c := range []string{ev.LocationName, ev.Location, ev.FacilityName} {
		if c != "" {
			out.Location = c
			break
		}
	}
	if t, ok := parseSprocketTime(ev.StartDate); ok {
		out.Start = t.Local()
		out.HasStart = true
	}
	if t, ok := parseSprocketTime(ev.EndDate); ok {
		out.End = t.Local()
	}
	return out
}

// isEmptyCalEvent reports whether a flattened element carries no usable signal
// (e.g. a malformed events[] element that failed to unmarshal). Such phantoms
// are skipped rather than rendered as blank "TBD" rows.
func isEmptyCalEvent(e calEvent) bool {
	return e.ID == 0 && e.Title == "" && !e.HasStart
}

// clubWideEventTypeIDs are the event types the dashboard requests for the
// club-wide (non-team) calendar layer: games, scrimmages, and general club
// events. Team-scoped requests pass no type filter so they include practices.
var clubWideEventTypeIDs = []int{1, 5, 6}

// fetchMyTeamIDs returns the distinct team IDs the authenticated user's players
// are assigned to. The calendar endpoint returns team events (training, games)
// only when scoped by teamID, so these drive the merged schedule. An empty
// result is not an error — the user may have no team assignments yet.
func fetchMyTeamIDs(ctx context.Context, c *client.Client) ([]int, error) {
	data, err := c.Get(ctx, "/api/club-users/player-teams", nil)
	if err != nil {
		return nil, fmt.Errorf("fetching your teams: %w", err)
	}
	var rows []map[string]any
	if err := json.Unmarshal(data, &rows); err != nil {
		// Tolerate an unexpected shape: fall back to club-wide events only.
		return nil, nil
	}
	seen := map[int]bool{}
	var ids []int
	for _, r := range rows {
		if f, ok := r["teamID"].(float64); ok && f > 0 {
			id := int(f)
			if !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
	}
	return ids, nil
}

// calISO formats a time as the RFC3339 string the calendar endpoint expects.
func calISO(t time.Time) string { return t.Format(time.RFC3339) }

// fetchCalendar fetches every event in [start, end] across all the user's
// players' teams PLUS club-wide events, chunked to respect the endpoint window
// limit, returning flattened events sorted by start time and de-duplicated by
// ID. The endpoint only returns team events (training/games) when scoped by
// teamID, so this looks up the user's teams first and merges a team-scoped
// query with a club-wide query — reproducing (and merging across teams) what
// the per-team web dashboard shows one team at a time.
func fetchCalendar(ctx context.Context, c *client.Client, start, end time.Time) ([]calEvent, error) {
	teamIDs, err := fetchMyTeamIDs(ctx, c)
	if err != nil {
		return nil, err
	}
	seen := map[int]bool{}
	var events []calEvent
	collect := func(body map[string]any) error {
		data, _, err := c.PostQueryWithParams(ctx, "/api/public/calendar", nil, body)
		if err != nil {
			return err
		}
		var resp calResponse
		if err := json.Unmarshal(data, &resp); err != nil {
			return fmt.Errorf("parsing calendar response: %w", err)
		}
		for _, raw := range resp.Events {
			ev := flattenCalElement(raw)
			if isEmptyCalEvent(ev) {
				continue
			}
			if ev.ID != 0 {
				if seen[ev.ID] {
					continue
				}
				seen[ev.ID] = true
			}
			events = append(events, ev)
		}
		return nil
	}
	for _, win := range chunkRange(start, end, calMaxWindowDays) {
		s := calISO(truncateToDay(win[0]))
		e := calISO(truncateToDay(win[1]).Add(24*time.Hour - time.Second)) // through end-of-day
		base := func() map[string]any {
			return map[string]any{"start": s, "end": e, "programID": []int{}, "excludePrograms": false, "includeUnpublishedEvents": false}
		}
		// Team-scoped layer: all event types (incl. practices) for the user's teams.
		if len(teamIDs) > 0 {
			b := base()
			b["teamID"] = teamIDs
			if err := collect(b); err != nil {
				return nil, fmt.Errorf("fetching team calendar %s..%s: %w", s, e, err)
			}
		}
		// Club-wide layer: games/scrimmages/club events not tied to a team.
		b := base()
		b["clubEventTypeID"] = clubWideEventTypeIDs
		b["teamID"] = nil
		b["allTeams"] = false
		b["allPrograms"] = false
		if err := collect(b); err != nil {
			return nil, fmt.Errorf("fetching club calendar %s..%s: %w", s, e, err)
		}
	}
	sortEventsByStart(events)
	return events, nil
}

// sortEventsByStart orders events chronologically; undated events sort last.
func sortEventsByStart(events []calEvent) {
	sort.SliceStable(events, func(i, j int) bool {
		if events[i].HasStart != events[j].HasStart {
			return events[i].HasStart
		}
		return events[i].Start.Before(events[j].Start)
	})
}

// eventTypeName maps a clubEventTypeID to a human label when known. The
// endpoint exposes the full lookup via `schedule event-types`; these are the
// common defaults so output is readable without a second call.
func eventTypeName(id int) string {
	switch id {
	case 1:
		return "Game"
	case 2:
		return "Practice"
	case 3:
		return "Event"
	case 4:
		return "Tournament"
	case 5:
		return "Scrimmage"
	default:
		return ""
	}
}

// homeAway renders a compact home/away marker for an event.
func homeAway(ev calEvent) string {
	if ev.AwayGame {
		return "away"
	}
	return "home"
}
