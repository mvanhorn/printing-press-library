// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.

package granola

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// granolaSchemaSQL is the set of CREATE TABLE statements added on top of
// the generator-owned store.go schema. All statements are idempotent
// (IF NOT EXISTS) so we can run them every open without harm.
var granolaSchemaSQL = []string{
	`CREATE TABLE IF NOT EXISTS meetings (
		id TEXT PRIMARY KEY,
		title TEXT,
		created_at TEXT,
		updated_at TEXT,
		started_at TEXT,
		ended_at TEXT,
		workspace_id TEXT,
		calendar_event_id TEXT,
		deleted_at TEXT,
		notes_markdown TEXT,
		notes_plain TEXT,
		transcript_available INTEGER NOT NULL DEFAULT 0,
		recipes_applied TEXT,
		creation_source TEXT,
		valid_meeting INTEGER NOT NULL DEFAULT 0,
		row_source TEXT NOT NULL DEFAULT 'cache'
	)`,
	`CREATE INDEX IF NOT EXISTS idx_meetings_started ON meetings(started_at)`,
	`CREATE INDEX IF NOT EXISTS idx_meetings_created ON meetings(created_at)`,
	`CREATE INDEX IF NOT EXISTS idx_meetings_workspace ON meetings(workspace_id)`,

	`CREATE TABLE IF NOT EXISTS attendees (
		meeting_id TEXT NOT NULL,
		email TEXT NOT NULL,
		name TEXT,
		response_status TEXT,
		row_source TEXT NOT NULL DEFAULT 'cache',
		PRIMARY KEY (meeting_id, email)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_attendees_email ON attendees(email)`,

	`CREATE TABLE IF NOT EXISTS transcript_segments (
		meeting_id TEXT NOT NULL,
		idx INTEGER NOT NULL,
		source TEXT,
		text TEXT,
		start_ts_ms INTEGER,
		end_ts_ms INTEGER,
		confidence REAL,
		speaker_name TEXT,
		diarization_label TEXT,
		row_source TEXT NOT NULL DEFAULT 'cache',
		PRIMARY KEY (meeting_id, idx)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_segments_meeting ON transcript_segments(meeting_id)`,
	`CREATE INDEX IF NOT EXISTS idx_segments_source ON transcript_segments(source)`,

	`CREATE TABLE IF NOT EXISTS folders (
		id TEXT PRIMARY KEY,
		title TEXT,
		parent_id TEXT,
		workspace_id TEXT,
		owner_id TEXT,
		preset TEXT,
		description TEXT,
		is_favourited INTEGER NOT NULL DEFAULT 0,
		row_source TEXT NOT NULL DEFAULT 'cache'
	)`,

	`CREATE TABLE IF NOT EXISTS folder_memberships (
		folder_id TEXT NOT NULL,
		meeting_id TEXT NOT NULL,
		row_source TEXT NOT NULL DEFAULT 'cache',
		PRIMARY KEY (folder_id, meeting_id)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_fm_meeting ON folder_memberships(meeting_id)`,

	`CREATE TABLE IF NOT EXISTS panel_templates (
		id TEXT PRIMARY KEY,
		slug TEXT,
		title TEXT,
		description TEXT,
		category TEXT,
		row_source TEXT NOT NULL DEFAULT 'cache'
	)`,
	`CREATE INDEX IF NOT EXISTS idx_panel_templates_slug ON panel_templates(slug)`,

	`CREATE TABLE IF NOT EXISTS recipes (
		id TEXT PRIMARY KEY,
		slug TEXT,
		name TEXT,
		description TEXT,
		category TEXT,
		source TEXT,
		row_source TEXT NOT NULL DEFAULT 'cache'
	)`,
	`CREATE INDEX IF NOT EXISTS idx_recipes_slug ON recipes(slug)`,

	`CREATE TABLE IF NOT EXISTS recipes_usage (
		recipe_id TEXT PRIMARY KEY,
		total_count INTEGER NOT NULL DEFAULT 0,
		last_used_at TEXT,
		row_source TEXT NOT NULL DEFAULT 'cache'
	)`,

	`CREATE TABLE IF NOT EXISTS chat_threads (
		id TEXT PRIMARY KEY,
		meeting_id TEXT,
		workspace_id TEXT,
		title TEXT,
		created_at TEXT,
		updated_at TEXT
	)`,
	`CREATE INDEX IF NOT EXISTS idx_chat_threads_meeting ON chat_threads(meeting_id)`,

	`CREATE TABLE IF NOT EXISTS chat_messages (
		id TEXT PRIMARY KEY,
		thread_id TEXT NOT NULL,
		role TEXT,
		turn_index INTEGER,
		content TEXT,
		created_at TEXT
	)`,
	`CREATE INDEX IF NOT EXISTS idx_chat_messages_thread ON chat_messages(thread_id)`,

	`CREATE TABLE IF NOT EXISTS workspaces (
		id TEXT PRIMARY KEY,
		display_name TEXT,
		plan_type TEXT,
		role TEXT,
		raw TEXT
	)`,

	`CREATE VIRTUAL TABLE IF NOT EXISTS meetings_fts USING fts5(
		title, notes_plain, content='meetings', content_rowid='rowid', tokenize='porter unicode61'
	)`,

	`CREATE VIRTUAL TABLE IF NOT EXISTS transcript_fts USING fts5(
		text, content='transcript_segments', tokenize='porter unicode61'
	)`,
}

// Row-ownership markers written into the row_source column. Every DELETE in
// a sync path is scoped to the marker that path owns, so the two data paths
// (desktop cache and public API) can populate the same install without one
// wiping the other's rows. See granolaAddedColumns for the migration.
const (
	// RowSourceCache marks rows hydrated from the desktop cache file.
	RowSourceCache = "cache"
	// RowSourceAPI marks rows hydrated from the public REST API.
	RowSourceAPI = "api"
)

