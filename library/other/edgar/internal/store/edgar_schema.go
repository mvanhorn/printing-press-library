// Copyright 2026 magoo242. Licensed under Apache-2.0. See LICENSE.

// EDGAR-specific tables for typed, command-driven data (separate from the
// generic resources/companies/filings tables produced by the generator).
// These power the LODESTAR hand-built commands: insider-summary, xbrl-pivot,
// fts, since, eightk-items, etc.

package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// EnsureEdgarSchema creates the EDGAR domain tables on first use. Idempotent;
// invoked lazily from the hand-built commands so the generator's migration
// pipeline doesn't need to know about these tables.
func (s *Store) EnsureEdgarSchema(ctx context.Context) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS edgar_companies (
			cik TEXT PRIMARY KEY,
			ticker TEXT NOT NULL,
			name TEXT NOT NULL,
			sic TEXT,
			cached_at INTEGER NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_edgar_companies_ticker ON edgar_companies(ticker)`,
		`CREATE TABLE IF NOT EXISTS edgar_filings (
			accession TEXT PRIMARY KEY,
			cik TEXT NOT NULL,
			form_type TEXT NOT NULL,
			filed_at TEXT NOT NULL,
			primary_doc_url TEXT,
			title TEXT,
			body_text TEXT,
			body_cached_at INTEGER,
			cached_at INTEGER NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_edgar_filings_cik_form ON edgar_filings(cik, form_type)`,
		`CREATE INDEX IF NOT EXISTS idx_edgar_filings_filed_at ON edgar_filings(filed_at)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS edgar_filings_fts USING fts5(
			accession UNINDEXED,
			cik UNINDEXED,
			form_type UNINDEXED,
			filed_at UNINDEXED,
			body_text,
			tokenize='porter unicode61'
		)`,
		`CREATE TABLE IF NOT EXISTS edgar_insider_transactions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			accession TEXT NOT NULL,
			cik TEXT NOT NULL,
			reporter_cik TEXT NOT NULL,
			reporter_name TEXT NOT NULL,
			reporter_title TEXT,
			is_senior_officer INTEGER NOT NULL DEFAULT 0,
			is_director INTEGER NOT NULL DEFAULT 0,
			transaction_date TEXT NOT NULL,
			transaction_code TEXT NOT NULL,
			is_discretionary INTEGER NOT NULL DEFAULT 0,
			shares REAL NOT NULL DEFAULT 0,
			price_per_share REAL,
			value_usd REAL,
			acquired_disposed TEXT,
			shares_owned_after REAL,
			UNIQUE(accession, reporter_cik, transaction_date, transaction_code, shares)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_edgar_insider_cik_date ON edgar_insider_transactions(cik, transaction_date)`,
		`CREATE INDEX IF NOT EXISTS idx_edgar_insider_senior ON edgar_insider_transactions(cik, is_senior_officer, transaction_date)`,
		`CREATE TABLE IF NOT EXISTS edgar_xbrl_facts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			cik TEXT NOT NULL,
			concept TEXT NOT NULL,
			unit TEXT NOT NULL,
			period_end TEXT NOT NULL,
			fiscal_year INTEGER NOT NULL,
			fiscal_period TEXT NOT NULL,
			value REAL NOT NULL,
			form_type TEXT,
			filed_at TEXT,
			UNIQUE(cik, concept, period_end, fiscal_period, unit)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_edgar_xbrl_cik_concept ON edgar_xbrl_facts(cik, concept, period_end)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("edgar schema: %w", err)
		}
	}
	return nil
}

// EdgarCompany is a cached ticker→CIK row.
type EdgarCompany struct {
	CIK      string `json:"cik"`
	Ticker   string `json:"ticker"`
	Name     string `json:"name"`
	SIC      string `json:"sic,omitempty"`
	CachedAt int64  `json:"cached_at"`
}

