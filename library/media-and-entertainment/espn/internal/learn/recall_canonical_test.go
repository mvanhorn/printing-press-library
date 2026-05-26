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
// seedPlaybook directly inserts a learning_playbooks row. Mirrors the
// shape store.UpsertPlaybook writes; keeps these tests decoupled from
// the store API surface.
func seedPlaybook(t *testing.T, db *sql.DB, family, playbookJSON, notes string) {
	t.Helper()
	// Ensure the table exists; canonical test DB doesn't create it.
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS learning_playbooks (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		query_family TEXT NOT NULL,
		playbook_json TEXT,
		notes_text TEXT,
		source TEXT NOT NULL DEFAULT 'taught',
		confidence INTEGER NOT NULL DEFAULT 2,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		last_observed_at DATETIME
	)`); err != nil {
		t.Fatalf("seed playbook table: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO learning_playbooks (query_family, playbook_json, notes_text, confidence)
		 VALUES (?, ?, ?, 2)`,
		family, playbookJSON, notes,
	); err != nil {
		t.Fatalf("seed playbook row: %v", err)
	}
}

func TestRecall_PlaybookSurfaces_OnFamilyMatch(t *testing.T) {
	t.Parallel()
	db := openCanonicalTestDB(t)
	seedCanonical(t, db, "nba_team", "Detroit Pistons",
		[]string{"Detroit Pistons", "Pistons", "DET"})

	pbJSON := `{"steps":[{"cmd":"teams basketball nba {team.id}"}],"entity_slots":["$TEAM"],"expected_tool_calls":3}`
	notes := "byathlete needs seasontype=2; categories has dup labels"
	// Family for "how did $TEAM end the season who led in ppg rpg spg"
	// is "end led ppg rpg season spg" after entities + stopwords stripped.
	seedPlaybook(t, db, "end led ppg rpg season spg", pbJSON, notes)

	got, err := Recall(context.Background(), db,
		"how did Pistons end the season who led in ppg rpg spg",
		Opts{EntityConfig: espnLikeConfig()})
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	if got.Playbook == nil {
		t.Fatalf("expected playbook attached; got %+v", got)
	}
	if got.Notes != notes {
		t.Errorf("notes mismatch: got %q want %q", got.Notes, notes)
	}
	if len(got.Playbook.Playbook.Steps) != 1 {
		t.Errorf("expected 1 step; got %d", len(got.Playbook.Playbook.Steps))
	}
	if got.Playbook.SlotsResolved == nil {
		t.Errorf("expected slots resolved")
	} else if slot, ok := got.Playbook.SlotsResolved["$TEAM"]; !ok {
		t.Errorf("$TEAM slot missing in %+v", got.Playbook.SlotsResolved)
	} else if slot["canonical"] != "Detroit Pistons" {
		t.Errorf("canonical = %v, want Detroit Pistons", slot["canonical"])
	}
}

func TestRecall_PlaybookSurfaces_DifferentEntitySameFamily(t *testing.T) {
	t.Parallel()
	db := openCanonicalTestDB(t)
	seedCanonical(t, db, "nba_team", "Golden State Warriors",
		[]string{"Golden State Warriors", "Warriors", "GSW"})
	seedCanonical(t, db, "nba_team", "Detroit Pistons",
		[]string{"Detroit Pistons", "Pistons", "DET"})

	pbJSON := `{"steps":[{"cmd":"teams basketball nba {team.id}"}],"entity_slots":["$TEAM"]}`
	seedPlaybook(t, db, "end led ppg rpg season spg", pbJSON, "")

	// Teach was for Warriors; recall asks Pistons. Same family.
	got, err := Recall(context.Background(), db,
		"how did Pistons end the season who led in ppg rpg spg",
		Opts{EntityConfig: espnLikeConfig()})
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	if got.Playbook == nil {
		t.Fatalf("playbook should fire across entities of same family; got %+v", got)
	}
	slot := got.Playbook.SlotsResolved["$TEAM"]
	if slot["canonical"] != "Detroit Pistons" {
		t.Errorf("slot should resolve to Pistons (the current query), got %v", slot["canonical"])
	}
}

func TestRecall_PlaybookAbsent_NoMatch(t *testing.T) {
	t.Parallel()
	db := openCanonicalTestDB(t)
	got, err := Recall(context.Background(), db, "completely cold query",
		Opts{EntityConfig: espnLikeConfig()})
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	if got.Playbook != nil {
		t.Errorf("no playbook stored -> envelope should have nil Playbook; got %+v", got.Playbook)
	}
	if got.Notes != "" {
		t.Errorf("no playbook stored -> envelope Notes should be empty; got %q", got.Notes)
	}
}

