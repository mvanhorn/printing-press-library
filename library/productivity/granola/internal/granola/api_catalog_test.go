// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0.

package granola

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// The bodies below are the live shapes probed against Granola 7.465.0 on
// 2026-08-03, trimmed to the fields this CLI reads. They are kept as raw JSON
// rather than constructed structs on purpose: the derivations these tests
// pin (source stamping, slug fallback, the stringified counter) exist
// precisely because the wire shape and the typed shape disagree, and building
// the input from the typed shape would test the code against itself.

const liveRecipesBody = `{
  "userRecipes": [
    {"id": "rec_user", "slug": "weekly-sync", "created_at": "2026-07-01",
     "config": {"description": "My weekly sync recipe"}}
  ],
  "sharedRecipes": [
    {"id": "rec_shared", "slug": "team-standup", "publisher_slug": "acme"}
  ],
  "publicRecipes": [
    {"id": "rec_public", "slug": "discovery-call"}
  ],
  "unlistedRecipes": [
    {"id": "rec_unlisted", "slug": "internal-only"}
  ],
  "defaultRecipes": [
    {"id": "rec_default", "slug": "granola-default"}
  ],
  "recipesUsage": {
    "rec_user": {"total_count": 12, "last_used_at": "2026-07-30T10:00:00Z"},
    "rec_public": {"recipe_id": "rec_public", "total_count": "3"}
  }
}`

const livePanelTemplatesBody = `[
  {"id": "tpl_1", "title": "Customer Discovery", "category": "sales",
   "description": "Discovery questions", "is_granola": true},
  {"id": "tpl_2", "title": "Ignored", "slug": "explicit-slug"}
]`

const liveDocumentListsBody = `{"lists": [
  {"id": "list_a", "title": "Customers", "parent_document_list_id": "list_root",
   "preset": "sales", "description": "Customer calls", "is_favourited": true,
   "workspace_id": "ws_1", "document_count": 2,
   "documents": [{"document_id": "m1"}, {"document_id": "m2"}]},
  {"id": "list_b", "title": "Personal",
   "documents": [{"id": "m3"}, {"id": "edge_row_key", "document_id": "m4"}]},
  {"id": "list_empty", "title": "Empty", "documents": []}
]}`

// fakeCatalogClient answers from raw bodies through the same parsers the live
// client uses, so a shape change is caught by these tests rather than only by
// a live account.
type fakeCatalogClient struct {
	recipes string
	panels  string
	lists   string
	recipeErr,
	panelErr,
	listErr error
}

func newFakeCatalogClient() *fakeCatalogClient {
	return &fakeCatalogClient{
		recipes: liveRecipesBody,
		panels:  livePanelTemplatesBody,
		lists:   liveDocumentListsBody,
	}
}

func (f *fakeCatalogClient) GetRecipes() (RecipeCatalog, error) {
	if f.recipeErr != nil {
		return RecipeCatalog{}, f.recipeErr
	}
	return parseRecipeCatalog([]byte(f.recipes))
}

func (f *fakeCatalogClient) GetPanelTemplates() ([]PanelTemplate, error) {
	if f.panelErr != nil {
		return nil, f.panelErr
	}
	return parsePanelTemplates([]byte(f.panels))
}

func (f *fakeCatalogClient) GetDocumentLists() ([]DocumentListMetadata, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return parseDocumentLists([]byte(f.lists))
}

func TestHydrateCatalogNilCache(t *testing.T) {
	if _, err := HydrateCatalogFromAPI(nil, newFakeCatalogClient()); err == nil {
		t.Fatal("expected an error on a nil cache")
	}
}