// granolaAddedColumns is the additive-migration list for columns introduced
// after the original CREATE TABLE statements shipped. EnsureSchema runs
// against databases created by older binaries, where CREATE TABLE IF NOT
// EXISTS is a no-op and the new columns would otherwise never appear.
//
// The ownership column is named row_source, not source: transcript_segments
// already has a `source` column carrying the AUDIO source
// (microphone/system), and overloading that name would be ambiguous at every
// call site.
//
// The DEFAULT 'cache' is load-bearing on upgrade. Before this column existed
// every row in these tables came from the cache path, so SQLite backfilling
// existing rows with 'cache' is not a guess — it is the historical truth, and
// it keeps SyncFromCache's newly scoped DELETEs able to clear them.
var granolaAddedColumns = []struct{ table, column, decl string }{
	{"meetings", "row_source", "TEXT NOT NULL DEFAULT 'cache'"},
	{"attendees", "row_source", "TEXT NOT NULL DEFAULT 'cache'"},
	{"folder_memberships", "row_source", "TEXT NOT NULL DEFAULT 'cache'"},
	{"transcript_segments", "row_source", "TEXT NOT NULL DEFAULT 'cache'"},
	// The catalog tables. These four describe things rather than meetings —
	// folders, panel templates, recipes and their usage counters — and both
	// data paths write them, so they need the same ownership marker for the
	// same reason: a row's creator must stay identifiable after any number of
	// later upserts from the other path.
	{"folders", "row_source", "TEXT NOT NULL DEFAULT 'cache'"},
	{"panel_templates", "row_source", "TEXT NOT NULL DEFAULT 'cache'"},
	{"recipes", "row_source", "TEXT NOT NULL DEFAULT 'cache'"},
	{"recipes_usage", "row_source", "TEXT NOT NULL DEFAULT 'cache'"},
	// Folder metadata the cache has always carried on DocumentListMetadata and
	// the write path silently dropped, so `folder list` reported an empty
	// description and a never-favourited folder for every store-backed read.
	// is_favourited is NOT NULL DEFAULT 0 rather than nullable: the cache
	// always states the flag, and "unknown" is not a value any reader wants to
	// render for a boolean.
	{"folders", "description", "TEXT"},
	{"folders", "is_favourited", "INTEGER NOT NULL DEFAULT 0"},
	// Speaker identity the public API can resolve and the cache never
	// carried. Deliberately nullable: cache-sourced rows leave these NULL so
	// downstream commands stay source-agnostic.
	{"transcript_segments", "speaker_name", "TEXT"},
	{"transcript_segments", "diarization_label", "TEXT"},
}

// EnsureSchema runs the additive Granola-specific migrations. Idempotent.
// Call this from any command that touches the granola tables.
func EnsureSchema(ctx context.Context, db *sql.DB) error {
	for _, stmt := range granolaSchemaSQL {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("granola schema: %w: %s", err, firstLine(stmt))
		}
	}
	for _, c := range granolaAddedColumns {
		if err := ensureGranolaColumn(ctx, db, c.table, c.column, c.decl); err != nil {
			return err
		}
	}
	return nil
}

