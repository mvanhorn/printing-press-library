// Package store is a minimal local SQLite cache for SurgeGraph entities
// that need historical snapshots (AI Visibility metrics, citations, usage)
// or full-text search across multi-entity local state. Pure Go via
// modernc.org/sqlite — no cgo required.
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	DB   *sql.DB
	Path string
}

// DefaultPath returns the canonical local store path:
// ~/.local/share/surgegraph-pp-cli/store.db.
func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "store.db"
	}
	return filepath.Join(home, ".local", "share", "surgegraph-pp-cli", "store.db")
}

// Open opens the SQLite database at path (creating parent dirs as needed)
// and runs idempotent migrations.
func Open(path string) (*Store, error) {
	if path == "" {
		path = DefaultPath()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("creating store dir: %w", err)
	}
	db, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("opening store: %w", err)
	}
	db.SetMaxOpenConns(1)
	s := &Store{DB: db, Path: path}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.DB.Close() }

func (s *Store) migrate() error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_, err := s.DB.ExecContext(ctx, schemaSQL)
	return err
}

const schemaSQL = `
CREATE TABLE IF NOT EXISTS projects (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	brand_name TEXT,
	raw_json TEXT NOT NULL,
	last_synced_at TIMESTAMP
);

CREATE TABLE IF NOT EXISTS prompts (
	id TEXT PRIMARY KEY,
	project_id TEXT NOT NULL,
	text TEXT NOT NULL,
	raw_json TEXT NOT NULL,
	last_seen_at TIMESTAMP NOT NULL,
	FOREIGN KEY (project_id) REFERENCES projects(id)
);
CREATE INDEX IF NOT EXISTS idx_prompts_project ON prompts(project_id);

CREATE TABLE IF NOT EXISTS visibility_snapshots (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	project_id TEXT NOT NULL,
	brand_name TEXT NOT NULL,
	model_id TEXT NOT NULL DEFAULT '',
	snapshot_date DATE NOT NULL,
	metric_type TEXT NOT NULL,
	payload TEXT NOT NULL,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE (project_id, brand_name, model_id, snapshot_date, metric_type)
);
CREATE INDEX IF NOT EXISTS idx_vis_date ON visibility_snapshots(project_id, snapshot_date);
CREATE INDEX IF NOT EXISTS idx_vis_metric ON visibility_snapshots(project_id, metric_type, snapshot_date);

CREATE TABLE IF NOT EXISTS citation_snapshots (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	project_id TEXT NOT NULL,
	snapshot_date DATE NOT NULL,
	domain TEXT NOT NULL,
	citation_count INTEGER NOT NULL,
	url_count INTEGER NOT NULL DEFAULT 0,
	raw_json TEXT NOT NULL,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE (project_id, snapshot_date, domain)
);
CREATE INDEX IF NOT EXISTS idx_cites_date ON citation_snapshots(project_id, snapshot_date);

CREATE TABLE IF NOT EXISTS prompt_runs (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	prompt_id TEXT NOT NULL,
	project_id TEXT NOT NULL,
	snapshot_date DATE NOT NULL,
	citation_count INTEGER NOT NULL DEFAULT 0,
	first_position INTEGER,
	visibility REAL,
	raw_json TEXT NOT NULL,
	UNIQUE (prompt_id, snapshot_date)
);
CREATE INDEX IF NOT EXISTS idx_pr_prompt ON prompt_runs(prompt_id, snapshot_date);
CREATE INDEX IF NOT EXISTS idx_pr_project ON prompt_runs(project_id, snapshot_date);

CREATE TABLE IF NOT EXISTS traffic_pages (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	project_id TEXT NOT NULL,
	snapshot_date DATE NOT NULL,
	url TEXT NOT NULL,
	ai_visits INTEGER NOT NULL DEFAULT 0,
	human_visits INTEGER NOT NULL DEFAULT 0,
	ctr REAL,
	raw_json TEXT NOT NULL,
	UNIQUE (project_id, snapshot_date, url)
);
CREATE INDEX IF NOT EXISTS idx_traffic_project ON traffic_pages(project_id, snapshot_date);

CREATE TABLE IF NOT EXISTS documents (
	id TEXT PRIMARY KEY,
	project_id TEXT NOT NULL,
	kind TEXT NOT NULL,
	title TEXT NOT NULL,
	updated_at TIMESTAMP,
	raw_json TEXT NOT NULL,
	last_seen_at TIMESTAMP NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_docs_project ON documents(project_id, kind);

CREATE TABLE IF NOT EXISTS knowledge_libraries (
	id TEXT PRIMARY KEY,
	project_id TEXT NOT NULL,
	name TEXT NOT NULL,
	raw_json TEXT NOT NULL,
	last_seen_at TIMESTAMP NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_kl_project ON knowledge_libraries(project_id);

CREATE TABLE IF NOT EXISTS knowledge_library_documents (
	id TEXT PRIMARY KEY,
	library_id TEXT NOT NULL,
	title TEXT,
	url TEXT,
	raw_json TEXT NOT NULL,
	last_seen_at TIMESTAMP NOT NULL,
	FOREIGN KEY (library_id) REFERENCES knowledge_libraries(id)
);
CREATE INDEX IF NOT EXISTS idx_kld_library ON knowledge_library_documents(library_id);
CREATE INDEX IF NOT EXISTS idx_kld_url ON knowledge_library_documents(url);

CREATE TABLE IF NOT EXISTS usage_snapshots (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	snapshot_date DATE NOT NULL UNIQUE,
	credits_total INTEGER,
	credits_used INTEGER,
	credits_remaining INTEGER,
	raw_json TEXT NOT NULL,
	created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS topic_researches (
	id TEXT PRIMARY KEY,
	project_id TEXT NOT NULL,
	seed_topic TEXT,
	status TEXT,
	raw_json TEXT NOT NULL,
	last_seen_at TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS domain_researches (
	id TEXT PRIMARY KEY,
	project_id TEXT NOT NULL,
	domain TEXT,
	status TEXT,
	raw_json TEXT NOT NULL,
	last_seen_at TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS sync_cursors (
	resource TEXT NOT NULL,
	project_id TEXT NOT NULL DEFAULT '',
	last_synced_at TIMESTAMP NOT NULL,
	row_count INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (resource, project_id)
);

CREATE VIRTUAL TABLE IF NOT EXISTS cache_fts USING fts5(
	kind UNINDEXED,
	id UNINDEXED,
	project_id UNINDEXED,
	title,
	body,
	tokenize='porter unicode61'
);
`

