// Copyright 2026 Paul Bockewitz and contributors. Licensed under Apache-2.0. See LICENSE.

// Store-reading helpers shared by the hand-built analytics commands. Not
// generated — safe to edit. All reads come from the local SQLite mirror the
// `sync` command populates (user, day_watched_time, external_time, videos,
// playlist), so these commands work fully offline.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"math"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/education/dreaming/internal/store"
)

type dreamingUser struct {
	WatchTime        int64  `json:"watch_time_seconds"`
	ExternalSeconds  int64  `json:"external_time_seconds"`
	DailyGoalSeconds int64  `json:"daily_goal_seconds"`
	Email            string `json:"email"`
}

// TotalSeconds is on-platform watch time plus logged external input.
func (u dreamingUser) TotalSeconds() int64 { return u.WatchTime + u.ExternalSeconds }

// TotalHours is the cumulative-input master metric.
func (u dreamingUser) TotalHours() float64 { return hoursFromSeconds(u.TotalSeconds()) }

// loadUser reads the single cached user row. ok=false means `sync` hasn't run.
func loadUser(ctx context.Context, db *store.Store) (dreamingUser, bool, error) {
	var u dreamingUser
	var email sql.NullString
	// watch_time arrives as a float from the API (fractional seconds); SQLite stores
	// it as REAL even though the schema declares INTEGER. Use NullFloat64 to avoid
	// a "converting float64 to int64" scan error, then truncate to whole seconds.
	var wt sql.NullFloat64
	var ext, goal sql.NullInt64
	row := db.DB().QueryRowContext(ctx,
		`SELECT watch_time, external_time_seconds, daily_goal_seconds, email FROM "user" LIMIT 1`)
	err := row.Scan(&wt, &ext, &goal, &email)
	if err == sql.ErrNoRows {
		return u, false, nil
	}
	if err != nil {
		return u, false, err
	}
	u.WatchTime = int64(wt.Float64)
	u.ExternalSeconds = ext.Int64
	u.DailyGoalSeconds = goal.Int64
	u.Email = email.String
	return u, true, nil
}

// ensureUser returns the cached user stats, fetching and caching /user live
// when the store has no user row (the single-object /user endpoint is not
// batch-syncable, so `sync` doesn't populate it).
func ensureUser(ctx context.Context, flags *rootFlags, db *store.Store) (dreamingUser, bool, error) {
	u, ok, err := loadUser(ctx, db)
	if err != nil {
		return dreamingUser{}, false, err
	}
	if ok {
		return u, true, nil
	}
	c, err := flags.newClient()
	if err != nil {
		return dreamingUser{}, false, err
	}
	raw, err := c.Get(ctx, "/user", nil)
	if err != nil {
		return dreamingUser{}, false, err
	}
	var env struct {
		User json.RawMessage `json:"user"`
	}
	inner := raw
	if json.Unmarshal(raw, &env) == nil && len(env.User) > 0 {
		inner = env.User
	}
	if err := db.UpsertUser(inner); err != nil {
		return dreamingUser{}, false, err
	}
	return loadUser(ctx, db)
}

type daySeconds struct {
	Date    string `json:"date"`
	Seconds int64  `json:"seconds"`
}

// loadDailyInput merges per-day on-platform watch time with logged external
// time into one input-seconds-per-date series, sorted by date ascending.
func loadDailyInput(ctx context.Context, db *store.Store) ([]daySeconds, error) {
	totals := map[string]int64{}
	add := func(query string) error {
		rows, err := db.DB().QueryContext(ctx, query)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var d sql.NullString
			var s sql.NullInt64
			if err := rows.Scan(&d, &s); err != nil {
				return err
			}
			date := d.String
			if len(date) >= 10 {
				date = date[:10]
			}
			if date == "" {
				continue
			}
			totals[date] += s.Int64
		}
		return rows.Err()
	}
	if err := add(`SELECT date, time_seconds FROM "day_watched_time"`); err != nil {
		return nil, err
	}
	if err := add(`SELECT date, time_seconds FROM "external_time"`); err != nil {
		return nil, err
	}
	out := make([]daySeconds, 0, len(totals))
	for d, s := range totals {
		out = append(out, daySeconds{Date: d, Seconds: s})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date < out[j].Date })
	return out, nil
}

// recentAverageSeconds returns the mean input-seconds/day over the last n
// calendar days (denominator is always n, not just active days), used as a
// pace estimate. The window is bounded by a calendar-date cutoff rather than an
// entry count, because series holds one entry per *active* day — slicing by
// count would span more than n calendar dates for a user with rest days and
// inflate the pace. Dividing by n gives the true daily average including rest days.
func recentAverageSeconds(series []daySeconds, days int) float64 {
	if len(series) == 0 || days <= 0 {
		return 0
	}
	cutoff := time.Now().AddDate(0, 0, -days).Format("2006-01-02")
	var sum int64
	for _, d := range series {
		if d.Date >= cutoff {
			sum += d.Seconds
		}
	}
	return float64(sum) / float64(days)
}

// streak computes current and longest consecutive-day input streaks.
func streaks(series []daySeconds) (current, longest int) {
	return streaksAt(series, time.Now())
}

func streaksAt(series []daySeconds, now time.Time) (current, longest int) {
	if len(series) == 0 {
		return 0, 0
	}
	parse := func(s string) (time.Time, bool) {
		t, err := time.ParseInLocation("2006-01-02", s, now.Location())
		return t, err == nil
	}
	gapDays := func(later, earlier time.Time) int {
		return int(math.Round(later.Sub(earlier).Hours() / 24))
	}
	var dates []time.Time
	for _, d := range series {
		if d.Seconds <= 0 {
			continue
		}
		if t, ok := parse(d.Date); ok {
			dates = append(dates, t)
		}
	}
	if len(dates) == 0 {
		return 0, 0
	}
	sort.Slice(dates, func(i, j int) bool { return dates[i].Before(dates[j]) })
	longest = 1
	run := 1
	for i := 1; i < len(dates); i++ {
		gap := gapDays(dates[i], dates[i-1])
		if gap == 1 {
			run++
		} else if gap == 0 {
			continue
		} else {
			run = 1
		}
		if run > longest {
			longest = run
		}
	}
	// Current streak: count back from the most recent active day if it is
	// today or yesterday (a one-day grace so an unsynced "today" doesn't reset).
	last := dates[len(dates)-1]
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	gapToToday := gapDays(today, last)
	if gapToToday > 1 {
		return 0, longest
	}
	current = 1
	for i := len(dates) - 1; i > 0; i-- {
		gap := gapDays(dates[i], dates[i-1])
		if gap == 1 {
			current++
		} else if gap == 0 {
			continue
		} else {
			break
		}
	}
	return current, longest
}