func TestHydrateCatalogFillsRecipeBucketsAndUsage(t *testing.T) {
	cache := &Cache{}
	res, err := HydrateCatalogFromAPI(cache, newFakeCatalogClient())
	if err != nil {
		t.Fatalf("HydrateCatalogFromAPI: %v", err)
	}

	bySource := map[string][]string{}
	for _, r := range cache.RecipesAll() {
		bySource[r.Source] = append(bySource[r.Source], r.ID)
	}
	for source, wantID := range map[string]string{
		"public": "rec_public",
		"user":   "rec_user",
		"shared": "rec_shared",
	} {
		got := bySource[source]
		if len(got) != 1 || got[0] != wantID {
			t.Errorf("source %q holds %v, want [%s] — a recipe with no Source cannot be filtered by `recipes list --source`",
				source, got, wantID)
		}
	}
	// The two buckets with no cache counterpart stay out, so an API-refreshed
	// install and a cache-synced one agree on what the catalog contains.
	for _, r := range cache.RecipesAll() {
		if r.ID == "rec_unlisted" || r.ID == "rec_default" {
			t.Errorf("%s was hydrated; the cache path has no bucket for it and nothing would ever retire it", r.ID)
		}
	}
	if res.Recipes != 3 {
		t.Errorf("res.Recipes = %d, want 3", res.Recipes)
	}

	// Name is derived, never carried on the wire: without the fallback most
	// rows render blank.
	for _, r := range cache.RecipesAll() {
		if r.Name != r.Slug {
			t.Errorf("recipe %s name = %q, want the slug %q", r.ID, r.Name, r.Slug)
		}
	}

	if len(cache.RecipesUsage) != 2 {
		t.Fatalf("usage entries = %d, want 2", len(cache.RecipesUsage))
	}
	if res.RecipeUsages != 2 {
		t.Errorf("res.RecipeUsages = %d, want 2", res.RecipeUsages)
	}
	// The record does not always repeat the id the dict is keyed by.
	if got := cache.RecipesUsage["rec_user"].RecipeID; got != "rec_user" {
		t.Errorf("usage recipe_id = %q, want it defaulted from the dict key", got)
	}
	if got := cache.RecipesUsage["rec_user"].LastUsedAt; got != "2026-07-30T10:00:00Z" {
		t.Errorf("usage last_used_at = %q", got)
	}
}

// TestRecipeUsageTotalCountSurvivesSscanf is the counter-specific guard: the
// API sends a number, RecipeUsage.TotalCount is a string, and both consumers
// (the recipes_usage write in store_sync.go and `recipes list --top-usage`)
// read it with fmt.Sscanf. Anything but decimal digits reads as zero, silently.
func TestRecipeUsageTotalCountSurvivesSscanf(t *testing.T) {
	cache := &Cache{}
	if _, err := HydrateCatalogFromAPI(cache, newFakeCatalogClient()); err != nil {
		t.Fatalf("HydrateCatalogFromAPI: %v", err)
	}
	for id, want := range map[string]int64{"rec_user": 12, "rec_public": 3} {
		raw := cache.RecipesUsage[id].TotalCount
		var got int64
		if _, err := fmt.Sscanf(raw, "%d", &got); err != nil {
			t.Errorf("%s total_count %q is not Sscanf-readable: %v", id, raw, err)
			continue
		}
		if got != want {
			t.Errorf("%s total_count = %q -> %d, want %d", id, raw, got, want)
		}
	}
}

// A number and a string on the wire must land identically: the desktop cache
// stores this counter as a string and the live API sends it bare, and the CLI
// cannot tell which one a given build will send.
func TestParseRecipesUsageAcceptsNumberAndString(t *testing.T) {
	got := parseRecipesUsage(json.RawMessage(`{"a":{"total_count":7},"b":{"total_count":"7"}}`))
	if got["a"].TotalCount != "7" || got["b"].TotalCount != "7" {
		t.Errorf("total_count number=%q string=%q, want both \"7\"", got["a"].TotalCount, got["b"].TotalCount)
	}
}

