// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/granola/internal/granola"
)

// ---------------------------------------------------------------------------
// Fixtures.
//
// PII: every name, address, folder title, and transcript line below is
// invented. Nothing here was captured from a live Granola account — this
// repository is public with permanent git history, so recorded API fixtures
// must stay fully synthetic.
// ---------------------------------------------------------------------------

const fixtureNoteAlphaDetail = `{
  "id": "note_alpha",
  "object": "note",
  "title": "Quarterly planning sync",
  "web_url": "https://notes.example.com/note_alpha",
  "owner": {"name": "Ada Placeholder", "email": "ada@example.com"},
  "created_at": "2026-07-01T15:00:00Z",
  "updated_at": "2026-07-01T16:05:00Z",
  "calendar_event": {
    "event_title": "Quarterly planning",
    "invitees": [{"email": "ada@example.com"}, {"email": "bo@example.com"}],
    "organiser": {"email": "ada@example.com"},
    "calendar_event_id": "evt_alpha_001",
    "scheduled_start_time": "2026-07-01T15:00:00Z",
    "scheduled_end_time": "2026-07-01T16:00:00Z"
  },
  "attendees": [
    {"name": "Ada Placeholder", "email": "ada@example.com"},
    {"name": "Bo Sample", "email": "bo@example.com"}
  ],
  "folder_membership": [{"id": "folder_roadmap", "name": "Roadmap"}],
  "space_membership": [],
  "summary_text": "Agreed the quarterly milestones.",
  "summary_markdown": "## Summary\n\nAgreed the quarterly milestones.",
  "transcript": [
    {"text": "Kicking off the planning review.", "start_time": "2026-07-01T15:00:00Z", "end_time": "2026-07-01T15:00:20Z", "speaker": {"source": "microphone", "name": "Ada Placeholder"}},
    {"text": "I have the roadmap open now.", "start_time": "2026-07-01T15:00:20Z", "end_time": "2026-07-01T15:01:20Z", "speaker": {"source": "speaker", "name": "Bo Sample", "diarization_label": "SPEAKER_01"}}
  ]
}`

const fixtureNoteGammaDetail = `{
  "id": "note_gamma",
  "object": "note",
  "title": "Design review",
  "created_at": "2026-07-03T11:00:00Z",
  "updated_at": "2026-07-03T11:45:00Z",
  "attendees": [{"name": "Cleo Fixture", "email": "cleo@example.com"}],
  "folder_membership": [],
  "space_membership": [],
  "transcript": null
}`

// newHydrateEnv points the CLI at an isolated HOME (so the SQLite store and
// config file land in a temp dir) and at a stub public API.
func newHydrateEnv(t *testing.T, h http.HandlerFunc) (*rootFlags, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GRANOLA_BASE_URL", srv.URL)
	t.Setenv("GRANOLA_API_KEY", "grn_test_key")
	t.Setenv("GRANOLA_CONFIG", "")
	return &rootFlags{timeout: 5 * time.Second, noCache: true}, srv
}

// notesListHandler serves a single page of GET /v1/notes containing ids, and
// dispatches GET /v1/notes/{id} to detail.
func notesListHandler(t *testing.T, ids []string, detail map[string]func(w http.ResponseWriter)) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/notes" {
			rows := make([]map[string]any, 0, len(ids))
			for _, id := range ids {
				rows = append(rows, map[string]any{"id": id, "object": "note", "title": id})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"notes": rows, "hasMore": false, "cursor": ""})
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/v1/notes/")
		fn, ok := detail[id]
		if !ok {
			t.Errorf("unexpected detail path %s", r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		fn(w)
	}
}

func writeJSON(body string) func(http.ResponseWriter) {
	return func(w http.ResponseWriter) { _, _ = w.Write([]byte(body)) }
}

func writeStatus(status int, body string) func(http.ResponseWriter) {
	return func(w http.ResponseWriter) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}
}

