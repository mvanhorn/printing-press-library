// Copyright 2026 mani. Licensed under Apache-2.0. See LICENSE.
// PATCH: novel SQLite-backed store for transcendence features (budget watch,
//        corpus build, local search, drift detect, replay, research track, etc.)

// Package store provides a SQLite-backed persistent store for the Tavily CLI's
// offline and analytical features. It records every live API call (search,
// extract, crawl, research) along with metadata so that commands like
// local-search, corpus-gaps, drift-detect, cost-report, and replay can operate
// without consuming additional API credits.
package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// DB wraps a SQLite connection and exposes typed helpers for the Tavily CLI.
type DB struct {
	db   *sql.DB
	Path string
}

// SearchRow is one stored search call.
type SearchRow struct {
	ID        int64
	Query     string
	Body      string // JSON body sent to API
	Response  string // JSON response from API
	Session   string // --session label for replay
	CreatedAt time.Time
}

// ExtractRow is one stored extract call.
type ExtractRow struct {
	ID        int64
	URLs      string // JSON array of URLs
	Response  string // JSON response
	Session   string
	CreatedAt time.Time
}

// CrawlRow is one stored crawl session.
type CrawlRow struct {
	ID          int64
	RootURL     string
	Params      string // JSON params
	PagesFetched int
	Status      string // running, complete, interrupted
	Checkpoint  string // JSON last-page state for resume
	Session     string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ResearchRow is one stored async research task.
type ResearchRow struct {
	ID        int64
	RequestID string
	Input     string
	Status    string // pending, running, complete, failed
	Report    string // final report text
	Session   string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// CreditRow tracks estimated credit usage per call.
type CreditRow struct {
	ID        int64
	Endpoint  string // search, extract, crawl, research
	Credits   float64
	Session   string
	CreatedAt time.Time
}

// Open opens (or creates) the Tavily CLI SQLite database at the default path.
func Open() (*DB, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("finding home dir: %w", err)
	}
	dir := filepath.Join(home, ".local", "share", "tavily-pp-cli")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("creating store dir: %w", err)
	}
	path := filepath.Join(dir, "tavily.db")
	return OpenAt(path)
}

// OpenAt opens the database at an explicit path (useful in tests).
func OpenAt(path string) (*DB, error) {
	db, err := sql.Open("sqlite", path+"?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("opening sqlite db at %s: %w", path, err)
	}
	store := &DB{db: db, Path: path}
	if err := store.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return store, nil
}

// Close closes the underlying database connection.
func (s *DB) Close() error {
	return s.db.Close()
}

