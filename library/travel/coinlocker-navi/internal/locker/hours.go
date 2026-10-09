// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.

package locker

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var clockRangeRe = regexp.MustCompile(`(\d{1,2})\s*[時:：]\s*(\d{1,2})?\s*分?\s*[～〜~\-ー－]\s*(\d{1,2})\s*[時:：]\s*(\d{1,2})?`)

// Open-during verdicts.
const (
	OpenYes        = "yes"
	OpenNo         = "no"
	OpenUnknown    = "unknown"
	OpenTrainHours = "train_hours"
)

// Hours kinds.
const (
	HoursClockRange       = "clock_range"
	HoursFirstToLastTrain = "first_to_last_train"
	Hours24h              = "24h"
	HoursUnknown          = "unknown"
	HoursUnparsed         = "unparsed"
	HoursWeekdayRanges    = "weekday_ranges" // Multi Ekicube per-day hours
)

// ParseHours normalizes a 利用時間 cell.
func ParseHours(raw string) Hours {
	raw = strings.TrimSpace(strings.ReplaceAll(raw, " ", " "))
	if isUnknown(raw) {
		return Hours{Kind: HoursUnknown}
	}
	h := Hours{Raw: strPtr(raw)}
	switch {
	case strings.Contains(raw, "24時間"):
		h.Kind = Hours24h
		return h
	case (strings.Contains(raw, "始発") || strings.Contains(raw, "初電")) && strings.Contains(raw, "終電"):
		h.Kind = HoursFirstToLastTrain
		return h
	}
	if m := clockRangeRe.FindStringSubmatch(raw); m != nil {
		oh, _ := strconv.Atoi(m[1])
		om := 0
		if m[2] != "" {
			om, _ = strconv.Atoi(m[2])
		}
		ch, _ := strconv.Atoi(m[3])
		cm := 0
		if m[4] != "" {
			cm, _ = strconv.Atoi(m[4])
		}
		if oh <= 30 && ch <= 30 && om < 60 && cm < 60 {
			o := fmt.Sprintf("%02d:%02d", oh, om)
			c := fmt.Sprintf("%02d:%02d", ch, cm)
			h.Kind = HoursClockRange
			h.Open, h.Close = &o, &c
			cl, op := ch*60+cm, oh*60+om
			h.Overnight = cl <= op || cl > 1440
			return h
		}
	}
	h.Kind = HoursUnparsed
	return h
}

// Window is a drop-off to pick-up window in minutes after midnight. End may
// exceed 1440 when the window crosses midnight.
type Window struct {
	Start, End int
	Raw        string
}

// ParseWindow parses "HH:MM-HH:MM".
func ParseWindow(s string) (Window, error) {
	s = strings.TrimSpace(s)
	parts := strings.Split(s, "-")
	if len(parts) != 2 {
		return Window{}, fmt.Errorf("window %q: want HH:MM-HH:MM", s)
	}
	a, err := parseClock(parts[0])
	if err != nil {
		return Window{}, err
	}
	b, err := parseClock(parts[1])
	if err != nil {
		return Window{}, err
	}
	if b <= a {
		b += 1440
	}
	return Window{Start: a, End: b, Raw: s}, nil
}

func parseClock(s string) (int, error) {
	s = strings.TrimSpace(s)
	hm := strings.Split(s, ":")
	if len(hm) != 2 {
		return 0, fmt.Errorf("time %q: want HH:MM", s)
	}
	h, err1 := strconv.Atoi(hm[0])
	m, err2 := strconv.Atoi(hm[1])
	if err1 != nil || err2 != nil || h < 0 || h > 24 || m < 0 || m > 59 {
		return 0, fmt.Errorf("time %q: want HH:MM", s)
	}
	return h*60 + m, nil
}

// OpenDuring tests whether published hours cover the whole window.
// first-to-last-train hours return train_hours because exact train times are
// not published on the record.
func OpenDuring(h Hours, w Window) string {
	switch h.Kind {
	case Hours24h:
		return OpenYes
	case HoursFirstToLastTrain:
		return OpenTrainHours
	case HoursClockRange:
	default:
		return OpenUnknown
	}
	o, err1 := parseSourceClock(*h.Open)
	c, err2 := parseSourceClock(*h.Close)
	if err1 != nil || err2 != nil {
		return OpenUnknown
	}
	if c <= o {
		c += 1440
	}
	if c-o >= 1440 {
		return OpenYes // published hours cover the whole day
	}
	for _, shift := range []int{0, 1440, -1440} {
		if w.Start+shift >= o && w.End+shift <= c {
			return OpenYes
		}
	}
	return OpenNo
}

// No-train span used to rank 始発～終電 records: about 01:00 to 04:30 JST,
// when regular trains do not run and most stations are shut.
const (
	noTrainStart = 60
	noTrainEnd   = 270
)

// WindowInNoTrainHours reports whether the window overlaps the usual
// no-train span (about 01:00 to 04:30). It only changes ordering; exact
// first and last train times are not published on the record.
func WindowInNoTrainHours(w Window) bool {
	for _, shift := range []int{0, 1440} {
		if w.Start < noTrainEnd+shift && w.End > noTrainStart+shift {
			return true
		}
	}
	return false
}

// parseSourceClock reads a published HH:MM, where hours up to 30 mean the
// next morning (25:00 = 01:00 after midnight).
func parseSourceClock(s string) (int, error) {
	hm := strings.Split(strings.TrimSpace(s), ":")
	if len(hm) != 2 {
		return 0, fmt.Errorf("time %q: want HH:MM", s)
	}
	h, err1 := strconv.Atoi(hm[0])
	m, err2 := strconv.Atoi(hm[1])
	if err1 != nil || err2 != nil || h < 0 || h > 30 || m < 0 || m > 59 {
		return 0, fmt.Errorf("time %q: want HH:MM", s)
	}
	return h*60 + m, nil
}
