package localstore

import (
	"context"
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/zoom/internal/local/recordings"
)

// UpsertRecording inserts or updates a local recording folder.
// Returns the row id used to key transcript segments.
func UpsertRecording(ctx context.Context, db *sql.DB, f recordings.Folder) (string, error) {
	if err := EnsureSchema(ctx, db); err != nil {
		return "", err
	}
	id := folderID(f)
	transcript := nullableString(f.TranscriptPath)
	chat := nullableString(f.ChatPath)
	meetingID := nullableString(f.MeetingID)
	_, err := db.ExecContext(ctx, `
		INSERT INTO local_recordings(id, path, name, topic, start, total_bytes,
			has_video, has_audio, has_chat, has_transcript, has_partial,
			transcript_path, chat_path, meeting_id, synced_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET
			path=excluded.path,
			name=excluded.name,
			topic=excluded.topic,
			start=excluded.start,
			total_bytes=excluded.total_bytes,
			has_video=excluded.has_video,
			has_audio=excluded.has_audio,
			has_chat=excluded.has_chat,
			has_transcript=excluded.has_transcript,
			has_partial=excluded.has_partial,
			transcript_path=excluded.transcript_path,
			chat_path=excluded.chat_path,
			meeting_id=excluded.meeting_id,
			synced_at=CURRENT_TIMESTAMP
	`, id, f.Path, f.Name, f.Topic, f.Start, f.TotalBytes,
		boolToInt(f.HasVideo), boolToInt(f.HasAudio), boolToInt(f.HasChat),
		boolToInt(f.HasTranscript), boolToInt(f.HasPartial),
		transcript, chat, meetingID)
	return id, err
}

