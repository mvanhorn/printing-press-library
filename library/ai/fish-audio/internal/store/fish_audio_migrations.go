// Copyright 2026 Jon Gouveia and contributors. Licensed under Apache-2.0. See LICENSE.

// fish_audio_migrations.go owns the tables the generated schema has no reason
// to know about: the local render log and the cached public voice catalog.
// Fish Audio persists neither server-side, so this file is the only record of
// what was rendered, what it cost, and which voices exist.
//
// Both tables are created lazily by their Ensure* function rather than by
// store.go's migrate(), so a CLI that never renders never pays for them and
// the generated migration path stays untouched by hand edits.

package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// RenderRow is one row of render_log: a single TTS render, its identity hash,
// its output file, and its cost.
type RenderRow struct {
	ID               int64   `json:"id"`
	CreatedAt        string  `json:"created_at"`
	RequestHash      string  `json:"request_hash"`
	Text             string  `json:"text"`
	Model            string  `json:"model"`
	VoiceID          string  `json:"voice_id"`
	Format           string  `json:"format"`
	BytesIn          int64   `json:"bytes_in"`
	BytesOut         int64   `json:"bytes_out"`
	CostUSD          float64 `json:"cost_usd"`
	CostUSDPaidEquiv float64 `json:"cost_usd_paid_equiv"`
	FilePath         string  `json:"file_path"`
	FileSHA256       string  `json:"file_sha256"`
	Source           string  `json:"source"`
}

const createRenderLogSQL = `
CREATE TABLE IF NOT EXISTS render_log (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	created_at TEXT NOT NULL,
	request_hash TEXT NOT NULL UNIQUE,
	text TEXT,
	model TEXT,
	voice_id TEXT,
	format TEXT,
	bytes_in INTEGER,
	bytes_out INTEGER,
	cost_usd REAL,
	cost_usd_paid_equiv REAL,
	file_path TEXT,
	file_sha256 TEXT,
	source TEXT
)`

// EnsureRenderLog creates the render_log table if it does not exist. It is
// safe to call on every command, including from concurrent batch workers.
func (s *Store) EnsureRenderLog(ctx context.Context) error {
	s.lockForWrite()
	defer s.unlockAfterWrite()
	if _, err := s.db.ExecContext(ctx, createRenderLogSQL); err != nil {
		return fmt.Errorf("creating render_log: %w", err)
	}
	for _, stmt := range []string{
		`CREATE INDEX IF NOT EXISTS render_log_created_at ON render_log(created_at)`,
		`CREATE INDEX IF NOT EXISTS render_log_voice ON render_log(voice_id)`,
		`CREATE INDEX IF NOT EXISTS render_log_model ON render_log(model)`,
	} {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("indexing render_log: %w", err)
		}
	}
	return nil
}

