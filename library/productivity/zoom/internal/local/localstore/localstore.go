// Package localstore owns the SQLite schema for Zoom-CLI-specific local state:
//
//   - saved_meetings — user-named bookmarks with FTS (T5 add-from-url lives here)
//   - local_recordings — one row per ~/Documents/Zoom/<folder>
//   - local_transcript_segments — one row per parsed VTT cue, with FTS over text
//   - notes — one row per ingested Zoom Notes PDF/DOCX export
//   - note_segments — paragraphs from an ingested note, with FTS over text
//   - note_todos — regex-extracted action items
//
// The package is hand-authored and sits beside the generated internal/store
// package; it never touches the generated `resources` table. Every schema
// statement is idempotent (CREATE ... IF NOT EXISTS), so commands call
// EnsureSchema(db) before any read/write with no memoization needed.
package localstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// EnsureSchema creates all hand-authored tables + FTS5 virtual tables
// idempotently. Safe to call from every command's RunE. Every statement is
// CREATE ... IF NOT EXISTS, so re-running is cheap; we deliberately do NOT
// memoize with a package-level sync.Once because that would capture the first
// *sql.DB and silently skip DDL for any later call with a different handle
// (e.g. a fresh temp DB in tests).
func EnsureSchema(ctx context.Context, db *sql.DB) error {
	{
		stmts := []string{
			// Saved meeting bookmarks (T5 + absorb #2).
			`CREATE TABLE IF NOT EXISTS saved_meetings (
				name TEXT PRIMARY KEY,
				meeting_id TEXT NOT NULL,
				pwd TEXT,
				url TEXT,
				notes TEXT,
				scheduled_for TIMESTAMP,
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
			)`,
			// Note: saved bookmarks are queried with simple SELECT/LIKE in
			// ListBookmarks/GetBookmark — bookmark counts are tiny (tens, not
			// thousands), so a full-text index is unnecessary. We deliberately do
			// NOT declare a saved_meetings_fts external-content table here: an
			// unpopulated external-content FTS index is dead weight that drifts
			// from its backing table. Add one (with populate-on-write) only if a
			// dedicated `saved search` command is ever introduced.
			// Local recordings (one row per ~/Documents/Zoom/<folder>).
			`CREATE TABLE IF NOT EXISTS local_recordings (
				id TEXT PRIMARY KEY,
				path TEXT NOT NULL UNIQUE,
				name TEXT NOT NULL,
				topic TEXT,
				start TIMESTAMP,
				total_bytes INTEGER DEFAULT 0,
				has_video INTEGER DEFAULT 0,
				has_audio INTEGER DEFAULT 0,
				has_chat INTEGER DEFAULT 0,
				has_transcript INTEGER DEFAULT 0,
				has_partial INTEGER DEFAULT 0,
				transcript_path TEXT,
				chat_path TEXT,
				meeting_id TEXT,
				synced_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
			)`,
			`CREATE INDEX IF NOT EXISTS idx_local_recordings_start ON local_recordings(start)`,
			`CREATE INDEX IF NOT EXISTS idx_local_recordings_meeting_id ON local_recordings(meeting_id)`,
			// Local transcript segments — one row per VTT cue.
			`CREATE TABLE IF NOT EXISTS local_transcript_segments (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				recording_id TEXT NOT NULL,
				cue_index INTEGER NOT NULL,
				start_ms INTEGER NOT NULL,
				end_ms INTEGER NOT NULL,
				speaker TEXT,
				text TEXT NOT NULL,
				UNIQUE(recording_id, cue_index)
			)`,
			`CREATE INDEX IF NOT EXISTS idx_local_segments_recording ON local_transcript_segments(recording_id)`,
			`CREATE VIRTUAL TABLE IF NOT EXISTS local_transcript_segments_fts USING fts5(text, content='local_transcript_segments', content_rowid='id')`,
			// Cloud recordings cache for cross-source joins (T2, T3).
			`CREATE TABLE IF NOT EXISTS cloud_recordings (
				meeting_uuid TEXT PRIMARY KEY,
				meeting_id TEXT,
				topic TEXT,
				start_time TIMESTAMP,
				duration_minutes INTEGER,
				total_bytes INTEGER DEFAULT 0,
				file_count INTEGER DEFAULT 0,
				retention_at TIMESTAMP,
				synced_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
			)`,
			`CREATE INDEX IF NOT EXISTS idx_cloud_recordings_meeting_id ON cloud_recordings(meeting_id)`,
			// Ingested Notes (T9).
			`CREATE TABLE IF NOT EXISTS notes (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				source_file TEXT NOT NULL UNIQUE,
				file_format TEXT NOT NULL,
				meeting_topic TEXT,
				meeting_id TEXT,
				start_time TIMESTAMP,
				word_count INTEGER DEFAULT 0,
				segment_count INTEGER DEFAULT 0,
				todo_count INTEGER DEFAULT 0,
				ingested_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
			)`,
			`CREATE INDEX IF NOT EXISTS idx_notes_meeting_id ON notes(meeting_id)`,
			`CREATE INDEX IF NOT EXISTS idx_notes_start_time ON notes(start_time)`,
			// Note paragraphs/segments with FTS.
			`CREATE TABLE IF NOT EXISTS note_segments (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				note_id INTEGER NOT NULL,
				ord INTEGER NOT NULL,
				heading TEXT,
				text TEXT NOT NULL,
				FOREIGN KEY(note_id) REFERENCES notes(id) ON DELETE CASCADE
			)`,
			`CREATE INDEX IF NOT EXISTS idx_note_segments_note ON note_segments(note_id)`,
			`CREATE VIRTUAL TABLE IF NOT EXISTS note_segments_fts USING fts5(text, heading, content='note_segments', content_rowid='id')`,
			// Note TODOs / action items.
			`CREATE TABLE IF NOT EXISTS note_todos (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				note_id INTEGER NOT NULL,
				ord INTEGER NOT NULL,
				pattern TEXT NOT NULL,
				text TEXT NOT NULL,
				owner TEXT,
				checked INTEGER DEFAULT 0,
				FOREIGN KEY(note_id) REFERENCES notes(id) ON DELETE CASCADE
			)`,
			`CREATE INDEX IF NOT EXISTS idx_note_todos_note ON note_todos(note_id)`,
		}
		ctx2, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		for _, s := range stmts {
			if _, err := db.ExecContext(ctx2, s); err != nil {
				return fmt.Errorf("localstore: %s: %w", firstLine(s), err)
			}
		}
	}
	return nil
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}

