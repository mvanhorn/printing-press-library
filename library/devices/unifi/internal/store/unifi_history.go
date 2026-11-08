// Copyright 2026 Ricardo Cabral and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored, not generator output: snapshot, first-seen and journal access
// for the novel commands. Tables are created in extras.go.

package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// SnapshotKeep is how many snapshots per console survive pruning. Drift needs two
// and digest a window of them; unbounded retention would grow by a full config
// copy on every sync.
const SnapshotKeep = 60

// SnapshotRow is one entity in a snapshot.
type SnapshotRow struct {
	ResourceType string
	EntityID     string
	Data         json.RawMessage
}

// FirstSeenFields carries the display fields newcomer shows.
type FirstSeenFields struct {
	Label, MAC, IP string
}

// Snapshot is a point-in-time copy of one console's mirror.
type Snapshot struct {
	ID        int64  `json:"id"`
	ConsoleID string `json:"console_id"`
	TakenAt   string `json:"taken_at"`
}

// SaveSnapshot writes rows and updates first-seen in one transaction, so a crash
// cannot record an entity as seen in a snapshot that does not exist. An entity
// present in the first snapshot to hold its resource type is marked baseline: it was
// already there when history began, so its first-seen date says nothing about when it
// joined. Per type rather than per console because a first sync may reach only some
// types, and the rest would otherwise all report as new on the next full sync. The cost
// is that the first camera on a console that had none is baseline too.
func (s *Store) SaveSnapshot(ctx context.Context, console string, takenAt time.Time, rows []SnapshotRow, fields map[string]FirstSeenFields, trackFirstSeen func(resourceType string) bool) (int64, error) {
	s.lockForWrite()
	defer s.unlockAfterWrite()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	stamp := takenAt.UTC().Format(time.RFC3339)
	// Decided before any row is written, so one snapshot's rows of a type all agree.
	baselineTypes := map[string]bool{}
	for _, row := range rows {
		if _, decided := baselineTypes[row.ResourceType]; decided {
			continue
		}
		var prior int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM unifi_first_seen WHERE console_id = ? AND resource_type = ?`, console, row.ResourceType).Scan(&prior); err != nil {
			return 0, err
		}
		baselineTypes[row.ResourceType] = prior == 0
	}
	res, err := tx.ExecContext(ctx, `INSERT INTO unifi_snapshots (console_id, taken_at) VALUES (?, ?)`, console, stamp)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	for _, row := range rows {
		// Rows come from the sealed mirror, but this table keeps 60 generations, so it
		// gets its own gate rather than relying on every caller.
		if _, err := tx.ExecContext(ctx, `INSERT OR REPLACE INTO unifi_snapshot_rows (snapshot_id, resource_type, entity_id, data) VALUES (?, ?, ?, ?)`, id, row.ResourceType, row.EntityID, string(sealStoredJSON(row.Data))); err != nil {
			return 0, err
		}
		if trackFirstSeen == nil || !trackFirstSeen(row.ResourceType) {
			continue
		}
		f := fields[row.ResourceType+"|"+row.EntityID]
		baseline := 0
		if baselineTypes[row.ResourceType] {
			baseline = 1
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO unifi_first_seen (console_id, resource_type, entity_id, first_seen_at, last_seen_at, baseline, label, mac, ip)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(console_id, resource_type, entity_id) DO UPDATE SET last_seen_at = excluded.last_seen_at, label = excluded.label, mac = excluded.mac, ip = excluded.ip`,
			console, row.ResourceType, row.EntityID, stamp, stamp, baseline, f.Label, f.MAC, f.IP); err != nil {
			return 0, err
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM unifi_snapshot_rows WHERE snapshot_id IN (
		SELECT id FROM unifi_snapshots WHERE console_id = ? ORDER BY id DESC LIMIT -1 OFFSET ?)`, console, SnapshotKeep); err != nil {
		return 0, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM unifi_snapshots WHERE id IN (
		SELECT id FROM unifi_snapshots WHERE console_id = ? ORDER BY id DESC LIMIT -1 OFFSET ?)`, console, SnapshotKeep); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return id, nil
}

