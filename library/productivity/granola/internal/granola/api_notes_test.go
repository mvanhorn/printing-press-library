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
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/granola/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/granola/internal/config"

	_ "modernc.org/sqlite"
)

// ---------------------------------------------------------------------------
// Fixtures.
//
// PII: every value below is invented. Names, @example.com addresses, folder
// titles, and transcript text are synthetic and were never captured from a
// real Granola account. This repository is public with permanent history —
// recorded fixtures must stay fully de-identified.
// ---------------------------------------------------------------------------

// noteDetailWithTranscriptJSON is a GET /v1/notes/{id}?include=transcript
// response: calendar event, attendees, both membership collections, both
// summary shapes, and a two-speaker transcript using the API's speaker
// vocabulary ("microphone" / "speaker").
const noteDetailWithTranscriptJSON = `{
  "id": "note_alpha",
  "object": "note",
  "title": "Quarterly planning sync",
  "web_url": "https://notes.example.com/note_alpha",
  "owner": {"name": "Ada Placeholder", "email": "ada@example.com"},
  "created_at": "2026-07-01T15:00:00Z",
  "updated_at": "2026-07-01T16:05:00Z",
  "calendar_event": {
    "event_title": "Quarterly planning",
    "invitees": [{"email": "ada@example.com"}, {"email": "bo@example.com"}, {"email": "cleo@example.com"}],
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
  "space_membership": [{"id": "space_team", "name": "Team space"}],
  "summary_text": "Agreed the quarterly milestones.",
  "summary_markdown": "## Summary\n\nAgreed the quarterly milestones.",
  "transcript": [
    {"text": "Kicking off the planning review.", "start_time": "2026-07-01T15:00:00Z", "end_time": "2026-07-01T15:00:30Z", "speaker": {"source": "microphone", "name": "Ada Placeholder"}},
    {"text": "Sounds good, I have the roadmap open.", "start_time": "2026-07-01T15:00:30Z", "end_time": "2026-07-01T15:01:30Z", "speaker": {"source": "speaker", "name": "Bo Sample", "diarization_label": "SPEAKER_01"}},
    {"text": "Let us start with milestone one.", "start_time": "2026-07-01T15:01:30Z", "end_time": "2026-07-01T15:02:00Z", "speaker": {"source": "microphone"}}
  ]
}`

// noteDetailNullTranscriptJSON is the shape returned when include=transcript
// was not requested, or the note simply has no transcript.
const noteDetailNullTranscriptJSON = `{
  "id": "note_beta",
  "object": "note",
  "title": "Solo scratchpad",
  "created_at": "2026-07-02T09:00:00Z",
  "updated_at": "2026-07-02T09:10:00Z",
  "calendar_event": null,
  "attendees": [],
  "folder_membership": [],
  "space_membership": [],
  "summary_text": "",
  "summary_markdown": "",
  "transcript": null
}`

func testClient(t *testing.T, baseURL string) *client.Client {
	t.Helper()
	c := client.New(&config.Config{BaseURL: baseURL, GranolaApiKey: "grn_test_key"}, 5*time.Second, 0)
	// Disable the on-disk GET cache so one test cannot replay another's body.
	c.NoCache = true
	return c
}

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := EnsureSchema(context.Background(), db); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	return db
}

func decodeNote(t *testing.T, raw string) APINote {
	t.Helper()
	var n APINote
	if err := json.Unmarshal([]byte(raw), &n); err != nil {
		t.Fatalf("decoding fixture: %v", err)
	}
	return n
}

// ---------------------------------------------------------------------------
// Transport-level tests.
// ---------------------------------------------------------------------------

func TestListNotesPage_ClampsPageSizeAndFollowsCursor(t *testing.T) {
	var seenPageSize, seenCursor []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/notes" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		seenPageSize = append(seenPageSize, r.URL.Query().Get("page_size"))
		seenCursor = append(seenCursor, r.URL.Query().Get("cursor"))
		_, _ = w.Write([]byte(`{"notes":[{"id":"note_alpha","title":"A"}],"hasMore":true,"cursor":"cur_2"}`))
	}))
	defer srv.Close()

	c := testClient(t, srv.URL)
	// 500 is well past the documented ceiling of 30; ListNotesPage must clamp
	// rather than let the API reject the request.
	page, err := ListNotesPage(c, "", 500, map[string]string{"updated_after": "", "folder_id": "folder_roadmap"})
	if err != nil {
		t.Fatalf("ListNotesPage: %v", err)
	}
	if len(page.Notes) != 1 || page.Notes[0].ID != "note_alpha" {
		t.Fatalf("unexpected page: %+v", page)
	}
	if !page.HasMore || page.Cursor != "cur_2" {
		t.Errorf("pagination envelope not decoded: hasMore=%v cursor=%q", page.HasMore, page.Cursor)
	}
	if seenPageSize[0] != "30" {
		t.Errorf("page_size not clamped to NotesPageSizeMax: got %q", seenPageSize[0])
	}

	if _, err := ListNotesPage(c, "cur_2", 10, nil); err != nil {
		t.Fatalf("second ListNotesPage: %v", err)
	}
	if seenCursor[1] != "cur_2" {
		t.Errorf("cursor not forwarded: got %q", seenCursor[1])
	}
	if seenPageSize[1] != "10" {
		t.Errorf("explicit in-range page_size not honored: got %q", seenPageSize[1])
	}
}

func TestGetNote_IncludeTranscriptParam(t *testing.T) {
	var withInclude, withoutInclude string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/notes/note_alpha":
			withInclude = r.URL.Query().Get("include")
			_, _ = w.Write([]byte(noteDetailWithTranscriptJSON))
		case "/v1/notes/note_beta":
			withoutInclude = r.URL.Query().Get("include")
			_, _ = w.Write([]byte(noteDetailNullTranscriptJSON))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	c := testClient(t, srv.URL)

	got, err := GetNote(c, "note_alpha", true)
	if err != nil {
		t.Fatalf("GetNote: %v", err)
	}
	if withInclude != "transcript" {
		t.Errorf("include param = %q, want transcript", withInclude)
	}
	if len(got.Transcript) != 3 {
		t.Errorf("transcript segments = %d, want 3", len(got.Transcript))
	}
	if got.CalendarEvent == nil || got.CalendarEvent.CalendarEventID != "evt_alpha_001" {
		t.Errorf("calendar_event not decoded: %+v", got.CalendarEvent)
	}

	got2, err := GetNote(c, "note_beta", false)
	if err != nil {
		t.Fatalf("GetNote without transcript: %v", err)
	}
	if withoutInclude != "" {
		t.Errorf("include param leaked when not requested: %q", withoutInclude)
	}
	if got2.Transcript != nil {
		t.Errorf("transcript should stay nil, got %+v", got2.Transcript)
	}
}