// SavedMeeting is one bookmark row.
type SavedMeeting struct {
	Name         string     `json:"name"`
	MeetingID    string     `json:"meeting_id"`
	Pwd          string     `json:"pwd,omitempty"`
	URL          string     `json:"url,omitempty"`
	Notes        string     `json:"notes,omitempty"`
	ScheduledFor *time.Time `json:"scheduled_for,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// SaveBookmark inserts or updates a saved bookmark.
func SaveBookmark(ctx context.Context, db *sql.DB, sm SavedMeeting) error {
	if err := EnsureSchema(ctx, db); err != nil {
		return err
	}
	if sm.Name == "" {
		return errors.New("localstore: saved meeting name is required")
	}
	if sm.MeetingID == "" {
		return errors.New("localstore: meeting_id is required")
	}
	_, err := db.ExecContext(ctx, `
		INSERT INTO saved_meetings(name, meeting_id, pwd, url, notes, scheduled_for, created_at, updated_at)
		VALUES(?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT(name) DO UPDATE SET
			meeting_id=excluded.meeting_id,
			pwd=excluded.pwd,
			url=excluded.url,
			notes=excluded.notes,
			scheduled_for=excluded.scheduled_for,
			updated_at=CURRENT_TIMESTAMP
	`, sm.Name, sm.MeetingID, nullableString(sm.Pwd), nullableString(sm.URL), nullableString(sm.Notes), sm.ScheduledFor)
	return err
}

// ListBookmarks returns all saved bookmarks ordered by name.
func ListBookmarks(ctx context.Context, db *sql.DB) ([]SavedMeeting, error) {
	if err := EnsureSchema(ctx, db); err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx, `
		SELECT name, meeting_id, COALESCE(pwd, ''), COALESCE(url, ''), COALESCE(notes, ''), scheduled_for, created_at, updated_at
		FROM saved_meetings ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SavedMeeting
	for rows.Next() {
		var sm SavedMeeting
		var sched sql.NullTime
		if err := rows.Scan(&sm.Name, &sm.MeetingID, &sm.Pwd, &sm.URL, &sm.Notes, &sched, &sm.CreatedAt, &sm.UpdatedAt); err != nil {
			continue
		}
		if sched.Valid {
			t := sched.Time
			sm.ScheduledFor = &t
		}
		out = append(out, sm)
	}
	return out, rows.Err()
}

// GetBookmark looks up a saved bookmark by name (case-insensitive).
func GetBookmark(ctx context.Context, db *sql.DB, name string) (*SavedMeeting, error) {
	if err := EnsureSchema(ctx, db); err != nil {
		return nil, err
	}
	row := db.QueryRowContext(ctx, `
		SELECT name, meeting_id, COALESCE(pwd, ''), COALESCE(url, ''), COALESCE(notes, ''), scheduled_for, created_at, updated_at
		FROM saved_meetings WHERE LOWER(name) = LOWER(?)`, name)
	var sm SavedMeeting
	var sched sql.NullTime
	if err := row.Scan(&sm.Name, &sm.MeetingID, &sm.Pwd, &sm.URL, &sm.Notes, &sched, &sm.CreatedAt, &sm.UpdatedAt); err != nil {
		return nil, err
	}
	if sched.Valid {
		t := sched.Time
		sm.ScheduledFor = &t
	}
	return &sm, nil
}

// DeleteBookmark removes a saved bookmark.
func DeleteBookmark(ctx context.Context, db *sql.DB, name string) (bool, error) {
	if err := EnsureSchema(ctx, db); err != nil {
		return false, err
	}
	r, err := db.ExecContext(ctx, `DELETE FROM saved_meetings WHERE LOWER(name) = LOWER(?)`, name)
	if err != nil {
		return false, err
	}
	n, _ := r.RowsAffected()
	return n > 0, nil
}

func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}