// ensureGranolaColumn adds a column to an existing table when it is missing.
// Mirrors store.ensureColumn (the repo's established idiom for additive
// schema changes) but operates on the *sql.DB the granola tables are opened
// with rather than the store's pinned migration connection.
//
// Skips silently when the table does not exist (the CREATE TABLE above will
// have declared the column already) or when the column is already present.
func ensureGranolaColumn(ctx context.Context, db *sql.DB, table, column, decl string) error {
	var name string
	err := db.QueryRowContext(ctx,
		`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("granola schema: checking table %s: %w", table, err)
	}

	rows, err := db.QueryContext(ctx, fmt.Sprintf(`PRAGMA table_info("%s")`, table))
	if err != nil {
		return fmt.Errorf("granola schema: table_info %s: %w", table, err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notnull, pk int
		var n, typ string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &n, &typ, &notnull, &dflt, &pk); err != nil {
			return fmt.Errorf("granola schema: scan table_info %s: %w", table, err)
		}
		if n == column {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("granola schema: iterating table_info %s: %w", table, err)
	}

	if _, err := db.ExecContext(ctx,
		fmt.Sprintf(`ALTER TABLE "%s" ADD COLUMN "%s" %s`, table, column, decl)); err != nil {
		// A concurrent EnsureSchema may have added the column between the
		// PRAGMA check and this ALTER. SQLite reports that as a plain error
		// ("duplicate column name") that busy_timeout does not retry; the
		// database is in the desired state either way.
		if strings.Contains(err.Error(), "duplicate column name") {
			return nil
		}
		return fmt.Errorf("granola schema: add column %s.%s: %w", table, column, err)
	}
	return nil
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// SyncResult summarizes a SyncFromCache run.
type SyncResult struct {
	Meetings     int `json:"meetings"`
	Attendees    int `json:"attendees"`
	Segments     int `json:"segments"`
	Folders      int `json:"folders"`
	Memberships  int `json:"folder_memberships"`
	Panels       int `json:"panel_templates"`
	Recipes      int `json:"recipes"`
	Workspaces   int `json:"workspaces"`
	ChatThreads  int `json:"chat_threads"`
	ChatMessages int `json:"chat_messages"`

	// UnparsedTimestamps counts transcript segment timestamps this run could
	// not parse; TimestampWarning is the operator-facing line describing them.
	// Both are zero/empty on a healthy sync. See timestampFailures.
	UnparsedTimestamps int    `json:"unparsed_timestamps,omitempty"`
	TimestampWarning   string `json:"timestamp_warning,omitempty"`

	// PreservedTranscripts counts meetings whose stored transcript this run
	// left alone because the other sync path's copy was larger;
	// PreservationWarning names them. Same non-fatal reporting channel as the
	// timestamp fields. See transcriptPreservations.
	PreservedTranscripts int    `json:"preserved_transcripts,omitempty"`
	PreservationWarning  string `json:"preservation_warning,omitempty"`
}

// SyncFromCache pushes every row from cache into the SQLite store. Uses
// REPLACE semantics so re-running is idempotent. Single transaction so
// readers never see a partial state.
// SyncOptions controls how a cache sync writes into the store.
type SyncOptions struct {
	// Degraded reports that the desktop cache could not be decrypted, so
	// Documents was hydrated from /v2/get-documents into an otherwise empty
	// Cache. It changes what an absent cache-derived row means: on a healthy
	// run "absent" means deleted upstream and the cache path retires its own
	// stale rows; on a degraded run it means unreadable, and retiring anything
	// would destroy rows that can never be re-synced once the DEK is gone.
	//
	// Degraded therefore suppresses the cache path's own-source retirement --
	// the cache-scoped folder_memberships clear and the zero-incoming segment
	// clear -- leaving the run upsert-only. It does not touch the other path's
	// row_source scoping, which guards a different failure.
	Degraded bool

	// PATCH(api-transcript-hydrate): who owns the transcript rows this run
	// writes. Empty means RowSourceCache, the historical behavior.
	//
	// On a degraded run the transcripts did not come from the cache -- they
	// were fetched from the API into an otherwise-empty Cache -- so stamping
	// them 'cache' would misreport their provenance and, worse, feed
	// prepareSegmentRewrite's cross-source comparison a false "the cache holds
	// N segments" that could block a later genuine API refresh.
	TranscriptOwner string

	// CatalogOwner is who owns the catalog rows this run creates: folders,
	// panel_templates, recipes and recipes_usage. Empty means RowSourceCache,
	// the historical behavior. Shaped exactly like TranscriptOwner above, and
	// for the same reason.
	//
	// This is a field rather than a literal at each write site because the
	// catalog is not cache-only property: an API-driven refresh writes the
	// same four tables and has to be able to claim the rows it creates. A
	// hard-coded 'cache' at four call sites leaves that caller nowhere to hook
	// in, and mislabelling API-created rows as cache-owned is unrecoverable --
	// once the marker is wrong nothing downstream can tell the two apart.
	//
	// It only decides ownership of rows this run CREATES. Every catalog upsert
	// leaves row_source out of its ON CONFLICT SET list, so an existing row
	// keeps the owner that created it no matter which path touches it next.
	CatalogOwner string
}

// transcriptOwner resolves the row_source for transcript writes.
func (o SyncOptions) transcriptOwner() string {
	if o.TranscriptOwner != "" {
		return o.TranscriptOwner
	}
	return RowSourceCache
}

// catalogOwner resolves the row_source for folder / panel / recipe writes.
func (o SyncOptions) catalogOwner() string {
	if o.CatalogOwner != "" {
		return o.CatalogOwner
	}
	return RowSourceCache
}

// SyncFromCache writes a fully-read desktop cache into the store.
func SyncFromCache(ctx context.Context, db *sql.DB, cache *Cache) (SyncResult, error) {
	return SyncFromCacheWithOptions(ctx, db, cache, SyncOptions{})
}

// SyncFromCacheWithOptions is SyncFromCache with explicit options. See
// SyncOptions.Degraded for the one behavioral difference.
func SyncFromCacheWithOptions(ctx context.Context, db *sql.DB, cache *Cache, opts SyncOptions) (SyncResult, error) {
	var res SyncResult
	if cache == nil {
		return res, fmt.Errorf("nil cache")
	}
	if err := EnsureSchema(ctx, db); err != nil {
		return res, err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return res, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	var badTimestamps timestampFailures
	var keptTranscripts transcriptPreservations

	// meetings + attendees + transcripts
	for id, doc := range cache.Documents {
		recipesJSON, _ := json.Marshal([]string{}) // filled below if usage info exists
		startedAt, endedAt := meetingTimeWindow(&doc, cache.TranscriptByID(id))
		transcriptAvail := 0
		if len(cache.TranscriptByID(id)) > 0 {
			transcriptAvail = 1
		}
		calEvent := ""
		if doc.GoogleCalendarEvent != nil {
			calEvent = doc.GoogleCalendarEvent.ID
		}
		deletedAt := ""
		if doc.DeletedAt != nil {
			deletedAt = *doc.DeletedAt
		}
		valid := 0
		if doc.ValidMeeting {
			valid = 1
		}
		// PATCH(api-detail-hydrate): merge instead of REPLACE. SQLite's
		// REPLACE deletes the existing row and inserts a new one, so on a
		// meeting both paths know it flipped row_source from 'api' to
		// 'cache' and blanked every field only the API carries. That
		// silently transferred ownership and discarded API data, breaking
		// the documented "each path clears only the rows it owns"
		// guarantee. ON CONFLICT DO UPDATE merges, and row_source is
		// deliberately absent from the SET list so the creating path keeps
		// ownership.
		_, err := tx.ExecContext(ctx, `INSERT INTO meetings(
			id, title, created_at, updated_at, started_at, ended_at, workspace_id,
			calendar_event_id, deleted_at, notes_markdown, notes_plain,
			transcript_available, recipes_applied, creation_source, valid_meeting,
			row_source
		) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET
			title                = COALESCE(NULLIF(excluded.title, ''), meetings.title),
			created_at           = COALESCE(NULLIF(excluded.created_at, ''), meetings.created_at),
			updated_at           = COALESCE(NULLIF(excluded.updated_at, ''), meetings.updated_at),
			started_at           = COALESCE(NULLIF(excluded.started_at, ''), meetings.started_at),
			ended_at             = COALESCE(NULLIF(excluded.ended_at, ''), meetings.ended_at),
			workspace_id         = COALESCE(NULLIF(excluded.workspace_id, ''), meetings.workspace_id),
			calendar_event_id    = COALESCE(NULLIF(excluded.calendar_event_id, ''), meetings.calendar_event_id),
			deleted_at           = COALESCE(NULLIF(excluded.deleted_at, ''), meetings.deleted_at),
			notes_markdown       = COALESCE(NULLIF(excluded.notes_markdown, ''), meetings.notes_markdown),
			notes_plain          = COALESCE(NULLIF(excluded.notes_plain, ''), meetings.notes_plain),
			transcript_available = MAX(excluded.transcript_available, meetings.transcript_available),
			recipes_applied      = COALESCE(NULLIF(excluded.recipes_applied, ''), meetings.recipes_applied),
			creation_source      = COALESCE(NULLIF(excluded.creation_source, ''), meetings.creation_source),
			valid_meeting        = MAX(excluded.valid_meeting, meetings.valid_meeting)`,
			id, doc.Title, doc.CreatedAt, doc.UpdatedAt, startedAt, endedAt,
			doc.WorkspaceID, calEvent, deletedAt, doc.NotesMarkdown, doc.NotesPlain,
			transcriptAvail, string(recipesJSON), doc.CreationSource, valid,
			RowSourceCache,
		)
		if err != nil {
			return res, fmt.Errorf("upsert meeting %s: %w", id, err)
		}
		res.Meetings++

		// Attendees: union of doc.people.attendees and meetingsMetadata[id].attendees
		emails := map[string]CalendarInvitee{}
		if doc.People != nil {
			for _, p := range doc.People.Attendees {
				if p.Email == "" {
					continue
				}
				emails[strings.ToLower(p.Email)] = CalendarInvitee{Name: p.Name, Email: p.Email, ResponseStatus: p.ResponseStatus}
			}
		}
		if md := cache.MeetingMetadataByID(id); md != nil {
			for _, p := range md.Attendees {
				if p.Email == "" {
					continue
				}
				emails[strings.ToLower(p.Email)] = p
			}
		}
		for em, a := range emails {
			// PATCH(api-detail-hydrate): merge, never REPLACE — see the
			// meetings upsert above. row_source stays out of the SET list
			// so the creating path keeps ownership of the row.
			_, err := tx.ExecContext(ctx, `INSERT INTO attendees(meeting_id,email,name,response_status,row_source) VALUES (?,?,?,?,?)
			ON CONFLICT(meeting_id,email) DO UPDATE SET
				name            = COALESCE(NULLIF(excluded.name, ''), attendees.name),
				response_status = COALESCE(NULLIF(excluded.response_status, ''), attendees.response_status)`, id, em, a.Name, a.ResponseStatus, RowSourceCache)
			if err != nil {
				return res, fmt.Errorf("upsert attendee %s/%s: %w", id, em, err)
			}
			res.Attendees++
		}

		// Transcript segments
		segs := cache.TranscriptByID(id)
		// PATCH(api-detail-hydrate): scope the clear by row ownership.
		//
		// Clearing is still needed to avoid stale tails when a transcript was
		// truncated, but an unscoped per-meeting DELETE destroyed API-hydrated
		// transcripts: openGranolaCache backfills cache.Documents from the
		// meetings table, so this loop iterates API-only meetings for which
		// the cache holds no transcript at all, deleting their segments and
		// inserting nothing.
		//
		// PATCH(transcript-retention-preserves-larger): and when the cache's
		// copy is smaller than the API's, this meeting is skipped outright
		// rather than allowed to take ownership. See prepareSegmentRewrite.
		//
		// PATCH(api-sync-survives-unreadable-cache): on a degraded run the
		// cache is unreadable, not empty, so len(segs) == 0 for every meeting.
		// Falling through would hand prepareSegmentRewrite incoming == 0 and
		// delete this meeting's stored cache-owned transcript -- for every
		// meeting the API hydrate returned. Skip instead: there is nothing to
		// write and nothing legitimate to retire.
		if opts.Degraded && len(segs) == 0 {
			continue
		}
		preserved, err := prepareSegmentRewrite(ctx, tx, id, opts.transcriptOwner(), len(segs))
		if err != nil {
			return res, err
		}
		if preserved > 0 {
			keptTranscripts.record(id, preserved, len(segs))
			continue
		}
		for i, seg := range segs {
			startMs := badTimestamps.parse(seg.StartTimestamp, id, i, "start_timestamp")
			endMs := badTimestamps.parse(seg.EndTimestamp, id, i, "end_timestamp")
			// speaker_name / diarization_label stay NULL: the desktop cache
			// never carried resolved speaker identity.
			_, err := tx.ExecContext(ctx, `INSERT INTO transcript_segments(meeting_id,idx,source,text,start_ts_ms,end_ts_ms,confidence,row_source) VALUES (?,?,?,?,?,?,?,?)`,
				id, i, seg.Source, seg.Text, startMs, endMs, seg.Confidence, opts.transcriptOwner())
			if err != nil {
				return res, fmt.Errorf("upsert segment %s/%d: %w", id, i, err)
			}
			res.Segments++
		}
	}

	// folders + memberships
	// PATCH(api-detail-hydrate): this DELETE used to be unscoped, which wiped
	// every API-hydrated folder membership on each cache sync — including
	// memberships for meetings the cache has never heard of.
	//
	// PATCH(api-sync-survives-unreadable-cache): skipped entirely on a degraded
	// run. cache.DocumentLists is empty because the cache could not be read, so
	// this clear would delete every cache-owned membership and re-insert none.
	if !opts.Degraded {
		if _, err := tx.ExecContext(ctx, `DELETE FROM folder_memberships WHERE row_source = ?`, RowSourceCache); err != nil {
			return res, err
		}
	}
	for fid, md := range cache.DocumentListsMetadata {
		// PATCH(catalog-provenance-and-merge): merge, never REPLACE — see the
		// meetings upsert above. REPLACE deleted the row and inserted a fresh
		// one, which blanked every column this payload does not carry and
		// silently reassigned row_source to whichever path ran last.
		//
		// is_favourited cannot use the NULLIF idiom: it is a boolean, so
		// "false" and "absent" are the same value and no expression over the
		// incoming column alone can tell them apart. Provenance can. Only the
		// path that owns the row is allowed to clear the flag, which keeps a
		// user's un-favourite working on the path that knows about it while
		// stopping a writer that simply does not carry the field from silently
		// resetting the other path's value.
		favourited := 0
		if md.IsFavourited {
			favourited = 1
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO folders(
			id, title, parent_id, workspace_id, owner_id, preset,
			description, is_favourited, row_source
		) VALUES (?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET
			title         = COALESCE(NULLIF(excluded.title, ''), folders.title),
			parent_id     = COALESCE(NULLIF(excluded.parent_id, ''), folders.parent_id),
			workspace_id  = COALESCE(NULLIF(excluded.workspace_id, ''), folders.workspace_id),
			owner_id      = COALESCE(NULLIF(excluded.owner_id, ''), folders.owner_id),
			preset        = COALESCE(NULLIF(excluded.preset, ''), folders.preset),
			description   = COALESCE(NULLIF(excluded.description, ''), folders.description),
			is_favourited = CASE WHEN excluded.row_source = folders.row_source
				THEN excluded.is_favourited ELSE folders.is_favourited END`,
			fid, md.Title, md.ParentDocumentListID, md.WorkspaceID, "", md.Preset,
			md.Description, favourited, opts.catalogOwner())
		if err != nil {
			return res, fmt.Errorf("upsert folder %s: %w", fid, err)
		}
		res.Folders++
	}
	for fid, mids := range cache.DocumentLists {
		for _, mid := range mids {
			// PATCH(api-detail-hydrate): merge, never REPLACE — see above.
			// The row carries no payload beyond its key pair, so an existing
			// row is left entirely alone rather than having its ownership
			// rewritten.
			_, err := tx.ExecContext(ctx, `INSERT INTO folder_memberships(folder_id,meeting_id,row_source) VALUES (?,?,?)
			ON CONFLICT(folder_id,meeting_id) DO NOTHING`, fid, mid, RowSourceCache)
			if err != nil {
				return res, fmt.Errorf("upsert folder membership %s/%s: %w", fid, mid, err)
			}
			res.Memberships++
		}
	}

	// panel_templates
	for _, p := range cache.PanelTemplates {
		// PATCH(catalog-provenance-and-merge): merge, never REPLACE — see the
		// folders upsert above.
		_, err := tx.ExecContext(ctx, `INSERT INTO panel_templates(id,slug,title,description,category,row_source) VALUES (?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET
			slug        = COALESCE(NULLIF(excluded.slug, ''), panel_templates.slug),
			title       = COALESCE(NULLIF(excluded.title, ''), panel_templates.title),
			description = COALESCE(NULLIF(excluded.description, ''), panel_templates.description),
			category    = COALESCE(NULLIF(excluded.category, ''), panel_templates.category)`,
			p.ID, p.Slug, p.Title, p.Description, p.Category, opts.catalogOwner())
		if err != nil {
			return res, fmt.Errorf("upsert panel %s: %w", p.ID, err)
		}
		res.Panels++
	}

	// recipes
	for _, r := range cache.RecipesAll() {
		// PATCH(catalog-provenance-and-merge): merge, never REPLACE — see the
		// folders upsert above. `source` here is the recipe's origin collection
		// (public/user/shared), which is unrelated to row_source: one says
		// which Granola list the recipe came from, the other which of this
		// CLI's two data paths wrote the row.
		_, err := tx.ExecContext(ctx, `INSERT INTO recipes(id,slug,name,description,category,source,row_source) VALUES (?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET
			slug        = COALESCE(NULLIF(excluded.slug, ''), recipes.slug),
			name        = COALESCE(NULLIF(excluded.name, ''), recipes.name),
			description = COALESCE(NULLIF(excluded.description, ''), recipes.description),
			category    = COALESCE(NULLIF(excluded.category, ''), recipes.category),
			source      = COALESCE(NULLIF(excluded.source, ''), recipes.source)`,
			r.ID, r.Slug, r.Name, r.Config.Description, r.Category, r.Source, opts.catalogOwner())
		if err != nil {
			return res, fmt.Errorf("upsert recipe %s: %w", r.ID, err)
		}
		res.Recipes++
	}

	// recipes_usage
	for rid, u := range cache.RecipesUsage {
		count := int64(0)
		fmt.Sscanf(u.TotalCount, "%d", &count)
		// PATCH(catalog-provenance-and-merge): merge, never REPLACE — see the
		// folders upsert above. NULLIF(...,0) is the integer form of the same
		// idiom: TotalCount is a string on the cache record and an absent or
		// unparseable one leaves count at 0, which must not overwrite a stored
		// total. A genuine decrease still lands, so this is not a latch.
		_, err := tx.ExecContext(ctx, `INSERT INTO recipes_usage(recipe_id,total_count,last_used_at,row_source) VALUES (?,?,?,?)
		ON CONFLICT(recipe_id) DO UPDATE SET
			total_count  = COALESCE(NULLIF(excluded.total_count, 0), recipes_usage.total_count),
			last_used_at = COALESCE(NULLIF(excluded.last_used_at, ''), recipes_usage.last_used_at)`,
			rid, count, u.LastUsedAt, opts.catalogOwner())
		if err != nil {
			return res, fmt.Errorf("upsert recipe usage %s: %w", rid, err)
		}
	}

	// workspaces
	for _, w := range cache.Workspaces {
		// Try to extract id + display_name from the raw workspace blob.
		var inner struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			DisplayName string `json:"display_name"`
		}
		_ = json.Unmarshal(w.Workspace, &inner)
		id := inner.ID
		name := inner.DisplayName
		if name == "" {
			name = inner.Name
		}
		if id == "" {
			// Skip un-identifiable workspace entries.
			continue
		}
		_, err := tx.ExecContext(ctx, `INSERT OR REPLACE INTO workspaces(id,display_name,plan_type,role,raw) VALUES (?,?,?,?,?)`,
			id, name, w.PlanType, w.Role, string(w.Workspace))
		if err != nil {
			return res, fmt.Errorf("upsert workspace %s: %w", id, err)
		}
		res.Workspaces++
	}

	// chat_threads
	for tid, t := range cache.ChatThreads {
		_, err := tx.ExecContext(ctx, `INSERT OR REPLACE INTO chat_threads(id,meeting_id,workspace_id,title,created_at,updated_at) VALUES (?,?,?,?,?,?)`,
			tid, t.Data.DocumentID, t.WorkspaceID, t.Data.Title, t.CreatedAt, t.UpdatedAt)
		if err != nil {
			return res, fmt.Errorf("upsert chat thread %s: %w", tid, err)
		}
		res.ChatThreads++
	}

	// chat_messages
	for mid, m := range cache.ChatMessages {
		_, err := tx.ExecContext(ctx, `INSERT OR REPLACE INTO chat_messages(id,thread_id,role,turn_index,content,created_at) VALUES (?,?,?,?,?,?)`,
			mid, m.Data.ThreadID, m.Data.Role, m.Data.TurnIndex, m.Data.RawText, m.CreatedAt)
		if err != nil {
			return res, fmt.Errorf("upsert chat message %s: %w", mid, err)
		}
		res.ChatMessages++
	}

	// Rebuild FTS indexes — drop and re-populate. INSERT INTO ... SELECT
	// is the FTS5 idiomatic populate-from-content-table pattern.
	if _, err := tx.ExecContext(ctx, `INSERT INTO meetings_fts(meetings_fts) VALUES ('rebuild')`); err != nil {
		return res, fmt.Errorf("rebuild meetings_fts: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO transcript_fts(transcript_fts) VALUES ('rebuild')`); err != nil {
		return res, fmt.Errorf("rebuild transcript_fts: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return res, err
	}
	committed = true
	res.UnparsedTimestamps = badTimestamps.count
	res.TimestampWarning = badTimestamps.warning()
	res.PreservedTranscripts = keptTranscripts.count()
	res.PreservationWarning = keptTranscripts.warning()
	return res, nil
}

// prepareSegmentRewrite decides what happens to a meeting's stored transcript
// when owner is about to write incoming segments for it, and performs the
// delete half of that decision.
//
// The return value is the number of segments preserved: 0 means "go ahead and
// write", and any positive value means the store already holds a LARGER
// transcript from the other path, nothing was deleted, and the caller must
// skip this meeting entirely.
//
// PATCH(transcript-retention-preserves-larger): the rule used to be "any
// non-empty incoming transcript is authoritative", which ran an unscoped
// per-meeting DELETE and rewrote the meeting from the incoming payload. That
// treats non-empty as complete, and it is not: Granola applies transcript
// retention upstream, while this local store is expected to outlive it. A
// meeting captured live and cache-synced with hundreds of segments comes back
// from the public API weeks later pruned to a handful — still non-empty — and
// the sync destroyed the complete local copy to store the remnant.
//
// Three cases, in the order they are decided:
//
//   - incoming == 0: nothing to write, so only this path's own stale rows are
//     retired and the other path's transcript survives untouched. Unchanged.
//   - the other path holds MORE segments than are incoming: preserve. Deleting
//     is the destructive half, so declining to delete is the whole fix; the
//     caller skips its own smaller write, because writing it on top would
//     either collide on the (meeting_id, idx) primary key or leave one
//     meeting's transcript interleaved across two sources.
//   - otherwise (incoming is at least as complete, or the other path holds
//     nothing at all): take ownership of the meeting and rewrite it, exactly as
//     before. A path replacing its OWN earlier transcript always lands here
//     whatever the size change, because the count is scoped to the other
//     source — a shrinking same-source rewrite is a normal upstream edit, not
//     a cross-source conflict.
func prepareSegmentRewrite(ctx context.Context, tx *sql.Tx, meetingID, owner string, incoming int) (int, error) {
	if incoming == 0 {
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM transcript_segments WHERE meeting_id = ? AND row_source = ?`, meetingID, owner); err != nil {
			return 0, fmt.Errorf("clear segments %s: %w", meetingID, err)
		}
		return 0, nil
	}
	var other int
	if err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM transcript_segments WHERE meeting_id = ? AND row_source <> ?`,
		meetingID, owner).Scan(&other); err != nil {
		return 0, fmt.Errorf("count foreign segments %s: %w", meetingID, err)
	}
	if other > incoming {
		return other, nil
	}
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM transcript_segments WHERE meeting_id = ?`, meetingID); err != nil {
		return 0, fmt.Errorf("clear segments %s: %w", meetingID, err)
	}
	return 0, nil
}

// transcriptPreservations accumulates the meetings prepareSegmentRewrite told
// a sync path to skip because the other path's stored transcript was larger.
//
// PATCH(transcript-retention-preserves-larger): the point of the accounting is
// that the outcome cannot be silent. Skipping a write is the right call, but a
// sync that reports "500 segments" one week and nothing the next, with no line
// explaining why, is indistinguishable from a broken sync. Deliberately not
// fatal, and deliberately shaped like timestampFailures: one count plus one
// operator-facing line, on the same non-fatal channel.
type transcriptPreservations struct {
	meetings []string
	kept     int
	skipped  int
}

// record notes one preserved meeting: kept is the segment count that stayed in
// the store, incoming the smaller count this path did not write.
func (p *transcriptPreservations) record(meetingID string, kept, incoming int) {
	p.meetings = append(p.meetings, meetingID)
	p.kept += kept
	p.skipped += incoming
}

// count is the number of meetings whose stored transcript was preserved.
func (p *transcriptPreservations) count() int { return len(p.meetings) }

// preservationWarningMaxIDs bounds how many meeting ids the warning names. A
// full non-incremental sync can touch hundreds of meetings, and an unbounded
// id list would turn one warning line into a wall of text.
const preservationWarningMaxIDs = 10

// warning renders the operator-facing line, or "" when nothing was preserved.
func (p *transcriptPreservations) warning() string {
	if len(p.meetings) == 0 {
		return ""
	}
	ids, more := p.meetings, ""
	if len(ids) > preservationWarningMaxIDs {
		ids = ids[:preservationWarningMaxIDs]
		more = fmt.Sprintf(" (+%d more)", len(p.meetings)-preservationWarningMaxIDs)
	}
	return fmt.Sprintf("%d transcript(s) were kept as stored because the other sync source holds a larger copy: "+
		"%d stored segment(s) preserved, %d incoming segment(s) not written. "+
		"This is expected when upstream transcript retention has pruned a meeting this store still has in full. "+
		"Meetings: %s%s", len(p.meetings), p.kept, p.skipped, strings.Join(ids, ", "), more)
}

// APISyncResult summarizes a SyncFromAPI run.
type APISyncResult struct {
	Meetings    int `json:"meetings"`
	Attendees   int `json:"attendees"`
	Segments    int `json:"transcript_segments"`
	Folders     int `json:"folders"`
	Memberships int `json:"folder_memberships"`
	Summaries   int `json:"summaries"`
	Events      int `json:"calendar_events"`

	// UnparsedTimestamps / TimestampWarning carry the same accounting
	// SyncResult does — see those fields and timestampFailures.
	UnparsedTimestamps int    `json:"unparsed_timestamps,omitempty"`
	TimestampWarning   string `json:"timestamp_warning,omitempty"`

	// PreservedTranscripts / PreservationWarning carry the same accounting
	// SyncResult does — see those fields and transcriptPreservations.
	PreservedTranscripts int    `json:"preserved_transcripts,omitempty"`
	PreservationWarning  string `json:"preservation_warning,omitempty"`
}

// SyncFromAPI hydrates per-note detail fetched from the public REST API into
// the same Granola domain tables SyncFromCache writes — meetings, attendees,
// transcript_segments, folders, folder_memberships — so the read commands
// find API-sourced data exactly where they find cache-sourced data.
//
// This deliberately does NOT touch the generator-owned generic tables
// (resources / notes). Nothing reads those; hydrating them would produce a
// store that looks populated while every meeting command still reports
// nothing.
//
// Everything runs in one transaction so a failure mid-way leaves the store at
// its previous state rather than half-hydrated.
//
// Row ownership: every row written here is stamped row_source='api', and the
// only DELETEs are scoped to that marker (or to a meeting this call is
// actively rewriting). Cache-sourced rows survive.
func SyncFromAPI(ctx context.Context, db *sql.DB, notes []APINote) (APISyncResult, error) {
	var res APISyncResult
	if err := EnsureSchema(ctx, db); err != nil {
		return res, err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return res, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	var badTimestamps timestampFailures
	var keptTranscripts transcriptPreservations
	for i := range notes {
		n := &notes[i]
		if n.ID == "" {
			continue
		}
		if err := upsertAPINote(ctx, tx, n, &res, &badTimestamps, &keptTranscripts); err != nil {
			return res, err
		}
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO meetings_fts(meetings_fts) VALUES ('rebuild')`); err != nil {
		return res, fmt.Errorf("rebuild meetings_fts: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO transcript_fts(transcript_fts) VALUES ('rebuild')`); err != nil {
		return res, fmt.Errorf("rebuild transcript_fts: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return res, err
	}
	committed = true
	res.UnparsedTimestamps = badTimestamps.count
	res.TimestampWarning = badTimestamps.warning()
	res.PreservedTranscripts = keptTranscripts.count()
	res.PreservationWarning = keptTranscripts.warning()
	return res, nil
}

// upsertAPINote writes one note's detail across every domain table.
func upsertAPINote(ctx context.Context, tx *sql.Tx, n *APINote, res *APISyncResult, bad *timestampFailures, kept *transcriptPreservations) error {
	startedAt, endedAt := apiTimeWindow(n)
	calEventID := ""
	if n.CalendarEvent != nil {
		calEventID = n.CalendarEvent.CalendarEventID
		res.Events++
	}
	transcriptAvail := 0
	if len(n.Transcript) > 0 {
		transcriptAvail = 1
	}
	if n.SummaryMarkdown != "" || n.SummaryText != "" {
		res.Summaries++
	}
	title := n.Title
	if title == "" && n.CalendarEvent != nil {
		title = n.CalendarEvent.EventTitle
	}

	// summary_markdown / summary_text land in notes_markdown / notes_plain:
	// those are the columns the meeting read path (show, export, memo,
	// notes-show, the cache backfill) already consumes as the meeting body.
	//
	// ON CONFLICT DO UPDATE rather than INSERT OR REPLACE for two reasons:
	// REPLACE reallocates the rowid, which desynchronises the external-content
	// meetings_fts index; and it would blank the cache-only columns
	// (workspace_id, recipes_applied, creation_source) for any meeting both
	// paths know about. COALESCE(NULLIF(...)) means an empty value from this
	// source never overwrites a populated one from the other. row_source is
	// intentionally left alone on conflict: whichever path created the row
	// keeps ownership, so neither path's scoped DELETE can reach the other's.
	_, err := tx.ExecContext(ctx, `INSERT INTO meetings(
		id, title, created_at, updated_at, started_at, ended_at, workspace_id,
		calendar_event_id, deleted_at, notes_markdown, notes_plain,
		transcript_available, recipes_applied, creation_source, valid_meeting,
		row_source
	) VALUES (?,?,?,?,?,?,'',?,'',?,?,?,'[]','granola_api',1,?)
	ON CONFLICT(id) DO UPDATE SET
		title                = COALESCE(NULLIF(excluded.title,''), meetings.title),
		created_at           = COALESCE(NULLIF(excluded.created_at,''), meetings.created_at),
		updated_at           = COALESCE(NULLIF(excluded.updated_at,''), meetings.updated_at),
		started_at           = COALESCE(NULLIF(excluded.started_at,''), meetings.started_at),
		ended_at             = COALESCE(NULLIF(excluded.ended_at,''), meetings.ended_at),
		calendar_event_id    = COALESCE(NULLIF(excluded.calendar_event_id,''), meetings.calendar_event_id),
		notes_markdown       = COALESCE(NULLIF(excluded.notes_markdown,''), meetings.notes_markdown),
		notes_plain          = COALESCE(NULLIF(excluded.notes_plain,''), meetings.notes_plain),
		transcript_available = MAX(meetings.transcript_available, excluded.transcript_available)`,
		n.ID, title, n.CreatedAt, n.UpdatedAt, startedAt, endedAt,
		calEventID, n.SummaryMarkdown, n.SummaryText, transcriptAvail, RowSourceAPI,
	)
	if err != nil {
		return fmt.Errorf("upsert api meeting %s: %w", n.ID, err)
	}
	res.Meetings++

	if err := upsertAPIAttendees(ctx, tx, n, res); err != nil {
		return err
	}
	if err := upsertAPIMemberships(ctx, tx, n, res); err != nil {
		return err
	}
	return upsertAPITranscript(ctx, tx, n, res, bad, kept)
}

// upsertAPIAttendees reconciles the note's attendees[] with its
// calendar_event.invitees[] on email — the store's key is
// PRIMARY KEY (meeting_id, email). Invitees carry email only; attendees carry
// name and email, so the attendee record's name wins whenever both describe
// the same address.
func upsertAPIAttendees(ctx context.Context, tx *sql.Tx, n *APINote, res *APISyncResult) error {
	// Drop this path's stale rows first so an attendee removed upstream does
	// not linger. Scoped to row_source so cache-owned attendees survive.
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM attendees WHERE meeting_id = ? AND row_source = ?`, n.ID, RowSourceAPI); err != nil {
		return fmt.Errorf("clear api attendees %s: %w", n.ID, err)
	}

	byEmail := map[string]string{} // lowercased email -> best known name
	order := []string{}
	add := func(email, name string) {
		email = strings.ToLower(strings.TrimSpace(email))
		if email == "" {
			return
		}
		existing, seen := byEmail[email]
		if !seen {
			order = append(order, email)
			byEmail[email] = name
			return
		}
		if existing == "" && name != "" {
			byEmail[email] = name
		}
	}
	// Invitees first (email only), then attendees, then the organiser. The
	// add() helper only upgrades a blank name, so a named attendee record
	// always beats the nameless invitee entry for the same address.
	if n.CalendarEvent != nil {
		for _, inv := range n.CalendarEvent.Invitees {
			add(inv.Email, inv.Name)
		}
	}
	for _, a := range n.Attendees {
		add(a.Email, a.Name)
	}
	if n.CalendarEvent != nil {
		add(n.CalendarEvent.OrganiserEmail(), n.CalendarEvent.OrganiserName())
	}

	for _, email := range order {
		// response_status is left untouched on conflict: the public API does
		// not carry RSVP state, and blanking a cache-supplied value would be
		// a regression.
		_, err := tx.ExecContext(ctx, `INSERT INTO attendees(meeting_id,email,name,response_status,row_source)
			VALUES (?,?,?,'',?)
			ON CONFLICT(meeting_id,email) DO UPDATE SET
				name = COALESCE(NULLIF(excluded.name,''), attendees.name)`,
			n.ID, email, byEmail[email], RowSourceAPI)
		if err != nil {
			return fmt.Errorf("upsert api attendee %s/%s: %w", n.ID, email, err)
		}
		res.Attendees++
	}
	return nil
}

// upsertAPIMemberships writes folder_membership and space_membership entries
// into folder_memberships. Both collections are container→note edges, and the
// folder commands read them from the same table.
func upsertAPIMemberships(ctx context.Context, tx *sql.Tx, n *APINote, res *APISyncResult) error {
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM folder_memberships WHERE meeting_id = ? AND row_source = ?`, n.ID, RowSourceAPI); err != nil {
		return fmt.Errorf("clear api memberships %s: %w", n.ID, err)
	}
	seen := map[string]bool{}
	for _, m := range append(append([]APIMembership{}, n.FolderMembership...), n.SpaceMembership...) {
		fid := m.Ident()
		if fid == "" || seen[fid] {
			continue
		}
		seen[fid] = true
		// Create the folder row when it is missing, but never clobber
		// cache-supplied folder metadata (parent_id, workspace_id, preset,
		// description, is_favourited) that the API does not carry — which is
		// why the SET list names title and nothing else.
		//
		// PATCH(catalog-provenance-and-merge): stamp row_source='api' on
		// creation. This path has always created folder rows, so without the
		// stamp the first run after the migration would file every genuinely
		// API-created folder under the column's 'cache' default and nothing
		// afterwards could tell the two apart. row_source stays out of the SET
		// list for the usual reason: a cache-created folder the API also knows
		// about keeps its cache ownership.
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO folders(id,title,parent_id,workspace_id,owner_id,preset,description,is_favourited,row_source)
			 VALUES (?,?,'','','','','',0,?)
			 ON CONFLICT(id) DO UPDATE SET title = COALESCE(NULLIF(folders.title,''), excluded.title)`,
			fid, m.Label(), RowSourceAPI); err != nil {
			return fmt.Errorf("upsert api folder %s: %w", fid, err)
		}
		res.Folders++
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO folder_memberships(folder_id,meeting_id,row_source) VALUES (?,?,?)
			 ON CONFLICT(folder_id,meeting_id) DO NOTHING`,
			fid, n.ID, RowSourceAPI); err != nil {
			return fmt.Errorf("upsert api folder membership %s/%s: %w", fid, n.ID, err)
		}
		res.Memberships++
	}
	return nil
}