// UpsertSegment writes one parsed VTT cue.
func UpsertSegment(ctx context.Context, db *sql.DB, recordingID string, cue recordings.Cue) error {
	if err := EnsureSchema(ctx, db); err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Capture the prior text (if any) before the upsert. For an FTS5
	// external-content table, MATCH scans the FTS term index, not the source
	// table — so a re-sync that only updates the source row leaves the term
	// index stale, and corrected VTT text silently fails to match. The FTS5
	// 'delete' command needs the OLD column values, hence the read-first.
	var existingID int64
	var oldText string
	var hadRow bool
	switch scanErr := tx.QueryRowContext(ctx,
		`SELECT id, text FROM local_transcript_segments WHERE recording_id = ? AND cue_index = ?`,
		recordingID, cue.Index).Scan(&existingID, &oldText); scanErr {
	case nil:
		hadRow = true
	case sql.ErrNoRows:
		hadRow = false
	default:
		return scanErr
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO local_transcript_segments(recording_id, cue_index, start_ms, end_ms, speaker, text)
		VALUES(?,?,?,?,?,?)
		ON CONFLICT(recording_id, cue_index) DO UPDATE SET
			start_ms=excluded.start_ms,
			end_ms=excluded.end_ms,
			speaker=excluded.speaker,
			text=excluded.text
	`, recordingID, cue.Index, cue.Start.Milliseconds(), cue.End.Milliseconds(), nullableString(cue.Speaker), cue.Text); err != nil {
		return err
	}

	// Resolve the (stable) row id.
	id := existingID
	if !hadRow {
		if err := tx.QueryRowContext(ctx,
			`SELECT id FROM local_transcript_segments WHERE recording_id = ? AND cue_index = ?`,
			recordingID, cue.Index).Scan(&id); err != nil {
			return err
		}
	}

	// Refresh the external-content FTS index: delete the old term entry (using
	// the OLD text) when one existed, then insert the new text. This is the
	// canonical FTS5 external-content update pattern — without the delete, the
	// re-sync path keeps matching stale terms.
	if hadRow {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO local_transcript_segments_fts(local_transcript_segments_fts, rowid, text) VALUES('delete', ?, ?)`,
			id, oldText); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO local_transcript_segments_fts(rowid, text) VALUES(?, ?)`,
		id, cue.Text); err != nil {
		return err
	}

	return tx.Commit()
}

// ListLocalRecordings returns rows ordered by start desc, optionally filtered.
type ListLocalOpts struct {
	Since       time.Time // zero = no filter
	PartialOnly bool
	Limit       int // 0 = unlimited
}

// LocalRecordingRow is the row shape returned to commands.
type LocalRecordingRow struct {
	ID             string    `json:"id"`
	Path           string    `json:"path"`
	Name           string    `json:"name"`
	Topic          string    `json:"topic"`
	Start          time.Time `json:"start"`
	TotalBytes     int64     `json:"total_bytes"`
	HasVideo       bool      `json:"has_video"`
	HasAudio       bool      `json:"has_audio"`
	HasChat        bool      `json:"has_chat"`
	HasTranscript  bool      `json:"has_transcript"`
	HasPartial     bool      `json:"has_partial"`
	TranscriptPath string    `json:"transcript_path,omitempty"`
	ChatPath       string    `json:"chat_path,omitempty"`
	MeetingID      string    `json:"meeting_id,omitempty"`
	SyncedAt       time.Time `json:"synced_at"`
}

// ListLocalRecordings returns ordered rows.
func ListLocalRecordings(ctx context.Context, db *sql.DB, opts ListLocalOpts) ([]LocalRecordingRow, error) {
	if err := EnsureSchema(ctx, db); err != nil {
		return nil, err
	}
	q := `SELECT id, path, name, COALESCE(topic, ''), start, total_bytes,
		has_video, has_audio, has_chat, has_transcript, has_partial,
		COALESCE(transcript_path, ''), COALESCE(chat_path, ''), COALESCE(meeting_id, ''), synced_at
		FROM local_recordings WHERE 1=1`
	args := []any{}
	if !opts.Since.IsZero() {
		q += " AND start >= ?"
		args = append(args, opts.Since)
	}
	if opts.PartialOnly {
		q += " AND has_partial = 1"
	}
	q += " ORDER BY start DESC"
	if opts.Limit > 0 {
		q += " LIMIT ?"
		args = append(args, opts.Limit)
	}
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LocalRecordingRow
	for rows.Next() {
		var r LocalRecordingRow
		var hv, ha, hc, ht, hp int
		var startNT sql.NullTime
		if err := rows.Scan(&r.ID, &r.Path, &r.Name, &r.Topic, &startNT, &r.TotalBytes,
			&hv, &ha, &hc, &ht, &hp,
			&r.TranscriptPath, &r.ChatPath, &r.MeetingID, &r.SyncedAt); err != nil {
			continue
		}
		if startNT.Valid {
			r.Start = startNT.Time
		}
		r.HasVideo = hv == 1
		r.HasAudio = ha == 1
		r.HasChat = hc == 1
		r.HasTranscript = ht == 1
		r.HasPartial = hp == 1
		out = append(out, r)
	}
	return out, rows.Err()
}

// SearchSegments performs an FTS5 query and returns matched cues with the
// owning recording's topic + path.
type SegmentMatch struct {
	RecordingID    string        `json:"recording_id"`
	RecordingPath  string        `json:"recording_path"`
	RecordingTopic string        `json:"recording_topic"`
	StartMs        int64         `json:"start_ms"`
	StartHHMMSS    string        `json:"start_hhmmss"`
	Speaker        string        `json:"speaker,omitempty"`
	Text           string        `json:"text"`
	DeepLink       string        `json:"deep_link,omitempty"`
	Source         string        `json:"source"` // "local" or "cloud"
	RecordingStart time.Time     `json:"recording_start,omitempty"`
	Duration       time.Duration `json:"-"`
}

func SearchLocalSegments(ctx context.Context, db *sql.DB, query string, speaker string, since time.Time, limit int) ([]SegmentMatch, error) {
	if err := EnsureSchema(ctx, db); err != nil {
		return nil, err
	}
	if query == "" {
		return nil, nil
	}
	q := `SELECT lr.id, lr.path, COALESCE(lr.topic, ''), s.start_ms, COALESCE(s.speaker, ''), s.text, lr.start
		FROM local_transcript_segments s
		JOIN local_transcript_segments_fts fts ON fts.rowid = s.id
		JOIN local_recordings lr ON lr.id = s.recording_id
		WHERE local_transcript_segments_fts MATCH ?`
	args := []any{ftsQuote(query)}
	if speaker != "" {
		q += " AND s.speaker = ?"
		args = append(args, speaker)
	}
	if !since.IsZero() {
		q += " AND lr.start >= ?"
		args = append(args, since)
	}
	q += " ORDER BY lr.start DESC, s.start_ms ASC"
	if limit > 0 {
		q += " LIMIT ?"
		args = append(args, limit)
	}
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SegmentMatch
	for rows.Next() {
		var m SegmentMatch
		var startNT sql.NullTime
		if err := rows.Scan(&m.RecordingID, &m.RecordingPath, &m.RecordingTopic, &m.StartMs, &m.Speaker, &m.Text, &startNT); err != nil {
			continue
		}
		if startNT.Valid {
			m.RecordingStart = startNT.Time
		}
		m.StartHHMMSS = formatHHMMSS(m.StartMs)
		m.Source = "local"
		out = append(out, m)
	}
	return out, rows.Err()
}

func formatHHMMSS(ms int64) string {
	d := time.Duration(ms) * time.Millisecond
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	if h > 0 {
		return formatInt(h, 2) + ":" + formatInt(m, 2) + ":" + formatInt(s, 2)
	}
	return formatInt(m, 2) + ":" + formatInt(s, 2)
}

func formatInt(n, width int) string {
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	for len(s) < width {
		s = "0" + s
	}
	if s == "" {
		s = "00"
	}
	return s
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func folderID(f recordings.Folder) string {
	h := sha1.Sum([]byte(f.Path))
	return hex.EncodeToString(h[:8])
}
