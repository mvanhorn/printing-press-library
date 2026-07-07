// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

// recall_canonical_test.go covers cross-alias canonical resolution at
// recall time. Ported from ESPN PR #851's recall_canonical_test.go and
// adapted for prediction-goat's seeded canonicals (country ISO codes).
//
// PATCH(learn-loop-backport U3): part of the ESPN learn-loop cascade
// backport. See docs/plans/2026-05-25-001-feat-prediction-goat-learn-
// loop-backport-plan.md.

package learn

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

// openRecallCanonicalDB opens a fresh test DB with the minimal schema
// needed for recall + canonical resolution. Kept inline so these tests
// don't depend on the higher-level store.Open path (and the seeded
// production lookups it carries) — every scenario seeds exactly the
// entity_lookups rows it needs.
func openRecallCanonicalDB(t *testing.T) *sql.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "canonical.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	for _, q := range []string{
		`CREATE TABLE resources (
			resource_type TEXT NOT NULL,
			id TEXT NOT NULL,
			data JSON NOT NULL,
			synced_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (resource_type, id)
		)`,
		`CREATE TABLE search_learnings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			query_pattern TEXT NOT NULL,
			query_entities TEXT,
			venue TEXT,
			resource_type TEXT,
			resource_id TEXT NOT NULL,
			action TEXT NOT NULL,
			alias_target TEXT,
			source TEXT NOT NULL,
			confidence INTEGER DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_observed_at DATETIME,
			notes TEXT
		)`,
		`CREATE UNIQUE INDEX idx_learn_unique ON search_learnings(query_pattern, resource_id, action)`,
		`CREATE TABLE search_recipes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			query_template TEXT NOT NULL,
			resource_template TEXT NOT NULL,
			resource_type TEXT NOT NULL,
			venue TEXT,
			strategy TEXT NOT NULL,
			entity_kind TEXT NOT NULL,
			confidence INTEGER NOT NULL DEFAULT 2,
			source TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_observed_at DATETIME,
			example_query TEXT,
			example_resource TEXT
		)`,
		`CREATE TABLE entity_lookups (
			kind TEXT NOT NULL,
			canonical TEXT NOT NULL,
			value TEXT NOT NULL,
			source TEXT NOT NULL DEFAULT 'seeded',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (kind, canonical, value)
		)`,
		`CREATE TABLE learning_playbooks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			query_family TEXT NOT NULL UNIQUE,
			playbook_json TEXT,
			notes_text TEXT,
			source TEXT NOT NULL,
			confidence INTEGER NOT NULL DEFAULT 2,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_observed_at DATETIME
		)`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("schema: %v", err)
		}
	}
	return db
}

// seedCanonical inserts (kind, canonical, value) rows into
// entity_lookups. Mirrors how lookups.SeedFromConfig writes seeds.
func seedCanonical(t *testing.T, db *sql.DB, kind, canonical string, values []string) {
	t.Helper()
	for _, v := range values {
		if _, err := db.Exec(
			`INSERT OR IGNORE INTO entity_lookups (kind, canonical, value, source) VALUES (?, ?, ?, ?)`,
			kind, canonical, v, "seeded",
		); err != nil {
			t.Fatalf("seed lookup: %v", err)
		}
	}
}

