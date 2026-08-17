// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/tiimo/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/tiimo/internal/store"
)

// tiimoTimeLayout is the naive-local timestamp shape the Tiimo API emits and
// accepts: no timezone suffix, no fractional seconds. Activities also carry
// dedicated *Utc and *Local variants plus an offset, but every read path in
// this CLI works from the naive local values because that is what the app's
// own timeline is laid out against.
const tiimoTimeLayout = "2006-01-02T15:04:05"

// tiimoDateLayout is the day-granularity form used by the API's fromDate and
// toDate query parameters and by every --from / --to flag in this CLI.
const tiimoDateLayout = "2006-01-02"

// activityRow is one row of the local `activities` mirror. Every field is
// scanned NULL-safely: the Tiimo API omits or nulls most optional fields
// (an activity that was never started has no actual times, a one-off has no
// recurrence), and a bare-string scan would error and silently drop the row
// inside the usual `for rows.Next()` loop.
type activityRow struct {
	ActivityID      string `json:"activity_id"`
	Title           string `json:"title"`
	StartTime       string `json:"start_time"`
	EndTime         string `json:"end_time"`
	Duration        int    `json:"duration"`
	StartTimeActual string `json:"start_time_actual,omitempty"`
	EndTimeActual   string `json:"end_time_actual,omitempty"`
	DurationActual  int    `json:"duration_actual,omitempty"`
	DurationPaused  int    `json:"duration_paused,omitempty"`
	CompletedAt     string `json:"completed_at,omitempty"`
	IsAllDay        bool   `json:"is_all_day"`
	IsRepeating     bool   `json:"is_repeating"`
	RecurrenceType  string `json:"recurrence_type,omitempty"`
	IsReadOnly      bool   `json:"is_read_only"`
	Origin          string `json:"origin,omitempty"`
	IconID          string `json:"icon_id,omitempty"`

	// GroupingLabel is the app's own time-of-day bucket (Morning, Afternoon,
	// Evening, Anytime). It is nested under `grouping` in the API payload
	// rather than promoted to its own column, so it is parsed out of the
	// stored blob.
	GroupingLabel string `json:"grouping_label,omitempty"`
	// Checklist is the activity's nested step list. Tiimo has no standalone
	// checklist resource -- that endpoint 404s -- so steps only ever arrive
	// embedded in their parent activity.
	Checklist []checklistItem `json:"checklist,omitempty"`
}

// checklistItem is one step of an activity's nested checklist.
type checklistItem struct {
	ChecklistItemID string `json:"checklist_item_id"`
	Title           string `json:"title"`
	IsChecked       bool   `json:"is_checked"`
	CheckedAt       string `json:"checked_at,omitempty"`
	Index           int    `json:"index"`
}

// activityBlob is the subset of the stored activity JSON carrying nested
// structures that the flattened domain table drops.
type activityBlob struct {
	Grouping struct {
		GroupingLabel string `json:"groupingLabel"`
	} `json:"grouping"`
	Checklist *struct {
		ChecklistItems []struct {
			ChecklistItemID string  `json:"checklistItemId"`
			Title           string  `json:"title"`
			IsChecked       bool    `json:"isChecked"`
			CheckedAt       *string `json:"checkedAt"`
			Index           int     `json:"index"`
		} `json:"checklistItems"`
	} `json:"checklist"`
}

// bucketForHour derives a time-of-day bucket when the API supplied no
// grouping label. Boundaries mirror the app's own Morning/Afternoon/Evening
// split.
func bucketForHour(h int) string {
	switch {
	case h < 12:
		return "Morning"
	case h < 17:
		return "Afternoon"
	default:
		return "Evening"
	}
}

// Bucket returns the activity's time-of-day bucket, preferring the label the
// API supplied and falling back to its start hour.
func (a activityRow) Bucket() string {
	if a.GroupingLabel != "" {
		return a.GroupingLabel
	}
	if a.IsAllDay {
		return "Anytime"
	}
	if s, ok := a.Start(); ok {
		return bucketForHour(s.Hour())
	}
	return "Anytime"
}

// Day returns the calendar date the activity is scheduled on, derived from
// its start time. Returns "" when the start time is unparseable.
func (a activityRow) Day() string {
	if len(a.StartTime) >= 10 {
		return a.StartTime[:10]
	}
	return ""
}

