// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package recipes

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Default confidence floor for an inferred recipe. Mirrors the
// search_learnings teach-side floor in store.UpsertLearning (also 2)
// so recipe hits and direct teaches sort comparably when recall
// merges them.
const DefaultConfidence = 2

// Strategy values for Recipe.Strategy. Kept as const strings to keep
// the column a stable schema for tests and the future LLM-readable
// envelope.
const (
	// StrategySubstitute names a fully-deterministic resource ID:
	// substituting the entity into the resource_template yields
	// exactly one candidate which Apply verifies by direct lookup
	// in the resources table.
	StrategySubstitute = "substitute"

	// StrategySubstituteThenSearchPrefix names a partially-
	// deterministic resource ID: the resource_template ends with "*"
	// and Apply does a LIKE prefix search in the resources table
	// after substitution. Polymarket slugs (which carry an
	// unpredictable trailing numeric segment) use this strategy.
	StrategySubstituteThenSearchPrefix = "substitute-then-search-prefix"
)

// Source values for Recipe.Source. "inferred" rows come from
// Extract; "taught" rows come from the teach-recipe CLI command.
const (
	SourceInferred = "inferred"
	SourceTaught   = "taught"
)

// Recipe is one row of the search_recipes table. It encodes a
// template-with-typed-entity-slot pattern: given an input query that
// matches QueryTemplate (modulo one entity slot), the engine
// substitutes the live query's entity via lookups.Lookup(EntityKind,
// ...) into ResourceTemplate and returns the result as a candidate
// resource.
//
// Field order matches the table column order so callers writing
// Recipe literals can mirror the schema. JSON tags exist because the
// teach-recipe CLI and a future diagnostics surface (`recipes list
// --json`) emit Recipe envelopes.
type Recipe struct {
	ID               int64      `json:"id"`
	QueryTemplate    string     `json:"query_template"`
	ResourceTemplate string     `json:"resource_template"`
	ResourceType     string     `json:"resource_type"`
	Venue            string     `json:"venue,omitempty"`
	Strategy         string     `json:"strategy"`
	EntityKind       string     `json:"entity_kind"`
	Confidence       int        `json:"confidence"`
	Source           string     `json:"source"`
	CreatedAt        time.Time  `json:"created_at"`
	LastObservedAt   *time.Time `json:"last_observed_at,omitempty"`
	ExampleQuery     string     `json:"example_query,omitempty"`
	ExampleResource  string     `json:"example_resource,omitempty"`
}

// ListFilter narrows List. Zero values are unfiltered. EntityKind,
// Strategy, Source, ResourceType, Venue match by equality; Limit
// defaults to 200 when zero.
type ListFilter struct {
	EntityKind   string
	Strategy     string
	Source       string
	ResourceType string
	Venue        string
	Limit        int
}

// ForgetFilter selects rows for Forget. At least one of EntityKind,
// QueryTemplate, ResourceTemplate, or All must be set; otherwise
// Forget returns an error rather than wiping the whole table.
type ForgetFilter struct {
	EntityKind       string
	QueryTemplate    string
	ResourceTemplate string
	All              bool
}

