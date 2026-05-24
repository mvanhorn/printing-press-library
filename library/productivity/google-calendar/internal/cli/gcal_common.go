// Hand-authored novel-feature support for google-calendar-pp-cli.
// Survives `printing-press generate --force` as a whole hand-authored file.
//
// The generated `sync` only mirrors settings and the calendar list; events and
// ACL rules are calendar-scoped dependents the profiler cannot auto-walk. The
// transcendence commands (free, conflicts, changes, load, rsvp-status, book,
// acl-audit) therefore share the loaders below, which fetch per-calendar live
// (caching into the typed `events`/`acl` tables) with a local-store fast path
// and a verify-env short-circuit.
package cli

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/google-calendar/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/google-calendar/internal/store"
)

// gcalAttendee is the subset of a Calendar event attendee the novel commands read.
type gcalAttendee struct {
	Email          string `json:"email,omitempty"`
	DisplayName    string `json:"displayName,omitempty"`
	ResponseStatus string `json:"responseStatus,omitempty"`
	Self           bool   `json:"self,omitempty"`
	Organizer      bool   `json:"organizer,omitempty"`
}

// gcalEvent is the parsed in-memory shape used by every novel command. Times
// are resolved from the start/end dateTime|date variants; AllDay is true when
// the event used date-only bounds.
type gcalEvent struct {
	ID           string
	CalendarID   string
	Summary      string
	Status       string
	Created      time.Time
	Updated      time.Time
	Start        time.Time
	End          time.Time
	AllDay       bool
	Transparency string
	Attendees    []gcalAttendee
}

type rawEventTime struct {
	DateTime string `json:"dateTime"`
	Date     string `json:"date"`
	TimeZone string `json:"timeZone"`
}

type rawEvent struct {
	ID           string         `json:"id"`
	Summary      string         `json:"summary"`
	Status       string         `json:"status"`
	Created      string         `json:"created"`
	Updated      string         `json:"updated"`
	Transparency string         `json:"transparency"`
	Start        rawEventTime   `json:"start"`
	End          rawEventTime   `json:"end"`
	Attendees    []gcalAttendee `json:"attendees"`
}

// parseEventBound resolves a start/end bound. Returns (t, allDay, ok). All-day
// bounds parse the date in the local zone at midnight.
func parseEventBound(rt rawEventTime) (time.Time, bool, bool) {
	if rt.DateTime != "" {
		if t, err := time.Parse(time.RFC3339, rt.DateTime); err == nil {
			return t, false, true
		}
	}
	if rt.Date != "" {
		if t, err := time.Parse("2006-01-02", rt.Date); err == nil {
			return t, true, true
		}
	}
	return time.Time{}, false, false
}

// parseStoredEvent converts a stored/raw event JSON blob into a gcalEvent.
// Returns ok=false only when the blob is not a JSON object with an id.
func parseStoredEvent(calendarID string, data json.RawMessage) (gcalEvent, bool) {
	var re rawEvent
	if err := json.Unmarshal(data, &re); err != nil || re.ID == "" {
		return gcalEvent{}, false
	}
	ev := gcalEvent{
		ID:           re.ID,
		CalendarID:   calendarID,
		Summary:      re.Summary,
		Status:       re.Status,
		Transparency: re.Transparency,
		Attendees:    re.Attendees,
	}
	// All-day is signaled by the start bound using a date (not dateTime).
	// Google sets both start.date and end.date for all-day events, but the
	// start is the authoritative signal.
	if t, allDay, ok := parseEventBound(re.Start); ok {
		ev.Start = t
		ev.AllDay = allDay
	}
	if t, _, ok := parseEventBound(re.End); ok {
		ev.End = t
	}
	if re.Created != "" {
		if t, err := time.Parse(time.RFC3339, re.Created); err == nil {
			ev.Created = t
		}
	}
	if re.Updated != "" {
		if t, err := time.Parse(time.RFC3339, re.Updated); err == nil {
			ev.Updated = t
		}
	}
	return ev, true
}

