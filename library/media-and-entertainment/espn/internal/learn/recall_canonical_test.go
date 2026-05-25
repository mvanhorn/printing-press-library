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
