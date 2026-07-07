// Copyright 2026 Mathias Michel and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// Store wraps a SQLite database for TED procurement notices.
type Store struct {
	db *sql.DB
}

// Notice represents a single TED procurement notice stored locally.
type Notice struct {
	ID                 string
	NoticeType         string
	PublicationDate    string
	BuyerName          string
	BuyerCountry       string
	CPVCode            string
	CPVCodesJSON       string
	EstimatedValue     float64
	Currency           string
	WinnerName         string
	WinnerCountry      string
	ContractValue      float64
	ProcedureType      string
	SubmissionDeadline string
	Title              string
	PlaceOfPerformance string
	NoticeURL          string
	PreviousNoticeID   string
	RawData            string
	SyncedAt           string
}

const schema = `
CREATE TABLE IF NOT EXISTS notices (
    id TEXT PRIMARY KEY,
    notice_type TEXT,
    publication_date TEXT,
    buyer_name TEXT,
    buyer_country TEXT,
    cpv_code TEXT,
    cpv_codes_json TEXT,
    estimated_value REAL,
    currency TEXT DEFAULT 'EUR',
    winner_name TEXT,
    winner_country TEXT,
    contract_value REAL,
    procedure_type TEXT,
    submission_deadline TEXT,
    title TEXT,
    place_of_performance TEXT,
    notice_url TEXT,
    previous_notice_id TEXT,
    raw_data TEXT,
    synced_at TEXT
);

CREATE VIRTUAL TABLE IF NOT EXISTS notices_fts USING fts5(
    title, buyer_name, winner_name,
    content=notices, content_rowid=rowid
);

CREATE TABLE IF NOT EXISTS sync_state (
    key TEXT PRIMARY KEY,
    value TEXT
);
`

// Open opens (or creates) the SQLite database at the given path.
func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("init schema: %w", err)
	}
	// Add previous_notice_id column to existing databases that predate this field.
	_, _ = db.Exec(`ALTER TABLE notices ADD COLUMN previous_notice_id TEXT`)
	return &Store{db: db}, nil
}

// DB returns the underlying *sql.DB for raw queries.
func (s *Store) DB() *sql.DB { return s.db }

// Close closes the database connection.
func (s *Store) Close() error { return s.db.Close() }

// UpsertNotice inserts or replaces a notice record.
func (s *Store) UpsertNotice(n Notice) error {
	// PATCH: Delete the old external-content FTS row before reindexing an updated notice.
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin upsert notice %s: %w", n.ID, err)
	}
	defer tx.Rollback()

	var oldRowID int64
	var oldTitle, oldBuyerName, oldWinnerName sql.NullString
	err = tx.QueryRow(`SELECT rowid, title, buyer_name, winner_name FROM notices WHERE id=?`, n.ID).
		Scan(&oldRowID, &oldTitle, &oldBuyerName, &oldWinnerName)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("load existing notice %s: %w", n.ID, err)
	}
	if err == nil {
		_, err = tx.Exec(`INSERT INTO notices_fts(notices_fts, rowid, title, buyer_name, winner_name)
			VALUES('delete', ?, ?, ?, ?)`, oldRowID, oldTitle, oldBuyerName, oldWinnerName)
		if err != nil {
			return fmt.Errorf("delete notice %s from fts: %w", n.ID, err)
		}
	}

	const q = `INSERT INTO notices
		(id, notice_type, publication_date, buyer_name, buyer_country,
		 cpv_code, cpv_codes_json, estimated_value, currency,
		 winner_name, winner_country, contract_value, procedure_type,
		 submission_deadline, title, place_of_performance, notice_url,
		 previous_notice_id, raw_data, synced_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET
			notice_type=excluded.notice_type,
			publication_date=excluded.publication_date,
			buyer_name=excluded.buyer_name,
			buyer_country=excluded.buyer_country,
			cpv_code=excluded.cpv_code,
			cpv_codes_json=excluded.cpv_codes_json,
			estimated_value=excluded.estimated_value,
			currency=excluded.currency,
			winner_name=excluded.winner_name,
			winner_country=excluded.winner_country,
			contract_value=excluded.contract_value,
			procedure_type=excluded.procedure_type,
			submission_deadline=excluded.submission_deadline,
			title=excluded.title,
			place_of_performance=excluded.place_of_performance,
			notice_url=excluded.notice_url,
			previous_notice_id=excluded.previous_notice_id,
			raw_data=excluded.raw_data,
			synced_at=excluded.synced_at`
	_, err = tx.Exec(q,
		n.ID, n.NoticeType, n.PublicationDate, n.BuyerName, n.BuyerCountry,
		n.CPVCode, n.CPVCodesJSON, n.EstimatedValue, n.Currency,
		n.WinnerName, n.WinnerCountry, n.ContractValue, n.ProcedureType,
		n.SubmissionDeadline, n.Title, n.PlaceOfPerformance, n.NoticeURL,
		n.PreviousNoticeID, n.RawData, n.SyncedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert notice %s: %w", n.ID, err)
	}
	_, err = tx.Exec(`INSERT INTO notices_fts(rowid, title, buyer_name, winner_name)
		SELECT rowid, title, buyer_name, winner_name FROM notices WHERE id=?`, n.ID)
	if err != nil {
		return fmt.Errorf("insert notice %s into fts: %w", n.ID, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit upsert notice %s: %w", n.ID, err)
	}
	return nil
}

