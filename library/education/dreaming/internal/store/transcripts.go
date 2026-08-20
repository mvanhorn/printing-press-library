// Copyright 2026 Paul Bockewitz and contributors. Licensed under Apache-2.0. See LICENSE.

// Transcript-cue persistence for dreaming-pp-cli. Not generated — hand-written
// to back the `transcript` and `concordance` commands. Stores per-cue caption
// text with timing in an FTS5 table so the corpus is keyword-searchable
// offline and joinable to the videos catalog by video_id.

package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

// normalizeDreamingVideo maps the live Dreaming /videos field names onto the
// generic typed-column names the store expects, so SQL-backed commands (next,
// diet, stats by-guide) populate from real API data:
//
//	difficultyScore (ELO-style int) -> difficulty
//	guides ([]string)               -> guide (first)
//	tags ([]string)                 -> topic (comma-joined)
//	seriesId                        -> series
//
// The real catalog has no dialect field, so dialect stays empty. Existing
// snake/camel matches are left untouched (only fills absent targets).
func normalizeDreamingVideo(obj map[string]any) {
	setIfAbsent := func(key string, val any) {
		if val == nil {
			return
		}
		if _, ok := obj[key]; !ok {
			obj[key] = val
		}
	}
	if v, ok := obj["difficultyScore"]; ok {
		setIfAbsent("difficulty", v)
	}
	if g, ok := obj["guides"].([]any); ok && len(g) > 0 {
		if s, ok := g[0].(string); ok {
			setIfAbsent("guide", s)
		}
	}
	if t, ok := obj["tags"].([]any); ok && len(t) > 0 {
		parts := make([]string, 0, len(t))
		for _, x := range t {
			if s, ok := x.(string); ok {
				parts = append(parts, s)
			}
		}
		if len(parts) > 0 {
			setIfAbsent("topic", strings.Join(parts, ", "))
		}
	}
	if v, ok := obj["seriesId"]; ok {
		setIfAbsent("series", v)
	}
}

// TranscriptCue is one stored caption cue.
type TranscriptCue struct {
	VideoID string
	Index   int
	StartMS int64
	EndMS   int64
	Text    string
}

// ConcordanceHit is a single corpus match returned by transcript search.
type ConcordanceHit struct {
	VideoID    string `json:"video_id"`
	VideoTitle string `json:"video_title,omitempty"`
	Level      string `json:"level,omitempty"`
	Guide      string `json:"guide,omitempty"`
	Dialect    string `json:"dialect,omitempty"`
	StartMS    int64  `json:"start_ms"`
	Timestamp  string `json:"timestamp"`
	Text       string `json:"context"`
	VideoURL   string `json:"video_url,omitempty"`
}

// ensureTranscriptSchema creates the transcript FTS table on first use.
func (s *Store) ensureTranscriptSchema() error {
	_, err := s.db.Exec(`CREATE VIRTUAL TABLE IF NOT EXISTS transcript_cues USING fts5(
		video_id UNINDEXED,
		cue_index UNINDEXED,
		start_ms UNINDEXED,
		end_ms UNINDEXED,
		text,
		tokenize='unicode61'
	)`)
	return err
}

// EnsureTranscriptSchema is the exported initializer used by command setup.
func (s *Store) EnsureTranscriptSchema() error { return s.ensureTranscriptSchema() }

// ReplaceTranscript stores all cues for a video, replacing any prior cues for
// that video id so a re-sync does not duplicate rows.
func (s *Store) ReplaceTranscript(videoID string, cues []TranscriptCue) error {
	if err := s.ensureTranscriptSchema(); err != nil {
		return err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM transcript_cues WHERE video_id = ?`, videoID); err != nil {
		return err
	}
	stmt, err := tx.Prepare(`INSERT INTO transcript_cues (video_id, cue_index, start_ms, end_ms, text) VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, c := range cues {
		if _, err := stmt.Exec(videoID, c.Index, c.StartMS, c.EndMS, c.Text); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// HasTranscript reports whether any cues are cached for a video.
func (s *Store) HasTranscript(videoID string) bool {
	if err := s.ensureTranscriptSchema(); err != nil {
		return false
	}
	var one int
	err := s.db.QueryRow(`SELECT 1 FROM transcript_cues WHERE video_id = ? LIMIT 1`, videoID).Scan(&one)
	return err == nil
}

// TranscriptText reconstructs the flat transcript text for a video, in order.
func (s *Store) TranscriptText(videoID string) (string, bool, error) {
	if err := s.ensureTranscriptSchema(); err != nil {
		return "", false, err
	}
	rows, err := s.db.Query(`SELECT text FROM transcript_cues WHERE video_id = ? ORDER BY cue_index`, videoID)
	if err != nil {
		return "", false, err
	}
	defer rows.Close()
	var parts []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return "", false, err
		}
		parts = append(parts, t)
	}
	if len(parts) == 0 {
		return "", false, rows.Err()
	}
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += " "
		}
		out += p
	}
	return out, true, rows.Err()
}

// TranscriptStats returns the number of distinct videos and total cues cached.
func (s *Store) TranscriptStats() (videos int, cues int, err error) {
	if err = s.ensureTranscriptSchema(); err != nil {
		return 0, 0, err
	}
	if err = s.db.QueryRow(`SELECT COUNT(DISTINCT video_id), COUNT(*) FROM transcript_cues`).Scan(&videos, &cues); err != nil {
		return 0, 0, err
	}
	return videos, cues, nil
}