// InsertRenderRow records a render and returns its row id. A repeat of an
// identical request updates the existing row in place, so the hash stays a
// unique identity and the log does not grow one row per retry.
func (s *Store) InsertRenderRow(ctx context.Context, row RenderRow) (int64, error) {
	if row.CreatedAt == "" {
		row.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	s.lockForWrite()
	defer s.unlockAfterWrite()
	_, err := s.db.ExecContext(ctx, `
INSERT INTO render_log (created_at, request_hash, text, model, voice_id, format, bytes_in, bytes_out, cost_usd, cost_usd_paid_equiv, file_path, file_sha256, source)
VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)
ON CONFLICT(request_hash) DO UPDATE SET
	created_at=excluded.created_at,
	bytes_out=excluded.bytes_out,
	cost_usd=excluded.cost_usd,
	cost_usd_paid_equiv=excluded.cost_usd_paid_equiv,
	file_path=excluded.file_path,
	file_sha256=excluded.file_sha256,
	source=excluded.source`,
		row.CreatedAt, row.RequestHash, row.Text, row.Model, row.VoiceID, row.Format,
		row.BytesIn, row.BytesOut, row.CostUSD, row.CostUSDPaidEquiv,
		row.FilePath, row.FileSHA256, row.Source)
	if err != nil {
		return 0, fmt.Errorf("inserting render_log row: %w", err)
	}
	var id int64
	if err := s.db.QueryRowContext(ctx, `SELECT id FROM render_log WHERE request_hash = ?`, row.RequestHash).Scan(&id); err != nil {
		return 0, fmt.Errorf("reading render_log row id: %w", err)
	}
	return id, nil
}

const renderRowColumns = `id, created_at, request_hash, text, model, voice_id, format, bytes_in, bytes_out, cost_usd, cost_usd_paid_equiv, file_path, file_sha256, source`

// scanRenderRow reads one row. Every text and numeric column is nullable in
// SQLite regardless of the Go type, so each one is scanned through a sql.Null*
// wrapper: a NULL written by an older binary must read back as a zero value,
// not fail the whole query.
func scanRenderRow(scan func(dest ...any) error) (RenderRow, error) {
	var (
		row       RenderRow
		createdAt sql.NullString
		hash      sql.NullString
		text      sql.NullString
		model     sql.NullString
		voice     sql.NullString
		format    sql.NullString
		bytesIn   sql.NullInt64
		bytesOut  sql.NullInt64
		cost      sql.NullFloat64
		paid      sql.NullFloat64
		filePath  sql.NullString
		sha       sql.NullString
		source    sql.NullString
	)
	if err := scan(&row.ID, &createdAt, &hash, &text, &model, &voice, &format, &bytesIn, &bytesOut, &cost, &paid, &filePath, &sha, &source); err != nil {
		return RenderRow{}, err
	}
	row.CreatedAt = createdAt.String
	row.RequestHash = hash.String
	row.Text = text.String
	row.Model = model.String
	row.VoiceID = voice.String
	row.Format = format.String
	row.BytesIn = bytesIn.Int64
	row.BytesOut = bytesOut.Int64
	row.CostUSD = cost.Float64
	row.CostUSDPaidEquiv = paid.Float64
	row.FilePath = filePath.String
	row.FileSHA256 = sha.String
	row.Source = source.String
	return row, nil
}

// RenderRowByHash returns the render recorded for hash, or nil when there is
// none. `tts render --skip-if-rendered` uses it to decide whether to spend a
// call.
func (s *Store) RenderRowByHash(ctx context.Context, hash string) (*RenderRow, error) {
	row, err := scanRenderRow(s.db.QueryRowContext(ctx, `SELECT `+renderRowColumns+` FROM render_log WHERE request_hash = ?`, hash).Scan)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading render_log by hash: %w", err)
	}
	return &row, nil
}

// RenderRowByID returns the render with the given log id, or nil when there is
// none.
func (s *Store) RenderRowByID(ctx context.Context, id int64) (*RenderRow, error) {
	row, err := scanRenderRow(s.db.QueryRowContext(ctx, `SELECT `+renderRowColumns+` FROM render_log WHERE id = ?`, id).Scan)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading render_log by id: %w", err)
	}
	return &row, nil
}

// RenderLogFilter narrows a render log listing.
type RenderLogFilter struct {
	Limit int
	Voice string
	Model string
	// Since is an inclusive lower bound on created_at, RFC3339. Empty means
	// no lower bound.
	Since string
}