// LastSyncDate returns the stored last-sync date for a query key, or "" if not set.
func (s *Store) LastSyncDate(queryKey string) (string, error) {
	var val string
	err := s.db.QueryRow(`SELECT value FROM sync_state WHERE key=?`, queryKey).Scan(&val)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return val, err
}

// SetLastSyncDate stores the last-sync date for a query key.
func (s *Store) SetLastSyncDate(queryKey, date string) error {
	_, err := s.db.Exec(`INSERT OR REPLACE INTO sync_state (key, value) VALUES (?, ?)`, queryKey, date)
	return err
}

// Search performs an FTS5 full-text search and returns matching notices.
func (s *Store) Search(query string, limit int) ([]Notice, error) {
	if limit <= 0 {
		limit = 20
	}
	const q = `SELECT n.id, n.notice_type, n.publication_date, n.buyer_name, n.buyer_country,
		n.cpv_code, n.cpv_codes_json, n.estimated_value, n.currency,
		n.winner_name, n.winner_country, n.contract_value, n.procedure_type,
		n.submission_deadline, n.title, n.place_of_performance, n.notice_url,
		n.previous_notice_id, n.raw_data, n.synced_at
	FROM notices_fts f
	JOIN notices n ON n.rowid = f.rowid
	WHERE notices_fts MATCH ?
	LIMIT ?`
	rows, err := s.db.Query(q, query, limit)
	if err != nil {
		return nil, fmt.Errorf("fts search: %w", err)
	}
	defer rows.Close()
	return scanNotices(rows)
}

// Count returns the total number of synced notices.
func (s *Store) Count() (int64, error) {
	var n int64
	err := s.db.QueryRow(`SELECT COUNT(*) FROM notices`).Scan(&n)
	return n, err
}

func scanNotices(rows *sql.Rows) ([]Notice, error) {
	var out []Notice
	for rows.Next() {
		var n Notice
		if err := rows.Scan(
			&n.ID, &n.NoticeType, &n.PublicationDate, &n.BuyerName, &n.BuyerCountry,
			&n.CPVCode, &n.CPVCodesJSON, &n.EstimatedValue, &n.Currency,
			&n.WinnerName, &n.WinnerCountry, &n.ContractValue, &n.ProcedureType,
			&n.SubmissionDeadline, &n.Title, &n.PlaceOfPerformance, &n.NoticeURL,
			&n.PreviousNoticeID, &n.RawData, &n.SyncedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// ResolveMultilingual resolves a multilingual TED API field value to a plain string,
// preferring English and falling back through German, French, Dutch.
func ResolveMultilingual(v interface{}) string {
	switch t := v.(type) {
	case map[string]interface{}:
		// PATCH(amend-2026-06-09: handle scalar-string language values) — TED v3
		// returns some multilingual fields as map[lang] -> []string (e.g.
		// title-lot) and others as map[lang] -> string (e.g. title-proc). The
		// original code only unwrapped the array shape, so scalar fields like
		// title-proc silently resolved to "" and the title fallback chain skipped
		// them. Handle both shapes per language.
		for _, lang := range []string{"eng", "deu", "fra", "nld"} {
			switch lv := t[lang].(type) {
			case []interface{}:
				if len(lv) > 0 {
					if s, ok := lv[0].(string); ok && s != "" {
						return s
					}
				}
			case string:
				if lv != "" {
					return lv
				}
			}
		}
	case []interface{}:
		if len(t) > 0 {
			if s, ok := t[0].(string); ok {
				return s
			}
		}
	case string:
		return t
	}
	return ""
}

// CPVFromList extracts the primary CPV code and a JSON array of all codes from a raw field value.
func CPVFromList(v interface{}) (primary string, allJSON string) {
	var codes []string
	switch t := v.(type) {
	case []interface{}:
		for _, item := range t {
			if s, ok := item.(string); ok {
				codes = append(codes, s)
			}
		}
	case string:
		codes = []string{t}
	}
	if len(codes) > 0 {
		primary = codes[0]
	}
	b, _ := json.Marshal(codes)
	allJSON = string(b)
	return
}