func TestGetNote_NotFoundIsTyped(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"note not found"}`))
	}))
	defer srv.Close()

	_, err := GetNote(testClient(t, srv.URL), "note_missing", true)
	if !errors.Is(err, ErrNoteNotFound) {
		t.Fatalf("expected ErrNoteNotFound, got %v", err)
	}
	if errors.Is(err, ErrAPIUnauthorized) {
		t.Error("404 must not classify as an auth failure")
	}
}

// TestGetNote_UnauthorizedIsTyped also pins the 401/403 split. Both mean the
// credential was rejected, so both keep matching ErrAPIUnauthorized (the CLI's
// auth exit code and hint hang off that). Only 403 additionally matches
// ErrAPIForbidden, which is what lets a caller iterating note ids skip the one
// note it may not read instead of discarding a whole run.
func TestGetNote_UnauthorizedIsTyped(t *testing.T) {
	for status, wantForbidden := range map[int]bool{
		http.StatusUnauthorized: false,
		http.StatusForbidden:    true,
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
		}))
		_, err := GetNote(testClient(t, srv.URL), "note_alpha", true)
		srv.Close()
		if !errors.Is(err, ErrAPIUnauthorized) {
			t.Fatalf("status %d: expected ErrAPIUnauthorized, got %v", status, err)
		}
		if errors.Is(err, ErrNoteNotFound) {
			t.Errorf("status %d must not classify as not-found", status)
		}
		if got := errors.Is(err, ErrAPIForbidden); got != wantForbidden {
			t.Errorf("status %d: errors.Is(err, ErrAPIForbidden) = %v, want %v", status, got, wantForbidden)
		}
	}
}

// TestNormalizeSpeakerSource is the guard on the silent-wrong-answer bug: the
// API says "speaker" (singular) for far-end audio; talktime's switch matches
// "system" / "speakers". An unnormalized write makes talktime report zero
// seconds for the other party while nothing errors.
func TestNormalizeSpeakerSource(t *testing.T) {
	cases := map[string]string{
		"microphone": "microphone",
		"mic":        "microphone",
		"speaker":    "system",
		"speakers":   "system",
		"system":     "system",
		"SPEAKER":    "system",
		" speaker ":  "system",
		"":           "",
		"mixed":      "mixed",
	}
	for in, want := range cases {
		if got := NormalizeSpeakerSource(in); got != want {
			t.Errorf("NormalizeSpeakerSource(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCalendarEventOrganiser_BothShapes(t *testing.T) {
	obj := &APICalendarEvent{Organiser: json.RawMessage(`{"name":"Ada Placeholder","email":"ada@example.com"}`)}
	if got := obj.OrganiserEmail(); got != "ada@example.com" {
		t.Errorf("object organiser email = %q", got)
	}
	if got := obj.OrganiserName(); got != "Ada Placeholder" {
		t.Errorf("object organiser name = %q", got)
	}
	str := &APICalendarEvent{Organiser: json.RawMessage(`"cleo@example.com"`)}
	if got := str.OrganiserEmail(); got != "cleo@example.com" {
		t.Errorf("string organiser email = %q", got)
	}
	var nilEvt *APICalendarEvent
	if got := nilEvt.OrganiserEmail(); got != "" {
		t.Errorf("nil organiser email = %q", got)
	}
}

// ---------------------------------------------------------------------------
// SyncFromAPI: hydration into the domain tables.
// ---------------------------------------------------------------------------

func TestSyncFromAPI_HydratesTranscriptSegments(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	note := decodeNote(t, noteDetailWithTranscriptJSON)

	res, err := SyncFromAPI(ctx, db, []APINote{note})
	if err != nil {
		t.Fatalf("SyncFromAPI: %v", err)
	}
	if res.Segments != 3 {
		t.Errorf("result segments = %d, want 3", res.Segments)
	}

	rows, err := db.QueryContext(ctx,
		`SELECT idx, source, text, start_ts_ms, end_ts_ms, row_source FROM transcript_segments WHERE meeting_id='note_alpha' ORDER BY idx`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	type seg struct {
		idx            int
		source, text   string
		startMs, endMs int64
		rowSource      string
	}
	var got []seg
	for rows.Next() {
		var s seg
		if err := rows.Scan(&s.idx, &s.source, &s.text, &s.startMs, &s.endMs, &s.rowSource); err != nil {
			t.Fatal(err)
		}
		got = append(got, s)
	}
	if len(got) != 3 {
		t.Fatalf("stored segments = %d, want 3", len(got))
	}
	wantSources := []string{"microphone", "system", "microphone"}
	for i, s := range got {
		if s.idx != i {
			t.Errorf("segment %d: idx = %d (ordering not preserved)", i, s.idx)
		}
		if s.source != wantSources[i] {
			t.Errorf("segment %d: source = %q, want %q", i, s.source, wantSources[i])
		}
		if s.rowSource != RowSourceAPI {
			t.Errorf("segment %d: row_source = %q, want %q", i, s.rowSource, RowSourceAPI)
		}
		if s.endMs <= s.startMs {
			t.Errorf("segment %d: timestamps not parsed (start=%d end=%d)", i, s.startMs, s.endMs)
		}
	}
	if got[0].text != "Kicking off the planning review." {
		t.Errorf("segment 0 text = %q", got[0].text)
	}
}

// TestSyncFromAPI_ResolvedSpeakerIdentity: API-sourced segments carry the
// speaker identity the desktop cache never had; cache-sourced segments leave
// those columns NULL so downstream commands stay source-agnostic.
func TestSyncFromAPI_ResolvedSpeakerIdentity(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	if _, err := SyncFromAPI(ctx, db, []APINote{decodeNote(t, noteDetailWithTranscriptJSON)}); err != nil {
		t.Fatalf("SyncFromAPI: %v", err)
	}

	var name, label sql.NullString
	if err := db.QueryRowContext(ctx,
		`SELECT speaker_name, diarization_label FROM transcript_segments WHERE meeting_id='note_alpha' AND idx=1`).
		Scan(&name, &label); err != nil {
		t.Fatal(err)
	}
	if !name.Valid || name.String != "Bo Sample" {
		t.Errorf("speaker_name = %+v, want Bo Sample", name)
	}
	if !label.Valid || label.String != "SPEAKER_01" {
		t.Errorf("diarization_label = %+v, want SPEAKER_01", label)
	}
	// Segment 2 has a speaker block with no resolved name — must stay NULL,
	// not the empty string, so "unknown" is distinguishable from "blank".
	var name2 sql.NullString
	if err := db.QueryRowContext(ctx,
		`SELECT speaker_name FROM transcript_segments WHERE meeting_id='note_alpha' AND idx=2`).Scan(&name2); err != nil {
		t.Fatal(err)
	}
	if name2.Valid {
		t.Errorf("unresolved speaker_name should be NULL, got %q", name2.String)
	}

	// A cache-hydrated segment leaves both columns NULL.
	cache := &Cache{
		Documents: map[string]Document{"cache_note": {ID: "cache_note", Title: "Cache meeting"}},
		Transcripts: map[string][]TranscriptSegment{"cache_note": {
			{Source: "microphone", Text: "cache line", StartTimestamp: "2026-06-01T10:00:00Z", EndTimestamp: "2026-06-01T10:00:10Z"},
		}},
	}
	if _, err := SyncFromCache(ctx, db, cache); err != nil {
		t.Fatalf("SyncFromCache: %v", err)
	}
	var cn, cl sql.NullString
	if err := db.QueryRowContext(ctx,
		`SELECT speaker_name, diarization_label FROM transcript_segments WHERE meeting_id='cache_note' AND idx=0`).
		Scan(&cn, &cl); err != nil {
		t.Fatal(err)
	}
	if cn.Valid || cl.Valid {
		t.Errorf("cache-sourced segment must leave speaker columns NULL, got %+v / %+v", cn, cl)
	}
}

// TestSyncFromAPI_NullTranscriptIsNotAFailure: a note whose transcript is
// null hydrates successfully with zero segments.
func TestSyncFromAPI_NullTranscriptIsNotAFailure(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	note := decodeNote(t, noteDetailNullTranscriptJSON)
	if note.Transcript != nil {
		t.Fatalf("fixture should decode transcript as nil, got %+v", note.Transcript)
	}

	res, err := SyncFromAPI(ctx, db, []APINote{note})
	if err != nil {
		t.Fatalf("SyncFromAPI on null transcript: %v", err)
	}
	if res.Meetings != 1 {
		t.Errorf("meetings = %d, want 1", res.Meetings)
	}
	if res.Segments != 0 {
		t.Errorf("segments = %d, want 0", res.Segments)
	}
	var n, avail int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM transcript_segments WHERE meeting_id='note_beta'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("stored segments = %d, want 0", n)
	}
	if err := db.QueryRowContext(ctx, `SELECT transcript_available FROM meetings WHERE id='note_beta'`).Scan(&avail); err != nil {
		t.Fatal(err)
	}
	if avail != 0 {
		t.Errorf("transcript_available = %d, want 0", avail)
	}
}

func TestSyncFromAPI_HydratesSummaryAndCalendarEvent(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	if _, err := SyncFromAPI(ctx, db, []APINote{decodeNote(t, noteDetailWithTranscriptJSON)}); err != nil {
		t.Fatalf("SyncFromAPI: %v", err)
	}

	var title, md, plain, evtID, startedAt, endedAt, rowSource string
	err := db.QueryRowContext(ctx,
		`SELECT title, notes_markdown, notes_plain, calendar_event_id, started_at, ended_at, row_source
		 FROM meetings WHERE id='note_alpha'`).
		Scan(&title, &md, &plain, &evtID, &startedAt, &endedAt, &rowSource)
	if err != nil {
		t.Fatal(err)
	}
	if title != "Quarterly planning sync" {
		t.Errorf("title = %q", title)
	}
	if md != "## Summary\n\nAgreed the quarterly milestones." {
		t.Errorf("summary_markdown did not land on the meeting summary column: %q", md)
	}
	if plain != "Agreed the quarterly milestones." {
		t.Errorf("summary_text did not land: %q", plain)
	}
	if evtID != "evt_alpha_001" {
		t.Errorf("calendar_event_id = %q", evtID)
	}
	if startedAt != "2026-07-01T15:00:00Z" {
		t.Errorf("started_at = %q, want the scheduled start time", startedAt)
	}
	if endedAt != "2026-07-01T16:00:00Z" {
		t.Errorf("ended_at = %q, want the scheduled end time", endedAt)
	}
	if rowSource != RowSourceAPI {
		t.Errorf("row_source = %q, want %q", rowSource, RowSourceAPI)
	}
}

// TestSyncFromAPI_AttendeesDeduplicatedOnEmail: attendees[] and
// calendar_event.invitees[] overlap. The store keys on (meeting_id, email),
// and invitees carry email only, so the named attendee record's name must win
// and the invitee-only address must still produce a row.
func TestSyncFromAPI_AttendeesDeduplicatedOnEmail(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	if _, err := SyncFromAPI(ctx, db, []APINote{decodeNote(t, noteDetailWithTranscriptJSON)}); err != nil {
		t.Fatalf("SyncFromAPI: %v", err)
	}

	rows, err := db.QueryContext(ctx, `SELECT email, name, row_source FROM attendees WHERE meeting_id='note_alpha' ORDER BY email`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	got := map[string]string{}
	for rows.Next() {
		var email, name, rowSource string
		if err := rows.Scan(&email, &name, &rowSource); err != nil {
			t.Fatal(err)
		}
		got[email] = name
		if rowSource != RowSourceAPI {
			t.Errorf("attendee %s: row_source = %q", email, rowSource)
		}
	}
	// ada + bo appear in both collections; cleo is invitee-only.
	if len(got) != 3 {
		t.Fatalf("attendees = %d (%v), want 3 deduplicated on email", len(got), got)
	}
	if got["ada@example.com"] != "Ada Placeholder" {
		t.Errorf("ada name = %q, want the attendee record's name", got["ada@example.com"])
	}
	if got["bo@example.com"] != "Bo Sample" {
		t.Errorf("bo name = %q, want the attendee record's name", got["bo@example.com"])
	}
	if _, ok := got["cleo@example.com"]; !ok {
		t.Error("invitee-only address produced no attendee row")
	}
	if got["cleo@example.com"] != "" {
		t.Errorf("invitee-only name = %q, want empty (invitees carry no name)", got["cleo@example.com"])
	}
}

// TestSyncFromAPI_FolderAndSpaceMembership: both membership collections map
// onto folder_memberships.
func TestSyncFromAPI_FolderAndSpaceMembership(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	res, err := SyncFromAPI(ctx, db, []APINote{decodeNote(t, noteDetailWithTranscriptJSON)})
	if err != nil {
		t.Fatalf("SyncFromAPI: %v", err)
	}
	if res.Memberships != 2 {
		t.Errorf("result memberships = %d, want 2", res.Memberships)
	}

	rows, err := db.QueryContext(ctx, `SELECT folder_id, row_source FROM folder_memberships WHERE meeting_id='note_alpha' ORDER BY folder_id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var fid, rowSource string
		if err := rows.Scan(&fid, &rowSource); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, fid)
		if rowSource != RowSourceAPI {
			t.Errorf("membership %s: row_source = %q", fid, rowSource)
		}
	}
	if len(ids) != 2 || ids[0] != "folder_roadmap" || ids[1] != "space_team" {
		t.Fatalf("folder_memberships = %v, want [folder_roadmap space_team]", ids)
	}

	var title string
	if err := db.QueryRowContext(ctx, `SELECT title FROM folders WHERE id='folder_roadmap'`).Scan(&title); err != nil {
		t.Fatalf("folder row not created: %v", err)
	}
	if title != "Roadmap" {
		t.Errorf("folder title = %q, want Roadmap", title)
	}
}

