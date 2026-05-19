package thstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/trendhunter/internal/thparse"
)

type AuthorRow struct {
	Name       string `json:"name"`
	TrendCount int    `json:"trend_count"`
	FirstSeen  string `json:"first_seen"`
	LastSeen   string `json:"last_seen"`
}

type KeywordRow struct {
	Keyword string `json:"keyword"`
	Count   int    `json:"count"`
}

func EnsureSchema(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS parsed_trends (
			slug TEXT PRIMARY KEY,
			title TEXT,
			description TEXT,
			image_url TEXT,
			keywords TEXT,
			author TEXT,
			category TEXT,
			trend_id TEXT,
			pub_date TEXT,
			body_text TEXT,
			related_slugs TEXT,
			faq TEXT,
			source_url TEXT,
			source TEXT,
			first_seen DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_seen DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_parsed_trends_category_first_seen ON parsed_trends(category, first_seen)`,
		`CREATE INDEX IF NOT EXISTS idx_parsed_trends_author_first_seen ON parsed_trends(author, first_seen)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS parsed_trends_fts USING fts5(
			title, description, keywords, body_text,
			content='parsed_trends',
			content_rowid='rowid'
		)`,
		`CREATE TABLE IF NOT EXISTS sitemap_entries (
			url TEXT PRIMARY KEY,
			last_mod TEXT,
			kind TEXT,
			last_synced DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS inbox_cursor (
			id INTEGER PRIMARY KEY CHECK (id=1),
			last_seen_at DATETIME
		)`,
		// Keep parsed_trends_fts in sync with parsed_trends via triggers.
		// INSERT OR REPLACE on parsed_trends fires DELETE+INSERT, so the AI/AD
		// pair covers upsert; the AU trigger handles plain UPDATE statements.
		`CREATE TRIGGER IF NOT EXISTS parsed_trends_fts_ai AFTER INSERT ON parsed_trends BEGIN
			INSERT INTO parsed_trends_fts(rowid, title, description, keywords, body_text)
			VALUES (new.rowid, new.title, new.description, new.keywords, new.body_text);
		END`,
		`CREATE TRIGGER IF NOT EXISTS parsed_trends_fts_ad AFTER DELETE ON parsed_trends BEGIN
			INSERT INTO parsed_trends_fts(parsed_trends_fts, rowid, title, description, keywords, body_text)
			VALUES('delete', old.rowid, old.title, old.description, old.keywords, old.body_text);
		END`,
		`CREATE TRIGGER IF NOT EXISTS parsed_trends_fts_au AFTER UPDATE ON parsed_trends BEGIN
			INSERT INTO parsed_trends_fts(parsed_trends_fts, rowid, title, description, keywords, body_text)
			VALUES('delete', old.rowid, old.title, old.description, old.keywords, old.body_text);
			INSERT INTO parsed_trends_fts(rowid, title, description, keywords, body_text)
			VALUES (new.rowid, new.title, new.description, new.keywords, new.body_text);
		END`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	// One-time rebuild for upgrade safety: if parsed_trends has data but the
	// FTS index is empty (older schema without triggers), fill the FTS index
	// once. Triggers maintain it from there on.
	var hasFTS, hasData bool
	if err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM parsed_trends_fts)`).Scan(&hasFTS); err != nil {
		return err
	}
	if err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM parsed_trends)`).Scan(&hasData); err != nil {
		return err
	}
	if !hasFTS && hasData {
		if _, err := db.Exec(`INSERT INTO parsed_trends_fts(parsed_trends_fts) VALUES('rebuild')`); err != nil {
			return err
		}
	}
	return nil
}

func UpsertTrend(ctx context.Context, db *sql.DB, t thparse.Trend) error {
	if t.Slug == "" {
		return fmt.Errorf("trend slug is required")
	}
	keywords := strings.Join(t.Keywords, ", ")
	related, err := json.Marshal(t.RelatedSlugs)
	if err != nil {
		return err
	}
	faq, err := json.Marshal(t.FAQ)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, `INSERT OR REPLACE INTO parsed_trends
		(slug, title, description, image_url, keywords, author, category, trend_id, pub_date, body_text, related_slugs, faq, source_url, source, first_seen, last_seen)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
			COALESCE((SELECT first_seen FROM parsed_trends WHERE slug = ?), CURRENT_TIMESTAMP),
			CURRENT_TIMESTAMP)`,
		t.Slug, t.Title, t.Description, t.ImageURL, keywords, t.Author, t.Category, t.TrendID,
		t.PubDate, t.BodyText, string(related), string(faq), t.SourceURL, t.Source, t.Slug)
	return err
}

func GetTrend(ctx context.Context, db *sql.DB, slug string) (*thparse.Trend, bool, error) {
	row := db.QueryRowContext(ctx, `SELECT slug, title, description, image_url, keywords, author, category, trend_id, pub_date, body_text, related_slugs, faq, source_url, source
		FROM parsed_trends WHERE slug = ?`, slug)
	t, err := scanTrend(row)
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return t, true, nil
}

func SearchTrends(ctx context.Context, db *sql.DB, query string, limit int) ([]thparse.Trend, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := db.QueryContext(ctx, `SELECT p.slug, p.title, p.description, p.image_url, p.keywords, p.author, p.category, p.trend_id, p.pub_date, p.body_text, p.related_slugs, p.faq, p.source_url, p.source
		FROM parsed_trends_fts f
		JOIN parsed_trends p ON p.rowid = f.rowid
		WHERE parsed_trends_fts MATCH ?
		ORDER BY bm25(parsed_trends_fts)
		LIMIT ?`, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTrends(rows)
}

func ListTrendsByCategory(ctx context.Context, db *sql.DB, category string, since time.Duration, limit int) ([]thparse.Trend, error) {
	if limit <= 0 {
		limit = 50
	}
	cutoff := time.Now().Add(-since).UTC().Format(time.RFC3339)
	where := `first_seen >= ?`
	args := []any{cutoff}
	if category != "" {
		where += ` AND category = ?`
		args = append(args, category)
	}
	args = append(args, limit)
	rows, err := db.QueryContext(ctx, `SELECT slug, title, description, image_url, keywords, author, category, trend_id, pub_date, body_text, related_slugs, faq, source_url, source
		FROM parsed_trends WHERE `+where+` ORDER BY first_seen DESC LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTrends(rows)
}