// ---------- Projects ----------

type Project struct {
	ID           string
	Name         string
	BrandName    string
	RawJSON      string
	LastSyncedAt time.Time
}

func (s *Store) UpsertProject(ctx context.Context, p Project) error {
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO projects (id, name, brand_name, raw_json, last_synced_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			brand_name = excluded.brand_name,
			raw_json = excluded.raw_json,
			last_synced_at = excluded.last_synced_at
	`, p.ID, p.Name, p.BrandName, p.RawJSON, p.LastSyncedAt)
	return err
}

func (s *Store) ListProjects(ctx context.Context) ([]Project, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id, name, COALESCE(brand_name,''), raw_json, last_synced_at FROM projects ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Project
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.Name, &p.BrandName, &p.RawJSON, &p.LastSyncedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ---------- Visibility snapshots ----------

type VisibilitySnapshot struct {
	ProjectID    string
	BrandName    string
	ModelID      string
	SnapshotDate string // YYYY-MM-DD
	MetricType   string // overview|trend|sentiment|traffic_summary
	Payload      json.RawMessage
}

func (s *Store) UpsertVisibilitySnapshot(ctx context.Context, v VisibilitySnapshot) error {
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO visibility_snapshots (project_id, brand_name, model_id, snapshot_date, metric_type, payload)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(project_id, brand_name, model_id, snapshot_date, metric_type)
		DO UPDATE SET payload = excluded.payload, created_at = CURRENT_TIMESTAMP
	`, v.ProjectID, v.BrandName, v.ModelID, v.SnapshotDate, v.MetricType, string(v.Payload))
	return err
}

