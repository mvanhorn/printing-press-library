// Copyright 2026 David Barbier and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: human period strings for Umami stats windows.

package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// periodWindow is a resolved [start, end) range in epoch milliseconds.
type periodWindow struct {
	StartMs int64
	EndMs   int64
}

// parsePeriod resolves a human period string into an epoch-ms window ending
// at (or containing) now. Supported: "24h"-style hours, "7d" days, "2w"
// weeks, "today", "yesterday", "this-week", "this-month", "last-month".
func parsePeriod(s string, now time.Time) (periodWindow, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	switch s {
	case "today":
		return periodWindow{ms(dayStart), ms(now)}, nil
	case "yesterday":
		y := dayStart.AddDate(0, 0, -1)
		return periodWindow{ms(y), ms(dayStart)}, nil
	case "this-week":
		// ISO week: Monday start.
		offset := (int(dayStart.Weekday()) + 6) % 7
		monday := dayStart.AddDate(0, 0, -offset)
		return periodWindow{ms(monday), ms(now)}, nil
	case "this-month":
		first := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		return periodWindow{ms(first), ms(now)}, nil
	case "last-month":
		first := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		prev := first.AddDate(0, -1, 0)
		return periodWindow{ms(prev), ms(first)}, nil
	}
	if len(s) > 1 {
		unit := s[len(s)-1]
		n, err := strconv.Atoi(s[:len(s)-1])
		if err == nil && n > 0 {
			switch unit {
			case 'h':
				return periodWindow{ms(now.Add(-time.Duration(n) * time.Hour)), ms(now)}, nil
			case 'd':
				return periodWindow{ms(now.AddDate(0, 0, -n)), ms(now)}, nil
			case 'w':
				return periodWindow{ms(now.AddDate(0, 0, -7*n)), ms(now)}, nil
			}
		}
	}
	return periodWindow{}, fmt.Errorf("invalid --period %q: use forms like 24h, 7d, 2w, today, yesterday, this-week, this-month, last-month", s)
}

// previousWindow returns the equal-length window immediately before w.
func previousWindow(w periodWindow) periodWindow {
	length := w.EndMs - w.StartMs
	return periodWindow{StartMs: w.StartMs - length, EndMs: w.StartMs}
}

func ms(t time.Time) int64 { return t.UnixMilli() }

// dayKey formats a time as the YYYY-MM-DD key used by the local history tables.
func dayKey(t time.Time) string { return t.Format("2006-01-02") }