func ListAuthorVelocity(ctx context.Context, db *sql.DB, since time.Duration, top int) ([]AuthorRow, error) {
	if top <= 0 {
		top = 20
	}
	cutoff := time.Now().Add(-since).UTC().Format(time.RFC3339)
	rows, err := db.QueryContext(ctx, `SELECT author, COUNT(*), MIN(first_seen), MAX(last_seen)
		FROM parsed_trends
		WHERE author <> '' AND first_seen >= ?
		GROUP BY author
		ORDER BY COUNT(*) DESC, MAX(last_seen) DESC
		LIMIT ?`, cutoff, top)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AuthorRow
	for rows.Next() {
		var r AuthorRow
		if err := rows.Scan(&r.Name, &r.TrendCount, &r.FirstSeen, &r.LastSeen); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func KeywordCounts(ctx context.Context, db *sql.DB, since time.Duration, limit int) ([]KeywordRow, error) {
	cutoff := time.Now().Add(-since).UTC().Format(time.RFC3339)
	return keywordCountsWhere(ctx, db, `first_seen >= ?`, []any{cutoff}, limit)
}

func UpsertSitemap(ctx context.Context, db *sql.DB, e thparse.SitemapEntry) error {
	if e.URL == "" {
		return nil
	}
	_, err := db.ExecContext(ctx, `INSERT INTO sitemap_entries (url, last_mod, kind, last_synced)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(url) DO UPDATE SET last_mod=excluded.last_mod, kind=excluded.kind, last_synced=CURRENT_TIMESTAMP`,
		e.URL, e.LastMod, e.Kind)
	return err
}

func LookupCursor(ctx context.Context, db *sql.DB) (time.Time, bool, error) {
	var raw string
	err := db.QueryRowContext(ctx, `SELECT last_seen_at FROM inbox_cursor WHERE id = 1`).Scan(&raw)
	if err == sql.ErrNoRows {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, err
	}
	t, err := parseDBTime(raw)
	if err != nil {
		return time.Time{}, false, err
	}
	return t, true, nil
}

func UpdateCursor(ctx context.Context, db *sql.DB, t time.Time) error {
	_, err := db.ExecContext(ctx, `INSERT INTO inbox_cursor (id, last_seen_at) VALUES (1, ?)
		ON CONFLICT(id) DO UPDATE SET last_seen_at=excluded.last_seen_at`, t.UTC().Format(time.RFC3339))
	return err
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanTrend(row rowScanner) (*thparse.Trend, error) {
	var t thparse.Trend
	var keywords, related, faq string
	if err := row.Scan(&t.Slug, &t.Title, &t.Description, &t.ImageURL, &keywords, &t.Author, &t.Category,
		&t.TrendID, &t.PubDate, &t.BodyText, &related, &faq, &t.SourceURL, &t.Source); err != nil {
		return nil, err
	}
	t.Keywords = splitStoredList(keywords)
	_ = json.Unmarshal([]byte(related), &t.RelatedSlugs)
	_ = json.Unmarshal([]byte(faq), &t.FAQ)
	return &t, nil
}

func scanTrends(rows *sql.Rows) ([]thparse.Trend, error) {
	var out []thparse.Trend
	for rows.Next() {
		t, err := scanTrend(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *t)
	}
	return out, rows.Err()
}

func splitStoredList(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func keywordCountsWhere(ctx context.Context, db *sql.DB, where string, args []any, limit int) ([]KeywordRow, error) {
	query := `SELECT keywords FROM parsed_trends`
	if where != "" {
		query += ` WHERE ` + where
	}
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	counts := map[string]int{}
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		for _, kw := range splitStoredList(raw) {
			kw = strings.ToLower(strings.TrimSpace(kw))
			if kw != "" {
				counts[kw]++
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]KeywordRow, 0, len(counts))
	for kw, n := range counts {
		out = append(out, KeywordRow{Keyword: kw, Count: n})
	}
	sortKeywordRows(out)
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func sortKeywordRows(rows []KeywordRow) {
	for i := 1; i < len(rows); i++ {
		for j := i; j > 0 && keywordLess(rows[j], rows[j-1]); j-- {
			rows[j], rows[j-1] = rows[j-1], rows[j]
		}
	}
}

func keywordLess(a, b KeywordRow) bool {
	if a.Count != b.Count {
		return a.Count > b.Count
	}
	return a.Keyword < b.Keyword
}

func parseDBTime(raw string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02T15:04:05Z07:00"} {
		if t, err := time.Parse(layout, raw); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid cursor time %q", raw)
}
