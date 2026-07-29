// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/granola/internal/granola"
	"github.com/mvanhorn/printing-press-library/library/productivity/granola/internal/store"
	"github.com/spf13/cobra"
)

// openGranolaCache loads the local cache file. Returns a typed error if
// the file is missing so commands can surface a helpful message.
//
// PATCH(encrypted-cache): two changes vs. the generator-produced version:
//
//  1. Pass an empty path to LoadCache so the resolver picks
//     cache-v6.json.enc when it exists. The previous version pinned to
//     DefaultCachePath() which is the plaintext path - on modern Granola
//     installs that file is a stale stub.
//  2. Backfill cache.Documents from the SQLite store. Granola desktop
//     moved documents out of the local cache at the same time as the
//     encryption rollout; sync populates the meetings table from the
//     API. Without this backfill, every command that reads cache.Documents
//     directly (show, notes-show, memo, export, tiptap) returns "meeting
//     not in cache" even when sync has run.
func openGranolaCache() (*granola.Cache, error) {
	path, _ := granola.ResolveCachePath()
	c, err := granola.LoadCache("")
	if err != nil {
		return nil, fmt.Errorf("loading Granola cache at %s: %w", path, err)
	}
	// Best-effort document backfill; errors logged but not fatal so
	// commands that only need transcripts/folders still work when the
	// store is unavailable.
	_ = backfillDocumentsFromStore(c)
	return c, nil
}

// PATCH(dual-path-store-read): granolaRead is the read seam every
// cache-direct read command now goes through.
//
// openGranolaCache() hard-fails when the desktop cache cannot be decrypted,
// which is now the permanent steady state: Granola's key migration made
// cache-v6.json.enc unreadable and killed the WorkOS token path the internal
// API used. Everything the read commands need is already hydrated into the
// SQLite domain tables (meetings, attendees, transcript_segments,
// folder_memberships) by the sync commands, so the read side must consult the
// store first and treat the desktop cache as a fallback rather than a
// precondition.
//
// Precedence is fixed and identical for every rerouted command:
//
//  1. the local SQLite store,
//  2. the desktop cache, when the store has no row and a cache is readable.
//
// Neither step requires a network call or a GRANOLA_API_KEY - reading data
// that is already local must never depend on credentials.
type granolaRead struct {
	ctx   context.Context
	store *store.Store   // nil when the database does not exist yet
	cache *granola.Cache // nil when the desktop cache is unreadable

	docs   map[string]granola.Document // lazily built union, store-first
	sorted []string
}

// openGranolaRead opens the read view. It fails only when neither data path
// is available at all; a missing store or an unreadable cache on its own is
// a normal, survivable state.
func openGranolaRead(ctx context.Context) (*granolaRead, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	v := &granolaRead{ctx: ctx}
	// A store that exists but fails to open is treated as absent; the cache
	// fallback below is still worth trying.
	if s, err := openGranolaStoreRead(ctx); err == nil {
		v.store = s
	}
	// A cache that will not decrypt is the steady state on migrated
	// installs; the decrypt error is deliberately dropped rather than
	// surfaced, because "run sync" is the only actionable advice and
	// doctor already reports the decrypt status in detail.
	if c, err := granola.LoadCache(""); err == nil {
		v.cache = c
	}
	if v.store == nil && v.cache == nil {
		return nil, errNoLocalGranolaData()
	}
	return v, nil
}

// Close releases the store handle. Safe on a nil receiver so callers can
// `defer v.Close()` immediately after the error check.
func (v *granolaRead) Close() {
	if v == nil || v.store == nil {
		return
	}
	_ = v.store.Close()
	v.store = nil
}

// hasCache reports whether a desktop cache was actually readable. Commands
// use it to decide whether a cache-only fallback is still on the table.
func (v *granolaRead) hasCache() bool { return v != nil && v.cache != nil }

// Cache exposes the desktop cache for the state only it carries (panels,
// recipes, chat threads, workspaces, document lists). Returns nil when the
// cache is unreadable - callers must handle that.
func (v *granolaRead) Cache() *granola.Cache {
	if v == nil {
		return nil
	}
	return v.cache
}

// errNoLocalGranolaData is the message a read command surfaces when neither
// the store nor the cache can answer. Deliberately does not wrap the
// safestorage decrypt failure: "refresh refused for encrypted source" tells
// the user nothing actionable, whereas "run sync" does.
func errNoLocalGranolaData() error {
	return notFoundErr(fmt.Errorf("no local Granola data available - run `granola-pp-cli sync-api` (or `granola-pp-cli sync` for a desktop-cache install) first"))
}