// eventQuery describes which events to load.
type eventQuery struct {
	calendars   []string
	timeMin     time.Time // zero == unbounded
	timeMax     time.Time // zero == unbounded
	updatedMin  time.Time // zero == none (used by `changes`)
	showDeleted bool
}

// resolveCalendars splits a --calendars CSV; empty input defaults to ["primary"].
func resolveCalendars(csv string) []string {
	csv = strings.TrimSpace(csv)
	if csv == "" {
		return []string{"primary"}
	}
	var out []string
	for _, c := range strings.Split(csv, ",") {
		if c = strings.TrimSpace(c); c != "" {
			out = append(out, c)
		}
	}
	if len(out) == 0 {
		return []string{"primary"}
	}
	return out
}

// useLocalEvents reports whether to read from the local store rather than the
// live API. Verify runs are always local+deterministic; --data-source local is
// the explicit opt-in.
func useLocalEvents(flags *rootFlags) bool {
	return cliutil.IsVerifyEnv() || (flags != nil && flags.dataSource == "local")
}

// gcalLoadEvents loads events matching q, live-per-calendar by default (caching
// into the events table) with a local-store path under verify or --data-source
// local. On live error with --data-source auto it falls back to local.
func gcalLoadEvents(cmd *cobra.Command, flags *rootFlags, q eventQuery) ([]gcalEvent, DataProvenance, error) {
	prov := DataProvenance{Source: "local", ResourceType: "events"}
	dbPath := defaultDBPath("google-calendar-pp-cli")
	db, err := store.OpenWithContext(cmd.Context(), dbPath)
	if err != nil {
		return nil, prov, fmt.Errorf("opening database: %w", err)
	}
	defer db.Close()

	if useLocalEvents(flags) {
		evs, err := loadEventsLocal(db, q)
		return evs, prov, err
	}

	c, err := flags.newClient()
	if err != nil {
		return nil, prov, err
	}
	var all []gcalEvent
	var liveErr error
	for _, cal := range q.calendars {
		params := map[string]string{"singleEvents": "true", "maxResults": "2500", "orderBy": "startTime"}
		if !q.timeMin.IsZero() {
			params["timeMin"] = q.timeMin.UTC().Format(time.RFC3339)
		}
		if !q.timeMax.IsZero() {
			params["timeMax"] = q.timeMax.UTC().Format(time.RFC3339)
		}
		if !q.updatedMin.IsZero() {
			params["updatedMin"] = q.updatedMin.UTC().Format(time.RFC3339)
			// orderBy=startTime is invalid alongside updatedMin without a window; drop it.
			delete(params, "orderBy")
		}
		if q.showDeleted {
			params["showDeleted"] = "true"
		}
		// Page through every result for this calendar. Without this loop the
		// Calendar API caps a single response at maxResults and silently drops
		// the rest, so a busy window would compute free/busy and conflicts
		// against a truncated set. The page cap is a runaway guard, not a
		// truncation: 200 pages * 2500 = 500k events is far beyond any real
		// window.
		pageToken := ""
		const maxPages = 200
		for page := 0; page < maxPages; page++ {
			if pageToken != "" {
				params["pageToken"] = pageToken
			}
			data, err := c.Get(cmd.Context(), "/calendars/"+url.PathEscape(cal)+"/events", params)
			if err != nil {
				liveErr = err
				break
			}
			items, next := extractEventPage(data)
			for _, it := range items {
				if ev, ok := parseStoredEvent(cal, it); ok {
					all = append(all, ev)
					cacheEvent(db, cal, ev.ID, it)
				}
			}
			if next == "" {
				break
			}
			pageToken = next
		}
		delete(params, "pageToken")
	}
	if liveErr != nil && len(all) == 0 {
		if flags.dataSource == "live" {
			return nil, prov, classifyAPIError(liveErr, flags)
		}
		// auto: fall back to whatever is cached locally.
		evs, lerr := loadEventsLocal(db, q)
		if lerr != nil {
			return nil, prov, classifyAPIError(liveErr, flags)
		}
		prov.Reason = "api_unreachable"
		return filterEvents(evs, q), prov, nil
	}
	if liveErr != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: one or more calendars failed to load (%v); results may be incomplete\n", liveErr)
	}
	prov.Source = "live"
	return filterEvents(all, q), prov, nil
}

