// Copyright 2026 Micah Baldwin and contributors. Licensed under Apache-2.0. See LICENSE.
package lrcat

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// DayCount is one calendar day's shooting summary.
type DayCount struct {
	Date   string `json:"date"`
	Photos int64  `json:"photos"`
	Picks  int64  `json:"picks"`
}

// StreakReport summarizes day-level shooting coverage for a range.
type StreakReport struct {
	Since         string     `json:"since"`
	Until         string     `json:"until"`
	DaysWithShots int        `json:"days_with_shots"`
	TotalDays     int        `json:"total_days"`
	CurrentStreak int        `json:"current_streak"`
	LongestStreak int        `json:"longest_streak"`
	Gaps          []string   `json:"gaps"`
	Days          []DayCount `json:"days,omitempty"`
}

const dayFormat = "2006-01-02"

// dayCounts returns per-day photo and pick counts within [since, until].
func (c *Catalog) dayCounts(ctx context.Context, since, until string) (map[string]DayCount, error) {
	rows, err := c.DB.QueryContext(ctx, `
		SELECT substr(captureTime,1,10) d, count(*),
		       sum(CASE WHEN pick = 1 THEN 1 ELSE 0 END)
		FROM Adobe_images
		WHERE captureTime IS NOT NULL
		  AND substr(captureTime,1,10) >= ? AND substr(captureTime,1,10) <= ?
		GROUP BY d`, since, until)
	if err != nil {
		return nil, err
	}
	out := map[string]DayCount{}
	for rows.Next() {
		var d DayCount
		if err := rows.Scan(&d.Date, &d.Photos, &d.Picks); err != nil {
			_ = rows.Close()
			return nil, err
		}
		out[d.Date] = d
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	return out, rows.Close()
}

// computeStreaks turns a shot-day set into streak/gap accounting. Pure logic,
// separated for testing. today is the anchor for the current streak.
func computeStreaks(shot map[string]DayCount, since, until, today time.Time) StreakReport {
	rep := StreakReport{
		Since: since.Format(dayFormat),
		Until: until.Format(dayFormat),
		Gaps:  []string{},
	}
	run := 0
	for d := since; !d.After(until); d = d.AddDate(0, 0, 1) {
		key := d.Format(dayFormat)
		rep.TotalDays++
		if _, ok := shot[key]; ok {
			rep.DaysWithShots++
			run++
			if run > rep.LongestStreak {
				rep.LongestStreak = run
			}
		} else {
			run = 0
			rep.Gaps = append(rep.Gaps, key)
		}
	}
	// Current streak: count back from today (or yesterday if today is empty,
	// so an unfinished day doesn't read as a broken streak).
	anchor := today
	if _, ok := shot[anchor.Format(dayFormat)]; !ok {
		anchor = anchor.AddDate(0, 0, -1)
	}
	for d := anchor; !d.Before(since); d = d.AddDate(0, 0, -1) {
		if _, ok := shot[d.Format(dayFormat)]; !ok {
			break
		}
		rep.CurrentStreak++
	}
	return rep
}

// Streaks reports current/longest daily-shooting streaks and gap days.
func (c *Catalog) Streaks(ctx context.Context, since, until string, includeDays bool) (*StreakReport, error) {
	now := time.Now()
	u := now
	if until != "" {
		t, err := time.ParseInLocation(dayFormat, until, time.Local)
		if err != nil {
			return nil, fmt.Errorf("--until: expected YYYY-MM-DD, got %q", until)
		}
		u = t
	}
	s := u.AddDate(0, 0, -364)
	if since != "" {
		t, err := time.ParseInLocation(dayFormat, since, time.Local)
		if err != nil {
			return nil, fmt.Errorf("--since: expected YYYY-MM-DD, got %q", since)
		}
		s = t
	}
	if s.After(u) {
		return nil, fmt.Errorf("--since %s is after --until %s", s.Format(dayFormat), u.Format(dayFormat))
	}
	shot, err := c.dayCounts(ctx, s.Format(dayFormat), u.Format(dayFormat))
	if err != nil {
		return nil, err
	}
	rep := computeStreaks(shot, s, u, now)
	if includeDays {
		for d := s; !d.After(u); d = d.AddDate(0, 0, 1) {
			if dc, ok := shot[d.Format(dayFormat)]; ok {
				rep.Days = append(rep.Days, dc)
			}
		}
	}
	return &rep, nil
}

// PickOfDay returns at most one photo for the given day using the selection
// ladder: flagged pick first, then highest rating, then most recently touched.
func (c *Catalog) PickOfDay(ctx context.Context, date string) (*Photo, error) {
	if _, err := time.ParseInLocation(dayFormat, date, time.Local); err != nil {
		return nil, fmt.Errorf("--date: expected YYYY-MM-DD, got %q", date)
	}
	q := photoSelect + `
		WHERE substr(i.captureTime,1,10) = ?
		ORDER BY (i.pick = 1) DESC, i.rating DESC NULLS LAST, i.touchTime DESC, i.id_local DESC
		LIMIT 1`
	rows, err := c.DB.QueryContext(ctx, q, date)
	if err != nil {
		return nil, fmt.Errorf("querying pick of day: %w", err)
	}
	photos, err := scanPhotos(rows)
	if err != nil {
		return nil, err
	}
	if len(photos) == 0 {
		return nil, nil
	}
	return &photos[0], nil
}

// PickOfDayRange returns one pick per day for each day in [since, until] that
// has photos. Days without photos are omitted.
func (c *Catalog) PickOfDayRange(ctx context.Context, since, until string) ([]Photo, error) {
	s, err := time.ParseInLocation(dayFormat, since, time.Local)
	if err != nil {
		return nil, fmt.Errorf("--since: expected YYYY-MM-DD, got %q", since)
	}
	u, err := time.ParseInLocation(dayFormat, until, time.Local)
	if err != nil {
		return nil, fmt.Errorf("--until: expected YYYY-MM-DD, got %q", until)
	}
	if s.After(u) {
		return nil, fmt.Errorf("--since %s is after --until %s", since, until)
	}
	shot, err := c.dayCounts(ctx, since, until)
	if err != nil {
		return nil, err
	}
	out := make([]Photo, 0)
	for d := s; !d.After(u); d = d.AddDate(0, 0, 1) {
		key := d.Format(dayFormat)
		if _, ok := shot[key]; !ok {
			continue
		}
		p, err := c.PickOfDay(ctx, key)
		if err != nil {
			return nil, err
		}
		if p != nil {
			out = append(out, *p)
		}
	}
	return out, nil
}

// YearOnDay is one year's photos for a fixed calendar date.
type YearOnDay struct {
	Year   string `json:"year"`
	Photos int64  `json:"photos"`
	Best   *Photo `json:"best,omitempty"`
}

// OnThisDay returns per-year summaries for a calendar month/day across all years.
func (c *Catalog) OnThisDay(ctx context.Context, month, day int) ([]YearOnDay, error) {
	if month < 1 || month > 12 || day < 1 || day > 31 {
		return nil, fmt.Errorf("invalid calendar date: month %d day %d", month, day)
	}
	md := fmt.Sprintf("%02d-%02d", month, day)
	rows, err := c.DB.QueryContext(ctx, `
		SELECT substr(captureTime,1,4) y, count(*)
		FROM Adobe_images
		WHERE captureTime IS NOT NULL AND substr(captureTime,6,5) = ?
		GROUP BY y ORDER BY y DESC`, md)
	if err != nil {
		return nil, err
	}
	years := make([]YearOnDay, 0)
	for rows.Next() {
		var y YearOnDay
		if err := rows.Scan(&y.Year, &y.Photos); err != nil {
			_ = rows.Close()
			return nil, err
		}
		years = append(years, y)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	// Resolve the best image per year after the row set is drained.
	for i := range years {
		p, err := c.PickOfDay(ctx, fmt.Sprintf("%s-%s", years[i].Year, md))
		if err != nil {
			return nil, err
		}
		years[i].Best = p
	}
	return years, nil
}

// ProjectReport is progress accounting for a fixed-length, collection-backed project.
type ProjectReport struct {
	Collection      string   `json:"collection"`
	Target          int      `json:"target"`
	Start           string   `json:"start"`
	DaysWithPhotos  int      `json:"days_with_photos"`
	Note            string   `json:"note,omitempty"`
	PercentComplete float64  `json:"percent_complete"`
	MissedDays      []string `json:"missed_days"`
	ProjectedFinish string   `json:"projected_finish,omitempty"`
	Complete        bool     `json:"complete"`
}

// Project reports progress for a collection-backed daily project of a fixed
// target length. start defaults to the collection's first capture day.
func (c *Catalog) Project(ctx context.Context, collection string, target int, start string) (*ProjectReport, error) {
	if target <= 0 {
		return nil, fmt.Errorf("--target must be a positive day count")
	}
	// Resolve the substring to exactly one collection so two similarly named
	// projects can never merge into one report. Exact match wins; a unique
	// substring match is accepted; multiple matches are an error.
	nameRows, err := c.DB.QueryContext(ctx, `
		SELECT id_local, name FROM AgLibraryCollection
		WHERE systemOnly = 0 AND name IS NOT NULL AND lower(name) LIKE lower(?)
		ORDER BY name, id_local`, "%"+collection+"%")
	if err != nil {
		return nil, err
	}
	type collMatch struct {
		id   int64
		name string
	}
	matches := make([]collMatch, 0)
	for nameRows.Next() {
		var m collMatch
		if err := nameRows.Scan(&m.id, &m.name); err != nil {
			_ = nameRows.Close()
			return nil, err
		}
		matches = append(matches, m)
	}
	if err := nameRows.Err(); err != nil {
		_ = nameRows.Close()
		return nil, err
	}
	if err := nameRows.Close(); err != nil {
		return nil, err
	}
	// Resolve to exactly one collection row (id, not name) so two projects can
	// never merge — neither via similar names nor via duplicate names under
	// different collection sets.
	exact := make([]collMatch, 0)
	for _, m := range matches {
		if strings.EqualFold(m.name, collection) {
			exact = append(exact, m)
		}
	}
	var resolvedID int64 = -1
	resolved := collection
	switch {
	case len(exact) == 1:
		resolvedID, resolved = exact[0].id, exact[0].name
	case len(exact) > 1:
		return nil, fmt.Errorf("%d collections share the exact name %q (under different collection sets); rename one so the project is unambiguous", len(exact), collection)
	case len(matches) == 1:
		resolvedID, resolved = matches[0].id, matches[0].name
	case len(matches) > 1:
		names := make([]string, 0, len(matches))
		for _, m := range matches {
			names = append(names, m.name)
		}
		return nil, fmt.Errorf("--collection %q matches %d collections (%s); use the exact name", collection, len(matches), strings.Join(names, ", "))
	}
	rows, err := c.DB.QueryContext(ctx, `
		SELECT DISTINCT substr(i.captureTime,1,10) d
		FROM Adobe_images i
		JOIN AgLibraryCollectionImage ci ON ci.image = i.id_local
		WHERE ci.collection = ? AND i.captureTime IS NOT NULL
		ORDER BY d`, resolvedID)
	if err != nil {
		return nil, err
	}
	days := make([]string, 0)
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			_ = rows.Close()
			return nil, err
		}
		days = append(days, d)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if len(days) == 0 {
		// Empty local result, not a failure: report zero progress with a note
		// so agents can distinguish "no such collection" from real progress.
		return &ProjectReport{
			Collection: resolved,
			Target:     target,
			Start:      start,
			MissedDays: []string{},
			Note:       fmt.Sprintf("no photos with capture dates found in a collection matching %q; run 'collections' to see names", collection),
		}, nil
	}
	if start == "" {
		start = days[0]
	}
	s, err := time.ParseInLocation(dayFormat, start, time.Local)
	if err != nil {
		return nil, fmt.Errorf("--start: expected YYYY-MM-DD, got %q", start)
	}
	shot := map[string]bool{}
	for _, d := range days {
		shot[d] = true
	}
	rep := &ProjectReport{Collection: resolved, Target: target, Start: start, MissedDays: []string{}}
	today := time.Now()
	todayKey := today.Format(dayFormat)
	for d := s; !d.After(today); d = d.AddDate(0, 0, 1) {
		key := d.Format(dayFormat)
		if shot[key] {
			rep.DaysWithPhotos++
		} else if key != todayKey {
			// An unfinished today is not yet a miss.
			rep.MissedDays = append(rep.MissedDays, key)
		}
		if rep.DaysWithPhotos >= target {
			break
		}
	}
	rep.PercentComplete = float64(rep.DaysWithPhotos) / float64(target) * 100
	rep.Complete = rep.DaysWithPhotos >= target
	if !rep.Complete {
		remaining := target - rep.DaysWithPhotos
		// Today can still be shot if it isn't yet, so the earliest finish
		// is one day sooner in that case.
		if !shot[todayKey] {
			remaining--
		}
		rep.ProjectedFinish = today.AddDate(0, 0, remaining).Format(dayFormat)
	}
	return rep, nil
}