// Documents returns the union of store meetings and cache documents, keyed
// by id. Store rows win field-by-field for the columns the store owns; the
// cache supplies the fields it alone carries (TipTap notes, people, calendar
// event) so legacy installs do not regress.
func (v *granolaRead) Documents() map[string]granola.Document {
	if v == nil {
		return nil
	}
	if v.docs != nil {
		return v.docs
	}
	docs := map[string]granola.Document{}
	if v.cache != nil {
		for id, d := range v.cache.Documents {
			docs[id] = d
		}
	}
	for id, sd := range v.storeDocuments() {
		docs[id] = mergeStoreDocument(docs[id], sd)
	}
	v.docs = docs
	return docs
}

// DocumentByID returns the merged document for id, or nil.
func (v *granolaRead) DocumentByID(id string) *granola.Document {
	d, ok := v.Documents()[id]
	if !ok {
		return nil
	}
	return &d
}

// SortedDocumentIDs mirrors Cache.SortedDocumentIDs (descending created_at)
// over the merged document set.
func (v *granolaRead) SortedDocumentIDs() []string {
	if v == nil {
		return nil
	}
	if v.sorted != nil {
		return v.sorted
	}
	docs := v.Documents()
	ids := make([]string, 0, len(docs))
	for id := range docs {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		return docs[ids[i]].CreatedAt > docs[ids[j]].CreatedAt
	})
	v.sorted = ids
	return ids
}

// storeDocuments reads the meetings table into Document structs. Best-effort:
// a query failure yields an empty map so the cache fallback still serves.
func (v *granolaRead) storeDocuments() map[string]granola.Document {
	out := map[string]granola.Document{}
	if v == nil || v.store == nil {
		return out
	}
	rows, err := v.store.DB().QueryContext(v.ctx, `
		SELECT id, title, created_at, updated_at,
		       workspace_id, deleted_at, notes_markdown, notes_plain,
		       creation_source, valid_meeting
		FROM meetings
	`)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var d granola.Document
		var deletedAt string
		var validMeeting int
		if err := rows.Scan(
			&d.ID, &d.Title, &d.CreatedAt, &d.UpdatedAt,
			&d.WorkspaceID, &deletedAt,
			&d.NotesMarkdown, &d.NotesPlain,
			&d.CreationSource, &validMeeting,
		); err != nil {
			return out
		}
		if deletedAt != "" {
			da := deletedAt
			d.DeletedAt = &da
		}
		d.ValidMeeting = validMeeting != 0
		out[d.ID] = d
	}
	return out
}

// mergeStoreDocument overlays the store's columns onto a (possibly zero)
// cache document. Empty store values never blank out a richer cache value -
// "store first" means the store wins where it has an answer, not that it
// erases what only the cache knows.
func mergeStoreDocument(base, sd granola.Document) granola.Document {
	out := base
	out.ID = sd.ID
	if sd.Title != "" {
		out.Title = sd.Title
	}
	if sd.CreatedAt != "" {
		out.CreatedAt = sd.CreatedAt
	}
	if sd.UpdatedAt != "" {
		out.UpdatedAt = sd.UpdatedAt
	}
	if sd.WorkspaceID != "" {
		out.WorkspaceID = sd.WorkspaceID
	}
	if sd.NotesMarkdown != "" {
		out.NotesMarkdown = sd.NotesMarkdown
	}
	if sd.NotesPlain != "" {
		out.NotesPlain = sd.NotesPlain
	}
	if sd.CreationSource != "" {
		out.CreationSource = sd.CreationSource
	}
	if sd.DeletedAt != nil {
		out.DeletedAt = sd.DeletedAt
	}
	if sd.ValidMeeting {
		out.ValidMeeting = true
	}
	return out
}

// TranscriptByID returns the transcript segments for id, store first.
func (v *granolaRead) TranscriptByID(id string) []granola.TranscriptSegment {
	segs, _ := v.transcriptWithSource(id)
	return segs
}

// transcriptWithSource returns the segments plus which path served them
// ("store" or "cache"); the string is empty when there are none.
//
// The store query is deliberately source-agnostic: rows written by the cache
// sync and rows written by the public API sync are the same shape, so
// row_source is never filtered on here.
func (v *granolaRead) transcriptWithSource(id string) ([]granola.TranscriptSegment, string) {
	if v == nil {
		return nil, ""
	}
	if segs := v.storeTranscript(id); len(segs) > 0 {
		return segs, "store"
	}
	if v.cache != nil {
		if segs := v.cache.TranscriptByID(id); len(segs) > 0 {
			return segs, "cache"
		}
	}
	return nil, ""
}

