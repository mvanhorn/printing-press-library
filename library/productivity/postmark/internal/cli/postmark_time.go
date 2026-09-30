// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"math"
	"strings"
	"time"
	_ "time/tzdata" // Eastern day boundaries must not depend on the host having zoneinfo installed.

	"github.com/mvanhorn/printing-press-library/library/productivity/postmark/internal/cliutil"
)

// Postmark interprets stats and message date filters in US Eastern time, so
// day buckets and fromdate/todate values use that zone, written without a
// zone suffix.
const (
	postmarkEasternZone   = "America/New_York"
	postmarkEasternLayout = "2006-01-02T15:04:05"
	postmarkDateLayout    = "2006-01-02"
)

func postmarkEastern() *time.Location {
	loc, err := time.LoadLocation(postmarkEasternZone)
	if err != nil {
		// Not reached: time/tzdata is embedded, so the zone always loads.
		return time.FixedZone("EST", -5*60*60)
	}
	return loc
}

// postmarkEasternTimestamp formats t the way Postmark's bounce and message
// date filters expect.
func postmarkEasternTimestamp(t time.Time) string {
	return t.In(postmarkEastern()).Format(postmarkEasternLayout)
}

// postmarkDayStart returns midnight Eastern for the day containing t.
func postmarkDayStart(t time.Time) time.Time {
	et := t.In(postmarkEastern())
	return time.Date(et.Year(), et.Month(), et.Day(), 0, 0, 0, 0, et.Location())
}

// postmarkParseTime parses Postmark timestamps, which mix RFC 3339 with
// seven-digit fractional seconds and offset-less Eastern values.
func postmarkParseTime(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	for _, layout := range []string{"2006-01-02T15:04:05.9999999", postmarkEasternLayout, postmarkDateLayout} {
		if t, err := time.ParseInLocation(layout, s, postmarkEastern()); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// parsePositiveDuration parses a loose duration flag value (7d, 4w, 36h).
// Zero, negative, and unparseable values are usage errors that name the flag
// and show examples.
func parsePositiveDuration(value, flagName, examples string) (time.Duration, error) {
	d, err := cliutil.ParseDurationLoose(value)
	if err != nil || d <= 0 {
		return 0, usageErr(fmt.Errorf("--%s must be a positive duration such as %s (got %q)", flagName, examples, value))
	}
	return d, nil
}

// postmarkParseDays converts a loose duration into whole days, rounding
// partial days up, because Postmark stats are bucketed by day.
func postmarkParseDays(value, flagName string) (int, error) {
	d, err := parsePositiveDuration(value, flagName, "7d, 4w, or 48h")
	if err != nil {
		return 0, err
	}
	return int(math.Ceil(d.Hours() / 24)), nil
}
