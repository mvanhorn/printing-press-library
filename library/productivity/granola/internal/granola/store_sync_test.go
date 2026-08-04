// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0.

package granola

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

// ---------------------------------------------------------------------------
// Helpers.
//
// PII: every value below is invented. Folder titles, recipe names and
// descriptions are synthetic and were never captured from a real Granola
// account.
// ---------------------------------------------------------------------------

// legacyCatalogSchema is the folders / panel_templates / recipes /
// recipes_usage shape an older binary created: no row_source anywhere, and no
// folder description or favourite flag. CREATE TABLE IF NOT EXISTS is a no-op
// against these, so only the additive column migration can upgrade them.
var legacyCatalogSchema = []string{
	`CREATE TABLE folders (
		id TEXT PRIMARY KEY, title TEXT, parent_id TEXT,
		workspace_id TEXT, owner_id TEXT, preset TEXT
	)`,
	`CREATE TABLE panel_templates (
		id TEXT PRIMARY KEY, slug TEXT, title TEXT, description TEXT, category TEXT
	)`,
	`CREATE TABLE recipes (
		id TEXT PRIMARY KEY, slug TEXT, name TEXT, description TEXT,
		category TEXT, source TEXT
	)`,
	`CREATE TABLE recipes_usage (
		recipe_id TEXT PRIMARY KEY,
		total_count INTEGER NOT NULL DEFAULT 0,
		last_used_at TEXT
	)`,
}

// catalogTables are the four tables this round gave provenance columns to.
var catalogTables = []string{"folders", "panel_templates", "recipes", "recipes_usage"}

// openRawDB opens an empty database WITHOUT running EnsureSchema, so a test
// can install an older binary's table shapes first.
func openRawDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "raw.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// tableColumns returns the column set of table.
func tableColumns(t *testing.T, db *sql.DB, table string) map[string]bool {
	t.Helper()
	rows, err := db.QueryContext(context.Background(), fmt.Sprintf(`PRAGMA table_info("%s")`, table))
	if err != nil {
		t.Fatalf("table_info %s: %v", table, err)
	}
	defer rows.Close()
	cols := map[string]bool{}
	for rows.Next() {
		var cid, notnull, pk int
		var name, typ string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			t.Fatalf("scan table_info %s: %v", table, err)
		}
		cols[name] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate table_info %s: %v", table, err)
	}
	return cols
}