// storeTranscript reads transcript_segments in idx order and rebuilds the
// cache-shaped TranscriptSegment the whole CLI already speaks.
func (v *granolaRead) storeTranscript(id string) []granola.TranscriptSegment {
	if v == nil || v.store == nil {
		return nil
	}
	rows, err := v.store.DB().QueryContext(v.ctx, `
		SELECT source, text, start_ts_ms, end_ts_ms, confidence
		FROM transcript_segments
		WHERE meeting_id = ?
		ORDER BY idx ASC
	`, id)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []granola.TranscriptSegment
	for rows.Next() {
		var (
			source, text   sql.NullString
			startMs, endMs sql.NullInt64
			confidence     sql.NullFloat64
			seg            granola.TranscriptSegment
		)
		if err := rows.Scan(&source, &text, &startMs, &endMs, &confidence); err != nil {
			return out
		}
		seg.DocumentID = id
		seg.Source = source.String
		seg.Text = text.String
		seg.Confidence = confidence.Float64
		seg.IsFinal = true
		if startMs.Valid && startMs.Int64 > 0 {
			seg.StartTimestamp = millisToISO(startMs.Int64)
		}
		if endMs.Valid && endMs.Int64 > 0 {
			seg.EndTimestamp = millisToISO(endMs.Int64)
		}
		out = append(out, seg)
	}
	return out
}

// millisToISO renders a stored epoch-millis timestamp in the same layout the
// cache uses, so granola.ParseISO round-trips it exactly.
func millisToISO(ms int64) string {
	return time.UnixMilli(ms).UTC().Format("2006-01-02T15:04:05.000Z")
}

// MeetingMetadataByID returns attendee metadata for a meeting, store first.
// The store has no creator column, so store-sourced metadata carries
// Attendees only; commands that key off Creator degrade to their
// no-creator branch rather than misattributing.
func (v *granolaRead) MeetingMetadataByID(id string) *granola.MeetingMetadata {
	if v == nil {
		return nil
	}
	if md := v.storeMeetingMetadata(id); md != nil {
		return md
	}
	if v.cache != nil {
		return v.cache.MeetingMetadataByID(id)
	}
	return nil
}

func (v *granolaRead) storeMeetingMetadata(id string) *granola.MeetingMetadata {
	if v == nil || v.store == nil {
		return nil
	}
	rows, err := v.store.DB().QueryContext(v.ctx,
		`SELECT email, name, response_status FROM attendees WHERE meeting_id = ? ORDER BY email ASC`, id)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var md granola.MeetingMetadata
	for rows.Next() {
		var email, name, status sql.NullString
		if err := rows.Scan(&email, &name, &status); err != nil {
			return nil
		}
		md.Attendees = append(md.Attendees, granola.CalendarInvitee{
			Name:           name.String,
			Email:          email.String,
			ResponseStatus: status.String,
		})
	}
	if len(md.Attendees) == 0 {
		return nil
	}
	return &md
}

// Folders returns the folder list, store first. Store rows carry only
// id/title/parent_id/workspace_id/preset; description, is_favourited,
// members and rules exist solely in the cache's documentListsMetadata and
// are merged in when a cache is readable.
func (v *granolaRead) Folders() []granola.DocumentListMetadata {
	if v == nil {
		return nil
	}
	byID := map[string]granola.DocumentListMetadata{}
	var order []string
	if v.cache != nil {
		for fid, md := range v.cache.DocumentListsMetadata {
			md.ID = fid
			byID[fid] = md
			order = append(order, fid)
		}
	}
	for _, sf := range v.storeFolders() {
		existing, ok := byID[sf.ID]
		if !ok {
			byID[sf.ID] = sf
			order = append(order, sf.ID)
			continue
		}
		if sf.Title != "" {
			existing.Title = sf.Title
		}
		if sf.ParentDocumentListID != "" {
			existing.ParentDocumentListID = sf.ParentDocumentListID
		}
		if sf.WorkspaceID != "" {
			existing.WorkspaceID = sf.WorkspaceID
		}
		if sf.Preset != "" {
			existing.Preset = sf.Preset
		}
		byID[sf.ID] = existing
	}
	sort.Strings(order)
	out := make([]granola.DocumentListMetadata, 0, len(order))
	for _, fid := range order {
		out = append(out, byID[fid])
	}
	return out
}

