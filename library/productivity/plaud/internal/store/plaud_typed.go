// Copyright 2026 jnalv414. Licensed under Apache-2.0. See LICENSE.

// Plaud-typed helpers — the typed tables (recordings_typed, transcripts,
// summaries, filetags_typed, speakers) are the substrate the transcendence
// commands (commitments, topic, about, forgotten, themes, cross-meeting,
// silence, mentioned-me) read from. The generic `resources` table is still
// populated by the framework sync; these helpers exist so domain code reads
// strongly-typed rows directly rather than re-parsing JSON on every query.

package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

// PlaudRecording mirrors the subset of /file/simple/web row fields we promote.
type PlaudRecording struct {
	ID            string `json:"id"`
	Filename      string `json:"filename"`
	StartTime     int64  `json:"start_time"`
	EndTime       int64  `json:"end_time"`
	Duration      int64  `json:"duration"`
	SerialNumber  string `json:"serial_number"`
	Scene         string `json:"scene"`
	IsTrash       int    `json:"is_trash"`
	IsTrans       int    `json:"is_trans"`
	IsSummary     int    `json:"is_summary"`
	EditTime      int64  `json:"edit_time"`
	Timezone      string `json:"timezone"`
	FileTagIDList []any  `json:"filetag_id_list"`
}

// PlaudTranscriptSegment is one row from /ai/transsumm.data_result[].
type PlaudTranscriptSegment struct {
	StartTime       float64 `json:"start_time"`
	EndTime         float64 `json:"end_time"`
	Content         string  `json:"content"`
	Speaker         string  `json:"speaker"`
	OriginalSpeaker string  `json:"original_speaker"`
}

// PlaudSummary is the normalized result from /ai/transsumm.data_result_summ.
// The wire shape is inconsistent (string, {markdown}, {content:str},
// {content:{markdown}}, {ai_content,header}) — callers normalize before
// passing here.
type PlaudSummary struct {
	Markdown    string   `json:"markdown"`
	Decisions   []string `json:"decisions"`
	ActionItems []string `json:"action_items"`
	Topics      []string `json:"topics"`
	Header      string   `json:"header"`
}

// PlaudFileTag mirrors a /filetag/ row.
type PlaudFileTag struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	ParentID string `json:"parent_id"`
}

// UpsertRecording writes a recording row to recordings_typed.
func (s *Store) UpsertRecording(rec PlaudRecording, rawJSON []byte) error {
	tagJSON := "[]"
	if len(rec.FileTagIDList) > 0 {
		if b, err := json.Marshal(rec.FileTagIDList); err == nil {
			tagJSON = string(b)
		}
	}
	_, err := s.db.Exec(`
		INSERT INTO recordings_typed (
			id, filename, start_time, end_time, duration, serial_number,
			scene, is_trash, is_trans, is_summary, edit_time, timezone,
			filetag_id_list, raw_json, synced_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, strftime('%s','now'))
		ON CONFLICT(id) DO UPDATE SET
			filename = excluded.filename,
			start_time = excluded.start_time,
			end_time = excluded.end_time,
			duration = excluded.duration,
			serial_number = excluded.serial_number,
			scene = excluded.scene,
			is_trash = excluded.is_trash,
			is_trans = excluded.is_trans,
			is_summary = excluded.is_summary,
			edit_time = excluded.edit_time,
			timezone = excluded.timezone,
			filetag_id_list = excluded.filetag_id_list,
			raw_json = excluded.raw_json,
			synced_at = strftime('%s','now')
	`, rec.ID, rec.Filename, rec.StartTime, rec.EndTime, rec.Duration,
		rec.SerialNumber, rec.Scene, rec.IsTrash, rec.IsTrans, rec.IsSummary,
		rec.EditTime, rec.Timezone, tagJSON, string(rawJSON))
	if err != nil {
		return fmt.Errorf("upsert recording %s: %w", rec.ID, err)
	}
	return nil
}