// seedCanonicalLearning writes a learning row directly via SQL to keep
// these tests decoupled from the (evolving) write path.
func seedCanonicalLearning(t *testing.T, db *sql.DB, pattern, entitiesJSON, resourceID, resourceType string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO search_learnings (
		query_pattern, query_entities, resource_id, resource_type, action, source, confidence
	) VALUES (?, ?, ?, ?, 'boost', 'taught', 2)`,
		pattern, entitiesJSON, resourceID, resourceType,
	); err != nil {
		t.Fatalf("seed learning: %v", err)
	}
}

// TestRecall_CrossAlias_USAToUnitedStates covers R1. A teach written
// under "United States" should fire on a "USA" query when both
// aliases resolve to the same canonical via entity_lookups.
func TestRecall_CrossAlias_USAToUnitedStates(t *testing.T) {
	t.Parallel()
	db := openRecallCanonicalDB(t)
	// Both "United States" and "USA" resolve to the same canonical.
	seedCanonical(t, db, "country", "United States",
		[]string{"United States", "USA", "United-States"})
	// Teach taught with "United States" → market PT-US.
	seedCanonicalLearning(t, db, "cup united states world", `["United States"]`,
		"KXMENWORLDCUP-26-US", "kalshi_markets")

	got, err := Recall(context.Background(), db, "odds USA wins world cup", Opts{})
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	if !got.Found {
		t.Fatalf("want Found=true via cross-alias canonical resolution; got %+v", got)
	}
	if len(got.Results) != 1 {
		t.Fatalf("want 1 result, got %d", len(got.Results))
	}
	if got.Results[0].ResourceID != "KXMENWORLDCUP-26-US" {
		t.Errorf("ResourceID = %q, want KXMENWORLDCUP-26-US", got.Results[0].ResourceID)
	}
}

// TestRecall_CrossAlias_PromotesEntityMatchExact confirms a cross-alias
// match upgrades the per-hit EntityMatch from Mismatch to Exact and
// emits the cross_alias_match warning on that result.
func TestRecall_CrossAlias_PromotesEntityMatchExact(t *testing.T) {
	t.Parallel()
	db := openRecallCanonicalDB(t)
	seedCanonical(t, db, "country", "United States",
		[]string{"United States", "USA"})
	seedCanonicalLearning(t, db, "cup united states world", `["United States"]`,
		"KXMENWORLDCUP-26-US", "kalshi_markets")

	got, err := Recall(context.Background(), db, "odds USA wins world cup", Opts{})
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	if !got.Found || len(got.Results) == 0 {
		t.Fatalf("want a hit, got %+v", got)
	}
	if got.Results[0].EntityMatch != EntityMatchExact {
		t.Errorf("EntityMatch = %q, want %q (cross-alias should promote Mismatch -> Exact)",
			got.Results[0].EntityMatch, EntityMatchExact)
	}
	foundWarning := false
	for _, w := range got.Results[0].Warnings {
		if w == WarningCrossAliasMatch {
			foundWarning = true
			break
		}
	}
	if !foundWarning {
		t.Errorf("want %q warning on cross-alias hit; got %v",
			WarningCrossAliasMatch, got.Results[0].Warnings)
	}
}

// TestRecall_SimilarShapeDifferentEntity_SurfacesWarning covers R2.
// When a stored row has the same structural shape but a different
// canonical, recall should surface a top-level
// similar_shape_different_entity warning naming the alternative
// canonical, NOT the misleading no_learnings_for_query_family.
func TestRecall_SimilarShapeDifferentEntity_SurfacesWarning(t *testing.T) {
	t.Parallel()
	db := openRecallCanonicalDB(t)
	seedCanonical(t, db, "country", "Portugal", []string{"Portugal", "PT"})
	seedCanonical(t, db, "country", "Brazil", []string{"Brazil", "BR"})
	// Teach Portugal -> resource.
	seedCanonicalLearning(t, db, "cup portugal world", `["Portugal"]`,
		"KXMENWORLDCUP-26-PT", "kalshi_markets")

	got, err := Recall(context.Background(), db, "odds Brazil wins world cup", Opts{})
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	if got.Found {
		t.Fatalf("different-entity query should not be Found; got %+v", got)
	}
	want := WarningSimilarShapeDifferentEntity + ":Portugal"
	foundWarning := false
	for _, w := range got.Warnings {
		if w == want {
			foundWarning = true
		}
	}
	if !foundWarning {
		t.Errorf("want warning %q in envelope; got %v", want, got.Warnings)
	}
	for _, w := range got.Warnings {
		if w == TopWarningNoLearningsForQueryFamily {
			t.Errorf("similar-shape warning should suppress %q; got %v",
				TopWarningNoLearningsForQueryFamily, got.Warnings)
		}
	}
}

// TestRecall_SameEntity_NoCanonicalLookup_DropsBelowJMin covers R7's
// case-2 guard (Greptile PR #851 round 3). Stored and query share a
// literal entity but entity_lookups has no row for it. Non-entity
// Jaccard falls between crossAliasMin (0.3) and jMin (0.6). Without
// the case-2 entitySlicesIntersect guard, the row would slip through
// the looser floor.
func TestRecall_SameEntity_NoCanonicalLookup_DropsBelowJMin(t *testing.T) {
	t.Parallel()
	db := openRecallCanonicalDB(t)
	// Intentionally NOT seeding entity_lookups for "Mariners".
	seedCanonicalLearning(t, db, "doing mariners season year", `["Mariners"]`,
		"team-12", "teams")

	got, err := Recall(context.Background(), db,
		"are the Mariners doing well this year overall", Opts{})
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	if got.Found {
		t.Errorf("same literal entity with no entity_lookups should NOT admit a sub-jMin hit; got %+v", got)
	}
	if len(got.Results) > 0 {
		t.Errorf("expected zero Results, got %d (effective jMin downgraded)", len(got.Results))
	}
}

// TestRecall_SameEntity_CanonicalJaccardDoesNotInflateScore covers R7's
// case-1 guard (Greptile PR #851 round 4). Stored and query share a
// literal entity that IS in entity_lookups. canonicalJaccard would
// boost the score to 1.0 because both sides resolve to the same single
// canonical, admitting structurally-disjoint rows. The case-1 guard
// drops same-entity rows before the boost fires.
func TestRecall_SameEntity_CanonicalJaccardDoesNotInflateScore(t *testing.T) {
	t.Parallel()
	db := openRecallCanonicalDB(t)
	seedCanonical(t, db, "country", "Brazil", []string{"Brazil", "BR"})
	// Stored row shares entity (Brazil) and is structurally disjoint
	// from the query (zero non-entity-token overlap). Pre-fix:
	// canonicalJaccard would boost this to score=1.0.
	seedCanonicalLearning(t, db, "brazil match today", `["Brazil"]`,
		"tv-1", "tv")

	got, err := Recall(context.Background(), db,
		"Brazil end of year stats overall", Opts{})
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	if got.Found {
		t.Errorf("structurally disjoint same-entity row must not be admitted via canonical-overlap boost; got %+v", got)
	}
	for _, r := range got.Results {
		if r.ResourceID == "tv-1" {
			t.Errorf("unrelated stored row leaked into Results with score=%v", r.MatchScore)
		}
	}
}

// TestRecall_CaseInsensitive_EntitySlicesIntersect covers R7's case-
// insensitivity requirement. normalized.Entities is lowercased by
// Normalize (in some paths); stored query_entities preserves the
// extractor casing. The guard must compare case-insensitively or the
// same-entity check silently misses.
func TestRecall_CaseInsensitive_EntitySlicesIntersect(t *testing.T) {
	t.Parallel()
	// Direct unit test on the helper for the load-bearing semantics.
	if !entitySlicesIntersect([]string{"portugal"}, []string{"Portugal"}) {
		t.Errorf("entitySlicesIntersect should match case-insensitively: portugal vs Portugal")
	}
	if !entitySlicesIntersect([]string{"PORTUGAL"}, []string{"portugal"}) {
		t.Errorf("entitySlicesIntersect should match case-insensitively: PORTUGAL vs portugal")
	}
	if entitySlicesIntersect([]string{"Portugal"}, []string{"Brazil"}) {
		t.Errorf("entitySlicesIntersect should not match different entities")
	}
	if entitySlicesIntersect([]string{}, []string{"Portugal"}) {
		t.Errorf("entitySlicesIntersect should be false when either side is empty")
	}
	if entitySlicesIntersect([]string{"Portugal"}, []string{}) {
		t.Errorf("entitySlicesIntersect should be false when either side is empty")
	}

	// End-to-end through Recall: case-2 guard fires correctly even
	// though the stored entity is "Portugal" (extractor casing) and
	// the query's promoted entity is "portugal" (lowercased token).
	db := openRecallCanonicalDB(t)
	// No entity_lookups for Portugal => case-2 path.
	seedCanonicalLearning(t, db, "doing portugal year season", `["Portugal"]`,
		"team-pt", "teams")

	got, err := Recall(context.Background(), db,
		"how is Portugal doing well this year overall", Opts{})
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	if got.Found {
		t.Errorf("case-insensitive same-entity guard should drop the row; got %+v", got)
	}
}

// TestRecall_AmbiguousAlias_FiresOnSingleEntityMultiCanonical covers
// the Greptile PR #851 round 2 finding. A query whose SINGLE entity
// resolves to multiple canonicals should trip ambiguous_alias.
// A multi-entity query whose union of canonicals is multi-element but
// each individual entity resolves to one canonical should NOT.
func TestRecall_AmbiguousAlias_FiresOnSingleEntityMultiCanonical(t *testing.T) {
	t.Parallel()
	db := openRecallCanonicalDB(t)
	// Single alias "Cards" resolves to TWO canonicals.
	seedCanonical(t, db, "team_nfl", "Arizona Cardinals", []string{"Cards"})
	seedCanonical(t, db, "team_mlb", "St. Louis Cardinals", []string{"Cards"})

	got, err := Recall(context.Background(), db, "Cards game tonight", Opts{})
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	foundWarning := false
	for _, w := range got.Warnings {
		if w == WarningAmbiguousAlias {
			foundWarning = true
		}
	}
	if !foundWarning {
		t.Errorf("ambiguous_alias must fire for single-entity multi-canonical (got %v)", got.Warnings)
	}
}

func TestRecall_AmbiguousAlias_DoesNotFireOnMultiEntityQuery(t *testing.T) {
	t.Parallel()
	db := openRecallCanonicalDB(t)
	// Two distinct entities, each resolving to exactly one canonical.
	seedCanonical(t, db, "country", "Portugal", []string{"Portugal", "PT"})
	seedCanonical(t, db, "country", "Brazil", []string{"Brazil", "BR"})

	got, err := Recall(context.Background(), db, "Portugal vs Brazil today", Opts{})
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	for _, w := range got.Warnings {
		if w == WarningAmbiguousAlias {
			t.Errorf("ambiguous_alias should NOT fire on multi-entity query where each entity resolves uniquely; got %v", got.Warnings)
		}
	}
}

// TestRecall_TrueCrossAliasHit_DoesNotSurfaceSimilarShapeWarning
// confirms a cross-alias hit lands in Results — not Mismatches — and
// the envelope doesn't double-surface a similar-shape warning.
func TestRecall_TrueCrossAliasHit_DoesNotSurfaceSimilarShapeWarning(t *testing.T) {
	t.Parallel()
	db := openRecallCanonicalDB(t)
	seedCanonical(t, db, "country", "United States",
		[]string{"United States", "USA"})
	seedCanonicalLearning(t, db, "cup united states world", `["United States"]`,
		"KXMENWORLDCUP-26-US", "kalshi_markets")

	got, err := Recall(context.Background(), db, "odds USA wins world cup", Opts{})
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	if !got.Found {
		t.Fatalf("cross-alias query should hit; got %+v", got)
	}
	for _, w := range got.Warnings {
		if strings.HasPrefix(w, WarningSimilarShapeDifferentEntity+":") {
			t.Errorf("real hit should not surface similar-shape warning; got %v", got.Warnings)
		}
	}
}

// TestRecall_EmptyEntityLookups_FallsBackToLiteral covers the edge case
// where entity_lookups is empty. Cross-alias path can't fire; literal-
// entity matching must still work.
func TestRecall_EmptyEntityLookups_FallsBackToLiteral(t *testing.T) {
	t.Parallel()
	db := openRecallCanonicalDB(t)
	// No entity_lookups seeded.
	seedCanonicalLearning(t, db, "cup portugal world", `["Portugal"]`,
		"KXMENWORLDCUP-26-PT", "kalshi_markets")

	got, err := Recall(context.Background(), db, "odds Portugal wins world cup", Opts{})
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	if !got.Found {
		t.Errorf("want Found=true via literal match even without entity_lookups; got %+v", got)
	}
}

// TestRecall_LegacyNullEntityRow_OpportunisticBackfill exercises U2's
// end-to-end behavior through the U3 switch: a legacy row with
// query_entities=NULL whose query_pattern carries a canonical-
// resolvable token should still match cross-alias queries by walking
// the lowercased tokens through the resolver. Read-only — the DB
// column stays NULL.
func TestRecall_LegacyNullEntityRow_OpportunisticBackfill(t *testing.T) {
	t.Parallel()
	db := openRecallCanonicalDB(t)
	// Single-token alias so the backfill walking individual lowercased
	// tokens through the resolver actually fires.
	seedCanonical(t, db, "country", "United States",
		[]string{"USA", "US"})
	// query_pattern's lowercased "usa" token resolves through the
	// canonical resolver and is treated as the row's effective entity.
	if _, err := db.Exec(`INSERT INTO search_learnings (
		query_pattern, query_entities, resource_id, resource_type, action, source, confidence
	) VALUES (?, NULL, ?, ?, 'boost', 'taught', 2)`,
		"cup usa world", "KXMENWORLDCUP-26-US", "kalshi_markets",
	); err != nil {
		t.Fatalf("seed legacy row: %v", err)
	}

	got, err := Recall(context.Background(), db, "odds US wins world cup", Opts{})
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	if !got.Found {
		t.Errorf("legacy null-entity row should hit via opportunistic backfill; got %+v", got)
	}

	// Confirm read-only: column stays NULL.
	var stored sql.NullString
	if err := db.QueryRow(
		`SELECT query_entities FROM search_learnings WHERE resource_id = ?`,
		"KXMENWORLDCUP-26-US",
	).Scan(&stored); err != nil {
		t.Fatalf("stored row lookup: %v", err)
	}
	if stored.Valid && stored.String != "" && stored.String != "null" {
		t.Errorf("backfill should be read-only; stored column got modified to %q", stored.String)
	}
}

// seedPlaybookRow inserts a learning_playbooks row directly via SQL.
// Keeps the playbook-envelope tests decoupled from the store wrapper —
// the recall path queries the underlying *sql.DB directly.
func seedPlaybookRow(t *testing.T, db *sql.DB, family, playbookJSON, notes string) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO learning_playbooks (query_family, playbook_json, notes_text, source, confidence)
		 VALUES (?, ?, ?, 'taught', 2)`,
		family, playbookJSON, notes,
	); err != nil {
		t.Fatalf("seed playbook: %v", err)
	}
}

