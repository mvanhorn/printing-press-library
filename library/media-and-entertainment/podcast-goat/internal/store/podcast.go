// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
// PATCH: v0.1 episodes / segments / feeds / spend_log + FTS5.

package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcast-goat/internal/transcript"
)

// PodcastStore provides typed access to the v0.1 transcript tables. It is a
// thin wrapper over the generated *Store so generated code stays untouched.
type PodcastStore struct {
	S *Store
}

// NewPodcastStore returns a PodcastStore and ensures the tables exist.
func NewPodcastStore(ctx context.Context, s *Store) (*PodcastStore, error) {
	ps := &PodcastStore{S: s}
	if err := ps.migrate(ctx); err != nil {
		return nil, err
	}
	return ps, nil
}

func (p *PodcastStore) migrate(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS episodes (
			id              TEXT PRIMARY KEY,
			source          TEXT NOT NULL,
			show            TEXT,
			tier            TEXT NOT NULL,
			url             TEXT NOT NULL,
			title           TEXT,
			host            TEXT,
			guests_json     TEXT,
			published_at    TEXT,
			duration_sec    INTEGER,
			provider        TEXT,
			cost_credits    REAL,
			fetched_at      DATETIME NOT NULL,
			content_md      TEXT NOT NULL,
			content_jsonl   TEXT NOT NULL,
			sections_json   TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS episodes_show_idx ON episodes(show)`,
		`CREATE INDEX IF NOT EXISTS episodes_source_idx ON episodes(source)`,

		`CREATE TABLE IF NOT EXISTS episode_segments (
			episode_id TEXT NOT NULL,
			seq        INTEGER NOT NULL,
			ts_sec     INTEGER NOT NULL,
			speaker    TEXT,
			text       TEXT,
			PRIMARY KEY (episode_id, seq),
			FOREIGN KEY (episode_id) REFERENCES episodes(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS segments_speaker_idx ON episode_segments(speaker)`,

		`CREATE VIRTUAL TABLE IF NOT EXISTS episodes_fts USING fts5(
			episode_id UNINDEXED,
			show,
			title,
			guests,
			content_md,
			tokenize = 'porter unicode61'
		)`,

		`CREATE TABLE IF NOT EXISTS feeds (
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			rss_url      TEXT NOT NULL UNIQUE,
			source       TEXT NOT NULL DEFAULT 'rss',
			show_title   TEXT,
			added_at     DATETIME NOT NULL,
			last_sync_at DATETIME
		)`,

		`CREATE TABLE IF NOT EXISTS spend_log (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			ts            DATETIME NOT NULL,
			provider      TEXT NOT NULL,
			episode_url   TEXT,
			episode_id    TEXT,
			cost_credits  REAL,
			cost_usd      REAL
		)`,
		`CREATE INDEX IF NOT EXISTS spend_provider_idx ON spend_log(provider)`,
	}
	p.S.writeMu.Lock()
	defer p.S.writeMu.Unlock()
	tx, err := p.S.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	for _, q := range stmts {
		if _, err := tx.ExecContext(ctx, q); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("podcast migrate: %w (%s)", err, q[:min(len(q), 80)])
		}
	}
	return tx.Commit()
}

// UpsertTranscript writes a transcript and rebuilds its FTS row + segment rows.
func (p *PodcastStore) UpsertTranscript(ctx context.Context, t *transcript.Transcript) error {
	if t == nil {
		return fmt.Errorf("nil transcript")
	}
	if t.ID == "" {
		t.ID = transcript.IDFor(t.URL)
	}
	guestsJSON, _ := json.Marshal(t.Guests)
	sectionsJSON, _ := json.Marshal(t.SectionTimestamps)
	md := t.CanonicalMarkdown()
	jsonl := t.JSONL()

	p.S.writeMu.Lock()
	defer p.S.writeMu.Unlock()
	tx, err := p.S.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO episodes (
			id, source, show, tier, url, title, host, guests_json,
			published_at, duration_sec, provider, cost_credits, fetched_at,
			content_md, content_jsonl, sections_json
		) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET
			source=excluded.source,
			show=excluded.show,
			tier=excluded.tier,
			url=excluded.url,
			title=excluded.title,
			host=excluded.host,
			guests_json=excluded.guests_json,
			published_at=excluded.published_at,
			duration_sec=excluded.duration_sec,
			provider=excluded.provider,
			cost_credits=excluded.cost_credits,
			fetched_at=excluded.fetched_at,
			content_md=excluded.content_md,
			content_jsonl=excluded.content_jsonl,
			sections_json=excluded.sections_json
	`,
		t.ID, t.Source, t.Show, string(t.Tier), t.URL, t.Title, t.Host, string(guestsJSON),
		t.Published, t.DurationSec, t.Provider, t.CostCredits, t.FetchedAt,
		md, jsonl, string(sectionsJSON),
	)
	if err != nil {
		return fmt.Errorf("episodes upsert: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM episode_segments WHERE episode_id = ?`, t.ID); err != nil {
		return fmt.Errorf("segments delete: %w", err)
	}
	for i, seg := range t.Segments {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO episode_segments (episode_id, seq, ts_sec, speaker, text) VALUES (?, ?, ?, ?, ?)`,
			t.ID, i, seg.TsSec, seg.Speaker, seg.Text,
		); err != nil {
			return fmt.Errorf("segments insert: %w", err)
		}
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM episodes_fts WHERE episode_id = ?`, t.ID); err != nil {
		return fmt.Errorf("fts delete: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO episodes_fts (episode_id, show, title, guests, content_md) VALUES (?, ?, ?, ?, ?)`,
		t.ID, t.Show, t.Title, strings.Join(t.Guests, ", "), md,
	); err != nil {
		return fmt.Errorf("fts insert: %w", err)
	}
	return tx.Commit()
}

// EpisodeRow is the typed view of one episodes row. JSON tags use snake_case
// so agent consumers of `cache list --json` see a consistent field convention
// across the CLI.
type EpisodeRow struct {
	ID          string    `json:"id"`
	Source      string    `json:"source"`
	Show        string    `json:"show"`
	Tier        string    `json:"tier"`
	URL         string    `json:"url"`
	Title       string    `json:"title"`
	Host        string    `json:"host"`
	Guests      []string  `json:"guests"`
	PublishedAt string    `json:"published_at"`
	DurationSec int       `json:"duration_sec"`
	Provider    string    `json:"provider"`
	CostCredits float64   `json:"cost_credits"`
	FetchedAt   time.Time `json:"fetched_at"`
	ContentMD   string    `json:"content_md,omitempty"`
}

// GetTranscript loads an episode by URL hash.
func (p *PodcastStore) GetTranscript(ctx context.Context, url string) (*EpisodeRow, error) {
	id := transcript.IDFor(url)
	return p.getByID(ctx, id)
}

func (p *PodcastStore) getByID(ctx context.Context, id string) (*EpisodeRow, error) {
	row := p.S.db.QueryRowContext(ctx, `
		SELECT id, source, COALESCE(show,''), tier, url, COALESCE(title,''), COALESCE(host,''),
		       COALESCE(guests_json,'[]'), COALESCE(published_at,''),
		       COALESCE(duration_sec,0), COALESCE(provider,''), COALESCE(cost_credits,0),
		       fetched_at, COALESCE(content_md,'')
		FROM episodes WHERE id = ?`, id)
	var r EpisodeRow
	var guestsJSON string
	var fetchedAtStr string
	err := row.Scan(&r.ID, &r.Source, &r.Show, &r.Tier, &r.URL, &r.Title, &r.Host,
		&guestsJSON, &r.PublishedAt, &r.DurationSec, &r.Provider, &r.CostCredits,
		&fetchedAtStr, &r.ContentMD)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(guestsJSON), &r.Guests)
	if t, perr := time.Parse(time.RFC3339Nano, fetchedAtStr); perr == nil {
		r.FetchedAt = t
	}
	return &r, nil
}

// ListEpisodes returns recent episodes (newest fetched first).
func (p *PodcastStore) ListEpisodes(ctx context.Context, limit int, sourceFilter string) ([]EpisodeRow, error) {
	if limit <= 0 {
		limit = 100
	}
	q := `SELECT id, source, COALESCE(show,''), tier, url, COALESCE(title,''),
	             COALESCE(host,''), COALESCE(guests_json,'[]'), COALESCE(published_at,''),
	             COALESCE(duration_sec,0), COALESCE(provider,''), COALESCE(cost_credits,0),
	             fetched_at, ''
	      FROM episodes`
	args := []interface{}{}
	if sourceFilter != "" {
		q += ` WHERE source = ?`
		args = append(args, sourceFilter)
	}
	q += ` ORDER BY fetched_at DESC LIMIT ?`
	args = append(args, limit)

	rows, err := p.S.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []EpisodeRow
	for rows.Next() {
		var r EpisodeRow
		var guestsJSON, fetchedAtStr, ignored string
		if err := rows.Scan(&r.ID, &r.Source, &r.Show, &r.Tier, &r.URL, &r.Title,
			&r.Host, &guestsJSON, &r.PublishedAt, &r.DurationSec, &r.Provider,
			&r.CostCredits, &fetchedAtStr, &ignored); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(guestsJSON), &r.Guests)
		if t, perr := time.Parse(time.RFC3339Nano, fetchedAtStr); perr == nil {
			r.FetchedAt = t
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// SearchEpisodes runs an FTS5 BM25 query and returns episode hits ranked by relevance.
func (p *PodcastStore) SearchEpisodes(ctx context.Context, query string, limit int) ([]EpisodeRow, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := p.S.db.QueryContext(ctx, `
		SELECT e.id, e.source, COALESCE(e.show,''), e.tier, e.url, COALESCE(e.title,''),
		       COALESCE(e.host,''), COALESCE(e.guests_json,'[]'), COALESCE(e.published_at,''),
		       COALESCE(e.duration_sec,0), COALESCE(e.provider,''), COALESCE(e.cost_credits,0),
		       e.fetched_at
		FROM episodes_fts f
		JOIN episodes e ON e.id = f.episode_id
		WHERE episodes_fts MATCH ?
		ORDER BY bm25(episodes_fts) ASC
		LIMIT ?`, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []EpisodeRow
	for rows.Next() {
		var r EpisodeRow
		var guestsJSON, fetchedAtStr string
		if err := rows.Scan(&r.ID, &r.Source, &r.Show, &r.Tier, &r.URL, &r.Title,
			&r.Host, &guestsJSON, &r.PublishedAt, &r.DurationSec, &r.Provider,
			&r.CostCredits, &fetchedAtStr); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(guestsJSON), &r.Guests)
		if t, perr := time.Parse(time.RFC3339Nano, fetchedAtStr); perr == nil {
			r.FetchedAt = t
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// SegmentHit is one segment from a quote search. JSON tags use snake_case
// for agent-consumer consistency.
type SegmentHit struct {
	EpisodeID    string `json:"episode_id"`
	EpisodeURL   string `json:"episode_url"`
	EpisodeTitle string `json:"episode_title"`
	Show         string `json:"show"`
	Seq          int    `json:"seq"`
	TsSec        int    `json:"ts_sec"`
	Speaker      string `json:"speaker"`
	Text         string `json:"text"`
}

// SearchSegments returns segments whose text matches the phrase (LIKE %phrase%),
// expanded with N segments above and below from the same episode.
func (p *PodcastStore) SearchSegments(ctx context.Context, phrase string, contextN, limit int) (map[string][]SegmentHit, error) {
	if limit <= 0 {
		limit = 25
	}
	if contextN < 0 {
		contextN = 0
	}
	rows, err := p.S.db.QueryContext(ctx, `
		SELECT s.episode_id, e.url, COALESCE(e.title,''), COALESCE(e.show,''),
		       s.seq, s.ts_sec, COALESCE(s.speaker,''), s.text
		FROM episode_segments s
		JOIN episodes e ON e.id = s.episode_id
		WHERE s.text LIKE ?
		ORDER BY s.episode_id, s.seq
		LIMIT ?`, "%"+phrase+"%", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type seedKey struct {
		EpisodeID string
		Seq       int
	}
	type seedInfo struct {
		URL, Title, Show string
	}
	seeds := []seedKey{}
	seedMeta := map[string]seedInfo{}
	for rows.Next() {
		var k seedKey
		var info seedInfo
		var tsSec int
		var speaker, text string
		if err := rows.Scan(&k.EpisodeID, &info.URL, &info.Title, &info.Show, &k.Seq, &tsSec, &speaker, &text); err != nil {
			return nil, err
		}
		seeds = append(seeds, k)
		if _, ok := seedMeta[k.EpisodeID]; !ok {
			seedMeta[k.EpisodeID] = info
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := map[string][]SegmentHit{}
	for _, seed := range seeds {
		ctxRows, err := p.S.db.QueryContext(ctx, `
			SELECT seq, ts_sec, COALESCE(speaker,''), text
			FROM episode_segments
			WHERE episode_id = ? AND seq BETWEEN ? AND ?
			ORDER BY seq`, seed.EpisodeID, seed.Seq-contextN, seed.Seq+contextN)
		if err != nil {
			return nil, err
		}
		meta := seedMeta[seed.EpisodeID]
		for ctxRows.Next() {
			var seq, tsSec int
			var speaker, text string
			if err := ctxRows.Scan(&seq, &tsSec, &speaker, &text); err != nil {
				ctxRows.Close()
				return nil, err
			}
			out[meta.URL] = append(out[meta.URL], SegmentHit{
				EpisodeID:    seed.EpisodeID,
				EpisodeURL:   meta.URL,
				EpisodeTitle: meta.Title,
				Show:         meta.Show,
				Seq:          seq,
				TsSec:        tsSec,
				Speaker:      speaker,
				Text:         text,
			})
		}
		ctxRows.Close()
		// Insert a sentinel gap-separator between blocks within the same episode.
	}
	return out, nil
}

// SpeakerAggregate is one row of the speakers-list output.
type SpeakerAggregate struct {
	Speaker      string   `json:"speaker"`
	EpisodeCount int      `json:"episode_count"`
	SegmentCount int      `json:"segment_count"`
	Shows        []string `json:"shows"`
}

// ListSpeakers aggregates segment counts by speaker.
func (p *PodcastStore) ListSpeakers(ctx context.Context, showFilter string, minSegments int) ([]SpeakerAggregate, error) {
	q := `SELECT COALESCE(s.speaker,''),
	             COUNT(DISTINCT s.episode_id),
	             COUNT(*) ,
	             GROUP_CONCAT(DISTINCT COALESCE(e.show,''))
	      FROM episode_segments s
	      JOIN episodes e ON e.id = s.episode_id`
	args := []interface{}{}
	if showFilter != "" {
		q += ` WHERE e.show = ?`
		args = append(args, showFilter)
	}
	q += ` GROUP BY s.speaker HAVING COUNT(*) >= ? ORDER BY COUNT(*) DESC`
	args = append(args, max(minSegments, 1))
	rows, err := p.S.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SpeakerAggregate
	for rows.Next() {
		var ag SpeakerAggregate
		var shows string
		if err := rows.Scan(&ag.Speaker, &ag.EpisodeCount, &ag.SegmentCount, &shows); err != nil {
			return nil, err
		}
		if shows != "" {
			ag.Shows = strings.Split(shows, ",")
		}
		if ag.Speaker == "" {
			continue
		}
		out = append(out, ag)
	}
	return out, rows.Err()
}

// RecordSpend logs a paid-tier hit for budget reporting.
func (p *PodcastStore) RecordSpend(ctx context.Context, provider, episodeURL string, credits, usd float64) error {
	p.S.writeMu.Lock()
	defer p.S.writeMu.Unlock()
	_, err := p.S.db.ExecContext(ctx,
		`INSERT INTO spend_log (ts, provider, episode_url, episode_id, cost_credits, cost_usd) VALUES (?,?,?,?,?,?)`,
		time.Now().UTC(), provider, episodeURL, transcript.IDFor(episodeURL), credits, usd,
	)
	return err
}

// ResetSpend clears spend_log.
func (p *PodcastStore) ResetSpend(ctx context.Context) error {
	p.S.writeMu.Lock()
	defer p.S.writeMu.Unlock()
	_, err := p.S.db.ExecContext(ctx, `DELETE FROM spend_log`)
	return err
}

// BudgetRow is one pivot row. JSON tags use snake_case for consistency.
type BudgetRow struct {
	Show         string  `json:"show"`
	Provider     string  `json:"provider"`
	Month        string  `json:"month"`
	Episodes     int     `json:"episodes"`
	TotalCredits float64 `json:"total_credits"`
	TotalUSD     float64 `json:"total_usd"`
}

// BudgetByShow aggregates spend_log joined to episodes by show+provider+month.
//
// The `month` derivation uses SUBSTR rather than strftime because Go's
// database/sql writes time.Time values as `"2026-05-17 21:38:54.959214 +0000 UTC"`,
// which SQLite's strftime can't parse (it expects ISO-8601 without the
// " +0000 UTC" suffix) and silently returns NULL. SUBSTR over the first 7
// characters yields "2026-05" reliably regardless of how the timestamp was
// stringified. COALESCE on month adds defense for any rows where ts itself
// is NULL.
func (p *PodcastStore) BudgetByShow(ctx context.Context, sinceDays int) ([]BudgetRow, error) {
	if sinceDays <= 0 {
		sinceDays = 90
	}
	rows, err := p.S.db.QueryContext(ctx, `
		SELECT COALESCE(e.show,'(unknown)') AS show,
		       sp.provider,
		       COALESCE(SUBSTR(sp.ts, 1, 7), '(unknown)') AS month,
		       COUNT(DISTINCT sp.episode_id) AS eps,
		       SUM(COALESCE(sp.cost_credits,0)),
		       SUM(COALESCE(sp.cost_usd,0))
		FROM spend_log sp
		LEFT JOIN episodes e ON e.id = sp.episode_id
		WHERE sp.ts >= datetime('now', ?)
		GROUP BY e.show, sp.provider, month
		ORDER BY month DESC, sp.provider, show`, fmt.Sprintf("-%d days", sinceDays))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BudgetRow
	for rows.Next() {
		var r BudgetRow
		if err := rows.Scan(&r.Show, &r.Provider, &r.Month, &r.Episodes, &r.TotalCredits, &r.TotalUSD); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// FeedRow is one feeds-table row.
type FeedRow struct {
	ID         int64
	URL        string
	Source     string
	ShowTitle  string
	AddedAt    time.Time
	LastSyncAt sql.NullTime
}

// AddFeed inserts a feed.
func (p *PodcastStore) AddFeed(ctx context.Context, rssURL, source, showTitle string) (int64, error) {
	p.S.writeMu.Lock()
	defer p.S.writeMu.Unlock()
	res, err := p.S.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO feeds (rss_url, source, show_title, added_at) VALUES (?, ?, ?, ?)`,
		rssURL, source, showTitle, time.Now().UTC(),
	)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	return id, nil
}

// ListFeeds returns all subscribed feeds.
func (p *PodcastStore) ListFeeds(ctx context.Context) ([]FeedRow, error) {
	rows, err := p.S.db.QueryContext(ctx,
		`SELECT id, rss_url, source, COALESCE(show_title,''), added_at, last_sync_at FROM feeds ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []FeedRow
	for rows.Next() {
		var r FeedRow
		var added string
		var last sql.NullString
		if err := rows.Scan(&r.ID, &r.URL, &r.Source, &r.ShowTitle, &added, &last); err != nil {
			return nil, err
		}
		if t, err := time.Parse(time.RFC3339Nano, added); err == nil {
			r.AddedAt = t
		}
		if last.Valid {
			if t, err := time.Parse(time.RFC3339Nano, last.String); err == nil {
				r.LastSyncAt = sql.NullTime{Time: t, Valid: true}
			}
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// MarkFeedSynced bumps last_sync_at.
func (p *PodcastStore) MarkFeedSynced(ctx context.Context, id int64) error {
	p.S.writeMu.Lock()
	defer p.S.writeMu.Unlock()
	_, err := p.S.db.ExecContext(ctx,
		`UPDATE feeds SET last_sync_at = ? WHERE id = ?`,
		time.Now().UTC(), id,
	)
	return err
}

// ClearBySource removes cached episodes for one source AND the spend_log rows
// that point to episodes from that source. Without the spend cleanup, budget
// pivots show orphan "(unknown)" rows for the cleared episodes — the spend
// entry survives without a matching episode to attribute show/title from.
func (p *PodcastStore) ClearBySource(ctx context.Context, src string) (int64, error) {
	p.S.writeMu.Lock()
	defer p.S.writeMu.Unlock()
	tx, err := p.S.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM episode_segments WHERE episode_id IN (SELECT id FROM episodes WHERE source = ?)`, src); err != nil {
		return 0, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM episodes_fts WHERE episode_id IN (SELECT id FROM episodes WHERE source = ?)`, src); err != nil {
		return 0, err
	}
	// Spend log entries are keyed by episode_id, which is the same SHA-256 of
	// the URL the episode row used. Sweeping by episode_id keeps spend cleanup
	// scoped to the source being cleared without needing a source column on
	// spend_log.
	if _, err := tx.ExecContext(ctx, `DELETE FROM spend_log WHERE episode_id IN (SELECT id FROM episodes WHERE source = ?)`, src); err != nil {
		return 0, err
	}
	res, err := tx.ExecContext(ctx, `DELETE FROM episodes WHERE source = ?`, src)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	// Additional orphan sweep: prior cache-clear invocations (before this
	// sweep existed) left spend_log rows whose episode_id no longer maps to
	// any episode. They surface in `budget show` as "(unknown)" pivot rows.
	// Sweeping them here keeps the budget view clean without a separate
	// vacuum command.
	if _, err := tx.ExecContext(ctx, `DELETE FROM spend_log WHERE episode_id NOT IN (SELECT id FROM episodes)`); err != nil {
		return 0, err
	}
	return n, tx.Commit()
}

// min/max provided by Go 1.21+ builtins.