// UpsertTranscriptSegments writes (recording_id, idx) → segment for every
// segment in the slice, replacing any prior segments for that recording.
func (s *Store) UpsertTranscriptSegments(recordingID string, segments []PlaudTranscriptSegment) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transcripts tx: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM transcripts WHERE recording_id = ?`, recordingID); err != nil {
		return fmt.Errorf("clearing prior transcripts for %s: %w", recordingID, err)
	}
	for i, seg := range segments {
		if _, err := tx.Exec(`
			INSERT INTO transcripts (recording_id, idx, start_time, end_time, content, speaker, original_speaker)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`, recordingID, i, seg.StartTime, seg.EndTime, seg.Content, seg.Speaker, seg.OriginalSpeaker); err != nil {
			return fmt.Errorf("insert segment %d for %s: %w", i, recordingID, err)
		}
	}
	return tx.Commit()
}

// UpsertSummary writes a normalized summary for a recording.
func (s *Store) UpsertSummary(recordingID string, sum PlaudSummary, rawJSON []byte) error {
	decisionsJSON, _ := json.Marshal(sum.Decisions)
	actionItemsJSON, _ := json.Marshal(sum.ActionItems)
	topicsJSON, _ := json.Marshal(sum.Topics)
	_, err := s.db.Exec(`
		INSERT INTO summaries (recording_id, markdown, decisions, action_items, topics, header, raw_json)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(recording_id) DO UPDATE SET
			markdown = excluded.markdown,
			decisions = excluded.decisions,
			action_items = excluded.action_items,
			topics = excluded.topics,
			header = excluded.header,
			raw_json = excluded.raw_json
	`, recordingID, sum.Markdown, string(decisionsJSON), string(actionItemsJSON), string(topicsJSON), sum.Header, string(rawJSON))
	if err != nil {
		return fmt.Errorf("upsert summary %s: %w", recordingID, err)
	}
	return nil
}

// UpsertFileTag writes a file tag row.
func (s *Store) UpsertFileTag(tag PlaudFileTag, rawJSON []byte) error {
	_, err := s.db.Exec(`
		INSERT INTO filetags_typed (id, name, parent_id, raw_json)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			parent_id = excluded.parent_id,
			raw_json = excluded.raw_json
	`, tag.ID, tag.Name, tag.ParentID, string(rawJSON))
	if err != nil {
		return fmt.Errorf("upsert filetag %s: %w", tag.ID, err)
	}
	return nil
}

// RefreshSpeakerAgg recomputes the speakers table from the transcripts table.
// Idempotent; call after a sync run.
func (s *Store) RefreshSpeakerAgg() error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin speakers tx: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM speakers`); err != nil {
		return fmt.Errorf("clearing speakers: %w", err)
	}
	if _, err := tx.Exec(`
		INSERT INTO speakers (name, original_speaker, appearance_count, first_seen, last_seen)
		SELECT t.speaker AS name,
		       MAX(t.original_speaker) AS original_speaker,
		       COUNT(*) AS appearance_count,
		       MIN(r.start_time) AS first_seen,
		       MAX(r.start_time) AS last_seen
		FROM transcripts t
		JOIN recordings_typed r ON r.id = t.recording_id
		WHERE t.speaker IS NOT NULL AND t.speaker != ''
		GROUP BY t.speaker
	`); err != nil {
		return fmt.Errorf("rebuilding speakers: %w", err)
	}
	return tx.Commit()
}

// NormalizeSummaryShape collapses /ai/transsumm.data_result_summ's 4
// inconsistent shapes into PlaudSummary. raw is the JSON value of
// data_result_summ verbatim. Best-effort — returns an empty PlaudSummary
// when the shape is unrecognized rather than failing the sync.
func NormalizeSummaryShape(raw json.RawMessage) PlaudSummary {
	out := PlaudSummary{}
	if len(raw) == 0 {
		return out
	}
	// Try string form first (the most common shape for normal recordings).
	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		// The string may itself be JSON; try to parse it as an object too.
		asString = strings.TrimSpace(asString)
		if strings.HasPrefix(asString, "{") {
			var inner map[string]json.RawMessage
			if err := json.Unmarshal([]byte(asString), &inner); err == nil {
				return normalizeSummaryObject(inner)
			}
		}
		out.Markdown = asString
		return out
	}
	// Try object form.
	var asObj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &asObj); err == nil {
		return normalizeSummaryObject(asObj)
	}
	return out
}

func normalizeSummaryObject(obj map[string]json.RawMessage) PlaudSummary {
	out := PlaudSummary{}
	if v, ok := obj["markdown"]; ok {
		_ = json.Unmarshal(v, &out.Markdown)
	}
	if v, ok := obj["header"]; ok {
		_ = json.Unmarshal(v, &out.Header)
	}
	if v, ok := obj["ai_content"]; ok && out.Markdown == "" {
		_ = json.Unmarshal(v, &out.Markdown)
	}
	if v, ok := obj["content"]; ok && out.Markdown == "" {
		// content can be string or { markdown: "..." }
		var asString string
		if err := json.Unmarshal(v, &asString); err == nil {
			out.Markdown = asString
		} else {
			var nested map[string]json.RawMessage
			if err := json.Unmarshal(v, &nested); err == nil {
				if md, ok := nested["markdown"]; ok {
					_ = json.Unmarshal(md, &out.Markdown)
				}
			}
		}
	}
	if v, ok := obj["decisions"]; ok {
		_ = json.Unmarshal(v, &out.Decisions)
	}
	if v, ok := obj["action_items"]; ok {
		_ = json.Unmarshal(v, &out.ActionItems)
	}
	if v, ok := obj["topics"]; ok {
		_ = json.Unmarshal(v, &out.Topics)
	}
	return out
}