// upsertAPITranscript writes the transcript array into transcript_segments in
// the same shape cache-sourced segments use, translating the API's speaker
// vocabulary (see NormalizeSpeakerSource) and carrying the resolved speaker
// identity the cache never had.
//
// A nil/empty transcript is a normal outcome — the note may have none, or the
// caller may not have asked for include=transcript — and is not an error.
//
// PATCH(transcript-retention-preserves-larger): so is a SHORTER one. Upstream
// transcript retention prunes older notes, and this store is expected to
// outlive it, so a note that comes back with fewer segments than the cache
// path already stored is skipped rather than allowed to overwrite the fuller
// local copy. See prepareSegmentRewrite.
func upsertAPITranscript(ctx context.Context, tx *sql.Tx, n *APINote, res *APISyncResult, bad *timestampFailures, kept *transcriptPreservations) error {
	preserved, err := prepareSegmentRewrite(ctx, tx, n.ID, RowSourceAPI, len(n.Transcript))
	if err != nil {
		return err
	}
	if preserved > 0 {
		kept.record(n.ID, preserved, len(n.Transcript))
		return nil
	}
	for i, seg := range n.Transcript {
		startMs := bad.parse(seg.StartTime, n.ID, i, "start_time")
		endMs := bad.parse(seg.EndTime, n.ID, i, "end_time")
		source := ""
		var speakerName, label any
		if seg.Speaker != nil {
			source = NormalizeSpeakerSource(seg.Speaker.Source)
			if nm := seg.Speaker.ResolvedName(); nm != "" {
				speakerName = nm
			}
			if lb := seg.Speaker.ResolvedLabel(); lb != "" {
				label = lb
			}
		}
		// confidence is 0: the public API does not expose a per-segment
		// confidence score. Writing a fabricated 1.0 would make
		// talktime's confidence_avg lie about API-sourced meetings.
		_, err := tx.ExecContext(ctx, `INSERT INTO transcript_segments(
			meeting_id, idx, source, text, start_ts_ms, end_ts_ms, confidence,
			speaker_name, diarization_label, row_source
		) VALUES (?,?,?,?,?,?,0,?,?,?)`,
			n.ID, i, source, seg.Text, startMs, endMs, speakerName, label, RowSourceAPI)
		if err != nil {
			return fmt.Errorf("upsert api segment %s/%d: %w", n.ID, i, err)
		}
		res.Segments++
	}
	return nil
}

