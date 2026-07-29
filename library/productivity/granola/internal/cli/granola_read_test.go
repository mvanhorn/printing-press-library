// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
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

// cacheOnlyCallSites is the explicit allowlist for files that legitimately
// still open the desktop cache directly. Every other openGranolaCache()
// call site must have been rerouted through openGranolaRead().
//
// Keep the reason honest: an entry here is a statement that the data simply
// is not in the store, not a note that rerouting was skipped.
var cacheOnlyCallSites = map[string]string{
	"chat.go":       "chat threads and messages are cache-only state; no API sync path fills chat_threads/chat_messages",
	"recipes.go":    "recipes are cache-only state; the store's recipes table is populated from the cache alone",
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