func TestHydrateCatalogDerivesPanelTemplateSlug(t *testing.T) {
	cache := &Cache{}
	res, err := HydrateCatalogFromAPI(cache, newFakeCatalogClient())
	if err != nil {
		t.Fatalf("HydrateCatalogFromAPI: %v", err)
	}
	if res.PanelTemplates != 2 || len(cache.PanelTemplates) != 2 {
		t.Fatalf("panel templates = %d (result says %d), want 2", len(cache.PanelTemplates), res.PanelTemplates)
	}
	bySlug := map[string]PanelTemplate{}
	for _, p := range cache.PanelTemplates {
		bySlug[p.Slug] = p
	}
	// The live payload carries no slug at all; `panel --template <slug>`
	// resolves on it, so it has to be derived exactly as LoadCache derives it.
	if got, ok := bySlug["customer-discovery"]; !ok || got.ID != "tpl_1" {
		t.Errorf("blank slug was not derived from the title; slugs present: %v", slugsOf(cache.PanelTemplates))
	}
	// An explicit slug is never overwritten.
	if got, ok := bySlug["explicit-slug"]; !ok || got.ID != "tpl_2" {
		t.Errorf("explicit slug lost; slugs present: %v", slugsOf(cache.PanelTemplates))
	}
}

func slugsOf(ps []PanelTemplate) []string {
	out := make([]string, 0, len(ps))
	for _, p := range ps {
		out = append(out, p.Slug)
	}
	sort.Strings(out)
	return out
}

// TestHydrateCatalogFillsFoldersAndMembershipFromOneCall pins the payload
// property the whole folder refresh depends on: /v2/get-document-lists carries
// each list's membership inline, so folder metadata and folder edges come from
// a single call and no per-folder fan-out is needed.
func TestHydrateCatalogFillsFoldersAndMembershipFromOneCall(t *testing.T) {
	cache := &Cache{}
	res, err := HydrateCatalogFromAPI(cache, newFakeCatalogClient())
	if err != nil {
		t.Fatalf("HydrateCatalogFromAPI: %v", err)
	}
	if res.Folders != 3 || len(cache.DocumentListsMetadata) != 3 {
		t.Fatalf("folders = %d (result says %d), want 3", len(cache.DocumentListsMetadata), res.Folders)
	}
	md := cache.DocumentListsMetadata["list_a"]
	if md.Title != "Customers" || md.ParentDocumentListID != "list_root" || md.Preset != "sales" ||
		md.Description != "Customer calls" || !md.IsFavourited {
		t.Errorf("folder metadata incomplete: %+v", md)
	}
	if got := cache.FolderMeetings("list_a"); len(got) != 2 || got[0] != "m1" || got[1] != "m2" {
		t.Errorf("list_a membership = %v, want [m1 m2]", got)
	}
	// An element that names the document in `id` rather than `document_id`
	// still resolves — and when both appear, `id` is the edge's own key and
	// document_id is the meeting, so taking `id` there would file the
	// membership under an id no meeting has.
	if got := cache.FolderMeetings("list_b"); len(got) != 2 || got[0] != "m3" || got[1] != "m4" {
		t.Errorf("list_b membership = %v, want [m3 m4]", got)
	}
	if got := cache.FolderMeetings("list_empty"); len(got) != 0 {
		t.Errorf("empty folder produced membership %v", got)
	}
	if res.Memberships != 4 {
		t.Errorf("res.Memberships = %d, want 4", res.Memberships)
	}
}

// TestHydrateCatalogSurfacesUnauthorizedPerSurface: the three endpoints are
// unrelated, so one rejecting the credential must leave the other two staged
// and the sync that follows intact.
func TestHydrateCatalogSurfacesUnauthorizedPerSurface(t *testing.T) {
	client := newFakeCatalogClient()
	client.recipeErr = ErrAPIUnauthorized

	cache := &Cache{}
	res, err := HydrateCatalogFromAPI(cache, client)
	if err == nil {
		t.Fatal("a rejected surface must be reported, not swallowed")
	}
	if !errors.Is(err, ErrAPIUnauthorized) || !errors.Is(res.RecipesErr, ErrAPIUnauthorized) {
		t.Errorf("401 lost its classification: joined=%v recipes=%v", err, res.RecipesErr)
	}
	if res.PanelTemplatesErr != nil || res.DocumentListsErr != nil {
		t.Errorf("one failing surface poisoned the others: panels=%v lists=%v",
			res.PanelTemplatesErr, res.DocumentListsErr)
	}
	if len(cache.PanelTemplates) != 2 || len(cache.DocumentListsMetadata) != 3 {
		t.Errorf("healthy surfaces were not staged: panels=%d folders=%d",
			len(cache.PanelTemplates), len(cache.DocumentListsMetadata))
	}
	if len(cache.RecipesAll()) != 0 {
		t.Errorf("the failed surface staged %d recipes", len(cache.RecipesAll()))
	}
}

