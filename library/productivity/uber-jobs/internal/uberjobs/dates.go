// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package uberjobs

import (
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/uber-jobs/internal/cliutil"
)

// DateFloor is the migration timestamp the site stamps on old postings whose
// true posting date it no longer carries (63 of 584 rows on 2026-10-05, all
// old-series ids). Floor rows get posted_date/posted_on = null and
// posted_date_is_floor = true (owner decision); posted_raw keeps the value.
const DateFloor = "2026-06-19T07:30:00Z"

// TrackerDateLayout is "Month D, YYYY" with no zero padding.
const TrackerDateLayout = "January 2, 2006"

// ParseSiteDate parses a DisplayDate (one live shape: YYYY-MM-DDTHH:MM:SSZ),
// also accepting RFC3339 with offsets or fractions, and a bare YYYY-MM-DD
// (the Oracle fallback's PostedDate).
func ParseSiteDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339, time.RFC3339Nano, "2006-01-02T15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), true
		}
	}
	return time.Time{}, false
}

// IsFloorDate reports whether a raw date is the migration floor.
func IsFloorDate(raw string) bool {
	t, ok := ParseSiteDate(raw)
	if !ok {
		return false
	}
	floor, _ := ParseSiteDate(DateFloor)
	return t.Equal(floor)
}

// postingDates returns posted_on (ISO date), posted_date (tracker form), and
// whether the raw value is the migration floor.
func postingDates(raw string) (*string, *string, bool) {
	t, ok := ParseSiteDate(raw)
	if !ok {
		return nil, nil, false
	}
	if IsFloorDate(raw) {
		return nil, nil, true
	}
	on := t.Format("2006-01-02")
	tracker := t.Format(TrackerDateLayout)
	return &on, &tracker, false
}

// PostedWithin parses a recency window such as 24h, 7d, or 2w. It rejects
// non-positive and unparseable values so bad input fails before any request.
func PostedWithin(raw string) (time.Duration, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	d, err := cliutil.ParseDurationLoose(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid --posted-within %q: use a duration such as 24h, 7d, or 2w", raw)
	}
	if d <= 0 {
		return 0, fmt.Errorf("invalid --posted-within %q: must be positive", raw)
	}
	return d, nil
}

// WithinWindow reports whether a posting's true posting date falls inside the
// window ending at now. The floor is inclusive at the start of the UTC day
// the window opens on. Floor-dated and undated postings never match.
func WithinWindow(p Posting, window time.Duration, now time.Time) bool {
	if window <= 0 {
		return true
	}
	if p.PostedDateIsFloor || p.PostedRaw == nil {
		return false
	}
	t, ok := ParseSiteDate(*p.PostedRaw)
	if !ok {
		return false
	}
	start := now.UTC().Add(-window)
	dayStart := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
	return !t.Before(dayStart)
}

// SortKey returns the time used by --sort recent; floor and undated rows sort last.
func SortKey(p Posting) time.Time {
	if p.PostedDateIsFloor || p.PostedRaw == nil {
		return time.Time{}
	}
	t, _ := ParseSiteDate(*p.PostedRaw)
	return t
}
