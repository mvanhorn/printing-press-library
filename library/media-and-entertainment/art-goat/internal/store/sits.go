// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0.

// art-goat journal storage: sits table accessors for the contemplative
// spine. Schema lives in works.go alongside the works table so a single
// EnsureArtGoatTables call provisions both.

package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// Sit is one journal entry — one sit, one optional reflection.
type Sit struct {
	ID              int64
	StartedAt       time.Time
	EndedAt         sql.NullTime
	WorkID          string
	DurationSeconds int
	Prompt          string
	Reflection      string
	Mood            sql.NullInt64
	Tags            string
	Mode            string // "atomic" | "phased" | "bare"
}

// InsertSit writes a new journal entry and returns the row ID.
//
// Wraps the parent-row insert and the FTS row insert in a single
// transaction so a crash between them cannot leave the FTS index
// permanently desynced from the sits table. Also serializes started_at /
// ended_at as RFC3339 strings before insert so SQLite's DATE() function
// can parse them — Go's default time.Time stringification emits a
// `2006-01-02 15:04:05 -0700 MST` form that DATE() returns NULL for,
// which silently breaks computeStreak.
func (s *Store) InsertSit(ctx context.Context, sit Sit) (int64, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin insert-sit tx: %w", err)
	}
	defer tx.Rollback()

	startedAt := sit.StartedAt.UTC().Format(time.RFC3339Nano)
	var endedAt any
	if sit.EndedAt.Valid {
		endedAt = sit.EndedAt.Time.UTC().Format(time.RFC3339Nano)
	}

	res, err := tx.ExecContext(ctx, `
INSERT INTO sits (
    started_at, ended_at, work_id, duration_seconds,
    prompt, reflection, mood, tags, mode
) VALUES (?,?,?,?,?,?,?,?,?)`,
		startedAt, endedAt, sit.WorkID, sit.DurationSeconds,
		sit.Prompt, sit.Reflection, sit.Mood, sit.Tags, sit.Mode,
	)
	if err != nil {
		return 0, fmt.Errorf("insert sit: %w", err)
	}
	id, _ := res.LastInsertId()
	if _, err := tx.ExecContext(ctx, `DELETE FROM sits_fts WHERE sit_id = ?`, id); err != nil {
		return id, fmt.Errorf("delete sits_fts: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO sits_fts (sit_id, reflection, prompt, tags) VALUES (?, ?, ?, ?)`,
		id, sit.Reflection, sit.Prompt, sit.Tags,
	); err != nil {
		return id, fmt.Errorf("insert sits_fts: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return id, fmt.Errorf("commit insert-sit: %w", err)
	}
	return id, nil
}

// RecentSitWorkIDs returns the set of work_id values from sits within the
// last `windowDays` days. Used by today's anti-repeat logic.
func (s *Store) RecentSitWorkIDs(ctx context.Context, windowDays int) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT DISTINCT work_id FROM sits
WHERE started_at >= datetime('now', ?)
ORDER BY started_at DESC`,
		fmt.Sprintf("-%d days", windowDays),
	)
	if err != nil {
		return nil, fmt.Errorf("recent sit work ids: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return out, err
		}
		if id != "" {
			out = append(out, id)
		}
	}
	return out, rows.Err()
}

// GetSit returns one sit by ID, or nil if none. Used by `journal compare`.
func (s *Store) GetSit(ctx context.Context, id int64) (*Sit, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, started_at, ended_at, work_id, duration_seconds,
       prompt, reflection, mood, tags, mode
FROM sits WHERE id = ?`, id)
	sit := &Sit{}
	err := row.Scan(
		&sit.ID, &sit.StartedAt, &sit.EndedAt, &sit.WorkID, &sit.DurationSeconds,
		&sit.Prompt, &sit.Reflection, &sit.Mood, &sit.Tags, &sit.Mode,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get sit %d: %w", id, err)
	}
	return sit, nil
}

