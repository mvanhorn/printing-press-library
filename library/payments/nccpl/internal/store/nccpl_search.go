package store

// Text search over the stored observations.
//
// Every read path this CLI ships is keyed by resource first: coverage, panel,
// universe, leverage and risk-changes all require the caller to already know
// which of the 24 resources to look in. Nothing answers the question a research
// caller starts from -- "where does this symbol appear at all?" -- short of
// exporting all 24 panels and grepping them.
//
// The generated resources/resources_fts mirror cannot answer it either: every
// NCCPL data endpoint is a dated POST, so no resource is syncable through the
// generic mirror and nothing is ever written to it. nccpl_obs is where the data
// lands, so that is what this searches.
//
// Matching is a literal, case-insensitive substring test rather than a
// tokenized index, because the queries this exists to serve are identifiers and
// dates -- HUBC, 2026-09-01, a UIN prefix -- which a word tokenizer either
// splits or stems into something that no longer matches.
//
// A hit is always reported with the (resource, date, row_key) that identifies it
// and the observed_at vintage recorded when the value was first seen. A search
// reports what is stored and nothing else: a session that was never fetched
// stays absent from the results rather than being interpolated, and observed_at
// is copied out of the row rather than reconstructed.

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// defaultNCCPLSearchLimit caps a search that did not ask for a limit.
const defaultNCCPLSearchLimit = 50

// NCCPLHit is one matching observation, carrying the columns that identify it.
type NCCPLHit struct {
	Resource   string `json:"resource"`
	Date       string `json:"date"`
	Key        string `json:"key"`
	Payload    string `json:"payload"`
	ObservedAt string `json:"observed_at"`
	// MatchedKey reports whether the query matched the row key, so a caller
	// can tell an identity hit from a hit somewhere inside the record.
	MatchedKey bool `json:"matched_key"`
}

// NCCPLSearchOptions narrows a SearchNCCPLObservations call.
type NCCPLSearchOptions struct {
	// Resources limits the search to these resource names. Empty means every
	// stored resource.
	Resources []string
	// From and To bound the settlement date inclusively. Empty means unbounded.
	From string
	To   string
	// Limit caps the number of hits returned. Zero or less means 50.
	Limit int
}

// SearchNCCPLObservations returns stored observations whose row key or payload
// contains query, newest settlement date first.
//
// Results are capped at opts.Limit; a caller handed exactly Limit hits should
// treat the result as truncated rather than complete.
func SearchNCCPLObservations(ctx context.Context, s *Store, query string, opts NCCPLSearchOptions) ([]NCCPLHit, error) {
	if s == nil || s.DB() == nil {
		return nil, fmt.Errorf("nccpl search: nil store")
	}
	needle := strings.ToLower(strings.TrimSpace(query))
	if needle == "" {
		return nil, fmt.Errorf("nccpl search: empty query")
	}
	limit := opts.Limit
	if limit <= 0 {
		limit = defaultNCCPLSearchLimit
	}

	pattern := "%" + escapeNCCPLLikeLiteral(needle) + "%"
	q := `SELECT resource, date, row_key, payload, observed_at
FROM nccpl_obs
WHERE (LOWER(row_key) LIKE ? ESCAPE '\' OR LOWER(payload) LIKE ? ESCAPE '\')`
	args := []any{pattern, pattern}

	if clause, resourceArgs := nccplResourceInClause(opts.Resources); clause != "" {
		q += clause
		args = append(args, resourceArgs...)
	}
	if opts.From != "" {
		q += ` AND date >= ?`
		args = append(args, opts.From)
	}
	if opts.To != "" {
		q += ` AND date <= ?`
		args = append(args, opts.To)
	}
	q += ` ORDER BY date DESC, resource, row_key LIMIT ?`
	args = append(args, limit)

	rows, err := s.DB().QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("nccpl search query: %w", err)
	}
	out := make([]NCCPLHit, 0)
	for rows.Next() {
		var (
			hit      NCCPLHit
			payload  sql.NullString
			observed sql.NullString
		)
		if err := rows.Scan(&hit.Resource, &hit.Date, &hit.Key, &payload, &observed); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("nccpl search scan: %w", err)
		}
		hit.Payload = payload.String
		hit.ObservedAt = observed.String
		hit.MatchedKey = strings.Contains(strings.ToLower(hit.Key), needle)
		out = append(out, hit)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("nccpl search iterate: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("nccpl search close: %w", err)
	}
	return out, nil
}

// nccplResourceInClause builds the optional `AND resource IN (...)` filter.
// Blank and duplicate entries are dropped so a caller-supplied list cannot
// widen the filter or emit a bare placeholder.
func nccplResourceInClause(resources []string) (string, []any) {
	seen := make(map[string]bool, len(resources))
	args := make([]any, 0, len(resources))
	placeholders := make([]string, 0, len(resources))
	for _, name := range resources {
		name = strings.TrimSpace(name)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		args = append(args, name)
		placeholders = append(placeholders, "?")
	}
	if len(args) == 0 {
		return "", nil
	}
	return " AND resource IN (" + strings.Join(placeholders, ", ") + ")", args
}

// escapeNCCPLLikeLiteral neutralizes the LIKE wildcards so a query containing
// % or _ matches the literal text the caller typed. Paired with ESCAPE '\'.
func escapeNCCPLLikeLiteral(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}
