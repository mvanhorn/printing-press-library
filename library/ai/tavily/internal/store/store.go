package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// DefaultDBPath returns ~/.tavily-pp-cli/tavily.db
func DefaultDBPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".tavily-pp-cli", "tavily.db")
}

// DB wraps a SQLite database connection.
type DB struct {
	db *sql.DB
}

// Open creates or opens the SQLite database at dbPath and runs migrations.
func Open(dbPath string) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("creating db dir: %w", err)
	}
	sqlDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening db: %w", err)
	}
	if _, err := sqlDB.Exec("PRAGMA journal_mode=WAL"); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("setting WAL mode: %w", err)
	}
	d := &DB{db: sqlDB}
	if err := d.migrate(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("migrating db: %w", err)
	}
	return d, nil
}

// Close closes the underlying database connection.
func (d *DB) Close() error {
	return d.db.Close()
}

func (d *DB) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS search_results (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			query TEXT NOT NULL,
			params_hash TEXT NOT NULL,
			results_json TEXT NOT NULL,
			answer TEXT,
			response_time REAL,
			credits_used INTEGER DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS extracted_pages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			url TEXT NOT NULL,
			content TEXT NOT NULL,
			format TEXT DEFAULT 'markdown',
			source TEXT DEFAULT 'extract',
			pipeline_id TEXT,
			fetched_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS crawl_results (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			base_url TEXT NOT NULL,
			urls_json TEXT NOT NULL,
			content_json TEXT NOT NULL,
			page_count INTEGER DEFAULT 0,
			fetched_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS map_results (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			base_url TEXT NOT NULL,
			urls_json TEXT NOT NULL,
			url_count INTEGER DEFAULT 0,
			fetched_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS research_reports (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			input_query TEXT NOT NULL,
			report_text TEXT NOT NULL,
			model TEXT DEFAULT 'auto',
			citation_format TEXT DEFAULT 'numbered',
			sources_json TEXT,
			response_time INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS usage_snapshots (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			plan TEXT,
			total_usage INTEGER DEFAULT 0,
			plan_limit INTEGER,
			search_usage INTEGER DEFAULT 0,
			extract_usage INTEGER DEFAULT 0,
			crawl_usage INTEGER DEFAULT 0,
			map_usage INTEGER DEFAULT 0,
			research_usage INTEGER DEFAULT 0,
			snapshot_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS research_fts USING fts5(input_query, report_text, content=research_reports, content_rowid=id)`,
	}
	for _, s := range stmts {
		if _, err := d.db.Exec(s); err != nil {
			return fmt.Errorf("exec %q: %w", s[:40], err)
		}
	}
	return nil
}

// --- Insert methods ---

func (d *DB) InsertSearchResult(query, paramsHash, resultsJSON, answer string, responseTime float64, creditsUsed int) (int64, error) {
	res, err := d.db.Exec(
		`INSERT INTO search_results (query, params_hash, results_json, answer, response_time, credits_used) VALUES (?,?,?,?,?,?)`,
		query, paramsHash, resultsJSON, answer, responseTime, creditsUsed,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (d *DB) InsertExtractedPage(url, content, format, source, pipelineID string) (int64, error) {
	res, err := d.db.Exec(
		`INSERT INTO extracted_pages (url, content, format, source, pipeline_id) VALUES (?,?,?,?,?)`,
		url, content, format, source, pipelineID,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (d *DB) InsertCrawlResult(baseURL string, urls []string, contentJSON string, pageCount int) (int64, error) {
	urlsJSON, _ := json.Marshal(urls)
	res, err := d.db.Exec(
		`INSERT INTO crawl_results (base_url, urls_json, content_json, page_count) VALUES (?,?,?,?)`,
		baseURL, string(urlsJSON), contentJSON, pageCount,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (d *DB) InsertMapResult(baseURL string, urls []string, urlCount int) (int64, error) {
	urlsJSON, _ := json.Marshal(urls)
	res, err := d.db.Exec(
		`INSERT INTO map_results (base_url, urls_json, url_count) VALUES (?,?,?)`,
		baseURL, string(urlsJSON), urlCount,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (d *DB) InsertResearchReport(inputQuery, reportText, model, citationFormat, sourcesJSON string, responseTime int) (int64, error) {
	res, err := d.db.Exec(
		`INSERT INTO research_reports (input_query, report_text, model, citation_format, sources_json, response_time) VALUES (?,?,?,?,?,?)`,
		inputQuery, reportText, model, citationFormat, sourcesJSON, responseTime,
	)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	// Sync FTS index
	_, _ = d.db.Exec(`INSERT INTO research_fts(rowid, input_query, report_text) VALUES (?,?,?)`, id, inputQuery, reportText)
	return id, nil
}

func (d *DB) InsertUsageSnapshot(plan string, totalUsage, planLimit, searchUsage, extractUsage, crawlUsage, mapUsage, researchUsage int) (int64, error) {
	res, err := d.db.Exec(
		`INSERT INTO usage_snapshots (plan, total_usage, plan_limit, search_usage, extract_usage, crawl_usage, map_usage, research_usage) VALUES (?,?,?,?,?,?,?,?)`,
		plan, totalUsage, planLimit, searchUsage, extractUsage, crawlUsage, mapUsage, researchUsage,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// --- Query methods ---

// MapResult represents a stored map result row.
type MapResult struct {
	ID        int64     `json:"id"`
	BaseURL   string    `json:"base_url"`
	URLs      []string  `json:"urls"`
	URLCount  int       `json:"url_count"`
	FetchedAt time.Time `json:"fetched_at"`
}

func (d *DB) GetMapResultsByBaseURL(baseURL string) ([]MapResult, error) {
	rows, err := d.db.Query(
		`SELECT id, base_url, urls_json, url_count, fetched_at FROM map_results WHERE base_url=? ORDER BY fetched_at DESC`,
		baseURL,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []MapResult
	for rows.Next() {
		var r MapResult
		var urlsJSON string
		if err := rows.Scan(&r.ID, &r.BaseURL, &urlsJSON, &r.URLCount, &r.FetchedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(urlsJSON), &r.URLs)
		results = append(results, r)
	}
	return results, rows.Err()
}

// SearchResult represents a stored search result row.
type SearchResult struct {
	ID           int64     `json:"id"`
	Query        string    `json:"query"`
	ParamsHash   string    `json:"params_hash"`
	ResultsJSON  string    `json:"results_json"`
	Answer       string    `json:"answer"`
	ResponseTime float64   `json:"response_time"`
	CreditsUsed  int       `json:"credits_used"`
	CreatedAt    time.Time `json:"created_at"`
}

func (d *DB) GetSearchResultsByQuery(query string) ([]SearchResult, error) {
	rows, err := d.db.Query(
		`SELECT id, query, params_hash, results_json, answer, response_time, credits_used, created_at FROM search_results WHERE query=? ORDER BY created_at DESC`,
		query,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		var answer sql.NullString
		if err := rows.Scan(&r.ID, &r.Query, &r.ParamsHash, &r.ResultsJSON, &answer, &r.ResponseTime, &r.CreditsUsed, &r.CreatedAt); err != nil {
			return nil, err
		}
		if answer.Valid {
			r.Answer = answer.String
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// ResearchMatch represents an FTS search match.
type ResearchMatch struct {
	ID         int64     `json:"id"`
	InputQuery string    `json:"input_query"`
	Excerpt    string    `json:"excerpt"`
	CreatedAt  time.Time `json:"created_at"`
}

func (d *DB) SearchResearchReports(terms string, limit int) ([]ResearchMatch, error) {
	rows, err := d.db.Query(
		`SELECT r.id, r.input_query, snippet(research_fts, 1, '>>>', '<<<', '...', 40) as excerpt, r.created_at
		 FROM research_fts f
		 JOIN research_reports r ON f.rowid = r.id
		 WHERE research_fts MATCH ?
		 ORDER BY rank
		 LIMIT ?`,
		terms, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []ResearchMatch
	for rows.Next() {
		var r ResearchMatch
		if err := rows.Scan(&r.ID, &r.InputQuery, &r.Excerpt, &r.CreatedAt); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// StaleItem represents a page or crawl result older than a threshold.
type StaleItem struct {
	Type      string    `json:"type"`
	URL       string    `json:"url"`
	FetchedAt time.Time `json:"fetched_at"`
	AgeDays   int       `json:"age_days"`
}

func (d *DB) GetStaleContent(days int) ([]StaleItem, error) {
	cutoff := time.Now().AddDate(0, 0, -days).Format("2006-01-02 15:04:05")
	var items []StaleItem

	// Stale extracted pages
	rows, err := d.db.Query(
		`SELECT url, fetched_at FROM extracted_pages WHERE fetched_at < ? ORDER BY fetched_at ASC`, cutoff,
	)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var s StaleItem
		s.Type = "extracted_page"
		if err := rows.Scan(&s.URL, &s.FetchedAt); err != nil {
			rows.Close()
			return nil, err
		}
		s.AgeDays = int(time.Since(s.FetchedAt).Hours() / 24)
		items = append(items, s)
	}
	rows.Close()

	// Stale crawl results
	rows, err = d.db.Query(
		`SELECT base_url, fetched_at FROM crawl_results WHERE fetched_at < ? ORDER BY fetched_at ASC`, cutoff,
	)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var s StaleItem
		s.Type = "crawl_result"
		if err := rows.Scan(&s.URL, &s.FetchedAt); err != nil {
			rows.Close()
			return nil, err
		}
		s.AgeDays = int(time.Since(s.FetchedAt).Hours() / 24)
		items = append(items, s)
	}
	rows.Close()

	return items, nil
}

// UsageSnapshot represents a stored usage snapshot row.
type UsageSnapshot struct {
	ID            int64     `json:"id"`
	Plan          string    `json:"plan"`
	TotalUsage    int       `json:"total_usage"`
	PlanLimit     int       `json:"plan_limit"`
	SearchUsage   int       `json:"search_usage"`
	ExtractUsage  int       `json:"extract_usage"`
	CrawlUsage    int       `json:"crawl_usage"`
	MapUsage      int       `json:"map_usage"`
	ResearchUsage int       `json:"research_usage"`
	SnapshotAt    time.Time `json:"snapshot_at"`
}

func (d *DB) GetUsageHistory(days int) ([]UsageSnapshot, error) {
	cutoff := time.Now().AddDate(0, 0, -days).Format("2006-01-02 15:04:05")
	rows, err := d.db.Query(
		`SELECT id, plan, total_usage, plan_limit, search_usage, extract_usage, crawl_usage, map_usage, research_usage, snapshot_at
		 FROM usage_snapshots WHERE snapshot_at >= ? ORDER BY snapshot_at ASC`, cutoff,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []UsageSnapshot
	for rows.Next() {
		var s UsageSnapshot
		var plan sql.NullString
		var planLimit sql.NullInt64
		if err := rows.Scan(&s.ID, &plan, &s.TotalUsage, &planLimit, &s.SearchUsage, &s.ExtractUsage, &s.CrawlUsage, &s.MapUsage, &s.ResearchUsage, &s.SnapshotAt); err != nil {
			return nil, err
		}
		if plan.Valid {
			s.Plan = plan.String
		}
		if planLimit.Valid {
			s.PlanLimit = int(planLimit.Int64)
		}
		results = append(results, s)
	}
	return results, rows.Err()
}