// TestSyncFromAPI_DoesNotTouchGenericResourcesTable: the hydration must land
// in the domain tables, not the generator-owned resources table nothing reads.
func TestSyncFromAPI_DoesNotTouchGenericResourcesTable(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	if _, err := db.ExecContext(ctx, `CREATE TABLE resources (
		id TEXT NOT NULL, resource_type TEXT NOT NULL, data JSON NOT NULL,
		PRIMARY KEY (resource_type, id))`); err != nil {
		t.Fatal(err)
	}
	if _, err := SyncFromAPI(ctx, db, []APINote{decodeNote(t, noteDetailWithTranscriptJSON)}); err != nil {
		t.Fatalf("SyncFromAPI: %v", err)
	}
	var n int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM resources`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("SyncFromAPI wrote %d rows to the generic resources table; it must write only the domain tables", n)
	}
	var meetings int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM meetings`).Scan(&meetings); err != nil {
		t.Fatal(err)
	}
	if meetings != 1 {
		t.Errorf("meetings = %d, want 1", meetings)
	}
}

// ---------------------------------------------------------------------------
// R18: row ownership across the two sync paths.
// ---------------------------------------------------------------------------

// TestAPIRowsSurviveCacheSync reproduces the data-loss bug this unit fixes.
// openGranolaCache backfills cache.Documents from the meetings table, so
// SyncFromCache iterates API-hydrated meetings for which the cache holds no
// transcript at all. Before the row_source scoping, that wiped their segments
// and — via the unscoped `DELETE FROM folder_memberships` — every API-sourced
// membership in the store.
func TestAPIRowsSurviveCacheSync(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	if _, err := SyncFromAPI(ctx, db, []APINote{decodeNote(t, noteDetailWithTranscriptJSON)}); err != nil {
		t.Fatalf("SyncFromAPI: %v", err)
	}

	// The cache now carries the API meeting (backfilled from the meetings
	// table) plus one genuinely cache-only meeting, and no transcripts for
	// either the API note or its folders.
	cache := &Cache{
		Documents: map[string]Document{
			"note_alpha": {ID: "note_alpha", Title: "Quarterly planning sync"},
			"cache_note": {ID: "cache_note", Title: "Cache meeting"},
		},
		Transcripts: map[string][]TranscriptSegment{"cache_note": {
			{Source: "microphone", Text: "cache line", StartTimestamp: "2026-06-01T10:00:00Z", EndTimestamp: "2026-06-01T10:00:10Z"},
		}},
		DocumentLists:         map[string][]string{"folder_cache": {"cache_note"}},
		DocumentListsMetadata: map[string]DocumentListMetadata{"folder_cache": {ID: "folder_cache", Title: "Cache folder"}},
	}
	if _, err := SyncFromCache(ctx, db, cache); err != nil {
		t.Fatalf("SyncFromCache: %v", err)
	}

	assertCount(t, db, `SELECT COUNT(*) FROM transcript_segments WHERE meeting_id='note_alpha'`, 3,
		"API transcript segments did not survive a cache sync")
	assertCount(t, db, `SELECT COUNT(*) FROM folder_memberships WHERE meeting_id='note_alpha'`, 2,
		"API folder memberships did not survive a cache sync")
	assertCount(t, db, `SELECT COUNT(*) FROM attendees WHERE meeting_id='note_alpha'`, 3,
		"API attendees did not survive a cache sync")
	assertCount(t, db, `SELECT COUNT(*) FROM meetings WHERE id='note_alpha'`, 1,
		"API meeting row did not survive a cache sync")
	// And the cache's own rows landed.
	assertCount(t, db, `SELECT COUNT(*) FROM folder_memberships WHERE meeting_id='cache_note'`, 1,
		"cache folder membership missing after cache sync")
}

