// Copyright 2026 usc-tk. Licensed under Apache-2.0. See LICENSE.

// Schema v2 additions for the novel "transcendence" commands. These tables
// are created on top of the generated schema (resources, sync_state,
// resources_fts) and are owned by hand-written commands:
//
//   - vertical_match     // score a project against a named vertical
//   - project_snapshots  // periodic snapshots for funding-velocity rollup
//   - sync_runs          // last-successful-sync watermarks for sync --diff
//   - magazine_articles  // mirrored Kickstarter Magazine articles
//   - magazine_fts       // FTS5 mirror of magazine_articles for search
//
// All tables are additive — they do not modify any generated table — so the
// generated migration path still owns user_version. Idempotent (CREATE
// TABLE IF NOT EXISTS) and safe to call on every Open().

package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

// EnsureV2Schema is preserved as a no-op stub for backward compatibility
// with callers in internal/cli that invoke it on every command entry. The
// v2 DDL it previously executed has been folded into the main migrations
// slice in store.go (alongside the v1 tables) so PRAGMA user_version
// stamps to StoreSchemaVersion = 2 atomically inside the BEGIN IMMEDIATE
// migration transaction. The schema-version gate at the top of migrate()
// then correctly refuses older binaries (v1) from opening a v2 database,
// closing the bypass flagged in security review.
//
// New code should rely on store.OpenWithContext (which runs migrate()) to
// have all v1+v2 tables ready. This stub remains so existing callers
// compile without changes; it intentionally does nothing.
func (s *Store) EnsureV2Schema(ctx context.Context) error {
	_ = ctx
	return nil
}

// RecordSyncRun appends a sync_runs row recording the completion of a sync
// pass for the given resource. records is the number of records the run
// observed; finished is the wall-clock time of completion.
func (s *Store) RecordSyncRun(resource string, records int, finished time.Time) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.Exec(
		`INSERT INTO sync_runs (resource, finished_at, records_synced) VALUES (?, ?, ?)`,
		resource, finished.Unix(), records,
	)
	return err
}

// LastSyncRun returns the most recent finished_at for a resource, or zero
// time if no successful sync has been recorded. resource may be the empty
// string to ask "any resource".
func (s *Store) LastSyncRun(resource string) (time.Time, error) {
	var ts sql.NullInt64
	var err error
	if resource == "" {
		err = s.db.QueryRow(
			`SELECT MAX(finished_at) FROM sync_runs`,
		).Scan(&ts)
	} else {
		err = s.db.QueryRow(
			`SELECT MAX(finished_at) FROM sync_runs WHERE resource = ?`, resource,
		).Scan(&ts)
	}
	if err != nil {
		return time.Time{}, err
	}
	if !ts.Valid {
		return time.Time{}, nil
	}
	return time.Unix(ts.Int64, 0), nil
}

// UpsertProjectSnapshot records a point-in-time (pledged, backers, state)
// observation for a project. The (project_id, snapshot_at) primary key
// keeps repeat calls within the same second idempotent.
func (s *Store) UpsertProjectSnapshot(projectID int64, snapshotAt time.Time, pledged float64, backers int, state string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.Exec(
		`INSERT INTO project_snapshots (project_id, snapshot_at, pledged, backers_count, state)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(project_id, snapshot_at) DO UPDATE SET pledged=excluded.pledged,
		   backers_count=excluded.backers_count, state=excluded.state`,
		projectID, snapshotAt.Unix(), pledged, backers, state,
	)
	return err
}

// ProjectSnapshot is the read shape for project_snapshots rows.
type ProjectSnapshot struct {
	ProjectID    int64   `json:"project_id"`
	SnapshotAt   int64   `json:"snapshot_at"`
	Pledged      float64 `json:"pledged"`
	BackersCount int     `json:"backers_count"`
	State        string  `json:"state"`
}