func (v *granolaRead) storeFolders() []granola.DocumentListMetadata {
	if v == nil || v.store == nil {
		return nil
	}
	rows, err := v.store.DB().QueryContext(v.ctx,
		`SELECT id, title, parent_id, workspace_id, preset FROM folders ORDER BY id ASC`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []granola.DocumentListMetadata
	for rows.Next() {
		var f granola.DocumentListMetadata
		var parent, workspace, preset sql.NullString
		if err := rows.Scan(&f.ID, &f.Title, &parent, &workspace, &preset); err != nil {
			return out
		}
		f.ParentDocumentListID = parent.String
		f.WorkspaceID = workspace.String
		f.Preset = preset.String
		out = append(out, f)
	}
	return out
}

// FolderByName resolves a folder by id or case-insensitive title, using the
// merged folder set.
func (v *granolaRead) FolderByName(name string) *granola.DocumentListMetadata {
	if v == nil {
		return nil
	}
	for _, f := range v.Folders() {
		if f.ID == name || strings.EqualFold(f.Title, name) {
			res := f
			return &res
		}
	}
	return nil
}

// FolderMeetings returns the meeting ids in a folder, store first.
//
// Memberships are filtered to meetings the view can actually resolve.
// folder_memberships outlives the meetings it points at - a cache-sourced
// membership row survives a cache-sourced meeting being replaced by an
// API-sourced one - and streaming an id no data path knows about only
// produces a per-meeting error line downstream.
func (v *granolaRead) FolderMeetings(folderID string) []string {
	if v == nil {
		return nil
	}
	ids := v.storeFolderMeetings(folderID)
	if len(ids) == 0 && v.cache != nil {
		ids = v.cache.FolderMeetings(folderID)
	}
	docs := v.Documents()
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if _, ok := docs[id]; ok {
			out = append(out, id)
		}
	}
	return out
}

func (v *granolaRead) storeFolderMeetings(folderID string) []string {
	if v == nil || v.store == nil {
		return nil
	}
	rows, err := v.store.DB().QueryContext(v.ctx,
		`SELECT meeting_id FROM folder_memberships WHERE folder_id = ? ORDER BY meeting_id ASC`, folderID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var mid string
		if err := rows.Scan(&mid); err != nil {
			return out
		}
		out = append(out, mid)
	}
	return out
}

// backfillDocumentsFromStore reads rows from the meetings table that
// sync populated from /v2/get-documents and reconstructs lightweight
// Document structs into c.Documents. Quietly returns nil if the store
// does not exist yet (fresh install before first sync) - the caller's
// behavior on an empty cache.Documents is appropriate in that case.
func backfillDocumentsFromStore(c *granola.Cache) error {
	if c == nil {
		return nil
	}
	if c.Documents == nil {
		c.Documents = map[string]granola.Document{}
	}
	if len(c.Documents) > 0 {
		// Cache already has documents (pre-encryption Granola or test
		// fixtures); don't shadow them with potentially stale store rows.
		return nil
	}
	ctx := context.Background()
	s, err := openGranolaStoreRead(ctx)
	if err != nil || s == nil {
		return err
	}
	defer s.Close()

	rows, err := s.DB().QueryContext(ctx, `
		SELECT id, title, created_at, updated_at, started_at, ended_at,
		       workspace_id, deleted_at, notes_markdown, notes_plain,
		       creation_source, valid_meeting
		FROM meetings
	`)
	if err != nil {
		return fmt.Errorf("backfill: query meetings: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var d granola.Document
		var deletedAt, startedAt, endedAt string
		var validMeeting int
		_ = startedAt
		_ = endedAt
		if err := rows.Scan(
			&d.ID, &d.Title, &d.CreatedAt, &d.UpdatedAt,
			&startedAt, &endedAt,
			&d.WorkspaceID, &deletedAt,
			&d.NotesMarkdown, &d.NotesPlain,
			&d.CreationSource, &validMeeting,
		); err != nil {
			return fmt.Errorf("backfill: scan meeting: %w", err)
		}
		if deletedAt != "" {
			da := deletedAt
			d.DeletedAt = &da
		}
		d.ValidMeeting = validMeeting != 0
		c.Documents[d.ID] = d
	}
	return rows.Err()
}

// openGranolaStore opens (or creates) the SQLite store and ensures the
// granola-specific schema is in place.
func openGranolaStore(ctx context.Context) (*store.Store, error) {
	return openGranolaStoreAt(ctx, "")
}

// openGranolaStoreAt opens the Granola domain store at an explicit path,
// falling back to the default location when dbPath is empty.
//
// PATCH(api-list-stage-matches-live-contract): the sync commands expose a
// --db flag, but it previously reached only the generated generic store, so a
// caller pointing --db at a scratch database still had the domain tables
// (meetings, attendees, transcript_segments, folder_memberships) written to
// the default store. Honoring the override here keeps --db meaningful for the
// tables the read commands actually consume.
func openGranolaStoreAt(ctx context.Context, dbPath string) (*store.Store, error) {
	if dbPath == "" {
		dbPath = defaultDBPath("granola-pp-cli")
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("creating data dir: %w", err)
	}
	s, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening local store: %w", err)
	}
	if err := granola.EnsureSchema(ctx, s.DB()); err != nil {
		s.Close()
		return nil, err
	}
	return s, nil
}

