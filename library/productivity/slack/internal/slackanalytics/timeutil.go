// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package slackanalytics

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// RetentionWall is the message age past which a workspace on Slack's free
// plan can no longer surface history through the API or the client. Anything
// older than this in the local mirror exists nowhere else, which is the whole
// point of keeping the mirror.
const RetentionWall = 90 * 24 * time.Hour

// ParseSlackTS converts a Slack message timestamp ("1712345678.000200",
// and the bare-seconds and integer forms the API also emits) into a UTC
// time. The bool reports whether the value parsed; callers must not treat
// a false return as the zero epoch.
func ParseSlackTS(ts string) (time.Time, bool) {
	trimmed := strings.TrimSpace(ts)
	if trimmed == "" {
		return time.Time{}, false
	}
	secPart, fracPart, hasFrac := strings.Cut(trimmed, ".")
	sec, err := strconv.ParseInt(secPart, 10, 64)
	if err != nil || sec <= 0 {
		return time.Time{}, false
	}
	var nsec int64
	if hasFrac && fracPart != "" {
		// Slack's fractional part is six digits (microseconds); pad or
		// truncate defensively so odd widths still land in nanoseconds.
		digits := fracPart
		if len(digits) > 9 {
			digits = digits[:9]
		}
		for len(digits) < 9 {
			digits += "0"
		}
		frac, ferr := strconv.ParseInt(digits, 10, 64)
		if ferr != nil {
			return time.Time{}, false
		}
		nsec = frac
	}
	return time.Unix(sec, nsec).UTC(), true
}

// FormatSlackTS renders a time back into Slack's seconds.microseconds form.
func FormatSlackTS(t time.Time) string {
	return fmt.Sprintf("%d.%06d", t.Unix(), t.Nanosecond()/1000)
}

// BeyondRetention reports whether msg is older than the supplied retention
// wall relative to now. A non-positive wall disables the check.
func BeyondRetention(msg, now time.Time, wall time.Duration) bool {
	if wall <= 0 || msg.IsZero() {
		return false
	}
	return now.Sub(msg) > wall
}

// AgeDays returns whole days between t and now, floored at zero. Future
// timestamps (clock skew between Slack and the local host) report 0 rather
// than a negative age.
func AgeDays(t, now time.Time) int {
	if t.IsZero() {
		return 0
	}
	d := now.Sub(t)
	if d <= 0 {
		return 0
	}
	return int(d / (24 * time.Hour))
}

// RoundDays converts a duration to fractional days rounded to two decimals,
// which is the precision the coverage and health reports print.
func RoundDays(d time.Duration) float64 {
	if d <= 0 {
		return 0
	}
	return math.Round(d.Hours()/24*100) / 100
}
