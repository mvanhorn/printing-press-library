// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/granola/internal/granola"
)

// PATCH(dual-path-store-read): regressions for the store-read seam.
//
// The user-visible requirement these guard is narrow and literal: reading a
// transcript that is already in the local store must work with and without a
// GRANOLA_API_KEY, and must never depend on the desktop cache being
// decryptable. Every test here therefore points GRANOLA_CACHE_PATH at a file
// that does not exist, which is the closest hermetic analogue of the real
// failure (a cache-v6.json.enc encrypted under a migrated key).

type fixtureSeg struct {
	source  string
	text    string
	startMs int64
	endMs   int64
}

// newGranolaFixture isolates HOME (so defaultDBPath resolves into a temp
// dir) and makes the desktop cache unreadable. Returns the store path.
func newGranolaFixture(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GRANOLA_CACHE_PATH", filepath.Join(home, "no-such-cache.json"))
	return filepath.Join(home, ".local", "share", "granola-pp-cli", "data.db")
}

// seedStore writes one meeting plus its segments into a fresh store at
// dbPath, using the same schema the sync commands write.
func seedStore(t *testing.T, dbPath, id, title string, segs []fixtureSeg) {
	t.Helper()
	ctx := context.Background()
	s, err := openGranolaStoreAt(ctx, dbPath)
	if err != nil {
		t.Fatalf("openGranolaStoreAt: %v", err)
	}
	defer s.Close()
	if _, err := s.DB().ExecContext(ctx,
		`INSERT INTO meetings(id,title,created_at,updated_at,started_at,ended_at,workspace_id,calendar_event_id,deleted_at,notes_markdown,notes_plain,transcript_available,recipes_applied,creation_source,valid_meeting,row_source)
		 VALUES (?,?,?,?,?,?,'','','','','',1,'','api',1,'api')`,
		id, title,
		"2026-05-01T10:00:00.000Z", "2026-05-01T11:00:00.000Z",
		"2026-05-01T10:00:00.000Z", "2026-05-01T11:00:00.000Z",
	); err != nil {
		t.Fatalf("insert meeting: %v", err)
	}
	for i, seg := range segs {
		if _, err := s.DB().ExecContext(ctx,
			`INSERT INTO transcript_segments(meeting_id,idx,source,text,start_ts_ms,end_ts_ms,confidence,speaker_name,diarization_label,row_source)
			 VALUES (?,?,?,?,?,?,0,NULL,NULL,'api')`,
			id, i, seg.source, seg.text, seg.startMs, seg.endMs,
		); err != nil {
			t.Fatalf("insert segment %d: %v", i, err)
		}
	}
}

// runCLI executes the real command tree with auto-refresh disabled so the
// test observes only the read path.
func runCLI(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	rc := RootCmd()
	rc.SetOut(&out)
	rc.SetErr(&out)
	rc.SilenceUsage = true
	rc.SilenceErrors = true
	rc.SetArgs(append(args, "--no-refresh"))
	err := rc.Execute()
	return out.String(), err
}