// SnapshotsSince returns all project_snapshots rows after the given epoch
// time, ordered by (project_id ASC, snapshot_at ASC). Used by funding-rank
// to compute deltas.
func (s *Store) SnapshotsSince(since time.Time) ([]ProjectSnapshot, error) {
	rows, err := s.db.Query(
		`SELECT project_id, snapshot_at, pledged, backers_count, state
		 FROM project_snapshots
		 WHERE snapshot_at >= ?
		 ORDER BY project_id ASC, snapshot_at ASC`,
		since.Unix(),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ProjectSnapshot
	for rows.Next() {
		var s ProjectSnapshot
		if err := rows.Scan(&s.ProjectID, &s.SnapshotAt, &s.Pledged, &s.BackersCount, &s.State); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// UpsertVerticalMatch records a score for (project, vertical). Repeated
// calls overwrite the prior score (latest scoring wins).
func (s *Store) UpsertVerticalMatch(projectID int64, vertical string, score float64, scoredAt time.Time) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.Exec(
		`INSERT INTO vertical_match (project_id, vertical, score, scored_at)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(project_id, vertical) DO UPDATE SET score=excluded.score, scored_at=excluded.scored_at`,
		projectID, vertical, score, scoredAt.Unix(),
	)
	return err
}

// MagazineArticle is the read/write shape for magazine_articles rows.
type MagazineArticle struct {
	Slug        string `json:"slug"`
	Title       string `json:"title"`
	Body        string `json:"body"`
	Author      string `json:"author"`
	PublishedAt int64  `json:"published_at"`
	Tags        string `json:"tags"`
	URL         string `json:"url"`
	SyncedAt    int64  `json:"synced_at"`
}

// UpsertMagazineArticle stores or replaces a magazine article and keeps
// the FTS5 mirror in sync. The two tables are kept in parity manually
// rather than via SQL triggers because modernc.org/sqlite's FTS5
// implementation can be finicky with trigger-driven INSERT/DELETE.
func (s *Store) UpsertMagazineArticle(a MagazineArticle) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if a.SyncedAt == 0 {
		a.SyncedAt = time.Now().Unix()
	}
	if _, err := tx.Exec(
		`INSERT INTO magazine_articles (slug, title, body, author, published_at, tags, url, synced_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(slug) DO UPDATE SET title=excluded.title, body=excluded.body,
		   author=excluded.author, published_at=excluded.published_at, tags=excluded.tags,
		   url=excluded.url, synced_at=excluded.synced_at`,
		a.Slug, a.Title, a.Body, a.Author, a.PublishedAt, a.Tags, a.URL, a.SyncedAt,
	); err != nil {
		return err
	}
	// FTS parity: delete-then-insert keyed on slug as a TEXT column
	if _, err := tx.Exec(`DELETE FROM magazine_fts WHERE slug = ?`, a.Slug); err != nil {
		return err
	}
	if _, err := tx.Exec(
		`INSERT INTO magazine_fts (slug, title, body, tags) VALUES (?, ?, ?, ?)`,
		a.Slug, a.Title, a.Body, a.Tags,
	); err != nil {
		return err
	}
	return tx.Commit()
}

// MagazineSearchResult is one hit from a magazine_fts MATCH query.
type MagazineSearchResult struct {
	Slug        string `json:"slug"`
	Title       string `json:"title"`
	Snippet     string `json:"snippet"`
	PublishedAt int64  `json:"published_at"`
	URL         string `json:"url"`
}

// MagazineSearch runs an FTS5 MATCH query against the magazine_fts table
// and joins back to magazine_articles for the snippet body. Returns hits
// ordered by FTS5 rank.
//
// Returns (nil, nil) when no articles have been synced yet — callers
// should surface this as "run sync first" rather than as an error.
func (s *Store) MagazineSearch(query string, limit int) ([]MagazineSearchResult, error) {
	if limit <= 0 {
		limit = 10
	}
	// Pre-flight: is the articles table empty?
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM magazine_articles`).Scan(&n); err == nil && n == 0 {
		return nil, nil
	}
	rows, err := s.db.Query(
		`SELECT m.slug, m.title, m.body, m.published_at, m.url
		 FROM magazine_fts f
		 JOIN magazine_articles m ON m.slug = f.slug
		 WHERE magazine_fts MATCH ?
		 ORDER BY rank
		 LIMIT ?`,
		query, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MagazineSearchResult
	for rows.Next() {
		var r MagazineSearchResult
		var body string
		if err := rows.Scan(&r.Slug, &r.Title, &body, &r.PublishedAt, &r.URL); err != nil {
			return nil, err
		}
		r.Snippet = snippetOf(body, 240)
		out = append(out, r)
	}
	return out, rows.Err()
}

// snippetOf trims body to maxLen chars, breaking on whitespace where
// possible. Used so MagazineSearchResult.Snippet stays tractable.
func snippetOf(body string, maxLen int) string {
	if len(body) <= maxLen {
		return body
	}
	cut := maxLen
	for i := maxLen - 1; i > maxLen-40 && i > 0; i-- {
		if body[i] == ' ' || body[i] == '\n' {
			cut = i
			break
		}
	}
	return body[:cut] + "..."
}

// DiscoverProject mirrors the subset of Kickstarter Discover-JSON fields
// the novel commands consume. JSON tags match the API exactly so a
// json.Unmarshal directly over the project payload populates this struct.
type DiscoverProject struct {
	ID             int64   `json:"id"`
	Name           string  `json:"name"`
	Blurb          string  `json:"blurb"`
	Slug           string  `json:"slug"`
	Goal           float64 `json:"goal"`
	Pledged        float64 `json:"pledged"`
	State          string  `json:"state"`
	LaunchedAt     int64   `json:"launched_at"`
	StateChangedAt int64   `json:"state_changed_at"`
	BackersCount   int     `json:"backers_count"`
	URLs           struct {
		Web struct {
			Project string `json:"project"`
		} `json:"web"`
	} `json:"urls"`
	Category struct {
		ID            int64  `json:"id"`
		Name          string `json:"name"`
		Slug          string `json:"slug"`
		AnalyticsName string `json:"analytics_name"`
	} `json:"category"`
	Creator struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
		Slug string `json:"slug"`
	} `json:"creator"`
	Tags []string `json:"tags"`
}

// LoadDiscoverProjects returns every DiscoverProject the local store
// knows about. Reads from two complementary sources:
//
//  1. resource_type='discover_project' — individual project rows written
//     by IngestDiscoverProjects after a live fan-out or sync run; these
//     unmarshal directly as DiscoverProject.
//
//  2. resource_type='discover' — page-level rows written by the generated
//     sync code, which store the WHOLE response body as a single row.
//     Each row's payload contains a {"projects":[...]} array we expand
//     into individual projects on the fly.
//
// Per-project rows are deduplicated by ID, with the discover_project row
// (if present) winning. Both code paths are tolerant of failure: a row
// that doesn't parse simply doesn't contribute. limit, when > 0, caps
// the number of envelope-style rows we scan; per-project rows are
// always loaded in full because each row is small.
func (s *Store) LoadDiscoverProjects(limit int) ([]DiscoverProject, error) {
	if limit <= 0 {
		limit = 5000
	}
	byID := map[int64]DiscoverProject{}

	// Pass 1: individual project rows
	projRows, err := s.db.Query(
		`SELECT data FROM resources WHERE resource_type = 'discover_project'`,
	)
	if err == nil {
		for projRows.Next() {
			var raw string
			if err := projRows.Scan(&raw); err != nil {
				continue
			}
			var p DiscoverProject
			if err := json.Unmarshal([]byte(raw), &p); err != nil {
				continue
			}
			if p.ID == 0 {
				continue
			}
			byID[p.ID] = p
		}
		projRows.Close()
	}

	// Pass 2: page-level envelope rows
	rows, err := s.db.Query(
		`SELECT data FROM resources WHERE resource_type = 'discover' ORDER BY updated_at DESC LIMIT ?`,
		limit,
	)
	if err != nil {
		// Fall back to whatever pass 1 produced; an empty result from
		// pass 2 doesn't invalidate per-project rows.
		out := make([]DiscoverProject, 0, len(byID))
		for _, p := range byID {
			out = append(out, p)
		}
		return out, nil
	}
	defer rows.Close()
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			continue
		}
		// Try direct project shape first
		var single DiscoverProject
		if err := json.Unmarshal([]byte(raw), &single); err == nil && single.ID != 0 {
			if _, ok := byID[single.ID]; !ok {
				byID[single.ID] = single
			}
			continue
		}
		// Otherwise expect {"projects":[...]} envelope
		var env struct {
			Projects []DiscoverProject `json:"projects"`
		}
		if err := json.Unmarshal([]byte(raw), &env); err != nil {
			continue
		}
		for _, p := range env.Projects {
			if p.ID == 0 {
				continue
			}
			if _, ok := byID[p.ID]; !ok {
				byID[p.ID] = p
			}
		}
	}
	out := make([]DiscoverProject, 0, len(byID))
	for _, p := range byID {
		out = append(out, p)
	}
	return out, nil
}

// IngestDiscoverProjects writes each project into the generic resources
// table as resource_type='discover_project' so subsequent local reads
// see one row per project. Used by sync --diff to populate the per-
// project surface that funding-rank / vertical / category velocity
// query. Idempotent: repeated upserts replace prior values.
func (s *Store) IngestDiscoverProjects(projects []DiscoverProject) error {
	if len(projects) == 0 {
		return nil
	}
	items := make([]json.RawMessage, 0, len(projects))
	for _, p := range projects {
		b, err := json.Marshal(p)
		if err != nil {
			continue
		}
		items = append(items, json.RawMessage(b))
	}
	_, _, err := s.UpsertBatch("discover_project", items)
	return err
}

// ExpandDiscoverProjectsFromEnvelopes scans resource_type='discover'
// page-level rows, decodes each {"projects":[...]} envelope, and writes
// each contained project as a `discover_project` row. Returns the count
// of projects ingested. Safe to call after every sync — UpsertBatch
// dedupes by ID, so a project that already lives in the per-project
// table just gets its payload refreshed.
func (s *Store) ExpandDiscoverProjectsFromEnvelopes() (int, error) {
	rows, err := s.db.Query(
		`SELECT data FROM resources WHERE resource_type = 'discover'`,
	)
	if err != nil {
		return 0, err
	}
	var projects []DiscoverProject
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			continue
		}
		var env struct {
			Projects []DiscoverProject `json:"projects"`
		}
		if err := json.Unmarshal([]byte(raw), &env); err != nil {
			continue
		}
		for _, p := range env.Projects {
			if p.ID != 0 {
				projects = append(projects, p)
			}
		}
	}
	rows.Close()
	if err := s.IngestDiscoverProjects(projects); err != nil {
		return 0, err
	}
	return len(projects), nil
}
