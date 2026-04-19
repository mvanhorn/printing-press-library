// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

// P2 transcendence helpers: TTL cache, tail snapshot, and person-touch tracking.
// These back the `intersect`, `since`, `tail`, `engagement`, and `stale` commands.

package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// P2CacheGet returns cached data for a key if present and not expired.
// Returns (nil, false) on miss or expiry.
func (s *Store) P2CacheGet(key string) (json.RawMessage, bool) {
	var data string
	var expires string
	err := s.db.QueryRow(
		`SELECT data, expires_at FROM p2_cache WHERE key = ? AND expires_at > CURRENT_TIMESTAMP`,
		key,
	).Scan(&data, &expires)
	if err != nil {
		return nil, false
	}
	return json.RawMessage(data), true
}

// P2CacheSet stores data under key with the given TTL. A zero or negative TTL
// caches for 1 hour by default.
func (s *Store) P2CacheSet(key string, data json.RawMessage, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = time.Hour
	}
	expires := time.Now().Add(ttl).UTC().Format("2006-01-02 15:04:05")
	_, err := s.db.Exec(
		`INSERT INTO p2_cache (key, data, expires_at) VALUES (?, ?, ?)
		 ON CONFLICT(key) DO UPDATE SET data = excluded.data, expires_at = excluded.expires_at`,
		key, string(data), expires,
	)
	return err
}