// Start parses the planned start timestamp.
func (a activityRow) Start() (time.Time, bool) { return parseTiimoTime(a.StartTime) }

// End returns the planned end timestamp, falling back to start+duration when
// the API left endTime empty (which it does for duration-only activities).
func (a activityRow) End() (time.Time, bool) {
	if t, ok := parseTiimoTime(a.EndTime); ok {
		return t, true
	}
	if s, ok := a.Start(); ok && a.Duration > 0 {
		return s.Add(time.Duration(a.Duration) * time.Second), true
	}
	return time.Time{}, false
}

// Completed reports whether the activity was marked done.
func (a activityRow) Completed() bool { return strings.TrimSpace(a.CompletedAt) != "" }

// ClockScheduled reports whether the activity occupies a real position on the
// clock, as opposed to floating inside a time-of-day bucket.
//
// Tiimo stores bucket-scheduled activities ("sometime this morning") with a
// start time of exactly midnight and a grouping label; only the label and the
// duration are meaningful. Treating those as real spans makes every activity
// on a day collide with every other -- a single ordinary day produced 91
// bogus overlap pairs before this existed -- and makes free-time arithmetic
// meaningless. Commands that reason about clock position must filter on this.
func (a activityRow) ClockScheduled() bool {
	if a.IsAllDay {
		return false
	}
	s, ok := a.Start()
	if !ok {
		return false
	}
	if s.Hour() == 0 && s.Minute() == 0 && s.Second() == 0 {
		// Midnight is Tiimo's "no fixed time" sentinel. An activity genuinely
		// planned for midnight is vanishingly rare and indistinguishable here;
		// treating it as unscheduled is the safer error.
		return false
	}
	return true
}

// Started reports whether there is evidence the activity was actually run.
//
// durationActual alone is NOT evidence: Tiimo pre-populates it with the
// planned duration for every occurrence, including ones never touched, so
// keying off it marks all 1190 occurrences "started" and reports a confident
// zero drift. Real evidence is a completion stamp, recorded pause time, or an
// actual start that diverges from the planned start.
func (a activityRow) Started() bool {
	if a.Completed() || a.DurationPaused > 0 {
		return true
	}
	actual, okA := parseTiimoTime(a.StartTimeActual)
	planned, okP := a.Start()
	if okA && okP && !actual.Equal(planned) {
		return true
	}
	if okA && !okP {
		return true
	}
	// An actual duration that diverges from the plan is also real evidence;
	// equality is the pre-filled default and proves nothing.
	return a.DurationActual > 0 && a.DurationActual != a.Duration
}

// parseTiimoTime parses a Tiimo naive-local timestamp. It tolerates a trailing
// "Z" or offset because the *Utc variants of the same fields carry one.
func parseTiimoTime(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	// Parse in the LOCAL zone, not UTC.
	//
	// Tiimo timestamps are naive wall-clock times the user planned against,
	// with no offset. time.Parse would tag them UTC, while every window bound
	// in this CLI is built with time.Local -- so on any machine not at UTC the
	// comparison is silently skewed by the offset. That made `today` and
	// `gaps` return nothing on a day with fourteen activities, because Tiimo
	// stores bucket-scheduled items at T00:00:00 and they fell off the front
	// of the window.
	for _, layout := range []string{tiimoTimeLayout, tiimoDateLayout} {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			// The API uses year 1 as a null sentinel for startTimeLocal.
			if t.Year() < 2 {
				return time.Time{}, false
			}
			return t, true
		}
	}
	// RFC3339 values carry their own offset, so honor it and convert to local
	// for consistent comparison against the window bounds.
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		if t.Year() < 2 {
			return time.Time{}, false
		}
		return t.In(time.Local), true
	}
	return time.Time{}, false
}

// parseTiimoDate parses a YYYY-MM-DD flag value.
func parseTiimoDate(s string) (time.Time, error) {
	t, err := time.Parse(tiimoDateLayout, strings.TrimSpace(s))
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date %q: want YYYY-MM-DD", s)
	}
	return t, nil
}

