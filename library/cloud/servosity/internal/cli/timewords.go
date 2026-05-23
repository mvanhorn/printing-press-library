// Copyright 2026 dstevens. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// parseHumanTime resolves a small set of human-friendly time strings to an
// absolute time.Time relative to `now`. Supports:
//
//	"now"
//	"yesterday"                     (00:00 yesterday local)
//	"tomorrow"                      (00:00 tomorrow local)
//	"6am", "11pm", "06:00", "23:00" (today at that time, local)
//	"6am tomorrow", "11pm yesterday"
//	"30m", "2h", "3d", "1w"          (relative to now)
//	"+30m", "-2h"                   (signed relative)
//	RFC3339 / "2006-01-02"          (passthrough)
//
// Returns an error for unsupported patterns rather than silently picking a
// default — surfacing "I do not know what that means" beats acting on a
// guess against a production API.
func parseHumanTime(s string, now time.Time) (time.Time, error) {
	in := strings.ToLower(strings.TrimSpace(s))
	if in == "" {
		return time.Time{}, fmt.Errorf("empty time string")
	}
	if in == "now" {
		return now, nil
	}

	loc := now.Location()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)

	switch in {
	case "yesterday":
		return today.AddDate(0, 0, -1), nil
	case "tomorrow":
		return today.AddDate(0, 0, 1), nil
	case "today":
		return today, nil
	}

	// "6am tomorrow" / "11pm yesterday" / "6am"
	tail := ""
	clockPart := in
	for _, anchor := range []string{" tomorrow", " yesterday", " today"} {
		if strings.HasSuffix(in, anchor) {
			clockPart = strings.TrimSuffix(in, anchor)
			tail = strings.TrimSpace(anchor)
			break
		}
	}
	if hr, min, ok := parseClock(clockPart); ok {
		base := today
		switch tail {
		case "tomorrow":
			base = today.AddDate(0, 0, 1)
		case "yesterday":
			base = today.AddDate(0, 0, -1)
		}
		return time.Date(base.Year(), base.Month(), base.Day(), hr, min, 0, 0, loc), nil
	}

	// Relative: "+30m", "-2h", "30m", "2h30m", "3d", "1w"
	signed := in
	sign := 1
	if strings.HasPrefix(signed, "+") {
		signed = signed[1:]
	} else if strings.HasPrefix(signed, "-") {
		sign = -1
		signed = signed[1:]
	}
	if dur, ok := parseExtendedDuration(signed); ok {
		return now.Add(time.Duration(sign) * dur), nil
	}

	// RFC3339 or YYYY-MM-DD
	for _, layout := range []string{time.RFC3339, time.RFC3339Nano, "2006-01-02 15:04:05", "2006-01-02"} {
		if t, err := time.ParseInLocation(layout, s, loc); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unrecognized time string: %q (try \"6am tomorrow\", \"yesterday\", \"2h\", or RFC3339)", s)
}

var clockRE = regexp.MustCompile(`^([0-9]{1,2})(?::([0-9]{2}))?\s*(am|pm)?$`)

func parseClock(s string) (hour, minute int, ok bool) {
	m := clockRE.FindStringSubmatch(s)
	if m == nil {
		return 0, 0, false
	}
	hr, err := strconv.Atoi(m[1])
	if err != nil {
		return 0, 0, false
	}
	if m[2] != "" {
		min, err := strconv.Atoi(m[2])
		if err != nil || min > 59 {
			return 0, 0, false
		}
		minute = min
	}
	switch m[3] {
	case "am", "pm":
		if hr < 1 || hr > 12 {
			return 0, 0, false
		}
		if m[3] == "am" && hr == 12 {
			hr = 0
		} else if m[3] == "pm" && hr < 12 {
			hr += 12
		}
	default:
		// 24-hour clock if no am/pm
	}
	if hr > 23 {
		return 0, 0, false
	}
	return hr, minute, true
}

var extDurUnitRE = regexp.MustCompile(`^([0-9]+)([smhdw])$`)

// parseExtendedDuration extends time.ParseDuration with `d` (24h) and `w` (7d).
// Accepts simple single-unit forms like "30m", "2h", "3d", "1w" — composed
// "2h30m" is delegated to time.ParseDuration.
func parseExtendedDuration(s string) (time.Duration, bool) {
	if s == "" {
		return 0, false
	}
	if m := extDurUnitRE.FindStringSubmatch(s); m != nil {
		n, err := strconv.Atoi(m[1])
		if err != nil {
			return 0, false
		}
		switch m[2] {
		case "s":
			return time.Duration(n) * time.Second, true
		case "m":
			return time.Duration(n) * time.Minute, true
		case "h":
			return time.Duration(n) * time.Hour, true
		case "d":
			return time.Duration(n) * 24 * time.Hour, true
		case "w":
			return time.Duration(n) * 7 * 24 * time.Hour, true
		}
	}
	if d, err := time.ParseDuration(s); err == nil {
		return d, true
	}
	return 0, false
}