// TestTranscriptGet_ServedFromStore_NoCacheNoAPIKey is the headline
// acceptance case: segments hydrated by the API sync are readable with no
// decryptable cache and no credential of any kind.
func TestTranscriptGet_ServedFromStore_NoCacheNoAPIKey(t *testing.T) {
	db := newGranolaFixture(t)
	t.Setenv("GRANOLA_API_KEY", "")
	seedStore(t, db, "not_store1", "Store Meeting", []fixtureSeg{
		{source: "microphone", text: "hello there", startMs: 1_700_000_000_000, endMs: 1_700_000_005_000},
		{source: "system", text: "general kenobi", startMs: 1_700_000_005_000, endMs: 1_700_000_010_000},
		{source: "microphone", text: "third line", startMs: 1_700_000_010_000, endMs: 1_700_000_015_000},
	})

	out, err := runCLI(t, "transcript", "get", "not_store1", "--format", "text")
	if err != nil {
		t.Fatalf("transcript get: %v (out=%q)", err, out)
	}
	for _, want := range []string{"hello there", "general kenobi", "third line"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	if got := len(strings.Fields(strings.ReplaceAll(strings.TrimSpace(out), "\n", "|"))); got == 0 {
		t.Errorf("empty output")
	}
	// Order must follow idx, not insertion luck.
	if i, j := strings.Index(out, "hello there"), strings.Index(out, "general kenobi"); i > j {
		t.Errorf("segments out of idx order:\n%s", out)
	}
}

// TestTranscriptGet_APIKeyDoesNotChangeStoreRead pins the other half of the
// dual-path requirement: a configured key must not divert or alter a read
// that the store can already serve.
func TestTranscriptGet_APIKeyDoesNotChangeStoreRead(t *testing.T) {
	db := newGranolaFixture(t)
	seedStore(t, db, "not_store2", "Store Meeting", []fixtureSeg{
		{source: "microphone", text: "line one", startMs: 1_700_000_000_000, endMs: 1_700_000_002_000},
		{source: "system", text: "line two", startMs: 1_700_000_002_000, endMs: 1_700_000_004_000},
	})

	t.Setenv("GRANOLA_API_KEY", "")
	withoutKey, err := runCLI(t, "transcript", "get", "not_store2", "--json")
	if err != nil {
		t.Fatalf("without key: %v (out=%q)", err, withoutKey)
	}
	t.Setenv("GRANOLA_API_KEY", "grn_some_key")
	withKey, err := runCLI(t, "transcript", "get", "not_store2", "--json")
	if err != nil {
		t.Fatalf("with key: %v (out=%q)", err, withKey)
	}
	if withoutKey != withKey {
		t.Errorf("API key changed a store read:\nwithout:\n%s\nwith:\n%s", withoutKey, withKey)
	}

	var envelope struct {
		DocumentID string                      `json:"document_id"`
		Source     string                      `json:"source"`
		Segments   []granola.TranscriptSegment `json:"segments"`
	}
	if err := json.Unmarshal([]byte(withKey), &envelope); err != nil {
		t.Fatalf("unmarshal envelope: %v (out=%q)", err, withKey)
	}
	if envelope.DocumentID != "not_store2" {
		t.Errorf("document_id = %q", envelope.DocumentID)
	}
	if envelope.Source != "store" {
		t.Errorf("source = %q, want %q", envelope.Source, "store")
	}
	if len(envelope.Segments) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(envelope.Segments))
	}
	if envelope.Segments[0].Text != "line one" || envelope.Segments[1].Source != "system" {
		t.Errorf("segments not reconstructed faithfully: %+v", envelope.Segments)
	}
}

// TestTalktime_StoreSegments_ReportsMicAndSystemSeconds guards the
// normalized-source vocabulary: sourceSeconds only counts
// microphone/mic and system/speakers, so a store round-trip that mangled
// either the label or the timestamps would silently report zero.
func TestTalktime_StoreSegments_ReportsMicAndSystemSeconds(t *testing.T) {
	db := newGranolaFixture(t)
	t.Setenv("GRANOLA_API_KEY", "")
	seedStore(t, db, "not_store3", "Talky", []fixtureSeg{
		{source: "microphone", text: "a", startMs: 1_700_000_000_000, endMs: 1_700_000_030_000},
		{source: "microphone", text: "b", startMs: 1_700_000_030_000, endMs: 1_700_000_060_000},
		{source: "system", text: "c", startMs: 1_700_000_060_000, endMs: 1_700_000_105_000},
		{source: "system", text: "d", startMs: 1_700_000_105_000, endMs: 1_700_000_150_000},
	})

	out, err := runCLI(t, "talktime", "not_store3")
	if err != nil {
		t.Fatalf("talktime: %v (out=%q)", err, out)
	}
	var agg map[string]any
	if err := json.Unmarshal([]byte(out), &agg); err != nil {
		t.Fatalf("unmarshal talktime: %v (out=%q)", err, out)
	}
	mic, _ := agg["microphone_seconds"].(float64)
	sys, _ := agg["system_seconds"].(float64)
	if mic != 60 {
		t.Errorf("microphone_seconds = %v, want 60", mic)
	}
	if sys != 90 {
		t.Errorf("system_seconds = %v, want 90", sys)
	}
	if segs, _ := agg["segment_count"].(float64); segs != 4 {
		t.Errorf("segment_count = %v, want 4", segs)
	}
}