// TestCacheRowsSurviveAPISync is the mirror image: a cache-hydrated meeting
// the API knows nothing about keeps its rows across an API sync.
func TestCacheRowsSurviveAPISync(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	cache := &Cache{
		Documents: map[string]Document{"cache_note": {
			ID: "cache_note", Title: "Cache meeting",
			People: &DocPeople{Attendees: []DocPerson{{Name: "Dev Example", Email: "dev@example.com"}}},
		}},
		Transcripts: map[string][]TranscriptSegment{"cache_note": {
			{Source: "microphone", Text: "cache line one", StartTimestamp: "2026-06-01T10:00:00Z", EndTimestamp: "2026-06-01T10:00:10Z"},
			{Source: "system", Text: "cache line two", StartTimestamp: "2026-06-01T10:00:10Z", EndTimestamp: "2026-06-01T10:00:25Z"},
		}},
		DocumentLists:         map[string][]string{"folder_cache": {"cache_note"}},
		DocumentListsMetadata: map[string]DocumentListMetadata{"folder_cache": {ID: "folder_cache", Title: "Cache folder"}},
	}
	if _, err := SyncFromCache(ctx, db, cache); err != nil {
		t.Fatalf("SyncFromCache: %v", err)
	}

	if _, err := SyncFromAPI(ctx, db, []APINote{decodeNote(t, noteDetailWithTranscriptJSON)}); err != nil {
		t.Fatalf("SyncFromAPI: %v", err)
	}

	assertCount(t, db, `SELECT COUNT(*) FROM transcript_segments WHERE meeting_id='cache_note'`, 2,
		"cache transcript segments did not survive an API sync")
	assertCount(t, db, `SELECT COUNT(*) FROM folder_memberships WHERE meeting_id='cache_note'`, 1,
		"cache folder membership did not survive an API sync")
	assertCount(t, db, `SELECT COUNT(*) FROM attendees WHERE meeting_id='cache_note'`, 1,
		"cache attendee did not survive an API sync")
	assertCount(t, db, `SELECT COUNT(*) FROM meetings WHERE id='cache_note'`, 1,
		"cache meeting row did not survive an API sync")
}

