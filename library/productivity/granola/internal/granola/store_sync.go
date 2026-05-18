// Copyright 2026 dstevens. Licensed under Apache-2.0. See LICENSE.

package granola

import (
	"context"
	"database/sql"
	"encoding/json"
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
		valid_meeting INTEGER NOT NULL DEFAULT 0
	)`,
	`CREATE INDEX IF NOT EXISTS idx_meetings_started ON meetings(started_at)`,
	`CREATE INDEX IF NOT EXISTS idx_meetings_created ON meetings(created_at)`,
	`CREATE INDEX IF NOT EXISTS idx_meetings_workspace ON meetings(workspace_id)`,

	`CREATE TABLE IF NOT EXISTS attendees (
		meeting_id TEXT NOT NULL,
		email TEXT NOT NULL,
		name TEXT,
		response_status TEXT,
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
		preset TEXT
	)`,

	`CREATE TABLE IF NOT EXISTS folder_memberships (
		folder_id TEXT NOT NULL,
		meeting_id TEXT NOT NULL,
		PRIMARY KEY (folder_id, meeting_id)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_fm_meeting ON folder_memberships(meeting_id)`,

	`CREATE TABLE IF NOT EXISTS panel_templates (
		id TEXT PRIMARY KEY,
		slug TEXT,
		title TEXT,
		description TEXT,
		category TEXT
	)`,
	`CREATE INDEX IF NOT EXISTS idx_panel_templates_slug ON panel_templates(slug)`,

	`CREATE TABLE IF NOT EXISTS recipes (
		id TEXT PRIMARY KEY,
		slug TEXT,
		name TEXT,
		description TEXT,
		category TEXT,
		source TEXT
	)`,
	`CREATE INDEX IF NOT EXISTS idx_recipes_slug ON recipes(slug)`,

	`CREATE TABLE IF NOT EXISTS recipes_usage (
		recipe_id TEXT PRIMARY KEY,
		total_count INTEGER NOT NULL DEFAULT 0,
		last_used_at TEXT
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

// EnsureSchema runs the additive Granola-specific migrations. Idempotent.
// Call this from any command that touches the granola tables.
func EnsureSchema(ctx context.Context, db *sql.DB) error {
	for _, stmt := range granolaSchemaSQL {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("granola schema: %w: %s", err, firstLine(stmt))
		}
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
}

// SyncFromCache pushes every row from cache into the SQLite store. Uses
// REPLACE semantics so re-running is idempotent. Single transaction so
// readers never see a partial state.
func SyncFromCache(ctx context.Context, db *sql.DB, cache *Cache) (SyncResult, error) {
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
		_, err := tx.ExecContext(ctx, `INSERT OR REPLACE INTO meetings(
			id, title, created_at, updated_at, started_at, ended_at, workspace_id,
			calendar_event_id, deleted_at, notes_markdown, notes_plain,
			transcript_available, recipes_applied, creation_source, valid_meeting
		) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			id, doc.Title, doc.CreatedAt, doc.UpdatedAt, startedAt, endedAt,
			doc.WorkspaceID, calEvent, deletedAt, doc.NotesMarkdown, doc.NotesPlain,
			transcriptAvail, string(recipesJSON), doc.CreationSource, valid,
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
			_, err := tx.ExecContext(ctx, `INSERT OR REPLACE INTO attendees(meeting_id,email,name,response_status) VALUES (?,?,?,?)`, id, em, a.Name, a.ResponseStatus)
			if err != nil {
				return res, fmt.Errorf("upsert attendee %s/%s: %w", id, em, err)
			}
			res.Attendees++
		}

		// Transcript segments
		segs := cache.TranscriptByID(id)
		// Clear existing first to avoid stale tails when transcript was truncated.
		if _, err := tx.ExecContext(ctx, `DELETE FROM transcript_segments WHERE meeting_id = ?`, id); err != nil {
			return res, fmt.Errorf("clear segments %s: %w", id, err)
		}
		for i, seg := range segs {
			startMs, _ := isoToMillis(seg.StartTimestamp)
			endMs, _ := isoToMillis(seg.EndTimestamp)
			_, err := tx.ExecContext(ctx, `INSERT INTO transcript_segments(meeting_id,idx,source,text,start_ts_ms,end_ts_ms,confidence) VALUES (?,?,?,?,?,?,?)`,
				id, i, seg.Source, seg.Text, startMs, endMs, seg.Confidence)
			if err != nil {
				return res, fmt.Errorf("upsert segment %s/%d: %w", id, i, err)
			}
			res.Segments++
		}
	}

	// folders + memberships
	if _, err := tx.ExecContext(ctx, `DELETE FROM folder_memberships`); err != nil {
		return res, err
	}
	for fid, md := range cache.DocumentListsMetadata {
		_, err := tx.ExecContext(ctx, `INSERT OR REPLACE INTO folders(id,title,parent_id,workspace_id,owner_id,preset) VALUES (?,?,?,?,?,?)`,
			fid, md.Title, md.ParentDocumentListID, md.WorkspaceID, "", md.Preset)
		if err != nil {
			return res, fmt.Errorf("upsert folder %s: %w", fid, err)
		}
		res.Folders++
	}
	for fid, mids := range cache.DocumentLists {
		for _, mid := range mids {
			_, err := tx.ExecContext(ctx, `INSERT OR REPLACE INTO folder_memberships(folder_id,meeting_id) VALUES (?,?)`, fid, mid)
			if err != nil {
				return res, fmt.Errorf("upsert folder membership %s/%s: %w", fid, mid, err)
			}
			res.Memberships++
		}
	}

	// panel_templates
	for _, p := range cache.PanelTemplates {
		_, err := tx.ExecContext(ctx, `INSERT OR REPLACE INTO panel_templates(id,slug,title,description,category) VALUES (?,?,?,?,?)`,
			p.ID, p.Slug, p.Title, p.Description, p.Category)
		if err != nil {
			return res, fmt.Errorf("upsert panel %s: %w", p.ID, err)
		}
		res.Panels++
	}

	// recipes
	for _, r := range cache.RecipesAll() {
		_, err := tx.ExecContext(ctx, `INSERT OR REPLACE INTO recipes(id,slug,name,description,category,source) VALUES (?,?,?,?,?,?)`,
			r.ID, r.Slug, r.Name, r.Config.Description, r.Category, r.Source)
		if err != nil {
			return res, fmt.Errorf("upsert recipe %s: %w", r.ID, err)
		}
		res.Recipes++
	}

	// recipes_usage
	for rid, u := range cache.RecipesUsage {
		count := int64(0)
		fmt.Sscanf(u.TotalCount, "%d", &count)
		_, err := tx.ExecContext(ctx, `INSERT OR REPLACE INTO recipes_usage(recipe_id,total_count,last_used_at) VALUES (?,?,?)`, rid, count, u.LastUsedAt)
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
	return res, nil
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