// SitNearestToDate returns the sit whose started_at is closest to target
// within ±windowDays days. Returns nil when no sit falls inside the
// window. The "closest" choice is computed in Go (not SQL) so the
// comparison handles all parseStoredTime-supported layouts uniformly.
//
// windowDays must be > 0; the caller is responsible for clamping if it
// reads from user input.
func (s *Store) SitNearestToDate(ctx context.Context, target time.Time, windowDays int) (*Sit, error) {
	if windowDays <= 0 {
		windowDays = 7
	}
	low := target.AddDate(0, 0, -windowDays)
	high := target.AddDate(0, 0, windowDays)
	rows, err := s.db.QueryContext(ctx, `
SELECT id, started_at, ended_at, work_id, duration_seconds,
       prompt, reflection, mood, tags, mode
FROM sits
WHERE started_at >= ? AND started_at <= ?
ORDER BY started_at ASC`,
		low.UTC().Format(time.RFC3339Nano),
		high.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return nil, fmt.Errorf("sits near date: %w", err)
	}
	defer rows.Close()
	var best *Sit
	var bestDelta time.Duration
	for rows.Next() {
		sit := Sit{}
		if err := rows.Scan(
			&sit.ID, &sit.StartedAt, &sit.EndedAt, &sit.WorkID, &sit.DurationSeconds,
			&sit.Prompt, &sit.Reflection, &sit.Mood, &sit.Tags, &sit.Mode,
		); err != nil {
			return nil, err
		}
		delta := sit.StartedAt.Sub(target)
		if delta < 0 {
			delta = -delta
		}
		if best == nil || delta < bestDelta {
			bestDelta = delta
			s := sit
			best = &s
		}
	}
	return best, rows.Err()
}