// TestBothPathsOnSameMeeting_NoPrimaryKeyCollision: when both sources carry a
// transcript for the same meeting, exactly one of them ends up owning every
// segment. Without that, the (meeting_id, idx) primary key collides and the
// sync errors out — or worse, leaves a mixed-provenance transcript with a
// stale tail.
//
// PATCH(transcript-retention-preserves-larger): the incoming cache transcript
// here used to be SHORTER than the API's, which is now the preservation case
// (TestLargerAPITranscriptSurvivesSmallerCacheSync covers it). Ownership
// transfer is what this test is about, so the incoming copy is longer than the
// three-segment API fixture.
func TestBothPathsOnSameMeeting_NoPrimaryKeyCollision(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	if _, err := SyncFromAPI(ctx, db, []APINote{decodeNote(t, noteDetailWithTranscriptJSON)}); err != nil {
		t.Fatalf("SyncFromAPI: %v", err)
	}
	// The cache now has a LONGER transcript for the same meeting.
	cache := &Cache{
		Documents: map[string]Document{"note_alpha": {ID: "note_alpha", Title: "Quarterly planning sync"}},
		Transcripts: map[string][]TranscriptSegment{"note_alpha": {
			{Source: "microphone", Text: "line one", StartTimestamp: "2026-07-01T15:00:00Z", EndTimestamp: "2026-07-01T15:00:05Z"},
			{Source: "system", Text: "line two", StartTimestamp: "2026-07-01T15:00:05Z", EndTimestamp: "2026-07-01T15:00:15Z"},
			{Source: "microphone", Text: "line three", StartTimestamp: "2026-07-01T15:00:15Z", EndTimestamp: "2026-07-01T15:00:25Z"},
			{Source: "system", Text: "line four", StartTimestamp: "2026-07-01T15:00:25Z", EndTimestamp: "2026-07-01T15:00:35Z"},
		}},
	}
	if _, err := SyncFromCache(ctx, db, cache); err != nil {
		t.Fatalf("SyncFromCache: %v", err)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM transcript_segments WHERE meeting_id='note_alpha'`, 4,
		"cache rewrite left a stale API tail on the same meeting")
	assertCount(t, db, `SELECT COUNT(*) FROM transcript_segments WHERE meeting_id='note_alpha' AND row_source='api'`, 0,
		"API rows remained after the cache took ownership of this meeting's transcript")
}

// TestSharedMeetingKeepsAPIOwnershipAcrossCacheSync covers the collision the
// two survival tests above never reach: a meeting BOTH paths know. They use
// disjoint meeting ids, so the ON CONFLICT branch of the cache sync's meeting,
// attendee, and membership writes never ran — and while those writes were
// INSERT OR REPLACE, SQLite deleted the existing row and inserted a fresh one,
// flipping row_source from 'api' to 'cache' and blanking every column only the
// API carries. The row survived by count while its contents were destroyed.
func TestSharedMeetingKeepsAPIOwnershipAcrossCacheSync(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	if _, err := SyncFromAPI(ctx, db, []APINote{decodeNote(t, noteDetailWithTranscriptJSON)}); err != nil {
		t.Fatalf("SyncFromAPI: %v", err)
	}

	// The cache's view of the SAME meeting, which is what openGranolaCache's
	// store backfill produces: title only, no summary, no calendar event, no
	// valid_meeting flag. Every field below is weaker than what the API wrote.
	cache := &Cache{
		Documents: map[string]Document{"note_alpha": {
			ID: "note_alpha", Title: "Quarterly planning sync",
			People: &DocPeople{Attendees: []DocPerson{
				// Same (meeting_id, email) key the API already owns: no name,
				// plus an RSVP only the cache carries.
				{Email: "ada@example.com", ResponseStatus: "accepted"},
			}},
		}},
		Transcripts:           map[string][]TranscriptSegment{},
		DocumentLists:         map[string][]string{"folder_roadmap": {"note_alpha"}},
		DocumentListsMetadata: map[string]DocumentListMetadata{"folder_roadmap": {ID: "folder_roadmap", Title: "Roadmap"}},
	}
	if _, err := SyncFromCache(ctx, db, cache); err != nil {
		t.Fatalf("SyncFromCache: %v", err)
	}

	var (
		rowSource, notesMD, notesPlain, calEvent string
		validMeeting                             int
	)
	if err := db.QueryRowContext(ctx, `SELECT row_source, notes_markdown, notes_plain,
		calendar_event_id, valid_meeting FROM meetings WHERE id='note_alpha'`).Scan(
		&rowSource, &notesMD, &notesPlain, &calEvent, &validMeeting); err != nil {
		t.Fatalf("reading the shared meeting: %v", err)
	}
	if rowSource != RowSourceAPI {
		t.Errorf("meeting row_source = %q, want %q: the cache sync took ownership of a row the API created, so the next API sync's scoped DELETE can no longer reach it", rowSource, RowSourceAPI)
	}
	if !strings.Contains(notesMD, "Agreed the quarterly milestones.") {
		t.Errorf("notes_markdown = %q, want the API summary: a cache sync blanked a column only the API carries", notesMD)
	}
	if notesPlain == "" {
		t.Error("notes_plain was blanked by the cache sync")
	}
	if calEvent != "evt_alpha_001" {
		t.Errorf("calendar_event_id = %q, want evt_alpha_001", calEvent)
	}
	if validMeeting != 1 {
		t.Errorf("valid_meeting = %d, want 1", validMeeting)
	}

	// Attendees share the (meeting_id, email) primary key, so the same REPLACE
	// hazard applies: the API's resolved name must survive a nameless cache
	// record, and the cache's RSVP must merge in without taking ownership.
	var attRowSource, attName, attRSVP string
	if err := db.QueryRowContext(ctx, `SELECT row_source, name, response_status
		FROM attendees WHERE meeting_id='note_alpha' AND email='ada@example.com'`).Scan(
		&attRowSource, &attName, &attRSVP); err != nil {
		t.Fatalf("reading the shared attendee: %v", err)
	}
	if attRowSource != RowSourceAPI {
		t.Errorf("attendee row_source = %q, want %q", attRowSource, RowSourceAPI)
	}
	if attName != "Ada Placeholder" {
		t.Errorf("attendee name = %q, want the API-resolved name: a nameless cache record overwrote it", attName)
	}
	if attRSVP != "accepted" {
		t.Errorf("attendee response_status = %q, want accepted: the cache's contribution should merge in", attRSVP)
	}

	// folder_memberships carries no payload beyond its key pair, so the only
	// thing a REPLACE could destroy there is the ownership marker itself.
	var memRowSource string
	if err := db.QueryRowContext(ctx, `SELECT row_source FROM folder_memberships
		WHERE folder_id='folder_roadmap' AND meeting_id='note_alpha'`).Scan(&memRowSource); err != nil {
		t.Fatalf("reading the shared folder membership: %v", err)
	}
	if memRowSource != RowSourceAPI {
		t.Errorf("folder membership row_source = %q, want %q", memRowSource, RowSourceAPI)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM folder_memberships WHERE meeting_id='note_alpha'`, 2,
		"the shared membership was duplicated rather than merged")
}

// TestSharedMeetingKeepsCacheOwnershipAcrossAPISync is the mirror direction:
// a cache-created meeting keeps row_source='cache' and its cache-only columns
// when an API sync touches the same id.
func TestSharedMeetingKeepsCacheOwnershipAcrossAPISync(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	cache := &Cache{
		Documents: map[string]Document{"note_alpha": {
			ID: "note_alpha", Title: "Quarterly planning sync",
			// workspace_id, creation_source and the RSVP are cache-only: the
			// public API has no equivalent field to carry them.
			WorkspaceID: "ws_cache_001", CreationSource: "granola_desktop",
			ValidMeeting: true,
			People: &DocPeople{Attendees: []DocPerson{
				{Name: "Ada Placeholder", Email: "ada@example.com", ResponseStatus: "accepted"},
			}},
		}},
		Transcripts:           map[string][]TranscriptSegment{},
		DocumentLists:         map[string][]string{"folder_roadmap": {"note_alpha"}},
		DocumentListsMetadata: map[string]DocumentListMetadata{"folder_roadmap": {ID: "folder_roadmap", Title: "Roadmap"}},
	}
	if _, err := SyncFromCache(ctx, db, cache); err != nil {
		t.Fatalf("SyncFromCache: %v", err)
	}
	if _, err := SyncFromAPI(ctx, db, []APINote{decodeNote(t, noteDetailWithTranscriptJSON)}); err != nil {
		t.Fatalf("SyncFromAPI: %v", err)
	}

	var rowSource, workspaceID, creationSource string
	var validMeeting int
	if err := db.QueryRowContext(ctx, `SELECT row_source, workspace_id, creation_source, valid_meeting
		FROM meetings WHERE id='note_alpha'`).Scan(&rowSource, &workspaceID, &creationSource, &validMeeting); err != nil {
		t.Fatalf("reading the shared meeting: %v", err)
	}
	if rowSource != RowSourceCache {
		t.Errorf("meeting row_source = %q, want %q: the API sync took ownership of a row the cache created", rowSource, RowSourceCache)
	}
	if workspaceID != "ws_cache_001" {
		t.Errorf("workspace_id = %q, want ws_cache_001: the API sync blanked a cache-only column", workspaceID)
	}
	if creationSource != "granola_desktop" {
		t.Errorf("creation_source = %q, want granola_desktop", creationSource)
	}
	if validMeeting != 1 {
		t.Errorf("valid_meeting = %d, want 1", validMeeting)
	}

	var attRowSource, attName, attRSVP string
	if err := db.QueryRowContext(ctx, `SELECT row_source, name, response_status
		FROM attendees WHERE meeting_id='note_alpha' AND email='ada@example.com'`).Scan(
		&attRowSource, &attName, &attRSVP); err != nil {
		t.Fatalf("reading the shared attendee: %v", err)
	}
	if attRowSource != RowSourceCache {
		t.Errorf("attendee row_source = %q, want %q", attRowSource, RowSourceCache)
	}
	if attName != "Ada Placeholder" {
		t.Errorf("attendee name = %q, want Ada Placeholder", attName)
	}
	if attRSVP != "accepted" {
		t.Errorf("attendee response_status = %q, want accepted: the API carries no RSVP and must not blank one", attRSVP)
	}

	var memRowSource string
	if err := db.QueryRowContext(ctx, `SELECT row_source FROM folder_memberships
		WHERE folder_id='folder_roadmap' AND meeting_id='note_alpha'`).Scan(&memRowSource); err != nil {
		t.Fatalf("reading the shared folder membership: %v", err)
	}
	if memRowSource != RowSourceCache {
		t.Errorf("folder membership row_source = %q, want %q", memRowSource, RowSourceCache)
	}
}