// UpsertEdgarCompany inserts or updates a ticker→CIK mapping.
func (s *Store) UpsertEdgarCompany(ctx context.Context, c EdgarCompany) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if c.CachedAt == 0 {
		c.CachedAt = time.Now().Unix()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO edgar_companies (cik, ticker, name, sic, cached_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(cik) DO UPDATE SET ticker=excluded.ticker, name=excluded.name, sic=excluded.sic, cached_at=excluded.cached_at`,
		c.CIK, strings.ToUpper(c.Ticker), c.Name, c.SIC, c.CachedAt,
	)
	return err
}

// LookupEdgarCompanyByTicker returns the cached EdgarCompany for ticker, or
// sql.ErrNoRows when absent.
func (s *Store) LookupEdgarCompanyByTicker(ctx context.Context, ticker string) (EdgarCompany, error) {
	var c EdgarCompany
	row := s.db.QueryRowContext(ctx,
		`SELECT cik, ticker, name, COALESCE(sic,''), cached_at FROM edgar_companies WHERE ticker = ? LIMIT 1`,
		strings.ToUpper(ticker),
	)
	err := row.Scan(&c.CIK, &c.Ticker, &c.Name, &c.SIC, &c.CachedAt)
	return c, err
}

// EdgarFiling is a cached filing row.
type EdgarFiling struct {
	Accession     string `json:"accession"`
	CIK           string `json:"cik"`
	FormType      string `json:"form_type"`
	FiledAt       string `json:"filed_at"`
	PrimaryDocURL string `json:"primary_doc_url,omitempty"`
	Title         string `json:"title,omitempty"`
	BodyText      string `json:"body_text,omitempty"`
	BodyCachedAt  int64  `json:"body_cached_at,omitempty"`
	CachedAt      int64  `json:"cached_at"`
}

// UpsertEdgarFiling inserts/updates a filing row. If body is empty, preserves
// any existing body_text rather than clearing it.
func (s *Store) UpsertEdgarFiling(ctx context.Context, f EdgarFiling) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if f.CachedAt == 0 {
		f.CachedAt = time.Now().Unix()
	}
	if f.BodyText == "" {
		_, err := s.db.ExecContext(ctx,
			`INSERT INTO edgar_filings (accession, cik, form_type, filed_at, primary_doc_url, title, cached_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?)
			 ON CONFLICT(accession) DO UPDATE SET cik=excluded.cik, form_type=excluded.form_type, filed_at=excluded.filed_at,
				primary_doc_url=COALESCE(NULLIF(excluded.primary_doc_url,''), edgar_filings.primary_doc_url),
				title=COALESCE(NULLIF(excluded.title,''), edgar_filings.title),
				cached_at=excluded.cached_at`,
			f.Accession, f.CIK, f.FormType, f.FiledAt, f.PrimaryDocURL, f.Title, f.CachedAt,
		)
		return err
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO edgar_filings (accession, cik, form_type, filed_at, primary_doc_url, title, body_text, body_cached_at, cached_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(accession) DO UPDATE SET cik=excluded.cik, form_type=excluded.form_type, filed_at=excluded.filed_at,
			primary_doc_url=COALESCE(NULLIF(excluded.primary_doc_url,''), edgar_filings.primary_doc_url),
			title=COALESCE(NULLIF(excluded.title,''), edgar_filings.title),
			body_text=excluded.body_text, body_cached_at=excluded.body_cached_at, cached_at=excluded.cached_at`,
		f.Accession, f.CIK, f.FormType, f.FiledAt, f.PrimaryDocURL, f.Title, f.BodyText, f.BodyCachedAt, f.CachedAt,
	)
	if err != nil {
		return err
	}
	// Update FTS5 index. Delete then insert; FTS5 doesn't support stable UPDATE on UNINDEXED columns reliably.
	rowid := ftsRowID("edgar_filings", f.Accession)
	if _, e := s.db.ExecContext(ctx, `DELETE FROM edgar_filings_fts WHERE rowid = ?`, rowid); e != nil {
		// non-fatal
		_ = e
	}
	_, e := s.db.ExecContext(ctx,
		`INSERT INTO edgar_filings_fts (rowid, accession, cik, form_type, filed_at, body_text) VALUES (?, ?, ?, ?, ?, ?)`,
		rowid, f.Accession, f.CIK, f.FormType, f.FiledAt, f.BodyText,
	)
	return e
}

