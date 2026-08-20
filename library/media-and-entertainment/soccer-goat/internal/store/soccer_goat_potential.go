// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored extension (not generated). Adds the `potential` table that
// backs FIFA/FC potential ratings, keyed by the EA player id (which the EA
// drop-api and the sofifa-derived dataset share exactly) with a normalized-name
// fallback. Kept in its own file per the "extended store schema" durability
// rule so `generate --force` never clobbers it.

package store

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
)

// PotentialRow is one player's current/potential pair with provenance.
type PotentialRow struct {
	EAID           int
	Name           string
	NameNormalized string
	Overall        int
	Potential      int
	Source         string // e.g. "dataset:sofifa-2025" or "live:fifacm"
	CapturedAt     string // RFC3339; may be empty for bundled dataset rows
}

var potentialNonAlnum = regexp.MustCompile(`[^a-z0-9 ]+`)
var potentialSpaces = regexp.MustCompile(`\s+`)

// latinFold maps the accented Latin runes common in football names to ASCII so
// a report name ("Kylian Mbappé") and a dataset name ("Kylian Mbappé Lottin")
// normalize to comparable forms. This is a lightweight stand-in for full
// Unicode NFKD folding — chosen to avoid a golang.org/x/text dependency, since
// the primary join is the exact EA id and normalized-name matching is only the
// fallback for players the id lookup missed.
var latinFold = strings.NewReplacer(
	"à", "a", "á", "a", "â", "a", "ã", "a", "ä", "a", "å", "a", "ā", "a", "ă", "a", "ą", "a",
	"è", "e", "é", "e", "ê", "e", "ë", "e", "ē", "e", "ĕ", "e", "ę", "e", "ě", "e",
	"ì", "i", "í", "i", "î", "i", "ï", "i", "ī", "i", "į", "i", "ı", "i",
	"ò", "o", "ó", "o", "ô", "o", "õ", "o", "ö", "o", "ø", "o", "ō", "o", "ő", "o",
	"ù", "u", "ú", "u", "û", "u", "ü", "u", "ū", "u", "ů", "u", "ű", "u",
	"ñ", "n", "ń", "n", "ň", "n", "ç", "c", "ć", "c", "č", "c", "š", "s", "ś", "s", "ş", "s",
	"ž", "z", "ź", "z", "ż", "z", "ý", "y", "ÿ", "y", "ğ", "g", "ł", "l", "đ", "d", "ð", "d", "þ", "th", "ß", "ss", "æ", "ae", "œ", "oe",
)

// NormalizePotentialName lowercases, folds diacritics, and collapses
// punctuation/whitespace. It must match the normalization used when the bundled
// dataset's name_normalized column was built.
func NormalizePotentialName(s string) string {
	folded := latinFold.Replace(strings.ToLower(s))
	folded = potentialNonAlnum.ReplaceAllString(folded, " ")
	return strings.TrimSpace(potentialSpaces.ReplaceAllString(folded, " "))
}

// ensurePotentialTable lazily creates the potential table. Safe to call on
// every access; CREATE TABLE IF NOT EXISTS is a no-op once it exists.
func (s *Store) ensurePotentialTable(ctx context.Context) error {
	const ddl = `
CREATE TABLE IF NOT EXISTS potential (
	ea_id           INTEGER PRIMARY KEY,
	name            TEXT NOT NULL DEFAULT '',
	name_normalized TEXT NOT NULL DEFAULT '',
	overall         INTEGER NOT NULL DEFAULT 0,
	potential       INTEGER NOT NULL DEFAULT 0,
	source          TEXT NOT NULL DEFAULT '',
	captured_at     TEXT NOT NULL DEFAULT ''
);`
	if _, err := s.DB().ExecContext(ctx, ddl); err != nil {
		return fmt.Errorf("create potential table: %w", err)
	}
	if _, err := s.DB().ExecContext(ctx,
		`CREATE INDEX IF NOT EXISTS idx_potential_name ON potential(name_normalized);`); err != nil {
		return fmt.Errorf("create potential name index: %w", err)
	}
	return nil
}

