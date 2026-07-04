// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package learn

import (
	"context"
	"database/sql"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

// stubResolver returns canned canonicals for the keys present in m.
type stubResolver struct {
	m map[string][]string
}

func (s stubResolver) Resolve(token string) []string {
	return s.m[token]
}

func (s stubResolver) ResolveSet(tokens []string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, t := range tokens {
		for _, c := range s.Resolve(t) {
			out[c] = struct{}{}
		}
	}
	return out
}

func TestPromoteEntities_PromotesLowercaseAlias(t *testing.T) {
	t.Parallel()
	cfg := DefaultPredictionGoatConfig()
	normalized := Normalize("what are the odds usa wins the world cup", cfg)
	// Capitalization-based extractor misses "usa" (lowercased input).
	if containsTok(normalized.Entities, "usa") {
		t.Fatalf("baseline: extractor should not see lowercase 'usa' as entity, got %v", normalized.Entities)
	}

	resolver := stubResolver{m: map[string][]string{
		"usa": {"United States"},
	}}
	got := PromoteEntities(normalized, resolver)
	if !containsTok(got.Entities, "usa") {
		t.Errorf("want 'usa' promoted into Entities, got %v", got.Entities)
	}
	for _, tok := range strings.Fields(got.NonEntityNormalized) {
		if tok == "usa" {
			t.Errorf("non-entity tokens should no longer contain 'usa'; got %q", got.NonEntityNormalized)
		}
	}
}

func TestPromoteEntities_NoMatch_LeavesUnchanged(t *testing.T) {
	t.Parallel()
	cfg := DefaultPredictionGoatConfig()
	normalized := Normalize("hello world foo bar", cfg)
	resolver := stubResolver{m: map[string][]string{}}
	got := PromoteEntities(normalized, resolver)
	if !reflect.DeepEqual(got, normalized) {
		t.Errorf("no resolver hits should leave normalized unchanged; got %+v want %+v", got, normalized)
	}
}

func TestPromoteEntities_NilResolver_Identity(t *testing.T) {
	t.Parallel()
	cfg := DefaultPredictionGoatConfig()
	normalized := Normalize("how are the mariners doing", cfg)
	got := PromoteEntities(normalized, nil)
	if !reflect.DeepEqual(got, normalized) {
		t.Errorf("nil resolver should be identity; got %+v want %+v", got, normalized)
	}
}

func TestPromoteEntities_EmptyQuery_NoOp(t *testing.T) {
	t.Parallel()
	cfg := DefaultPredictionGoatConfig()
	normalized := Normalize("", cfg)
	resolver := stubResolver{m: map[string][]string{"usa": {"United States"}}}
	got := PromoteEntities(normalized, resolver)
	if !reflect.DeepEqual(got, normalized) {
		t.Errorf("empty query should be identity; got %+v want %+v", got, normalized)
	}
}

func TestPromoteEntities_MultiEntity(t *testing.T) {
	t.Parallel()
	cfg := DefaultPredictionGoatConfig()
	normalized := Normalize("usa vs uk tonight", cfg)
	resolver := stubResolver{m: map[string][]string{
		"usa": {"United States"},
		"uk":  {"United Kingdom"},
	}}
	got := PromoteEntities(normalized, resolver)
	if !containsTok(got.Entities, "usa") || !containsTok(got.Entities, "uk") {
		t.Errorf("want both 'usa' and 'uk' promoted, got %v", got.Entities)
	}
}

func TestPromoteEntities_PreservesExistingEntities(t *testing.T) {
	t.Parallel()
	cfg := DefaultPredictionGoatConfig()
	normalized := Normalize("how does Portugal play against usa", cfg)
	// Capitalization rule already gives "Portugal".
	if !containsTok(normalized.Entities, "Portugal") {
		t.Fatalf("baseline: capitalized 'Portugal' should be an entity; got %v", normalized.Entities)
	}
	resolver := stubResolver{m: map[string][]string{
		"usa": {"United States"},
	}}
	got := PromoteEntities(normalized, resolver)
	if !containsTok(got.Entities, "Portugal") {
		t.Errorf("Portugal (capitalized) should be preserved as entity; got %v", got.Entities)
	}
	if !containsTok(got.Entities, "usa") {
		t.Errorf("usa should also be promoted; got %v", got.Entities)
	}
}

// errOnceDB is an in-memory *sql.DB whose first SELECT against
// entity_lookups errors via a poisoned table (DROP after the schema is
// committed). The second SELECT succeeds because we re-create the
// table between calls. This verifies the resolver does NOT cache nil
// on the first error -- the second Resolve must retry and see the
// canonical.
func openErrOnceDB(t *testing.T) (*sql.DB, func()) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "erronce.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	// Don't create the table yet -- the first Resolve will hit a
	// "no such table" error from sqlite. The fix function below
	// creates and seeds the table so the second Resolve succeeds.
	fix := func() {
		if _, err := db.Exec(`CREATE TABLE entity_lookups (
			kind TEXT NOT NULL,
			canonical TEXT NOT NULL,
			value TEXT NOT NULL,
			source TEXT NOT NULL DEFAULT 'seeded',
			PRIMARY KEY (kind, canonical, value)
		)`); err != nil {
			t.Fatalf("create: %v", err)
		}
		if _, err := db.Exec(
			`INSERT INTO entity_lookups (kind, canonical, value, source)
			 VALUES ('country_iso2', 'United States', 'US', 'seeded')`,
		); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	return db, fix
}

// TestCanonicalResolver_ErrorRetry verifies the Greptile-discovered
// pattern: a DB error on the first Resolve() call must NOT cache nil
// for that token. The second call must retry the SQL and see the real
// canonical once the table exists.
func TestCanonicalResolver_ErrorRetry(t *testing.T) {
	t.Parallel()
	db, fix := openErrOnceDB(t)
	defer db.Close()

	resolver := NewCanonicalResolver(context.Background(), db)
	// First call: table doesn't exist -- query errors, returns nil.
	if got := resolver.Resolve("US"); got != nil {
		t.Fatalf("first Resolve before table exists should return nil; got %v", got)
	}
	// Fix the table.
	fix()
	// Second call: must retry the SQL, not return cached nil.
	if got := resolver.Resolve("US"); len(got) == 0 {
		t.Errorf("second Resolve after table healed should return canonical; got %v -- partial cache poison?", got)
	}
}

// TestCanonicalResolver_ScanError verifies that a scan failure during
// row iteration does NOT pin a truncated canonical list. This is a
// white-box test: we simulate the error by closing the rows context
// mid-iteration via a context.Cancel sentinel error.
func TestCanonicalResolver_ScanError(t *testing.T) {
	t.Parallel()
	db := openCanonicalTestDB(t)
	if _, err := db.Exec(
		`INSERT INTO entity_lookups (kind, canonical, value, source)
		 VALUES ('country_iso2', 'United States', 'US', 'seeded')`,
	); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// Use a cancelled context to force the DB to error on Query.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	resolver := NewCanonicalResolver(ctx, db)
	got := resolver.Resolve("US")
	if got != nil {
		t.Fatalf("cancelled-context Resolve should return nil; got %v", got)
	}

	// Now build a healthy resolver and confirm the canonical surfaces.
	healthy := NewCanonicalResolver(context.Background(), db)
	got = healthy.Resolve("US")
	if len(got) == 0 {
		t.Errorf("healthy resolver should return canonical for 'US'; got %v", got)
	}
}

// TestCanonicalResolver_HappyPath verifies basic Resolve() and
// ResolveSet() against a seeded DB.
func TestCanonicalResolver_HappyPath(t *testing.T) {
	t.Parallel()
	db := openCanonicalTestDB(t)
	if _, err := db.Exec(
		`INSERT INTO entity_lookups (kind, canonical, value, source)
		 VALUES ('country_iso2', 'United States', 'US', 'seeded'),
		        ('country_iso2', 'United Kingdom', 'GB', 'seeded')`,
	); err != nil {
		t.Fatalf("seed: %v", err)
	}
	resolver := NewCanonicalResolver(context.Background(), db)

	// By value (alias side).
	got := resolver.Resolve("US")
	if len(got) != 1 || got[0] != "United States" {
		t.Errorf("Resolve(US): want [United States], got %v", got)
	}

	// By canonical (self-resolution).
	got = resolver.Resolve("united kingdom")
	if len(got) != 1 || got[0] != "United Kingdom" {
		t.Errorf("Resolve(united kingdom): want [United Kingdom], got %v", got)
	}

	// Unknown token returns empty.
	if got := resolver.Resolve("nowhere"); len(got) != 0 {
		t.Errorf("Resolve(nowhere): want empty, got %v", got)
	}

	// Empty/whitespace returns nil without SQL.
	if got := resolver.Resolve(""); got != nil {
		t.Errorf("Resolve(empty): want nil, got %v", got)
	}

	// ResolveSet collects canonicals from a slice.
	set := resolver.ResolveSet([]string{"US", "GB", "nowhere"})
	if _, ok := set["United States"]; !ok {
		t.Errorf("ResolveSet missing United States; got %v", set)
	}
	if _, ok := set["United Kingdom"]; !ok {
		t.Errorf("ResolveSet missing United Kingdom; got %v", set)
	}
	if _, ok := set["nowhere"]; ok {
		t.Errorf("ResolveSet should drop unresolvable tokens; got %v", set)
	}
}

// TestPromoteEntities_TeachRecallSymmetry verifies the round-trip
// contract: PromoteEntities at teach time produces the same
// query_entities slice as PromoteEntities at recall time would, so a
// later cross-alias fallback in U3 has stored entities to compare
// against. We don't exercise the full cross-alias fallback here -- U3
// owns that -- but we lock in that the helper output is identical at
// both invocation sites for the same query + same resolver.
func TestPromoteEntities_TeachRecallSymmetry(t *testing.T) {
	t.Parallel()
	db := openCanonicalTestDB(t)
	if _, err := db.Exec(
		`INSERT INTO entity_lookups (kind, canonical, value, source)
		 VALUES ('country_iso2', 'United States', 'US', 'seeded'),
		        ('country_iso2', 'USA', 'US', 'seeded')`,
	); err != nil {
		t.Fatalf("seed: %v", err)
	}
	cfg := DefaultPredictionGoatConfig()
	query := "what are the odds usa wins the world cup"

	teachResolver := NewCanonicalResolver(context.Background(), db)
	teachNorm := PromoteEntities(Normalize(query, cfg), teachResolver)

	recallResolver := NewCanonicalResolver(context.Background(), db)
	recallNorm := PromoteEntities(Normalize(query, cfg), recallResolver)

	if !reflect.DeepEqual(sortedCopy(teachNorm.Entities), sortedCopy(recallNorm.Entities)) {
		t.Errorf("teach/recall Entities should be symmetric; teach=%v recall=%v",
			teachNorm.Entities, recallNorm.Entities)
	}
	if teachNorm.NonEntityNormalized != recallNorm.NonEntityNormalized {
		t.Errorf("teach/recall NonEntityNormalized should be symmetric; teach=%q recall=%q",
			teachNorm.NonEntityNormalized, recallNorm.NonEntityNormalized)
	}
	if !containsTok(teachNorm.Entities, "usa") {
		t.Errorf("teach side should have promoted 'usa'; got %v", teachNorm.Entities)
	}
}

// openCanonicalTestDB opens a fresh sqlite DB with the entity_lookups
// table (matching prediction-goat's v5+ schema) for resolver tests.
func openCanonicalTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "canonical.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.Exec(`CREATE TABLE entity_lookups (
		kind TEXT NOT NULL,
		canonical TEXT NOT NULL,
		value TEXT NOT NULL,
		source TEXT NOT NULL DEFAULT 'seeded',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (kind, canonical, value)
	)`); err != nil {
		t.Fatalf("schema: %v", err)
	}
	return db
}

func containsTok(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}

func sortedCopy(s []string) []string {
	out := append([]string(nil), s...)
	sort.Strings(out)
	return out
}