func openHydratedStore(t *testing.T) *sql.DB {
	t.Helper()
	s, err := openGranolaStore(context.Background())
	if err != nil {
		t.Fatalf("openGranolaStore: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s.DB()
}

// ---------------------------------------------------------------------------

// TestRunAPIHydrate_WritesDomainTablesNotResources is the headline check: the
// hydration lands in the tables the read commands consume (meetings,
// attendees, transcript_segments, folder_memberships), and NOT in the generic
// resources table the generated sync writes and nothing reads.
func TestRunAPIHydrate_WritesDomainTablesNotResources(t *testing.T) {
	flags, _ := newHydrateEnv(t, notesListHandler(t,
		[]string{"note_alpha", "note_gamma"},
		map[string]func(http.ResponseWriter){
			"note_alpha": writeJSON(fixtureNoteAlphaDetail),
			"note_gamma": writeJSON(fixtureNoteGammaDetail),
		}))

	res, err := runAPIHydrate(context.Background(), flags, apiHydrateOptions{})
	if err != nil {
		t.Fatalf("runAPIHydrate: %v", err)
	}
	if res.NotesListed != 2 || res.NotesFetched != 2 || res.Skipped != 0 {
		t.Errorf("listed=%d fetched=%d skipped=%d, want 2/2/0", res.NotesListed, res.NotesFetched, res.Skipped)
	}
	if res.Meetings != 2 {
		t.Errorf("meetings = %d, want 2", res.Meetings)
	}
	if res.Segments != 2 {
		t.Errorf("segments = %d, want 2", res.Segments)
	}
	if res.Memberships != 1 {
		t.Errorf("memberships = %d, want 1", res.Memberships)
	}

	db := openHydratedStore(t)
	assertQueryCount(t, db, `SELECT COUNT(*) FROM meetings`, 2)
	assertQueryCount(t, db, `SELECT COUNT(*) FROM attendees WHERE meeting_id='note_alpha'`, 2)
	assertQueryCount(t, db, `SELECT COUNT(*) FROM transcript_segments WHERE meeting_id='note_alpha'`, 2)
	assertQueryCount(t, db, `SELECT COUNT(*) FROM folder_memberships WHERE meeting_id='note_alpha'`, 1)
	// The post-freeze meeting carries its summary.
	var md string
	if err := db.QueryRow(`SELECT notes_markdown FROM meetings WHERE id='note_alpha'`).Scan(&md); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(md, "Agreed the quarterly milestones.") {
		t.Errorf("summary_markdown not hydrated onto the meeting: %q", md)
	}
	// The generic table the generated sync targets stays untouched by hydration.
	assertQueryCount(t, db, `SELECT COUNT(*) FROM resources`, 0)
}

// TestRunAPIHydrate_SkipsNotFoundNote: a 404 on one note id in a page is
// routine (the note was deleted between the list and detail calls). That id is
// skipped with a recorded warning; every other note still hydrates.
func TestRunAPIHydrate_SkipsNotFoundNote(t *testing.T) {
	flags, _ := newHydrateEnv(t, notesListHandler(t,
		[]string{"note_alpha", "note_missing", "note_gamma"},
		map[string]func(http.ResponseWriter){
			"note_alpha":   writeJSON(fixtureNoteAlphaDetail),
			"note_missing": writeStatus(http.StatusNotFound, `{"error":"not found"}`),
			"note_gamma":   writeJSON(fixtureNoteGammaDetail),
		}))

	res, err := runAPIHydrate(context.Background(), flags, apiHydrateOptions{})
	if err != nil {
		t.Fatalf("a 404 on one note must not fail the run: %v", err)
	}
	if res.Skipped != 1 {
		t.Errorf("skipped = %d, want 1", res.Skipped)
	}
	if res.Meetings != 2 {
		t.Errorf("meetings = %d, want 2 (the other notes must still hydrate)", res.Meetings)
	}
	if len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0], "note_missing") {
		t.Errorf("warnings = %v, want one naming note_missing", res.Warnings)
	}

	db := openHydratedStore(t)
	assertQueryCount(t, db, `SELECT COUNT(*) FROM meetings WHERE id='note_missing'`, 0)
	assertQueryCount(t, db, `SELECT COUNT(*) FROM meetings WHERE id='note_alpha'`, 1)
	assertQueryCount(t, db, `SELECT COUNT(*) FROM meetings WHERE id='note_gamma'`, 1)
}