// Upsert inserts a recipe row, or — when (query_template,
// resource_template, strategy) already exists — bumps confidence by
// 1 and refreshes last_observed_at. Returns the row ID and a bool
// indicating whether the row was newly inserted.
//
// Idempotency is built on the unique index idx_recipes_unique. This
// is the contract Extract relies on: a re-Extract pass over the same
// learnings produces zero new rows but bumps confidence on the
// matching existing rows, so a recipe that re-derives from steady
// taught data climbs in rank over time without spawning duplicates.
func Upsert(db *sql.DB, r Recipe) (int64, bool, error) {
	if db == nil {
		return 0, false, errors.New("recipes.Upsert: db is nil")
	}
	if strings.TrimSpace(r.QueryTemplate) == "" {
		return 0, false, errors.New("recipes.Upsert: query_template is required")
	}
	if strings.TrimSpace(r.ResourceTemplate) == "" {
		return 0, false, errors.New("recipes.Upsert: resource_template is required")
	}
	if strings.TrimSpace(r.Strategy) == "" {
		return 0, false, errors.New("recipes.Upsert: strategy is required")
	}
	if strings.TrimSpace(r.EntityKind) == "" {
		return 0, false, errors.New("recipes.Upsert: entity_kind is required")
	}
	if strings.TrimSpace(r.ResourceType) == "" {
		return 0, false, errors.New("recipes.Upsert: resource_type is required")
	}
	if r.Confidence <= 0 {
		r.Confidence = DefaultConfidence
	}
	if strings.TrimSpace(r.Source) == "" {
		r.Source = SourceInferred
	}

	now := time.Now().UTC()

	tx, err := db.Begin()
	if err != nil {
		return 0, false, fmt.Errorf("recipes.Upsert begin: %w", err)
	}
	defer tx.Rollback()

	var existingID int64
	err = tx.QueryRow(
		`SELECT id FROM search_recipes
		 WHERE query_template = ? AND resource_template = ? AND strategy = ?`,
		r.QueryTemplate, r.ResourceTemplate, r.Strategy,
	).Scan(&existingID)
	if err == nil {
		if _, err := tx.Exec(
			`UPDATE search_recipes
			 SET confidence = confidence + 1, last_observed_at = ?
			 WHERE id = ?`,
			now, existingID,
		); err != nil {
			return 0, false, fmt.Errorf("recipes.Upsert bump confidence: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return 0, false, err
		}
		return existingID, false, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, false, fmt.Errorf("recipes.Upsert lookup: %w", err)
	}

	var venue, exampleQuery, exampleResource any
	if r.Venue != "" {
		venue = r.Venue
	}
	if r.ExampleQuery != "" {
		exampleQuery = r.ExampleQuery
	}
	if r.ExampleResource != "" {
		exampleResource = r.ExampleResource
	}

	res, err := tx.Exec(
		`INSERT INTO search_recipes
		 (query_template, resource_template, resource_type, venue, strategy, entity_kind,
		  confidence, source, created_at, last_observed_at, example_query, example_resource)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.QueryTemplate, r.ResourceTemplate, r.ResourceType, venue, r.Strategy, r.EntityKind,
		r.Confidence, r.Source, now, now, exampleQuery, exampleResource,
	)
	if err != nil {
		return 0, false, fmt.Errorf("recipes.Upsert insert: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, false, fmt.Errorf("recipes.Upsert last id: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, false, err
	}
	return id, true, nil
}

// List returns recipes filtered by f, ordered by last_observed_at
// DESC, then confidence DESC, then id DESC. A nil db returns an
// error.
func List(db *sql.DB, f ListFilter) ([]Recipe, error) {
	if db == nil {
		return nil, errors.New("recipes.List: db is nil")
	}
	clauses := []string{}
	args := []any{}
	if f.EntityKind != "" {
		clauses = append(clauses, "entity_kind = ?")
		args = append(args, f.EntityKind)
	}
	if f.Strategy != "" {
		clauses = append(clauses, "strategy = ?")
		args = append(args, f.Strategy)
	}
	if f.Source != "" {
		clauses = append(clauses, "source = ?")
		args = append(args, f.Source)
	}
	if f.ResourceType != "" {
		clauses = append(clauses, "resource_type = ?")
		args = append(args, f.ResourceType)
	}
	if f.Venue != "" {
		clauses = append(clauses, "venue = ?")
		args = append(args, f.Venue)
	}
	where := ""
	if len(clauses) > 0 {
		where = "WHERE " + strings.Join(clauses, " AND ")
	}
	limit := f.Limit
	if limit <= 0 {
		limit = 200
	}
	args = append(args, limit)

	q := fmt.Sprintf(`SELECT id, query_template, resource_template, resource_type, COALESCE(venue, ''),
		strategy, entity_kind, confidence, source, created_at, last_observed_at,
		COALESCE(example_query, ''), COALESCE(example_resource, '')
		FROM search_recipes %s
		ORDER BY last_observed_at DESC, confidence DESC, id DESC
		LIMIT ?`, where)

	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("recipes.List query: %w", err)
	}
	defer rows.Close()

	out := make([]Recipe, 0)
	for rows.Next() {
		var r Recipe
		var lastObs sql.NullTime
		if err := rows.Scan(&r.ID, &r.QueryTemplate, &r.ResourceTemplate, &r.ResourceType, &r.Venue,
			&r.Strategy, &r.EntityKind, &r.Confidence, &r.Source, &r.CreatedAt, &lastObs,
			&r.ExampleQuery, &r.ExampleResource); err != nil {
			return nil, fmt.Errorf("recipes.List scan: %w", err)
		}
		if lastObs.Valid {
			t := lastObs.Time
			r.LastObservedAt = &t
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("recipes.List rows: %w", err)
	}
	return out, nil
}

// Forget deletes recipes matching f and returns the count removed.
// Pass All=true to delete every row (the kill-switch for the manual
// CLI surface); otherwise at least one of EntityKind, QueryTemplate,
// or ResourceTemplate must be set, mirroring store.ForgetLearnings.
func Forget(db *sql.DB, f ForgetFilter) (int64, error) {
	if db == nil {
		return 0, errors.New("recipes.Forget: db is nil")
	}
	if !f.All && f.EntityKind == "" && f.QueryTemplate == "" && f.ResourceTemplate == "" {
		return 0, errors.New("recipes.Forget: pass --all, --entity-kind, --query-template, or --resource-template")
	}
	clauses := []string{}
	args := []any{}
	if f.EntityKind != "" {
		clauses = append(clauses, "entity_kind = ?")
		args = append(args, f.EntityKind)
	}
	if f.QueryTemplate != "" {
		clauses = append(clauses, "query_template = ?")
		args = append(args, f.QueryTemplate)
	}
	if f.ResourceTemplate != "" {
		clauses = append(clauses, "resource_template = ?")
		args = append(args, f.ResourceTemplate)
	}
	var q string
	if len(clauses) == 0 {
		// f.All == true path: wipe everything.
		q = "DELETE FROM search_recipes"
	} else {
		q = "DELETE FROM search_recipes WHERE " + strings.Join(clauses, " AND ")
	}
	res, err := db.Exec(q, args...)
	if err != nil {
		return 0, fmt.Errorf("recipes.Forget: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}
