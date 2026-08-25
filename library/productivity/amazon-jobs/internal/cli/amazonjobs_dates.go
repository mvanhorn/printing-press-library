// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored date/freshness helpers for the amazon-jobs CLI. Not generated;
// preserved across `generate --force`.

package cli

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// postedDateLayout is the only layout amazon.jobs uses for `posted_date`.
// The field is day-granular: the API exposes no sub-day posting timestamp, so
// every recency comparison in this CLI is a date comparison, never a clock one.
const postedDateLayout = "January 2, 2006"

// collapseSpaces normalizes runs of whitespace to a single space.
//
// amazon.jobs pads single-digit days with a second space ("May  9, 2026"),
// which time.Parse rejects against the "January 2, 2006" layout. This is not an
// edge case: it affects every posting from the 1st to the 9th of a month, ~19%
// of live records in a 1000-req sample. Normalizing first is what makes
// parsePostedDate work at all for those rows.
var collapseSpaces = regexp.MustCompile(`\s+`)

// parsePostedDate parses amazon.jobs' `posted_date` into a local-midnight time.
// Reports false when the field is empty or not in the expected shape, so
// callers can decide whether an unparseable date should include or exclude a
// row rather than silently treating it as epoch.
func parsePostedDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(collapseSpaces.ReplaceAllString(s, " "))
	if s == "" {
		return time.Time{}, false
	}
	t, err := time.ParseInLocation(postedDateLayout, s, time.Local)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// relativeAgeRe matches amazon.jobs' `updated_time` strings. Observed live
// shapes are "about 21 hours", "1 day", "29 minutes", and "about 1 hour"; the
// remaining units are accepted defensively in case the upstream renderer starts
// emitting coarser buckets.
var relativeAgeRe = regexp.MustCompile(`^(?:about\s+)?(?:(\d+)|an?)\s+(minute|hour|day|week|month|year)s?$`)

// relativeAgeUnit maps a matched unit to its approximate duration. Month and
// year are nominal averages -- `updated_time` is itself an approximation, so
// carrying calendar precision here would be false rigor.
var relativeAgeUnit = map[string]time.Duration{
	"minute": time.Minute,
	"hour":   time.Hour,
	"day":    24 * time.Hour,
	"week":   7 * 24 * time.Hour,
	"month":  30 * 24 * time.Hour,
	"year":   365 * 24 * time.Hour,
}

// parseRelativeAge converts amazon.jobs' relative `updated_time` string into an
// approximate age. Reports false for empty or unrecognized input.
func parseRelativeAge(s string) (time.Duration, bool) {
	s = strings.ToLower(strings.TrimSpace(collapseSpaces.ReplaceAllString(s, " ")))
	m := relativeAgeRe.FindStringSubmatch(s)
	if m == nil {
		return 0, false
	}
	n := int64(1) // the "a"/"an" branch has no digits and means one unit
	if m[1] != "" {
		parsed, err := strconv.ParseInt(m[1], 10, 64)
		if err != nil {
			return 0, false
		}
		n = parsed
	}
	unit, ok := relativeAgeUnit[m[2]]
	if !ok {
		return 0, false
	}
	return time.Duration(n) * unit, true
}

// Thresholds for the updated-vs-posted divergence marker. A row is marked when
// it was posted longer ago than divergencePostedAge but reports an
// `updated_time` newer than divergenceUpdatedAge.
const (
	divergencePostedAge  = 14 * 24 * time.Hour
	divergenceUpdatedAge = 48 * time.Hour
)

// updatedDiverged reports whether a job's `updated_time` looks dramatically
// fresher than its `posted_date`.
//
// This exists because `updated_time` does not mean "posted": it tracks the last
// touch of any kind, and amazon.jobs re-indexes its backlog continuously. In a
// 1000-req live sample, 514 reqs were posted more than 14 days ago and every
// one of them reported an `updated_time` inside 48 hours -- including a req
// posted in August 2025 that read "about 21 hours". Anyone optimizing for
// "apply the fastest" who sorts or filters on `updated_time` is therefore
// reading a re-index clock, not a posting clock. Marking the row is how the
// human-facing output stops implying freshness the data does not support.
func updatedDiverged(j Job, now time.Time) bool {
	posted, ok := parsePostedDate(j.PostedDate)
	if !ok {
		return false
	}
	age, ok := parseRelativeAge(j.UpdatedTime)
	if !ok {
		return false
	}
	return now.Sub(posted) > divergencePostedAge && age < divergenceUpdatedAge
}

// annotateFreshness stamps UpdatedDiverged on every job in place, against a
// single "now" so a slow page scan cannot mark two identical rows differently.
// Curated commands call this on their result set just before rendering.
func annotateFreshness(jobs []Job) {
	now := time.Now()
	for i := range jobs {
		jobs[i].UpdatedDiverged = updatedDiverged(jobs[i], now)
	}
}

// startOfDay truncates to local midnight. Used to make --posted-within
// inclusive by date: the cutoff day itself always counts as inside the window.
func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// postedWithinCutoff returns the inclusive date floor for a --posted-within
// window: the start of the day that `d` before now falls on.
//
// Anchoring to start-of-day is what makes the flag's contract honest given a
// day-granular source field. "7d" means "posted on or after (today - 7 days)",
// counted in whole dates, so it cannot silently drop a req posted earlier in
// the day seven days ago just because the clock time is off by hours.
func postedWithinCutoff(now time.Time, d time.Duration) time.Time {
	return startOfDay(now.Add(-d))
}
