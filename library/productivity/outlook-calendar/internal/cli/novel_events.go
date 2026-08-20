// Hand-built (Phase 3): shared event-loading helpers for the local-store
// transcendence commands (conflicts, freetime, review, pending, recurring-drift,
// prep, with, tz-audit). Microsoft Graph events ride in the synced `events`
// table's `data` JSON; this file projects the relevant fields into a Go-typed
// shape so individual commands stay focused on their own logic.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// graphEvent mirrors the Microsoft Graph `event` shape — only the fields the
// novel commands actually use. Times are kept as strings to preserve the
// original timezone metadata; convert via parseGraphTime.
type graphEvent struct {
	ID             string
	Subject        string
	BodyPreview    string
	WebLink        string
	OnlineMeeting  bool
	OnlineProvider string
	IsCancelled    bool
	IsOrganizer    bool
	IsAllDay       bool
	ShowAs         string
	Type           string // singleInstance | occurrence | exception | seriesMaster
	SeriesMasterID string
	Start          graphDateTime
	End            graphDateTime
	Location       string
	Organizer      graphAddress
	Attendees      []graphAttendee
	ResponseStatus graphResponseStatus
	Recurrence     json.RawMessage
	LastModified   string
	Categories     []string
}

type graphDateTime struct {
	DateTime string `json:"dateTime"`
	TimeZone string `json:"timeZone"`
}

type graphAddress struct {
	EmailAddress struct {
		Address string `json:"address"`
		Name    string `json:"name"`
	} `json:"emailAddress"`
}

type graphAttendee struct {
	Type         string       `json:"type"` // required | optional | resource
	EmailAddress graphAddress `json:"-"`
	Email        string       `json:"email"`
	Name         string       `json:"name"`
	Response     string       `json:"response"`
}

type graphResponseStatus struct {
	Response string `json:"response"`
	Time     string `json:"time"`
}

// parseGraphTime turns a Microsoft Graph dateTime+timeZone pair into a
// `time.Time`. Microsoft serialises with up to seven fractional-second
// digits; time.Parse handles the canonical RFC3339 form, so we try a
// short list of known shapes.
func parseGraphTime(g graphDateTime) (time.Time, error) {
	if g.DateTime == "" {
		return time.Time{}, errors.New("empty dateTime")
	}
	zone, err := time.LoadLocation(g.TimeZone)
	if err != nil || g.TimeZone == "" {
		zone = time.UTC
	}
	formats := []string{
		"2006-01-02T15:04:05.9999999",
		"2006-01-02T15:04:05.999999",
		"2006-01-02T15:04:05",
		time.RFC3339Nano,
		time.RFC3339,
	}
	for _, f := range formats {
		if t, err := time.ParseInLocation(f, g.DateTime, zone); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognised dateTime format %q", g.DateTime)
}

// loadEvents reads every non-cancelled event row from the store and unmarshals
// the `data` JSON into graphEvent. Pass start/end to range-filter via
// json_extract on start/end dateTime fields; pass zero values to load all.
//
// SQL bounds are coarse and rely on stored timestamps being UTC-shaped
// (`YYYY-MM-DDTHH:MM:SS[.fraction]`), which is what Graph's calendarView
// emits. The Go-side overlap filter below is the precise gate; SQL is the
// memory/IO bound so we don't pull years of history just to inspect a week.
func loadEvents(ctx context.Context, db *sql.DB, start, end time.Time) ([]graphEvent, error) {
	q := `SELECT data FROM events WHERE COALESCE(is_cancelled, 0) = 0`
	args := []any{}
	// PATCH: push start/end window into SQL WHERE via json_extract so conflicts/freetime/prep don't pull the whole events table into memory; Go-side overlap filter remains the precise gate.
	if !start.IsZero() {
		q += ` AND json_extract(data, '$.start.dateTime') IS NOT NULL`
		q += ` AND json_extract(data, '$.end.dateTime') >= ?`
		args = append(args, start.UTC().Format("2006-01-02T15:04:05"))
	}
	if !end.IsZero() {
		q += ` AND json_extract(data, '$.start.dateTime') <= ?`
		args = append(args, end.UTC().Format("2006-01-02T15:04:05.9999999"))
	}
	q += ` ORDER BY json_extract(data, '$.start.dateTime')`
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("loading events: %w", err)
	}
	defer rows.Close()

	var out []graphEvent
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		ev, err := parseGraphEvent(raw)
		if err != nil {
			continue
		}
		if !start.IsZero() {
			s, terr := parseGraphTime(ev.Start)
			if terr != nil {
				continue
			}
			if !end.IsZero() && s.After(end) {
				continue
			}
			e, terr := parseGraphTime(ev.End)
			if terr != nil {
				continue
			}
			if e.Before(start) {
				continue
			}
		}
		out = append(out, ev)
	}
	return out, rows.Err()
}

