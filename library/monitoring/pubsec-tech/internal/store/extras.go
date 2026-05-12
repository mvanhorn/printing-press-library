// Package store extras: hand-authored tables and helpers for novel features
// (RSS articles, sources, mention tags, NAICS/PSC reference tables, vendor
// watchlists). The generator does not emit these from the spec because they
// are not API-backed resources — they live entirely in the local store.

package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// MigrateExtras creates the novel-feature tables if they don't exist. Safe to
// call on every CLI invocation; CREATE TABLE IF NOT EXISTS is a no-op on
// subsequent runs. Called lazily by the novel commands before any read/write
// against the extras tables — keeps the cold path on absorbed-only invocations
// (e.g., `awards search`) zero-cost.
func (s *Store) MigrateExtras(ctx context.Context) error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS sources (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			feed_url TEXT NOT NULL,
			category TEXT,
			tier TEXT,
			enabled INTEGER NOT NULL DEFAULT 1,
			last_etag TEXT,
			last_modified TEXT,
			last_fetched_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS articles (
			id TEXT PRIMARY KEY,
			source_id TEXT NOT NULL,
			guid TEXT,
			title TEXT,
			link TEXT,
			summary TEXT,
			content TEXT,
			author TEXT,
			categories TEXT,
			published_at DATETIME,
			fetched_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			read INTEGER NOT NULL DEFAULT 0,
			FOREIGN KEY(source_id) REFERENCES sources(id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_articles_source ON articles(source_id)`,
		`CREATE INDEX IF NOT EXISTS idx_articles_published ON articles(published_at)`,
		`CREATE INDEX IF NOT EXISTS idx_articles_read ON articles(read)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS articles_fts USING fts5(
			title, summary, content, author, content='articles', content_rowid='rowid', tokenize='porter unicode61'
		)`,
		`CREATE TRIGGER IF NOT EXISTS articles_fts_insert AFTER INSERT ON articles BEGIN
			INSERT INTO articles_fts(rowid, title, summary, content, author) VALUES (new.rowid, new.title, new.summary, new.content, new.author);
		END`,
		`CREATE TRIGGER IF NOT EXISTS articles_fts_delete AFTER DELETE ON articles BEGIN
			INSERT INTO articles_fts(articles_fts, rowid, title, summary, content, author) VALUES('delete', old.rowid, old.title, old.summary, old.content, old.author);
		END`,
		`CREATE TRIGGER IF NOT EXISTS articles_fts_update AFTER UPDATE ON articles BEGIN
			INSERT INTO articles_fts(articles_fts, rowid, title, summary, content, author) VALUES('delete', old.rowid, old.title, old.summary, old.content, old.author);
			INSERT INTO articles_fts(rowid, title, summary, content, author) VALUES (new.rowid, new.title, new.summary, new.content, new.author);
		END`,
		`CREATE TABLE IF NOT EXISTS tags (
			article_id TEXT NOT NULL,
			kind TEXT NOT NULL,
			value TEXT NOT NULL,
			match_span TEXT,
			PRIMARY KEY(article_id, kind, value),
			FOREIGN KEY(article_id) REFERENCES articles(id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_tags_kind_value ON tags(kind, value)`,
		`CREATE TABLE IF NOT EXISTS naics_codes (
			code TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			parent_code TEXT,
			depth INTEGER
		)`,
		`CREATE INDEX IF NOT EXISTS idx_naics_title ON naics_codes(title)`,
		`CREATE TABLE IF NOT EXISTS psc_codes (
			code TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			category TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_psc_title ON psc_codes(title)`,
		`CREATE TABLE IF NOT EXISTS watchlists (
			kind TEXT NOT NULL,
			value TEXT NOT NULL,
			last_tick_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY(kind, value)
		)`,
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	for _, m := range migrations {
		if _, err := s.db.ExecContext(ctx, m); err != nil {
			return fmt.Errorf("extras migration failed (%s): %w", firstLine(m), err)
		}
	}
	return nil
}

func firstLine(s string) string {
	if i := strings.Index(s, "\n"); i >= 0 {
		return s[:i]
	}
	return s
}

// Source represents one configured RSS source.
type Source struct {
	ID            string
	Name          string
	FeedURL       string
	Category      string
	Tier          string
	Enabled       bool
	LastETag      string
	LastModified  string
	LastFetchedAt sql.NullTime
}

// Article represents one stored RSS article. The mention extractor reads
// title+summary+content and writes per-vendor/agency rows into `tags`.
type Article struct {
	ID          string
	SourceID    string
	GUID        string
	Title       string
	Link        string
	Summary     string
	Content     string
	Author      string
	Categories  string
	PublishedAt time.Time
	Read        bool
}

// UpsertSource inserts or updates a source. enabled defaults true on insert.
func (s *Store) UpsertSource(ctx context.Context, src Source) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO sources (id, name, feed_url, category, tier, enabled)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name=excluded.name, feed_url=excluded.feed_url, category=excluded.category, tier=excluded.tier
	`, src.ID, src.Name, src.FeedURL, src.Category, src.Tier, boolToInt(src.Enabled))
	return err
}

// SetSourceEnabled flips the enabled bit for one source.
func (s *Store) SetSourceEnabled(ctx context.Context, id string, enabled bool) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	res, err := s.db.ExecContext(ctx, `UPDATE sources SET enabled=? WHERE id=?`, boolToInt(enabled), id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("source not found: %s", id)
	}
	return nil
}

// SaveSourceFetchMetadata records ETag, Last-Modified, and last-fetched timestamps.
func (s *Store) SaveSourceFetchMetadata(ctx context.Context, id, etag, lastModified string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.ExecContext(ctx, `
		UPDATE sources SET last_etag=?, last_modified=?, last_fetched_at=CURRENT_TIMESTAMP WHERE id=?
	`, etag, lastModified, id)
	return err
}

// ListSources returns sources in alphabetical order. enabledOnly limits to enabled feeds.
func (s *Store) ListSources(ctx context.Context, enabledOnly bool) ([]Source, error) {
	q := `SELECT id, name, feed_url, COALESCE(category,''), COALESCE(tier,''), enabled,
	             COALESCE(last_etag,''), COALESCE(last_modified,''), last_fetched_at
	      FROM sources`
	if enabledOnly {
		q += ` WHERE enabled=1`
	}
	q += ` ORDER BY name`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Source
	for rows.Next() {
		var src Source
		var enabled int
		if err := rows.Scan(&src.ID, &src.Name, &src.FeedURL, &src.Category, &src.Tier, &enabled,
			&src.LastETag, &src.LastModified, &src.LastFetchedAt); err != nil {
			return nil, err
		}
		src.Enabled = enabled == 1
		out = append(out, src)
	}
	return out, rows.Err()
}

// UpsertArticle inserts or updates an article. Returns true if new (insert),
// false if it already existed (update path).
func (s *Store) UpsertArticle(ctx context.Context, a Article) (bool, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO articles (id, source_id, guid, title, link, summary, content, author, categories, published_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			title=excluded.title, summary=excluded.summary, content=excluded.content,
			author=excluded.author, categories=excluded.categories, published_at=excluded.published_at
	`, a.ID, a.SourceID, a.GUID, a.Title, a.Link, a.Summary, a.Content, a.Author, a.Categories, a.PublishedAt)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n == 1, nil
}

// ArticlesSince returns articles published or fetched after the given time,
// across enabled sources. limit caps the return; 0 means unlimited.
func (s *Store) ArticlesSince(ctx context.Context, since time.Time, limit int) ([]Article, error) {
	q := `SELECT a.id, a.source_id, COALESCE(a.guid,''), COALESCE(a.title,''),
	             COALESCE(a.link,''), COALESCE(a.summary,''), COALESCE(a.content,''),
	             COALESCE(a.author,''), COALESCE(a.categories,''), a.published_at, a.read
	      FROM articles a
	      INNER JOIN sources s ON s.id = a.source_id
	      WHERE s.enabled=1 AND COALESCE(a.published_at, a.fetched_at) >= ?
	      ORDER BY a.published_at DESC`
	args := []any{since}
	if limit > 0 {
		q += ` LIMIT ?`
		args = append(args, limit)
	}
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Article
	for rows.Next() {
		var a Article
		var pub sql.NullTime
		var read int
		if err := rows.Scan(&a.ID, &a.SourceID, &a.GUID, &a.Title, &a.Link,
			&a.Summary, &a.Content, &a.Author, &a.Categories, &pub, &read); err != nil {
			return nil, err
		}
		if pub.Valid {
			a.PublishedAt = pub.Time
		}
		a.Read = read == 1
		out = append(out, a)
	}
	return out, rows.Err()
}

// ArticleByURLOrTitle returns the article whose link matches url exactly, or
// whose title contains the haystack substring (case-insensitive). Used by
// `explain` so a user can paste either a URL or a headline.
func (s *Store) ArticleByURLOrTitle(ctx context.Context, hint string) (*Article, error) {
	hint = strings.TrimSpace(hint)
	if hint == "" {
		return nil, fmt.Errorf("empty article hint")
	}
	// First try exact link match
	var a Article
	var pub sql.NullTime
	var read int
	err := s.db.QueryRowContext(ctx, `
		SELECT id, source_id, COALESCE(guid,''), COALESCE(title,''), COALESCE(link,''),
		       COALESCE(summary,''), COALESCE(content,''), COALESCE(author,''),
		       COALESCE(categories,''), published_at, read
		FROM articles WHERE link=? LIMIT 1
	`, hint).Scan(&a.ID, &a.SourceID, &a.GUID, &a.Title, &a.Link, &a.Summary, &a.Content,
		&a.Author, &a.Categories, &pub, &read)
	if err == nil {
		if pub.Valid {
			a.PublishedAt = pub.Time
		}
		a.Read = read == 1
		return &a, nil
	}
	if err != sql.ErrNoRows {
		return nil, err
	}
	// Fall back to LIKE on title
	like := "%" + strings.ToLower(hint) + "%"
	err = s.db.QueryRowContext(ctx, `
		SELECT id, source_id, COALESCE(guid,''), COALESCE(title,''), COALESCE(link,''),
		       COALESCE(summary,''), COALESCE(content,''), COALESCE(author,''),
		       COALESCE(categories,''), published_at, read
		FROM articles WHERE LOWER(title) LIKE ? ORDER BY published_at DESC LIMIT 1
	`, like).Scan(&a.ID, &a.SourceID, &a.GUID, &a.Title, &a.Link, &a.Summary, &a.Content,
		&a.Author, &a.Categories, &pub, &read)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("no article matches %q (try a URL or a longer headline excerpt)", hint)
		}
		return nil, err
	}
	if pub.Valid {
		a.PublishedAt = pub.Time
	}
	a.Read = read == 1
	return &a, nil
}

// Tag represents one extracted mention linking an article to an entity.
type Tag struct {
	ArticleID string
	Kind      string // "recipient" or "agency"
	Value     string // canonical name
	MatchSpan string // the matched substring (for evidence)
}

// UpsertTagsForArticle replaces any existing tags for the article with the new set.
func (s *Store) UpsertTagsForArticle(ctx context.Context, articleID string, tags []Tag) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM tags WHERE article_id=?`, articleID); err != nil {
		return err
	}
	stmt, err := tx.PrepareContext(ctx, `INSERT OR IGNORE INTO tags (article_id, kind, value, match_span) VALUES (?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, t := range tags {
		if _, err := stmt.ExecContext(ctx, articleID, t.Kind, t.Value, t.MatchSpan); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// TagsForArticle returns all tags for an article.
func (s *Store) TagsForArticle(ctx context.Context, articleID string) ([]Tag, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT article_id, kind, value, COALESCE(match_span,'') FROM tags WHERE article_id=?
	`, articleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Tag
	for rows.Next() {
		var t Tag
		if err := rows.Scan(&t.ArticleID, &t.Kind, &t.Value, &t.MatchSpan); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// ArticlesForEntity returns recent articles tagged with the given entity.
func (s *Store) ArticlesForEntity(ctx context.Context, kind, value string, since time.Time, limit int) ([]Article, error) {
	q := `SELECT a.id, a.source_id, COALESCE(a.guid,''), COALESCE(a.title,''),
	             COALESCE(a.link,''), COALESCE(a.summary,''), COALESCE(a.content,''),
	             COALESCE(a.author,''), COALESCE(a.categories,''), a.published_at, a.read
	      FROM articles a
	      INNER JOIN tags t ON t.article_id = a.id
	      WHERE t.kind=? AND LOWER(t.value)=LOWER(?) AND COALESCE(a.published_at, a.fetched_at) >= ?
	      ORDER BY a.published_at DESC`
	args := []any{kind, value, since}
	if limit > 0 {
		q += ` LIMIT ?`
		args = append(args, limit)
	}
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Article
	for rows.Next() {
		var a Article
		var pub sql.NullTime
		var read int
		if err := rows.Scan(&a.ID, &a.SourceID, &a.GUID, &a.Title, &a.Link,
			&a.Summary, &a.Content, &a.Author, &a.Categories, &pub, &read); err != nil {
			return nil, err
		}
		if pub.Valid {
			a.PublishedAt = pub.Time
		}
		a.Read = read == 1
		out = append(out, a)
	}
	return out, rows.Err()
}

// CodeEntry is a row from naics_codes or psc_codes.
type CodeEntry struct {
	Code     string
	Title    string
	Category string // PSC category; empty for NAICS
	Parent   string // NAICS parent code; empty for PSC
	Depth    int    // NAICS depth; 0 for PSC
}

// UpsertCodes batch-inserts NAICS or PSC entries. table must be "naics_codes" or "psc_codes".
func (s *Store) UpsertCodes(ctx context.Context, table string, codes []CodeEntry) error {
	if table != "naics_codes" && table != "psc_codes" {
		return fmt.Errorf("unsupported codes table %q", table)
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var stmt *sql.Stmt
	if table == "naics_codes" {
		stmt, err = tx.PrepareContext(ctx,
			`INSERT INTO naics_codes (code, title, parent_code, depth) VALUES (?, ?, ?, ?)
			 ON CONFLICT(code) DO UPDATE SET title=excluded.title, parent_code=excluded.parent_code, depth=excluded.depth`)
	} else {
		stmt, err = tx.PrepareContext(ctx,
			`INSERT INTO psc_codes (code, title, category) VALUES (?, ?, ?)
			 ON CONFLICT(code) DO UPDATE SET title=excluded.title, category=excluded.category`)
	}
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, c := range codes {
		if table == "naics_codes" {
			if _, err := stmt.ExecContext(ctx, c.Code, c.Title, c.Parent, c.Depth); err != nil {
				return err
			}
		} else {
			if _, err := stmt.ExecContext(ctx, c.Code, c.Title, c.Category); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

// LookupCode resolves a NAICS or PSC term to (code, title) pairs.
// If the term is an exact code match, returns just that row. Otherwise does a
// LIKE search on the title field and returns up to `limit` results ordered by
// shortest title (tighter matches first).
func (s *Store) LookupCode(ctx context.Context, kind, term string, limit int) ([]CodeEntry, error) {
	table := "naics_codes"
	if kind == "psc" {
		table = "psc_codes"
	}
	term = strings.TrimSpace(term)
	if term == "" {
		return nil, fmt.Errorf("empty term")
	}
	// Exact code hit?
	var c CodeEntry
	q := fmt.Sprintf(`SELECT code, title, '' as category, '' as parent, 0 as depth FROM %s WHERE code=? LIMIT 1`, table)
	if table == "naics_codes" {
		q = fmt.Sprintf(`SELECT code, title, '' as category, COALESCE(parent_code,'') as parent, COALESCE(depth,0) as depth FROM %s WHERE code=? LIMIT 1`, table)
	} else {
		q = fmt.Sprintf(`SELECT code, title, COALESCE(category,'') as category, '' as parent, 0 as depth FROM %s WHERE code=? LIMIT 1`, table)
	}
	err := s.db.QueryRowContext(ctx, q, term).Scan(&c.Code, &c.Title, &c.Category, &c.Parent, &c.Depth)
	if err == nil {
		return []CodeEntry{c}, nil
	}
	if err != sql.ErrNoRows {
		return nil, err
	}
	// Title search
	like := "%" + strings.ToLower(term) + "%"
	if table == "naics_codes" {
		q = fmt.Sprintf(`SELECT code, title, '' as category, COALESCE(parent_code,'') as parent, COALESCE(depth,0) as depth FROM %s WHERE LOWER(title) LIKE ? ORDER BY LENGTH(title) ASC LIMIT ?`, table)
	} else {
		q = fmt.Sprintf(`SELECT code, title, COALESCE(category,'') as category, '' as parent, 0 as depth FROM %s WHERE LOWER(title) LIKE ? ORDER BY LENGTH(title) ASC LIMIT ?`, table)
	}
	rows, err := s.db.QueryContext(ctx, q, like, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CodeEntry
	for rows.Next() {
		var ce CodeEntry
		if err := rows.Scan(&ce.Code, &ce.Title, &ce.Category, &ce.Parent, &ce.Depth); err != nil {
			return nil, err
		}
		out = append(out, ce)
	}
	return out, rows.Err()
}

// ListCodes returns every row in the codes table, optionally filtered to a
// minimum depth (NAICS only). Used by `code list`.
func (s *Store) ListCodes(ctx context.Context, kind string, minDepth int) ([]CodeEntry, error) {
	table := "naics_codes"
	if kind == "psc" {
		table = "psc_codes"
	}
	var q string
	if table == "naics_codes" {
		q = fmt.Sprintf(`SELECT code, title, '' as category, COALESCE(parent_code,'') as parent, COALESCE(depth,0) as depth FROM %s WHERE COALESCE(depth,0) >= ? ORDER BY code`, table)
	} else {
		q = fmt.Sprintf(`SELECT code, title, COALESCE(category,'') as category, '' as parent, 0 as depth FROM %s ORDER BY code`, table)
	}
	args := []any{minDepth}
	if table == "psc_codes" {
		args = nil
		q = fmt.Sprintf(`SELECT code, title, COALESCE(category,'') as category, '' as parent, 0 as depth FROM %s ORDER BY code`, table)
	}
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CodeEntry
	for rows.Next() {
		var ce CodeEntry
		if err := rows.Scan(&ce.Code, &ce.Title, &ce.Category, &ce.Parent, &ce.Depth); err != nil {
			return nil, err
		}
		out = append(out, ce)
	}
	return out, rows.Err()
}

// CountCodes returns the number of rows in the codes table (used by `doctor`
// and the lazy-seed path).
func (s *Store) CountCodes(ctx context.Context, kind string) (int, error) {
	table := "naics_codes"
	if kind == "psc" {
		table = "psc_codes"
	}
	var n int
	err := s.db.QueryRowContext(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s`, table)).Scan(&n)
	return n, err
}

// ListRecipientNames returns canonical recipient names already in the local
// store. Used by the mention extractor.
func (s *Store) ListRecipientNames(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT name FROM recipients WHERE name IS NOT NULL AND name<>'' ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// ListAgencyNames returns the names + abbreviations of synced agencies.
func (s *Store) ListAgencyNames(ctx context.Context) ([]struct{ Name, Abbrev string }, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT COALESCE(name,''), COALESCE(abbreviation,'') FROM agencies ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []struct{ Name, Abbrev string }
	for rows.Next() {
		var n, a string
		if err := rows.Scan(&n, &a); err != nil {
			return nil, err
		}
		out = append(out, struct{ Name, Abbrev string }{n, a})
	}
	return out, rows.Err()
}

// RecipientByName returns the JSON of a recipient with the given name (LIKE-match).
func (s *Store) RecipientByName(ctx context.Context, name string) ([]byte, error) {
	like := "%" + strings.ToLower(strings.TrimSpace(name)) + "%"
	var data []byte
	err := s.db.QueryRowContext(ctx, `SELECT data FROM recipients WHERE LOWER(name) LIKE ? ORDER BY LENGTH(name) ASC LIMIT 1`, like).Scan(&data)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return data, nil
}

// AwardsRecompete returns awards with PoP-current-end-date inside the window,
// optionally filtered to a set of NAICS codes embedded in the JSON data.
// Returns the raw award JSON rows.
func (s *Store) AwardsRecompete(ctx context.Context, windowEnd time.Time, naicsCodes []string, limit int) ([]struct {
	ID   string
	Data []byte
}, error) {
	// We don't store PoP-end as a typed column, so this is best-effort using
	// json_extract. We accept the small perf hit because recompete radar runs
	// on demand against tens of thousands of rows, not millions.
	q := `SELECT id, data FROM awards
	      WHERE json_extract(data, '$.period_of_performance.end_date') IS NOT NULL
	        AND date(json_extract(data, '$.period_of_performance.end_date')) <= date(?)
	        AND date(json_extract(data, '$.period_of_performance.end_date')) >= date('now')`
	args := []any{windowEnd}
	if len(naicsCodes) > 0 {
		placeholders := strings.Repeat("?,", len(naicsCodes))
		placeholders = placeholders[:len(placeholders)-1]
		q += " AND json_extract(data, '$.naics.code') IN (" + placeholders + ")"
		for _, c := range naicsCodes {
			args = append(args, c)
		}
	}
	q += " ORDER BY date(json_extract(data, '$.period_of_performance.end_date')) ASC"
	if limit > 0 {
		q += " LIMIT ?"
		args = append(args, limit)
	}
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []struct {
		ID   string
		Data []byte
	}
	for rows.Next() {
		var r struct {
			ID   string
			Data []byte
		}
		if err := rows.Scan(&r.ID, &r.Data); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// OpportunitiesByTitle returns opportunities whose title or solicitation
// number contains a substring (LIKE match). Used by recompete to find
// already-posted follow-on RFPs and by news↔contract correlation.
func (s *Store) OpportunitiesByTitle(ctx context.Context, hint string, limit int) ([]struct {
	ID   string
	Data []byte
}, error) {
	like := "%" + strings.ToLower(strings.TrimSpace(hint)) + "%"
	q := `SELECT id, data FROM opportunities
	      WHERE LOWER(COALESCE(json_extract(data, '$.title'),'')) LIKE ?
	         OR LOWER(COALESCE(json_extract(data, '$.solicitationNumber'),'')) LIKE ?
	      ORDER BY id`
	args := []any{like, like}
	if limit > 0 {
		q += " LIMIT ?"
		args = append(args, limit)
	}
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []struct {
		ID   string
		Data []byte
	}
	for rows.Next() {
		var r struct {
			ID   string
			Data []byte
		}
		if err := rows.Scan(&r.ID, &r.Data); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// SetWatchlistTick advances the last-tick timestamp on a watchlist row.
// Returns the previous tick (zero time if first set).
func (s *Store) SetWatchlistTick(ctx context.Context, kind, value string) (time.Time, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	var prev sql.NullTime
	err := s.db.QueryRowContext(ctx, `SELECT last_tick_at FROM watchlists WHERE kind=? AND value=?`, kind, value).Scan(&prev)
	if err != nil && err != sql.ErrNoRows {
		return time.Time{}, err
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO watchlists (kind, value, last_tick_at) VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(kind, value) DO UPDATE SET last_tick_at=CURRENT_TIMESTAMP
	`, kind, value)
	if err != nil {
		return time.Time{}, err
	}
	if prev.Valid {
		return prev.Time, nil
	}
	return time.Time{}, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
