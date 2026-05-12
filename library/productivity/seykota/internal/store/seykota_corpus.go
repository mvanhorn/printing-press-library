// Copyright 2026 kjuju600. Licensed under Apache-2.0. See LICENSE.

// This file adds the seykota.com content archive to the generated store.
// It lives alongside store.go but is not generated — it owns the `corpus`
// table and its FTS5 index, and the read/write helpers the seykota
// commands use. The schema is created lazily via EnsureCorpus (idempotent,
// IF NOT EXISTS) so it does not need a StoreSchemaVersion bump.
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/seykota/internal/corpus"
)

var corpusSchemaStatements = []string{
	`CREATE TABLE IF NOT EXISTS corpus (
		id            TEXT PRIMARY KEY,
		source        TEXT NOT NULL,
		url           TEXT NOT NULL,
		title         TEXT,
		year          TEXT,
		month         TEXT,
		month_n       INTEGER,
		range         TEXT,
		slug          TEXT,
		updated       TEXT,
		section       TEXT,
		ord           INTEGER,
		contributors  TEXT,
		body          TEXT NOT NULL,
		fetched_at    TEXT NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_corpus_source ON corpus(source)`,
	`CREATE INDEX IF NOT EXISTS idx_corpus_year ON corpus(year)`,
	`CREATE INDEX IF NOT EXISTS idx_corpus_slug ON corpus(slug)`,
	`CREATE VIRTUAL TABLE IF NOT EXISTS corpus_fts USING fts5(
		title, body, content='corpus', content_rowid='rowid', tokenize='porter unicode61'
	)`,
	`CREATE TRIGGER IF NOT EXISTS corpus_ai AFTER INSERT ON corpus BEGIN
		INSERT INTO corpus_fts(rowid, title, body) VALUES (new.rowid, new.title, new.body);
	END`,
	`CREATE TRIGGER IF NOT EXISTS corpus_ad AFTER DELETE ON corpus BEGIN
		INSERT INTO corpus_fts(corpus_fts, rowid, title, body) VALUES('delete', old.rowid, old.title, old.body);
	END`,
	`CREATE TRIGGER IF NOT EXISTS corpus_au AFTER UPDATE ON corpus BEGIN
		INSERT INTO corpus_fts(corpus_fts, rowid, title, body) VALUES('delete', old.rowid, old.title, old.body);
		INSERT INTO corpus_fts(rowid, title, body) VALUES (new.rowid, new.title, new.body);
	END`,
}

// EnsureCorpus creates the corpus table, indexes, FTS5 index and triggers
// if they do not already exist. Safe to call on every command.
func (s *Store) EnsureCorpus(ctx context.Context) error {
	for _, stmt := range corpusSchemaStatements {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("ensuring corpus schema: %w", err)
		}
	}
	return nil
}