// TestRunAPIHydrate_SkipsForbiddenNote: a 403 on ONE note's detail is a
// verdict about that note (ownership changed, archived, outside the token's
// scope), not about the credential — every other id in the run still fetches
// fine. Treating it as fatal returned before SyncFromAPI ever ran, throwing
// away every note already fetched, so it is skipped like a 404 instead.
func TestRunAPIHydrate_SkipsForbiddenNote(t *testing.T) {
	flags, _ := newHydrateEnv(t, notesListHandler(t,
		[]string{"note_alpha", "note_forbidden", "note_gamma"},
		map[string]func(http.ResponseWriter){
			"note_alpha":     writeJSON(fixtureNoteAlphaDetail),
			"note_forbidden": writeStatus(http.StatusForbidden, `{"error":"forbidden"}`),
			"note_gamma":     writeJSON(fixtureNoteGammaDetail),
		}))

	res, err := runAPIHydrate(context.Background(), flags, apiHydrateOptions{})
	if err != nil {
		t.Fatalf("a 403 on one note must not fail the run: %v", err)
	}
	if res.Skipped != 1 {
		t.Errorf("skipped = %d, want 1", res.Skipped)
	}
	if res.Meetings != 2 {
		t.Errorf("meetings = %d, want 2 (the other notes must still hydrate)", res.Meetings)
	}
	if len(res.Warnings) != 1 ||
		!strings.Contains(res.Warnings[0], "note_forbidden") ||
		!strings.Contains(res.Warnings[0], "403") {
		t.Errorf("warnings = %v, want one naming note_forbidden and its 403", res.Warnings)
	}

	db := openHydratedStore(t)
	assertQueryCount(t, db, `SELECT COUNT(*) FROM meetings WHERE id='note_forbidden'`, 0)
	assertQueryCount(t, db, `SELECT COUNT(*) FROM meetings WHERE id='note_alpha'`, 1)
	assertQueryCount(t, db, `SELECT COUNT(*) FROM meetings WHERE id='note_gamma'`, 1)
}