// openGranolaStoreRead opens the store for reading; returns (nil, nil)
// if the database hasn't been created yet so the caller can emit a
// helpful "run sync first" message.
func openGranolaStoreRead(ctx context.Context) (*store.Store, error) {
	dbPath := defaultDBPath("granola-pp-cli")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return nil, nil
	}
	s, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, err
	}
	if err := granola.EnsureSchema(ctx, s.DB()); err != nil {
		s.Close()
		return nil, err
	}
	return s, nil
}

// emitJSON writes v to cmd's stdout as JSON, honoring --compact and
// --select via printJSONFiltered.
func emitJSON(cmd *cobra.Command, flags *rootFlags, v any) error {
	return printJSONFiltered(cmd.OutOrStdout(), v, flags)
}

// emitNDJSON writes each item on its own line.
func emitNDJSON(cmd *cobra.Command, items []any) error {
	w := cmd.OutOrStdout()
	for _, it := range items {
		b, err := json.Marshal(it)
		if err != nil {
			return err
		}
		if _, err := w.Write(append(b, '\n')); err != nil {
			return err
		}
	}
	return nil
}

// emitNDJSONLine writes one ndjson line.
func emitNDJSONLine(w io.Writer, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = w.Write(append(b, '\n'))
	return err
}

// parseTimeWindow translates --last 7d / --since DATE / --until DATE
// into an absolute [from, to] pair. Either end may be zero-valued.
// --since accepts both absolute dates ("2026-05-01") and relative
// durations ("7d", "24h") — relative durations are subtracted from now.
func parseTimeWindow(last, since, until string) (from, to time.Time, err error) {
	now := time.Now()
	if last != "" {
		d, perr := parseDurationLoose(last)
		if perr != nil {
			err = fmt.Errorf("invalid --last %q: %w", last, perr)
			return
		}
		from = now.Add(-d)
		to = now
		return
	}
	if since != "" {
		from, err = parseSinceOrDate(since, now)
		if err != nil {
			err = fmt.Errorf("invalid --since %q: %w", since, err)
			return
		}
	}
	if until != "" {
		to, err = parseSinceOrDate(until, now)
		if err != nil {
			err = fmt.Errorf("invalid --until %q: %w", until, err)
			return
		}
	}
	return
}

// parseSinceOrDate tries a relative duration first (suffixes d/w/h/m/s),
// then falls back to an absolute date.
func parseSinceOrDate(s string, now time.Time) (time.Time, error) {
	if d, err := parseDurationLoose(s); err == nil {
		return now.Add(-d), nil
	}
	return parseAnyDate(s)
}

// timeNow is a wall-clock indirection used by commands so tests can swap
// the clock. Defaults to time.Now.
var timeNow = time.Now

// parseDurationLoose accepts "7d", "30d", "3h", and standard Go durations.
func parseDurationLoose(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty duration")
	}
	// "Nd" -> N*24h
	if strings.HasSuffix(s, "d") {
		var n int
		if _, err := fmt.Sscanf(s, "%dd", &n); err == nil {
			return time.Duration(n) * 24 * time.Hour, nil
		}
	}
	// "Nw" -> N*7d
	if strings.HasSuffix(s, "w") {
		var n int
		if _, err := fmt.Sscanf(s, "%dw", &n); err == nil {
			return time.Duration(n) * 7 * 24 * time.Hour, nil
		}
	}
	return time.ParseDuration(s)
}

func parseAnyDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05Z", "2006-01-02 15:04", "2006-01-02", "01/02/2006"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	if d, err := parseDurationLoose(s); err == nil {
		return time.Now().Add(-d), nil
	}
	return time.Time{}, fmt.Errorf("unrecognized date %q", s)
}

// withinWindow returns true if t is inside [from, to] when those are set.
// Zero from/to are unbounded.
func withinWindow(t time.Time, from, to time.Time) bool {
	if t.IsZero() {
		return false
	}
	if !from.IsZero() && t.Before(from) {
		return false
	}
	if !to.IsZero() && t.After(to) {
		return false
	}
	return true
}

// stderr writes a one-line user-visible note to stderr (warnings only).
func stderr(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

// Ensure sql import is referenced even when no .go file under cli/ uses
// it directly; we re-export from this package.
var _ = sql.Open