// CorpusCount returns the number of indexed documents (optionally limited
// to one source: "faq", "tsp", "risk", or "" for all).
func (s *Store) CorpusCount(source string) (int, error) {
	q := `SELECT COUNT(*) FROM corpus`
	args := []any{}
	if source != "" {
		q += ` WHERE source = ?`
		args = append(args, source)
	}
	var n int
	if err := s.db.QueryRow(q, args...).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

// CorpusEmpty reports whether the corpus table has zero rows (or doesn't
// exist yet).
func (s *Store) CorpusEmpty(ctx context.Context) bool {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM corpus`).Scan(&n)
	return err != nil || n == 0
}

// CorpusFetchedAt returns the most recent fetched_at across all corpus rows
// (RFC3339), or "" if the corpus is empty / has no parseable timestamp.
func (s *Store) CorpusFetchedAt() string {
	var v sql.NullString
	if err := s.db.QueryRow(`SELECT MAX(fetched_at) FROM corpus`).Scan(&v); err != nil {
		return ""
	}
	return v.String
}

// ReplaceCorpus deletes all corpus rows and inserts docs in a single
// transaction. Used by `index build`.
func (s *Store) ReplaceCorpus(ctx context.Context, docs []corpus.Doc) (int, error) {
	if err := s.EnsureCorpus(ctx); err != nil {
		return 0, err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM corpus`); err != nil {
		return 0, err
	}
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO corpus
		(id, source, url, title, year, month, month_n, range, slug, updated, section, ord, contributors, body, fetched_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()
	n := 0
	for _, d := range docs {
		if d.FetchedAt == "" {
			d.FetchedAt = time.Now().UTC().Format(time.RFC3339)
		}
		contribJSON := ""
		if len(d.Contributors) > 0 {
			b, _ := json.Marshal(d.Contributors)
			contribJSON = string(b)
		}
		if _, err := stmt.ExecContext(ctx,
			d.ID, d.Source, d.URL, d.Title, nullStr(d.Year), nullStr(d.Month), d.MonthN,
			nullStr(d.Range), nullStr(d.Slug), nullStr(d.Updated), nullStr(d.Section), d.Ord,
			nullStr(contribJSON), d.Body, d.FetchedAt); err != nil {
			return 0, fmt.Errorf("inserting %s: %w", d.ID, err)
		}
		n++
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return n, nil
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func scanDoc(rs *sql.Rows) (corpus.Doc, error) {
	var d corpus.Doc
	var year, month, rng, slug, updated, section, contrib sql.NullString
	var monthN, ord sql.NullInt64
	if err := rs.Scan(&d.ID, &d.Source, &d.URL, &d.Title, &year, &month, &monthN, &rng, &slug, &updated, &section, &ord, &contrib, &d.Body, &d.FetchedAt); err != nil {
		return d, err
	}
	d.Year, d.Month, d.Range, d.Slug, d.Updated, d.Section = year.String, month.String, rng.String, slug.String, updated.String, section.String
	d.MonthN, d.Ord = int(monthN.Int64), int(ord.Int64)
	if contrib.Valid && contrib.String != "" {
		_ = json.Unmarshal([]byte(contrib.String), &d.Contributors)
	}
	return d, nil
}

const docCols = `id, source, url, title, year, month, month_n, range, slug, updated, section, ord, contributors, body, fetched_at`

// GetDoc returns a single doc by ID.
func (s *Store) GetDoc(id string) (corpus.Doc, error) {
	rs, err := s.db.Query(`SELECT `+docCols+` FROM corpus WHERE id = ?`, id)
	if err != nil {
		return corpus.Doc{}, err
	}
	defer rs.Close()
	if !rs.Next() {
		return corpus.Doc{}, sql.ErrNoRows
	}
	return scanDoc(rs)
}

// SearchHit is one full-text search result.
type SearchHit struct {
	corpus.Doc
	Snippet string  `json:"snippet"`
	Rank    float64 `json:"-"`
}

// SearchOpts narrows a full-text search.
type SearchOpts struct {
	Source string // "" | "faq" | "tsp" | "risk"
	Year   string // "" or a 4-digit year (FAQ)
	Limit  int    // 0 -> 20
}

// SearchCorpus runs an FTS5 query over the title+body of every indexed
// document, optionally filtered by source/year, ranked by bm25.
func (s *Store) SearchCorpus(query string, opts SearchOpts) ([]SearchHit, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil, fmt.Errorf("empty search query")
	}
	limit := opts.Limit
	if limit <= 0 {
		limit = 20
	}
	sb := &strings.Builder{}
	sb.WriteString(`SELECT c.id, c.source, c.url, c.title, c.year, c.month, c.month_n, c.range, c.slug, c.updated, c.section, c.ord, c.contributors, c.body, c.fetched_at,
		snippet(corpus_fts, 1, '«', '»', ' … ', 14) AS snip, bm25(corpus_fts) AS rank
		FROM corpus_fts JOIN corpus c ON c.rowid = corpus_fts.rowid
		WHERE corpus_fts MATCH ?`)
	args := []any{ftsQuery(q)}
	if opts.Source != "" {
		sb.WriteString(` AND c.source = ?`)
		args = append(args, opts.Source)
	}
	if opts.Year != "" {
		sb.WriteString(` AND c.year = ?`)
		args = append(args, opts.Year)
	}
	sb.WriteString(` ORDER BY rank LIMIT ?`)
	args = append(args, limit)

	rs, err := s.db.Query(sb.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var out []SearchHit
	for rs.Next() {
		var d corpus.Doc
		var year, month, rng, slug, updated, section, contrib sql.NullString
		var monthN, ord sql.NullInt64
		var snip string
		var rank float64
		if err := rs.Scan(&d.ID, &d.Source, &d.URL, &d.Title, &year, &month, &monthN, &rng, &slug, &updated, &section, &ord, &contrib, &d.Body, &d.FetchedAt, &snip, &rank); err != nil {
			return nil, err
		}
		d.Year, d.Month, d.Range, d.Slug, d.Updated, d.Section = year.String, month.String, rng.String, slug.String, updated.String, section.String
		d.MonthN, d.Ord = int(monthN.Int64), int(ord.Int64)
		if contrib.Valid && contrib.String != "" {
			_ = json.Unmarshal([]byte(contrib.String), &d.Contributors)
		}
		out = append(out, SearchHit{Doc: d, Snippet: strings.TrimSpace(snip), Rank: rank})
	}
	return out, rs.Err()
}

// ftsQuery turns a user query string into a safe FTS5 MATCH expression.
// Bare alphanumeric words are AND-ed (each wrapped as a prefix-quoted
// token); a query already containing FTS operators (quotes, OR, NEAR,
// parentheses, *) is passed through with each bare word still quoted to
// neutralize stray punctuation.
func ftsQuery(q string) string {
	q = strings.TrimSpace(q)
	// If the user quoted a phrase, honor it verbatim.
	if strings.HasPrefix(q, `"`) && strings.HasSuffix(q, `"`) && len(q) >= 2 {
		return q
	}
	hasOps := strings.ContainsAny(q, `"*()`) ||
		strings.Contains(q, " OR ") || strings.Contains(q, " NOT ") || strings.Contains(q, " NEAR")
	tokens := strings.FieldsFunc(q, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_')
	})
	var quoted []string
	for _, t := range tokens {
		t = strings.Trim(t, "-_")
		if t == "" {
			continue
		}
		quoted = append(quoted, `"`+t+`"`)
	}
	if len(quoted) == 0 {
		return `"` + strings.ReplaceAll(q, `"`, "") + `"`
	}
	if hasOps {
		// keep OR semantics when the user asked for them
		if strings.Contains(q, " OR ") {
			return strings.Join(quoted, " OR ")
		}
	}
	return strings.Join(quoted, " AND ")
}

// ListDocs returns docs for one source, ordered for display.
//
//	faq:  newest first (year desc, month_n desc)
//	tsp:  by ord, then slug
//	risk: by ord
func (s *Store) ListDocs(source string) ([]corpus.Doc, error) {
	order := `ord ASC, id ASC`
	switch source {
	case corpus.SourceFAQ:
		order = `year DESC, month_n DESC, id DESC`
	case corpus.SourceTSP:
		order = `ord ASC, slug ASC`
	}
	rs, err := s.db.Query(`SELECT `+docCols+` FROM corpus WHERE source = ? ORDER BY `+order, source)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var out []corpus.Doc
	for rs.Next() {
		d, err := scanDoc(rs)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rs.Err()
}

// FAQByYearMonth returns the FAQ doc for a year + month folder. month is
// matched case-insensitively against the stored folder name (e.g. "JAN"
// or "Jan"), or against the month number / 3-letter prefix.
func (s *Store) FAQByYearMonth(year, month string) (corpus.Doc, error) {
	month = strings.TrimSpace(month)
	rs, err := s.db.Query(`SELECT `+docCols+` FROM corpus WHERE source='faq' AND year = ?
		AND (lower(month) = lower(?) OR lower(substr(month,1,3)) = lower(substr(?,1,3))) ORDER BY id LIMIT 1`,
		year, month, month)
	if err != nil {
		return corpus.Doc{}, err
	}
	defer rs.Close()
	if rs.Next() {
		return scanDoc(rs)
	}
	// month-number fallback
	rs2, err := s.db.Query(`SELECT `+docCols+` FROM corpus WHERE source='faq' AND year = ? AND month_n = ? ORDER BY id LIMIT 1`, year, atoiSafe(month))
	if err != nil {
		return corpus.Doc{}, err
	}
	defer rs2.Close()
	if rs2.Next() {
		return scanDoc(rs2)
	}
	return corpus.Doc{}, sql.ErrNoRows
}

// TSPBySlug returns the TSP section doc whose slug matches (case-insensitive).
func (s *Store) TSPBySlug(slug string) (corpus.Doc, error) {
	rs, err := s.db.Query(`SELECT `+docCols+` FROM corpus WHERE source='tsp' AND (lower(slug)=lower(?) OR lower(title)=lower(?)) ORDER BY ord LIMIT 1`, slug, slug)
	if err != nil {
		return corpus.Doc{}, err
	}
	defer rs.Close()
	if rs.Next() {
		return scanDoc(rs)
	}
	return corpus.Doc{}, sql.ErrNoRows
}

// RiskDoc returns the (single) risk-essay doc.
func (s *Store) RiskDoc() (corpus.Doc, error) {
	rs, err := s.db.Query(`SELECT ` + docCols + ` FROM corpus WHERE source='risk' ORDER BY ord LIMIT 1`)
	if err != nil {
		return corpus.Doc{}, err
	}
	defer rs.Close()
	if rs.Next() {
		return scanDoc(rs)
	}
	return corpus.Doc{}, sql.ErrNoRows
}

// FAQYears returns the distinct FAQ years present, oldest first.
func (s *Store) FAQYears() ([]string, error) {
	rs, err := s.db.Query(`SELECT DISTINCT year FROM corpus WHERE source='faq' AND year IS NOT NULL AND year != '' ORDER BY year`)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var out []string
	for rs.Next() {
		var y string
		if err := rs.Scan(&y); err != nil {
			return nil, err
		}
		out = append(out, y)
	}
	return out, rs.Err()
}

// ContributorCount is one contributor with the number of FAQ months they
// appear in.
type ContributorCount struct {
	Name   string   `json:"name"`
	Months int      `json:"months"`
	When   []string `json:"when,omitempty"` // "YYYY MON", only when a name filter is given
}

// Contributors aggregates FAQ contributor mentions across the archive. With
// nameFilter empty, returns every contributor with a month count (most
// frequent first). With a non-empty nameFilter (case-insensitive substring),
// returns matching contributors with the list of months they appear in.
func (s *Store) Contributors(nameFilter string) ([]ContributorCount, error) {
	rs, err := s.db.Query(`SELECT year, month, contributors FROM corpus WHERE source='faq' AND contributors IS NOT NULL AND contributors != '' ORDER BY year DESC, month_n DESC`)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	type agg struct {
		count int
		when  []string
	}
	m := map[string]*agg{}
	display := map[string]string{}
	for rs.Next() {
		var year, month, cj string
		if err := rs.Scan(&year, &month, &cj); err != nil {
			return nil, err
		}
		var names []string
		if json.Unmarshal([]byte(cj), &names) != nil {
			continue
		}
		when := strings.TrimSpace(month + " " + year)
		for _, n := range names {
			n = strings.TrimSpace(n)
			if n == "" {
				continue
			}
			key := strings.ToLower(n)
			a := m[key]
			if a == nil {
				a = &agg{}
				m[key] = a
				display[key] = n
			}
			a.count++
			a.when = append(a.when, when)
		}
	}
	filter := strings.ToLower(strings.TrimSpace(nameFilter))
	var out []ContributorCount
	for key, a := range m {
		if filter != "" && !strings.Contains(key, filter) {
			continue
		}
		cc := ContributorCount{Name: display[key], Months: a.count}
		if filter != "" {
			cc.When = a.when
		}
		out = append(out, cc)
	}
	// stable sort: months desc, then name asc
	sortContributors(out)
	return out, rs.Err()
}

func sortContributors(cs []ContributorCount) {
	for i := 1; i < len(cs); i++ {
		for j := i; j > 0; j-- {
			if less := cs[j].Months > cs[j-1].Months || (cs[j].Months == cs[j-1].Months && cs[j].Name < cs[j-1].Name); less {
				cs[j], cs[j-1] = cs[j-1], cs[j]
			} else {
				break
			}
		}
	}
}

// ReadOnlyQuery runs a user-supplied SELECT against the store and returns
// column names + string-rendered rows. It rejects anything that isn't a
// single SELECT/WITH...SELECT statement (defense in depth on top of the
// driver's read-only connection check is not available here, so the guard
// is textual).
func (s *Store) ReadOnlyQuery(ctx context.Context, query string, max int) ([]string, [][]string, error) {
	trimmed := strings.TrimSpace(query)
	low := strings.ToLower(trimmed)
	if !(strings.HasPrefix(low, "select") || strings.HasPrefix(low, "with ")) {
		return nil, nil, fmt.Errorf("only SELECT queries are allowed")
	}
	for _, bad := range []string{"insert ", "update ", "delete ", "drop ", "alter ", "create ", "replace ", "attach ", "pragma ", "vacuum"} {
		if strings.Contains(low, bad) {
			return nil, nil, fmt.Errorf("query contains a forbidden keyword: %s", strings.TrimSpace(bad))
		}
	}
	if strings.Contains(trimmed, ";") && !strings.HasSuffix(trimmed, ";") {
		return nil, nil, fmt.Errorf("only a single statement is allowed")
	}
	if max <= 0 {
		max = 200
	}
	rs, err := s.db.QueryContext(ctx, trimmed)
	if err != nil {
		return nil, nil, err
	}
	defer rs.Close()
	cols, err := rs.Columns()
	if err != nil {
		return nil, nil, err
	}
	var rows [][]string
	for rs.Next() && len(rows) < max {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rs.Scan(ptrs...); err != nil {
			return nil, nil, err
		}
		row := make([]string, len(cols))
		for i, v := range vals {
			switch x := v.(type) {
			case nil:
				row[i] = ""
			case []byte:
				row[i] = string(x)
			default:
				row[i] = fmt.Sprintf("%v", x)
			}
		}
		rows = append(rows, row)
	}
	return cols, rows, rs.Err()
}

func atoiSafe(s string) int {
	n := 0
	for _, r := range strings.TrimSpace(s) {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int(r-'0')
	}
	return n
}