// LatestSnapshotPair returns the two most-recent snapshot rows for one
// (project, brand, metric_type) tuple. Returns (newer, older, ok).
func (s *Store) LatestSnapshotPair(ctx context.Context, projectID, brandName, metric string) (newer, older *VisibilitySnapshot, err error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT brand_name, COALESCE(model_id,''), snapshot_date, metric_type, payload
		FROM visibility_snapshots
		WHERE project_id = ? AND brand_name = ? AND metric_type = ?
		ORDER BY snapshot_date DESC
		LIMIT 2
	`, projectID, brandName, metric)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var snaps []VisibilitySnapshot
	for rows.Next() {
		v := VisibilitySnapshot{ProjectID: projectID}
		var payload string
		if err := rows.Scan(&v.BrandName, &v.ModelID, &v.SnapshotDate, &v.MetricType, &payload); err != nil {
			return nil, nil, err
		}
		v.Payload = json.RawMessage(payload)
		snaps = append(snaps, v)
	}
	switch len(snaps) {
	case 0:
		return nil, nil, nil
	case 1:
		return &snaps[0], nil, nil
	default:
		return &snaps[0], &snaps[1], nil
	}
}

// ---------- Citation snapshots ----------

type CitationSnapshot struct {
	ProjectID     string
	SnapshotDate  string
	Domain        string
	CitationCount int
	URLCount      int
	RawJSON       json.RawMessage
}

func (s *Store) UpsertCitationSnapshot(ctx context.Context, c CitationSnapshot) error {
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO citation_snapshots (project_id, snapshot_date, domain, citation_count, url_count, raw_json)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(project_id, snapshot_date, domain) DO UPDATE SET
			citation_count = excluded.citation_count,
			url_count = excluded.url_count,
			raw_json = excluded.raw_json
	`, c.ProjectID, c.SnapshotDate, c.Domain, c.CitationCount, c.URLCount, string(c.RawJSON))
	return err
}

// CitationRanks returns a flat list of (snapshot_date, domain, count) rows for the project
// within the time window (inclusive), ordered newest first.
func (s *Store) CitationRanksSince(ctx context.Context, projectID string, since time.Time) ([]CitationSnapshot, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT snapshot_date, domain, citation_count, url_count, raw_json
		FROM citation_snapshots
		WHERE project_id = ? AND snapshot_date >= ?
		ORDER BY snapshot_date DESC, citation_count DESC
	`, projectID, since.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CitationSnapshot
	for rows.Next() {
		c := CitationSnapshot{ProjectID: projectID}
		var raw string
		if err := rows.Scan(&c.SnapshotDate, &c.Domain, &c.CitationCount, &c.URLCount, &raw); err != nil {
			return nil, err
		}
		c.RawJSON = json.RawMessage(raw)
		out = append(out, c)
	}
	return out, rows.Err()
}

// ---------- Prompt runs (date-bucketed citation/position per prompt) ----------

type PromptRun struct {
	PromptID      string
	ProjectID     string
	SnapshotDate  string
	CitationCount int
	FirstPosition sql.NullInt64
	Visibility    sql.NullFloat64
	RawJSON       json.RawMessage
}

func (s *Store) UpsertPromptRun(ctx context.Context, r PromptRun) error {
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO prompt_runs (prompt_id, project_id, snapshot_date, citation_count, first_position, visibility, raw_json)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(prompt_id, snapshot_date) DO UPDATE SET
			citation_count = excluded.citation_count,
			first_position = excluded.first_position,
			visibility = excluded.visibility,
			raw_json = excluded.raw_json
	`, r.PromptID, r.ProjectID, r.SnapshotDate, r.CitationCount, r.FirstPosition, r.Visibility, string(r.RawJSON))
	return err
}

// PromptDeltas returns per-prompt aggregates comparing two snapshot windows.
// "newer" runs are runs with snapshot_date in (sinceDate, now]; "older" runs
// are runs at or before sinceDate. Negative deltas indicate decline.
type PromptDelta struct {
	PromptID         string
	Prompt           string
	NewerCitations   int
	OlderCitations   int
	CitationDelta    int
	NewerFirstPos    int
	OlderFirstPos    int
	PositionDelta    int // negative = improved (lower position is better)
	LatestSnapshotAt string
}