// migrate runs the DDL that brings a fresh database up to the current
// schema. The statements must apply atomically: SQLite's CREATE TRIGGER
// does not validate the target table at creation time, so a half-applied
// migration can leave a searches_ai trigger pointing at a missing
// searches_fts virtual table, which then fails every subsequent
// INSERT INTO searches with 'no such table: searches_fts'.
// PATCH: wrap DDL in a transaction so partial migrations roll back.
func (s *DB) migrate() error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin migration tx: %w", err)
	}
	if _, err := tx.Exec(`
		CREATE TABLE IF NOT EXISTS searches (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			query      TEXT NOT NULL,
			body       TEXT NOT NULL DEFAULT '{}',
			response   TEXT NOT NULL DEFAULT '{}',
			session    TEXT NOT NULL DEFAULT '',
			created_at INTEGER NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_searches_query   ON searches(query);
		CREATE INDEX IF NOT EXISTS idx_searches_session ON searches(session);
		CREATE INDEX IF NOT EXISTS idx_searches_ts      ON searches(created_at);

		CREATE VIRTUAL TABLE IF NOT EXISTS searches_fts USING fts5(
			query,
			response,
			content='searches',
			content_rowid='id'
		);
		CREATE TRIGGER IF NOT EXISTS searches_ai AFTER INSERT ON searches BEGIN
			INSERT INTO searches_fts(rowid, query, response) VALUES (new.id, new.query, new.response);
		END;
		CREATE TRIGGER IF NOT EXISTS searches_ad AFTER DELETE ON searches BEGIN
			INSERT INTO searches_fts(searches_fts, rowid, query, response) VALUES('delete', old.id, old.query, old.response);
		END;

		CREATE TABLE IF NOT EXISTS extracts (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			urls       TEXT NOT NULL DEFAULT '[]',
			response   TEXT NOT NULL DEFAULT '{}',
			session    TEXT NOT NULL DEFAULT '',
			created_at INTEGER NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_extracts_session ON extracts(session);
		CREATE INDEX IF NOT EXISTS idx_extracts_ts      ON extracts(created_at);

		CREATE VIRTUAL TABLE IF NOT EXISTS extracts_fts USING fts5(
			urls,
			response,
			content='extracts',
			content_rowid='id'
		);
		CREATE TRIGGER IF NOT EXISTS extracts_ai AFTER INSERT ON extracts BEGIN
			INSERT INTO extracts_fts(rowid, urls, response) VALUES (new.id, new.urls, new.response);
		END;
		CREATE TRIGGER IF NOT EXISTS extracts_ad AFTER DELETE ON extracts BEGIN
			INSERT INTO extracts_fts(extracts_fts, rowid, urls, response) VALUES('delete', old.id, old.urls, old.response);
		END;

		CREATE TABLE IF NOT EXISTS crawls (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			root_url      TEXT NOT NULL,
			params        TEXT NOT NULL DEFAULT '{}',
			pages_fetched INTEGER NOT NULL DEFAULT 0,
			status        TEXT NOT NULL DEFAULT 'running',
			checkpoint    TEXT NOT NULL DEFAULT '{}',
			session       TEXT NOT NULL DEFAULT '',
			created_at    INTEGER NOT NULL,
			updated_at    INTEGER NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_crawls_root    ON crawls(root_url);
		CREATE INDEX IF NOT EXISTS idx_crawls_session ON crawls(session);

		CREATE TABLE IF NOT EXISTS research_tasks (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			request_id TEXT NOT NULL UNIQUE,
			input      TEXT NOT NULL,
			status     TEXT NOT NULL DEFAULT 'pending',
			report     TEXT NOT NULL DEFAULT '',
			session    TEXT NOT NULL DEFAULT '',
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_research_request ON research_tasks(request_id);
		CREATE INDEX IF NOT EXISTS idx_research_session ON research_tasks(session);

		CREATE TABLE IF NOT EXISTS credits (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			endpoint   TEXT NOT NULL,
			credits    REAL NOT NULL DEFAULT 0,
			session    TEXT NOT NULL DEFAULT '',
			created_at INTEGER NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_credits_endpoint ON credits(endpoint);
		CREATE INDEX IF NOT EXISTS idx_credits_session  ON credits(session);
		CREATE INDEX IF NOT EXISTS idx_credits_ts       ON credits(created_at);
	`); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("apply migration: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration: %w", err)
	}
	return nil
}

// --- Search ---

