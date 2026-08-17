// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package lookups

import (
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// LookupRow is the canonical seed-row shape used by both seeds/ files
// and by the v4->v5 migration's batch INSERT. It exists as a plain
// struct (not as an exported alias for a database/sql row) so the
// seeds subpackage can declare a []LookupRow at compile time without
// pulling in database/sql.
//
// Source is the provenance string written into entity_lookups.source.
// Seeds default to "seeded"; runtime teaches default to "taught";
// recipe-extraction-derived rows use "inferred".
type LookupRow struct {
	Kind      string
	Canonical string
	Value     string
	Source    string
}

// computedKinds enumerates the kinds whose Lookup result is derived
// purely from the canonical input by string transform, with no
// reference to the database. They live here as a closed set because
// adding one is a source-code change (the transform itself must be
// implemented in computedLookup below) and because recipes that name
// a computed kind must be portable across CLIs without depending on
// the DB content.
//
// Order does not matter; this is a membership set, not a priority
// list. Using a map literal lets the predicate run in O(1) without
// allocating per call.
var computedKinds = map[string]struct{}{
	"lowercase":        {},
	"uppercase":        {},
	"kebab-case":       {},
	"capitalize-first": {},
	"slug":             {},
}

// nonSlugRune matches every character that is NOT a lowercase ASCII
// letter, digit, or hyphen. Used by the slug computed kind to strip
// punctuation after kebab-casing. Pre-compiled as a package var to
// avoid recompiling per Lookup call.
var nonSlugRune = regexp.MustCompile(`[^a-z0-9-]+`)

// IsComputedKind reports whether kind is resolved by an in-package
// string transform instead of by a row in entity_lookups. Callers
// (e.g. the recipe engine) use this to skip the DB round-trip on
// hot paths and to validate recipe templates at write time.
func IsComputedKind(kind string) bool {
	_, ok := computedKinds[kind]
	return ok
}

// computedLookup applies the named computed-kind transform to
// canonical and returns the result with found=true. Unknown kinds
// return ("", false) — callers must guard with IsComputedKind, but
// the explicit fallthrough keeps the function safe to call on its
// own.
//
// Why these specific transforms: lowercase/uppercase are the
// universal slug/ticker forms. kebab-case is the Polymarket slug
// shape minus the trailing numeric ID. capitalize-first is the
// generic "title case the entity for display" transform that recipe
// templates use when the resource shape preserves capitalization.
// slug strips non-alphanumeric-or-hyphen characters after
// kebab-casing — useful for canonical names with apostrophes,
// accented letters, or punctuation that the upstream resource
// scrubbed (e.g., "Côte d'Ivoire" -> "cte-divoire" if the source
// stripped diacritics, "cote-d-ivoire" via a different rule).
//
// For canonical names with diacritics the slug transform deliberately
// does NOT normalize Unicode — that is a recipe-substitution failure
// the inference engine should detect by trying alternate kinds rather
// than burying inside one transform.
func computedLookup(kind, canonical string) (string, bool) {
	switch kind {
	case "lowercase":
		return strings.ToLower(canonical), true
	case "uppercase":
		return strings.ToUpper(canonical), true
	case "kebab-case":
		return strings.ReplaceAll(strings.ToLower(canonical), " ", "-"), true
	case "capitalize-first":
		return capitalizeFirst(canonical), true
	case "slug":
		kebab := strings.ReplaceAll(strings.ToLower(canonical), " ", "-")
		return nonSlugRune.ReplaceAllString(kebab, ""), true
	}
	return "", false
}

// capitalizeFirst returns canonical with the first rune uppercased
// and the rest lowercased. Empty input returns empty output.
//
// Why a hand-rolled implementation: strings.Title is deprecated, and
// golang.org/x/text/cases would pull a dependency just for one case
// fold. The rune-iteration shape handles non-ASCII first letters
// (e.g. "ÉTATS" -> "États") correctly without depending on locale.
func capitalizeFirst(canonical string) string {
	if canonical == "" {
		return ""
	}
	runes := []rune(canonical)
	runes[0] = unicode.ToUpper(runes[0])
	for i := 1; i < len(runes); i++ {
		runes[i] = unicode.ToLower(runes[i])
	}
	return string(runes)
}

// Lookup returns the value mapped to the (kind, canonical) pair, or
// ("", false) if no such mapping exists. Computed kinds short-circuit
// the DB entirely (so a closed-DB handle still works on computed
// kinds; tests rely on this).
//
// Canonical comparison is case-insensitive on the canonical side
// (the table is queried via LOWER(canonical) = LOWER(?)) so a recipe
// captured against "Portugal" still resolves a query carrying
// "portugal" or "PORTUGAL". Values are returned verbatim from the
// table — the caller decides whether case matters for substitution.
//
// When multiple rows match the same (kind, canonical) pair (the PK
// is (kind, canonical, value), so different values are allowed),
// Lookup returns the first one ordered by source priority
// ('taught' > 'inferred' > 'seeded') then by created_at ascending.
// For all-values access use LookupAll.
func Lookup(db *sql.DB, kind, canonical string) (string, bool, error) {
	if v, ok := computedLookup(kind, canonical); ok {
		return v, true, nil
	}
	if db == nil {
		return "", false, errors.New("lookups.Lookup: db is nil")
	}

	// Source priority: user-taught rows outrank auto-inferred rows
	// outrank seeded rows. The CASE expression maps source strings
	// to a priority integer (lower wins via ASC sort); unknown
	// source values get the lowest priority so a future provenance
	// tag we forgot to enumerate here still returns something rather
	// than nothing.
	const q = `
		SELECT value
		FROM entity_lookups
		WHERE kind = ? AND LOWER(canonical) = LOWER(?)
		ORDER BY CASE source
		  WHEN 'taught' THEN 0
		  WHEN 'inferred' THEN 1
		  WHEN 'seeded' THEN 2
		  ELSE 3
		END ASC, created_at ASC
		LIMIT 1
	`
	var value string
	err := db.QueryRow(q, kind, canonical).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("lookups.Lookup query: %w", err)
	}
	return value, true, nil
}