// dateWindow resolves the common --from/--to/--days flag trio into an
// inclusive day range. Exactly one of (from,to) or days need be supplied;
// when nothing is supplied the window is today only.
//
// days is accepted as a loose duration ("30d", "4w", "72h") via
// cliutil.ParseDurationLoose, because Go's time.ParseDuration rejects the
// day and week suffixes that users and agents reasonably expect.
func dateWindow(from, to, days string) (time.Time, time.Time, error) {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)

	if strings.TrimSpace(days) != "" {
		d, err := cliutil.ParseDurationLoose(strings.TrimSpace(days))
		if err != nil {
			// Bare integers mean days, which is the shape the --days flag
			// advertises in its own help text.
			var n int
			if _, scanErr := fmt.Sscanf(strings.TrimSpace(days), "%d", &n); scanErr == nil && n > 0 {
				d = time.Duration(n) * 24 * time.Hour
			} else {
				return time.Time{}, time.Time{}, usageErr(fmt.Errorf("invalid --days %q: want 30, 30d, or 4w", days))
			}
		}
		if d <= 0 {
			return time.Time{}, time.Time{}, usageErr(fmt.Errorf("--days must be positive"))
		}
		return today.Add(-d), today.AddDate(0, 0, 1).Add(-time.Second), nil
	}

	start := today
	if strings.TrimSpace(from) != "" {
		t, err := parseTiimoDate(from)
		if err != nil {
			return time.Time{}, time.Time{}, usageErr(err)
		}
		start = t
	}
	end := start
	if strings.TrimSpace(to) != "" {
		t, err := parseTiimoDate(to)
		if err != nil {
			return time.Time{}, time.Time{}, usageErr(err)
		}
		end = t
	}
	if end.Before(start) {
		return time.Time{}, time.Time{}, usageErr(fmt.Errorf("--to (%s) is before --from (%s)", end.Format(tiimoDateLayout), start.Format(tiimoDateLayout)))
	}
	// Make the window inclusive of the whole end day.
	return start, end.AddDate(0, 0, 1).Add(-time.Second), nil
}

// localMirror opens the local SQLite mirror read-only.
//
// The returned bool is false when no mirror exists yet. Callers must treat
// that as an empty-cache state, not an error: emit a valid empty result for
// machine output modes and a sync hint for humans. openLocalMirror never
// creates the database, so a read-only command cannot leave a stray file
// behind on a machine that has never synced.
func openLocalMirror(ctx context.Context, cmd *cobra.Command, flags *rootFlags, dbPath string) (*store.Store, bool, error) {
	if strings.TrimSpace(dbPath) == "" {
		dbPath = defaultDBPath("tiimo-pp-cli")
	}
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return nil, false, nil
	}
	s, err := store.OpenReadOnlyContext(ctx, dbPath)
	if err != nil {
		return nil, false, fmt.Errorf("opening local mirror at %s: %w", dbPath, err)
	}
	// A mirror file that exists but has never been synced produces confident
	// empty analyses -- "14h free every day" reads identically whether the
	// schedule is genuinely clear or the store was simply never filled. Every
	// local read goes through here, so announce it once on stderr and leave
	// stdout parseable. The empty resource type asks "has anything ever been
	// synced", which is the honest question for a shared helper.
	hintIfUnsynced(cmd, s, "")
	return s, true, nil
}

