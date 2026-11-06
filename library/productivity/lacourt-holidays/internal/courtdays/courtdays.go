// Copyright 2026 Bob Benjy and contributors. Licensed under Apache-2.0. See LICENSE.

// Package courtdays holds the court-day arithmetic used by the LASC holiday
// commands. It is pure logic: holidays are supplied by a YearLoader, so the
// package never touches the network or disk.
package courtdays

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Holiday is one judicial holiday as published by the court.
type Holiday struct {
	Date        string `json:"date"` // ISO YYYY-MM-DD
	Day         string `json:"day"`
	Description string `json:"description"`
}

// Time returns the holiday date as a UTC midnight time.
func (h Holiday) Time() time.Time {
	t, _ := time.Parse(isoLayout, h.Date)
	return t
}

const isoLayout = "2006-01-02"

// ErrUnpublishedYear reports that the court has not published holidays for a
// year, so court-day answers for that year would be guesses.
type ErrUnpublishedYear struct{ Year int }

func (e ErrUnpublishedYear) Error() string {
	return fmt.Sprintf("the court has not published holidays for %d; court-day math for that year would be unreliable", e.Year)
}

// IsUnpublished reports whether err is an ErrUnpublishedYear.
func IsUnpublished(err error) bool {
	var e ErrUnpublishedYear
	return errors.As(err, &e)
}

