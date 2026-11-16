// Copyright 2026 Josselin and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written queries over the serp_snapshots, serp_results and news_seen
// tables declared in extras.go.

package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/autom/internal/serp"
)

// SerpKey identifies one tracked keyword: the same query on the same engine,
// country and language.
type SerpKey struct {
	Query   string `json:"query"`
	Engine  string `json:"engine"`
	Country string `json:"country,omitempty"`
	Lang    string `json:"lang,omitempty"`
}

// SerpSnapshot is one saved results page.
type SerpSnapshot struct {
	ID          int64     `json:"id"`
	Query       string    `json:"query"`
	Engine      string    `json:"engine"`
	Country     string    `json:"country,omitempty"`
	Lang        string    `json:"lang,omitempty"`
	Location    string    `json:"location,omitempty"`
	Command     string    `json:"command,omitempty"`
	TakenAt     time.Time `json:"taken_at"`
	ResultCount int       `json:"result_count"`
}

// Key returns the keyword identity of the snapshot.
func (s SerpSnapshot) Key() SerpKey {
	return SerpKey{Query: s.Query, Engine: s.Engine, Country: s.Country, Lang: s.Lang}
}

// tsLayout is fixed-width UTC so text order is time order (RFC3339Nano drops
// trailing zeros, which breaks ORDER BY and cutoff comparisons within a second).
const tsLayout = "2006-01-02T15:04:05.000000000Z"

func normKey(k SerpKey) SerpKey {
	return SerpKey{
		Query:   strings.TrimSpace(k.Query),
		Engine:  strings.ToLower(strings.TrimSpace(k.Engine)),
		Country: strings.ToLower(strings.TrimSpace(k.Country)),
		Lang:    strings.ToLower(strings.TrimSpace(k.Lang)),
	}
}