// apiTimeWindow derives started_at/ended_at for an API note, preferring the
// calendar event's scheduled window, then the transcript's first/last
// timestamp, then the note's own created/updated timestamps.
func apiTimeWindow(n *APINote) (string, string) {
	startedAt := n.CreatedAt
	endedAt := n.UpdatedAt
	if n.CalendarEvent != nil {
		if s := n.CalendarEvent.ScheduledStartTime; s != "" {
			startedAt = s
		}
		if s := n.CalendarEvent.ScheduledEndTime; s != "" {
			endedAt = s
		}
	}
	if len(n.Transcript) > 0 {
		if startedAt == "" {
			startedAt = n.Transcript[0].StartTime
		}
		if endedAt == "" {
			endedAt = n.Transcript[len(n.Transcript)-1].EndTime
		}
	}
	return startedAt, endedAt
}

// meetingTimeWindow returns started_at/ended_at for a doc, preferring
// google_calendar_event.start/end when present, else falling back to
// the first/last transcript segment timestamp, else created_at.
func meetingTimeWindow(d *Document, segs []TranscriptSegment) (string, string) {
	startedAt := d.CreatedAt
	endedAt := d.UpdatedAt
	if d.GoogleCalendarEvent != nil {
		// google_calendar_event.start/end is shaped {"dateTime":"..."} or {"date":"..."}.
		if s := extractCalTime(d.GoogleCalendarEvent.Start); s != "" {
			startedAt = s
		}
		if s := extractCalTime(d.GoogleCalendarEvent.End); s != "" {
			endedAt = s
		}
	}
	if len(segs) > 0 {
		if startedAt == "" {
			startedAt = segs[0].StartTimestamp
		}
		if endedAt == "" {
			endedAt = segs[len(segs)-1].EndTimestamp
		}
	}
	return startedAt, endedAt
}