// GetEdgarFiling returns a filing row by accession.
func (s *Store) GetEdgarFiling(ctx context.Context, accession string) (EdgarFiling, error) {
	var f EdgarFiling
	var (
		body          sql.NullString
		bodyCachedAt  sql.NullInt64
		primaryDocURL sql.NullString
		title         sql.NullString
	)
	row := s.db.QueryRowContext(ctx,
		`SELECT accession, cik, form_type, filed_at, primary_doc_url, title, body_text, body_cached_at, cached_at
		 FROM edgar_filings WHERE accession = ?`, accession,
	)
	if err := row.Scan(&f.Accession, &f.CIK, &f.FormType, &f.FiledAt, &primaryDocURL, &title, &body, &bodyCachedAt, &f.CachedAt); err != nil {
		return f, err
	}
	if primaryDocURL.Valid {
		f.PrimaryDocURL = primaryDocURL.String
	}
	if title.Valid {
		f.Title = title.String
	}
	if body.Valid {
		f.BodyText = body.String
	}
	if bodyCachedAt.Valid {
		f.BodyCachedAt = bodyCachedAt.Int64
	}
	return f, nil
}

// PATCH(greptile-form4-limit-truncation-signal): CountEdgarFilings returns the
// unfiltered count of cached filings matching cik/formTypes/since, ignoring
// any limit clause. Used by Form 4 ingest to detect when LIMIT-clamped
// ListEdgarFilings results silently truncated a high-volume issuer's filings
// in the window.
func (s *Store) CountEdgarFilings(ctx context.Context, cik string, formTypes []string, since string) (int, error) {
	q := `SELECT COUNT(*) FROM edgar_filings WHERE cik = ?`
	args := []any{cik}
	if len(formTypes) > 0 {
		placeholders := make([]string, len(formTypes))
		for i, ft := range formTypes {
			placeholders[i] = "?"
			args = append(args, ft)
		}
		q += " AND form_type IN (" + strings.Join(placeholders, ",") + ")"
	}
	if since != "" {
		q += " AND filed_at >= ?"
		args = append(args, since)
	}
	var n int
	if err := s.db.QueryRowContext(ctx, q, args...).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

// ListEdgarFilings returns filings matching the optional filters. since="" means no since filter.
func (s *Store) ListEdgarFilings(ctx context.Context, cik string, formTypes []string, since string, limit int) ([]EdgarFiling, error) {
	q := `SELECT accession, cik, form_type, filed_at, COALESCE(primary_doc_url,''), COALESCE(title,''), CASE WHEN body_text IS NULL THEN 0 ELSE 1 END, COALESCE(body_cached_at,0), cached_at FROM edgar_filings WHERE cik = ?`
	args := []any{cik}
	if len(formTypes) > 0 {
		placeholders := make([]string, len(formTypes))
		for i, ft := range formTypes {
			placeholders[i] = "?"
			args = append(args, ft)
		}
		q += " AND form_type IN (" + strings.Join(placeholders, ",") + ")"
	}
	if since != "" {
		q += " AND filed_at >= ?"
		args = append(args, since)
	}
	q += " ORDER BY filed_at DESC"
	if limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", limit)
	}
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []EdgarFiling
	for rows.Next() {
		var f EdgarFiling
		var bodyCached int
		if err := rows.Scan(&f.Accession, &f.CIK, &f.FormType, &f.FiledAt, &f.PrimaryDocURL, &f.Title, &bodyCached, &f.BodyCachedAt, &f.CachedAt); err != nil {
			return nil, err
		}
		// re-use BodyText as a "body cached" indicator via empty string vs non-empty?
		// Instead just leave BodyText empty; callers can call GetEdgarFiling for body.
		_ = bodyCached
		out = append(out, f)
	}
	return out, rows.Err()
}