// ListRenderRows returns render log rows newest first.
func (s *Store) ListRenderRows(ctx context.Context, filter RenderLogFilter) ([]RenderRow, error) {
	query := `SELECT ` + renderRowColumns + ` FROM render_log`
	var where []string
	var args []any
	if filter.Voice != "" {
		where = append(where, "voice_id = ?")
		args = append(args, filter.Voice)
	}
	if filter.Model != "" {
		where = append(where, "model = ?")
		args = append(args, filter.Model)
	}
	if filter.Since != "" {
		where = append(where, "created_at >= ?")
		args = append(args, filter.Since)
	}
	if len(where) > 0 {
		// #nosec G202 -- every fragment in `where` is a literal declared above and
		// every value binds through a ? placeholder in `args`.
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY id DESC"
	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing render_log: %w", err)
	}
	defer rows.Close()
	out := make([]RenderRow, 0)
	for rows.Next() {
		row, scanErr := scanRenderRow(rows.Scan)
		if scanErr != nil {
			return nil, fmt.Errorf("scanning render_log row: %w", scanErr)
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// SpendRow is one bucket of the grouped spend report.
type SpendRow struct {
	Group            string  `json:"group"`
	Renders          int64   `json:"renders"`
	BytesIn          int64   `json:"bytes_in"`
	BytesOut         int64   `json:"bytes_out"`
	CostUSD          float64 `json:"cost_usd"`
	CostUSDPaidEquiv float64 `json:"cost_usd_paid_equiv"`
}

// renderSpendGroupColumns maps a --group-by value to its SQL expression. The
// map is the allow-list: the value never reaches SQL as caller-supplied text.
var renderSpendGroupColumns = map[string]string{
	"voice": "voice_id",
	"model": "model",
	"day":   "substr(created_at, 1, 10)",
}

// RenderSpendGroups lists the accepted --group-by values.
func RenderSpendGroups() []string { return []string{"voice", "model", "day"} }

// RenderSpend aggregates the render log into per-group totals, highest spend
// first.
func (s *Store) RenderSpend(ctx context.Context, groupBy, since string) ([]SpendRow, error) {
	column, ok := renderSpendGroupColumns[groupBy]
	if !ok {
		return nil, fmt.Errorf("invalid group %q: must be one of %s", groupBy, strings.Join(RenderSpendGroups(), ", "))
	}
	query := `SELECT ` + column + ` AS grp, COUNT(*), COALESCE(SUM(bytes_in),0), COALESCE(SUM(bytes_out),0), COALESCE(SUM(cost_usd),0), COALESCE(SUM(cost_usd_paid_equiv),0) FROM render_log`
	var args []any
	if since != "" {
		query += " WHERE created_at >= ?"
		args = append(args, since)
	}
	query += " GROUP BY grp ORDER BY SUM(cost_usd) DESC, grp ASC"
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("aggregating render_log: %w", err)
	}
	defer rows.Close()
	out := make([]SpendRow, 0)
	for rows.Next() {
		var (
			grp  sql.NullString
			row  SpendRow
			cost sql.NullFloat64
			paid sql.NullFloat64
		)
		if err := rows.Scan(&grp, &row.Renders, &row.BytesIn, &row.BytesOut, &cost, &paid); err != nil {
			return nil, fmt.Errorf("scanning spend row: %w", err)
		}
		row.Group = grp.String
		row.CostUSD = cost.Float64
		row.CostUSDPaidEquiv = paid.Float64
		out = append(out, row)
	}
	return out, rows.Err()
}

// VoiceCatalogRow is one cached entry of the public voice catalog.
type VoiceCatalogRow struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Tags        string `json:"tags"`
	Author      string `json:"author"`
	Languages   string `json:"languages"`
	Visibility  string `json:"visibility"`
	Source      string `json:"source"`
	SyncedAt    string `json:"synced_at"`
}

const createVoiceCatalogSQL = `
CREATE TABLE IF NOT EXISTS voice_catalog (
	id TEXT PRIMARY KEY,
	title TEXT,
	description TEXT,
	tags TEXT,
	author TEXT,
	languages TEXT,
	visibility TEXT,
	source TEXT,
	synced_at TEXT
)`

// EnsureVoiceCatalog creates the cached voice catalog and its full-text index.
// Fish Audio has no server-side voice search, so `voice discover` syncs pages
// of GET /model into this table and searches locally.
func (s *Store) EnsureVoiceCatalog(ctx context.Context) error {
	s.lockForWrite()
	defer s.unlockAfterWrite()
	if _, err := s.db.ExecContext(ctx, createVoiceCatalogSQL); err != nil {
		return fmt.Errorf("creating voice_catalog: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `CREATE VIRTUAL TABLE IF NOT EXISTS voice_catalog_fts USING fts5(id UNINDEXED, title, description, tags)`); err != nil {
		return fmt.Errorf("creating voice_catalog_fts: %w", err)
	}
	return nil
}

// UpsertVoiceCatalog writes catalog rows and refreshes their FTS entries.
func (s *Store) UpsertVoiceCatalog(ctx context.Context, rows []VoiceCatalogRow) (int, error) {
	if len(rows) == 0 {
		return 0, nil
	}
	s.lockForWrite()
	defer s.unlockAfterWrite()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("beginning voice_catalog write: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	written := 0
	for _, row := range rows {
		if row.ID == "" {
			continue
		}
		if row.SyncedAt == "" {
			row.SyncedAt = time.Now().UTC().Format(time.RFC3339)
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO voice_catalog (id, title, description, tags, author, languages, visibility, source, synced_at)
VALUES (?,?,?,?,?,?,?,?,?)
ON CONFLICT(id) DO UPDATE SET
	title=excluded.title, description=excluded.description, tags=excluded.tags,
	author=excluded.author, languages=excluded.languages, visibility=excluded.visibility,
	source=excluded.source, synced_at=excluded.synced_at`,
			row.ID, row.Title, row.Description, row.Tags, row.Author, row.Languages, row.Visibility, row.Source, row.SyncedAt); err != nil {
			return 0, fmt.Errorf("upserting voice_catalog row %q: %w", row.ID, err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM voice_catalog_fts WHERE id = ?`, row.ID); err != nil {
			return 0, fmt.Errorf("clearing voice_catalog_fts row %q: %w", row.ID, err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO voice_catalog_fts (id, title, description, tags) VALUES (?,?,?,?)`,
			row.ID, row.Title, row.Description, row.Tags); err != nil {
			return 0, fmt.Errorf("indexing voice_catalog row %q: %w", row.ID, err)
		}
		written++
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("committing voice_catalog write: %w", err)
	}
	return written, nil
}

// SearchVoiceCatalog runs a local full-text search over the cached catalog.
// An empty query lists the catalog. source filters on the `source` column
// ("self" or "public"); "all" or an empty string keeps everything.
func (s *Store) SearchVoiceCatalog(ctx context.Context, query, source string, limit int) ([]VoiceCatalogRow, error) {
	if limit <= 0 {
		limit = 20
	}
	var (
		sqlText string
		args    []any
	)
	base := `SELECT c.id, c.title, c.description, c.tags, c.author, c.languages, c.visibility, c.source, c.synced_at FROM voice_catalog c`
	if strings.TrimSpace(query) == "" {
		sqlText = base
		if source != "" && source != "all" {
			sqlText += " WHERE c.source = ?"
			args = append(args, source)
		}
		sqlText += " ORDER BY c.title ASC LIMIT ?"
	} else {
		sqlText = base + ` JOIN voice_catalog_fts f ON f.id = c.id WHERE voice_catalog_fts MATCH ?`
		args = append(args, FTSMatchQuery(query))
		if source != "" && source != "all" {
			sqlText += " AND c.source = ?"
			args = append(args, source)
		}
		sqlText += " ORDER BY rank LIMIT ?"
	}
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return nil, fmt.Errorf("searching voice_catalog: %w", err)
	}
	defer rows.Close()
	out := make([]VoiceCatalogRow, 0)
	for rows.Next() {
		var (
			row                                                VoiceCatalogRow
			title, desc, tags, author, langs, vis, src, synced sql.NullString
		)
		if err := rows.Scan(&row.ID, &title, &desc, &tags, &author, &langs, &vis, &src, &synced); err != nil {
			return nil, fmt.Errorf("scanning voice_catalog row: %w", err)
		}
		row.Title = title.String
		row.Description = desc.String
		row.Tags = tags.String
		row.Author = author.String
		row.Languages = langs.String
		row.Visibility = vis.String
		row.Source = src.String
		row.SyncedAt = synced.String
		out = append(out, row)
	}
	return out, rows.Err()
}

// VoiceCatalogCount reports how many voices the local cache holds.
func (s *Store) VoiceCatalogCount(ctx context.Context) (int64, error) {
	var n int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM voice_catalog`).Scan(&n); err != nil {
		return 0, fmt.Errorf("counting voice_catalog: %w", err)
	}
	return n, nil
}