// TestCatalogShapeErrorsNameTheEndpoint: three calls go out together, so
// "unrecognized response shape" without a name leaves nobody able to tell
// which payload drifted.
func TestCatalogShapeErrorsNameTheEndpoint(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		endpoint string
		call     func([]byte) error
	}{
		{"recipes: array instead of object", `[{"id":"x"}]`, recipesEndpoint,
			func(b []byte) error { _, err := parseRecipeCatalog(b); return err }},
		{"recipes: no known collection", `{"somethingElse":[]}`, recipesEndpoint,
			func(b []byte) error { _, err := parseRecipeCatalog(b); return err }},
		{"panel templates: object with no list", `{"unexpected":1}`, panelTemplatesEndpoint,
			func(b []byte) error { _, err := parsePanelTemplates(b); return err }},
		{"document lists: scalar", `"not a list"`, documentListsEndpoint,
			func(b []byte) error { _, err := parseDocumentLists(b); return err }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.call([]byte(tc.body))
			var shape *APIShapeError
			if !errors.As(err, &shape) {
				t.Fatalf("got %v, want an *APIShapeError", err)
			}
			if shape.Endpoint != tc.endpoint {
				t.Errorf("endpoint = %q, want %q", shape.Endpoint, tc.endpoint)
			}
			if shape.Body == "" {
				t.Error("shape error carries no body sample, so a bug report cannot show the drift")
			}
		})
	}
}

// An empty account still answers with the known keys, and that must not be
// mistaken for drift.
func TestParseRecipeCatalogAcceptsEmptyCollections(t *testing.T) {
	cat, err := parseRecipeCatalog([]byte(`{"userRecipes":[],"publicRecipes":[],"recipesUsage":{}}`))
	if err != nil {
		t.Fatalf("empty-but-known payload rejected: %v", err)
	}
	if len(cat.UserRecipes) != 0 || len(cat.Usage) != 0 {
		t.Errorf("unexpected content: %+v", cat)
	}
}

// TestDocumentListEntriesTolerateAnyShape confirms the claim the Documents
// field rests on: its unmarshaler never errors, so adding it to
// DocumentListMetadata cannot abort LoadCache's single decode of the whole
// documentListsMetadata map.
func TestDocumentListEntriesTolerateAnyShape(t *testing.T) {
	cases := map[string][]string{
		`[{"document_id":"m1"},{"id":"m2"}]`: {"m1", "m2"},
		`["m1","m2"]`:                        {"m1", "m2"},
		`[]`:                                 {},
		`null`:                               {},
		`{"not":"an array"}`:                 {},
		`42`:                                 {},
	}
	for body, want := range cases {
		var got DocumentListEntries
		if err := json.Unmarshal([]byte(body), &got); err != nil {
			t.Fatalf("%s: unmarshaler returned an error, which would abort the whole cache decode: %v", body, err)
		}
		ids := make([]string, 0, len(got))
		for _, e := range got {
			ids = append(ids, e.MeetingID())
		}
		if len(ids) != len(want) {
			t.Errorf("%s -> %v, want %v", body, ids, want)
			continue
		}
		for i := range ids {
			if ids[i] != want[i] {
				t.Errorf("%s -> %v, want %v", body, ids, want)
				break
			}
		}
	}
}