// EdgarInsiderTransaction is one row from a parsed Form 4 transaction table.
type EdgarInsiderTransaction struct {
	Accession        string  `json:"accession"`
	CIK              string  `json:"cik"`
	ReporterCIK      string  `json:"reporter_cik"`
	ReporterName     string  `json:"reporter_name"`
	ReporterTitle    string  `json:"reporter_title,omitempty"`
	IsSeniorOfficer  bool    `json:"is_senior_officer"`
	IsDirector       bool    `json:"is_director"`
	TransactionDate  string  `json:"transaction_date"`
	TransactionCode  string  `json:"transaction_code"`
	IsDiscretionary  bool    `json:"is_discretionary"`
	Shares           float64 `json:"shares"`
	PricePerShare    float64 `json:"price_per_share,omitempty"`
	ValueUSD         float64 `json:"value_usd,omitempty"`
	AcquiredDisposed string  `json:"acquired_disposed,omitempty"`
	SharesOwnedAfter float64 `json:"shares_owned_after,omitempty"`
}

// UpsertEdgarInsiderTransaction inserts a transaction (UNIQUE constraint
// dedupes naturally). Returns the rows-affected count for the caller.
func (s *Store) UpsertEdgarInsiderTransaction(ctx context.Context, t EdgarInsiderTransaction) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO edgar_insider_transactions
		 (accession, cik, reporter_cik, reporter_name, reporter_title, is_senior_officer, is_director,
		  transaction_date, transaction_code, is_discretionary, shares, price_per_share, value_usd,
		  acquired_disposed, shares_owned_after)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.Accession, t.CIK, t.ReporterCIK, t.ReporterName, t.ReporterTitle,
		boolToInt(t.IsSeniorOfficer), boolToInt(t.IsDirector),
		t.TransactionDate, t.TransactionCode, boolToInt(t.IsDiscretionary),
		t.Shares, nullableFloat(t.PricePerShare), nullableFloat(t.ValueUSD),
		t.AcquiredDisposed, nullableFloat(t.SharesOwnedAfter),
	)
	return err
}

// ListEdgarInsiderTransactions queries insider rows for a CIK.
func (s *Store) ListEdgarInsiderTransactions(ctx context.Context, cik, since string, seniorOnly bool) ([]EdgarInsiderTransaction, error) {
	q := `SELECT accession, cik, reporter_cik, reporter_name, COALESCE(reporter_title,''),
		is_senior_officer, is_director, transaction_date, transaction_code, is_discretionary,
		shares, COALESCE(price_per_share,0), COALESCE(value_usd,0), COALESCE(acquired_disposed,''), COALESCE(shares_owned_after,0)
		FROM edgar_insider_transactions WHERE cik = ?`
	args := []any{cik}
	if since != "" {
		q += " AND transaction_date >= ?"
		args = append(args, since)
	}
	if seniorOnly {
		q += " AND is_senior_officer = 1"
	}
	q += " ORDER BY transaction_date DESC"
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []EdgarInsiderTransaction
	for rows.Next() {
		var t EdgarInsiderTransaction
		var snr, dir, disc int
		if err := rows.Scan(&t.Accession, &t.CIK, &t.ReporterCIK, &t.ReporterName, &t.ReporterTitle,
			&snr, &dir, &t.TransactionDate, &t.TransactionCode, &disc,
			&t.Shares, &t.PricePerShare, &t.ValueUSD, &t.AcquiredDisposed, &t.SharesOwnedAfter); err != nil {
			return nil, err
		}
		t.IsSeniorOfficer = snr == 1
		t.IsDirector = dir == 1
		t.IsDiscretionary = disc == 1
		out = append(out, t)
	}
	return out, rows.Err()
}

