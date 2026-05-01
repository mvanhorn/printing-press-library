package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/producthunt/internal/store"
	"github.com/spf13/cobra"
)

type calendarDayPayload struct {
	Date  string        `json:"date"`
	Day   string        `json:"day"`
	Count int           `json:"count"`
	Posts []postPayload `json:"posts,omitempty"`
}

type calendarPayload struct {
	WindowStart string               `json:"window_start"`
	WindowEnd   string               `json:"window_end"`
	Days        []calendarDayPayload `json:"days"`
}

func newCalendarCmd(flags *rootFlags) *cobra.Command {
	var week string
	var days int
	var includePosts bool
	var dbPath string

	cmd := &cobra.Command{
		Use:   "calendar",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Week-at-a-glance: which products were featured each day",
		Long: `Group the posts you've synced by their first-seen day and render a
calendar-shaped payload. Requires first_seen data, which accumulates as
you sync over multiple days.

--week accepts ISO-week strings like '2026-W16' or 'this-week' / 'last-week'.
--days N produces a rolling N-day window ending today (default 7).

Use --include-posts to embed each day's list of posts; otherwise output is
aggregate counts only.`,
		Example: `  # Last 7 days, counts only
  producthunt-pp-cli calendar

  # Specific ISO week with embedded posts
  producthunt-pp-cli calendar --week 2026-W16 --include-posts --json

  # Last 30 days, agent-narrow
  producthunt-pp-cli calendar --days 30 --agent --select 'days.date,days.count'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			autoWarm(flags, dbPath)
			db, err := openStore(dbPath)
			if err != nil {
				return configErr(err)
			}
			defer db.Close()

			var windowStart, windowEnd time.Time
			switch {
			case week != "":
				ws, we, err := parseWeekOrAlias(week)
				if err != nil {
					return usageErr(err)
				}
				windowStart, windowEnd = ws, we
			case days > 0:
				windowEnd = time.Now()
				windowStart = windowEnd.AddDate(0, 0, -days+1)
			default:
				windowEnd = time.Now()
				windowStart = windowEnd.AddDate(0, 0, -6)
			}

			// Normalize to midnight (local tz) to avoid partial-day weirdness.
			windowStart = startOfDay(windowStart)
			windowEnd = endOfDay(windowEnd)

			posts, err := db.ListPosts(store.ListPostsOpts{
				Since:     windowStart,
				Until:     windowEnd,
				SortField: "published",
				SortDesc:  false,
				Limit:     0,
			})
			if err != nil {
				return apiErr(err)
			}

			byDay := map[string][]postPayload{}
			for _, p := range posts {
				dayKey := ""
				if !p.PublishedAt.IsZero() {
					dayKey = p.PublishedAt.Local().Format("2006-01-02")
				} else {
					dayKey = p.FirstSeenAt.Local().Format("2006-01-02")
				}
				byDay[dayKey] = append(byDay[dayKey], postPayloadOf(p))
			}

			// Emit every day in the window, including zero-count days, so the
			// caller sees a complete N-day shape rather than silent gaps.
			out := calendarPayload{
				WindowStart: windowStart.UTC().Format(time.RFC3339),
				WindowEnd:   windowEnd.UTC().Format(time.RFC3339),
			}
			cursor := startOfDay(windowStart)
			for !cursor.After(windowEnd) {
				k := cursor.Format("2006-01-02")
				posts := byDay[k]
				day := calendarDayPayload{
					Date:  k,
					Day:   cursor.Format("Mon"),
					Count: len(posts),
				}
				if includePosts {
					day.Posts = posts
				}
				out.Days = append(out.Days, day)
				cursor = cursor.AddDate(0, 0, 1)
			}
			buf, _ := json.Marshal(out)
			return printOutputWithFlags(cmd.OutOrStdout(), buf, flags)
		},
	}

	cmd.Flags().StringVar(&week, "week", "", "ISO week like 2026-W16, or 'this-week'/'last-week'")
	cmd.Flags().IntVar(&days, "days", 0, "Rolling window: last N days ending today (default 7 when --week is not set)")
	cmd.Flags().BoolVar(&includePosts, "include-posts", false, "Embed each day's list of posts (default: counts only)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to local SQLite store")
	return cmd
}

// parseWeekOrAlias accepts 'YYYY-Www' ISO week notation or the aliases
// 'this-week' / 'last-week'. Returns the [start, end] window of the week
// in local time (Monday-Sunday).
func parseWeekOrAlias(v string) (time.Time, time.Time, error) {
	v = strings.TrimSpace(strings.ToLower(v))
	now := time.Now()
	switch v {
	case "this-week", "current", "":
		return weekWindowContaining(now)
	case "last-week", "previous":
		return weekWindowContaining(now.AddDate(0, 0, -7))
	}
	// Parse YYYY-Www
	parts := strings.SplitN(v, "-w", 2)
	if len(parts) != 2 {
		return time.Time{}, time.Time{}, fmt.Errorf("expected YYYY-Www (e.g. 2026-W16), got %q", v)
	}
	yearStr, weekStr := parts[0], parts[1]
	year, err := strconv.Atoi(yearStr)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("bad year %q", yearStr)
	}
	week, err := strconv.Atoi(weekStr)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("bad week %q", weekStr)
	}
	// Find the Monday of ISO week `week` in `year`.
	// Jan 4 is always in week 1; find that Monday then add week-1 weeks.
	jan4 := time.Date(year, time.January, 4, 0, 0, 0, 0, time.Local)
	// Offset back to Monday
	daysBack := int(jan4.Weekday()) - int(time.Monday)
	if daysBack < 0 {
		daysBack += 7
	}
	monday := jan4.AddDate(0, 0, -daysBack+(week-1)*7)
	return monday, monday.AddDate(0, 0, 7).Add(-time.Nanosecond), nil
}

func weekWindowContaining(t time.Time) (time.Time, time.Time, error) {
	daysBack := int(t.Weekday()) - int(time.Monday)
	if daysBack < 0 {
		daysBack += 7
	}
	monday := startOfDay(t.AddDate(0, 0, -daysBack))
	return monday, monday.AddDate(0, 0, 7).Add(-time.Nanosecond), nil
}

func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}
func endOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, int(time.Second-time.Nanosecond), t.Location())
}