// writeNoMirror emits the standard "run sync first" result. Machine output
// modes get a valid empty payload so an agent parsing the response sees an
// empty set rather than a crash; humans get an actionable hint on stderr.
func writeNoMirror[T any](cmd *cobra.Command, flags *rootFlags, dbPath string, empty []T) error {
	if strings.TrimSpace(dbPath) == "" {
		dbPath = defaultDBPath("tiimo-pp-cli")
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: tiimo-pp-cli sync --resources activities --db %s\n", dbPath, dbPath)
	if !wantsHumanTable(cmd.OutOrStdout(), flags) {
		return printJSONFiltered(cmd.OutOrStdout(), empty, flags)
	}
	return nil
}

// loadActivities reads every mirrored activity whose planned start falls
// inside [from, to].
//
// Rows are filtered in Go rather than SQL on the time bound because
// start_time is stored as the API's naive-local string; comparing those
// lexically works for the common case but breaks on the year-1 null
// sentinel and on rows the API returned without a start time at all.
func loadActivities(ctx context.Context, db *sql.DB, from, to time.Time) ([]activityRow, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT activity_id, title, start_time, end_time, duration,
		       start_time_actual, end_time_actual, duration_actual, duration_paused,
		       completed_at, is_all_day, is_repeating, recurrence_type,
		       is_read_only, origin, icon_id, data
		FROM activities`)
	if err != nil {
		return nil, fmt.Errorf("querying activities: %w", err)
	}

	out := make([]activityRow, 0)
	for rows.Next() {
		var (
			activityID, title                 sql.NullString
			startTime, endTime                sql.NullString
			duration                          sql.NullInt64
			startActual, endActual            sql.NullString
			durationActual, durationPaused    sql.NullInt64
			completedAt                       sql.NullString
			isAllDay, isRepeating, isReadOnly sql.NullBool
			recurrenceType, origin, iconID    sql.NullString
			blob                              sql.NullString
		)
		if err := rows.Scan(&activityID, &title, &startTime, &endTime, &duration,
			&startActual, &endActual, &durationActual, &durationPaused,
			&completedAt, &isAllDay, &isRepeating, &recurrenceType,
			&isReadOnly, &origin, &iconID, &blob); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scanning activity: %w", err)
		}

		// Nested grouping and checklist live only in the stored payload; a
		// malformed blob degrades those two fields rather than dropping the
		// whole row, because the flattened columns are still trustworthy.
		var groupingLabel string
		var steps []checklistItem
		if blob.Valid && blob.String != "" {
			var ab activityBlob
			if err := json.Unmarshal([]byte(blob.String), &ab); err == nil {
				groupingLabel = ab.Grouping.GroupingLabel
				if ab.Checklist != nil {
					steps = make([]checklistItem, 0, len(ab.Checklist.ChecklistItems))
					for _, it := range ab.Checklist.ChecklistItems {
						ci := checklistItem{
							ChecklistItemID: it.ChecklistItemID,
							Title:           it.Title,
							IsChecked:       it.IsChecked,
							Index:           it.Index,
						}
						if it.CheckedAt != nil {
							ci.CheckedAt = *it.CheckedAt
						}
						steps = append(steps, ci)
					}
				}
			}
		}

		out = append(out, activityRow{
			ActivityID:      activityID.String,
			Title:           title.String,
			StartTime:       startTime.String,
			EndTime:         endTime.String,
			Duration:        int(duration.Int64),
			StartTimeActual: startActual.String,
			EndTimeActual:   endActual.String,
			DurationActual:  int(durationActual.Int64),
			DurationPaused:  int(durationPaused.Int64),
			CompletedAt:     completedAt.String,
			IsAllDay:        isAllDay.Bool,
			IsRepeating:     isRepeating.Bool,
			RecurrenceType:  recurrenceType.String,
			IsReadOnly:      isReadOnly.Bool,
			Origin:          origin.String,
			IconID:          iconID.String,
			GroupingLabel:   groupingLabel,
			Checklist:       steps,
		})
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("iterating activities: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("closing activities rows: %w", err)
	}

	filtered := out[:0]
	for _, a := range out {
		s, ok := a.Start()
		if !ok {
			continue
		}
		if s.Before(from) || s.After(to) {
			continue
		}
		filtered = append(filtered, a)
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		return filtered[i].StartTime < filtered[j].StartTime
	})
	return filtered, nil
}

// humanDuration renders a second count the way the Tiimo UI does: "50m",
// "1h 30m", "45s". Durations in this API are always seconds.
func humanDuration(seconds int) string {
	if seconds <= 0 {
		return "0m"
	}
	d := time.Duration(seconds) * time.Second
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	switch {
	case h > 0 && m > 0:
		return fmt.Sprintf("%dh %dm", h, m)
	case h > 0:
		return fmt.Sprintf("%dh", h)
	case m > 0:
		return fmt.Sprintf("%dm", m)
	default:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
}

// writeTiimoResult is the shared tail every local-read novel command uses.
// It routes machine modes through the generated JSON helper (picking up
// --select, --compact, --csv and --quiet) and hands humans a rendering
// callback. Empty slices marshal as [] rather than null because callers
// build them with make(...).
func writeTiimoResult[T any](cmd *cobra.Command, flags *rootFlags, rows []T, human func(w io.Writer)) error {
	if !wantsHumanTable(cmd.OutOrStdout(), flags) {
		return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
	}
	human(cmd.OutOrStdout())
	return nil
}