// TestEnsureSchema_AdditiveOnLegacyDatabase: EnsureSchema runs against
// databases created before row_source existed. The new columns must be added
// in place, existing rows backfilled to 'cache' (which is what they in fact
// were), and the whole thing must be idempotent.
func TestEnsureSchema_AdditiveOnLegacyDatabase(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "legacy.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// The pre-change table shapes.
	legacy := []string{
		`CREATE TABLE meetings (id TEXT PRIMARY KEY, title TEXT, created_at TEXT, updated_at TEXT,
			started_at TEXT, ended_at TEXT, workspace_id TEXT, calendar_event_id TEXT, deleted_at TEXT,
			notes_markdown TEXT, notes_plain TEXT, transcript_available INTEGER NOT NULL DEFAULT 0,
			recipes_applied TEXT, creation_source TEXT, valid_meeting INTEGER NOT NULL DEFAULT 0)`,
		`CREATE TABLE attendees (meeting_id TEXT NOT NULL, email TEXT NOT NULL, name TEXT,
			response_status TEXT, PRIMARY KEY (meeting_id, email))`,
		`CREATE TABLE transcript_segments (meeting_id TEXT NOT NULL, idx INTEGER NOT NULL, source TEXT,
			text TEXT, start_ts_ms INTEGER, end_ts_ms INTEGER, confidence REAL, PRIMARY KEY (meeting_id, idx))`,
		`CREATE TABLE folder_memberships (folder_id TEXT NOT NULL, meeting_id TEXT NOT NULL,
			PRIMARY KEY (folder_id, meeting_id))`,
		`INSERT INTO meetings(id,title) VALUES ('legacy_note','Legacy')`,
		`INSERT INTO attendees(meeting_id,email,name) VALUES ('legacy_note','dev@example.com','Dev Example')`,
		`INSERT INTO transcript_segments(meeting_id,idx,source,text) VALUES ('legacy_note',0,'microphone','legacy line')`,
		`INSERT INTO folder_memberships(folder_id,meeting_id) VALUES ('folder_legacy','legacy_note')`,
	}
	for _, stmt := range legacy {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("legacy setup %q: %v", firstLine(stmt), err)
		}
	}

	for i := 0; i < 2; i++ { // idempotency: running twice must not error
		if err := EnsureSchema(ctx, db); err != nil {
			t.Fatalf("EnsureSchema pass %d: %v", i+1, err)
		}
	}

	for _, table := range []string{"meetings", "attendees", "transcript_segments", "folder_memberships"} {
		var src string
		q := `SELECT row_source FROM ` + table + ` LIMIT 1`
		if err := db.QueryRowContext(ctx, q).Scan(&src); err != nil {
			t.Fatalf("%s: row_source column missing after migration: %v", table, err)
		}
		if src != RowSourceCache {
			t.Errorf("%s: pre-existing row backfilled as %q, want %q", table, src, RowSourceCache)
		}
	}
	var name, label sql.NullString
	if err := db.QueryRowContext(ctx,
		`SELECT speaker_name, diarization_label FROM transcript_segments LIMIT 1`).Scan(&name, &label); err != nil {
		t.Fatalf("speaker columns missing after migration: %v", err)
	}
	if name.Valid || label.Valid {
		t.Error("legacy segment should have NULL speaker columns")
	}

	// And the migrated database still round-trips a real sync.
	if _, err := SyncFromAPI(ctx, db, []APINote{decodeNote(t, noteDetailWithTranscriptJSON)}); err != nil {
		t.Fatalf("SyncFromAPI on migrated legacy database: %v", err)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM transcript_segments WHERE meeting_id='legacy_note'`, 1,
		"legacy segment lost after an API sync")
}

// ---------------------------------------------------------------------------
// Unparseable transcript timestamps.
// ---------------------------------------------------------------------------

// TestSyncFromCache_ReportsUnparseableTimestamps: isoToMillis returns 0 on a
// parse failure and the read path treats start_ts_ms == 0 as "no timestamp",
// so a timestamp format this CLI does not recognise degrades to a blank time
// with no error anywhere. The sync must still store the segment, and must
// count what it could not parse.
func TestSyncFromCache_ReportsUnparseableTimestamps(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	cache := &Cache{
		Documents: map[string]Document{"cache_note": {ID: "cache_note", Title: "Cache meeting"}},
		Transcripts: map[string][]TranscriptSegment{"cache_note": {
			// A shape ParseISO does not accept, as a live-API format change
			// would produce.
			{Source: "microphone", Text: "bad start", StartTimestamp: "07/01/2026 3:00 PM", EndTimestamp: "2026-06-01T10:00:10Z"},
			// Empty timestamps are a normal, already-handled case and must not
			// be counted as failures.
			{Source: "system", Text: "no timestamps at all"},
		}},
	}
	res, err := SyncFromCache(ctx, db, cache)
	if err != nil {
		t.Fatalf("one unparseable timestamp must not fail the sync: %v", err)
	}
	if res.Segments != 2 {
		t.Errorf("segments = %d, want 2: the segment is still worth storing without its timestamp", res.Segments)
	}
	if res.UnparsedTimestamps != 1 {
		t.Errorf("UnparsedTimestamps = %d, want 1", res.UnparsedTimestamps)
	}
	for _, want := range []string{"cache_note", "start_timestamp", "07/01/2026 3:00 PM"} {
		if !strings.Contains(res.TimestampWarning, want) {
			t.Errorf("TimestampWarning = %q, want it to name %q", res.TimestampWarning, want)
		}
	}
	// And the failure really did land as the blank-on-read 0 the warning
	// describes, so the message is not overstating the problem.
	assertCount(t, db, `SELECT COUNT(*) FROM transcript_segments WHERE meeting_id='cache_note' AND start_ts_ms=0`, 2,
		"unparseable and empty timestamps both store as 0")
}

// TestSyncFromAPI_ReportsUnparseableTimestamps is the same guard on the API
// path, where a live format change is far more likely to appear first.
func TestSyncFromAPI_ReportsUnparseableTimestamps(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	note := decodeNote(t, noteDetailWithTranscriptJSON)
	note.Transcript[1].StartTime = "1751382030"

	res, err := SyncFromAPI(ctx, db, []APINote{note})
	if err != nil {
		t.Fatalf("one unparseable timestamp must not fail the sync: %v", err)
	}
	if res.Segments != 3 {
		t.Errorf("segments = %d, want 3", res.Segments)
	}
	if res.UnparsedTimestamps != 1 {
		t.Errorf("UnparsedTimestamps = %d, want 1", res.UnparsedTimestamps)
	}
	for _, want := range []string{"note_alpha", "segment 1", "start_time", "1751382030"} {
		if !strings.Contains(res.TimestampWarning, want) {
			t.Errorf("TimestampWarning = %q, want it to name %q", res.TimestampWarning, want)
		}
	}
}

// TestSyncResults_SilentWhenEveryTimestampParses keeps the accounting from
// becoming noise on a healthy sync.
func TestSyncResults_SilentWhenEveryTimestampParses(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	apiRes, err := SyncFromAPI(ctx, db, []APINote{decodeNote(t, noteDetailWithTranscriptJSON)})
	if err != nil {
		t.Fatalf("SyncFromAPI: %v", err)
	}
	if apiRes.UnparsedTimestamps != 0 || apiRes.TimestampWarning != "" {
		t.Errorf("clean API sync reported %d unparsed timestamps (%q)", apiRes.UnparsedTimestamps, apiRes.TimestampWarning)
	}
	cacheRes, err := SyncFromCache(ctx, db, &Cache{
		Documents: map[string]Document{"cache_note": {ID: "cache_note", Title: "Cache meeting"}},
		Transcripts: map[string][]TranscriptSegment{"cache_note": {
			{Source: "microphone", Text: "cache line", StartTimestamp: "2026-06-01T10:00:00Z", EndTimestamp: "2026-06-01T10:00:10Z"},
		}},
	})
	if err != nil {
		t.Fatalf("SyncFromCache: %v", err)
	}
	if cacheRes.UnparsedTimestamps != 0 || cacheRes.TimestampWarning != "" {
		t.Errorf("clean cache sync reported %d unparsed timestamps (%q)", cacheRes.UnparsedTimestamps, cacheRes.TimestampWarning)
	}
}

// ---------------------------------------------------------------------------
// Transcript retention: a smaller incoming transcript must not delete a larger
// one owned by the other path.
// ---------------------------------------------------------------------------

// cacheWithTranscript builds a minimal cache carrying one meeting and n
// synthetic transcript segments for it.
func cacheWithTranscript(id string, n int) *Cache {
	base := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)
	segs := make([]TranscriptSegment, 0, n)
	for i := 0; i < n; i++ {
		segs = append(segs, TranscriptSegment{
			Source:         "microphone",
			Text:           fmt.Sprintf("cache line %d", i),
			StartTimestamp: base.Add(time.Duration(i) * time.Minute).Format(time.RFC3339),
			EndTimestamp:   base.Add(time.Duration(i)*time.Minute + 30*time.Second).Format(time.RFC3339),
		})
	}
	return &Cache{
		Documents:   map[string]Document{id: {ID: id, Title: "Live captured meeting"}},
		Transcripts: map[string][]TranscriptSegment{id: segs},
	}
}

// apiNoteWithTranscript builds an APINote carrying n synthetic segments, the
// shape a retention-pruned note comes back as.
func apiNoteWithTranscript(id string, n int) APINote {
	base := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)
	note := APINote{
		ID:        id,
		Title:     "Live captured meeting",
		CreatedAt: base.Format(time.RFC3339),
		UpdatedAt: base.Add(time.Hour).Format(time.RFC3339),
	}
	for i := 0; i < n; i++ {
		note.Transcript = append(note.Transcript, APITranscriptSegment{
			Text:      fmt.Sprintf("api line %d", i),
			StartTime: base.Add(time.Duration(i) * time.Minute).Format(time.RFC3339),
			EndTime:   base.Add(time.Duration(i)*time.Minute + 30*time.Second).Format(time.RFC3339),
			Speaker:   &APISpeaker{Source: "microphone"},
		})
	}
	return note
}

// assertSingleProvenance fails when one meeting's transcript is a mix of both
// paths' rows. Preserving a transcript must not degrade into interleaving.
func assertSingleProvenance(t *testing.T, db *sql.DB, meetingID string) {
	t.Helper()
	var sources int
	if err := db.QueryRow(
		`SELECT COUNT(DISTINCT row_source) FROM transcript_segments WHERE meeting_id = ?`,
		meetingID).Scan(&sources); err != nil {
		t.Fatalf("provenance check for %s: %v", meetingID, err)
	}
	if sources > 1 {
		t.Errorf("meeting %s holds segments from %d sources: a transcript must never mix provenance", meetingID, sources)
	}
}

// TestLargerCacheTranscriptSurvivesSmallerAPISync is the data-loss case this
// unit fixes. A meeting captured live and cache-synced in full is later pruned
// upstream by Granola's transcript retention, so the API still returns a
// non-empty — but much shorter — transcript. Treating "non-empty" as
// "authoritative and complete" made the API sync delete the complete local copy
// and replace it with the remnant.
func TestLargerCacheTranscriptSurvivesSmallerAPISync(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	if _, err := SyncFromCache(ctx, db, cacheWithTranscript("note_alpha", 5)); err != nil {
		t.Fatalf("SyncFromCache: %v", err)
	}

	res, err := SyncFromAPI(ctx, db, []APINote{apiNoteWithTranscript("note_alpha", 2)})
	if err != nil {
		t.Fatalf("SyncFromAPI: %v", err)
	}

	assertCount(t, db, `SELECT COUNT(*) FROM transcript_segments WHERE meeting_id='note_alpha'`, 5,
		"the retention-pruned API transcript destroyed the larger cache-sourced one")
	assertCount(t, db, `SELECT COUNT(*) FROM transcript_segments WHERE meeting_id='note_alpha' AND row_source='cache'`, 5,
		"the preserved segments are no longer cache-owned")
	assertSingleProvenance(t, db, "note_alpha")
	if res.Segments != 0 {
		t.Errorf("Segments = %d, want 0: nothing was written for the preserved meeting", res.Segments)
	}
	if res.PreservedTranscripts != 1 {
		t.Errorf("PreservedTranscripts = %d, want 1", res.PreservedTranscripts)
	}
	for _, want := range []string{"note_alpha", "5", "2"} {
		if !strings.Contains(res.PreservationWarning, want) {
			t.Errorf("PreservationWarning = %q, want it to mention %q", res.PreservationWarning, want)
		}
	}
}

// TestLargerAPITranscriptSurvivesSmallerCacheSync is the mirror direction: the
// desktop cache's copy of a meeting is the partial one (the local cache file
// rolls over) while the API still holds the full transcript.
func TestLargerAPITranscriptSurvivesSmallerCacheSync(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	if _, err := SyncFromAPI(ctx, db, []APINote{apiNoteWithTranscript("note_alpha", 6)}); err != nil {
		t.Fatalf("SyncFromAPI: %v", err)
	}

	res, err := SyncFromCache(ctx, db, cacheWithTranscript("note_alpha", 1))
	if err != nil {
		t.Fatalf("SyncFromCache: %v", err)
	}

	assertCount(t, db, `SELECT COUNT(*) FROM transcript_segments WHERE meeting_id='note_alpha'`, 6,
		"the partial cache transcript destroyed the larger API-sourced one")
	assertCount(t, db, `SELECT COUNT(*) FROM transcript_segments WHERE meeting_id='note_alpha' AND row_source='api'`, 6,
		"the preserved segments are no longer API-owned")
	assertSingleProvenance(t, db, "note_alpha")
	if res.Segments != 0 {
		t.Errorf("Segments = %d, want 0: nothing was written for the preserved meeting", res.Segments)
	}
	if res.PreservedTranscripts != 1 {
		t.Errorf("PreservedTranscripts = %d, want 1", res.PreservedTranscripts)
	}
	if !strings.Contains(res.PreservationWarning, "note_alpha") {
		t.Errorf("PreservationWarning = %q, want it to name the meeting", res.PreservationWarning)
	}
}

// TestEqualOrLargerIncomingTakesOwnership: preservation is only for the
// shrinking case. An incoming transcript at least as complete as what is
// stored still takes ownership of the whole meeting and fully replaces it —
// anything less would strand a stale tail or collide on (meeting_id, idx).
func TestEqualOrLargerIncomingTakesOwnership(t *testing.T) {
	for _, tc := range []struct {
		name     string
		incoming int
	}{
		{"equal", 3},
		{"larger", 7},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			db := openTestDB(t)
			if _, err := SyncFromCache(ctx, db, cacheWithTranscript("note_alpha", 3)); err != nil {
				t.Fatalf("SyncFromCache: %v", err)
			}

			res, err := SyncFromAPI(ctx, db, []APINote{apiNoteWithTranscript("note_alpha", tc.incoming)})
			if err != nil {
				t.Fatalf("SyncFromAPI: %v", err)
			}

			assertCount(t, db,
				`SELECT COUNT(*) FROM transcript_segments WHERE meeting_id='note_alpha' AND row_source='api'`,
				tc.incoming, "the incoming transcript did not take ownership")
			assertCount(t, db, `SELECT COUNT(*) FROM transcript_segments WHERE meeting_id='note_alpha' AND row_source='cache'`, 0,
				"the replaced transcript left cache-owned rows behind")
			assertSingleProvenance(t, db, "note_alpha")
			if res.PreservedTranscripts != 0 || res.PreservationWarning != "" {
				t.Errorf("reported %d preserved transcripts (%q) on a normal ownership transfer",
					res.PreservedTranscripts, res.PreservationWarning)
			}
		})
	}
}

// TestSameSourceShrinkingRewriteIsNotPreservation: a path replacing its OWN
// earlier transcript with a shorter one is a normal rewrite — upstream edited,
// re-transcribed, or pruned the note — and must not be mistaken for the
// cross-source preservation case, which would freeze the store on the first
// transcript it ever saw.
func TestSameSourceShrinkingRewriteIsNotPreservation(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	if _, err := SyncFromAPI(ctx, db, []APINote{apiNoteWithTranscript("note_alpha", 5)}); err != nil {
		t.Fatalf("seeding SyncFromAPI: %v", err)
	}

	res, err := SyncFromAPI(ctx, db, []APINote{apiNoteWithTranscript("note_alpha", 2)})
	if err != nil {
		t.Fatalf("SyncFromAPI: %v", err)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM transcript_segments WHERE meeting_id='note_alpha'`, 2,
		"an API rewrite of its own transcript left a stale tail")
	if res.Segments != 2 {
		t.Errorf("Segments = %d, want 2: a same-source rewrite must still write", res.Segments)
	}
	if res.PreservedTranscripts != 0 || res.PreservationWarning != "" {
		t.Errorf("same-source shrink reported %d preserved transcripts (%q)",
			res.PreservedTranscripts, res.PreservationWarning)
	}

	// Same guard on the cache path.
	if _, err := SyncFromCache(ctx, db, cacheWithTranscript("cache_note", 4)); err != nil {
		t.Fatalf("seeding SyncFromCache: %v", err)
	}
	cacheRes, err := SyncFromCache(ctx, db, cacheWithTranscript("cache_note", 1))
	if err != nil {
		t.Fatalf("SyncFromCache: %v", err)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM transcript_segments WHERE meeting_id='cache_note'`, 1,
		"a cache rewrite of its own transcript left a stale tail")
	if cacheRes.PreservedTranscripts != 0 || cacheRes.PreservationWarning != "" {
		t.Errorf("same-source cache shrink reported %d preserved transcripts (%q)",
			cacheRes.PreservedTranscripts, cacheRes.PreservationWarning)
	}
}

// TestEmptyIncomingTranscriptClearsOnlyItsOwnRows pins the pre-existing
// behavior the retention rule must not disturb: a path with nothing to write
// retires its own stale rows and leaves the other path's transcript alone,
// without reporting a preservation (nothing was at risk).
func TestEmptyIncomingTranscriptClearsOnlyItsOwnRows(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	if _, err := SyncFromCache(ctx, db, cacheWithTranscript("note_alpha", 3)); err != nil {
		t.Fatalf("SyncFromCache: %v", err)
	}

	// An API note with no transcript at all for the same meeting.
	res, err := SyncFromAPI(ctx, db, []APINote{apiNoteWithTranscript("note_alpha", 0)})
	if err != nil {
		t.Fatalf("SyncFromAPI: %v", err)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM transcript_segments WHERE meeting_id='note_alpha' AND row_source='cache'`, 3,
		"an empty API transcript cleared cache-owned rows")
	if res.PreservedTranscripts != 0 || res.PreservationWarning != "" {
		t.Errorf("empty incoming transcript reported %d preserved transcripts (%q)",
			res.PreservedTranscripts, res.PreservationWarning)
	}

	// And a path with nothing to write still retires its OWN stale rows.
	if _, err := SyncFromAPI(ctx, db, []APINote{apiNoteWithTranscript("api_note", 2)}); err != nil {
		t.Fatalf("seeding SyncFromAPI: %v", err)
	}
	if _, err := SyncFromAPI(ctx, db, []APINote{apiNoteWithTranscript("api_note", 0)}); err != nil {
		t.Fatalf("SyncFromAPI: %v", err)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM transcript_segments WHERE meeting_id='api_note'`, 0,
		"an empty API transcript did not retire the API's own stale rows")
}

func assertCount(t *testing.T, db *sql.DB, query string, want int, msg string) {
	t.Helper()
	var got int
	if err := db.QueryRow(query).Scan(&got); err != nil {
		t.Fatalf("%s: %v", msg, err)
	}
	if got != want {
		t.Errorf("%s: got %d, want %d (%s)", msg, got, want, query)
	}
}