// LookupAll returns every value mapped to the (kind, canonical) pair,
// deduplicated and ordered by source priority then created_at. For a
// computed kind, the result has exactly one element (the computed
// transform). Returns an empty slice if no rows match — never returns
// nil except on error.
//
// Why both Lookup and LookupAll exist: the recipe substitution path
// wants one value (Lookup). Diagnostics and the `learnings list
// --lookups` surface want to display every alias (LookupAll). The
// two share a query shape but have different SELECT shapes, so
// keeping them as separate functions avoids a sentinel-bool parameter
// that would obscure the call sites.
func LookupAll(db *sql.DB, kind, canonical string) ([]string, error) {
	if v, ok := computedLookup(kind, canonical); ok {
		return []string{v}, nil
	}
	if db == nil {
		return nil, errors.New("lookups.LookupAll: db is nil")
	}
	const q = `
		SELECT DISTINCT value
		FROM entity_lookups
		WHERE kind = ? AND LOWER(canonical) = LOWER(?)
		ORDER BY CASE source
		  WHEN 'taught' THEN 0
		  WHEN 'inferred' THEN 1
		  WHEN 'seeded' THEN 2
		  ELSE 3
		END ASC, created_at ASC
	`
	rows, err := db.Query(q, kind, canonical)
	if err != nil {
		return nil, fmt.Errorf("lookups.LookupAll query: %w", err)
	}
	defer rows.Close()

	values := make([]string, 0)
	seen := make(map[string]struct{})
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, fmt.Errorf("lookups.LookupAll scan: %w", err)
		}
		if _, dup := seen[v]; dup {
			continue
		}
		seen[v] = struct{}{}
		values = append(values, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("lookups.LookupAll rows: %w", err)
	}
	return values, nil
}

