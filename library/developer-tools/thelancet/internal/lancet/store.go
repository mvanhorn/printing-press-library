package lancet

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/thelancet/internal/cliutil"
)

// EnsureSchema creates the Lancet analytics tables if they do not exist. The
// tables are additive to the generated store's schema and are prefixed
// "lancet_" to avoid collision.
func EnsureSchema(ctx context.Context, db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS lancet_works (
			work_id      TEXT PRIMARY KEY,
			doi          TEXT,
			title        TEXT,
			journal_issn TEXT,
			journal_name TEXT,
			pub_year     INTEGER,
			pub_date     TEXT,
			cited_count  INTEGER,
			is_oa        INTEGER,
			topic        TEXT,
			synced_at    TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS lancet_authorships (
			work_id     TEXT,
			author_id   TEXT,
			author_name TEXT,
			seq         INTEGER,
			PRIMARY KEY (work_id, author_id)
		)`,
		`CREATE TABLE IF NOT EXISTS lancet_affiliations (
			work_id          TEXT,
			author_id        TEXT,
			institution_id   TEXT,
			institution_name TEXT,
			country          TEXT,
			UNIQUE(work_id, author_id, institution_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_lancet_works_issn ON lancet_works(journal_issn)`,
		`CREATE INDEX IF NOT EXISTS idx_lancet_works_year ON lancet_works(pub_year)`,
		`CREATE INDEX IF NOT EXISTS idx_lancet_auth_author ON lancet_authorships(author_id)`,
		`CREATE INDEX IF NOT EXISTS idx_lancet_affil_inst ON lancet_affiliations(institution_name)`,
		`CREATE INDEX IF NOT EXISTS idx_lancet_affil_work_author ON lancet_affiliations(work_id, author_id)`,
	}
	for _, s := range stmts {
		if _, err := db.ExecContext(ctx, s); err != nil {
			return fmt.Errorf("lancet schema: %w", err)
		}
	}
	return nil
}

// WorkCount returns the number of works in the local store, optionally scoped
// to a journal ISSN (empty = all).
func WorkCount(ctx context.Context, db *sql.DB, issn string) (int, error) {
	q := `SELECT COUNT(*) FROM lancet_works`
	args := []any{}
	if issn != "" {
		q += ` WHERE journal_issn = ?`
		args = append(args, issn)
	}
	var n int
	err := db.QueryRowContext(ctx, q, args...).Scan(&n)
	return n, err
}

// upsertWork stores one decomposed work and its authorships/affiliations within
// a transaction.
func upsertWork(ctx context.Context, tx *sql.Tx, w decodedWork, issn, journalName, syncedAt string) error {
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO lancet_works
			(work_id, doi, title, journal_issn, journal_name, pub_year, pub_date, cited_count, is_oa, topic, synced_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(work_id) DO UPDATE SET
			cited_count=excluded.cited_count, topic=excluded.topic, synced_at=excluded.synced_at`,
		w.ID, w.DOI, w.Title, issn, journalName, w.Year, w.Date, w.Cited, boolToInt(w.IsOA), w.Topic, syncedAt,
	); err != nil {
		return err
	}
	// Replace authorship/affiliation rows for this work to stay idempotent.
	if _, err := tx.ExecContext(ctx, `DELETE FROM lancet_authorships WHERE work_id = ?`, w.ID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM lancet_affiliations WHERE work_id = ?`, w.ID); err != nil {
		return err
	}
	for i, a := range w.Authors {
		if a.ID == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT OR IGNORE INTO lancet_authorships (work_id, author_id, author_name, seq)
			VALUES (?,?,?,?)`, w.ID, a.ID, a.Name, i); err != nil {
			return err
		}
		for _, inst := range a.Institutions {
			if inst.Name == "" {
				continue
			}
			if _, err := tx.ExecContext(ctx, `
				INSERT OR IGNORE INTO lancet_affiliations (work_id, author_id, institution_id, institution_name, country)
				VALUES (?,?,?,?,?)`, w.ID, a.ID, inst.ID, inst.Name, inst.Country); err != nil {
				return err
			}
		}
	}
	return nil
}

// StoreWorks persists a batch of decoded works for one journal.
func StoreWorks(ctx context.Context, db *sql.DB, works []decodedWork, issn, journalName string) (int, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	syncedAt := time.Now().UTC().Format(time.RFC3339)
	journalName = cliutil.CleanText(journalName)
	n := 0
	for _, w := range works {
		if w.ID == "" {
			continue
		}
		if err := upsertWork(ctx, tx, w, issn, journalName, syncedAt); err != nil {
			return n, err
		}
		n++
	}
	if err := tx.Commit(); err != nil {
		return n, err
	}
	return n, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}