// extractEventItems pulls the `items` array out of a Calendar list response,
// tolerating a bare array too.
func extractEventItems(data json.RawMessage) []json.RawMessage {
	var arr []json.RawMessage
	if json.Unmarshal(data, &arr) == nil && arr != nil {
		return arr
	}
	var obj struct {
		Items []json.RawMessage `json:"items"`
	}
	if json.Unmarshal(data, &obj) == nil {
		return obj.Items
	}
	return nil
}

// extractEventPage pulls the `items` array and the `nextPageToken` out of a
// Calendar list response. A bare array (no envelope) has no next token.
func extractEventPage(data json.RawMessage) ([]json.RawMessage, string) {
	var arr []json.RawMessage
	if json.Unmarshal(data, &arr) == nil && arr != nil {
		return arr, ""
	}
	var obj struct {
		Items         []json.RawMessage `json:"items"`
		NextPageToken string            `json:"nextPageToken"`
	}
	if json.Unmarshal(data, &obj) == nil {
		return obj.Items, obj.NextPageToken
	}
	return nil, ""
}

func cacheEvent(db *store.Store, calendarID, id string, data json.RawMessage) {
	if id == "" {
		return
	}
	_, _ = db.DB().Exec(`INSERT OR REPLACE INTO "events"("id","calendars_id","data") VALUES(?,?,?)`, id, calendarID, string(data))
}

func loadEventsLocal(db *store.Store, q eventQuery) ([]gcalEvent, error) {
	query := `SELECT "id","calendars_id","data" FROM "events"`
	var args []any
	if len(q.calendars) > 0 {
		ph := make([]string, len(q.calendars))
		for i, c := range q.calendars {
			ph[i] = "?"
			args = append(args, c)
		}
		query += ` WHERE "calendars_id" IN (` + strings.Join(ph, ",") + `)`
	}
	rows, err := db.DB().Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying events: %w", err)
	}
	defer rows.Close()
	var out []gcalEvent
	for rows.Next() {
		var id, calID, data string
		if err := rows.Scan(&id, &calID, &data); err != nil {
			continue
		}
		if ev, ok := parseStoredEvent(calID, json.RawMessage(data)); ok {
			out = append(out, ev)
		}
	}
	return filterEvents(out, q), rows.Err()
}

// filterEvents applies the window / updatedMin filters in memory so the local
// and live paths return identical sets.
func filterEvents(evs []gcalEvent, q eventQuery) []gcalEvent {
	var out []gcalEvent
	for _, ev := range evs {
		if !q.updatedMin.IsZero() {
			if ev.Updated.IsZero() || ev.Updated.Before(q.updatedMin) {
				continue
			}
		}
		if !q.timeMin.IsZero() || !q.timeMax.IsZero() {
			// Keep events that overlap [timeMin, timeMax). Events with no
			// resolved start (e.g. cancelled stubs) are kept only for
			// updatedMin queries, dropped for pure window queries.
			if ev.Start.IsZero() {
				continue
			}
			end := ev.End
			if end.IsZero() {
				end = ev.Start
			}
			if !q.timeMax.IsZero() && !ev.Start.Before(q.timeMax) {
				continue
			}
			if !q.timeMin.IsZero() && !end.After(q.timeMin) {
				continue
			}
		}
		out = append(out, ev)
	}
	return out
}

// ---- ACL loading (acl-audit) ----

type gcalACLRule struct {
	CalendarID string `json:"calendar"`
	RuleID     string `json:"ruleId"`
	Role       string `json:"role"`
	ScopeType  string `json:"scopeType"`
	ScopeValue string `json:"scopeValue,omitempty"`
}

type rawACLRule struct {
	ID    string `json:"id"`
	Role  string `json:"role"`
	Scope struct {
		Type  string `json:"type"`
		Value string `json:"value"`
	} `json:"scope"`
}

