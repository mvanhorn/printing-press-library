// Copyright 2026 rushyant-m. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"database/sql"
	"fmt"
	"time"
)

// EnsureBSETables creates the hand-built tables the novel BSE commands rely
// on: the portfolio holdings list, the parsed concall paragraph store (plus
// its FTS5 index), and the results beat/miss outcomes ledger. Idempotent —
// every command that touches these tables calls it right after Open so a
// fresh database self-heals without a separate migration step.
func (s *Store) EnsureBSETables() error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	stmts := []string{
		`CREATE TABLE IF NOT EXISTS holdings (
			scrip_code  TEXT PRIMARY KEY,
			scrip_name  TEXT,
			sector      TEXT,
			last_synced DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS concall_chunks (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			filing_id   TEXT,
			scrip_code  TEXT,
			quarter     TEXT,
			paragraph_n INTEGER,
			body        TEXT,
			filed_at    DATETIME,
			UNIQUE(filing_id, paragraph_n)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_concall_chunks_scrip ON concall_chunks(scrip_code)`,
		`CREATE INDEX IF NOT EXISTS idx_concall_chunks_filing ON concall_chunks(filing_id)`,
		// External-content FTS5 table mirroring concall_chunks, kept in sync by
		// the triggers below. content_rowid='id' ties each FTS row to the
		// content table's integer PK, so the trigger-issued 'delete' commands
		// remove rows by rowid — the only delete shape modernc.org/sqlite's
		// FTS5 honors reliably. Writers touch only concall_chunks; the FTS
		// index maintains itself.
		`CREATE VIRTUAL TABLE IF NOT EXISTS concall_chunks_fts USING fts5(
			body,
			scrip_code UNINDEXED,
			quarter UNINDEXED,
			filing_id UNINDEXED,
			content='concall_chunks',
			content_rowid='id',
			tokenize='porter unicode61'
		)`,
		`CREATE TRIGGER IF NOT EXISTS concall_chunks_ai AFTER INSERT ON concall_chunks BEGIN
			INSERT INTO concall_chunks_fts(rowid, body, scrip_code, quarter, filing_id)
			VALUES (new.id, new.body, new.scrip_code, new.quarter, new.filing_id);
		END`,
		`CREATE TRIGGER IF NOT EXISTS concall_chunks_ad AFTER DELETE ON concall_chunks BEGIN
			INSERT INTO concall_chunks_fts(concall_chunks_fts, rowid, body, scrip_code, quarter, filing_id)
			VALUES ('delete', old.id, old.body, old.scrip_code, old.quarter, old.filing_id);
		END`,
		`CREATE TRIGGER IF NOT EXISTS concall_chunks_au AFTER UPDATE ON concall_chunks BEGIN
			INSERT INTO concall_chunks_fts(concall_chunks_fts, rowid, body, scrip_code, quarter, filing_id)
			VALUES ('delete', old.id, old.body, old.scrip_code, old.quarter, old.filing_id);
			INSERT INTO concall_chunks_fts(rowid, body, scrip_code, quarter, filing_id)
			VALUES (new.id, new.body, new.scrip_code, new.quarter, new.filing_id);
		END`,
		`CREATE TABLE IF NOT EXISTS results_outcomes (
			filing_id  TEXT,
			scrip_code TEXT,
			quarter    TEXT,
			revenue    TEXT,
			ebitda     TEXT,
			pat        TEXT,
			beat_miss  TEXT,
			UNIQUE(filing_id)
		)`,
	}
	for _, q := range stmts {
		if _, err := s.db.Exec(q); err != nil {
			return fmt.Errorf("ensure BSE tables: %w", err)
		}
	}
	return nil
}

// UpsertHolding inserts or updates a single holding. Pass an empty sector or
// name to leave the existing value untouched on update.
func (s *Store) UpsertHolding(scripCode, name, sector string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.Exec(
		`INSERT INTO holdings (scrip_code, scrip_name, sector)
		 VALUES (?, ?, ?)
		 ON CONFLICT(scrip_code) DO UPDATE SET
		   scrip_name = COALESCE(NULLIF(excluded.scrip_name, ''), holdings.scrip_name),
		   sector     = COALESCE(NULLIF(excluded.sector, ''), holdings.sector)`,
		scripCode, name, sector,
	)
	return err
}

// RemoveHolding deletes a holding by scrip code and reports whether a row was
// actually removed (false means the scrip was not in the portfolio).
func (s *Store) RemoveHolding(scripCode string) (bool, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	res, err := s.db.Exec(`DELETE FROM holdings WHERE scrip_code = ?`, scripCode)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// Holding is one row of the portfolio table.
type Holding struct {
	ScripCode  string     `json:"scrip_code"`
	ScripName  string     `json:"scrip_name"`
	Sector     string     `json:"sector"`
	LastSynced *time.Time `json:"last_synced,omitempty"`
}

// ListHoldings returns every holding ordered by scrip code.
func (s *Store) ListHoldings() ([]Holding, error) {
	rows, err := s.db.Query(`SELECT scrip_code, scrip_name, sector, last_synced FROM holdings ORDER BY scrip_code`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Holding
	for rows.Next() {
		var h Holding
		var name, sector sql.NullString
		var ls sql.NullTime
		if err := rows.Scan(&h.ScripCode, &name, &sector, &ls); err != nil {
			return nil, err
		}
		h.ScripName = name.String
		h.Sector = sector.String
		if ls.Valid {
			t := ls.Time
			h.LastSynced = &t
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// CountHoldings returns the number of rows in the holdings table.
func (s *Store) CountHoldings() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM holdings`).Scan(&n)
	return n, err
}

// TouchHoldingSync stamps last_synced on a holding after a sync pass.
func (s *Store) TouchHoldingSync(scripCode string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.Exec(`UPDATE holdings SET last_synced = ? WHERE scrip_code = ?`, time.Now(), scripCode)
	return err
}

// ReplaceConcallChunks atomically swaps in the parsed transcript paragraphs for
// one filing: it clears any prior chunks for the filing and inserts the new set
// inside a single transaction, so a crash can never leave the filing with its
// old rows deleted but no replacement written. paragraphs are 0-indexed by
// position; filedAt is the announcement's NEWS_DT. Only the concall_chunks
// content table is written here — the AFTER INSERT/DELETE triggers keep the
// external-content concall_chunks_fts index in lockstep, including the rowid
// 'delete' that the prior column-based FTS DELETE could silently no-op.
func (s *Store) ReplaceConcallChunks(filingID, scripCode, quarter string, paragraphs []string, filedAt time.Time) (int, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM concall_chunks WHERE filing_id = ?`, filingID); err != nil {
		return 0, fmt.Errorf("delete concall chunks: %w", err)
	}

	var n int
	for i, p := range paragraphs {
		if _, err := tx.Exec(
			`INSERT INTO concall_chunks (filing_id, scrip_code, quarter, paragraph_n, body, filed_at)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			filingID, scripCode, quarter, i, p, filedAt,
		); err != nil {
			return 0, fmt.Errorf("insert concall chunk: %w", err)
		}
		n++
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return n, nil
}

// UpsertResultsOutcome records a parsed results outcome. A filing is keyed by
// filing_id; re-running outcomes replaces the prior row for that filing via
// INSERT OR REPLACE honoring UNIQUE(filing_id).
func (s *Store) UpsertResultsOutcome(filingID, scripCode, quarter, revenue, ebitda, pat, beatMiss string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO results_outcomes (filing_id, scrip_code, quarter, revenue, ebitda, pat, beat_miss)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		filingID, scripCode, quarter, revenue, ebitda, pat, beatMiss,
	)
	return err
}