// TailSnapshotGet returns the last-seen IDs for a source and whether a snapshot existed.
func (s *Store) TailSnapshotGet(source string) (map[string]bool, bool, error) {
	var raw string
	err := s.db.QueryRow(
		`SELECT last_seen_ids FROM tail_snapshot WHERE source = ?`, source,
	).Scan(&raw)
	if err == sql.ErrNoRows {
		return map[string]bool{}, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var ids []string
	if err := json.Unmarshal([]byte(raw), &ids); err != nil {
		return map[string]bool{}, true, nil
	}
	out := make(map[string]bool, len(ids))
	for _, id := range ids {
		out[id] = true
	}
	return out, true, nil
}

// TailSnapshotSet stores the current set of seen IDs. A cap is applied so the
// snapshot never grows unboundedly.
func (s *Store) TailSnapshotSet(source string, ids []string) error {
	const cap = 2000
	if len(ids) > cap {
		ids = ids[len(ids)-cap:]
	}
	// Ensure deterministic output regardless of caller order.
	sort.Strings(ids)
	b, err := json.Marshal(ids)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(
		`INSERT INTO tail_snapshot (source, last_seen_ids, updated_at)
		 VALUES (?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(source) DO UPDATE SET last_seen_ids = excluded.last_seen_ids, updated_at = CURRENT_TIMESTAMP`,
		source, string(b),
	)
	return err
}

// RecordTouch logs an interaction with a person for engagement scoring.
func (s *Store) RecordTouch(personKey, source, eventType string, eventTime time.Time, data json.RawMessage) error {
	if personKey == "" || source == "" || eventType == "" {
		return fmt.Errorf("RecordTouch: personKey, source, eventType required")
	}
	if eventTime.IsZero() {
		eventTime = time.Now().UTC()
	}
	var dataArg any
	if len(data) == 0 {
		dataArg = nil
	} else {
		dataArg = string(data)
	}
	_, err := s.db.Exec(
		`INSERT INTO person_touches (person_key, source, event_type, event_time, data)
		 VALUES (?, ?, ?, ?, ?)`,
		personKey, source, eventType, eventTime.UTC().Format(time.RFC3339), dataArg,
	)
	return err
}

// TouchEvent is a materialized row from person_touches.
type TouchEvent struct {
	PersonKey string          `json:"person_key"`
	Source    string          `json:"source"`
	EventType string          `json:"event_type"`
	EventTime time.Time       `json:"event_time"`
	Data      json.RawMessage `json:"data,omitempty"`
}

// ListTouches returns all recorded touches for a person key, most-recent first.
// If sinceHours > 0, only touches within that window are returned.
func (s *Store) ListTouches(personKey string, sinceHours int) ([]TouchEvent, error) {
	var rows *sql.Rows
	var err error
	if sinceHours > 0 {
		rows, err = s.db.Query(
			`SELECT person_key, source, event_type, event_time, data
			 FROM person_touches
			 WHERE person_key = ? AND event_time >= datetime('now', ?)
			 ORDER BY event_time DESC`,
			personKey, fmt.Sprintf("-%d hours", sinceHours),
		)
	} else {
		rows, err = s.db.Query(
			`SELECT person_key, source, event_type, event_time, data
			 FROM person_touches
			 WHERE person_key = ?
			 ORDER BY event_time DESC`,
			personKey,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []TouchEvent
	for rows.Next() {
		var e TouchEvent
		var tsStr string
		var dataStr sql.NullString
		if err := rows.Scan(&e.PersonKey, &e.Source, &e.EventType, &tsStr, &dataStr); err != nil {
			return nil, err
		}
		// SQLite may return either RFC3339 or "YYYY-MM-DD HH:MM:SS".
		t, terr := time.Parse(time.RFC3339, tsStr)
		if terr != nil {
			t, _ = time.Parse("2006-01-02 15:04:05", tsStr)
		}
		e.EventTime = t
		if dataStr.Valid {
			e.Data = json.RawMessage(dataStr.String)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// DistinctTouchPersonsSince returns all unique person keys with at least one
// touch in the last sinceHours.
func (s *Store) DistinctTouchPersonsSince(sinceHours int) ([]string, error) {
	rows, err := s.db.Query(
		`SELECT DISTINCT person_key FROM person_touches WHERE event_time >= datetime('now', ?)`,
		fmt.Sprintf("-%d hours", sinceHours),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

// ListRecentResources returns full-data rows for a resource type updated after
// the given cutoff. Returns empty slice if the resource type is unknown.
func (s *Store) ListRecentResources(resourceType string, since time.Time, limit int) ([]json.RawMessage, error) {
	if limit <= 0 {
		limit = 200
	}
	rows, err := s.db.Query(
		`SELECT data FROM resources
		 WHERE resource_type = ? AND updated_at >= ?
		 ORDER BY updated_at DESC LIMIT ?`,
		resourceType, since.UTC().Format(time.RFC3339), limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []json.RawMessage
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		out = append(out, json.RawMessage(data))
	}
	return out, rows.Err()
}

// UnifiedPerson is the projection exposed to `graph export` and friends.
type UnifiedPerson struct {
	ID               int64           `json:"id"`
	FullName         string          `json:"name"`
	LinkedInURL      string          `json:"linkedin_url,omitempty"`
	HappenstanceUUID string          `json:"happenstance_uuid,omitempty"`
	Company          string          `json:"company,omitempty"`
	Title            string          `json:"title,omitempty"`
	Location         string          `json:"location,omitempty"`
	Sources          []string        `json:"sources"`
	HPData           json.RawMessage `json:"hp_data,omitempty"`
	LIData           json.RawMessage `json:"li_data,omitempty"`
}

// ListPeople returns all unified people up to limit.
func (s *Store) ListPeople(limit int) ([]UnifiedPerson, error) {
	if limit <= 0 {
		limit = 1000
	}
	rows, err := s.db.Query(
		`SELECT id, full_name, COALESCE(linkedin_url, ''), COALESCE(happenstance_uuid, ''),
		        COALESCE(company, ''), COALESCE(title, ''), COALESCE(location, ''),
		        sources, COALESCE(hp_data, ''), COALESCE(li_data, '')
		 FROM people
		 ORDER BY last_seen DESC LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []UnifiedPerson
	for rows.Next() {
		var p UnifiedPerson
		var sources, hpData, liData string
		if err := rows.Scan(&p.ID, &p.FullName, &p.LinkedInURL, &p.HappenstanceUUID,
			&p.Company, &p.Title, &p.Location, &sources, &hpData, &liData); err != nil {
			return nil, err
		}
		p.Sources = splitCSV(sources)
		if hpData != "" {
			p.HPData = json.RawMessage(hpData)
		}
		if liData != "" {
			p.LIData = json.RawMessage(liData)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ConnectionEdge is the materialized row from connection_edges.
type ConnectionEdge struct {
	ID             int64   `json:"id"`
	ViewerIdentity string  `json:"viewer_identity"`
	PersonID       int64   `json:"person_id"`
	Source         string  `json:"source"`
	Strength       float64 `json:"strength"`
}

// ListEdges returns all edges up to limit.
func (s *Store) ListEdges(limit int) ([]ConnectionEdge, error) {
	if limit <= 0 {
		limit = 10000
	}
	rows, err := s.db.Query(
		`SELECT id, viewer_identity, person_id, source, strength FROM connection_edges LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ConnectionEdge
	for rows.Next() {
		var e ConnectionEdge
		if err := rows.Scan(&e.ID, &e.ViewerIdentity, &e.PersonID, &e.Source, &e.Strength); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