func (s *Store) PromptDeltas(ctx context.Context, projectID string, since time.Time) ([]PromptDelta, error) {
	rows, err := s.DB.QueryContext(ctx, `
		WITH newer AS (
			SELECT prompt_id,
				AVG(citation_count) AS avg_c,
				AVG(COALESCE(first_position, 99)) AS avg_pos,
				MAX(snapshot_date) AS latest
			FROM prompt_runs
			WHERE project_id = ? AND snapshot_date > ?
			GROUP BY prompt_id
		),
		older AS (
			SELECT prompt_id,
				AVG(citation_count) AS avg_c,
				AVG(COALESCE(first_position, 99)) AS avg_pos
			FROM prompt_runs
			WHERE project_id = ? AND snapshot_date <= ?
			GROUP BY prompt_id
		)
		SELECT newer.prompt_id,
			COALESCE(p.text, ''),
			CAST(newer.avg_c AS INTEGER), CAST(COALESCE(older.avg_c, 0) AS INTEGER),
			CAST(newer.avg_pos AS INTEGER), CAST(COALESCE(older.avg_pos, 99) AS INTEGER),
			newer.latest
		FROM newer
		LEFT JOIN older ON older.prompt_id = newer.prompt_id
		LEFT JOIN prompts p ON p.id = newer.prompt_id
		ORDER BY (CAST(newer.avg_c AS INTEGER) - CAST(COALESCE(older.avg_c,0) AS INTEGER)) ASC
	`, projectID, since.Format("2006-01-02"), projectID, since.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PromptDelta
	for rows.Next() {
		var d PromptDelta
		if err := rows.Scan(&d.PromptID, &d.Prompt, &d.NewerCitations, &d.OlderCitations,
			&d.NewerFirstPos, &d.OlderFirstPos, &d.LatestSnapshotAt); err != nil {
			return nil, err
		}
		d.CitationDelta = d.NewerCitations - d.OlderCitations
		d.PositionDelta = d.NewerFirstPos - d.OlderFirstPos
		out = append(out, d)
	}
	return out, rows.Err()
}

// ---------- Traffic pages ----------

type TrafficPage struct {
	ProjectID    string
	SnapshotDate string
	URL          string
	AIVisits     int
	HumanVisits  int
	CTR          sql.NullFloat64
	RawJSON      json.RawMessage
}

func (s *Store) UpsertTrafficPage(ctx context.Context, t TrafficPage) error {
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO traffic_pages (project_id, snapshot_date, url, ai_visits, human_visits, ctr, raw_json)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(project_id, snapshot_date, url) DO UPDATE SET
			ai_visits = excluded.ai_visits,
			human_visits = excluded.human_visits,
			ctr = excluded.ctr,
			raw_json = excluded.raw_json
	`, t.ProjectID, t.SnapshotDate, t.URL, t.AIVisits, t.HumanVisits, t.CTR, string(t.RawJSON))
	return err
}

func (s *Store) TopTrafficPages(ctx context.Context, projectID string, limit int) ([]TrafficPage, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.DB.QueryContext(ctx, `
		SELECT project_id, snapshot_date, url, ai_visits, human_visits, ctr, raw_json
		FROM traffic_pages
		WHERE project_id = ?
		ORDER BY ai_visits DESC, snapshot_date DESC
		LIMIT ?
	`, projectID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TrafficPage
	for rows.Next() {
		var t TrafficPage
		var raw string
		if err := rows.Scan(&t.ProjectID, &t.SnapshotDate, &t.URL, &t.AIVisits, &t.HumanVisits, &t.CTR, &raw); err != nil {
			return nil, err
		}
		t.RawJSON = json.RawMessage(raw)
		out = append(out, t)
	}
	return out, rows.Err()
}

// ---------- Documents ----------

type Document struct {
	ID         string
	ProjectID  string
	Kind       string // writer|optimized
	Title      string
	UpdatedAt  sql.NullTime
	RawJSON    json.RawMessage
	LastSeenAt time.Time
}

func (s *Store) UpsertDocument(ctx context.Context, d Document) error {
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO documents (id, project_id, kind, title, updated_at, raw_json, last_seen_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			project_id = excluded.project_id,
			kind = excluded.kind,
			title = excluded.title,
			updated_at = excluded.updated_at,
			raw_json = excluded.raw_json,
			last_seen_at = excluded.last_seen_at
	`, d.ID, d.ProjectID, d.Kind, d.Title, d.UpdatedAt, string(d.RawJSON), d.LastSeenAt)
	return err
}

type StaleDoc struct {
	ID           string
	ProjectID    string
	Kind         string
	Title        string
	UpdatedAt    time.Time
	AITraffic30d int
}