// ListSnapshots returns one console's snapshots oldest first; "" means all.
func (s *Store) ListSnapshots(ctx context.Context, console string) ([]Snapshot, error) {
	query := `SELECT id, console_id, taken_at FROM unifi_snapshots`
	var args []any
	if console != "" {
		query += ` WHERE console_id = ?`
		args = append(args, console)
	}
	query += ` ORDER BY id`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Snapshot, 0)
	for rows.Next() {
		var snap Snapshot
		if err := rows.Scan(&snap.ID, &snap.ConsoleID, &snap.TakenAt); err != nil {
			return nil, err
		}
		out = append(out, snap)
	}
	return out, rows.Err()
}

// SnapshotRows loads a snapshot keyed "resource_type|entity_id".
func (s *Store) SnapshotRows(ctx context.Context, id int64) (map[string]SnapshotRow, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT resource_type, entity_id, data FROM unifi_snapshot_rows WHERE snapshot_id = ?`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]SnapshotRow{}
	for rows.Next() {
		var row SnapshotRow
		var data string
		if err := rows.Scan(&row.ResourceType, &row.EntityID, &data); err != nil {
			return nil, err
		}
		row.Data = json.RawMessage(data)
		out[row.ResourceType+"|"+row.EntityID] = row
	}
	return out, rows.Err()
}

// FirstSeenRow is a first-seen ledger entry.
type FirstSeenRow struct {
	ConsoleID    string
	ResourceType string
	EntityID     string
	FirstSeenAt  string
	LastSeenAt   string
	Baseline     bool
	Label        string
	MAC          string
	IP           string
}

// FirstSeenSince returns non-baseline entities first seen at or after cutoff.
func (s *Store) FirstSeenSince(ctx context.Context, console string, cutoff time.Time) ([]FirstSeenRow, error) {
	query := `SELECT console_id, resource_type, entity_id, first_seen_at, last_seen_at, baseline, label, mac, ip
		FROM unifi_first_seen WHERE baseline = 0 AND first_seen_at >= ?`
	args := []any{cutoff.UTC().Format(time.RFC3339)}
	if console != "" {
		query += ` AND console_id = ?`
		args = append(args, console)
	}
	query += ` ORDER BY first_seen_at, console_id, entity_id`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]FirstSeenRow, 0)
	for rows.Next() {
		var row FirstSeenRow
		var baseline int
		if err := rows.Scan(&row.ConsoleID, &row.ResourceType, &row.EntityID, &row.FirstSeenAt, &row.LastSeenAt, &baseline, &row.Label, &row.MAC, &row.IP); err != nil {
			return nil, err
		}
		row.Baseline = baseline != 0
		out = append(out, row)
	}
	return out, rows.Err()
}

// JournalEntry is one row of the change journal.
type JournalEntry struct {
	ID              string          `json:"id"`
	ConsoleID       string          `json:"console_id"`
	CreatedAt       string          `json:"created_at"`
	UpdatedAt       string          `json:"updated_at"`
	State           string          `json:"state"`
	Kind            string          `json:"kind"`
	Method          string          `json:"method"`
	Path            string          `json:"path"`
	ReadPath        string          `json:"read_path,omitempty"`
	ResourceType    string          `json:"resource_type,omitempty"`
	EntityID        string          `json:"entity_id,omitempty"`
	RequestBody     json.RawMessage `json:"request_body,omitempty"`
	BeforeImage     json.RawMessage `json:"before_image,omitempty"`
	AfterImage      json.RawMessage `json:"after_image,omitempty"`
	SecretsScrubbed bool            `json:"secrets_scrubbed"`
	Undoes          string          `json:"undoes,omitempty"`
	UndoneBy        string          `json:"undone_by,omitempty"`
	Transport       string          `json:"transport,omitempty"`
	Detail          string          `json:"detail,omitempty"`
}

// ErrJournalEntryNotFound means no journal row has that id.
var ErrJournalEntryNotFound = errors.New("journal entry not found")

// InsertJournalEntry commits on its own: the intent must be durable before the
// mutation is sent, so a crash mid-write still leaves evidence of the attempt.
func (s *Store) InsertJournalEntry(ctx context.Context, e JournalEntry) error {
	s.lockForWrite()
	defer s.unlockAfterWrite()
	secrets := 0
	if e.SecretsScrubbed {
		secrets = 1
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO unifi_change_journal
		(id, console_id, created_at, updated_at, state, kind, method, path, read_path, resource_type, entity_id, request_body, before_image, after_image, secrets_scrubbed, undoes, undone_by, transport, detail)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.ID, e.ConsoleID, e.CreatedAt, e.UpdatedAt, e.State, e.Kind, e.Method, e.Path, e.ReadPath, e.ResourceType, e.EntityID,
		// The change runner scrubs first; this is the store refusing to depend on that.
		string(sealStoredJSON(e.RequestBody)), string(sealStoredJSON(e.BeforeImage)), string(sealStoredJSON(e.AfterImage)), secrets, e.Undoes, e.UndoneBy, e.Transport, e.Detail)
	return err
}

// UpdateJournalEntry records the outcome of an attempt.
func (s *Store) UpdateJournalEntry(ctx context.Context, e JournalEntry) error {
	s.lockForWrite()
	defer s.unlockAfterWrite()
	_, err := s.db.ExecContext(ctx, `UPDATE unifi_change_journal SET updated_at = ?, state = ?, read_path = ?, entity_id = ?, after_image = ?, undone_by = ?, detail = ? WHERE id = ?`,
		e.UpdatedAt, e.State, e.ReadPath, e.EntityID, string(sealStoredJSON(e.AfterImage)), e.UndoneBy, e.Detail, e.ID)
	return err
}

const journalColumns = `id, console_id, created_at, updated_at, state, kind, method, path, read_path, resource_type, entity_id, request_body, before_image, after_image, secrets_scrubbed, undoes, undone_by, transport, detail`

func scanJournal(scan func(...any) error) (JournalEntry, error) {
	var e JournalEntry
	var body, before, after string
	var secrets int
	if err := scan(&e.ID, &e.ConsoleID, &e.CreatedAt, &e.UpdatedAt, &e.State, &e.Kind, &e.Method, &e.Path, &e.ReadPath, &e.ResourceType, &e.EntityID, &body, &before, &after, &secrets, &e.Undoes, &e.UndoneBy, &e.Transport, &e.Detail); err != nil {
		return e, err
	}
	e.SecretsScrubbed = secrets != 0
	if strings.TrimSpace(body) != "" {
		e.RequestBody = json.RawMessage(body)
	}
	if strings.TrimSpace(before) != "" {
		e.BeforeImage = json.RawMessage(before)
	}
	if strings.TrimSpace(after) != "" {
		e.AfterImage = json.RawMessage(after)
	}
	return e, nil
}

// GetJournalEntry loads one entry by id.
func (s *Store) GetJournalEntry(ctx context.Context, id string) (JournalEntry, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+journalColumns+` FROM unifi_change_journal WHERE id = ?`, id)
	e, err := scanJournal(row.Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return e, ErrJournalEntryNotFound
	}
	return e, err
}

// ListJournalEntries returns entries oldest first; "" console means all.
func (s *Store) ListJournalEntries(ctx context.Context, console string, since time.Time) ([]JournalEntry, error) {
	query := `SELECT ` + journalColumns + ` FROM unifi_change_journal WHERE created_at >= ?`
	args := []any{since.UTC().Format(time.RFC3339Nano)}
	if console != "" {
		query += ` AND console_id = ?`
		args = append(args, console)
	}
	query += ` ORDER BY created_at, id`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]JournalEntry, 0)
	for rows.Next() {
		e, err := scanJournal(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
