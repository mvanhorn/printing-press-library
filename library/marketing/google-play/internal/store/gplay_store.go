// Hand-authored snapshot layer for google-play-pp-cli. These tables are the
// foundation for the transcendence commands (movers, rank-history,
// watch-listing, keyword-history) that need time-series data the live Play
// Store never exposes. Migrations are lazy (CREATE TABLE IF NOT EXISTS) and
// kept out of the generated store.go so a regen cannot drop them.
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

const gplaySchema = `
CREATE TABLE IF NOT EXISTS chart_snapshots (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	collection  TEXT NOT NULL,
	category    TEXT NOT NULL,
	country     TEXT NOT NULL,
	captured_at INTEGER NOT NULL,
	rank        INTEGER NOT NULL,
	app_id      TEXT NOT NULL,
	title       TEXT,
	score       REAL,
	data        TEXT,
	UNIQUE(collection, category, country, captured_at, app_id) ON CONFLICT REPLACE
);
CREATE INDEX IF NOT EXISTS idx_chart_key ON chart_snapshots(collection, category, country, captured_at);
CREATE INDEX IF NOT EXISTS idx_chart_app ON chart_snapshots(app_id, collection, category, country, captured_at);

CREATE TABLE IF NOT EXISTS app_snapshots (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	app_id      TEXT NOT NULL,
	captured_at INTEGER NOT NULL,
	data        TEXT NOT NULL,
	UNIQUE(app_id, captured_at) ON CONFLICT REPLACE
);
CREATE INDEX IF NOT EXISTS idx_app_snap ON app_snapshots(app_id, captured_at);

CREATE TABLE IF NOT EXISTS keyword_ranks (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	term        TEXT NOT NULL,
	country     TEXT NOT NULL,
	app_id      TEXT NOT NULL,
	captured_at INTEGER NOT NULL,
	rank        INTEGER NOT NULL,
	scanned     INTEGER NOT NULL,
	UNIQUE(term, country, app_id, captured_at) ON CONFLICT REPLACE
);
CREATE INDEX IF NOT EXISTS idx_kw ON keyword_ranks(term, country, app_id, captured_at);

CREATE TABLE IF NOT EXISTS app_reviews (
	review_id   TEXT NOT NULL,
	app_id      TEXT NOT NULL,
	score       INTEGER,
	at          INTEGER,
	version     TEXT,
	reply       INTEGER NOT NULL DEFAULT 0,
	text        TEXT,
	PRIMARY KEY (app_id, review_id)
);
CREATE INDEX IF NOT EXISTS idx_rev_app ON app_reviews(app_id, at);
`

// EnsureGplaySchema lazily creates the snapshot tables. Safe to call repeatedly.
func (s *Store) EnsureGplaySchema(ctx context.Context) error {
	_, err := s.DB().ExecContext(ctx, gplaySchema)
	return err
}

// ChartRow is one ranked entry in a chart snapshot.
type ChartRow struct {
	Rank  int     `json:"rank"`
	AppID string  `json:"appId"`
	Title string  `json:"title"`
	Score float64 `json:"score"`
}