// gcalLoadACL loads ACL rules across the given calendars, live-per-calendar by
// default (caching into the acl table) with a local path under verify / local.
func gcalLoadACL(cmd *cobra.Command, flags *rootFlags, calendars []string) ([]gcalACLRule, DataProvenance, error) {
	prov := DataProvenance{Source: "local", ResourceType: "acl"}
	dbPath := defaultDBPath("google-calendar-pp-cli")
	db, err := store.OpenWithContext(cmd.Context(), dbPath)
	if err != nil {
		return nil, prov, fmt.Errorf("opening database: %w", err)
	}
	defer db.Close()

	if useLocalEvents(flags) {
		rules, err := loadACLLocal(db, calendars)
		return rules, prov, err
	}

	c, err := flags.newClient()
	if err != nil {
		return nil, prov, err
	}
	var all []gcalACLRule
	var liveErr error
	for _, cal := range calendars {
		data, err := c.Get(cmd.Context(), "/calendars/"+url.PathEscape(cal)+"/acl", nil)
		if err != nil {
			liveErr = err
			continue
		}
		for _, it := range extractEventItems(data) {
			var r rawACLRule
			if json.Unmarshal(it, &r) != nil || r.ID == "" {
				continue
			}
			all = append(all, gcalACLRule{CalendarID: cal, RuleID: r.ID, Role: r.Role, ScopeType: r.Scope.Type, ScopeValue: r.Scope.Value})
			_, _ = db.DB().Exec(`INSERT OR REPLACE INTO "acl"("id","calendars_id","data") VALUES(?,?,?)`, r.ID, cal, string(it))
		}
	}
	if liveErr != nil && len(all) == 0 {
		if flags.dataSource == "live" {
			return nil, prov, classifyAPIError(liveErr, flags)
		}
		rules, lerr := loadACLLocal(db, calendars)
		if lerr != nil {
			return nil, prov, classifyAPIError(liveErr, flags)
		}
		prov.Reason = "api_unreachable"
		return rules, prov, nil
	}
	prov.Source = "live"
	return all, prov, nil
}

func loadACLLocal(db *store.Store, calendars []string) ([]gcalACLRule, error) {
	query := `SELECT "calendars_id","data" FROM "acl"`
	var args []any
	if len(calendars) > 0 {
		ph := make([]string, len(calendars))
		for i, c := range calendars {
			ph[i] = "?"
			args = append(args, c)
		}
		query += ` WHERE "calendars_id" IN (` + strings.Join(ph, ",") + `)`
	}
	rows, err := db.DB().Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying acl: %w", err)
	}
	defer rows.Close()
	var out []gcalACLRule
	for rows.Next() {
		var calID, data string
		if err := rows.Scan(&calID, &data); err != nil {
			continue
		}
		var r rawACLRule
		if json.Unmarshal([]byte(data), &r) != nil || r.ID == "" {
			continue
		}
		out = append(out, gcalACLRule{CalendarID: calID, RuleID: r.ID, Role: r.Role, ScopeType: r.Scope.Type, ScopeValue: r.Scope.Value})
	}
	return out, rows.Err()
}

// ---- window / time parsing ----

var relWindowRe = regexp.MustCompile(`^(next|last|past)\s+(\d+)\s+(day|days|week|weeks|month|months|hour|hours)$`)
var shortRelRe = regexp.MustCompile(`^(\d+)([dhwm])$`)
var isoDateRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

func startOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// startOfWeek returns Monday 00:00 of t's week.
func startOfWeek(t time.Time) time.Time {
	d := startOfDay(t)
	weekday := int(d.Weekday()) // Sunday=0
	delta := (weekday + 6) % 7  // days since Monday
	return d.AddDate(0, 0, -delta)
}

