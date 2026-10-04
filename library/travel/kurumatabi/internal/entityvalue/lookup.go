// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.

// Package entityvalue is the shared lower-level entity binding resolver.
// Store transactions and the learning layer use exactly the same transforms
// and source precedence without importing one another.
package entityvalue

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

type Queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

var nonSlugRune = regexp.MustCompile(`[^a-z0-9-]+`)

func IsComputedKind(kind string) bool {
	switch kind {
	case "lowercase", "uppercase", "kebab-case", "capitalize-first", "slug":
		return true
	}
	return false
}

func Compute(kind, canonical string) (string, bool) {
	switch kind {
	case "lowercase":
		return strings.ToLower(canonical), true
	case "uppercase":
		return strings.ToUpper(canonical), true
	case "kebab-case":
		return strings.ReplaceAll(strings.ToLower(canonical), " ", "-"), true
	case "capitalize-first":
		runes := []rune(canonical)
		if len(runes) > 0 {
			runes[0] = unicode.ToUpper(runes[0])
			for i := 1; i < len(runes); i++ {
				runes[i] = unicode.ToLower(runes[i])
			}
		}
		return string(runes), true
	case "slug":
		return nonSlugRune.ReplaceAllString(strings.ReplaceAll(strings.ToLower(canonical), " ", "-"), ""), true
	}
	return "", false
}

func Lookup(ctx context.Context, db Queryer, kind, canonical string) (string, bool, error) {
	if v, ok := Compute(kind, canonical); ok {
		return v, true, nil
	}
	if db == nil {
		return "", false, errors.New("entityvalue.Lookup: db is nil")
	}
	const query = `SELECT value FROM entity_lookups
 WHERE kind = ? AND LOWER(canonical) = LOWER(?)
 ORDER BY CASE source WHEN 'taught' THEN 0 WHEN 'inferred' THEN 1
 WHEN 'synced' THEN 2 WHEN 'seeded' THEN 3 ELSE 4 END ASC, created_at ASC LIMIT 1`
	var value string
	err := db.QueryRowContext(ctx, query, kind, canonical).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("entityvalue.Lookup query: %w", err)
	}
	return value, true, nil
}