// TestTranscriptGet_NoLocalData_SuggestsSync asserts the failure mode the
// user actually hit: with nothing local and no readable cache, the message
// must point at sync rather than leaking the safestorage refusal.
func TestTranscriptGet_NoLocalData_SuggestsSync(t *testing.T) {
	newGranolaFixture(t) // no store seeded, no readable cache
	t.Setenv("GRANOLA_API_KEY", "")

	_, _, err := loadTranscript(context.Background(), "not_missing", "")
	if err == nil {
		t.Fatal("expected an error with no store and no cache")
	}
	msg := err.Error()
	if !strings.Contains(msg, "sync") {
		t.Errorf("error should point at sync, got %q", msg)
	}
	if strings.Contains(msg, "safestorage") || strings.Contains(msg, "refresh refused") {
		t.Errorf("error leaked the raw safestorage failure: %q", msg)
	}
	var ce *cliError
	if !As(err, &ce) || ce.code != 3 {
		t.Errorf("expected a not-found cliError (code 3), got %#v", err)
	}
}

// TestTranscriptGet_MeetingWithNoSegments_ReportsNoTranscript covers a
// meeting the store knows about but has no transcript for: that is an
// answer, not a crash.
func TestTranscriptGet_MeetingWithNoSegments_ReportsNoTranscript(t *testing.T) {
	db := newGranolaFixture(t)
	t.Setenv("GRANOLA_API_KEY", "")
	seedStore(t, db, "not_silent", "Silent Meeting", nil)

	_, _, err := loadTranscript(context.Background(), "not_silent", "")
	if err == nil {
		t.Fatal("expected a not-found error for a meeting with zero segments")
	}
	if !strings.Contains(err.Error(), "no transcript") {
		t.Errorf("expected a 'no transcript' message, got %q", err.Error())
	}
	var ce *cliError
	if !As(err, &ce) || ce.code != 3 {
		t.Errorf("expected a not-found cliError (code 3), got %#v", err)
	}
}