// InsertChartSnapshot records a ranked chart at capturedAt (unix seconds).
func (s *Store) InsertChartSnapshot(ctx context.Context, collection, category, country string, capturedAt int64, rows []ChartRow) error {
	if err := s.EnsureGplaySchema(ctx); err != nil {
		return err
	}
	tx, err := s.DB().BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(ctx, `INSERT OR REPLACE INTO chart_snapshots(collection,category,country,captured_at,rank,app_id,title,score) VALUES(?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, r := range rows {
		if _, err := stmt.ExecContext(ctx, collection, category, country, capturedAt, r.Rank, r.AppID, r.Title, r.Score); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// chartCapturedTimes returns the distinct snapshot timestamps for a chart key,
// newest first.
func (s *Store) chartCapturedTimes(ctx context.Context, collection, category, country string) ([]int64, error) {
	rows, err := s.DB().QueryContext(ctx,
		`SELECT DISTINCT captured_at FROM chart_snapshots WHERE collection=? AND category=? AND country=? ORDER BY captured_at DESC`,
		collection, category, country)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var t int64
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// ChartSnapshotAt returns the ranked rows of one chart snapshot.
func (s *Store) ChartSnapshotAt(ctx context.Context, collection, category, country string, capturedAt int64) ([]ChartRow, error) {
	rows, err := s.DB().QueryContext(ctx,
		`SELECT rank,app_id,title,score FROM chart_snapshots WHERE collection=? AND category=? AND country=? AND captured_at=? ORDER BY rank`,
		collection, category, country, capturedAt)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ChartRow
	for rows.Next() {
		var r ChartRow
		var title sql.NullString
		var score sql.NullFloat64
		if err := rows.Scan(&r.Rank, &r.AppID, &title, &score); err != nil {
			return nil, err
		}
		r.Title = title.String
		r.Score = score.Float64
		out = append(out, r)
	}
	return out, rows.Err()
}

// LatestTwoChartSnapshots returns (previous, latest) capture times for a chart
// key. ok is false when fewer than two snapshots exist.
func (s *Store) LatestTwoChartSnapshots(ctx context.Context, collection, category, country string) (prev, latest int64, ok bool, err error) {
	times, err := s.chartCapturedTimes(ctx, collection, category, country)
	if err != nil {
		return 0, 0, false, err
	}
	if len(times) < 2 {
		if len(times) == 1 {
			return 0, times[0], false, nil
		}
		return 0, 0, false, nil
	}
	return times[1], times[0], true, nil
}

// ChartCaptureCount returns how many distinct snapshots exist for a chart key.
func (s *Store) ChartCaptureCount(ctx context.Context, collection, category, country string) (int, error) {
	times, err := s.chartCapturedTimes(ctx, collection, category, country)
	return len(times), err
}

// AppRankSeries returns one app's rank over time within a chart key, oldest
// first.
type RankPoint struct {
	CapturedAt int64 `json:"capturedAt"`
	Rank       int   `json:"rank"`
}

func (s *Store) AppRankSeries(ctx context.Context, appID, collection, category, country string) ([]RankPoint, error) {
	if err := s.EnsureGplaySchema(ctx); err != nil {
		return nil, err
	}
	rows, err := s.DB().QueryContext(ctx,
		`SELECT captured_at,rank FROM chart_snapshots WHERE app_id=? AND collection=? AND category=? AND country=? ORDER BY captured_at`,
		appID, collection, category, country)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RankPoint
	for rows.Next() {
		var p RankPoint
		if err := rows.Scan(&p.CapturedAt, &p.Rank); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// InsertAppSnapshot records the full detail JSON of an app at capturedAt.
func (s *Store) InsertAppSnapshot(ctx context.Context, appID string, capturedAt int64, data json.RawMessage) error {
	if err := s.EnsureGplaySchema(ctx); err != nil {
		return err
	}
	_, err := s.DB().ExecContext(ctx,
		`INSERT OR REPLACE INTO app_snapshots(app_id,captured_at,data) VALUES(?,?,?)`,
		appID, capturedAt, string(data))
	return err
}

// AppSnapshot is one stored detail snapshot.
type AppSnapshot struct {
	CapturedAt int64
	Data       json.RawMessage
}

// LatestAppSnapshots returns up to n snapshots for an app, newest first.
func (s *Store) LatestAppSnapshots(ctx context.Context, appID string, n int) ([]AppSnapshot, error) {
	if err := s.EnsureGplaySchema(ctx); err != nil {
		return nil, err
	}
	rows, err := s.DB().QueryContext(ctx,
		`SELECT captured_at,data FROM app_snapshots WHERE app_id=? ORDER BY captured_at DESC LIMIT ?`,
		appID, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AppSnapshot
	for rows.Next() {
		var sn AppSnapshot
		var data string
		if err := rows.Scan(&sn.CapturedAt, &data); err != nil {
			return nil, err
		}
		sn.Data = json.RawMessage(data)
		out = append(out, sn)
	}
	return out, rows.Err()
}

// InsertKeywordRank records where appID ranked for a term in a country.
func (s *Store) InsertKeywordRank(ctx context.Context, term, country, appID string, capturedAt int64, rank, scanned int) error {
	if err := s.EnsureGplaySchema(ctx); err != nil {
		return err
	}
	_, err := s.DB().ExecContext(ctx,
		`INSERT OR REPLACE INTO keyword_ranks(term,country,app_id,captured_at,rank,scanned) VALUES(?,?,?,?,?,?)`,
		term, country, appID, capturedAt, rank, scanned)
	return err
}

// KeywordRankPoint is one keyword-rank observation.
type KeywordRankPoint struct {
	CapturedAt int64 `json:"capturedAt"`
	Rank       int   `json:"rank"`
	Scanned    int   `json:"scanned"`
}

// KeywordRankSeries returns rank observations for a term/country/app, oldest first.
func (s *Store) KeywordRankSeries(ctx context.Context, term, country, appID string) ([]KeywordRankPoint, error) {
	if err := s.EnsureGplaySchema(ctx); err != nil {
		return nil, err
	}
	rows, err := s.DB().QueryContext(ctx,
		`SELECT captured_at,rank,scanned FROM keyword_ranks WHERE term=? AND country=? AND app_id=? ORDER BY captured_at`,
		term, country, appID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []KeywordRankPoint
	for rows.Next() {
		var p KeywordRankPoint
		if err := rows.Scan(&p.CapturedAt, &p.Rank, &p.Scanned); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ReviewRow is the stored shape of a review used by review-digest.
type ReviewRow struct {
	ReviewID string
	Score    int
	At       int64
	Version  string
	Reply    bool
	Text     string
}

// UpsertReviews stores reviews for an app (idempotent on app_id+review_id).
func (s *Store) UpsertReviews(ctx context.Context, appID string, reviews []ReviewRow) error {
	if err := s.EnsureGplaySchema(ctx); err != nil {
		return err
	}
	tx, err := s.DB().BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO app_reviews(review_id,app_id,score,at,version,reply,text) VALUES(?,?,?,?,?,?,?)
		 ON CONFLICT(app_id,review_id) DO UPDATE SET score=excluded.score,at=excluded.at,version=excluded.version,reply=excluded.reply,text=excluded.text`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, r := range reviews {
		reply := 0
		if r.Reply {
			reply = 1
		}
		if _, err := stmt.ExecContext(ctx, r.ReviewID, appID, r.Score, r.At, r.Version, reply, r.Text); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// StoredReviews returns all stored reviews for an app.
func (s *Store) StoredReviews(ctx context.Context, appID string) ([]ReviewRow, error) {
	if err := s.EnsureGplaySchema(ctx); err != nil {
		return nil, err
	}
	rows, err := s.DB().QueryContext(ctx,
		`SELECT review_id,score,at,version,reply,text FROM app_reviews WHERE app_id=? ORDER BY at DESC`, appID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ReviewRow
	for rows.Next() {
		var r ReviewRow
		var score sql.NullInt64
		var at sql.NullInt64
		var version, text sql.NullString
		var reply int
		if err := rows.Scan(&r.ReviewID, &score, &at, &version, &reply, &text); err != nil {
			return nil, err
		}
		r.Score = int(score.Int64)
		r.At = at.Int64
		r.Version = version.String
		r.Reply = reply == 1
		r.Text = text.String
		out = append(out, r)
	}
	return out, rows.Err()
}

// NowUnix is a tiny indirection so tests can stamp deterministic times.
func NowUnix() int64 { return time.Now().Unix() }