// schemaSnapshot renders every object's DDL plus the four catalog tables'
// column lists into one comparable string. Used to prove a second EnsureSchema
// changes nothing.
func schemaSnapshot(t *testing.T, db *sql.DB) string {
	t.Helper()
	rows, err := db.QueryContext(context.Background(),
		`SELECT type, name, IFNULL(sql,'') FROM sqlite_master ORDER BY type, name`)
	if err != nil {
		t.Fatalf("read sqlite_master: %v", err)
	}
	defer rows.Close()
	var lines []string
	for rows.Next() {
		var typ, name, ddl string
		if err := rows.Scan(&typ, &name, &ddl); err != nil {
			t.Fatalf("scan sqlite_master: %v", err)
		}
		lines = append(lines, typ+"|"+name+"|"+ddl)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate sqlite_master: %v", err)
	}
	for _, tbl := range catalogTables {
		var cols []string
		for c := range tableColumns(t, db, tbl) {
			cols = append(cols, c)
		}
		sort.Strings(cols)
		lines = append(lines, "cols|"+tbl+"|"+strings.Join(cols, ","))
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

// queryString reads one text column.
func queryString(t *testing.T, db *sql.DB, query string, args ...any) string {
	t.Helper()
	var out sql.NullString
	if err := db.QueryRowContext(context.Background(), query, args...).Scan(&out); err != nil {
		t.Fatalf("query (%s): %v", query, err)
	}
	return out.String
}

// queryInt reads one integer column.
func queryInt(t *testing.T, db *sql.DB, query string, args ...any) int {
	t.Helper()
	var out sql.NullInt64
	if err := db.QueryRowContext(context.Background(), query, args...).Scan(&out); err != nil {
		t.Fatalf("query (%s): %v", query, err)
	}
	return int(out.Int64)
}

// ---------------------------------------------------------------------------
// Migration.
// ---------------------------------------------------------------------------

// TestEnsureSchema_UpgradesLegacyCatalogTables covers the upgrade path that
// matters most: a database an older binary created already has these four
// tables, so CREATE TABLE IF NOT EXISTS never runs and only the additive
// column list can introduce row_source and the folder metadata columns. The
// pre-existing rows must survive and must read back as cache-owned, which is
// the historical truth — before the API path existed every one of these rows
// came from the desktop cache.
func TestEnsureSchema_UpgradesLegacyCatalogTables(t *testing.T) {
	ctx := context.Background()
	db := openRawDB(t)
	for _, stmt := range legacyCatalogSchema {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("create legacy table: %v", err)
		}
	}
	seed := []struct {
		stmt string
		args []any
	}{
		{`INSERT INTO folders(id,title,parent_id,workspace_id,owner_id,preset) VALUES (?,?,?,?,?,?)`,
			[]any{"fold_legacy", "Customers", "fold_root", "ws_1", "usr_1", "sales"}},
		{`INSERT INTO panel_templates(id,slug,title,description,category) VALUES (?,?,?,?,?)`,
			[]any{"panel_legacy", "standup", "Standup", "Daily sync panel", "team"}},
		{`INSERT INTO recipes(id,slug,name,description,category,source) VALUES (?,?,?,?,?,?)`,
			[]any{"rec_legacy", "discovery", "Discovery", "Discovery call recipe", "sales", "user"}},
		{`INSERT INTO recipes_usage(recipe_id,total_count,last_used_at) VALUES (?,?,?)`,
			[]any{"rec_legacy", 17, "2026-01-02T03:04:05Z"}},
	}
	for _, s := range seed {
		if _, err := db.ExecContext(ctx, s.stmt, s.args...); err != nil {
			t.Fatalf("seed legacy row: %v", err)
		}
	}

	if err := EnsureSchema(ctx, db); err != nil {
		t.Fatalf("EnsureSchema on legacy db: %v", err)
	}

	want := map[string][]string{
		"folders":         {"row_source", "description", "is_favourited"},
		"panel_templates": {"row_source"},
		"recipes":         {"row_source"},
		"recipes_usage":   {"row_source"},
	}
	for tbl, cols := range want {
		have := tableColumns(t, db, tbl)
		for _, c := range cols {
			if !have[c] {
				t.Errorf("%s.%s missing after EnsureSchema", tbl, c)
			}
		}
	}

	// No data loss.
	if got := queryString(t, db, `SELECT title FROM folders WHERE id = ?`, "fold_legacy"); got != "Customers" {
		t.Errorf("folder title after migrate = %q, want %q", got, "Customers")
	}
	if got := queryString(t, db, `SELECT preset FROM folders WHERE id = ?`, "fold_legacy"); got != "sales" {
		t.Errorf("folder preset after migrate = %q, want %q", got, "sales")
	}
	if got := queryString(t, db, `SELECT description FROM panel_templates WHERE id = ?`, "panel_legacy"); got != "Daily sync panel" {
		t.Errorf("panel description after migrate = %q", got)
	}
	if got := queryString(t, db, `SELECT name FROM recipes WHERE id = ?`, "rec_legacy"); got != "Discovery" {
		t.Errorf("recipe name after migrate = %q", got)
	}
	if got := queryInt(t, db, `SELECT total_count FROM recipes_usage WHERE recipe_id = ?`, "rec_legacy"); got != 17 {
		t.Errorf("recipe usage total after migrate = %d, want 17", got)
	}

	// Pre-existing rows are cache-owned.
	backfilled := []struct {
		query string
		arg   string
	}{
		{`SELECT row_source FROM folders WHERE id = ?`, "fold_legacy"},
		{`SELECT row_source FROM panel_templates WHERE id = ?`, "panel_legacy"},
		{`SELECT row_source FROM recipes WHERE id = ?`, "rec_legacy"},
		{`SELECT row_source FROM recipes_usage WHERE recipe_id = ?`, "rec_legacy"},
	}
	for _, b := range backfilled {
		if got := queryString(t, db, b.query, b.arg); got != RowSourceCache {
			t.Errorf("%s = %q, want %q", b.query, got, RowSourceCache)
		}
	}
}

// TestEnsureSchema_SecondRunIsNoOp pins idempotency. EnsureSchema runs on
// every command that touches these tables, so a second run that re-added or
// re-declared anything would either error or churn the schema.
func TestEnsureSchema_SecondRunIsNoOp(t *testing.T) {
	ctx := context.Background()
	db := openRawDB(t)
	if err := EnsureSchema(ctx, db); err != nil {
		t.Fatalf("first EnsureSchema: %v", err)
	}
	if _, err := db.ExecContext(ctx,
		`INSERT INTO folders(id,title,description,is_favourited,row_source) VALUES (?,?,?,?,?)`,
		"fold_1", "Customers", "Everything customer facing", 1, RowSourceAPI); err != nil {
		t.Fatalf("seed folder: %v", err)
	}
	before := schemaSnapshot(t, db)

	if err := EnsureSchema(ctx, db); err != nil {
		t.Fatalf("second EnsureSchema: %v", err)
	}
	if after := schemaSnapshot(t, db); after != before {
		t.Errorf("second EnsureSchema changed the schema:\nbefore:\n%s\nafter:\n%s", before, after)
	}
	if got := queryString(t, db, `SELECT row_source FROM folders WHERE id = ?`, "fold_1"); got != RowSourceAPI {
		t.Errorf("row_source after second EnsureSchema = %q, want %q", got, RowSourceAPI)
	}
	if got := queryString(t, db, `SELECT description FROM folders WHERE id = ?`, "fold_1"); got != "Everything customer facing" {
		t.Errorf("description after second EnsureSchema = %q", got)
	}
}

// ---------------------------------------------------------------------------
// Folder metadata on the write path.
// ---------------------------------------------------------------------------

// TestSyncFromCache_WritesFolderDescriptionAndFavourite covers R7. The cache
// carries both fields on every folder and the write path used to drop them, so
// a store-only read reported an empty description and a folder that is never
// favourited no matter what the user set in the desktop app.
func TestSyncFromCache_WritesFolderDescriptionAndFavourite(t *testing.T) {
	db := openTestDB(t)
	cache := &Cache{
		DocumentListsMetadata: map[string]DocumentListMetadata{
			"fold_1": {
				ID:                   "fold_1",
				Title:                "Customers",
				Description:          "Everything customer facing",
				ParentDocumentListID: "fold_root",
				WorkspaceID:          "ws_1",
				Preset:               "sales",
				IsFavourited:         true,
			},
			"fold_2": {ID: "fold_2", Title: "Archive"},
		},
	}
	if _, err := SyncFromCache(context.Background(), db, cache); err != nil {
		t.Fatalf("SyncFromCache: %v", err)
	}

	if got := queryString(t, db, `SELECT description FROM folders WHERE id = ?`, "fold_1"); got != "Everything customer facing" {
		t.Errorf("folders.description = %q, want %q", got, "Everything customer facing")
	}
	if got := queryInt(t, db, `SELECT is_favourited FROM folders WHERE id = ?`, "fold_1"); got != 1 {
		t.Errorf("folders.is_favourited = %d, want 1", got)
	}
	if got := queryInt(t, db, `SELECT is_favourited FROM folders WHERE id = ?`, "fold_2"); got != 0 {
		t.Errorf("unfavourited folder is_favourited = %d, want 0", got)
	}
	if got := queryString(t, db, `SELECT description FROM folders WHERE id = ?`, "fold_2"); got != "" {
		t.Errorf("folder with no description = %q, want empty", got)
	}
}

// ---------------------------------------------------------------------------
// Retirement: no sync path may clear these tables.
// ---------------------------------------------------------------------------

// openDeleteGuardedDB opens a store whose four catalog tables abort on any
// DELETE. recursive_triggers is deliberately ON: SQLite fires delete triggers
// for rows removed by INSERT OR REPLACE only when it is, which is precisely
// the hidden delete this round set out to remove.
func openDeleteGuardedDB(t *testing.T) *sql.DB {
	t.Helper()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "guarded.db")
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=recursive_triggers(1)")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	// Pragmas are per-connection; one connection keeps the guard honest.
	db.SetMaxOpenConns(1)
	var recursive int
	if err := db.QueryRowContext(ctx, `PRAGMA recursive_triggers`).Scan(&recursive); err != nil {
		t.Fatalf("read recursive_triggers: %v", err)
	}
	if recursive != 1 {
		t.Fatalf("recursive_triggers = %d, want 1 (the guard cannot see REPLACE deletes without it)", recursive)
	}
	if err := EnsureSchema(ctx, db); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	for _, tbl := range catalogTables {
		if _, err := db.ExecContext(ctx, fmt.Sprintf(
			`CREATE TRIGGER guard_%s_delete BEFORE DELETE ON %s
			 BEGIN SELECT RAISE(ABORT, 'DELETE executed against %s'); END`, tbl, tbl, tbl)); err != nil {
			t.Fatalf("install delete guard on %s: %v", tbl, err)
		}
	}
	return db
}