// TestTranscriptGet_CacheFallback_WhenStoreEmpty keeps legacy installs
// working: a readable cache still answers when the store has no row.
func TestTranscriptGet_CacheFallback_WhenStoreEmpty(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GRANOLA_API_KEY", "")
	cachePath := filepath.Join(home, "cache.json")
	cache := buildSyntheticCache(map[string]docFixture{
		"legacy": {Title: "Legacy", CreatedAt: "2026-05-01T10:00:00Z", HasTranscript: true},
	})
	data, _ := json.Marshal(cache)
	if err := os.WriteFile(cachePath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GRANOLA_CACHE_PATH", cachePath)

	segs, source, err := loadTranscript(context.Background(), "legacy", "")
	if err != nil {
		t.Fatalf("loadTranscript: %v", err)
	}
	if source != "cache" {
		t.Errorf("source = %q, want %q", source, "cache")
	}
	if len(segs) == 0 {
		t.Fatal("expected the cache fallback to return segments")
	}
}

// TestGranolaRead_StorePrecedesCache pins the precedence rule: when both
// paths hold a transcript for the same meeting, the store wins.
func TestGranolaRead_StorePrecedesCache(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GRANOLA_API_KEY", "")
	cachePath := filepath.Join(home, "cache.json")
	cache := buildSyntheticCache(map[string]docFixture{
		"dual": {Title: "Dual", CreatedAt: "2026-05-01T10:00:00Z", HasTranscript: true},
	})
	data, _ := json.Marshal(cache)
	if err := os.WriteFile(cachePath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GRANOLA_CACHE_PATH", cachePath)
	seedStore(t, filepath.Join(home, ".local", "share", "granola-pp-cli", "data.db"),
		"dual", "Dual From Store", []fixtureSeg{
			{source: "microphone", text: "store wins", startMs: 1_700_000_000_000, endMs: 1_700_000_001_000},
		})

	segs, source, err := loadTranscript(context.Background(), "dual", "")
	if err != nil {
		t.Fatalf("loadTranscript: %v", err)
	}
	if source != "store" {
		t.Errorf("source = %q, want %q", source, "store")
	}
	if len(segs) != 1 || segs[0].Text != "store wins" {
		t.Errorf("expected the store transcript, got %+v", segs)
	}

	// The merged document keeps the cache's fields while the store's title wins.
	v, err := openGranolaRead(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()
	d := v.DocumentByID("dual")
	if d == nil {
		t.Fatal("merged document missing")
	}
	if d.Title != "Dual From Store" {
		t.Errorf("store title should win, got %q", d.Title)
	}
}

// --- recipe + chat store readers -------------------------------------------
//
// The four tables below (recipes, recipes_usage, chat_threads, chat_messages)
// are written on every cache sync and, before these readers existed, read by
// nothing. The tests pin two different merge shapes on purpose: recipes merge
// field-by-field (so cache-only config.instructions survives a store row),
// while chat threads/messages fall back all-or-nothing (there is no
// field-level merge to do).

// withStoreDB opens (creating on first use) the Granola store at dbPath and
// hands fn the raw handle. Mirrors seedStore for the tables the recipe and
// chat readers consume.
func withStoreDB(t *testing.T, dbPath string, fn func(ctx context.Context, db *sql.DB)) {
	t.Helper()
	ctx := context.Background()
	s, err := openGranolaStoreAt(ctx, dbPath)
	if err != nil {
		t.Fatalf("openGranolaStoreAt: %v", err)
	}
	defer s.Close()
	fn(ctx, s.DB())
}

// seedRecipe writes one recipes row with the exact column shape the cache
// sync writes.
func seedRecipe(t *testing.T, dbPath, id, slug, name, description, category, source string) {
	t.Helper()
	withStoreDB(t, dbPath, func(ctx context.Context, db *sql.DB) {
		if _, err := db.ExecContext(ctx,
			`INSERT OR REPLACE INTO recipes(id,slug,name,description,category,source) VALUES (?,?,?,?,?,?)`,
			id, slug, name, description, category, source); err != nil {
			t.Fatalf("insert recipe %s: %v", id, err)
		}
	})
}

// seedRecipeUsage writes a recipes_usage row. total_count is an INTEGER
// column, which is the whole reason the reader has to stringify it back.
func seedRecipeUsage(t *testing.T, dbPath, recipeID string, total int64, lastUsedAt string) {
	t.Helper()
	withStoreDB(t, dbPath, func(ctx context.Context, db *sql.DB) {
		if _, err := db.ExecContext(ctx,
			`INSERT OR REPLACE INTO recipes_usage(recipe_id,total_count,last_used_at) VALUES (?,?,?)`,
			recipeID, total, lastUsedAt); err != nil {
			t.Fatalf("insert recipe usage %s: %v", recipeID, err)
		}
	})
}

func seedChatThread(t *testing.T, dbPath, id, meetingID, workspaceID, title, createdAt, updatedAt string) {
	t.Helper()
	withStoreDB(t, dbPath, func(ctx context.Context, db *sql.DB) {
		if _, err := db.ExecContext(ctx,
			`INSERT OR REPLACE INTO chat_threads(id,meeting_id,workspace_id,title,created_at,updated_at) VALUES (?,?,?,?,?,?)`,
			id, meetingID, workspaceID, title, createdAt, updatedAt); err != nil {
			t.Fatalf("insert chat thread %s: %v", id, err)
		}
	})
}

func seedChatMessage(t *testing.T, dbPath, id, threadID, role string, turnIndex int, content, createdAt string) {
	t.Helper()
	withStoreDB(t, dbPath, func(ctx context.Context, db *sql.DB) {
		if _, err := db.ExecContext(ctx,
			`INSERT OR REPLACE INTO chat_messages(id,thread_id,role,turn_index,content,created_at) VALUES (?,?,?,?,?,?)`,
			id, threadID, role, turnIndex, content, createdAt); err != nil {
			t.Fatalf("insert chat message %s: %v", id, err)
		}
	})
}

// writeStateCache writes a v6-shaped cache carrying an arbitrary state block
// and points GRANOLA_CACHE_PATH at it. buildSyntheticCache only knows about
// documents and transcripts; recipe and chat state needs its own keys.
func writeStateCache(t *testing.T, dir string, state map[string]any) {
	t.Helper()
	path := filepath.Join(dir, "cache.json")
	data, err := json.Marshal(map[string]any{
		"cache": map[string]any{"version": 6, "state": state},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GRANOLA_CACHE_PATH", path)
}

// recipeByID finds one recipe in the merged list.
func recipeByID(recipes []granola.Recipe, id string) *granola.Recipe {
	for i := range recipes {
		if recipes[i].ID == id {
			return &recipes[i]
		}
	}
	return nil
}

// openRead is the boilerplate every reader test repeats.
func openRead(t *testing.T) *granolaRead {
	t.Helper()
	v, err := openGranolaRead(context.Background())
	if err != nil {
		t.Fatalf("openGranolaRead: %v", err)
	}
	t.Cleanup(v.Close)
	return v
}

// TestGranolaRead_Recipes_ServedFromStore_NoCache is the headline case for
// R1: recipes hydrated into the store answer with no readable cache, with
// the derived Source/Name fields LoadCache would have stamped.
func TestGranolaRead_Recipes_ServedFromStore_NoCache(t *testing.T) {
	db := newGranolaFixture(t)
	t.Setenv("GRANOLA_API_KEY", "")
	seedRecipe(t, db, "rec_a", "action-items", "", "Pull the action items", "meeting", "public")
	seedRecipe(t, db, "rec_b", "weekly-sync", "Weekly Sync", "Recap the week", "", "user")

	recipes := openRead(t).Recipes()
	if len(recipes) != 2 {
		t.Fatalf("expected 2 recipes from the store, got %d (%+v)", len(recipes), recipes)
	}
	a := recipeByID(recipes, "rec_a")
	if a == nil {
		t.Fatal("rec_a missing from the merged recipe list")
	}
	if a.Source != "public" {
		t.Errorf("rec_a source = %q, want %q", a.Source, "public")
	}
	// LoadCache defaults Name to Slug; a store row with an empty name column
	// must arrive with the same default applied.
	if a.Name != "action-items" {
		t.Errorf("rec_a name = %q, want the slug default %q", a.Name, "action-items")
	}
	if a.Config.Description != "Pull the action items" {
		t.Errorf("rec_a description = %q", a.Config.Description)
	}
	if a.Category != "meeting" {
		t.Errorf("rec_a category = %q", a.Category)
	}
	b := recipeByID(recipes, "rec_b")
	if b == nil {
		t.Fatal("rec_b missing from the merged recipe list")
	}
	if b.Name != "Weekly Sync" {
		t.Errorf("rec_b name = %q, want the stored name", b.Name)
	}
	if b.Source != "user" {
		t.Errorf("rec_b source = %q, want %q", b.Source, "user")
	}
}

// TestGranolaRead_Recipes_CacheOnlyFieldsSurviveMerge is the R2 guard: the
// store has no instructions column, so a store row must not blank the
// instructions a readable cache still carries.
func TestGranolaRead_Recipes_CacheOnlyFieldsSurviveMerge(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GRANOLA_API_KEY", "")
	writeStateCache(t, home, map[string]any{
		"userRecipes": []map[string]any{{
			"id":         "rec_a",
			"slug":       "weekly-sync",
			"visibility": "workspace",
			"config": map[string]any{
				"description":  "Cache description",
				"instructions": "Cache instructions",
			},
		}},
	})
	db := filepath.Join(home, ".local", "share", "granola-pp-cli", "data.db")
	seedRecipe(t, db, "rec_a", "weekly-sync", "Weekly Sync", "Store description", "meeting", "user")

	recipes := openRead(t).Recipes()
	if len(recipes) != 1 {
		t.Fatalf("cache and store rows for one recipe must merge into one, got %d (%+v)", len(recipes), recipes)
	}
	r := recipes[0]
	if r.Config.Instructions != "Cache instructions" {
		t.Errorf("cache-only instructions were lost: %q", r.Config.Instructions)
	}
	if r.Visibility != "workspace" {
		t.Errorf("cache-only visibility was lost: %q", r.Visibility)
	}
	if r.Config.Description != "Store description" {
		t.Errorf("store description should win, got %q", r.Config.Description)
	}
	if r.Name != "Weekly Sync" {
		t.Errorf("store name should win, got %q", r.Name)
	}
	if r.Source != "user" {
		t.Errorf("source = %q, want %q", r.Source, "user")
	}
}

// TestGranolaRead_Recipes_EmptyStoreValuesDoNotBlankCache pins the other
// half of the field-level merge: an empty store column is "no answer", not
// an instruction to erase.
func TestGranolaRead_Recipes_EmptyStoreValuesDoNotBlankCache(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GRANOLA_API_KEY", "")
	writeStateCache(t, home, map[string]any{
		"publicRecipes": []map[string]any{{
			"id":   "rec_a",
			"slug": "cache-slug",
			"config": map[string]any{
				"description":  "Cache description",
				"instructions": "Cache instructions",
			},
		}},
	})
	db := filepath.Join(home, ".local", "share", "granola-pp-cli", "data.db")
	seedRecipe(t, db, "rec_a", "", "", "", "", "")

	recipes := openRead(t).Recipes()
	if len(recipes) != 1 {
		t.Fatalf("expected 1 merged recipe, got %d (%+v)", len(recipes), recipes)
	}
	r := recipes[0]
	if r.Name != "cache-slug" {
		t.Errorf("empty store name blanked the cache name: %q", r.Name)
	}
	if r.Slug != "cache-slug" {
		t.Errorf("empty store slug blanked the cache slug: %q", r.Slug)
	}
	if r.Config.Description != "Cache description" {
		t.Errorf("empty store description blanked the cache description: %q", r.Config.Description)
	}
	if r.Source != "public" {
		t.Errorf("empty store source blanked the cache-derived source: %q", r.Source)
	}
}

// TestGranolaRead_RecipeUsage_TotalCountRoundTripsAsString covers the type
// mismatch that would otherwise read as zero: the column is an INTEGER, the
// typed field is the cache's string, and both consumers Sscanf it.
func TestGranolaRead_RecipeUsage_TotalCountRoundTripsAsString(t *testing.T) {
	db := newGranolaFixture(t)
	t.Setenv("GRANOLA_API_KEY", "")
	seedRecipe(t, db, "rec_a", "action-items", "", "", "", "public")
	seedRecipeUsage(t, db, "rec_a", 42, "2026-05-01T10:00:00.000Z")

	usage := openRead(t).RecipesUsage()
	u, ok := usage["rec_a"]
	if !ok {
		t.Fatalf("usage for rec_a missing: %+v", usage)
	}
	if u.TotalCount != "42" {
		t.Errorf("total_count = %q, want %q", u.TotalCount, "42")
	}
	if u.LastUsedAt != "2026-05-01T10:00:00.000Z" {
		t.Errorf("last_used_at = %q", u.LastUsedAt)
	}
	var n int64
	if _, err := fmt.Sscanf(u.TotalCount, "%d", &n); err != nil {
		t.Fatalf("Sscanf(%q): %v", u.TotalCount, err)
	}
	if n != 42 {
		t.Errorf("Sscanf yielded %d, want 42", n)
	}
}

// TestGranolaRead_ChatMessages_OrderedByTurnIndex: insertion order is not
// turn order, and the readers must not leak that.
func TestGranolaRead_ChatMessages_OrderedByTurnIndex(t *testing.T) {
	db := newGranolaFixture(t)
	t.Setenv("GRANOLA_API_KEY", "")
	seedChatThread(t, db, "thr_1", "not_m1", "ws_1", "Thread One",
		"2026-05-01T10:00:00.000Z", "2026-05-01T10:05:00.000Z")
	seedChatMessage(t, db, "msg_c", "thr_1", "assistant", 2, "third", "2026-05-01T10:02:00.000Z")
	seedChatMessage(t, db, "msg_a", "thr_1", "user", 0, "first", "2026-05-01T10:00:00.000Z")
	seedChatMessage(t, db, "msg_b", "thr_1", "assistant", 1, "second", "2026-05-01T10:01:00.000Z")
	// A second thread's messages must not bleed into the first.
	seedChatThread(t, db, "thr_2", "not_m2", "ws_1", "Thread Two", "", "")
	seedChatMessage(t, db, "msg_other", "thr_2", "user", 0, "elsewhere", "")

	v := openRead(t)
	threads := v.ChatThreads()
	th, ok := threads["thr_1"]
	if !ok {
		t.Fatalf("thr_1 missing from the store-served threads: %+v", threads)
	}
	if th.Data.Title != "Thread One" || th.Data.DocumentID != "not_m1" || th.WorkspaceID != "ws_1" {
		t.Errorf("thread not reconstructed faithfully: %+v", th)
	}
	if th.CreatedAt != "2026-05-01T10:00:00.000Z" || th.UpdatedAt != "2026-05-01T10:05:00.000Z" {
		t.Errorf("thread timestamps not reconstructed: %+v", th)
	}

	msgs := v.ChatThreadMessages("thr_1")
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages for thr_1, got %d (%+v)", len(msgs), msgs)
	}
	for i, want := range []string{"first", "second", "third"} {
		if msgs[i].Data.RawText != want {
			t.Errorf("message %d = %q, want %q (turn order broken: %+v)", i, msgs[i].Data.RawText, want, msgs)
		}
		if msgs[i].Data.TurnIndex != i {
			t.Errorf("message %d turn_index = %d", i, msgs[i].Data.TurnIndex)
		}
	}
	if msgs[0].Data.Role != "user" || msgs[1].Data.Role != "assistant" {
		t.Errorf("roles not reconstructed: %+v", msgs)
	}
	if msgs[0].ID != "msg_a" {
		t.Errorf("message id not reconstructed: %q", msgs[0].ID)
	}
	if all := v.ChatMessages(); len(all) != 4 {
		t.Errorf("ChatMessages() = %d messages, want 4", len(all))
	}
}

// TestGranolaRead_ChatThreadWithoutMessages_ReturnsEmptyList: a thread
// nobody replied in is an answer, not an error.
func TestGranolaRead_ChatThreadWithoutMessages_ReturnsEmptyList(t *testing.T) {
	db := newGranolaFixture(t)
	t.Setenv("GRANOLA_API_KEY", "")
	seedChatThread(t, db, "thr_empty", "not_m1", "ws_1", "Silent Thread", "2026-05-01T10:00:00.000Z", "")

	v := openRead(t)
	if _, ok := v.ChatThreads()["thr_empty"]; !ok {
		t.Fatal("thread with no messages should still be listed")
	}
	msgs := v.ChatThreadMessages("thr_empty")
	if msgs == nil {
		t.Fatal("expected an empty slice, got nil")
	}
	if len(msgs) != 0 {
		t.Errorf("expected no messages, got %+v", msgs)
	}
}

// TestGranolaRead_Chat_CacheFallback_WhenStoreEmpty keeps legacy installs
// working: chat is all-or-nothing, so an empty store defers to the cache.
func TestGranolaRead_Chat_CacheFallback_WhenStoreEmpty(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GRANOLA_API_KEY", "")
	writeStateCache(t, home, map[string]any{
		"entities": map[string]any{
			"chat_thread": map[string]any{
				"thr_1": map[string]any{
					"id":           "thr_1",
					"type":         "chat_thread",
					"workspace_id": "ws_1",
					"created_at":   "2026-05-01T10:00:00.000Z",
					"data": map[string]any{
						"title":       "Cache Thread",
						"document_id": "not_m1",
					},
				},
			},
			"chat_message": map[string]any{
				"msg_b": map[string]any{
					"id":   "msg_b",
					"data": map[string]any{"thread_id": "thr_1", "role": "assistant", "turn_index": 1, "raw_text": "cache second"},
				},
				"msg_a": map[string]any{
					"id":   "msg_a",
					"data": map[string]any{"thread_id": "thr_1", "role": "user", "turn_index": 0, "raw_text": "cache first"},
				},
			},
		},
	})

	v := openRead(t)
	th, ok := v.ChatThreads()["thr_1"]
	if !ok {
		t.Fatalf("cache thread missing: %+v", v.ChatThreads())
	}
	if th.Data.Title != "Cache Thread" {
		t.Errorf("thread title = %q", th.Data.Title)
	}
	msgs := v.ChatThreadMessages("thr_1")
	if len(msgs) != 2 {
		t.Fatalf("expected 2 cache messages, got %d (%+v)", len(msgs), msgs)
	}
	if msgs[0].Data.RawText != "cache first" || msgs[1].Data.RawText != "cache second" {
		t.Errorf("cache messages out of turn order: %+v", msgs)
	}
	if len(v.ChatMessages()) != 2 {
		t.Errorf("ChatMessages() = %d, want 2", len(v.ChatMessages()))
	}
}

// TestGranolaRead_RecipeAndChatReaders_EmptyEverywhere covers the state a
// fresh install sits in between `sync-api` and `sync`: the store exists but
// none of these four tables has a row, and no cache is readable. Every
// accessor must answer empty rather than panic.
func TestGranolaRead_RecipeAndChatReaders_EmptyEverywhere(t *testing.T) {
	db := newGranolaFixture(t)
	t.Setenv("GRANOLA_API_KEY", "")
	withStoreDB(t, db, func(context.Context, *sql.DB) {}) // create the schema, seed nothing

	v := openRead(t)
	if got := v.Recipes(); got == nil || len(got) != 0 {
		t.Errorf("Recipes() = %+v, want an empty slice", got)
	}
	if got := v.RecipesUsage(); got == nil || len(got) != 0 {
		t.Errorf("RecipesUsage() = %+v, want an empty map", got)
	}
	if got := v.ChatThreads(); got == nil || len(got) != 0 {
		t.Errorf("ChatThreads() = %+v, want an empty map", got)
	}
	if got := v.ChatMessages(); got == nil || len(got) != 0 {
		t.Errorf("ChatMessages() = %+v, want an empty map", got)
	}
	if got := v.ChatThreadMessages("nope"); got == nil || len(got) != 0 {
		t.Errorf("ChatThreadMessages() = %+v, want an empty slice", got)
	}
}

// cacheOnlyCallSites is the explicit allowlist for files that legitimately
// still open the desktop cache directly. Every other openGranolaCache()
// call site must have been rerouted through openGranolaRead().
//
// Keep the reason honest: an entry here is a statement that the data simply
// is not in the store, not a note that rerouting was skipped.
// chat.go and recipes.go were retired from this list once they were rerouted
// through openGranolaRead(): the store's recipes/recipes_usage/chat_threads/
// chat_messages tables are populated by the cache sync, so that data outlives
// the cache's decryptability and is not cache-only at read time.
var cacheOnlyCallSites = map[string]string{
	"workspaces.go": "workspaces are cache-only state with an existing live fallback",
	"sync_cache.go": "the cache-to-store sync is the writer; reading the cache is the whole point of the command",
}

// TestOpenGranolaCacheCallSites_AllAccountedFor fails when a new
// openGranolaCache() call site appears without either being rerouted to the
// store seam or being added to the cache-only allowlist above. It also fails
// on a stale allowlist entry, so the list cannot rot in the other direction.
func TestOpenGranolaCacheCallSites_AllAccountedFor(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	found := map[string]int{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "openGranolaCache" {
				found[name]++
			}
			return true
		})
	}

	var unaccounted []string
	for name := range found {
		if _, ok := cacheOnlyCallSites[name]; !ok {
			unaccounted = append(unaccounted, name)
		}
	}
	sort.Strings(unaccounted)
	if len(unaccounted) > 0 {
		t.Errorf("openGranolaCache() call sites are neither rerouted to openGranolaRead() nor on the cache-only allowlist: %v\n"+
			"Reroute them through the store seam, or add them to cacheOnlyCallSites with a reason.", unaccounted)
	}

	var stale []string
	for name := range cacheOnlyCallSites {
		if found[name] == 0 {
			stale = append(stale, name)
		}
	}
	sort.Strings(stale)
	if len(stale) > 0 {
		t.Errorf("cacheOnlyCallSites lists files that no longer call openGranolaCache(): %v", stale)
	}
}