// LastSit returns the most recent sit, or nil if none.
func (s *Store) LastSit(ctx context.Context) (*Sit, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, started_at, ended_at, work_id, duration_seconds,
       prompt, reflection, mood, tags, mode
FROM sits ORDER BY started_at DESC LIMIT 1`)
	sit := &Sit{}
	err := row.Scan(
		&sit.ID, &sit.StartedAt, &sit.EndedAt, &sit.WorkID, &sit.DurationSeconds,
		&sit.Prompt, &sit.Reflection, &sit.Mood, &sit.Tags, &sit.Mode,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("last sit: %w", err)
	}
	return sit, nil
}

// JournalStats holds the aggregated practice metrics journal stats
// returns. Streak is computed but lives at the bottom of the rendered
// output ("if you want to know").
type JournalStats struct {
	TotalSits       int
	TotalSeconds    int
	LastSitAt       sql.NullTime
	BySource        map[string]int
	ByMedium        map[string]int
	ByRegion        map[string]int
	ByPeriodCentury map[string]int
	AvgMoodOverall  float64
	MoodBySource    map[string]float64
	TopTags         []TagCount
	CurrentStreak   int
}

type TagCount struct {
	Tag   string
	Count int
}

// CollectJournalStats aggregates the sits table joined to works for the
// reframed practice metrics.
func (s *Store) CollectJournalStats(ctx context.Context) (*JournalStats, error) {
	stats := &JournalStats{
		BySource:        make(map[string]int),
		ByMedium:        make(map[string]int),
		ByRegion:        make(map[string]int),
		ByPeriodCentury: make(map[string]int),
		MoodBySource:    make(map[string]float64),
	}

	// Total sits + total seconds
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*), COALESCE(SUM(duration_seconds),0) FROM sits`,
	).Scan(&stats.TotalSits, &stats.TotalSeconds); err != nil {
		return nil, fmt.Errorf("count sits: %w", err)
	}
	if stats.TotalSits == 0 {
		return stats, nil
	}

	// Last sit. SQLite stores started_at as text; the precise format
	// depends on which Go time layout the writer used. Read as a
	// nullable string and parse defensively so an unusual layout
	// (e.g. with a `+0000 UTC` suffix) doesn't silently scan as the
	// zero value with Valid=true, which would render as 0001-01-01.
	var lastSitStr sql.NullString
	_ = s.db.QueryRowContext(ctx, `SELECT MAX(started_at) FROM sits`).Scan(&lastSitStr)
	if lastSitStr.Valid && lastSitStr.String != "" {
		if t, ok := parseStoredTime(lastSitStr.String); ok {
			stats.LastSitAt = sql.NullTime{Time: t, Valid: true}
		}
	}

	// By source (join through works). Distinguish reflection-only sits
	// (no work attached — empty/NULL work_id) from sits that point at a
	// work whose source field is genuinely missing. Labeling both as
	// "unknown" reads like a data-lookup failure to users.
	rows, err := s.db.QueryContext(ctx, `
SELECT CASE
  WHEN s.work_id IS NULL OR s.work_id = '' THEN 'reflection-only'
  ELSE COALESCE(NULLIF(w.source, ''), 'unknown')
END, COUNT(*)
FROM sits s LEFT JOIN works w ON w.id = s.work_id
GROUP BY 1 ORDER BY COUNT(*) DESC`)
	if err != nil {
		return stats, fmt.Errorf("by source: %w", err)
	}
	for rows.Next() {
		var src string
		var n int
		if err := rows.Scan(&src, &n); err == nil {
			stats.BySource[src] = n
		}
	}
	rows.Close()

	// By medium
	rows, err = s.db.QueryContext(ctx, `
SELECT CASE
  WHEN s.work_id IS NULL OR s.work_id = '' THEN 'reflection-only'
  ELSE COALESCE(NULLIF(w.medium, ''), 'unknown')
END, COUNT(*)
FROM sits s LEFT JOIN works w ON w.id = s.work_id
GROUP BY 1 ORDER BY COUNT(*) DESC`)
	if err == nil {
		for rows.Next() {
			var medium string
			var n int
			if err := rows.Scan(&medium, &n); err == nil {
				stats.ByMedium[medium] = n
			}
		}
		rows.Close()
	}

	// By culture region
	rows, err = s.db.QueryContext(ctx, `
SELECT CASE
  WHEN s.work_id IS NULL OR s.work_id = '' THEN 'reflection-only'
  ELSE COALESCE(NULLIF(w.culture_region, ''), 'unknown')
END, COUNT(*)
FROM sits s LEFT JOIN works w ON w.id = s.work_id
GROUP BY 1 ORDER BY COUNT(*) DESC`)
	if err == nil {
		for rows.Next() {
			var region string
			var n int
			if err := rows.Scan(&region, &n); err == nil {
				stats.ByRegion[region] = n
			}
		}
		rows.Close()
	}

	// By century (computed from date_start)
	rows, err = s.db.QueryContext(ctx, `
SELECT CASE WHEN w.date_start > 0 THEN CAST(w.date_start/100 AS INTEGER)*100 ELSE NULL END AS century, COUNT(*)
FROM sits s LEFT JOIN works w ON w.id = s.work_id
WHERE century IS NOT NULL
GROUP BY century ORDER BY century`)
	if err == nil {
		for rows.Next() {
			var century, n int
			if err := rows.Scan(&century, &n); err == nil {
				stats.ByPeriodCentury[fmt.Sprintf("%d-%d", century, century+99)] = n
			}
		}
		rows.Close()
	}

	// Avg mood
	_ = s.db.QueryRowContext(ctx,
		`SELECT COALESCE(AVG(mood), 0) FROM sits WHERE mood IS NOT NULL`,
	).Scan(&stats.AvgMoodOverall)

	// Mood by source
	rows, err = s.db.QueryContext(ctx, `
SELECT CASE
  WHEN s.work_id IS NULL OR s.work_id = '' THEN 'reflection-only'
  ELSE COALESCE(NULLIF(w.source, ''), 'unknown')
END, AVG(s.mood)
FROM sits s LEFT JOIN works w ON w.id = s.work_id
WHERE s.mood IS NOT NULL
GROUP BY 1`)
	if err == nil {
		for rows.Next() {
			var src string
			var avg float64
			if err := rows.Scan(&src, &avg); err == nil {
				stats.MoodBySource[src] = avg
			}
		}
		rows.Close()
	}

	// Top tags — sits.tags is comma-separated; do this in Go.
	tagRows, err := s.db.QueryContext(ctx, `SELECT tags FROM sits WHERE tags IS NOT NULL AND tags <> ''`)
	if err == nil {
		tagCounts := make(map[string]int)
		for tagRows.Next() {
			var tags string
			if err := tagRows.Scan(&tags); err == nil {
				for _, t := range strings.Split(tags, ",") {
					t = strings.TrimSpace(strings.ToLower(t))
					if t == "" {
						continue
					}
					// Drop bare numeric tokens. Mood ratings (1-5)
					// historically leaked into the tags column on a few
					// rows; a pure number is never a meaningful tag and
					// renders as e.g. "4: 1" in stats output.
					if isBareNumeric(t) {
						continue
					}
					tagCounts[t]++
				}
			}
		}
		tagRows.Close()
		for t, n := range tagCounts {
			stats.TopTags = append(stats.TopTags, TagCount{Tag: t, Count: n})
		}
		// Sort by count desc (simple inline sort).
		for i := 1; i < len(stats.TopTags); i++ {
			for j := i; j > 0 && stats.TopTags[j].Count > stats.TopTags[j-1].Count; j-- {
				stats.TopTags[j], stats.TopTags[j-1] = stats.TopTags[j-1], stats.TopTags[j]
			}
		}
		if len(stats.TopTags) > 6 {
			stats.TopTags = stats.TopTags[:6]
		}
	}

	// Current streak: walk back from today, counting consecutive days with at least one sit.
	streak, _ := s.computeStreak(ctx)
	stats.CurrentStreak = streak

	return stats, nil
}