// Upsert inserts or no-ops a single (kind, canonical, value) row with
// the given source. Idempotent: the table's PRIMARY KEY is
// (kind, canonical, value), so a re-insert of the same triple is a
// silent no-op via INSERT OR IGNORE. Different values for the same
// (kind, canonical) DO create separate rows — that is intentional, so
// "United States" can map to both "US" (alpha-2) and "USA" (alpha-3)
// under different kinds, and a single kind can carry multiple aliases
// where appropriate.
//
// Computed kinds (lowercase, kebab-case, etc.) cannot be Upsert'd —
// they have no table representation. Attempting to Upsert one returns
// an error rather than silently writing a row that will never be
// read.
//
// Source defaults to "taught" when the caller passes an empty string,
// matching the teach-lookup CLI's default. Pass "seeded" for the
// migration-time batch insert and "inferred" for rows the recipe
// extraction engine derives.
func Upsert(db *sql.DB, kind, canonical, value, source string) error {
	if IsComputedKind(kind) {
		return fmt.Errorf("lookups.Upsert: %q is a computed kind, not table-backed", kind)
	}
	if db == nil {
		return errors.New("lookups.Upsert: db is nil")
	}
	if strings.TrimSpace(kind) == "" {
		return errors.New("lookups.Upsert: kind is required")
	}
	if strings.TrimSpace(canonical) == "" {
		return errors.New("lookups.Upsert: canonical is required")
	}
	if strings.TrimSpace(value) == "" {
		return errors.New("lookups.Upsert: value is required")
	}
	if source == "" {
		source = "taught"
	}
	_, err := db.Exec(`
		INSERT OR IGNORE INTO entity_lookups (kind, canonical, value, source)
		VALUES (?, ?, ?, ?)
	`, kind, canonical, value, source)
	if err != nil {
		return fmt.Errorf("lookups.Upsert exec: %w", err)
	}
	return nil
}

// SeedBatch inserts every row in seeds via INSERT OR IGNORE inside a
// caller-supplied transaction. Returns the count of rows actually
// inserted (i.e., not skipped by the OR IGNORE conflict resolution).
//
// This is the function the v4->v5 schema migration calls. It takes a
// *sql.Tx (not a *sql.DB) so the migration can run the table CREATE
// and the seed inserts in one atomic unit — a partial seed would
// otherwise leave the DB in a half-populated state that subsequent
// re-Opens cannot distinguish from a deliberately-pruned table.
//
// Why a single prepared statement: SQLite hits a soft floor around
// 1500 batched bound values per statement. With ~500 seed rows and 4
// columns each, a multi-row VALUES batch would be near the limit and
// risky to extend. A prepared statement re-executed per row is
// negligibly slower for this volume and trivially correct.
func SeedBatch(tx *sql.Tx, seeds []LookupRow) (int, error) {
	if tx == nil {
		return 0, errors.New("lookups.SeedBatch: tx is nil")
	}
	stmt, err := tx.Prepare(`
		INSERT OR IGNORE INTO entity_lookups (kind, canonical, value, source)
		VALUES (?, ?, ?, ?)
	`)
	if err != nil {
		return 0, fmt.Errorf("prepare seed insert: %w", err)
	}
	defer stmt.Close()

	var inserted int
	for _, r := range seeds {
		source := r.Source
		if source == "" {
			source = "seeded"
		}
		res, err := stmt.Exec(r.Kind, r.Canonical, r.Value, source)
		if err != nil {
			return inserted, fmt.Errorf("insert seed (%s, %s, %s): %w", r.Kind, r.Canonical, r.Value, err)
		}
		n, err := res.RowsAffected()
		if err == nil {
			inserted += int(n)
		}
	}
	return inserted, nil
}
