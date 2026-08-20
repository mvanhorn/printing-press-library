package localstore

import (
	"context"
	"database/sql"
	"time"
)

// IngestedNote is one parsed Notes PDF/DOCX from the Zoom Notes export.
type IngestedNote struct {
	SourceFile   string    `json:"source_file"`
	FileFormat   string    `json:"file_format"`
	MeetingTopic string    `json:"meeting_topic,omitempty"`
	MeetingID    string    `json:"meeting_id,omitempty"`
	StartTime    time.Time `json:"start_time,omitempty"`
	Segments     []NoteSegment
	Todos        []NoteTodo
}

// NoteSegment is one paragraph/section.
type NoteSegment struct {
	Ord     int    `json:"ord"`
	Heading string `json:"heading,omitempty"`
	Text    string `json:"text"`
}

// NoteTodo is one extracted action item.
type NoteTodo struct {
	Ord     int    `json:"ord"`
	Pattern string `json:"pattern"`
	Text    string `json:"text"`
	Owner   string `json:"owner,omitempty"`
	Checked bool   `json:"checked"`
}

// IngestNote upserts a note plus all its segments and todos in one transaction.
func IngestNote(ctx context.Context, db *sql.DB, note IngestedNote) (int64, error) {
	if err := EnsureSchema(ctx, db); err != nil {
		return 0, err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	// Delete any existing note + cascading children. note_segments_fts is an
	// FTS5 external-content table, so SQLite does NOT propagate the cascade
	// delete into the FTS term index — we must issue explicit 'delete' commands
	// (with the OLD column values, in declared order text, heading) for every
	// existing segment first, or zombie entries accumulate at stale rowids on
	// each re-ingest (unbounded index growth, corrupted BM25 stats, failing
	// integrity-check).
	ftsRows, err := tx.QueryContext(ctx, `
		SELECT ns.id, ns.text, COALESCE(ns.heading, '')
		FROM note_segments ns
		JOIN notes n ON n.id = ns.note_id
		WHERE n.source_file = ?`, note.SourceFile)
	if err != nil {
		return 0, err
	}
	type ftsEntry struct {
		id      int64
		text    string
		heading string
	}
	var stale []ftsEntry
	for ftsRows.Next() {
		var e ftsEntry
		if scanErr := ftsRows.Scan(&e.id, &e.text, &e.heading); scanErr != nil {
			ftsRows.Close()
			return 0, scanErr
		}
		stale = append(stale, e)
	}
	ftsRows.Close()
	if err := ftsRows.Err(); err != nil {
		return 0, err
	}
	for _, e := range stale {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO note_segments_fts(note_segments_fts, rowid, text, heading) VALUES('delete', ?, ?, ?)`,
			e.id, e.text, e.heading); err != nil {
			return 0, err
		}
	}

	// Now delete the note row (cascades to note_segments + note_todos).
	if _, err := tx.ExecContext(ctx, `DELETE FROM notes WHERE source_file = ?`, note.SourceFile); err != nil {
		return 0, err
	}

	wc := 0
	for _, s := range note.Segments {
		wc += wordCount(s.Text)
	}

	res, err := tx.ExecContext(ctx, `
		INSERT INTO notes(source_file, file_format, meeting_topic, meeting_id, start_time, word_count, segment_count, todo_count)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?)`,
		note.SourceFile, note.FileFormat, nullableString(note.MeetingTopic), nullableString(note.MeetingID),
		nullableTime(note.StartTime), wc, len(note.Segments), len(note.Todos))
	if err != nil {
		return 0, err
	}
	noteID, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}

	for _, s := range note.Segments {
		r, err := tx.ExecContext(ctx, `INSERT INTO note_segments(note_id, ord, heading, text) VALUES(?,?,?,?)`,
			noteID, s.Ord, nullableString(s.Heading), s.Text)
		if err != nil {
			return 0, err
		}
		sid, _ := r.LastInsertId()
		if _, err := tx.ExecContext(ctx, `INSERT INTO note_segments_fts(rowid, text, heading) VALUES(?, ?, ?)`,
			sid, s.Text, s.Heading); err != nil {
			return 0, err
		}
	}
	for _, t := range note.Todos {
		if _, err := tx.ExecContext(ctx, `INSERT INTO note_todos(note_id, ord, pattern, text, owner, checked) VALUES(?,?,?,?,?,?)`,
			noteID, t.Ord, t.Pattern, t.Text, nullableString(t.Owner), boolToInt(t.Checked)); err != nil {
			return 0, err
		}
	}

	return noteID, tx.Commit()
}

// NoteRow is the row shape for notes list responses.
type NoteRow struct {
	ID           int64     `json:"id"`
	SourceFile   string    `json:"source_file"`
	FileFormat   string    `json:"file_format"`
	MeetingTopic string    `json:"meeting_topic,omitempty"`
	MeetingID    string    `json:"meeting_id,omitempty"`
	StartTime    time.Time `json:"start_time,omitempty"`
	WordCount    int       `json:"word_count"`
	SegmentCount int       `json:"segment_count"`
	TodoCount    int       `json:"todo_count"`
	IngestedAt   time.Time `json:"ingested_at"`
}

// ListNotes returns ingested notes sorted by start_time desc.
func ListNotes(ctx context.Context, db *sql.DB, since time.Time, limit int) ([]NoteRow, error) {
	if err := EnsureSchema(ctx, db); err != nil {
		return nil, err
	}
	q := `SELECT id, source_file, file_format, COALESCE(meeting_topic, ''), COALESCE(meeting_id, ''),
		start_time, word_count, segment_count, todo_count, ingested_at
		FROM notes WHERE 1=1`
	args := []any{}
	if !since.IsZero() {
		q += " AND (start_time >= ? OR ingested_at >= ?)"
		args = append(args, since, since)
	}
	q += " ORDER BY COALESCE(start_time, ingested_at) DESC"
	if limit > 0 {
		q += " LIMIT ?"
		args = append(args, limit)
	}
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []NoteRow
	for rows.Next() {
		var r NoteRow
		var startNT sql.NullTime
		if err := rows.Scan(&r.ID, &r.SourceFile, &r.FileFormat, &r.MeetingTopic, &r.MeetingID,
			&startNT, &r.WordCount, &r.SegmentCount, &r.TodoCount, &r.IngestedAt); err != nil {
			continue
		}
		if startNT.Valid {
			r.StartTime = startNT.Time
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// NoteSegmentMatch is the row shape for notes search.
type NoteSegmentMatch struct {
	NoteID       int64      `json:"note_id"`
	MeetingTopic string     `json:"meeting_topic,omitempty"`
	MeetingID    string     `json:"meeting_id,omitempty"`
	SourceFile   string     `json:"source_file"`
	Heading      string     `json:"heading,omitempty"`
	NoteExcerpt  string     `json:"note_excerpt"`
	StartTime    *time.Time `json:"start_time,omitempty"`
}

// SearchNotes runs FTS5 over ingested note segments.
func SearchNotes(ctx context.Context, db *sql.DB, query string, since time.Time, meetingID string, limit int) ([]NoteSegmentMatch, error) {
	if err := EnsureSchema(ctx, db); err != nil {
		return nil, err
	}
	if query == "" {
		return nil, nil
	}
	q := `SELECT n.id, COALESCE(n.meeting_topic, ''), COALESCE(n.meeting_id, ''), n.source_file,
		COALESCE(ns.heading, ''), ns.text, n.start_time
		FROM note_segments ns
		JOIN note_segments_fts fts ON fts.rowid = ns.id
		JOIN notes n ON n.id = ns.note_id
		WHERE note_segments_fts MATCH ?`
	args := []any{ftsQuote(query)}
	if !since.IsZero() {
		q += " AND COALESCE(n.start_time, n.ingested_at) >= ?"
		args = append(args, since)
	}
	if meetingID != "" {
		q += " AND n.meeting_id = ?"
		args = append(args, meetingID)
	}
	q += " ORDER BY COALESCE(n.start_time, n.ingested_at) DESC"
	if limit > 0 {
		q += " LIMIT ?"
		args = append(args, limit)
	}
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []NoteSegmentMatch
	for rows.Next() {
		var m NoteSegmentMatch
		var startNT sql.NullTime
		if err := rows.Scan(&m.NoteID, &m.MeetingTopic, &m.MeetingID, &m.SourceFile,
			&m.Heading, &m.NoteExcerpt, &startNT); err != nil {
			continue
		}
		if startNT.Valid {
			t := startNT.Time
			m.StartTime = &t
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// NoteTodoRow is the row shape for the todos extractor.
type NoteTodoRow struct {
	NoteID       int64      `json:"note_id"`
	MeetingTopic string     `json:"meeting_topic,omitempty"`
	MeetingID    string     `json:"meeting_id,omitempty"`
	StartTime    *time.Time `json:"start_time,omitempty"`
	Pattern      string     `json:"pattern"`
	Text         string     `json:"text"`
	Owner        string     `json:"owner,omitempty"`
	Checked      bool       `json:"checked"`
	SourceFile   string     `json:"source_file"`
}

// ListTodos returns extracted action items.
func ListTodos(ctx context.Context, db *sql.DB, since time.Time, meetingID string, limit int) ([]NoteTodoRow, error) {
	if err := EnsureSchema(ctx, db); err != nil {
		return nil, err
	}
	q := `SELECT n.id, COALESCE(n.meeting_topic, ''), COALESCE(n.meeting_id, ''), n.start_time,
		t.pattern, t.text, COALESCE(t.owner, ''), t.checked, n.source_file
		FROM note_todos t
		JOIN notes n ON n.id = t.note_id
		WHERE 1=1`
	args := []any{}
	if !since.IsZero() {
		q += " AND COALESCE(n.start_time, n.ingested_at) >= ?"
		args = append(args, since)
	}
	if meetingID != "" {
		q += " AND n.meeting_id = ?"
		args = append(args, meetingID)
	}
	q += " ORDER BY COALESCE(n.start_time, n.ingested_at) DESC, t.ord ASC"
	if limit > 0 {
		q += " LIMIT ?"
		args = append(args, limit)
	}
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []NoteTodoRow
	for rows.Next() {
		var r NoteTodoRow
		var startNT sql.NullTime
		var checked int
		if err := rows.Scan(&r.NoteID, &r.MeetingTopic, &r.MeetingID, &startNT,
			&r.Pattern, &r.Text, &r.Owner, &checked, &r.SourceFile); err != nil {
			continue
		}
		if startNT.Valid {
			t := startNT.Time
			r.StartTime = &t
		}
		r.Checked = checked == 1
		out = append(out, r)
	}
	return out, rows.Err()
}

func nullableTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}

func wordCount(s string) int {
	n := 0
	inWord := false
	for _, r := range s {
		switch r {
		case ' ', '\t', '\n', '\r', '\f', '\v':
			inWord = false
		default:
			if !inWord {
				n++
				inWord = true
			}
		}
	}
	return n
}