// CurrentStreak exposes the consecutive-day sit streak as a public read.
// Used by the per-sit streak greeting when the user has opted in.
func (s *Store) CurrentStreak(ctx context.Context) (int, error) {
	return s.computeStreak(ctx)
}

func (s *Store) computeStreak(ctx context.Context) (int, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT DISTINCT DATE(started_at) FROM sits ORDER BY DATE(started_at) DESC`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	streak := 0
	now := time.Now().UTC().Truncate(24 * time.Hour)
	expecting := now
	gracePeriod := false
	for rows.Next() {
		var dateStr string
		if err := rows.Scan(&dateStr); err != nil {
			return streak, err
		}
		d, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue
		}
		switch {
		case d.Equal(expecting):
			streak++
			expecting = expecting.AddDate(0, 0, -1)
		case d.Equal(expecting.AddDate(0, 0, -1)) && !gracePeriod && streak == 0:
			// First sit was yesterday (rows stream newest-first; if today
			// has no sit the first row's date == today - 1d). One-day
			// grace before saying streak=0.
			streak = 1
			expecting = d.AddDate(0, 0, -1)
			gracePeriod = true
		default:
			return streak, nil
		}
	}
	return streak, rows.Err()
}

// MoodAveragesByDimension returns three maps — source→avg mood,
// medium→avg mood, region→avg mood — over sits joined to works. Only
// sits with mood IS NOT NULL contribute. Empty maps when the journal has
// no mood-rated sits yet.
//
// Used by `today --mode bridge-from-last` to score candidate works by
// the historical mood their source/medium/region tends to produce.
// Mood-by-X is a proxy — works themselves carry no mood — but it's the
// best signal the journal gives us until per-work moods accumulate.
func (s *Store) MoodAveragesByDimension(ctx context.Context) (
	bySource map[string]float64,
	byMedium map[string]float64,
	byRegion map[string]float64,
	err error,
) {
	bySource = map[string]float64{}
	byMedium = map[string]float64{}
	byRegion = map[string]float64{}

	type pull struct {
		col string
		out map[string]float64
	}
	pulls := []pull{
		{"w.source", bySource},
		{"LOWER(w.medium)", byMedium},
		{"LOWER(w.culture_region)", byRegion},
	}
	for _, p := range pulls {
		// nolint:gosec // p.col is from a fixed allowlist literal, not user input.
		q := `SELECT ` + p.col + `, AVG(s.mood)
FROM sits s JOIN works w ON w.id = s.work_id
WHERE s.mood IS NOT NULL AND ` + p.col + ` IS NOT NULL AND ` + p.col + ` <> ''
GROUP BY ` + p.col
		rows, qerr := s.db.QueryContext(ctx, q)
		if qerr != nil {
			return nil, nil, nil, fmt.Errorf("mood averages: %w", qerr)
		}
		for rows.Next() {
			var key string
			var avg float64
			if err := rows.Scan(&key, &avg); err == nil {
				p.out[key] = avg
			}
		}
		rows.Close()
	}
	return bySource, byMedium, byRegion, nil
}

// AllSits returns every sit in chronological order (oldest first). If
// since is non-zero, only sits started on or after that instant are
// returned. Used by `journal export` to mirror the sits table to
// Markdown files, where ordering and full coverage matter more than
// FTS ranking.
func (s *Store) AllSits(ctx context.Context, since time.Time) ([]Sit, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if since.IsZero() {
		rows, err = s.db.QueryContext(ctx, `
SELECT id, started_at, ended_at, work_id, duration_seconds,
       prompt, reflection, mood, tags, mode
FROM sits ORDER BY started_at ASC`)
	} else {
		rows, err = s.db.QueryContext(ctx, `
SELECT id, started_at, ended_at, work_id, duration_seconds,
       prompt, reflection, mood, tags, mode
FROM sits WHERE started_at >= ? ORDER BY started_at ASC`, since)
	}
	if err != nil {
		return nil, fmt.Errorf("all sits: %w", err)
	}
	defer rows.Close()
	return scanSits(rows)
}

// SearchSits FTS-searches the sits journal.
func (s *Store) SearchSits(ctx context.Context, query string, limit int) ([]Sit, error) {
	if limit <= 0 {
		limit = 20
	}
	if strings.TrimSpace(query) == "" {
		return s.recentSits(ctx, limit)
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT s.id, s.started_at, s.ended_at, s.work_id, s.duration_seconds,
       s.prompt, s.reflection, s.mood, s.tags, s.mode
FROM sits_fts JOIN sits s ON s.id = sits_fts.sit_id
WHERE sits_fts MATCH ?
ORDER BY rank
LIMIT ?`, query, limit)
	if err != nil {
		return nil, fmt.Errorf("search sits: %w", err)
	}
	defer rows.Close()
	return scanSits(rows)
}