// parseWindow resolves a human window phrase into [start, end). Empty defaults
// to the next 7 days.
func parseWindow(s string) (time.Time, time.Time, error) {
	now := time.Now()
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return now, now.AddDate(0, 0, 7), nil
	}
	switch s {
	case "today":
		d := startOfDay(now)
		return d, d.AddDate(0, 0, 1), nil
	case "tomorrow":
		d := startOfDay(now).AddDate(0, 0, 1)
		return d, d.AddDate(0, 0, 1), nil
	case "yesterday":
		d := startOfDay(now).AddDate(0, 0, -1)
		return d, d.AddDate(0, 0, 1), nil
	case "this week":
		d := startOfWeek(now)
		return d, d.AddDate(0, 0, 7), nil
	case "next week":
		d := startOfWeek(now).AddDate(0, 0, 7)
		return d, d.AddDate(0, 0, 7), nil
	case "this month":
		y, m, _ := now.Date()
		d := time.Date(y, m, 1, 0, 0, 0, 0, now.Location())
		return d, d.AddDate(0, 1, 0), nil
	case "next month":
		y, m, _ := now.Date()
		d := time.Date(y, m, 1, 0, 0, 0, 0, now.Location()).AddDate(0, 1, 0)
		return d, d.AddDate(0, 1, 0), nil
	}
	if m := relWindowRe.FindStringSubmatch(s); m != nil {
		var n int
		fmt.Sscanf(m[2], "%d", &n)
		dir := 1
		if m[1] == "last" || m[1] == "past" {
			dir = -1
		}
		var end time.Time
		switch {
		case strings.HasPrefix(m[3], "day"):
			end = now.AddDate(0, 0, dir*n)
		case strings.HasPrefix(m[3], "week"):
			end = now.AddDate(0, 0, dir*n*7)
		case strings.HasPrefix(m[3], "month"):
			end = now.AddDate(0, dir*n, 0)
		case strings.HasPrefix(m[3], "hour"):
			end = now.Add(time.Duration(dir*n) * time.Hour)
		}
		if dir < 0 {
			return end, now, nil
		}
		return now, end, nil
	}
	if m := shortRelRe.FindStringSubmatch(s); m != nil {
		var n int
		fmt.Sscanf(m[1], "%d", &n)
		var end time.Time
		switch m[2] {
		case "d":
			end = now.AddDate(0, 0, n)
		case "w":
			end = now.AddDate(0, 0, n*7)
		case "m":
			end = now.Add(time.Duration(n) * time.Minute)
		case "h":
			end = now.Add(time.Duration(n) * time.Hour)
		}
		return now, end, nil
	}
	// Explicit range "A..B" or single date / RFC3339.
	if lo, hi, ok := strings.Cut(s, ".."); ok {
		start, err := parsePointTime(lo)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid window start %q: %w", lo, err)
		}
		end, err := parsePointTime(hi)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid window end %q: %w", hi, err)
		}
		return start, end, nil
	}
	if isoDateRe.MatchString(s) {
		d, err := time.ParseInLocation("2006-01-02", s, time.Local)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
		return d, d.AddDate(0, 0, 1), nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, t.AddDate(0, 0, 1), nil
	}
	return time.Time{}, time.Time{}, fmt.Errorf("unrecognized window %q (try: today, this week, next 7 days, 2026-05-24, or a..b range)", s)
}

// parsePointTime parses a single instant: RFC3339, YYYY-MM-DD, or relative Nd/Nh/Nw/Nm (into the past).
func parsePointTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Now().AddDate(0, 0, -7), nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	if isoDateRe.MatchString(s) {
		return time.ParseInLocation("2006-01-02", s, time.Local)
	}
	if m := shortRelRe.FindStringSubmatch(strings.ToLower(s)); m != nil {
		var n int
		fmt.Sscanf(m[1], "%d", &n)
		now := time.Now()
		switch m[2] {
		case "d":
			return now.AddDate(0, 0, -n), nil
		case "w":
			return now.AddDate(0, 0, -n*7), nil
		case "m":
			return now.Add(-time.Duration(n) * time.Minute), nil
		case "h":
			return now.Add(-time.Duration(n) * time.Hour), nil
		}
	}
	return time.Time{}, fmt.Errorf("expected RFC3339, YYYY-MM-DD, or a relative value like 7d")
}