// TestRecall_PlaybookSurfaces_OnFamilyMatch covers U7's primary
// contract: a query whose QueryFamily matches a stored
// learning_playbooks row carries the resolved playbook + notes on the
// recall envelope. Empty results list is fine — playbooks live on the
// envelope orthogonally to per-resource hits.
func TestRecall_PlaybookSurfaces_OnFamilyMatch(t *testing.T) {
	t.Parallel()
	db := openRecallCanonicalDB(t)
	seedCanonical(t, db, "country", "Portugal", []string{"Portugal", "PT"})

	query := "odds Portugal wins world cup"
	family := QueryFamily(PromoteEntities(
		Normalize(query, DefaultPredictionGoatConfig()),
		NewCanonicalResolver(context.Background(), db),
	))
	if family == "" {
		t.Fatalf("expected non-empty family for query %q", query)
	}
	playbookJSON := `{
		"query_family_examples": ["odds Portugal wins world cup"],
		"entity_slots": ["$COUNTRY"],
		"expected_tool_calls": 2,
		"steps": [
			{"cmd": "prediction-goat-pp-cli events list --query {$COUNTRY.canonical}", "purpose": "find event"}
		]
	}`
	seedPlaybookRow(t, db, family, playbookJSON,
		"World Cup markets sometimes use the parent ticker; always check children.\n")

	got, err := Recall(context.Background(), db, query, Opts{})
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	if got.Playbook == nil {
		t.Fatalf("want Playbook on envelope; got nil. Result=%+v", got)
	}
	if got.Playbook.QueryFamily != family {
		t.Errorf("Playbook.QueryFamily = %q, want %q", got.Playbook.QueryFamily, family)
	}
	if len(got.Playbook.Playbook.Steps) == 0 {
		t.Errorf("want >=1 step on playbook; got 0")
	}
	if got.Notes == "" {
		t.Errorf("want non-empty Notes on envelope; got empty")
	}
	if got.Playbook.SlotsResolved["$COUNTRY"] == nil {
		t.Errorf("want $COUNTRY slot resolved; got SlotsResolved=%v", got.Playbook.SlotsResolved)
	}
}