// TranscriptFilter scopes a corpus search to videos with matching catalog metadata.
type TranscriptFilter struct {
	Level   string
	Guide   string
	Dialect string
	Limit   int
}

// scanCueRows is shared by the FTS and full-scan paths.
func (s *Store) scanCueRows(rows *sql.Rows) ([]ConcordanceHit, error) {
	defer rows.Close()
	var hits []ConcordanceHit
	for rows.Next() {
		var h ConcordanceHit
		var title, level, guide, dialect sql.NullString
		if err := rows.Scan(&h.VideoID, &h.StartMS, &h.Text, &title, &level, &guide, &dialect); err != nil {
			return nil, err
		}
		h.VideoTitle = title.String
		h.Level = level.String
		h.Guide = guide.String
		h.Dialect = dialect.String
		hits = append(hits, h)
	}
	return hits, rows.Err()
}

const cueSelectJoin = `SELECT t.video_id, t.start_ms, t.text, v.title, v.level, v.guide, v.dialect
	FROM transcript_cues t LEFT JOIN videos v ON v.id = t.video_id`

func (s *Store) filterClause(f TranscriptFilter) (string, []any) {
	var clauses []string
	var args []any
	if f.Level != "" {
		clauses = append(clauses, "v.level = ?")
		args = append(args, f.Level)
	}
	if f.Guide != "" {
		clauses = append(clauses, "v.guide = ?")
		args = append(args, f.Guide)
	}
	if f.Dialect != "" {
		clauses = append(clauses, "v.dialect = ?")
		args = append(args, f.Dialect)
	}
	if len(clauses) == 0 {
		return "", args
	}
	out := " AND "
	for i, c := range clauses {
		if i > 0 {
			out += " AND "
		}
		out += c
	}
	return out, args
}

// SearchTranscriptFTS runs an FTS5 keyword/phrase query over the corpus.
func (s *Store) SearchTranscriptFTS(query string, f TranscriptFilter) ([]ConcordanceHit, error) {
	if err := s.ensureTranscriptSchema(); err != nil {
		return nil, err
	}
	limit := f.Limit
	if limit <= 0 {
		limit = 50
	}
	where := " WHERE transcript_cues MATCH ?"
	args := []any{query}
	fc, fargs := s.filterClause(f)
	where += fc
	args = append(args, fargs...)
	q := cueSelectJoin + where + " ORDER BY v.level, t.video_id, t.start_ms LIMIT ?"
	args = append(args, limit)
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("transcript search: %w", err)
	}
	return s.scanCueRows(rows)
}

// AllCuesForScan returns every cue (optionally metadata-filtered) for regex /
// verb-tense scanning in Go. Caller applies the pattern and caps results.
func (s *Store) AllCuesForScan(f TranscriptFilter) ([]ConcordanceHit, error) {
	if err := s.ensureTranscriptSchema(); err != nil {
		return nil, err
	}
	where := ""
	var args []any
	fc, fargs := s.filterClause(f)
	if fc != "" {
		where = " WHERE 1=1" + fc
		args = append(args, fargs...)
	}
	q := cueSelectJoin + where + " ORDER BY v.level, t.video_id, t.start_ms"
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("transcript scan: %w", err)
	}
	return s.scanCueRows(rows)
}

// --- Corpus export / import ------------------------------------------------
// These back the `transcript export` / `transcript import` commands, which let
// a complete transcript corpus be shared as a portable file (no personal data).
// Only the impersonal `videos` catalog rows and `transcript_cues` are touched;
// the user/playlist/external_time tables are never read or written here.

// AllVideoData returns the raw `data` JSON for every cached video, for export.
func (s *Store) AllVideoData() ([]json.RawMessage, error) {
	rows, err := s.db.Query(`SELECT data FROM videos`)
	if err != nil {
		return nil, fmt.Errorf("reading videos for export: %w", err)
	}
	defer rows.Close()
	var out []json.RawMessage
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		out = append(out, json.RawMessage(append([]byte(nil), data...)))
	}
	return out, rows.Err()
}

// AllTranscripts returns every cached transcript keyed by video id, for export.
func (s *Store) AllTranscripts() (map[string][]TranscriptCue, error) {
	if err := s.ensureTranscriptSchema(); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(`SELECT video_id, cue_index, start_ms, end_ms, text FROM transcript_cues ORDER BY video_id, cue_index`)
	if err != nil {
		return nil, fmt.Errorf("reading transcripts for export: %w", err)
	}
	defer rows.Close()
	out := map[string][]TranscriptCue{}
	for rows.Next() {
		var c TranscriptCue
		if err := rows.Scan(&c.VideoID, &c.Index, &c.StartMS, &c.EndMS, &c.Text); err != nil {
			return nil, err
		}
		out[c.VideoID] = append(out[c.VideoID], c)
	}
	return out, rows.Err()
}

// TranscriptCueCount returns the number of cached cues for a video (0 if none).
// Used by import to apply a "more complete wins" merge rule.
func (s *Store) TranscriptCueCount(videoID string) int {
	if err := s.ensureTranscriptSchema(); err != nil {
		return 0
	}
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM transcript_cues WHERE video_id = ?`, videoID).Scan(&n); err != nil {
		return 0
	}
	return n
}

var _ = context.Background
