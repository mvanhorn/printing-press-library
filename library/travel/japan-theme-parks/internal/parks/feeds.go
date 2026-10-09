// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.

package parks

import (
	"encoding/json"
	"fmt"
	"html"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/japan-theme-parks/internal/cliutil"
)

// Ride is one Queue-Times ride reading.
type Ride struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	Land        string    `json:"land"`
	IsOpen      bool      `json:"is_open"`
	WaitMinutes int       `json:"wait_minutes"`
	LastUpdated time.Time `json:"last_updated"`
}

type qtRide struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	IsOpen      bool   `json:"is_open"`
	WaitTime    int    `json:"wait_time"`
	LastUpdated string `json:"last_updated"`
}

type qtLand struct {
	Name  string   `json:"name"`
	Rides []qtRide `json:"rides"`
}

type qtPark struct {
	Lands []qtLand `json:"lands"`
	Rides []qtRide `json:"rides"`
}

// ParseQueueTimes decodes /parks/{id}/queue_times.json. Rides appear under
// lands[].rides for some parks and under top-level rides[] for others.
func ParseQueueTimes(data []byte) ([]Ride, error) {
	var p qtPark
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("decoding Queue-Times queue_times.json: %w", err)
	}
	out := make([]Ride, 0)
	add := func(land string, r qtRide) error {
		ts, err := time.Parse(time.RFC3339, r.LastUpdated)
		if err != nil {
			return fmt.Errorf("ride %d has unparseable last_updated %q: %w", r.ID, r.LastUpdated, err)
		}
		out = append(out, Ride{ID: r.ID, Name: cleanRemote(r.Name), Land: cleanRemote(land), IsOpen: r.IsOpen, WaitMinutes: r.WaitTime, LastUpdated: ts.UTC()})
		return nil
	}
	for _, l := range p.Lands {
		for _, r := range l.Rides {
			if err := add(l.Name, r); err != nil {
				return nil, err
			}
		}
	}
	for _, r := range p.Rides {
		if err := add("", r); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// ScheduleEntry is one ThemeParks.wiki schedule row.
type ScheduleEntry struct {
	Date        string `json:"date"`
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
	Opening     string `json:"openingTime"`
	Closing     string `json:"closingTime"`
}

// Schedule is a parsed ThemeParks.wiki park schedule.
type Schedule struct {
	ByDate  map[string][]ScheduleEntry
	MinDate string
	MaxDate string
}

// ParseSchedule decodes /v1/entity/{id}/schedule.
func ParseSchedule(data []byte) (Schedule, error) {
	var doc struct {
		Schedule []ScheduleEntry `json:"schedule"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return Schedule{}, fmt.Errorf("decoding ThemeParks.wiki schedule: %w", err)
	}
	s := Schedule{ByDate: map[string][]ScheduleEntry{}}
	for _, e := range doc.Schedule {
		if e.Date == "" {
			continue
		}
		e.Type, e.Description = cleanRemote(e.Type), cleanRemote(e.Description)
		s.ByDate[e.Date] = append(s.ByDate[e.Date], e)
		if s.MinDate == "" || e.Date < s.MinDate {
			s.MinDate = e.Date
		}
		if e.Date > s.MaxDate {
			s.MaxDate = e.Date
		}
	}
	for d := range s.ByDate {
		sort.SliceStable(s.ByDate[d], func(i, j int) bool { return s.ByDate[d][i].Opening < s.ByDate[d][j].Opening })
	}
	return s, nil
}

// HoursFor returns the operating window for a date. The second value is a
// reason when no hours are known.
func (s Schedule) HoursFor(date string) (open, close string, extra []ScheduleEntry, reason string) {
	if s.MaxDate == "" {
		return "", "", nil, "ThemeParks.wiki returned no schedule rows"
	}
	if date > s.MaxDate {
		return "", "", nil, fmt.Sprintf("beyond third-party source horizon (ThemeParks.wiki schedule ends %s)", s.MaxDate)
	}
	if date < s.MinDate {
		return "", "", nil, fmt.Sprintf("before the third-party schedule window (starts %s)", s.MinDate)
	}
	// A day can have several OPERATING rows (split sessions); report the
	// earliest opening and the latest closing.
	entries := s.ByDate[date]
	var first, last time.Time
	for _, e := range entries {
		if e.Type != "OPERATING" {
			extra = append(extra, e)
			continue
		}
		o, errO := time.Parse(time.RFC3339, e.Opening)
		c, errC := time.Parse(time.RFC3339, e.Closing)
		if errO != nil || errC != nil {
			extra = append(extra, e)
			continue
		}
		if first.IsZero() || o.Before(first) {
			first = o
		}
		if last.IsZero() || c.After(last) {
			last = c
		}
	}
	if !first.IsZero() {
		open, close = first.In(Tokyo).Format("15:04"), last.In(Tokyo).Format("15:04")
	}
	if open == "" {
		if len(entries) == 0 {
			return "", "", nil, "no ThemeParks.wiki schedule row for this date (closed or not published)"
		}
		return "", "", extra, "no OPERATING row for this date in ThemeParks.wiki"
	}
	return open, close, extra, ""
}

// cleanRemote normalizes text from a remote source once, at parse time:
// HTML entities are decoded, control characters are removed (tabs and
// newlines become spaces) and outer spaces are trimmed. JSON and table
// output then carry the same clean value.
func cleanRemote(s string) string {
	return strings.TrimSpace(cliutil.ScrubTerminal(html.UnescapeString(s)))
}