// TestRunAPIHydrate_AbortsOnForbiddenList: the list stage is the other half of
// the 403 split. There it means the token cannot list notes at all, so there
// is nothing to skip past and the run aborts with the auth exit code.
func TestRunAPIHydrate_AbortsOnForbiddenList(t *testing.T) {
	flags, _ := newHydrateEnv(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"forbidden"}`))
	})
	db := openHydratedStore(t)

	_, err := runAPIHydrate(context.Background(), flags, apiHydrateOptions{})
	if err == nil {
		t.Fatal("expected an error on a 403 from the list endpoint")
	}
	if !errors.Is(err, granola.ErrAPIUnauthorized) {
		t.Errorf("error chain lost ErrAPIUnauthorized: %v", err)
	}
	var ce *cliError
	if !errors.As(err, &ce) || ce.code != 4 {
		t.Errorf("expected the auth exit code (4), got %v", err)
	}
	assertQueryCount(t, db, `SELECT COUNT(*) FROM meetings`, 0)
}

// TestRunAPIHydrate_AbortsOnUnauthorized: a 401 means the credential is bad,
// so every remaining request would fail identically. The run aborts with an
// auth error and writes nothing — a half-populated store that looks complete
// is the failure mode this work exists to remove.
func TestRunAPIHydrate_AbortsOnUnauthorized(t *testing.T) {
	flags, _ := newHydrateEnv(t, notesListHandler(t,
		[]string{"note_alpha", "note_gamma"},
		map[string]func(http.ResponseWriter){
			"note_alpha": writeJSON(fixtureNoteAlphaDetail),
			"note_gamma": writeStatus(http.StatusUnauthorized, `{"error":"unauthorized"}`),
		}))

	// Create the store up front so "nothing was written" is a real assertion
	// rather than an artifact of the database file never existing.
	db := openHydratedStore(t)

	_, err := runAPIHydrate(context.Background(), flags, apiHydrateOptions{})
	if err == nil {
		t.Fatal("expected an error on 401")
	}
	if !errors.Is(err, granola.ErrAPIUnauthorized) {
		t.Errorf("error chain lost ErrAPIUnauthorized: %v", err)
	}
	var ce *cliError
	if !errors.As(err, &ce) || ce.code != 4 {
		t.Errorf("expected the auth exit code (4), got %v", err)
	}

	assertQueryCount(t, db, `SELECT COUNT(*) FROM meetings`, 0)
	assertQueryCount(t, db, `SELECT COUNT(*) FROM transcript_segments`, 0)
	assertQueryCount(t, db, `SELECT COUNT(*) FROM attendees`, 0)
}

// TestRunAPIHydrate_TalktimeSeesNonZeroSystemSeconds is the guard on the
// speaker-vocabulary normalization. The API emits speaker.source "speaker"
// (singular); talktime matches "system" / "speakers". Writing the API value
// through unchanged makes talktime silently report zero seconds for the other
// party while nothing errors — a wrong answer that looks healthy.
func TestRunAPIHydrate_TalktimeSeesNonZeroSystemSeconds(t *testing.T) {
	flags, _ := newHydrateEnv(t, notesListHandler(t,
		[]string{"note_alpha"},
		map[string]func(http.ResponseWriter){"note_alpha": writeJSON(fixtureNoteAlphaDetail)}))

	if _, err := runAPIHydrate(context.Background(), flags, apiHydrateOptions{}); err != nil {
		t.Fatalf("runAPIHydrate: %v", err)
	}

	segs := readStoredSegments(t, openHydratedStore(t), "note_alpha")
	if len(segs) != 2 {
		t.Fatalf("stored segments = %d, want 2", len(segs))
	}
	micSec, sysSec := sourceSeconds(segs)
	if micSec != 20 {
		t.Errorf("microphone seconds = %v, want 20", micSec)
	}
	if sysSec != 60 {
		t.Errorf("system seconds = %v, want 60 — the API's \"speaker\" source was not normalized to \"system\"", sysSec)
	}
	agg := aggregateBySources(segs)
	if agg["system_seconds"].(float64) == 0 {
		t.Error("talktime reports zero system seconds for an API-hydrated two-attendee meeting")
	}
}

// TestRunAPIHydrate_SurvivesSubsequentCacheSync: the operator-visible
// verification for R18 — after sync-api, a following cache sync leaves the
// API-hydrated rows intact.
func TestRunAPIHydrate_SurvivesSubsequentCacheSync(t *testing.T) {
	flags, _ := newHydrateEnv(t, notesListHandler(t,
		[]string{"note_alpha"},
		map[string]func(http.ResponseWriter){"note_alpha": writeJSON(fixtureNoteAlphaDetail)}))

	if _, err := runAPIHydrate(context.Background(), flags, apiHydrateOptions{}); err != nil {
		t.Fatalf("runAPIHydrate: %v", err)
	}
	db := openHydratedStore(t)

	// A cache sync where the cache carries the API meeting (as openGranolaCache's
	// store backfill produces) but no transcript and no folder for it.
	cache := &granola.Cache{
		Documents:   map[string]granola.Document{"note_alpha": {ID: "note_alpha", Title: "Quarterly planning sync"}},
		Transcripts: map[string][]granola.TranscriptSegment{},
	}
	if _, err := granola.SyncFromCache(context.Background(), db, cache); err != nil {
		t.Fatalf("SyncFromCache: %v", err)
	}

	assertQueryCount(t, db, `SELECT COUNT(*) FROM transcript_segments WHERE meeting_id='note_alpha'`, 2)
	assertQueryCount(t, db, `SELECT COUNT(*) FROM folder_memberships WHERE meeting_id='note_alpha'`, 1)
	assertQueryCount(t, db, `SELECT COUNT(*) FROM attendees WHERE meeting_id='note_alpha'`, 2)
	assertQueryCount(t, db, `SELECT COUNT(*) FROM meetings WHERE id='note_alpha'`, 1)
}

// TestRunAPIHydrate_SurfacesUnparseableTimestamps: a timestamp the store layer
// cannot parse is written as 0, which the read path renders as no timestamp at
// all. That degradation used to be invisible — the parse error was discarded
// at the call site — so the sync now reports it as a warning and a count.
func TestRunAPIHydrate_SurfacesUnparseableTimestamps(t *testing.T) {
	badTimestampDetail := strings.Replace(fixtureNoteAlphaDetail,
		`"start_time": "2026-07-01T15:00:00Z"`, `"start_time": "01 Jul 2026 15:00"`, 1)
	if badTimestampDetail == fixtureNoteAlphaDetail {
		t.Fatal("fixture no longer contains the timestamp this test rewrites")
	}
	flags, _ := newHydrateEnv(t, notesListHandler(t,
		[]string{"note_alpha"},
		map[string]func(http.ResponseWriter){"note_alpha": writeJSON(badTimestampDetail)}))

	res, err := runAPIHydrate(context.Background(), flags, apiHydrateOptions{})
	if err != nil {
		t.Fatalf("an unparseable timestamp must not fail the run: %v", err)
	}
	if res.UnparsedTimestamps != 1 {
		t.Errorf("UnparsedTimestamps = %d, want 1", res.UnparsedTimestamps)
	}
	if len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0], "01 Jul 2026 15:00") {
		t.Fatalf("warnings = %v, want one naming the unparseable value", res.Warnings)
	}

	// The operator-visible ndjson line must carry it too.
	var buf strings.Builder
	if err := writeAPIHydrateSummary(&buf, res); err != nil {
		t.Fatal(err)
	}
	var summary map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &summary); err != nil {
		t.Fatalf("summary is not one ndjson object: %v (%q)", err, buf.String())
	}
	if summary["unparsed_timestamps"] != float64(1) {
		t.Errorf("summary unparsed_timestamps = %v, want 1", summary["unparsed_timestamps"])
	}

	// The segment still landed; only its start timestamp is missing.
	db := openHydratedStore(t)
	assertQueryCount(t, db, `SELECT COUNT(*) FROM transcript_segments WHERE meeting_id='note_alpha'`, 2)
	assertQueryCount(t, db, `SELECT COUNT(*) FROM transcript_segments WHERE meeting_id='note_alpha' AND start_ts_ms=0`, 1)
}

func TestWriteAPIHydrateSummary_Shape(t *testing.T) {
	var buf strings.Builder
	err := writeAPIHydrateSummary(&buf, apiHydrateResult{
		NotesListed: 3, NotesFetched: 2, Skipped: 1,
		Meetings: 2, Attendees: 4, Segments: 9, Folders: 1, Memberships: 1,
		Summaries: 2, Events: 1,
		Warnings: []string{"note_missing: skipped"},
	})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &got); err != nil {
		t.Fatalf("summary is not one ndjson object: %v (%q)", err, buf.String())
	}
	for k, want := range map[string]any{
		"event": "sync_summary", "source": "granola_public_api", "stage": "detail_hydrate",
	} {
		if got[k] != want {
			t.Errorf("%s = %v, want %v", k, got[k], want)
		}
	}
	for _, k := range []string{"notes_listed", "notes_fetched", "notes_skipped", "meetings",
		"attendees", "transcript_segments", "folders", "folder_memberships", "warnings"} {
		if _, ok := got[k]; !ok {
			t.Errorf("summary missing %q", k)
		}
	}
}

// TestNewSyncApiCmd_DescribesTheRealCapability guards the doc text that used
// to claim the public API covers "~3 endpoints" and that transcripts are
// cache-only. Both are false now.
func TestNewSyncApiCmd_DescribesTheRealCapability(t *testing.T) {
	cmd := newSyncApiCmd(&rootFlags{})
	if cmd.Use != "sync-api" {
		t.Fatalf("Use = %q", cmd.Use)
	}
	blob := cmd.Short + "\n" + cmd.Long
	for _, stale := range []string{"~3 endpoints", "narrow set of resources"} {
		if strings.Contains(blob, stale) {
			t.Errorf("stale claim %q still present in sync-api help", stale)
		}
	}
	for _, want := range []string{"transcript", "attendees", "membership"} {
		if !strings.Contains(strings.ToLower(blob), want) {
			t.Errorf("sync-api help does not mention %q", want)
		}
	}
	// The generated flag surface must survive the RunE wrap.
	for _, f := range []string{"resources", "full", "since", "concurrency", "max-pages"} {
		if cmd.Flags().Lookup(f) == nil {
			t.Errorf("flag --%s was dropped from sync-api", f)
		}
	}
}

// ---------------------------------------------------------------------------

// readStoredSegments rebuilds granola.TranscriptSegment values from the store
// so the talktime aggregation helpers can be driven against hydrated rows.
func readStoredSegments(t *testing.T, db *sql.DB, meetingID string) []granola.TranscriptSegment {
	t.Helper()
	rows, err := db.Query(`SELECT source, text, start_ts_ms, end_ts_ms, confidence
		FROM transcript_segments WHERE meeting_id = ? ORDER BY idx`, meetingID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []granola.TranscriptSegment
	for rows.Next() {
		var src, text string
		var startMs, endMs int64
		var conf float64
		if err := rows.Scan(&src, &text, &startMs, &endMs, &conf); err != nil {
			t.Fatal(err)
		}
		out = append(out, granola.TranscriptSegment{
			DocumentID:     meetingID,
			Source:         src,
			Text:           text,
			StartTimestamp: time.UnixMilli(startMs).UTC().Format(time.RFC3339Nano),
			EndTimestamp:   time.UnixMilli(endMs).UTC().Format(time.RFC3339Nano),
			Confidence:     conf,
		})
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// TestRunAPIHydrate_PreservesLargerCacheTranscript is the operator-visible
// verification of the transcript-retention rule. Granola applies retention
// upstream while this store is expected to outlive it, so a meeting captured
// live and cache-synced in full comes back from the public API weeks later
// pruned but still non-empty. The sync used to treat non-empty as complete and
// replace the full local copy with the remnant, silently and with no count
// anywhere in the summary.
func TestRunAPIHydrate_PreservesLargerCacheTranscript(t *testing.T) {
	flags, _ := newHydrateEnv(t, notesListHandler(t,
		[]string{"note_alpha"},
		map[string]func(http.ResponseWriter){"note_alpha": writeJSON(fixtureNoteAlphaDetail)}))

	// Five cache-sourced segments against the fixture's surviving two.
	segs := make([]granola.TranscriptSegment, 0, 5)
	for i := 0; i < 5; i++ {
		segs = append(segs, granola.TranscriptSegment{
			Source:         "microphone",
			Text:           "cache line",
			StartTimestamp: "2026-07-01T15:00:00Z",
			EndTimestamp:   "2026-07-01T15:00:10Z",
		})
	}
	if _, err := granola.SyncFromCache(context.Background(), openHydratedStore(t), &granola.Cache{
		Documents:   map[string]granola.Document{"note_alpha": {ID: "note_alpha", Title: "Quarterly planning sync"}},
		Transcripts: map[string][]granola.TranscriptSegment{"note_alpha": segs},
	}); err != nil {
		t.Fatalf("SyncFromCache: %v", err)
	}

	res, err := runAPIHydrate(context.Background(), flags, apiHydrateOptions{})
	if err != nil {
		t.Fatalf("runAPIHydrate: %v", err)
	}
	db := openHydratedStore(t)
	assertQueryCount(t, db, `SELECT COUNT(*) FROM transcript_segments WHERE meeting_id='note_alpha'`, 5)
	assertQueryCount(t, db, `SELECT COUNT(*) FROM transcript_segments WHERE meeting_id='note_alpha' AND row_source='cache'`, 5)
	if res.PreservedTranscripts != 1 {
		t.Errorf("PreservedTranscripts = %d, want 1", res.PreservedTranscripts)
	}
	if len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0], "note_alpha") {
		t.Fatalf("warnings = %v, want one naming the preserved meeting", res.Warnings)
	}

	// The operator-visible ndjson line must carry it too.
	var buf strings.Builder
	if err := writeAPIHydrateSummary(&buf, res); err != nil {
		t.Fatal(err)
	}
	var summary map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &summary); err != nil {
		t.Fatalf("summary is not one ndjson object: %v (%q)", err, buf.String())
	}
	if summary["preserved_transcripts"] != float64(1) {
		t.Errorf("summary preserved_transcripts = %v, want 1", summary["preserved_transcripts"])
	}
}

func assertQueryCount(t *testing.T, db *sql.DB, query string, want int) {
	t.Helper()
	var got int
	if err := db.QueryRow(query).Scan(&got); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	if got != want {
		t.Errorf("%s = %d, want %d", query, got, want)
	}
}