// TestSyncPathsNeverDeleteFromCatalogTables is the regression guard for the
// failure a prior fix in this repo was about: an unscoped clear of a shared
// table destroys the other path's rows. These four tables get no retirement
// DELETE at all, and INSERT OR REPLACE is a delete in disguise -- it removes
// the conflicting row and inserts a fresh one, blanking every column the
// incoming payload omits and rewriting row_source to the column default.
func TestSyncPathsNeverDeleteFromCatalogTables(t *testing.T) {
	ctx := context.Background()
	db := openDeleteGuardedDB(t)

	// The guard must actually bite, or the rest of this test proves nothing.
	if _, err := db.ExecContext(ctx, `INSERT INTO folders(id,title) VALUES ('guard_probe','probe')`); err != nil {
		t.Fatalf("seed guard probe: %v", err)
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM folders WHERE id = 'guard_probe'`); err == nil {
		t.Fatalf("delete guard did not fire on a direct DELETE FROM folders")
	}

	cache := &Cache{
		Documents: map[string]Document{
			"doc_1": {ID: "doc_1", Title: "Standup"},
		},
		DocumentLists: map[string][]string{"fold_1": {"doc_1"}},
		DocumentListsMetadata: map[string]DocumentListMetadata{
			"fold_1": {ID: "fold_1", Title: "Customers", Description: "Customer work", IsFavourited: true},
		},
		PanelTemplates: []PanelTemplate{
			{ID: "panel_1", Slug: "standup", Title: "Standup", Description: "Daily sync", Category: "team"},
		},
		UserRecipes: []Recipe{
			{ID: "rec_1", Slug: "discovery", Name: "Discovery", Category: "sales", Source: "user",
				Config: RecipeConfig{Description: "Discovery call recipe"}},
		},
		RecipesUsage: map[string]RecipeUsage{
			"rec_1": {RecipeID: "rec_1", TotalCount: "9", LastUsedAt: "2026-01-02T03:04:05Z"},
		},
	}
	notes := []APINote{{
		ID:               "doc_api",
		Title:            "Roadmap review",
		FolderMembership: []APIMembership{{ID: "fold_1", Title: "Customers"}},
	}}

	// Run every path twice: the second run is the one that conflicts on the
	// primary key, which is where a REPLACE would delete.
	for i := 0; i < 2; i++ {
		if _, err := SyncFromCache(ctx, db, cache); err != nil {
			t.Fatalf("healthy SyncFromCache run %d deleted from a catalog table: %v", i+1, err)
		}
		if _, err := SyncFromCacheWithOptions(ctx, db, cache, SyncOptions{Degraded: true}); err != nil {
			t.Fatalf("degraded SyncFromCache run %d deleted from a catalog table: %v", i+1, err)
		}
		if _, err := SyncFromAPI(ctx, db, notes); err != nil {
			t.Fatalf("SyncFromAPI run %d deleted from a catalog table: %v", i+1, err)
		}
	}
}

// ---------------------------------------------------------------------------
// Provenance.
// ---------------------------------------------------------------------------

// TestSyncFromAPI_StampsFolderRowsAsAPIOwned pins step 4 of this round. The
// API note-sync path already created folder rows before row_source existed on
// this table, so without an explicit stamp the very first backfill would file
// genuine API-created folders under the column default and nothing could tell
// them apart afterwards.
func TestSyncFromAPI_StampsFolderRowsAsAPIOwned(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	notes := []APINote{{
		ID:               "doc_api",
		Title:            "Roadmap review",
		FolderMembership: []APIMembership{{ID: "fold_api", Title: "Roadmap"}},
		SpaceMembership:  []APIMembership{{SpaceID: "space_api", Name: "Product"}},
	}}
	if _, err := SyncFromAPI(ctx, db, notes); err != nil {
		t.Fatalf("SyncFromAPI: %v", err)
	}
	for _, id := range []string{"fold_api", "space_api"} {
		if got := queryString(t, db, `SELECT row_source FROM folders WHERE id = ?`, id); got != RowSourceAPI {
			t.Errorf("folders(%s).row_source = %q, want %q", id, got, RowSourceAPI)
		}
	}
}

// TestCacheCatalogUpsertKeepsForeignOwnershipAndColumns pins the merge
// semantics: an upsert of a row the OTHER path created must leave row_source
// alone (or each run would steal ownership from the last) and must not blank
// columns its own payload does not carry.
func TestCacheCatalogUpsertKeepsForeignOwnershipAndColumns(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	seeds := []struct {
		stmt string
		args []any
	}{
		{`INSERT INTO folders(id,title,parent_id,workspace_id,owner_id,preset,description,is_favourited,row_source)
		  VALUES (?,?,?,?,?,?,?,?,?)`,
			[]any{"fold_1", "Customers", "fold_root", "ws_1", "usr_1", "sales", "Customer work", 1, RowSourceAPI}},
		{`INSERT INTO panel_templates(id,slug,title,description,category,row_source) VALUES (?,?,?,?,?,?)`,
			[]any{"panel_1", "standup", "Standup", "Daily sync", "team", RowSourceAPI}},
		{`INSERT INTO recipes(id,slug,name,description,category,source,row_source) VALUES (?,?,?,?,?,?,?)`,
			[]any{"rec_1", "discovery", "Discovery", "Discovery call recipe", "sales", "user", RowSourceAPI}},
		{`INSERT INTO recipes_usage(recipe_id,total_count,last_used_at,row_source) VALUES (?,?,?,?)`,
			[]any{"rec_1", 42, "2026-01-02T03:04:05Z", RowSourceAPI}},
	}
	for _, s := range seeds {
		if _, err := db.ExecContext(ctx, s.stmt, s.args...); err != nil {
			t.Fatalf("seed api-owned row: %v", err)
		}
	}

	// A cache payload that knows the ids but carries almost nothing else --
	// the shape a partial or trimmed cache produces.
	cache := &Cache{
		DocumentListsMetadata: map[string]DocumentListMetadata{
			"fold_1": {ID: "fold_1", Title: "Customers"},
		},
		PanelTemplates: []PanelTemplate{{ID: "panel_1", Title: "Standup"}},
		UserRecipes:    []Recipe{{ID: "rec_1", Name: "Discovery"}},
		RecipesUsage:   map[string]RecipeUsage{"rec_1": {RecipeID: "rec_1"}},
	}
	if _, err := SyncFromCache(ctx, db, cache); err != nil {
		t.Fatalf("SyncFromCache: %v", err)
	}

	ownership := []struct{ query, arg string }{
		{`SELECT row_source FROM folders WHERE id = ?`, "fold_1"},
		{`SELECT row_source FROM panel_templates WHERE id = ?`, "panel_1"},
		{`SELECT row_source FROM recipes WHERE id = ?`, "rec_1"},
		{`SELECT row_source FROM recipes_usage WHERE recipe_id = ?`, "rec_1"},
	}
	for _, o := range ownership {
		if got := queryString(t, db, o.query, o.arg); got != RowSourceAPI {
			t.Errorf("%s = %q, want %q (the cache upsert stole ownership)", o.query, got, RowSourceAPI)
		}
	}

	preserved := []struct {
		query string
		arg   string
		want  string
	}{
		{`SELECT parent_id FROM folders WHERE id = ?`, "fold_1", "fold_root"},
		{`SELECT workspace_id FROM folders WHERE id = ?`, "fold_1", "ws_1"},
		{`SELECT preset FROM folders WHERE id = ?`, "fold_1", "sales"},
		{`SELECT description FROM folders WHERE id = ?`, "fold_1", "Customer work"},
		{`SELECT slug FROM panel_templates WHERE id = ?`, "panel_1", "standup"},
		{`SELECT description FROM panel_templates WHERE id = ?`, "panel_1", "Daily sync"},
		{`SELECT category FROM panel_templates WHERE id = ?`, "panel_1", "team"},
		{`SELECT slug FROM recipes WHERE id = ?`, "rec_1", "discovery"},
		{`SELECT description FROM recipes WHERE id = ?`, "rec_1", "Discovery call recipe"},
		{`SELECT category FROM recipes WHERE id = ?`, "rec_1", "sales"},
		{`SELECT source FROM recipes WHERE id = ?`, "rec_1", "user"},
		{`SELECT last_used_at FROM recipes_usage WHERE recipe_id = ?`, "rec_1", "2026-01-02T03:04:05Z"},
	}
	for _, p := range preserved {
		if got := queryString(t, db, p.query, p.arg); got != p.want {
			t.Errorf("%s = %q, want %q (the upsert blanked a column its payload omitted)", p.query, got, p.want)
		}
	}
	if got := queryInt(t, db, `SELECT total_count FROM recipes_usage WHERE recipe_id = ?`, "rec_1"); got != 42 {
		t.Errorf("recipes_usage.total_count = %d, want 42 (a zero count blanked the stored total)", got)
	}
}

// TestAPIFolderUpsertKeepsCacheMetadata is the mirror image: the API folder
// insert carries a title and nothing else, and must not blank the metadata
// only the cache has -- nor take ownership of a row the cache created.
func TestAPIFolderUpsertKeepsCacheMetadata(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	cache := &Cache{
		DocumentListsMetadata: map[string]DocumentListMetadata{
			"fold_1": {
				ID: "fold_1", Title: "Customers", Description: "Customer work",
				ParentDocumentListID: "fold_root", WorkspaceID: "ws_1",
				Preset: "sales", IsFavourited: true,
			},
		},
	}
	if _, err := SyncFromCache(ctx, db, cache); err != nil {
		t.Fatalf("SyncFromCache: %v", err)
	}

	notes := []APINote{{
		ID:               "doc_api",
		Title:            "Roadmap review",
		FolderMembership: []APIMembership{{ID: "fold_1", Title: "Customers"}},
	}}
	if _, err := SyncFromAPI(ctx, db, notes); err != nil {
		t.Fatalf("SyncFromAPI: %v", err)
	}

	if got := queryString(t, db, `SELECT row_source FROM folders WHERE id = ?`, "fold_1"); got != RowSourceCache {
		t.Errorf("folders.row_source = %q, want %q (the API upsert stole ownership)", got, RowSourceCache)
	}
	for _, c := range []struct{ col, want string }{
		{"title", "Customers"},
		{"description", "Customer work"},
		{"parent_id", "fold_root"},
		{"workspace_id", "ws_1"},
		{"preset", "sales"},
	} {
		got := queryString(t, db, fmt.Sprintf(`SELECT %s FROM folders WHERE id = ?`, c.col), "fold_1")
		if got != c.want {
			t.Errorf("folders.%s after API sync = %q, want %q", c.col, got, c.want)
		}
	}
	if got := queryInt(t, db, `SELECT is_favourited FROM folders WHERE id = ?`, "fold_1"); got != 1 {
		t.Errorf("folders.is_favourited after API sync = %d, want 1", got)
	}
}

// TestFolderFavouriteFlagIsOwnerScoped pins the one column the
// COALESCE(NULLIF(...)) merge cannot protect. is_favourited is a boolean, so a
// payload that does not carry the field is indistinguishable from one that
// says false; provenance is the only thing that can tell them apart. The
// owning path may clear the flag, a foreign path may not.
func TestFolderFavouriteFlagIsOwnerScoped(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	favourited := &Cache{DocumentListsMetadata: map[string]DocumentListMetadata{
		"fold_1": {ID: "fold_1", Title: "Customers", IsFavourited: true},
	}}
	notFavourited := &Cache{DocumentListsMetadata: map[string]DocumentListMetadata{
		"fold_1": {ID: "fold_1", Title: "Customers"},
	}}

	if _, err := SyncFromCache(ctx, db, favourited); err != nil {
		t.Fatalf("SyncFromCache: %v", err)
	}
	if got := queryInt(t, db, `SELECT is_favourited FROM folders WHERE id = ?`, "fold_1"); got != 1 {
		t.Fatalf("is_favourited after cache sync = %d, want 1", got)
	}

	// A foreign owner that does not carry the flag must leave it alone.
	if _, err := SyncFromCacheWithOptions(ctx, db, notFavourited, SyncOptions{CatalogOwner: RowSourceAPI}); err != nil {
		t.Fatalf("SyncFromCacheWithOptions: %v", err)
	}
	if got := queryInt(t, db, `SELECT is_favourited FROM folders WHERE id = ?`, "fold_1"); got != 1 {
		t.Errorf("is_favourited after a foreign-owner write = %d, want 1 (it was blanked)", got)
	}

	// The owning path still clears it, or un-favouriting would never stick.
	if _, err := SyncFromCache(ctx, db, notFavourited); err != nil {
		t.Fatalf("second SyncFromCache: %v", err)
	}
	if got := queryInt(t, db, `SELECT is_favourited FROM folders WHERE id = ?`, "fold_1"); got != 0 {
		t.Errorf("is_favourited after the owner un-favourited = %d, want 0", got)
	}
}

// TestSyncOptionsCatalogOwner pins the seam the API refresh needs. The default
// is the historical behavior (cache), and an explicit owner stamps every
// catalog table this run creates, so an API-driven catalog refresh can own its
// own rows without a literal at each write site.
func TestSyncOptionsCatalogOwner(t *testing.T) {
	ctx := context.Background()

	if got := (SyncOptions{}).catalogOwner(); got != RowSourceCache {
		t.Errorf("zero SyncOptions catalogOwner = %q, want %q", got, RowSourceCache)
	}
	if got := (SyncOptions{CatalogOwner: RowSourceAPI}).catalogOwner(); got != RowSourceAPI {
		t.Errorf("explicit catalogOwner = %q, want %q", got, RowSourceAPI)
	}

	db := openTestDB(t)
	cache := &Cache{
		DocumentListsMetadata: map[string]DocumentListMetadata{
			"fold_1": {ID: "fold_1", Title: "Customers"},
		},
		PanelTemplates: []PanelTemplate{{ID: "panel_1", Slug: "standup", Title: "Standup"}},
		UserRecipes:    []Recipe{{ID: "rec_1", Slug: "discovery", Name: "Discovery"}},
		RecipesUsage:   map[string]RecipeUsage{"rec_1": {RecipeID: "rec_1", TotalCount: "3"}},
	}
	if _, err := SyncFromCacheWithOptions(ctx, db, cache, SyncOptions{CatalogOwner: RowSourceAPI}); err != nil {
		t.Fatalf("SyncFromCacheWithOptions: %v", err)
	}
	for _, o := range []struct{ query, arg string }{
		{`SELECT row_source FROM folders WHERE id = ?`, "fold_1"},
		{`SELECT row_source FROM panel_templates WHERE id = ?`, "panel_1"},
		{`SELECT row_source FROM recipes WHERE id = ?`, "rec_1"},
		{`SELECT row_source FROM recipes_usage WHERE recipe_id = ?`, "rec_1"},
	} {
		if got := queryString(t, db, o.query, o.arg); got != RowSourceAPI {
			t.Errorf("%s = %q, want %q", o.query, got, RowSourceAPI)
		}
	}

	// A later default (cache) run must not take ownership back.
	if _, err := SyncFromCache(ctx, db, cache); err != nil {
		t.Fatalf("second SyncFromCache: %v", err)
	}
	if got := queryString(t, db, `SELECT row_source FROM folders WHERE id = ?`, "fold_1"); got != RowSourceAPI {
		t.Errorf("row_source after default-owner rerun = %q, want %q", got, RowSourceAPI)
	}
}