// TestRecall_PlaybookSurfaces_DifferentEntitySameFamily is the killer-
// feature test. A playbook seeded for one country ("Portugal") is
// retrieved by a structurally-identical query for a different country
// ("Brazil"). The family key is entity-stripped, so the lookup hits;
// slot resolution then binds $COUNTRY to Brazil's canonical via the
// per-call resolver. Demonstrates playbooks are entity-agnostic at the
// slot-binding layer — one author lift, infinite-entity replay.
func TestRecall_PlaybookSurfaces_DifferentEntitySameFamily(t *testing.T) {
	t.Parallel()
	db := openRecallCanonicalDB(t)
	seedCanonical(t, db, "country", "Portugal", []string{"Portugal", "PT"})
	seedCanonical(t, db, "country", "Brazil", []string{"Brazil", "BR"})

	// Teach the playbook against the Portugal query.
	portugalQuery := "odds Portugal wins world cup"
	family := QueryFamily(PromoteEntities(
		Normalize(portugalQuery, DefaultPredictionGoatConfig()),
		NewCanonicalResolver(context.Background(), db),
	))
	playbookJSON := `{
		"query_family_examples": ["odds Portugal wins world cup"],
		"entity_slots": ["$COUNTRY"],
		"expected_tool_calls": 2,
		"steps": [
			{"cmd": "prediction-goat-pp-cli events list --query {$COUNTRY.canonical}"}
		]
	}`
	seedPlaybookRow(t, db, family, playbookJSON, "Endpoint envelope varies by venue.\n")

	// Brazil query MUST hit the same playbook.
	got, err := Recall(context.Background(), db, "odds Brazil wins world cup", Opts{})
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	if got.Playbook == nil {
		t.Fatalf("cross-entity replay failed: want Playbook on envelope, got nil. Result=%+v", got)
	}
	if got.Playbook.QueryFamily != family {
		t.Errorf("Playbook.QueryFamily = %q, want %q", got.Playbook.QueryFamily, family)
	}
	// Slot must bind to Brazil's canonical, not Portugal's.
	slot := got.Playbook.SlotsResolved["$COUNTRY"]
	if slot == nil {
		t.Fatalf("want $COUNTRY slot resolved on Brazil query; got SlotsResolved=%v",
			got.Playbook.SlotsResolved)
	}
	if got, ok := slot["canonical"].(string); !ok || got != "Brazil" {
		t.Errorf("$COUNTRY.canonical = %v, want Brazil (slot must bind to query entity, not playbook author's)", slot)
	}
}

