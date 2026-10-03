// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/kurumatabi/internal/entityvalue"
)

type derivedPatternFamily struct{ template, resourceType, venue string }
type derivedPattern struct {
	id                               int64
	resourceTemplate, strategy, kind string
}
type retainedTeaching struct {
	query, resource string
	entities        []string
}

// reconcileDerivedPatterns preserves an existing inference only when the
// retained family still demonstrates its actual binding across distinct
// entities. It examines all relevant rows, not Extract's newest-50 window.
// Reads, example repair and unsupported-rule deletion share the undo tx.
func reconcileDerivedPatterns(ctx context.Context, tx *sql.Tx, f derivedPatternFamily) error {
	rows, err := tx.QueryContext(ctx, `SELECT id,resource_template,strategy,entity_kind
 FROM search_patterns WHERE source='inferred' AND query_template=?
 AND resource_type=? AND COALESCE(venue,'')=?`, f.template, f.resourceType, f.venue)
	if err != nil {
		return err
	}
	var patterns []derivedPattern
	for rows.Next() {
		var p derivedPattern
		if err := rows.Scan(&p.id, &p.resourceTemplate, &p.strategy, &p.kind); err != nil {
			rows.Close()
			return err
		}
		patterns = append(patterns, p)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if len(patterns) == 0 {
		return nil
	}

	rows, err = tx.QueryContext(ctx, `SELECT query_pattern,COALESCE(query_entities,'[]'),resource_id
 FROM search_learnings WHERE COALESCE(resource_type,'')=? AND COALESCE(venue,'')=?
 AND action='boost'
 AND source IN ('taught','inferred-followup','inferred-reach','inferred-pair')
 ORDER BY last_observed_at DESC,id DESC`, f.resourceType, f.venue)
	if err != nil {
		return err
	}
	var members []retainedTeaching
	for rows.Next() {
		var t retainedTeaching
		var raw string
		if err := rows.Scan(&t.query, &raw, &t.resource); err != nil {
			rows.Close()
			return err
		}
		if json.Unmarshal([]byte(raw), &t.entities) != nil || len(t.entities) == 0 {
			continue
		}
		if retainedFamilyTemplate(t.query, t.entities) == f.template {
			members = append(members, t)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}

	for _, p := range patterns {
		example, supported, err := retainedPatternSupport(ctx, tx, p, members)
		if err != nil {
			return err
		}
		if supported {
			// Example provenance may have named the forgotten contributor.
			_, err = tx.ExecContext(ctx, `UPDATE search_patterns SET example_query=?,example_resource=?
   WHERE id=? AND source='inferred'`, example.query, example.resource, p.id)
		} else {
			_, err = tx.ExecContext(ctx, `DELETE FROM search_patterns WHERE id=? AND source='inferred'`, p.id)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func retainedFamilyTemplate(query string, entities []string) string {
	skip := map[string]bool{}
	for _, e := range entities {
		skip[strings.ToLower(strings.TrimSpace(e))] = true
	}
	var tokens []string
	for _, t := range strings.Fields(query) {
		if !skip[strings.ToLower(t)] {
			tokens = append(tokens, t)
		}
	}
	if len(tokens) == 0 {
		return ""
	}
	tokens = append(tokens, "{entity}")
	sort.Strings(tokens)
	return strings.Join(tokens, " ")
}

// retainedPatternSupport counts only usable rows compatible with this existing
// rule. Stale or unrelated bindings cannot veto other independent evidence.
// The returned example belongs to that same compatible cohort.
func retainedPatternSupport(ctx context.Context, tx *sql.Tx, p derivedPattern, members []retainedTeaching) (retainedTeaching, bool, error) {
	var example retainedTeaching
	if len(members) < 2 {
		return example, false, nil
	}
	slot := "{entity:" + p.kind + "}"
	if !strings.Contains(p.resourceTemplate, slot) {
		slot = "{entity}"
	}
	if !strings.Contains(p.resourceTemplate, slot) {
		return example, false, nil
	}
	if p.strategy != "substitute" && p.strategy != "substitute-then-search-prefix" {
		return example, false, nil
	}
	entities, values := map[string]bool{}, map[string]bool{}
	haveExample := false
	for _, m := range members {
		// Match Extract's single-slot guard; incompatible rows contribute nothing.
		if len(m.entities) != 1 {
			continue
		}
		entity := strings.TrimSpace(m.entities[0])
		if entity == "" {
			continue
		}
		value, found, err := entityvalue.Lookup(ctx, tx, p.kind, entity)
		if err != nil {
			return example, false, fmt.Errorf("verify retained entity binding: %w", err)
		}
		if !found || value == "" {
			continue
		}
		candidate := strings.ReplaceAll(p.resourceTemplate, slot, value)
		if strings.Contains(candidate, "{entity") {
			continue
		}
		switch p.strategy {
		case "substitute":
			if candidate != m.resource {
				continue
			}
		case "substitute-then-search-prefix":
			if !strings.HasSuffix(candidate, "*") || !strings.HasPrefix(m.resource, strings.TrimSuffix(candidate, "*")) {
				continue
			}
		}
		entities[strings.ToLower(entity)] = true
		values[value] = true
		if !haveExample {
			example = m
			haveExample = true
		}
	}
	return example, len(entities) >= 2 && len(values) >= 2, nil
}
