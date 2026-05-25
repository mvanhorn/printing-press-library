// Cross-alias canonical-resolution tests.
//
// Teaching "Niners game tonight" → eventID is supposed to make
// "49ers game tonight" / "SF game tonight" hit the same learning
// from a cold start, because the seeded entity_lookups table
// records all three as values under the canonical "San Francisco
// 49ers". Before U3 of plan 2026-05-25-003 only the literal-alias
// path worked; this file locks in the canonical-resolution path.

package learn

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/espn/internal/learn/entities"

	_ "modernc.org/sqlite"
)

func openCanonicalTestDB(t *testing.T) *sql.DB {
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
		`CREATE TABLE search_patterns (
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
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("schema: %v", err)
		}
	}
	return db
}

// espnLikeConfig builds an entities.Config matching what espn's
// learn_init.go registers — sports stopwords like "game" and
// "tonight" so the entity extractor strips them and leaves entities
// only.
func espnLikeConfig() *entities.Config {
	cfg := entities.NewConfig()
	cfg.RegisterStopwords("game", "games", "tonight", "today", "weekend", "vs", "v", "versus")
	return cfg
}

// seedCanonical inserts a (kind, canonical, value) row into
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

// seedCanonicalLearning writes a learning row directly via SQL to
// avoid coupling these tests to the (still-evolving) write path.
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

func TestRecall_CrossAlias_NinersTo49ers(t *testing.T) {
	t.Parallel()
	db := openCanonicalTestDB(t)
	seedCanonical(t, db, "nfl_team", "San Francisco 49ers",
		[]string{"San Francisco 49ers", "Niners", "49ers", "SF"})
	// Teach: "Niners game tonight" → event 401547432
	seedCanonicalLearning(t, db, "niners game tonight", `["Niners"]`, "401547432", "events")

	// Recall with the different alias "49ers"
	got, err := Recall(context.Background(), db, "49ers game tonight", Opts{EntityConfig: espnLikeConfig()})
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	if !got.Found {
		t.Fatalf("want Found=true via cross-alias canonical resolution; got %+v", got)
	}
	if len(got.Results) != 1 {
		t.Fatalf("want 1 result, got %d", len(got.Results))
	}
	if got.Results[0].ResourceID != "401547432" {
		t.Errorf("ResourceID = %q, want 401547432", got.Results[0].ResourceID)
	}
}

func TestRecall_CrossAlias_NinersToSF(t *testing.T) {
	t.Parallel()
	db := openCanonicalTestDB(t)
	seedCanonical(t, db, "nfl_team", "San Francisco 49ers",
		[]string{"San Francisco 49ers", "Niners", "49ers", "SF"})
	seedCanonicalLearning(t, db, "niners game tonight", `["Niners"]`, "401547432", "events")

	got, err := Recall(context.Background(), db, "SF game tonight", Opts{EntityConfig: espnLikeConfig()})
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	if !got.Found {
		t.Fatalf("want Found=true via 'SF' alias; got %+v", got)
	}
}

func TestRecall_CrossAlias_WrongCanonicalMismatch(t *testing.T) {
	t.Parallel()
	db := openCanonicalTestDB(t)
	seedCanonical(t, db, "nfl_team", "San Francisco 49ers",
		[]string{"San Francisco 49ers", "Niners"})
	seedCanonical(t, db, "nfl_team", "Dallas Cowboys",
		[]string{"Dallas Cowboys", "Cowboys"})
	// Teach Niners → eventID; recall for a Cowboys query must NOT
	// surface the Niners row because the canonicals differ.
	seedCanonicalLearning(t, db, "niners game tonight", `["Niners"]`, "401547432", "events")

	got, err := Recall(context.Background(), db, "Cowboys game tonight", Opts{EntityConfig: espnLikeConfig()})
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	if got.Found {
		t.Errorf("want Found=false (canonicals differ), got %+v", got)
	}
}

func TestRecall_CrossAlias_NBAKind(t *testing.T) {
	t.Parallel()
	db := openCanonicalTestDB(t)
	seedCanonical(t, db, "nba_team", "Los Angeles Lakers",
		[]string{"Los Angeles Lakers", "Lakers", "LAL"})
	seedCanonicalLearning(t, db, "lakers game tonight", `["Lakers"]`, "401555555", "events")

	got, err := Recall(context.Background(), db, "LAL game tonight", Opts{EntityConfig: espnLikeConfig()})
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	if !got.Found {
		t.Fatalf("want Found=true (LAL → Los Angeles Lakers canonical match); got %+v", got)
	}
}

func TestRecall_CrossAlias_EmptyEntityLookups_FallsBackToLiteral(t *testing.T) {
	t.Parallel()
	db := openCanonicalTestDB(t)
	// No entity_lookups seeded. Cross-alias path can't fire because
	// canonicals are empty. Literal-entity match should still work
	// for same-alias queries (covered by U21).
	seedCanonicalLearning(t, db, "niners game tonight", `["Niners"]`, "401547432", "events")

	// Same-alias query should still hit via literal path.
	got, err := Recall(context.Background(), db, "Niners game tonight", Opts{EntityConfig: espnLikeConfig()})
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	if !got.Found {
		t.Errorf("want Found=true via literal match even without entity_lookups; got %+v", got)
	}
}

// TestRecall_LegacyNullEntityRow_OpportunisticBackfill exercises U2
// of plan 2026-05-25-004. A row written before symmetric teach-time
// promotion landed has query_entities=null. The recall path should
// walk the lowercased query_pattern through the canonical resolver
// to derive effective entities for cross-alias matching this call,
// without writing back to the DB.
func TestRecall_LegacyNullEntityRow_OpportunisticBackfill(t *testing.T) {
	t.Parallel()
	db := openCanonicalTestDB(t)
	seedCanonical(t, db, "nfl_team", "San Francisco 49ers",
		[]string{"San Francisco 49ers", "Niners", "49ers", "SF"})
	// Seed a legacy row: query_entities=null, query_pattern
	// contains lowercase 'niners' that resolves via entity_lookups.
	if _, err := db.Exec(`INSERT INTO search_learnings (
		query_pattern, query_entities, resource_id, resource_type, action, source, confidence
	) VALUES (?, NULL, ?, ?, 'boost', 'taught', 2)`,
		"niners game tonight", "401547432", "events",
	); err != nil {
		t.Fatalf("seed legacy row: %v", err)
	}

	// Recall with a different alias — cross-alias must still fire
	// against the legacy null-entity row.
	got, err := Recall(context.Background(), db, "49ers game tonight", Opts{EntityConfig: espnLikeConfig()})
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	if !got.Found {
		t.Errorf("legacy null-entity row should still hit via opportunistic backfill; got %+v", got)
	}

	// Confirm we did NOT write back: the column should still be NULL.
	var stored sql.NullString
	if err := db.QueryRow(
		`SELECT query_entities FROM search_learnings WHERE resource_id = ?`,
		"401547432",
	).Scan(&stored); err != nil {
		t.Fatalf("stored row lookup: %v", err)
	}
	if stored.Valid && stored.String != "" && stored.String != "null" {
		t.Errorf("backfill should be read-only; stored column got modified to %q", stored.String)
	}
}

// TestRecall_LegacyNullEntityRow_NoResolvableTokens confirms the
// backfill is strictly additive — a legacy null-entity row whose
// query_pattern has no canonical-resolvable tokens behaves as it did
// before this plan.
func TestRecall_LegacyNullEntityRow_NoResolvableTokens(t *testing.T) {
	t.Parallel()
	db := openCanonicalTestDB(t)
	// No entity_lookups seeded.
	if _, err := db.Exec(`INSERT INTO search_learnings (
		query_pattern, query_entities, resource_id, resource_type, action, source, confidence
	) VALUES (?, NULL, ?, ?, 'boost', 'taught', 2)`,
		"how weather forecast today", "weather-1", "weather",
	); err != nil {
		t.Fatalf("seed legacy row: %v", err)
	}

	got, err := Recall(context.Background(), db, "how mariners game tonight", Opts{EntityConfig: espnLikeConfig()})
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	if got.Found {
		t.Errorf("unrelated query against a null-entity row should not match; got %+v", got)
	}
}

// TestRecall_SimilarShapeDifferentEntity_SurfacesWarning exercises U3
// of plan 2026-05-25-004. The dogfood session 4 scenario: a Mariners
// learning exists, the agent asks about the Mets. Both queries share
// the structural shape "how doing year/season" but the entities
// differ. The default envelope should surface a top-level warning
// naming the alternative canonical, not the misleading
// no_learnings_for_query_family.
func TestRecall_SimilarShapeDifferentEntity_SurfacesWarning(t *testing.T) {
	t.Parallel()
	db := openCanonicalTestDB(t)
	seedCanonical(t, db, "mlb_team", "Seattle Mariners",
		[]string{"Seattle Mariners", "Mariners", "SEA"})
	seedCanonical(t, db, "mlb_team", "New York Mets",
		[]string{"New York Mets", "Mets", "NYM"})
	seedCanonicalLearning(t, db, "how mariners doing season year",
		`["Mariners"]`, "12", "teams")

	got, err := Recall(context.Background(), db, "how are the Mets doing this year", Opts{EntityConfig: espnLikeConfig()})
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	if got.Found {
		t.Fatalf("different-entity query should not be found; got %+v", got)
	}
	want := WarningSimilarShapeDifferentEntity + ":Seattle Mariners"
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
			t.Errorf("similar-shape warning should suppress %q; got %v", TopWarningNoLearningsForQueryFamily, got.Warnings)
		}
	}
}

// TestRecall_SimilarShapeDifferentEntity_MultipleCanonicals confirms
// the warning fires once per alternative canonical when several
// stored rows share the shape but resolve to different entities.
func TestRecall_SimilarShapeDifferentEntity_MultipleCanonicals(t *testing.T) {
	t.Parallel()
	db := openCanonicalTestDB(t)
	seedCanonical(t, db, "mlb_team", "Seattle Mariners",
		[]string{"Seattle Mariners", "Mariners"})
	seedCanonical(t, db, "mlb_team", "New York Yankees",
		[]string{"New York Yankees", "Yankees"})
	seedCanonical(t, db, "mlb_team", "Boston Red Sox",
		[]string{"Boston Red Sox", "Red Sox"})
	seedCanonicalLearning(t, db, "how mariners doing season year",
		`["Mariners"]`, "12", "teams")
	seedCanonicalLearning(t, db, "how yankees doing season year",
		`["Yankees"]`, "10", "teams")

	got, err := Recall(context.Background(), db, "how are the Red Sox doing this year", Opts{EntityConfig: espnLikeConfig()})
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	wantM := WarningSimilarShapeDifferentEntity + ":Seattle Mariners"
	wantY := WarningSimilarShapeDifferentEntity + ":New York Yankees"
	foundM, foundY := false, false
	for _, w := range got.Warnings {
		if w == wantM {
			foundM = true
		}
		if w == wantY {
			foundY = true
		}
	}
	if !foundM || !foundY {
		t.Errorf("want both %q and %q in envelope; got %v", wantM, wantY, got.Warnings)
	}
}

// TestRecall_NoMismatches_KeepsNoLearningsWarning confirms a true
// cold-start envelope still carries no_learnings_for_query_family.
func TestRecall_NoMismatches_KeepsNoLearningsWarning(t *testing.T) {
	t.Parallel()
	db := openCanonicalTestDB(t)
	got, err := Recall(context.Background(), db, "completely cold query", Opts{EntityConfig: espnLikeConfig()})
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	hasNoLearnings := false
	for _, w := range got.Warnings {
		if w == TopWarningNoLearningsForQueryFamily {
			hasNoLearnings = true
		}
	}
	if !hasNoLearnings {
		t.Errorf("cold envelope should carry %q; got %v", TopWarningNoLearningsForQueryFamily, got.Warnings)
	}
}

// TestRecall_TrueCrossAliasHit_DoesNotSurfaceSimilarShapeWarning
// confirms a row promoted to a real Hit via cross-alias canonical
// resolution doesn't double-surface as a similar-shape mismatch.
func TestRecall_TrueCrossAliasHit_DoesNotSurfaceSimilarShapeWarning(t *testing.T) {
	t.Parallel()
	db := openCanonicalTestDB(t)
	seedCanonical(t, db, "nfl_team", "San Francisco 49ers",
		[]string{"San Francisco 49ers", "Niners", "49ers"})
	seedCanonicalLearning(t, db, "niners game tonight", `["Niners"]`, "401547432", "events")

	got, err := Recall(context.Background(), db, "49ers game tonight", Opts{EntityConfig: espnLikeConfig()})
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

// TestRecall_CrossAliasJaccardMin_LowerFloorCatchesParaphrase
// exercises U4 of plan 2026-05-25-004. With jMin=0.6 the boolean
// at-threshold hack was needed to pass cross-alias hits at all; with
// a separate crossAliasMin=0.3 the canonical-overlap path admits
// paraphrased same-shape queries on their actual Jaccard ratio.
func TestRecall_CrossAliasJaccardMin_LowerFloorCatchesParaphrase(t *testing.T) {
	t.Parallel()
	db := openCanonicalTestDB(t)
	seedCanonical(t, db, "mlb_team", "Seattle Mariners",
		[]string{"Seattle Mariners", "Mariners", "SEA"})
	seedCanonical(t, db, "mlb_team", "New York Mets",
		[]string{"New York Mets", "Mets", "NYM"})
	// Mariners teach. Then ask the Mets question — different alias
	// AND different canonical, so this should NOT hit. Confirms the
	// cross-alias floor isn't a free pass; it still needs canonical
	// overlap.
	seedCanonicalLearning(t, db, "how mariners doing season year",
		`["Mariners"]`, "12", "teams")

	got, err := Recall(context.Background(), db, "how are the Mets doing this year", Opts{EntityConfig: espnLikeConfig()})
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	// Different canonical => no real Results, but envelope surfaces
	// the similar-shape warning from U3 + the Mariners row sits in
	// debug mismatches.
	if got.Found {
		t.Errorf("different canonical should not promote to Results; got %+v", got)
	}
}

// TestRecall_CrossAliasJaccardMin_OverlapEnablesLowerFloor confirms
// a query whose canonical truly overlaps an existing teach (different
// alias, same canonical) clears the lower floor even when literal
// non-entity Jaccard is below 0.6.
func TestRecall_CrossAliasJaccardMin_OverlapEnablesLowerFloor(t *testing.T) {
	t.Parallel()
	db := openCanonicalTestDB(t)
	seedCanonical(t, db, "nfl_team", "San Francisco 49ers",
		[]string{"San Francisco 49ers", "Niners", "49ers", "SF"})
	// Teach with a verbose query, recall with a terse one. Non-entity
	// Jaccard ratio drops because the term overlap is small, but the
	// canonical match should still let the row through.
	seedCanonicalLearning(t, db, "tonight game niners stadium home", `["Niners"]`, "401547432", "events")

	got, err := Recall(context.Background(), db, "49ers stadium", Opts{EntityConfig: espnLikeConfig()})
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	if !got.Found {
		t.Errorf("cross-alias canonical match should clear the lower floor; got %+v", got)
	}
}

func TestRecall_CrossAlias_PromotesEntityMatchExact(t *testing.T) {
	t.Parallel()
	db := openCanonicalTestDB(t)
	seedCanonical(t, db, "nfl_team", "San Francisco 49ers",
		[]string{"San Francisco 49ers", "Niners", "49ers"})
	seedCanonicalLearning(t, db, "niners game tonight", `["Niners"]`, "401547432", "events")

	got, err := Recall(context.Background(), db, "49ers game tonight", Opts{EntityConfig: espnLikeConfig()})
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	if !got.Found || len(got.Results) == 0 {
		t.Fatalf("want a hit, got %+v", got)
	}
	if got.Results[0].EntityMatch != EntityMatchExact {
		t.Errorf("EntityMatch = %q, want %q (cross-alias should promote Mismatch → Exact)",
			got.Results[0].EntityMatch, EntityMatchExact)
	}
	// Warning should flag the cross-alias resolution path.
	foundWarning := false
	for _, w := range got.Results[0].Warnings {
		if w == WarningCrossAliasMatch {
			foundWarning = true
			break
		}
	}
	if !foundWarning {
		t.Errorf("want %q warning on cross-alias hit; got %v", WarningCrossAliasMatch, got.Results[0].Warnings)
	}
}