// parseGraphEvent unmarshals a Microsoft Graph event JSON blob into the
// graphEvent shape used by the novel commands.
func parseGraphEvent(raw []byte) (graphEvent, error) {
	var wire struct {
		ID              string        `json:"id"`
		Subject         string        `json:"subject"`
		BodyPreview     string        `json:"bodyPreview"`
		WebLink         string        `json:"webLink"`
		IsOnlineMeeting bool          `json:"isOnlineMeeting"`
		OnlineProvider  string        `json:"onlineMeetingProvider"`
		IsCancelled     bool          `json:"isCancelled"`
		IsOrganizer     bool          `json:"isOrganizer"`
		IsAllDay        bool          `json:"isAllDay"`
		ShowAs          string        `json:"showAs"`
		Type            string        `json:"type"`
		SeriesMasterID  string        `json:"seriesMasterId"`
		Start           graphDateTime `json:"start"`
		End             graphDateTime `json:"end"`
		Location        struct {
			DisplayName string `json:"displayName"`
		} `json:"location"`
		Organizer      graphAddress        `json:"organizer"`
		Attendees      []rawAttendee       `json:"attendees"`
		ResponseStatus graphResponseStatus `json:"responseStatus"`
		Recurrence     json.RawMessage     `json:"recurrence"`
		LastModified   string              `json:"lastModifiedDateTime"`
		Categories     []string            `json:"categories"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		return graphEvent{}, err
	}
	ev := graphEvent{
		ID:             wire.ID,
		Subject:        wire.Subject,
		BodyPreview:    wire.BodyPreview,
		WebLink:        wire.WebLink,
		OnlineMeeting:  wire.IsOnlineMeeting,
		OnlineProvider: wire.OnlineProvider,
		IsCancelled:    wire.IsCancelled,
		IsOrganizer:    wire.IsOrganizer,
		IsAllDay:       wire.IsAllDay,
		ShowAs:         wire.ShowAs,
		Type:           wire.Type,
		SeriesMasterID: wire.SeriesMasterID,
		Start:          wire.Start,
		End:            wire.End,
		Location:       wire.Location.DisplayName,
		Organizer:      wire.Organizer,
		ResponseStatus: wire.ResponseStatus,
		Recurrence:     wire.Recurrence,
		LastModified:   wire.LastModified,
		Categories:     wire.Categories,
	}
	for _, a := range wire.Attendees {
		ev.Attendees = append(ev.Attendees, graphAttendee{
			Type:     a.Type,
			Email:    strings.ToLower(a.EmailAddress.Address),
			Name:     a.EmailAddress.Name,
			Response: a.Status.Response,
		})
	}
	return ev, nil
}

type rawAttendee struct {
	Type         string `json:"type"`
	EmailAddress struct {
		Address string `json:"address"`
		Name    string `json:"name"`
	} `json:"emailAddress"`
	Status graphResponseStatus `json:"status"`
}

// resolveWindow parses --from/--to flags into start/end times. Accepts ISO 8601,
// "today", "tomorrow", "+Nd"/"-Nd" relative offsets, and "next-Md" shortcuts.
func resolveWindow(from, to string, defaultDays int) (time.Time, time.Time, error) {
	now := time.Now()
	var start, end time.Time
	var err error
	if from == "" {
		start = now
	} else if start, err = parseHumanTime(from, now); err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("--from %q: %w", from, err)
	}
	if to == "" {
		end = start.Add(time.Duration(defaultDays) * 24 * time.Hour)
	} else if end, err = parseHumanTime(to, start); err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("--to %q: %w", to, err)
	}
	if end.Before(start) {
		return time.Time{}, time.Time{}, fmt.Errorf("--to (%s) is before --from (%s)", end, start)
	}
	return start, end, nil
}

func parseHumanTime(s string, anchor time.Time) (time.Time, error) {
	s = strings.TrimSpace(s)
	switch strings.ToLower(s) {
	case "now":
		return anchor, nil
	case "today":
		return time.Date(anchor.Year(), anchor.Month(), anchor.Day(), 0, 0, 0, 0, anchor.Location()), nil
	case "tomorrow":
		t := anchor.AddDate(0, 0, 1)
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, anchor.Location()), nil
	}
	if strings.HasPrefix(s, "+") || strings.HasPrefix(s, "-") {
		sign := 1
		if s[0] == '-' {
			sign = -1
		}
		body := s[1:]
		if len(body) > 1 {
			unit := body[len(body)-1]
			var n int
			if _, perr := fmt.Sscanf(body[:len(body)-1], "%d", &n); perr == nil {
				switch unit {
				case 'd':
					return anchor.AddDate(0, 0, sign*n), nil
				case 'h':
					return anchor.Add(time.Duration(sign*n) * time.Hour), nil
				case 'm':
					return anchor.Add(time.Duration(sign*n) * time.Minute), nil
				}
			}
		}
	}
	formats := []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02 15:04:05", "2006-01-02"}
	for _, f := range formats {
		if t, err := time.ParseInLocation(f, s, anchor.Location()); err == nil {
			return t, nil
		}
	}
	return time.Time{}, errors.New("unrecognised time format (use ISO 8601, today, tomorrow, +Nd, -Nh)")
}
