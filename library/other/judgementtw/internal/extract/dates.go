// Copyright 2026 wayne-lai. Licensed under Apache-2.0. See LICENSE.

package extract

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ROCEpochOffset is the year delta between the Republic of China (ROC, 民國)
// calendar and the Gregorian calendar. ROC year N == Gregorian year N + 1911.
const ROCEpochOffset = 1911

// ROCDate represents a date in the Republic of China (Taiwan) calendar.
type ROCDate struct {
	Year  int
	Month int
	Day   int
}

// Gregorian returns the equivalent Gregorian-calendar time.Time for an ROC date.
func (r ROCDate) Gregorian() time.Time {
	return time.Date(r.Year+ROCEpochOffset, time.Month(r.Month), r.Day, 0, 0, 0, 0, time.UTC)
}

// ROCFromGregorian converts a time.Time to its ROC equivalent.
func ROCFromGregorian(t time.Time) ROCDate {
	return ROCDate{
		Year:  t.Year() - ROCEpochOffset,
		Month: int(t.Month()),
		Day:   t.Day(),
	}
}

var (
	rocSlash          = regexp.MustCompile(`^\s*(\d{1,3})[/-](\d{1,2})[/-](\d{1,2})\s*$`)
	rocChinese        = regexp.MustCompile(`(\d{1,3})\s*年\s*(\d{1,2})\s*月\s*(\d{1,2})\s*日`)
	gregorianISO      = regexp.MustCompile(`^\s*(\d{4})[/-](\d{1,2})[/-](\d{1,2})\s*$`)
	gregorianYYYYMMDD = regexp.MustCompile(`^\s*(\d{4})(\d{2})(\d{2})\s*$`)
)

// ParseDate accepts any of:
//   - ROC slash form:        "115/4/30", "115-04-30"
//   - ROC Chinese form:      "115年4月30日", "民國115年4月30日"
//   - Gregorian ISO:         "2026-04-30", "2026/04/30"
//   - Gregorian YYYYMMDD:    "20260430"
//
// Returns the equivalent Gregorian date. The heuristic for distinguishing ROC
// from Gregorian is the year width: any year 1900+ is treated as Gregorian,
// anything below 999 is treated as ROC.
func ParseDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "民國")
	s = strings.TrimPrefix(s, "ROC")
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, fmt.Errorf("empty date")
	}
	if m := rocChinese.FindStringSubmatch(s); m != nil {
		y, _ := strconv.Atoi(m[1])
		mo, _ := strconv.Atoi(m[2])
		d, _ := strconv.Atoi(m[3])
		// Year may be ROC or Gregorian
		return resolveYear(y, mo, d), nil
	}
	if m := gregorianYYYYMMDD.FindStringSubmatch(s); m != nil {
		y, _ := strconv.Atoi(m[1])
		mo, _ := strconv.Atoi(m[2])
		d, _ := strconv.Atoi(m[3])
		return time.Date(y, time.Month(mo), d, 0, 0, 0, 0, time.UTC), nil
	}
	if m := gregorianISO.FindStringSubmatch(s); m != nil {
		y, _ := strconv.Atoi(m[1])
		mo, _ := strconv.Atoi(m[2])
		d, _ := strconv.Atoi(m[3])
		return time.Date(y, time.Month(mo), d, 0, 0, 0, 0, time.UTC), nil
	}
	if m := rocSlash.FindStringSubmatch(s); m != nil {
		y, _ := strconv.Atoi(m[1])
		mo, _ := strconv.Atoi(m[2])
		d, _ := strconv.Atoi(m[3])
		return resolveYear(y, mo, d), nil
	}
	return time.Time{}, fmt.Errorf("unrecognized date format %q (expected 115/4/30, 115年4月30日, 2026-04-30, or 20260430)", s)
}

func resolveYear(y, mo, d int) time.Time {
	if y >= 1900 {
		// Already Gregorian
		return time.Date(y, time.Month(mo), d, 0, 0, 0, 0, time.UTC)
	}
	// ROC
	return time.Date(y+ROCEpochOffset, time.Month(mo), d, 0, 0, 0, 0, time.UTC)
}

// FormatROC formats a Gregorian date as "民國 115 年 04 月 30 日".
func FormatROC(t time.Time) string {
	roc := ROCFromGregorian(t)
	return fmt.Sprintf("民國 %d 年 %02d 月 %02d 日", roc.Year, roc.Month, roc.Day)
}

// FormatYYYYMMDD formats a Gregorian date as compact 8-digit form.
func FormatYYYYMMDD(t time.Time) string {
	return t.Format("20060102")
}

// TaipeiTime returns the current time in the Asia/Taipei time zone.
// Falls back to UTC+8 when the IANA database is unavailable.
func TaipeiTime() time.Time {
	loc, err := time.LoadLocation("Asia/Taipei")
	if err != nil {
		loc = time.FixedZone("CST", 8*60*60)
	}
	return time.Now().In(loc)
}

// IsAPIServiceWindow reports whether the given time falls within the official
// Judicial Yuan open-data API service window (00:00–06:00 Asia/Taipei).
func IsAPIServiceWindow(t time.Time) bool {
	loc, err := time.LoadLocation("Asia/Taipei")
	if err != nil {
		loc = time.FixedZone("CST", 8*60*60)
	}
	tt := t.In(loc)
	return tt.Hour() >= 0 && tt.Hour() < 6
}

// SecondsUntilNextWindow returns the number of seconds from `now` until the
// next 00:00 Taipei. When `now` is already inside the window, returns 0.
func SecondsUntilNextWindow(now time.Time) int64 {
	loc, err := time.LoadLocation("Asia/Taipei")
	if err != nil {
		loc = time.FixedZone("CST", 8*60*60)
	}
	tt := now.In(loc)
	if tt.Hour() < 6 {
		return 0
	}
	tomorrow := time.Date(tt.Year(), tt.Month(), tt.Day()+1, 0, 0, 0, 0, loc)
	return int64(tomorrow.Sub(tt).Seconds())
}