// SaveSerpSnapshot stores a results page and returns its id.
func (s *Store) SaveSerpSnapshot(ctx context.Context, snap SerpSnapshot, results []serp.Result) (int64, error) {
	k := normKey(snap.Key())
	if k.Query == "" || k.Engine == "" {
		return 0, fmt.Errorf("snapshot needs a query and an engine")
	}
	if snap.TakenAt.IsZero() {
		snap.TakenAt = time.Now()
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.DB().BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	res, err := tx.ExecContext(ctx, `INSERT INTO serp_snapshots(query, engine, country, lang, location, command, taken_at, result_count)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		k.Query, k.Engine, k.Country, k.Lang, strings.TrimSpace(snap.Location), snap.Command,
		snap.TakenAt.UTC().Format(tsLayout), len(results))
	if err != nil {
		return 0, fmt.Errorf("saving snapshot: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	for _, r := range results {
		if _, err := tx.ExecContext(ctx, `INSERT INTO serp_results(snapshot_id, position, url, domain, title, snippet) VALUES (?, ?, ?, ?, ?, ?)`,
			id, r.Position, r.URL, r.Domain, r.Title, r.Snippet); err != nil {
			return 0, fmt.Errorf("saving result: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return id, nil
}

func scanSnapshots(rows *sql.Rows) ([]SerpSnapshot, error) {
	out := make([]SerpSnapshot, 0)
	for rows.Next() {
		var sn SerpSnapshot
		var taken string
		if err := rows.Scan(&sn.ID, &sn.Query, &sn.Engine, &sn.Country, &sn.Lang, &sn.Location, &sn.Command, &taken, &sn.ResultCount); err != nil {
			_ = rows.Close()
			return nil, err
		}
		t, err := time.Parse(tsLayout, taken)
		if err != nil {
			if t, err = time.Parse(time.RFC3339Nano, taken); err != nil {
				_ = rows.Close()
				return nil, fmt.Errorf("snapshot %d has an unreadable time %q: %w", sn.ID, taken, err)
			}
		}
		sn.TakenAt = t
		out = append(out, sn)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	return out, rows.Close()
}

const snapshotCols = `id, query, engine, country, lang, location, command, taken_at, result_count`

// SerpSnapshots lists snapshots newest first. Empty key fields match anything;
// the query matches case-insensitively. limit<=0 means no limit.
func (s *Store) SerpSnapshots(ctx context.Context, k SerpKey, limit int) ([]SerpSnapshot, error) {
	k = normKey(k)
	q := `SELECT ` + snapshotCols + ` FROM serp_snapshots WHERE 1=1`
	args := make([]any, 0)
	if k.Query != "" {
		q += ` AND lower(query) = lower(?)`
		args = append(args, k.Query)
	}
	if k.Engine != "" {
		q += ` AND engine = ?`
		args = append(args, k.Engine)
	}
	if k.Country != "" {
		q += ` AND country = ?`
		args = append(args, k.Country)
	}
	if k.Lang != "" {
		q += ` AND lang = ?`
		args = append(args, k.Lang)
	}
	q += ` ORDER BY taken_at DESC, id DESC`
	if limit > 0 {
		q += fmt.Sprintf(` LIMIT %d`, limit)
	}
	rows, err := s.DB().QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	return scanSnapshots(rows)
}

// SerpSnapshotsExact lists the snapshots of exactly one keyword identity,
// newest first: unlike SerpSnapshots, an empty country or lang only matches
// pages saved without one. limit<=0 means no limit.
func (s *Store) SerpSnapshotsExact(ctx context.Context, k SerpKey, limit int) ([]SerpSnapshot, error) {
	k = normKey(k)
	q := `SELECT ` + snapshotCols + ` FROM serp_snapshots WHERE lower(query) = lower(?) AND engine = ? AND country = ? AND lang = ? ORDER BY taken_at DESC, id DESC`
	if limit > 0 {
		q += fmt.Sprintf(` LIMIT %d`, limit)
	}
	rows, err := s.DB().QueryContext(ctx, q, k.Query, k.Engine, k.Country, k.Lang)
	if err != nil {
		return nil, err
	}
	return scanSnapshots(rows)
}

// SerpSnapshotByID returns one snapshot.
func (s *Store) SerpSnapshotByID(ctx context.Context, id int64) (SerpSnapshot, bool, error) {
	rows, err := s.DB().QueryContext(ctx, `SELECT `+snapshotCols+` FROM serp_snapshots WHERE id = ?`, id)
	if err != nil {
		return SerpSnapshot{}, false, err
	}
	list, err := scanSnapshots(rows)
	if err != nil || len(list) == 0 {
		return SerpSnapshot{}, false, err
	}
	return list[0], true, nil
}

// SerpResults returns the results of one snapshot in position order.
func (s *Store) SerpResults(ctx context.Context, snapshotID int64) ([]serp.Result, error) {
	rows, err := s.DB().QueryContext(ctx, `SELECT position, url, domain, title, snippet FROM serp_results WHERE snapshot_id = ? ORDER BY position`, snapshotID)
	if err != nil {
		return nil, err
	}
	out := make([]serp.Result, 0)
	for rows.Next() {
		var r serp.Result
		if err := rows.Scan(&r.Position, &r.URL, &r.Domain, &r.Title, &r.Snippet); err != nil {
			_ = rows.Close()
			return nil, err
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	return out, rows.Close()
}

// SerpKeys lists every tracked keyword, optionally filtered by country.
func (s *Store) SerpKeys(ctx context.Context, country string) ([]SerpKey, error) {
	q := `SELECT MIN(query), engine, country, lang FROM serp_snapshots`
	args := make([]any, 0)
	if c := strings.ToLower(strings.TrimSpace(country)); c != "" {
		q += ` WHERE country = ?`
		args = append(args, c)
	}
	q += ` GROUP BY lower(query), engine, country, lang ORDER BY lower(query), engine, country, lang`
	rows, err := s.DB().QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	out := make([]SerpKey, 0)
	for rows.Next() {
		var k SerpKey
		if err := rows.Scan(&k.Query, &k.Engine, &k.Country, &k.Lang); err != nil {
			_ = rows.Close()
			return nil, err
		}
		out = append(out, k)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	return out, rows.Close()
}

// LatestSerpSnapshot returns the newest snapshot of a keyword taken at or
// before `before` (zero time = no bound).
func (s *Store) LatestSerpSnapshot(ctx context.Context, k SerpKey, before time.Time) (SerpSnapshot, bool, error) {
	k = normKey(k)
	q := `SELECT ` + snapshotCols + ` FROM serp_snapshots WHERE lower(query) = lower(?) AND engine = ? AND country = ? AND lang = ?`
	args := []any{k.Query, k.Engine, k.Country, k.Lang}
	if !before.IsZero() {
		q += ` AND taken_at <= ?`
		args = append(args, before.UTC().Format(tsLayout))
	}
	q += ` ORDER BY taken_at DESC, id DESC LIMIT 1`
	rows, err := s.DB().QueryContext(ctx, q, args...)
	if err != nil {
		return SerpSnapshot{}, false, err
	}
	list, err := scanSnapshots(rows)
	if err != nil || len(list) == 0 {
		return SerpSnapshot{}, false, err
	}
	return list[0], true, nil
}

// CountSerpSnapshots returns how many snapshots are saved.
func (s *Store) CountSerpSnapshots(ctx context.Context) (int, error) {
	var n int
	err := s.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM serp_snapshots`).Scan(&n)
	return n, err
}

// NewsScopeSeen reports whether anything was recorded for this watch scope.
func (s *Store) NewsScopeSeen(ctx context.Context, scope string) (bool, error) {
	var one int
	err := s.DB().QueryRowContext(ctx, `SELECT 1 FROM news_seen WHERE query = ? LIMIT 1`, scope).Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil, err
}

// NewsSeen reports whether any of the keys was recorded before. Callers scope
// the keys (news watch prefixes them with its query and edition) so separate
// watches keep separate memories.
func (s *Store) NewsSeen(ctx context.Context, keys []string) (bool, error) {
	for _, k := range keys {
		var one int
		err := s.DB().QueryRowContext(ctx, `SELECT 1 FROM news_seen WHERE key = ?`, k).Scan(&one)
		if err == nil {
			return true, nil
		}
		if err != sql.ErrNoRows {
			return false, err
		}
	}
	return false, nil
}

// MarkNewsSeen records every key of an article under a watch scope.
func (s *Store) MarkNewsSeen(ctx context.Context, scope string, r serp.Result, keys []string, at time.Time) error {
	if len(keys) == 0 {
		return nil
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.DB().BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, k := range keys {
		if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO news_seen(key, query, url, source, title, published, first_seen) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			k, scope, r.URL, r.Source, r.Title, r.Date, at.UTC().Format(tsLayout)); err != nil {
			return fmt.Errorf("recording news article: %w", err)
		}
	}
	return tx.Commit()
}