// TranscriptSearchResult is one row of an FTS5 transcript search.
type TranscriptSearchResult struct {
	RecordingID string  `json:"recording_id"`
	Filename    string  `json:"filename"`
	StartTime   int64   `json:"start_time"`
	Speaker     string  `json:"speaker"`
	Content     string  `json:"content"`
	Snippet     string  `json:"snippet"`
	SegmentIdx  int     `json:"segment_idx"`
	SegStart    float64 `json:"segment_start"`
}

// SearchTranscripts runs an FTS5 MATCH against transcript content, joined to
// recordings_typed for filename/start_time. Domain-typed search method —
// callers get a strongly-typed row shape instead of raw JSON.
func (s *Store) SearchTranscripts(query string, speakerFilter string, sinceEpoch int64, limit int) ([]TranscriptSearchResult, error) {
	if limit <= 0 {
		limit = 50
	}
	sqlQuery := `
		SELECT t.recording_id, r.filename, r.start_time, t.speaker, t.content,
		       snippet(transcripts_fts, 0, '<<', '>>', '…', 12) AS snippet,
		       t.idx, t.start_time AS seg_start
		FROM transcripts_fts
		JOIN transcripts t ON t.rowid = transcripts_fts.rowid
		JOIN recordings_typed r ON r.id = t.recording_id
		WHERE transcripts_fts MATCH ?
	`
	args := []any{query}
	if speakerFilter != "" {
		sqlQuery += " AND t.speaker LIKE ?"
		args = append(args, "%"+speakerFilter+"%")
	}
	if sinceEpoch > 0 {
		sqlQuery += " AND r.start_time >= ?"
		args = append(args, sinceEpoch)
	}
	sqlQuery += " ORDER BY r.start_time DESC LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.Query(sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("SearchTranscripts query: %w", err)
	}
	defer rows.Close()
	out := []TranscriptSearchResult{}
	for rows.Next() {
		var r TranscriptSearchResult
		var speaker, content, snippet, filename sql.NullString
		var segStart float64
		if err := rows.Scan(&r.RecordingID, &filename, &r.StartTime, &speaker, &content, &snippet, &r.SegmentIdx, &segStart); err != nil {
			return nil, fmt.Errorf("SearchTranscripts scan: %w", err)
		}
		r.Filename = filename.String
		r.Speaker = speaker.String
		r.Content = content.String
		r.Snippet = snippet.String
		r.SegStart = segStart
		out = append(out, r)
	}
	return out, rows.Err()
}

// SearchSpeakers returns speakers whose name matches a substring query.
// Typed-domain helper for the speakers command.
func (s *Store) SearchSpeakers(nameSubstring string, limit int) ([]map[string]any, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.Query(`
		SELECT name, original_speaker, appearance_count, first_seen, last_seen
		FROM speakers
		WHERE name LIKE ?
		ORDER BY last_seen DESC
		LIMIT ?
	`, "%"+nameSubstring+"%", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var name, orig sql.NullString
		var count, first, last int64
		if err := rows.Scan(&name, &orig, &count, &first, &last); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{
			"name":             name.String,
			"original_speaker": orig.String,
			"appearance_count": count,
			"first_seen":       first,
			"last_seen":        last,
		})
	}
	return out, rows.Err()
}

// HasTranscript returns true if any transcript segment exists for the recording.
func (s *Store) HasTranscript(recordingID string) (bool, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM transcripts WHERE recording_id = ?`, recordingID).Scan(&count)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// CachedUserName returns the authenticated user's display name from the
// generic resources table (where /user/me is cached as resource_type='users').
// Returns "" if no user row has been synced yet.
func (s *Store) CachedUserName() (string, error) {
	var data string
	err := s.db.QueryRow(`SELECT data FROM resources WHERE resource_type = 'users' LIMIT 1`).Scan(&data)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	var parsed map[string]json.RawMessage
	if err := json.Unmarshal([]byte(data), &parsed); err != nil {
		return "", nil
	}
	// Plaud wraps user in data_user.
	if v, ok := parsed["data_user"]; ok {
		var u map[string]json.RawMessage
		if json.Unmarshal(v, &u) == nil {
			for _, k := range []string{"nickname", "email", "name"} {
				if raw, ok := u[k]; ok {
					var s string
					if json.Unmarshal(raw, &s) == nil && s != "" {
						return s, nil
					}
				}
			}
		}
	}
	for _, k := range []string{"nickname", "name", "email"} {
		if raw, ok := parsed[k]; ok {
			var s string
			if json.Unmarshal(raw, &s) == nil && s != "" {
				return s, nil
			}
		}
	}
	return "", nil
}