// TestRecall_PlaybookAbsent_NoMatch covers the negative case: a
// query whose family has no stored playbook returns envelope.Playbook
// = nil and no error. Notes also empty.
func TestRecall_PlaybookAbsent_NoMatch(t *testing.T) {
	t.Parallel()
	db := openRecallCanonicalDB(t)
	// No playbook rows seeded.
	got, err := Recall(context.Background(), db, "some unrelated query", Opts{})
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	if got.Playbook != nil {
		t.Errorf("want envelope.Playbook = nil when family has no row; got %+v", got.Playbook)
	}
	if got.Notes != "" {
		t.Errorf("want envelope.Notes empty; got %q", got.Notes)
	}
}

// TestRecall_PlaybookNotesOnly covers the notes-only row: a stored
// playbook with empty playbook_json but populated notes_text surfaces
// Notes on the envelope; the Playbook wrapper is still attached so the
// agent gets the guidance even without structured choreography. The
// embedded Playbook value has zero steps (matching ESPN's behavior).
func TestRecall_PlaybookNotesOnly(t *testing.T) {
	t.Parallel()
	db := openRecallCanonicalDB(t)
	query := "world cup parent ticker workaround"
	family := QueryFamily(PromoteEntities(
		Normalize(query, DefaultPredictionGoatConfig()),
		NewCanonicalResolver(context.Background(), db),
	))
	notes := "Kalshi parent tickers don't carry per-team odds; always drill to children.\n"
	seedPlaybookRow(t, db, family, "", notes)

	got, err := Recall(context.Background(), db, query, Opts{})
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	if got.Notes != notes {
		t.Errorf("Notes = %q, want %q", got.Notes, notes)
	}
	if got.Playbook == nil {
		t.Fatalf("notes-only row should still attach Playbook wrapper carrying the notes; got nil")
	}
	if len(got.Playbook.Playbook.Steps) != 0 {
		t.Errorf("notes-only row should have zero steps; got %d", len(got.Playbook.Playbook.Steps))
	}
	if got.Playbook.Notes != notes {
		t.Errorf("ResolvedPlaybook.Notes = %q, want %q", got.Playbook.Notes, notes)
	}
}