// UpsertPotential inserts or replaces one potential row keyed by ea_id.
func (s *Store) UpsertPotential(ctx context.Context, row PotentialRow) error {
	if err := s.ensurePotentialTable(ctx); err != nil {
		return err
	}
	return s.upsertPotentialTx(ctx, s.DB(), row)
}

type potentialExecer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func (s *Store) upsertPotentialTx(ctx context.Context, ex potentialExecer, row PotentialRow) error {
	norm := row.NameNormalized
	if norm == "" {
		norm = NormalizePotentialName(row.Name)
	}
	_, err := ex.ExecContext(ctx, `
INSERT INTO potential (ea_id, name, name_normalized, overall, potential, source, captured_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(ea_id) DO UPDATE SET
	name=excluded.name,
	name_normalized=excluded.name_normalized,
	overall=excluded.overall,
	potential=excluded.potential,
	source=excluded.source,
	captured_at=excluded.captured_at
`, row.EAID, row.Name, norm, row.Overall, row.Potential, row.Source, row.CapturedAt)
	if err != nil {
		return fmt.Errorf("upsert potential ea_id=%d: %w", row.EAID, err)
	}
	return nil
}

// UpsertPotentialBatch loads many rows in a single transaction (used by sync).
func (s *Store) UpsertPotentialBatch(ctx context.Context, rows []PotentialRow) (int, error) {
	if err := s.ensurePotentialTable(ctx); err != nil {
		return 0, err
	}
	tx, err := s.DB().BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin potential batch: %w", err)
	}
	n := 0
	for _, row := range rows {
		if row.EAID <= 0 || row.Potential <= 0 {
			continue
		}
		if err := s.upsertPotentialTx(ctx, tx, row); err != nil {
			_ = tx.Rollback()
			return n, err
		}
		n++
	}
	if err := tx.Commit(); err != nil {
		return n, fmt.Errorf("commit potential batch: %w", err)
	}
	return n, nil
}

// LookupPotential resolves a potential row by EA id first, then by normalized
// name. Returns ok=false on a miss (never a zero-value row posing as data).
func (s *Store) LookupPotential(ctx context.Context, eaID int, name string) (PotentialRow, bool, error) {
	if err := s.ensurePotentialTable(ctx); err != nil {
		return PotentialRow{}, false, err
	}
	if eaID > 0 {
		if row, ok, err := s.scanPotential(ctx,
			`SELECT ea_id, name, name_normalized, overall, potential, source, captured_at FROM potential WHERE ea_id = ?`,
			eaID); err != nil || ok {
			return row, ok, err
		}
	}
	nn := NormalizePotentialName(name)
	if nn == "" {
		return PotentialRow{}, false, nil
	}
	return s.scanPotential(ctx,
		`SELECT ea_id, name, name_normalized, overall, potential, source, captured_at FROM potential WHERE name_normalized = ? LIMIT 1`,
		nn)
}

func (s *Store) scanPotential(ctx context.Context, query string, arg any) (PotentialRow, bool, error) {
	var (
		eaID              sql.NullInt64
		name, normd, src  sql.NullString
		overall, pot      sql.NullInt64
		capturedAt        sql.NullString
	)
	err := s.DB().QueryRowContext(ctx, query, arg).Scan(&eaID, &name, &normd, &overall, &pot, &src, &capturedAt)
	if err == sql.ErrNoRows {
		return PotentialRow{}, false, nil
	}
	if err != nil {
		return PotentialRow{}, false, fmt.Errorf("lookup potential: %w", err)
	}
	return PotentialRow{
		EAID:           int(eaID.Int64),
		Name:           name.String,
		NameNormalized: normd.String,
		Overall:        int(overall.Int64),
		Potential:      int(pot.Int64),
		Source:         src.String,
		CapturedAt:     capturedAt.String,
	}, true, nil
}

// PotentialCount returns how many rows the table holds (0 when unsynced).
func (s *Store) PotentialCount(ctx context.Context) (int, error) {
	if err := s.ensurePotentialTable(ctx); err != nil {
		return 0, err
	}
	var n int
	if err := s.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM potential`).Scan(&n); err != nil {
		return 0, fmt.Errorf("count potential: %w", err)
	}
	return n, nil
}