// EdgarXBRLFact represents one row from a flattened companyfacts response.
type EdgarXBRLFact struct {
	CIK          string  `json:"cik"`
	Concept      string  `json:"concept"`
	Unit         string  `json:"unit"`
	PeriodEnd    string  `json:"period_end"`
	FiscalYear   int     `json:"fiscal_year"`
	FiscalPeriod string  `json:"fiscal_period"`
	Value        float64 `json:"value"`
	FormType     string  `json:"form_type,omitempty"`
	FiledAt      string  `json:"filed_at,omitempty"`
}

// UpsertEdgarXBRLFact inserts a fact; UNIQUE constraint dedupes.
func (s *Store) UpsertEdgarXBRLFact(ctx context.Context, f EdgarXBRLFact) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO edgar_xbrl_facts
		 (cik, concept, unit, period_end, fiscal_year, fiscal_period, value, form_type, filed_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		f.CIK, f.Concept, f.Unit, f.PeriodEnd, f.FiscalYear, f.FiscalPeriod, f.Value, f.FormType, f.FiledAt,
	)
	return err
}

// QueryEdgarXBRLFacts returns matching XBRL facts for a CIK + concepts.
func (s *Store) QueryEdgarXBRLFacts(ctx context.Context, cik string, concepts []string, sinceDate string) ([]EdgarXBRLFact, error) {
	if len(concepts) == 0 {
		return nil, nil
	}
	placeholders := make([]string, len(concepts))
	args := []any{cik}
	for i, c := range concepts {
		placeholders[i] = "?"
		args = append(args, c)
	}
	q := `SELECT cik, concept, unit, period_end, fiscal_year, fiscal_period, value, COALESCE(form_type,''), COALESCE(filed_at,'')
		FROM edgar_xbrl_facts WHERE cik = ? AND concept IN (` + strings.Join(placeholders, ",") + `)`
	if sinceDate != "" {
		q += " AND period_end >= ?"
		args = append(args, sinceDate)
	}
	q += " ORDER BY period_end DESC"
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []EdgarXBRLFact
	for rows.Next() {
		var f EdgarXBRLFact
		if err := rows.Scan(&f.CIK, &f.Concept, &f.Unit, &f.PeriodEnd, &f.FiscalYear, &f.FiscalPeriod, &f.Value, &f.FormType, &f.FiledAt); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// SearchEdgarFTS runs an FTS5 query against edgar_filings_fts.
type EdgarFTSHit struct {
	Accession  string `json:"accession"`
	CIK        string `json:"cik"`
	FormType   string `json:"form_type"`
	FiledAt    string `json:"filed_at"`
	Snippet    string `json:"snippet"`
	ByteOffset int    `json:"byte_offset"`
}

// SearchEdgarFTS runs an FTS5 MATCH and returns hits with a 200-char snippet.
func (s *Store) SearchEdgarFTS(ctx context.Context, query, cikFilter, formFilter string, limit int) ([]EdgarFTSHit, error) {
	if limit <= 0 {
		limit = 50
	}
	q := `SELECT accession, cik, form_type, filed_at,
		snippet(edgar_filings_fts, 4, '<mark>', '</mark>', '...', 16) as snip
		FROM edgar_filings_fts WHERE edgar_filings_fts MATCH ?`
	args := []any{query}
	if cikFilter != "" {
		q += " AND cik = ?"
		args = append(args, cikFilter)
	}
	if formFilter != "" {
		q += " AND form_type = ?"
		args = append(args, formFilter)
	}
	q += fmt.Sprintf(" ORDER BY rank LIMIT %d", limit)
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []EdgarFTSHit
	for rows.Next() {
		var h EdgarFTSHit
		if err := rows.Scan(&h.Accession, &h.CIK, &h.FormType, &h.FiledAt, &h.Snippet); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func nullableFloat(v float64) any {
	if v == 0 {
		return nil
	}
	return v
}

// Ensure json import is referenced (for future use; keep stable).
var _ = json.RawMessage(nil)
