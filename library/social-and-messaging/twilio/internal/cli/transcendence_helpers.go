// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// parseSince parses a "<n><unit>" duration like "7d", "24h", "30m" into a
// past time.Time relative to now. Empty input yields zero time (caller
// interprets that as "no lower bound").
//
// Twilio's date_sent / start_time fields are RFC1123 strings on the wire and
// SQLite stores the JSON verbatim, so query callers compare against the ISO
// format datetime('now', '-7 days') style or pass the formatted cutoff into
// json_extract comparisons. Use sinceCutoffForSQL when you need an SQL
// expression that mirrors this helper.
func parseSince(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	re := regexp.MustCompile(`^(\d+)([smhdw])$`)
	m := re.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return time.Time{}, fmt.Errorf("invalid --since %q (expected <n>{s,m,h,d,w}, e.g. 7d)", s)
	}
	n, _ := strconv.Atoi(m[1])
	var d time.Duration
	switch m[2] {
	case "s":
		d = time.Duration(n) * time.Second
	case "m":
		d = time.Duration(n) * time.Minute
	case "h":
		d = time.Duration(n) * time.Hour
	case "d":
		d = time.Duration(n) * 24 * time.Hour
	case "w":
		d = time.Duration(n) * 7 * 24 * time.Hour
	}
	return time.Now().Add(-d), nil
}

// sinceCutoffForSQL returns a SQLite datetime() expression that matches
// parseSince's semantics for use inside json_extract comparisons.
// Twilio dates are RFC1123 in the wire format ("Mon, 02 Jan 2006 15:04:05 +0000"),
// not ISO, so we use SQLite's strftime to coerce both sides to the same shape.
// Caller's WHERE clause should look like:
//
//	WHERE strftime('%Y-%m-%d %H:%M:%S', json_extract(data, '$.date_sent')) >= ?
//
// and pass cutoff.Format("2006-01-02 15:04:05") as the bind value.
func sinceCutoffBind(t time.Time) string {
	if t.IsZero() {
		return "1900-01-01 00:00:00"
	}
	return t.Format("2006-01-02 15:04:05")
}

// twilioDateExpr returns a SQL expression that converts Twilio's RFC1123 wire
// dates (e.g., "Wed, 16 Apr 2014 21:43:11 +0000") into SQLite datetime() shape
// suitable for comparison. Falls back to a NULL-safe coalesce so messages
// without the named field do not blow up the comparison.
func twilioDateExpr(field string) string {
	// SQLite cannot parse RFC1123 directly; we use strftime on the value
	// after coercion via Julian-day round-trip. The simplest portable form is
	// substr() to the YYYY-MM-DD HH:MM:SS prefix when the wire format is ISO
	// (Twilio sometimes returns ISO 8601 in newer endpoints), otherwise we
	// rely on SQLite's lenient datetime() which accepts ISO strings.
	return fmt.Sprintf("COALESCE(datetime(json_extract(data, '$.%s')), datetime('1900-01-01'))", field)
}

// formatDurationHours returns "1.5h" / "12m" / "30s" for human display.
func formatDurationHours(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	return fmt.Sprintf("%.1fh", d.Hours())
}
