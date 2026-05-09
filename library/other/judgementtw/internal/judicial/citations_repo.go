// Copyright 2026 wayne-lai. Licensed under Apache-2.0. See LICENSE.

package judicial

import (
	"context"
	"database/sql"

	"judgementtw-pp-cli/internal/extract"
)

// IndexCitations writes a judgment's extracted citations and JID references to
// the local store. Idempotent: re-indexing the same JID overwrites prior data.
func IndexCitations(ctx context.Context, db *sql.DB, jid string, citations []extract.Citation, jidRefs []string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM citations WHERE jid = ?`, jid); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM jid_refs WHERE from_jid = ?`, jid); err != nil {
		return err
	}

	stmt, err := tx.PrepareContext(ctx,
		`INSERT OR IGNORE INTO citations (jid, statute, article) VALUES (?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, c := range citations {
		if _, err := stmt.ExecContext(ctx, jid, c.Statute, c.Article); err != nil {
			return err
		}
	}

	refStmt, err := tx.PrepareContext(ctx,
		`INSERT OR IGNORE INTO jid_refs (from_jid, to_jid) VALUES (?, ?)`)
	if err != nil {
		return err
	}
	defer refStmt.Close()
	for _, ref := range jidRefs {
		if ref == jid {
			continue
		}
		if _, err := refStmt.ExecContext(ctx, jid, ref); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// IndexSentences writes extracted sentence rows for a judgment. Idempotent.
func IndexSentences(ctx context.Context, db *sql.DB, jid string, sentences []extract.Sentence) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM sentences WHERE jid = ?`, jid); err != nil {
		return err
	}
	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO sentences (jid, kind, prison_months, fine_ntd, probation, raw)
		 VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, s := range sentences {
		if _, err := stmt.ExecContext(ctx, jid, string(s.Kind),
			s.PrisonMonths, s.FineNTD, s.Probation, s.Raw); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// CitationCount returns counts of judgments grouped by court for a given
// statute (and optional article — pass 0 to ignore article).
type CitationCount struct {
	Statute string `json:"statute"`
	Article int    `json:"article"`
	Court   string `json:"court"`
	Year    int    `json:"year"`
	Count   int    `json:"count"`
}

// CountByStatute joins citations × judgments to produce per-court counts.
// When article > 0, restricts to a specific article.
func CountByStatute(ctx context.Context, db *sql.DB, statute string, article int) ([]CitationCount, error) {
	q := `
		SELECT c.statute, c.article,
		       SUBSTR(c.jid, 1, 3) AS court,
		       CAST(SUBSTR(c.jid, INSTR(c.jid, ',') + 1,
		           INSTR(SUBSTR(c.jid, INSTR(c.jid, ',') + 1), ',') - 1) AS INTEGER) AS year,
		       COUNT(*) AS n
		FROM citations c
		WHERE c.statute = ?` + articleFilter(article) + `
		GROUP BY c.statute, c.article, court, year
		ORDER BY year DESC, n DESC`
	args := []any{statute}
	if article > 0 {
		args = append(args, article)
	}
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CitationCount
	for rows.Next() {
		var r CitationCount
		if err := rows.Scan(&r.Statute, &r.Article, &r.Court, &r.Year, &r.Count); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func articleFilter(article int) string {
	if article > 0 {
		return ` AND c.article = ?`
	}
	return ``
}

// CitedBy returns JIDs that cite a given JID (reverse-citation lookup).
func CitedBy(ctx context.Context, db *sql.DB, jid string, limit int) ([]string, error) {
	q := `SELECT from_jid FROM jid_refs WHERE to_jid = ? ORDER BY from_jid`
	if limit > 0 {
		q += ` LIMIT ?`
	}
	args := []any{jid}
	if limit > 0 {
		args = append(args, limit)
	}
	rows, err := db.QueryContext(ctx, q, args...)
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

// CitationsOf returns all (statute, article) pairs cited by a given JID.
func CitationsOf(ctx context.Context, db *sql.DB, jid string) ([]extract.Citation, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT statute, article FROM citations WHERE jid = ? ORDER BY statute, article`, jid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []extract.Citation
	for rows.Next() {
		var c extract.Citation
		if err := rows.Scan(&c.Statute, &c.Article); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