func (s *Store) StaleDocs(ctx context.Context, projectID string, olderThanDays int) ([]StaleDoc, error) {
	cutoff := time.Now().UTC().Add(-time.Duration(olderThanDays) * 24 * time.Hour)
	rows, err := s.DB.QueryContext(ctx, `
		SELECT d.id, d.project_id, d.kind, d.title, COALESCE(d.updated_at, d.last_seen_at) AS up,
			COALESCE((SELECT SUM(ai_visits) FROM traffic_pages t WHERE t.project_id = d.project_id AND t.url LIKE '%' || d.id || '%' AND t.snapshot_date >= date('now','-30 days')), 0)
		FROM documents d
		WHERE d.project_id = ? AND COALESCE(d.updated_at, d.last_seen_at) <= ?
		ORDER BY 6 DESC, up ASC
	`, projectID, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []StaleDoc
	for rows.Next() {
		var d StaleDoc
		if err := rows.Scan(&d.ID, &d.ProjectID, &d.Kind, &d.Title, &d.UpdatedAt, &d.AITraffic30d); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// ---------- Knowledge libraries ----------

type KnowledgeLibrary struct {
	ID         string
	ProjectID  string
	Name       string
	RawJSON    json.RawMessage
	LastSeenAt time.Time
}

func (s *Store) UpsertKnowledgeLibrary(ctx context.Context, k KnowledgeLibrary) error {
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO knowledge_libraries (id, project_id, name, raw_json, last_seen_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			project_id = excluded.project_id,
			name = excluded.name,
			raw_json = excluded.raw_json,
			last_seen_at = excluded.last_seen_at
	`, k.ID, k.ProjectID, k.Name, string(k.RawJSON), k.LastSeenAt)
	return err
}

type KnowledgeLibraryDocument struct {
	ID         string
	LibraryID  string
	Title      string
	URL        string
	RawJSON    json.RawMessage
	LastSeenAt time.Time
}

func (s *Store) UpsertKnowledgeLibraryDocument(ctx context.Context, d KnowledgeLibraryDocument) error {
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO knowledge_library_documents (id, library_id, title, url, raw_json, last_seen_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			library_id = excluded.library_id,
			title = excluded.title,
			url = excluded.url,
			raw_json = excluded.raw_json,
			last_seen_at = excluded.last_seen_at
	`, d.ID, d.LibraryID, d.Title, d.URL, string(d.RawJSON), d.LastSeenAt)
	return err
}

// KnowledgeImpact joins library documents with citation_snapshots to score
// how often a library's URLs appear in tracked AI citations.
type KnowledgeImpactRow struct {
	LibraryID     string `json:"library_id"`
	LibraryName   string `json:"library_name"`
	DocCount      int    `json:"doc_count"`
	CitedDocCount int    `json:"cited_doc_count"`
	TotalCites    int    `json:"total_cites"`
}

func (s *Store) KnowledgeImpact(ctx context.Context, projectID string) ([]KnowledgeImpactRow, error) {
	// PATCH(greptile-10): the original SQL used `instr(cs.raw_json, kld.url) > 0`
	// to detect citation references — the same raw-blob substring match that
	// PATCH(greptile-9) replaced in visibility traffic-citations. A doc URL
	// like https://example.com/blog would prefix-match unrelated citation
	// URLs (.../blog/post-extended) and would also match the doc URL string
	// appearing inside an image src, canonical href, or author link embedded
	// in the snapshot blob — both paths inflated cited_doc_count and
	// total_cites. The fix moves the join into Go: parse each citation
	// snapshot once, extract the URL-shaped strings, then exact-match
	// per-library doc URLs against the snapshot's URL set.
	libRows, err := s.DB.QueryContext(ctx, `
		SELECT kl.id, kl.name,
			(SELECT COUNT(*) FROM knowledge_library_documents kld WHERE kld.library_id = kl.id)
		FROM knowledge_libraries kl
		WHERE kl.project_id = ?
	`, projectID)
	if err != nil {
		return nil, err
	}
	type libInfo struct {
		id, name string
		docCount int
	}
	var libs []libInfo
	for libRows.Next() {
		var li libInfo
		if err := libRows.Scan(&li.id, &li.name, &li.docCount); err != nil {
			libRows.Close()
			return nil, err
		}
		libs = append(libs, li)
	}
	libRows.Close()
	if err := libRows.Err(); err != nil {
		return nil, err
	}
	if len(libs) == 0 {
		return nil, nil
	}

	docsByLib := make(map[string]map[string]struct{}, len(libs))
	docRows, err := s.DB.QueryContext(ctx, `
		SELECT kld.library_id, kld.url
		FROM knowledge_library_documents kld
		JOIN knowledge_libraries kl ON kl.id = kld.library_id
		WHERE kl.project_id = ?
	`, projectID)
	if err != nil {
		return nil, err
	}
	for docRows.Next() {
		var libID, url string
		if err := docRows.Scan(&libID, &url); err != nil {
			docRows.Close()
			return nil, err
		}
		n := normalizeCitationURL(url)
		if n == "" {
			continue
		}
		if docsByLib[libID] == nil {
			docsByLib[libID] = map[string]struct{}{}
		}
		docsByLib[libID][n] = struct{}{}
	}
	docRows.Close()
	if err := docRows.Err(); err != nil {
		return nil, err
	}

	snapRows, err := s.DB.QueryContext(ctx, `
		SELECT cs.raw_json, cs.citation_count
		FROM citation_snapshots cs
		WHERE cs.project_id = ?
	`, projectID)
	if err != nil {
		return nil, err
	}
	type parsedSnap struct {
		urls  map[string]struct{}
		cites int
	}
	var snaps []parsedSnap
	for snapRows.Next() {
		// modernc.org/sqlite returns TEXT columns as Go string; scan into a
		// string and convert, matching the pattern used by CitationRanksSince.
		var raw string
		var cites int
		if err := snapRows.Scan(&raw, &cites); err != nil {
			snapRows.Close()
			return nil, err
		}
		urls := extractCitationURLs(json.RawMessage(raw))
		if len(urls) == 0 {
			continue
		}
		snaps = append(snaps, parsedSnap{urls: urls, cites: cites})
	}
	snapRows.Close()
	if err := snapRows.Err(); err != nil {
		return nil, err
	}

	out := make([]KnowledgeImpactRow, 0, len(libs))
	for _, lib := range libs {
		row := KnowledgeImpactRow{LibraryID: lib.id, LibraryName: lib.name, DocCount: lib.docCount}
		docs := docsByLib[lib.id]
		if len(docs) > 0 && len(snaps) > 0 {
			cited := map[string]struct{}{}
			for _, sn := range snaps {
				hit := false
				for u := range docs {
					if _, ok := sn.urls[u]; ok {
						cited[u] = struct{}{}
						hit = true
					}
				}
				if hit {
					row.TotalCites += sn.cites
				}
			}
			row.CitedDocCount = len(cited)
		}
		out = append(out, row)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TotalCites > out[j].TotalCites })
	return out, nil
}

// normalizeCitationURL lowercases, trims whitespace, and strips a trailing
// slash so exact comparison is resilient to inconsequential differences.
// Parallels the helper in internal/cli/visibility_novel.go; duplicated
// rather than imported because the cli package depends on store, not the
// other way around.
func normalizeCitationURL(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	s = strings.TrimRight(s, "/")
	return strings.ToLower(s)
}

// extractCitationURLs parses a citation_snapshots.raw_json blob and returns
// the set of URL-shaped strings it contains, normalized for exact match.
// Only http(s) strings are extracted; arbitrary string fields (titles,
// snippets) are ignored so they cannot accidentally match a library
// document URL.
func extractCitationURLs(raw json.RawMessage) map[string]struct{} {
	if len(raw) == 0 {
		return nil
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil
	}
	out := map[string]struct{}{}
	collectURLs(v, out)
	return out
}

func collectURLs(v any, into map[string]struct{}) {
	switch x := v.(type) {
	case string:
		if isLikelyHTTPURL(x) {
			if n := normalizeCitationURL(x); n != "" {
				into[n] = struct{}{}
			}
		}
	case map[string]any:
		for _, vv := range x {
			collectURLs(vv, into)
		}
	case []any:
		for _, item := range x {
			collectURLs(item, into)
		}
	}
}

func isLikelyHTTPURL(s string) bool {
	t := strings.TrimSpace(s)
	return strings.HasPrefix(t, "http://") || strings.HasPrefix(t, "https://")
}

// ---------- Usage snapshots ----------

type UsageSnapshot struct {
	Date             string
	CreditsTotal     sql.NullInt64
	CreditsUsed      sql.NullInt64
	CreditsRemaining sql.NullInt64
	RawJSON          json.RawMessage
}

func (s *Store) UpsertUsageSnapshot(ctx context.Context, u UsageSnapshot) error {
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO usage_snapshots (snapshot_date, credits_total, credits_used, credits_remaining, raw_json)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(snapshot_date) DO UPDATE SET
			credits_total = excluded.credits_total,
			credits_used = excluded.credits_used,
			credits_remaining = excluded.credits_remaining,
			raw_json = excluded.raw_json
	`, u.Date, u.CreditsTotal, u.CreditsUsed, u.CreditsRemaining, string(u.RawJSON))
	return err
}

func (s *Store) UsageHistory(ctx context.Context, since time.Time) ([]UsageSnapshot, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT snapshot_date, credits_total, credits_used, credits_remaining, raw_json
		FROM usage_snapshots
		WHERE snapshot_date >= ?
		ORDER BY snapshot_date ASC
	`, since.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []UsageSnapshot
	for rows.Next() {
		var u UsageSnapshot
		var raw string
		if err := rows.Scan(&u.Date, &u.CreditsTotal, &u.CreditsUsed, &u.CreditsRemaining, &raw); err != nil {
			return nil, err
		}
		u.RawJSON = json.RawMessage(raw)
		out = append(out, u)
	}
	return out, rows.Err()
}

// ---------- Prompts (entity registry) ----------

type Prompt struct {
	ID         string
	ProjectID  string
	Text       string
	RawJSON    json.RawMessage
	LastSeenAt time.Time
}

func (s *Store) UpsertPrompt(ctx context.Context, p Prompt) error {
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO prompts (id, project_id, text, raw_json, last_seen_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			project_id = excluded.project_id,
			text = excluded.text,
			raw_json = excluded.raw_json,
			last_seen_at = excluded.last_seen_at
	`, p.ID, p.ProjectID, p.Text, string(p.RawJSON), p.LastSeenAt)
	return err
}

// ---------- Topic / domain research ----------

type TopicResearch struct {
	ID         string
	ProjectID  string
	SeedTopic  string
	Status     string
	RawJSON    json.RawMessage
	LastSeenAt time.Time
}

func (s *Store) UpsertTopicResearch(ctx context.Context, t TopicResearch) error {
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO topic_researches (id, project_id, seed_topic, status, raw_json, last_seen_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			project_id = excluded.project_id,
			seed_topic = excluded.seed_topic,
			status = excluded.status,
			raw_json = excluded.raw_json,
			last_seen_at = excluded.last_seen_at
	`, t.ID, t.ProjectID, t.SeedTopic, t.Status, string(t.RawJSON), t.LastSeenAt)
	return err
}

type DomainResearch struct {
	ID         string
	ProjectID  string
	Domain     string
	Status     string
	RawJSON    json.RawMessage
	LastSeenAt time.Time
}

func (s *Store) UpsertDomainResearch(ctx context.Context, d DomainResearch) error {
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO domain_researches (id, project_id, domain, status, raw_json, last_seen_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			project_id = excluded.project_id,
			domain = excluded.domain,
			status = excluded.status,
			raw_json = excluded.raw_json,
			last_seen_at = excluded.last_seen_at
	`, d.ID, d.ProjectID, d.Domain, d.Status, string(d.RawJSON), d.LastSeenAt)
	return err
}

// ---------- Sync cursors ----------

type SyncCursor struct {
	Resource     string    `json:"resource"`
	ProjectID    string    `json:"project_id"`
	LastSyncedAt time.Time `json:"last_synced_at"`
	RowCount     int       `json:"row_count"`
}

func (s *Store) BumpCursor(ctx context.Context, resource, projectID string, rowCount int) error {
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO sync_cursors (resource, project_id, last_synced_at, row_count)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(resource, project_id) DO UPDATE SET
			last_synced_at = excluded.last_synced_at,
			row_count = excluded.row_count
	`, resource, projectID, time.Now().UTC(), rowCount)
	return err
}

func (s *Store) Cursors(ctx context.Context) ([]SyncCursor, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT resource, project_id, last_synced_at, row_count FROM sync_cursors ORDER BY resource`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SyncCursor
	for rows.Next() {
		var c SyncCursor
		if err := rows.Scan(&c.Resource, &c.ProjectID, &c.LastSyncedAt, &c.RowCount); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// SyncDiff returns per-resource row counts since `since`.
type SyncDiffRow struct {
	Resource     string    `json:"resource"`
	ProjectID    string    `json:"project_id"`
	RowsTotal    int       `json:"rows_total"`
	RowsSince    int       `json:"rows_since"`
	LastSyncedAt time.Time `json:"last_synced_at"`
}

// PATCH(greptile-4): actually execute the per-resource queries that respect
// `since` instead of discarding them with `_ = q` and returning RowsSince=0.
// The previous body built the right query map then ran a hard-coded
// `SELECT ” as project_id, COUNT(*), 0 FROM <table>` for every resource,
// which made RowsSince meaningless and `--since` a no-op flag.
func (s *Store) SyncDiff(ctx context.Context, since time.Time) ([]SyncDiffRow, error) {
	type queryDef struct {
		resource string
		sql      string
		args     []any
	}
	// prompts has no per-row created/updated timestamp suitable for an
	// agent-meaningful "rows since X" answer; the existing schema records
	// only last_seen_at (set on every sync). Surface 0 explicitly so callers
	// don't reason from a misleading number — the `LastSyncedAt` cursor
	// column already conveys when prompts were last refreshed.
	defs := []queryDef{
		{"projects",
			"SELECT '' AS project_id, COUNT(*), (SELECT COUNT(*) FROM projects WHERE last_synced_at >= ?) FROM projects",
			[]any{since}},
		{"prompts",
			"SELECT COALESCE(project_id,'') AS project_id, COUNT(*), 0 FROM prompts GROUP BY project_id",
			nil},
		{"visibility_snapshots",
			"SELECT project_id, COUNT(*), (SELECT COUNT(*) FROM visibility_snapshots v WHERE v.project_id = visibility_snapshots.project_id AND v.created_at >= ?) FROM visibility_snapshots GROUP BY project_id",
			[]any{since}},
		{"citation_snapshots",
			"SELECT project_id, COUNT(*), (SELECT COUNT(*) FROM citation_snapshots c WHERE c.project_id = citation_snapshots.project_id AND c.created_at >= ?) FROM citation_snapshots GROUP BY project_id",
			[]any{since}},
		{"documents",
			"SELECT project_id, COUNT(*), (SELECT COUNT(*) FROM documents d WHERE d.project_id = documents.project_id AND d.last_seen_at >= ?) FROM documents GROUP BY project_id",
			[]any{since}},
	}
	cursors, err := s.Cursors(ctx)
	if err != nil {
		return nil, err
	}
	cursorMap := make(map[string]time.Time)
	for _, c := range cursors {
		cursorMap[c.Resource+"|"+c.ProjectID] = c.LastSyncedAt
	}
	var out []SyncDiffRow
	for _, def := range defs {
		rows, err := s.DB.QueryContext(ctx, def.sql, def.args...)
		if err != nil {
			return nil, fmt.Errorf("sync-diff %s: %w", def.resource, err)
		}
		for rows.Next() {
			var d SyncDiffRow
			d.Resource = def.resource
			if err := rows.Scan(&d.ProjectID, &d.RowsTotal, &d.RowsSince); err != nil {
				rows.Close()
				return nil, err
			}
			if ts, ok := cursorMap[def.resource+"|"+d.ProjectID]; ok {
				d.LastSyncedAt = ts
			}
			out = append(out, d)
		}
		rows.Close()
	}
	return out, nil
}

// ---------- Full-text search ----------

type FTSDoc struct {
	Kind      string
	ID        string
	ProjectID string
	Title     string
	Body      string
}

// UpsertFTS removes any existing row for (kind,id) before inserting. FTS5 does
// not natively support UPSERT, so we delete-then-insert. Safe under the
// single-writer constraint (SetMaxOpenConns(1)).
func (s *Store) UpsertFTS(ctx context.Context, d FTSDoc) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	if _, err := tx.ExecContext(ctx, `DELETE FROM cache_fts WHERE kind = ? AND id = ?`, d.Kind, d.ID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO cache_fts(kind, id, project_id, title, body) VALUES (?, ?, ?, ?, ?)`,
		d.Kind, d.ID, d.ProjectID, d.Title, d.Body); err != nil {
		return err
	}
	return tx.Commit()
}

type FTSHit struct {
	Kind      string  `json:"kind"`
	ID        string  `json:"id"`
	ProjectID string  `json:"project_id"`
	Title     string  `json:"title"`
	Snippet   string  `json:"snippet"`
	Rank      float64 `json:"rank"`
}

// Search runs an FTS5 MATCH against cache_fts. kinds, when non-empty, filters
// kind values. limit defaults to 25.
func (s *Store) Search(ctx context.Context, query string, kinds []string, limit int) ([]FTSHit, error) {
	if limit <= 0 {
		limit = 25
	}
	args := []any{query}
	kindFilter := ""
	if len(kinds) > 0 {
		kindFilter = "AND kind IN ("
		for i, k := range kinds {
			if i > 0 {
				kindFilter += ","
			}
			kindFilter += "?"
			args = append(args, k)
		}
		kindFilter += ")"
	}
	args = append(args, limit)
	q := fmt.Sprintf(`
		SELECT kind, id, project_id, title, snippet(cache_fts, 4, '[', ']', '…', 16), rank
		FROM cache_fts
		WHERE cache_fts MATCH ? %s
		ORDER BY rank
		LIMIT ?
	`, kindFilter)
	rows, err := s.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []FTSHit
	for rows.Next() {
		var h FTSHit
		if err := rows.Scan(&h.Kind, &h.ID, &h.ProjectID, &h.Title, &h.Snippet, &h.Rank); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}