func extractCalTime(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var inner struct {
		DateTime string `json:"dateTime"`
		Date     string `json:"date"`
	}
	if err := json.Unmarshal(raw, &inner); err != nil {
		return ""
	}
	if inner.DateTime != "" {
		return inner.DateTime
	}
	return inner.Date
}

// timestampFailures accumulates transcript timestamps a sync could not parse.
//
// PATCH(api-detail-hydrate): both write paths used to call isoToMillis with
// the error discarded (`startMs, _ := ...`). An unparseable timestamp becomes
// 0, and the read path treats start_ts_ms == 0 as "no timestamp", so a format
// change in the live API would silently render every segment blank with no
// signal anywhere. Counting the failures and keeping the first one makes that
// visible in the sync summary. Deliberately not fatal: one malformed segment
// should not fail a whole sync, and per-segment reporting would drown the
// summary on a large transcript.
type timestampFailures struct {
	count   int
	example string
}

// parse converts an ISO timestamp to epoch millis, recording rather than
// swallowing a parse failure. A failure still yields 0 — the segment is worth
// storing without its timestamp — but it is now counted.
func (f *timestampFailures) parse(value, meetingID string, idx int, field string) int64 {
	ms, err := isoToMillis(value)
	if err == nil {
		return ms
	}
	f.count++
	if f.example == "" {
		f.example = fmt.Sprintf("meeting %s segment %d %s: %v", meetingID, idx, field, err)
	}
	return 0
}

// warning renders the operator-facing line, or "" when everything parsed.
func (f *timestampFailures) warning() string {
	if f.count == 0 {
		return ""
	}
	return fmt.Sprintf("%d transcript timestamp(s) could not be parsed and were stored as 0, "+
		"which reads back as no timestamp; first failure: %s", f.count, f.example)
}

func isoToMillis(s string) (int64, error) {
	if s == "" {
		return 0, nil
	}
	t, err := ParseISO(s)
	if err != nil {
		return 0, err
	}
	return t.UnixMilli(), nil
}

// Ensure time import is used.
var _ = time.Now