// TestLoadCacheStillParsesFoldersWithDocumentsKey is the other half of that
// claim, at the level the risk actually lives: a cache whose folder records
// carry a `documents` key in an unexpected shape must still yield every
// folder.
func TestLoadCacheStillParsesFoldersWithDocumentsKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache-v6.json")
	body := `{"cache":{"version":6,"state":{"documentListsMetadata":{
		"f1":{"id":"f1","title":"One","documents":["m1"]},
		"f2":{"id":"f2","title":"Two","documents":[{"document_id":"m2"}]},
		"f3":{"id":"f3","title":"Three","documents":{"weird":true}},
		"f4":{"id":"f4","title":"Four"}
	}}}}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := LoadCache(path)
	if err != nil {
		t.Fatalf("LoadCache: %v", err)
	}
	for _, id := range []string{"f1", "f2", "f3", "f4"} {
		if _, ok := c.DocumentListsMetadata[id]; !ok {
			t.Errorf("folder %s was dropped; parsed set: %d folders", id, len(c.DocumentListsMetadata))
		}
	}
}

// TestSyncWritesAPIHydratedCatalogWithAPIOwnership is the end of the unit: the
// hydrator stages, SyncFromCache persists, and the rows this path creates must
// be identifiable as API-created while cache-created rows keep both their
// ownership and the metadata only the cache carries.
func TestSyncWritesAPIHydratedCatalogWithAPIOwnership(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	// The store as a healthy cache sync left it: a folder with metadata the
	// API payload does not carry, and a recipe.
	mustExec(t, db, `INSERT INTO folders(id,title,parent_id,workspace_id,owner_id,preset,description,is_favourited,row_source)
		VALUES ('list_a','Customers','list_cache_parent','ws_1','','cache-preset','from cache',1,?)`, RowSourceCache)
	mustExec(t, db, `INSERT INTO recipes(id,slug,name,source,row_source) VALUES ('rec_cache','cached','Cached','user',?)`, RowSourceCache)

	cache := &Cache{}
	if _, err := HydrateCatalogFromAPI(cache, newFakeCatalogClient()); err != nil {
		t.Fatalf("HydrateCatalogFromAPI: %v", err)
	}
	// The API payload for list_a deliberately carries no parent id below the
	// one the cache supplied; blank it out to model the field the API omits.
	md := cache.DocumentListsMetadata["list_a"]
	md.ParentDocumentListID = ""
	md.Preset = ""
	cache.DocumentListsMetadata["list_a"] = md

	if _, err := SyncFromCacheWithOptions(ctx, db, cache, SyncOptions{
		Degraded:     true,
		CatalogOwner: RowSourceAPI,
	}); err != nil {
		t.Fatalf("SyncFromCacheWithOptions: %v", err)
	}

	// A folder only the API knows about is inserted, and owned by this path.
	if got := scanString(t, db, `SELECT row_source FROM folders WHERE id = 'list_b'`); got != RowSourceAPI {
		t.Errorf("API-created folder row_source = %q, want %q", got, RowSourceAPI)
	}
	// A folder both paths know keeps its cache-supplied metadata and its
	// cache ownership: the API payload omits these fields, it does not clear
	// them.
	if got := scanString(t, db, `SELECT parent_id FROM folders WHERE id = 'list_a'`); got != "list_cache_parent" {
		t.Errorf("parent_id = %q, want the cache-supplied value", got)
	}
	if got := scanString(t, db, `SELECT preset FROM folders WHERE id = 'list_a'`); got != "cache-preset" {
		t.Errorf("preset = %q, want the cache-supplied value", got)
	}
	if got := scanString(t, db, `SELECT row_source FROM folders WHERE id = 'list_a'`); got != RowSourceCache {
		t.Errorf("pre-existing folder row_source = %q, want %q", got, RowSourceCache)
	}

	// The membership edges came from the same call as the metadata.
	// row_source on this table is store_sync's call, not this path's, so only
	// the edges themselves are asserted.
	if got := countRows(t, db, `SELECT COUNT(*) FROM folder_memberships WHERE folder_id = 'list_a'`); got != 2 {
		t.Errorf("list_a memberships = %d, want 2", got)
	}

	if got := scanString(t, db, `SELECT row_source FROM recipes WHERE id = 'rec_user'`); got != RowSourceAPI {
		t.Errorf("API-created recipe row_source = %q, want %q", got, RowSourceAPI)
	}
	if got := scanString(t, db, `SELECT source FROM recipes WHERE id = 'rec_shared'`); got != "shared" {
		t.Errorf("recipe collection source = %q, want %q", got, "shared")
	}
	if got := scanString(t, db, `SELECT row_source FROM recipes WHERE id = 'rec_cache'`); got != RowSourceCache {
		t.Errorf("pre-existing recipe row_source = %q, want %q", got, RowSourceCache)
	}
	if got := scanString(t, db, `SELECT row_source FROM panel_templates WHERE id = 'tpl_1'`); got != RowSourceAPI {
		t.Errorf("panel template row_source = %q, want %q", got, RowSourceAPI)
	}
	if got := scanString(t, db, `SELECT slug FROM panel_templates WHERE id = 'tpl_1'`); got != "customer-discovery" {
		t.Errorf("panel template slug = %q, want the title-derived slug", got)
	}
	// The counter has to arrive as digits or this column silently stores 0.
	if got := countRows(t, db, `SELECT total_count FROM recipes_usage WHERE recipe_id = 'rec_user'`); got != 12 {
		t.Errorf("recipes_usage.total_count = %d, want 12", got)
	}
	if got := scanString(t, db, `SELECT row_source FROM recipes_usage WHERE recipe_id = 'rec_user'`); got != RowSourceAPI {
		t.Errorf("usage row_source = %q, want %q", got, RowSourceAPI)
	}
}

func mustExec(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := db.ExecContext(context.Background(), query, args...); err != nil {
		t.Fatalf("exec (%s): %v", query, err)
	}
}

func scanString(t *testing.T, db *sql.DB, query string, args ...any) string {
	t.Helper()
	var out sql.NullString
	if err := db.QueryRowContext(context.Background(), query, args...).Scan(&out); err != nil {
		t.Fatalf("query (%s): %v", query, err)
	}
	return out.String
}

// TestInternalClientCatalogEndpoints pins the three paths and the parsing the
// live client does, so a rename upstream fails here rather than in a silent
// empty refresh.
func TestInternalClientCatalogEndpoints(t *testing.T) {
	seen := map[string]int{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen[r.URL.Path]++
		switch r.URL.Path {
		case recipesEndpoint:
			_, _ = w.Write([]byte(liveRecipesBody))
		case panelTemplatesEndpoint:
			_, _ = w.Write([]byte(livePanelTemplatesBody))
		case documentListsEndpoint:
			_, _ = w.Write([]byte(liveDocumentListsBody))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"Not implemented"}`))
		}
	}))
	defer srv.Close()

	t.Setenv("GRANOLA_WORKOS_TOKEN", "test-token")
	ResetTokenCache()
	defer ResetTokenCache()
	client, _ := NewInternalClient()
	client.SetBaseURL(srv.URL)

	cache := &Cache{}
	res, err := HydrateCatalogFromAPI(cache, client)
	if err != nil {
		t.Fatalf("HydrateCatalogFromAPI over HTTP: %v", err)
	}
	if res.Recipes != 3 || res.PanelTemplates != 2 || res.Folders != 3 || res.Memberships != 4 {
		t.Errorf("result = %+v", res)
	}
	for _, path := range []string{recipesEndpoint, panelTemplatesEndpoint, documentListsEndpoint} {
		if seen[path] != 1 {
			t.Errorf("%s called %d times, want 1", path, seen[path])
		}
	}
	// One call per surface for the whole set: no budget, no fan-out.
	if seen[documentListsLegacyEndpoint] != 0 {
		t.Errorf("the v1 document-lists fallback fired against a healthy v2")
	}
}

// TestDocumentListsFallbackOnlyOn404: the /v1 surface answers 404 "Not
// implemented", so retrying there on a 401 turned an expired credential into
// a missing-endpoint error and buried the real remedy.
func TestDocumentListsFallbackOnlyOn404(t *testing.T) {
	seen := map[string]int{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen[r.URL.Path]++
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
	}))
	defer srv.Close()

	t.Setenv("GRANOLA_WORKOS_TOKEN", "test-token")
	ResetTokenCache()
	defer ResetTokenCache()
	client, _ := NewInternalClient()
	client.SetBaseURL(srv.URL)

	if _, err := client.GetDocumentLists(); err == nil {
		t.Fatal("expected the 401 to surface")
	}
	if seen[documentListsLegacyEndpoint] != 0 {
		t.Errorf("a 401 retried against the legacy endpoint %d times", seen[documentListsLegacyEndpoint])
	}
}