func TestRecall_PlaybookNotesOnly(t *testing.T) {
	t.Parallel()
	db := openCanonicalTestDB(t)
	// Seed Mets so it gets promoted to entity (stripped from family).
	seedCanonical(t, db, "mlb_team", "New York Mets", []string{"Mets", "NYM"})
	seedPlaybook(t, db, "doing far so year", "", "use the byathlete endpoint")

	got, err := Recall(context.Background(), db, "how are the mets doing so far this year",
		Opts{EntityConfig: espnLikeConfig()})
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	if got.Notes != "use the byathlete endpoint" {
		t.Errorf("notes-only row should surface notes; got %q (normalized=%q)", got.Notes, got.Normalized)
	}
	// Playbook field should be nil when playbook_json is empty AND has no steps.
	if got.Playbook != nil && len(got.Playbook.Playbook.Steps) != 0 {
		t.Errorf("notes-only row should not synthesize steps; got %+v", got.Playbook)
	}
}

func TestRecall_PlaybookCoexistsWithResourceHit(t *testing.T) {
	t.Parallel()
	db := openCanonicalTestDB(t)
	seedCanonical(t, db, "nba_team", "Detroit Pistons",
		[]string{"Detroit Pistons", "Pistons", "DET"})
	// Resource learning AND playbook for the same query.
	seedCanonicalLearning(t, db,
		"how pistons end season who led ppg rpg spg",
		`["Pistons"]`, "8", "teams")
	seedPlaybook(t, db, "end led ppg rpg season spg",
		`{"steps":[{"cmd":"teams basketball nba {team.id}"}]}`,
		"recipe goes here")

	got, err := Recall(context.Background(), db,
		"how did Pistons end the season who led in ppg rpg spg",
		Opts{EntityConfig: espnLikeConfig()})
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	if !got.Found {
		t.Errorf("resource hit should still fire; got found=false")
	}
	if got.Playbook == nil {
		t.Errorf("playbook should also fire; got nil")
	}
}

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

// TestRecall_SameEntity_NoCanonicalLookup_DropsBelowJMin guards the
// regression Greptile flagged on PR #851: when both query and stored
// row share a literal entity but entity_lookups has no canonical row
// for it, queryCanonicals and storedCanonicals are both empty.
// Without an explicit guard, case 2 of the cross-alias fallback would
// admit such rows below jMin via the looser crossAliasMin floor —
// silently downgrading the effective Jaccard minimum from 0.6 to 0.3
// for every unregistered entity. With the guard in place, the row
// must be dropped at the jMin gate.
func TestRecall_SameEntity_NoCanonicalLookup_DropsBelowJMin(t *testing.T) {
	t.Parallel()
	db := openCanonicalTestDB(t)
	// Intentionally do NOT seed entity_lookups for "mariners". The
	// stored row carries "mariners" as a literal entity; the recall
	// query carries the same literal entity. Non-entity Jaccard
	// between the two should fall in (crossAliasMin, jMin) so the
	// only fallback that could admit the row is case 2.
	seedCanonicalLearning(t, db,
		"how mariners doing season year",
		`["Mariners"]`, "12", "teams")

	got, err := Recall(context.Background(), db,
		"are the mariners doing well this year overall",
		Opts{EntityConfig: espnLikeConfig()})
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

// TestRecall_SameEntity_CanonicalJaccardDoesNotInflateScore guards
// the second Greptile finding on PR #851 round 3: case 1 of the
// fallback switch (canonicalOverlap branch) used to boost the score
// via canonicalJaccard for any row whose canonicals overlapped the
// query — including same-literal-entity rows where the boost is
// trivially 1.0 (one canonical on each side, identical). A
// structurally unrelated stored row could surface at score=1.0 above
// jMin. The case-1 entitySlicesIntersect guard mirrors the case-2
// guard added in the prior round.
func TestRecall_SameEntity_CanonicalJaccardDoesNotInflateScore(t *testing.T) {
	t.Parallel()
	db := openCanonicalTestDB(t)
	seedCanonical(t, db, "mlb_team", "Seattle Mariners",
		[]string{"Seattle Mariners", "Mariners", "SEA"})
	// Stored row shares the entity ("mariners") and is structurally
	// disjoint from the recall query: zero non-entity-token overlap.
	// Pre-fix, canonicalJaccard would boost this to score=1.0.
	seedCanonicalLearning(t, db,
		"today scoreboard mariners",
		`["Mariners"]`, "tv-scoreboard", "tv")

	got, err := Recall(context.Background(), db,
		"how did mariners end the year ppg stats",
		Opts{EntityConfig: espnLikeConfig()})
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	if got.Found {
		t.Errorf("structurally disjoint same-entity row must not be admitted via canonical-overlap boost; got %+v", got)
	}
	for _, r := range got.Results {
		if r.ResourceID == "tv-scoreboard" {
			t.Errorf("unrelated stored row leaked into Results with score=%v", r.MatchScore)
		}
	}
}