func (s *Store) recentSits(ctx context.Context, limit int) ([]Sit, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, started_at, ended_at, work_id, duration_seconds,
       prompt, reflection, mood, tags, mode
FROM sits ORDER BY started_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSits(rows)
}

func scanSits(rows *sql.Rows) ([]Sit, error) {
	var out []Sit
	for rows.Next() {
		sit := Sit{}
		if err := rows.Scan(
			&sit.ID, &sit.StartedAt, &sit.EndedAt, &sit.WorkID, &sit.DurationSeconds,
			&sit.Prompt, &sit.Reflection, &sit.Mood, &sit.Tags, &sit.Mode,
		); err != nil {
			return out, err
		}
		out = append(out, sit)
	}
	return out, rows.Err()
}

// isBareNumeric reports whether s is a non-empty all-digit token (e.g.
// "4", "10"). Such tokens are filtered out of the journal tag
// aggregator because mood ratings have historically leaked into the
// tags column on a small number of rows and rendered as e.g. "4: 1".
func isBareNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// parseStoredTime parses the started_at/ended_at text format SQLite
// returns when Go writes a time.Time via the standard driver. The Go
// stdlib SQLite driver writes times as "2006-01-02 15:04:05.999999999
// -0700 MST" by default; reads can choose a layout but raw SELECT of
// the column returns the original text. Parse permissively so callers
// (e.g. JournalStats.LastSitAt) don't get a zero time from a layout
// mismatch.
func parseStoredTime(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	layouts := []string{
		"2006-01-02 15:04:05.999999999 -0700 MST",
		"2006-01-02 15:04:05.999999999 -0700",
		"2006-01-02 15:04:05 -0700 MST",
		"2006-01-02 15:04:05",
		time.RFC3339Nano,
		time.RFC3339,
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}