// TestRecall_PlaybookCoexistsWithResourceHit confirms playbooks land
// on the envelope orthogonally to per-resource hits. A query that hits
// both a search_learnings row AND has a stored playbook for its family
// returns both surfaces populated.
func TestRecall_PlaybookCoexistsWithResourceHit(t *testing.T) {
	t.Parallel()
	db := openRecallCanonicalDB(t)
	seedCanonical(t, db, "country", "Portugal", []string{"Portugal", "PT"})

	query := "odds Portugal wins world cup"
	// Seed a direct learning that matches.
	seedCanonicalLearning(t, db, "cup portugal world", `["Portugal"]`,
		"KXMENWORLDCUP-26-PT", "kalshi_markets")
	// Seed a playbook for the same family.
	family := QueryFamily(PromoteEntities(
		Normalize(query, DefaultPredictionGoatConfig()),
		NewCanonicalResolver(context.Background(), db),
	))
	playbookJSON := `{
		"query_family_examples": ["odds Portugal wins world cup"],
		"entity_slots": ["$COUNTRY"],
		"steps": [{"cmd": "prediction-goat-pp-cli markets get KXMENWORLDCUP-26-{$COUNTRY.canonical}"}]
	}`
	seedPlaybookRow(t, db, family, playbookJSON, "Drill to child ticker.\n")

	got, err := Recall(context.Background(), db, query, Opts{})
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	if !got.Found {
		t.Errorf("want Found=true (direct learning hit); got Result=%+v", got)
	}
	if len(got.Results) == 0 {
		t.Errorf("want >=1 result from search_learnings; got 0")
	}
	if got.Playbook == nil {
		t.Errorf("want envelope.Playbook attached alongside results; got nil")
	}
	if got.Notes == "" {
		t.Errorf("want envelope.Notes populated alongside results; got empty")
	}
}