// ParseHolidays decodes the upstream payload. It accepts either a JSON array
// of {date "MM/DD/YYYY", day, description} or that array double-encoded as a
// JSON string (the raw wire shape of the LASC endpoint).
func ParseHolidays(raw []byte) ([]Holiday, error) {
	trimmed := strings.TrimSpace(string(raw))
	if strings.HasPrefix(trimmed, `"`) {
		var inner string
		if err := json.Unmarshal([]byte(trimmed), &inner); err != nil {
			return nil, fmt.Errorf("decoding holiday payload: %w", err)
		}
		trimmed = strings.TrimSpace(inner)
	}
	var wire []struct {
		Date        string `json:"date"`
		Day         string `json:"day"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal([]byte(trimmed), &wire); err != nil {
		return nil, fmt.Errorf("decoding holiday list: %w", err)
	}
	out := make([]Holiday, 0, len(wire))
	for _, w := range wire {
		t, err := ParseDate(w.Date)
		if err != nil {
			return nil, fmt.Errorf("holiday %q: %w", w.Description, err)
		}
		day := w.Day
		if day == "" {
			day = t.Weekday().String()
		}
		out = append(out, Holiday{Date: t.Format(isoLayout), Day: day, Description: strings.TrimSpace(w.Description)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date < out[j].Date })
	return out, nil
}

// ParseDate accepts YYYY-MM-DD or MM/DD/YYYY and returns UTC midnight.
func ParseDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	for _, layout := range []string{isoLayout, "01/02/2006", "1/2/2006"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid date %q (use YYYY-MM-DD or MM/DD/YYYY)", s)
}

// Midnight truncates t to a UTC calendar date.
func Midnight(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// YearLoader returns the published holidays for a year. An empty slice means
// the court has not published that year.
type YearLoader func(year int) ([]Holiday, error)

// Calendar answers court-day questions, loading years lazily.
type Calendar struct {
	load  YearLoader
	years map[int]map[string]Holiday
	lists map[int][]Holiday
}

// NewCalendar builds a Calendar over a loader.
func NewCalendar(load YearLoader) *Calendar {
	return &Calendar{load: load, years: map[int]map[string]Holiday{}, lists: map[int][]Holiday{}}
}

// Year returns the holidays for a year, or ErrUnpublishedYear.
func (c *Calendar) Year(year int) ([]Holiday, error) {
	if _, err := c.yearMap(year); err != nil {
		return nil, err
	}
	out := make([]Holiday, len(c.lists[year]))
	copy(out, c.lists[year])
	return out, nil
}

func (c *Calendar) yearMap(year int) (map[string]Holiday, error) {
	if m, ok := c.years[year]; ok {
		if len(m) == 0 {
			return nil, ErrUnpublishedYear{Year: year}
		}
		return m, nil
	}
	hs, err := c.load(year)
	if err != nil {
		return nil, err
	}
	m := map[string]Holiday{}
	for _, h := range hs {
		m[h.Date] = h
	}
	sorted := make([]Holiday, len(hs))
	copy(sorted, hs)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Date < sorted[j].Date })
	c.years[year] = m
	c.lists[year] = sorted
	if len(m) == 0 {
		return nil, ErrUnpublishedYear{Year: year}
	}
	return m, nil
}

// Status explains whether a date is a court day.
type Status struct {
	Date     string   `json:"date"`
	Weekday  string   `json:"weekday"`
	CourtDay bool     `json:"court_day"`
	Reason   string   `json:"reason"`
	Holiday  *Holiday `json:"holiday,omitempty"`
}

// Check reports whether d is a court day.
func (c *Calendar) Check(d time.Time) (Status, error) {
	d = Midnight(d)
	st := Status{Date: d.Format(isoLayout), Weekday: d.Weekday().String()}
	m, err := c.yearMap(d.Year())
	if err != nil {
		return st, err
	}
	if h, ok := m[st.Date]; ok {
		h := h
		st.Holiday = &h
		st.Reason = "court holiday: " + h.Description
		return st, nil
	}
	if d.Weekday() == time.Saturday || d.Weekday() == time.Sunday {
		st.Reason = "weekend"
		return st, nil
	}
	st.CourtDay = true
	st.Reason = "court is open"
	return st, nil
}

// Skipped records a non-court day passed over during counting.
type Skipped struct {
	Date   string `json:"date"`
	Reason string `json:"reason"`
}

// AddResult is the outcome of AddCourtDays.
type AddResult struct {
	Start           string    `json:"start"`
	CourtDays       int       `json:"court_days"`
	Result          string    `json:"result"`
	ResultWeekday   string    `json:"result_weekday"`
	StartIsCourtDay bool      `json:"start_is_court_day"`
	SkippedHolidays []Holiday `json:"skipped_holidays"`
}

// AddCourtDays counts n court days from start, excluding the start date
// (CCP 12 convention: exclude the first day). Negative n counts backward.
func (c *Calendar) AddCourtDays(start time.Time, n int) (AddResult, error) {
	start = Midnight(start)
	res := AddResult{Start: start.Format(isoLayout), CourtDays: n, SkippedHolidays: make([]Holiday, 0)}
	startStatus, err := c.Check(start)
	if err != nil {
		return res, err
	}
	res.StartIsCourtDay = startStatus.CourtDay
	step := 1
	remaining := n
	if n < 0 {
		step = -1
		remaining = -n
	}
	d := start
	for remaining > 0 {
		d = d.AddDate(0, 0, step)
		st, err := c.Check(d)
		if err != nil {
			return res, err
		}
		if st.CourtDay {
			remaining--
			continue
		}
		if st.Holiday != nil {
			res.SkippedHolidays = append(res.SkippedHolidays, *st.Holiday)
		}
	}
	res.Result = d.Format(isoLayout)
	res.ResultWeekday = d.Weekday().String()
	return res, nil
}

// DeadlineResult is the outcome of a calendar-day period with roll-forward.
type DeadlineResult struct {
	Start        string    `json:"start"`
	CalendarDays int       `json:"calendar_days"`
	RawEnd       string    `json:"raw_end"`
	Deadline     string    `json:"deadline"`
	Weekday      string    `json:"weekday"`
	Rolled       bool      `json:"rolled"`
	RolledPast   []Skipped `json:"rolled_past"`
}

// CalendarDeadline adds n calendar days and, when the last day is not a court
// day, moves to the nearest court day: forward by default (CCP 12a pattern),
// backward when back is true (periods counted back from an event).
func (c *Calendar) CalendarDeadline(start time.Time, n int, back bool) (DeadlineResult, error) {
	start = Midnight(start)
	end := start.AddDate(0, 0, n)
	res := DeadlineResult{Start: start.Format(isoLayout), CalendarDays: n, RawEnd: end.Format(isoLayout), RolledPast: make([]Skipped, 0)}
	step := 1
	if back {
		step = -1
	}
	d := end
	for {
		st, err := c.Check(d)
		if err != nil {
			return res, err
		}
		if st.CourtDay {
			break
		}
		res.Rolled = true
		res.RolledPast = append(res.RolledPast, Skipped{Date: st.Date, Reason: st.Reason})
		d = d.AddDate(0, 0, step)
	}
	res.Deadline = d.Format(isoLayout)
	res.Weekday = d.Weekday().String()
	return res, nil
}

// BetweenResult is the outcome of CourtDaysBetween.
type BetweenResult struct {
	From      string    `json:"from"`
	To        string    `json:"to"`
	CourtDays int       `json:"court_days"`
	Holidays  []Holiday `json:"holidays"`
	Counting  string    `json:"counting"`
}

// CourtDaysBetween counts court days after from through to (from excluded,
// to included), so AddCourtDays(from, result) lands on to when to is a court
// day. When to is before from the count is negative.
func (c *Calendar) CourtDaysBetween(from, to time.Time) (BetweenResult, error) {
	from, to = Midnight(from), Midnight(to)
	res := BetweenResult{From: from.Format(isoLayout), To: to.Format(isoLayout), Holidays: make([]Holiday, 0),
		Counting: "excludes the from date, includes the to date"}
	sign := 1
	lo, hi := from, to
	if to.Before(from) {
		sign = -1
		lo, hi = to.AddDate(0, 0, -1), from.AddDate(0, 0, -1)
		res.Counting = "to is before from: counts court days from the to date up to the day before from, as a negative number"
	}
	count := 0
	for d := lo.AddDate(0, 0, 1); !d.After(hi); d = d.AddDate(0, 0, 1) {
		st, err := c.Check(d)
		if err != nil {
			return res, err
		}
		if st.CourtDay {
			count++
		} else if st.Holiday != nil {
			res.Holidays = append(res.Holidays, *st.Holiday)
		}
	}
	res.CourtDays = sign * count
	return res, nil
}

// NextHolidays returns up to limit holidays on or after from, crossing into
// later published years. It stops at the first unpublished year.
func (c *Calendar) NextHolidays(from time.Time, limit int) ([]Holiday, error) {
	from = Midnight(from)
	key := from.Format(isoLayout)
	out := make([]Holiday, 0, limit)
	for year := from.Year(); len(out) < limit; year++ {
		hs, err := c.Year(year)
		if err != nil {
			if IsUnpublished(err) {
				if year == from.Year() {
					return out, err
				}
				break
			}
			return out, err
		}
		for _, h := range hs {
			if h.Date >= key {
				out = append(out, h)
				if len(out) == limit {
					break
				}
			}
		}
	}
	return out, nil
}

// ICS renders holidays as an RFC 5545 VCALENDAR of all-day events.
func ICS(holidays []Holiday, stamp time.Time) string {
	var b strings.Builder
	w := func(s string) { b.WriteString(foldICSLine(s)); b.WriteString("\r\n") }
	w("BEGIN:VCALENDAR")
	w("VERSION:2.0")
	w("PRODID:-//lacourt-holidays-pp-cli//LASC Court Holidays//EN")
	w("CALSCALE:GREGORIAN")
	w("METHOD:PUBLISH")
	w("X-WR-CALNAME:LA Superior Court Holidays")
	dtstamp := stamp.UTC().Format("20060102T150405Z")
	for _, h := range holidays {
		t := h.Time()
		w("BEGIN:VEVENT")
		w("UID:" + t.Format("20060102") + "-lasc-holiday@lacourt-holidays-pp-cli")
		w("DTSTAMP:" + dtstamp)
		w("DTSTART;VALUE=DATE:" + t.Format("20060102"))
		w("DTEND;VALUE=DATE:" + t.AddDate(0, 0, 1).Format("20060102"))
		w("SUMMARY:" + icsEscape("LASC closed: "+h.Description))
		w("DESCRIPTION:" + icsEscape("Superior Court of Los Angeles County judicial holiday ("+h.Day+"). Source: https://www.lacourt.ca.gov/apps/court-holidays"))
		w("TRANSP:TRANSPARENT")
		w("END:VEVENT")
	}
	w("END:VCALENDAR")
	return b.String()
}

func icsEscape(s string) string {
	r := strings.NewReplacer(`\`, `\\`, ";", `\;`, ",", `\,`, "\r\n", `\n`, "\r", `\n`, "\n", `\n`)
	return r.Replace(s)
}

// foldICSLine folds a content line at 75 octets per RFC 5545 section 3.1,
// splitting only on UTF-8 rune boundaries; continuation lines start with a space.
func foldICSLine(s string) string {
	if len(s) <= 75 {
		return s
	}
	var b strings.Builder
	limit, n := 75, 0
	for _, r := range s {
		size := len(string(r))
		if n+size > limit {
			b.WriteString("\r\n ")
			n, limit = 0, 74
		}
		b.WriteRune(r)
		n += size
	}
	return b.String()
}