// InsertSearch records a search call.
func (s *DB) InsertSearch(query, bodyJSON, responseJSON, session string) (int64, error) {
	res, err := s.db.Exec(
		`INSERT INTO searches(query, body, response, session, created_at) VALUES(?,?,?,?,?)`,
		query, bodyJSON, responseJSON, session, time.Now().Unix(),
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// SearchesByQuery returns the most recent stored result for a query.
func (s *DB) SearchesByQuery(query string, limit int) ([]SearchRow, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := s.db.Query(
		`SELECT id, query, body, response, session, created_at FROM searches WHERE query=? ORDER BY created_at DESC LIMIT ?`,
		query, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSearchRows(rows)
}

// quoteFTSTerm makes a user-supplied search term safe to bind to an FTS5
// MATCH clause. SQLite's FTS5 engine parses the bound value as a query
// expression, so bare keywords (AND/OR/NOT/NEAR) or stray punctuation cause
// hard parse errors. We wrap the term in FTS5 phrase quotes when it looks
// like plain prose, and leave it alone when the caller has explicitly
// opted into operator syntax (parens, quotes, or an uppercase operator
// surrounded by spaces).
// PATCH: defensive FTS5 quoting so common phrases like "AI AND ML" don't
// raise 'fts5: syntax error near AND' before reaching the matcher.
func quoteFTSTerm(term string) string {
	if strings.ContainsAny(term, `"()`) ||
		strings.Contains(term, " AND ") ||
		strings.Contains(term, " OR ") ||
		strings.Contains(term, " NOT ") ||
		strings.Contains(term, " NEAR ") {
		return term
	}
	return `"` + strings.ReplaceAll(term, `"`, `""`) + `"`
}

// FTSSearch runs full-text search over all stored search queries and responses.
func (s *DB) FTSSearch(term string, limit int) ([]SearchRow, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.Query(
		`SELECT s.id, s.query, s.body, s.response, s.session, s.created_at
		 FROM searches_fts f
		 JOIN searches s ON s.id = f.rowid
		 WHERE searches_fts MATCH ?
		 ORDER BY rank
		 LIMIT ?`,
		quoteFTSTerm(term), limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSearchRows(rows)
}

// FTSExtract runs full-text search over stored extracted content.
func (s *DB) FTSExtract(term string, limit int) ([]ExtractRow, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.Query(
		`SELECT e.id, e.urls, e.response, e.session, e.created_at
		 FROM extracts_fts f
		 JOIN extracts e ON e.id = f.rowid
		 WHERE extracts_fts MATCH ?
		 ORDER BY rank
		 LIMIT ?`,
		quoteFTSTerm(term), limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanExtractRows(rows)
}

// AllSessions returns all distinct session labels that have at least one search.
func (s *DB) AllSessions() ([]string, error) {
	rows, err := s.db.Query(`SELECT DISTINCT session FROM searches WHERE session != '' ORDER BY session`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var sess string
		if err := rows.Scan(&sess); err != nil {
			return nil, err
		}
		out = append(out, sess)
	}
	return out, rows.Err()
}

// SearchesBySession returns all search rows for a given session label.
func (s *DB) SearchesBySession(session string) ([]SearchRow, error) {
	rows, err := s.db.Query(
		`SELECT id, query, body, response, session, created_at FROM searches WHERE session=? ORDER BY created_at ASC`,
		session,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSearchRows(rows)
}

// ExtractsBySession returns all extract rows for a session.
func (s *DB) ExtractsBySession(session string) ([]ExtractRow, error) {
	rows, err := s.db.Query(
		`SELECT id, urls, response, session, created_at FROM extracts WHERE session=? ORDER BY created_at ASC`,
		session,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanExtractRows(rows)
}

// BaselineSearch returns the most recent stored result for query (for drift-detect).
func (s *DB) BaselineSearch(query string) (*SearchRow, error) {
	row := s.db.QueryRow(
		`SELECT id, query, body, response, session, created_at FROM searches WHERE query=? ORDER BY created_at DESC LIMIT 1`,
		query,
	)
	var r SearchRow
	var ts int64
	err := row.Scan(&r.ID, &r.Query, &r.Body, &r.Response, &r.Session, &ts)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	r.CreatedAt = time.Unix(ts, 0)
	return &r, nil
}

// --- Extract ---

// InsertExtract records an extract call.
func (s *DB) InsertExtract(urlsJSON, responseJSON, session string) (int64, error) {
	res, err := s.db.Exec(
		`INSERT INTO extracts(urls, response, session, created_at) VALUES(?,?,?,?)`,
		urlsJSON, responseJSON, session, time.Now().Unix(),
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ExtractedURLs returns the set of all URLs that have been extracted.
func (s *DB) ExtractedURLs() (map[string]bool, error) {
	rows, err := s.db.Query(`SELECT urls FROM extracts`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]bool)
	for rows.Next() {
		var urlsJSON string
		if err := rows.Scan(&urlsJSON); err != nil {
			return nil, err
		}
		var urls []string
		if err := json.Unmarshal([]byte(urlsJSON), &urls); err != nil {
			continue
		}
		for _, u := range urls {
			out[u] = true
		}
	}
	return out, rows.Err()
}

// ExtractsOlderThan returns extracted pages older than the given duration.
func (s *DB) ExtractsOlderThan(age time.Duration) ([]ExtractRow, error) {
	cutoff := time.Now().Add(-age).Unix()
	rows, err := s.db.Query(
		`SELECT id, urls, response, session, created_at FROM extracts WHERE created_at < ? ORDER BY created_at ASC`,
		cutoff,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanExtractRows(rows)
}

// --- Crawl ---

// InsertCrawl starts a crawl session record.
func (s *DB) InsertCrawl(rootURL, paramsJSON, session string) (int64, error) {
	now := time.Now().Unix()
	res, err := s.db.Exec(
		`INSERT INTO crawls(root_url, params, status, session, created_at, updated_at) VALUES(?,?,?,?,?,?)`,
		rootURL, paramsJSON, "running", session, now, now,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UpdateCrawlCheckpoint saves intermediate state for crawl resume.
func (s *DB) UpdateCrawlCheckpoint(id int64, pagesFetched int, checkpointJSON, status string) error {
	_, err := s.db.Exec(
		`UPDATE crawls SET pages_fetched=?, checkpoint=?, status=?, updated_at=? WHERE id=?`,
		pagesFetched, checkpointJSON, status, time.Now().Unix(), id,
	)
	return err
}

// InterruptedCrawls returns crawls in "running" or "interrupted" state.
func (s *DB) InterruptedCrawls(rootURL string) ([]CrawlRow, error) {
	rows, err := s.db.Query(
		`SELECT id, root_url, params, pages_fetched, status, checkpoint, session, created_at, updated_at
		 FROM crawls WHERE root_url=? AND status IN ('running','interrupted') ORDER BY created_at DESC`,
		rootURL,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCrawlRows(rows)
}

// --- Research ---

// InsertResearch records a new async research task.
func (s *DB) InsertResearch(requestID, input, session string) (int64, error) {
	now := time.Now().Unix()
	res, err := s.db.Exec(
		`INSERT OR IGNORE INTO research_tasks(request_id, input, status, session, created_at, updated_at) VALUES(?,?,?,?,?,?)`,
		requestID, input, "pending", session, now, now,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UpdateResearch updates the status and report for a research task.
func (s *DB) UpdateResearch(requestID, status, report string) error {
	_, err := s.db.Exec(
		`UPDATE research_tasks SET status=?, report=?, updated_at=? WHERE request_id=?`,
		status, report, time.Now().Unix(), requestID,
	)
	return err
}

// PendingResearch returns research tasks that are not yet complete.
func (s *DB) PendingResearch() ([]ResearchRow, error) {
	rows, err := s.db.Query(
		`SELECT id, request_id, input, status, report, session, created_at, updated_at
		 FROM research_tasks WHERE status NOT IN ('complete','failed') ORDER BY created_at ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanResearchRows(rows)
}

// GetResearch retrieves a research task by request_id.
func (s *DB) GetResearch(requestID string) (*ResearchRow, error) {
	row := s.db.QueryRow(
		`SELECT id, request_id, input, status, report, session, created_at, updated_at FROM research_tasks WHERE request_id=?`,
		requestID,
	)
	var r ResearchRow
	var createdAt, updatedAt int64
	err := row.Scan(&r.ID, &r.RequestID, &r.Input, &r.Status, &r.Report, &r.Session, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	r.CreatedAt = time.Unix(createdAt, 0)
	r.UpdatedAt = time.Unix(updatedAt, 0)
	return &r, nil
}

// --- Credits ---

// InsertCredit records a credit-spending event.
func (s *DB) InsertCredit(endpoint string, credits float64, session string) error {
	_, err := s.db.Exec(
		`INSERT INTO credits(endpoint, credits, session, created_at) VALUES(?,?,?,?)`,
		endpoint, credits, session, time.Now().Unix(),
	)
	return err
}

// CreditsSince returns total credits spent per endpoint since a timestamp.
func (s *DB) CreditsSince(since time.Time) (map[string]float64, error) {
	rows, err := s.db.Query(
		`SELECT endpoint, SUM(credits) FROM credits WHERE created_at >= ? GROUP BY endpoint`,
		since.Unix(),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]float64)
	for rows.Next() {
		var endpoint string
		var total float64
		if err := rows.Scan(&endpoint, &total); err != nil {
			return nil, err
		}
		out[endpoint] = total
	}
	return out, rows.Err()
}

// CreditsBySession returns total credits per session.
func (s *DB) CreditsBySession(session string) (float64, error) {
	row := s.db.QueryRow(`SELECT COALESCE(SUM(credits),0) FROM credits WHERE session=?`, session)
	var total float64
	if err := row.Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

// RatePerHour computes credits/hr over the last N hours.
func (s *DB) RatePerHour(hours int) (float64, error) {
	if hours <= 0 {
		hours = 24
	}
	since := time.Now().Add(-time.Duration(hours) * time.Hour).Unix()
	row := s.db.QueryRow(`SELECT COALESCE(SUM(credits),0) FROM credits WHERE created_at >= ?`, since)
	var total float64
	if err := row.Scan(&total); err != nil {
		return 0, err
	}
	return total / float64(hours), nil
}

// DomainScores returns domains ranked by average relevance score across stored searches.
func (s *DB) DomainScores(limit int) ([]DomainScore, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.Query(`SELECT response FROM searches ORDER BY created_at DESC LIMIT 500`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type scoreAccum struct {
		total float64
		count int
	}
	accum := make(map[string]*scoreAccum)

	for rows.Next() {
		var respJSON string
		if err := rows.Scan(&respJSON); err != nil {
			continue
		}
		var resp struct {
			Results []struct {
				URL   string  `json:"url"`
				Score float64 `json:"score"`
			} `json:"results"`
		}
		if err := json.Unmarshal([]byte(respJSON), &resp); err != nil {
			continue
		}
		for _, r := range resp.Results {
			domain := extractDomain(r.URL)
			if domain == "" {
				continue
			}
			if accum[domain] == nil {
				accum[domain] = &scoreAccum{}
			}
			accum[domain].total += r.Score
			accum[domain].count++
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	scores := make([]DomainScore, 0, len(accum))
	for domain, a := range accum {
		scores = append(scores, DomainScore{
			Domain:   domain,
			AvgScore: a.total / float64(a.count),
			Count:    a.count,
		})
	}
	// sort descending by avg score
	sortDomainScores(scores)
	if len(scores) > limit {
		scores = scores[:limit]
	}
	return scores, nil
}

// DomainScore is one entry from DomainScores.
type DomainScore struct {
	Domain   string
	AvgScore float64
	Count    int
}

// URLsMentionedInSearches returns URLs from all stored search result sets.
func (s *DB) URLsMentionedInSearches() (map[string]int, error) {
	rows, err := s.db.Query(`SELECT response FROM searches`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]int)
	for rows.Next() {
		var respJSON string
		if err := rows.Scan(&respJSON); err != nil {
			continue
		}
		var resp struct {
			Results []struct {
				URL string `json:"url"`
			} `json:"results"`
		}
		if err := json.Unmarshal([]byte(respJSON), &resp); err != nil {
			continue
		}
		for _, r := range resp.Results {
			if r.URL != "" {
				out[r.URL]++
			}
		}
	}
	return out, rows.Err()
}

// --- helpers ---

func scanSearchRows(rows *sql.Rows) ([]SearchRow, error) {
	var out []SearchRow
	for rows.Next() {
		var r SearchRow
		var ts int64
		if err := rows.Scan(&r.ID, &r.Query, &r.Body, &r.Response, &r.Session, &ts); err != nil {
			return nil, err
		}
		r.CreatedAt = time.Unix(ts, 0)
		out = append(out, r)
	}
	return out, rows.Err()
}

func scanExtractRows(rows *sql.Rows) ([]ExtractRow, error) {
	var out []ExtractRow
	for rows.Next() {
		var r ExtractRow
		var ts int64
		if err := rows.Scan(&r.ID, &r.URLs, &r.Response, &r.Session, &ts); err != nil {
			return nil, err
		}
		r.CreatedAt = time.Unix(ts, 0)
		out = append(out, r)
	}
	return out, rows.Err()
}

func scanCrawlRows(rows *sql.Rows) ([]CrawlRow, error) {
	var out []CrawlRow
	for rows.Next() {
		var r CrawlRow
		var createdAt, updatedAt int64
		if err := rows.Scan(&r.ID, &r.RootURL, &r.Params, &r.PagesFetched, &r.Status, &r.Checkpoint, &r.Session, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		r.CreatedAt = time.Unix(createdAt, 0)
		r.UpdatedAt = time.Unix(updatedAt, 0)
		out = append(out, r)
	}
	return out, rows.Err()
}

func scanResearchRows(rows *sql.Rows) ([]ResearchRow, error) {
	var out []ResearchRow
	for rows.Next() {
		var r ResearchRow
		var createdAt, updatedAt int64
		if err := rows.Scan(&r.ID, &r.RequestID, &r.Input, &r.Status, &r.Report, &r.Session, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		r.CreatedAt = time.Unix(createdAt, 0)
		r.UpdatedAt = time.Unix(updatedAt, 0)
		out = append(out, r)
	}
	return out, rows.Err()
}

func extractDomain(rawURL string) string {
	// simple extraction — strip scheme and path
	s := rawURL
	if i := len("https://"); len(s) > i && (s[:8] == "https://" || s[:7] == "http://") {
		if s[:8] == "https://" {
			s = s[8:]
		} else {
			s = s[7:]
		}
	}
	if i := len(s); i > 0 {
		for j, c := range s {
			if c == '/' || c == '?' || c == '#' {
				return s[:j]
			}
		}
		return s
	}
	return ""
}

func sortDomainScores(scores []DomainScore) {
	// insertion sort (small N)
	for i := 1; i < len(scores); i++ {
		for j := i; j > 0 && scores[j].AvgScore > scores[j-1].AvgScore; j-- {
			scores[j], scores[j-1] = scores[j-1], scores[j]
		}
	}
}
