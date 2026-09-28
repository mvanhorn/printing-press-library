// Package notebook stores restaurant snapshots separately from personal trip notes.
package notebook

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrListNotFound     = errors.New("saved list not found")
	ErrMemberNotFound   = errors.New("restaurant is not in this saved list")
	ErrSnapshotNotFound = errors.New("restaurant has not been fetched; use show with its canonical Tabelog URL first")
)

// Store uses the already-open generated SQLite connection. New does not access it.
type Store struct{ db *sql.DB }

func New(db *sql.DB) *Store { return &Store{db: db} }

// Validation helpers let commands reject invalid input before opening storage.
func ValidateName(name string) error { _, err := validateName(name); return err }
func ValidateID(id string) error     { return validateID(id) }
func ValidateNote(note string) error { return validateNote(note) }

// Init is explicit so help and dry runs never create or migrate a notebook.
func (s *Store) Init(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, statement := range []string{
		`CREATE TABLE IF NOT EXISTS tabelog_snapshots (id TEXT PRIMARY KEY, data TEXT NOT NULL, updated_at TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS tabelog_notebooks (name TEXT PRIMARY KEY, created_at TEXT NOT NULL, updated_at TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS tabelog_memberships (list_name TEXT NOT NULL, restaurant_id TEXT NOT NULL, note TEXT NOT NULL DEFAULT '', position INTEGER NOT NULL, added_at TEXT NOT NULL, updated_at TEXT NOT NULL, PRIMARY KEY(list_name, restaurant_id), FOREIGN KEY(list_name) REFERENCES tabelog_notebooks(name), FOREIGN KEY(restaurant_id) REFERENCES tabelog_snapshots(id))`,
		`CREATE INDEX IF NOT EXISTS tabelog_membership_order ON tabelog_memberships(list_name, position)`,
		`CREATE INDEX IF NOT EXISTS tabelog_membership_restaurant ON tabelog_memberships(restaurant_id)`,
	} {
		if _, err = tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("initialize notebook: %w", err)
		}
	}
	return tx.Commit()
}

func validateName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || !utf8.ValidString(name) || utf8.RuneCountInString(name) > 120 {
		return "", fmt.Errorf("list name must contain 1 to 120 characters")
	}
	for _, r := range name {
		if r < 32 || r == 127 {
			return "", fmt.Errorf("list name cannot contain control characters")
		}
	}
	return name, nil
}

func validateID(id string) error {
	if len(id) < 7 || len(id) > 10 {
		return fmt.Errorf("restaurant ID must be a fetched Tabelog numeric ID")
	}
	for _, r := range id {
		if r < '0' || r > '9' {
			return fmt.Errorf("restaurant ID must be a fetched Tabelog numeric ID")
		}
	}
	return nil
}

func validateNote(note string) error {
	if !utf8.ValidString(note) || len(note) > 16384 {
		return fmt.Errorf("note must be UTF-8 text no longer than 16 KiB")
	}
	return nil
}

func validateSnapshot(id string, data json.RawMessage) error {
	if err := validateID(id); err != nil {
		return err
	}
	if len(data) > 4*1024*1024 {
		return fmt.Errorf("normalized restaurant snapshot exceeds 4 MiB")
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		return fmt.Errorf("invalid restaurant snapshot: %w", err)
	}
	var foundID, name, url string
	if json.Unmarshal(object["id"], &foundID) != nil || foundID != id {
		return fmt.Errorf("snapshot ID does not match %s", id)
	}
	if json.Unmarshal(object["name"], &name) != nil || strings.TrimSpace(name) == "" {
		return fmt.Errorf("snapshot restaurant name is missing")
	}
	if json.Unmarshal(object["url"], &url) != nil || strings.TrimSpace(url) == "" {
		return fmt.Errorf("snapshot restaurant URL is missing")
	}
	return nil
}

// ReplaceSnapshot replaces the complete validated payload. It never merges old fields.
func (s *Store) ReplaceSnapshot(ctx context.Context, id string, data json.RawMessage) error {
	if err := validateSnapshot(id, data); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO tabelog_snapshots(id,data,updated_at) VALUES(?,?,?) ON CONFLICT(id) DO UPDATE SET data=excluded.data, updated_at=excluded.updated_at`, id, string(data), time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		return err
	}
	if err := pruneUnreferenced(ctx, tx); err != nil {
		return err
	}
	return tx.Commit()
}

// ReplaceSnapshots commits a fetched result set atomically after validating every item.
func (s *Store) ReplaceSnapshots(ctx context.Context, snapshots map[string]json.RawMessage) error {
	ids := make([]string, 0, len(snapshots))
	for id := range snapshots {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		data := snapshots[id]
		if err := validateSnapshot(id, data); err != nil {
			return err
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, id := range ids {
		data := snapshots[id]
		if _, err := tx.ExecContext(ctx, `INSERT INTO tabelog_snapshots(id,data,updated_at) VALUES(?,?,?) ON CONFLICT(id) DO UPDATE SET data=excluded.data, updated_at=excluded.updated_at`, id, string(data), now); err != nil {
			return err
		}
	}
	if err := pruneUnreferenced(ctx, tx); err != nil {
		return err
	}
	return tx.Commit()
}

const unsavedSnapshotBudget = 16 * 1024 * 1024

// Saved membership protects snapshots. Only oldest unreferenced payloads are evicted.
func pruneUnreferenced(ctx context.Context, tx *sql.Tx) error {
	var total int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(length(CAST(data AS BLOB))),0) FROM tabelog_snapshots s WHERE NOT EXISTS (SELECT 1 FROM tabelog_memberships m WHERE m.restaurant_id=s.id)`).Scan(&total); err != nil {
		return err
	}
	if total <= unsavedSnapshotBudget {
		return nil
	}
	rows, err := tx.QueryContext(ctx, `SELECT s.id,length(CAST(s.data AS BLOB)) FROM tabelog_snapshots s WHERE NOT EXISTS (SELECT 1 FROM tabelog_memberships m WHERE m.restaurant_id=s.id) ORDER BY s.updated_at,s.id`)
	if err != nil {
		return err
	}
	type victim struct {
		id    string
		bytes int64
	}
	victims := make([]victim, 0)
	for rows.Next() {
		var v victim
		if err := rows.Scan(&v.id, &v.bytes); err != nil {
			_ = rows.Close()
			return err
		}
		if total > unsavedSnapshotBudget {
			victims = append(victims, v)
			total -= v.bytes
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, v := range victims {
		if _, err := tx.ExecContext(ctx, `DELETE FROM tabelog_snapshots WHERE id=? AND NOT EXISTS (SELECT 1 FROM tabelog_memberships WHERE restaurant_id=?)`, v.id, v.id); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) Snapshot(ctx context.Context, id string) (json.RawMessage, error) {
	if err := validateID(id); err != nil {
		return nil, err
	}
	var data string
	if err := s.db.QueryRowContext(ctx, `SELECT data FROM tabelog_snapshots WHERE id=?`, id).Scan(&data); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrSnapshotNotFound
		}
		return nil, err
	}
	return json.RawMessage(data), nil
}

type Entry struct {
	ID        string          `json:"id"`
	Note      string          `json:"note"`
	AddedAt   string          `json:"added_at"`
	UpdatedAt string          `json:"updated_at"`
	Snapshot  json.RawMessage `json:"snapshot"`
}

type Summary struct {
	Name      string `json:"name"`
	Count     int    `json:"count"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func (s *Store) Add(ctx context.Context, name, id, note string, noteSet bool) error {
	name, err := validateName(name)
	if err != nil {
		return err
	}
	if err := validateID(id); err != nil {
		return err
	}
	if err := validateNote(note); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var exists int
	if err := tx.QueryRowContext(ctx, `SELECT 1 FROM tabelog_snapshots WHERE id=?`, id).Scan(&exists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrSnapshotNotFound
		}
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := tx.ExecContext(ctx, `INSERT INTO tabelog_notebooks(name,created_at,updated_at) VALUES(?,?,?) ON CONFLICT(name) DO UPDATE SET updated_at=excluded.updated_at`, name, now, now); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO tabelog_memberships(list_name,restaurant_id,note,position,added_at,updated_at) VALUES(?,?,?,(SELECT COALESCE(MAX(position),0)+1 FROM tabelog_memberships WHERE list_name=?),?,?) ON CONFLICT(list_name,restaurant_id) DO UPDATE SET note=CASE WHEN ? THEN excluded.note ELSE tabelog_memberships.note END, updated_at=excluded.updated_at`, name, id, note, name, now, now, noteSet); err != nil {
		return err
	}
	return tx.Commit()
}

// Entries drains and closes its single query before returning. Selected IDs keep list order.
func (s *Store) Entries(ctx context.Context, name string, selected []string) ([]Entry, error) {
	name, err := validateName(name)
	if err != nil {
		return nil, err
	}
	wanted := make(map[string]bool, len(selected))
	for _, id := range selected {
		if err := validateID(id); err != nil {
			return nil, err
		}
		wanted[id] = true
	}
	var exists int
	if err := s.db.QueryRowContext(ctx, `SELECT 1 FROM tabelog_notebooks WHERE name=?`, name).Scan(&exists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrListNotFound
		}
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT m.restaurant_id,m.note,m.added_at,m.updated_at,s.data FROM tabelog_memberships m LEFT JOIN tabelog_snapshots s ON s.id=m.restaurant_id WHERE m.list_name=? ORDER BY m.position,m.restaurant_id`, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	entries := make([]Entry, 0)
	for rows.Next() {
		var entry Entry
		var snapshot sql.NullString
		if err := rows.Scan(&entry.ID, &entry.Note, &entry.AddedAt, &entry.UpdatedAt, &snapshot); err != nil {
			return nil, err
		}
		if len(wanted) > 0 && !wanted[entry.ID] {
			continue
		}
		if !snapshot.Valid {
			return nil, fmt.Errorf("saved restaurant %s has no snapshot: %w", entry.ID, ErrSnapshotNotFound)
		}
		entry.Snapshot = json.RawMessage(snapshot.String)
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if len(wanted) > 0 {
		found := make(map[string]bool, len(entries))
		for _, entry := range entries {
			found[entry.ID] = true
		}
		for _, id := range selected {
			if !found[id] {
				return nil, fmt.Errorf("restaurant %s: %w", id, ErrMemberNotFound)
			}
		}
	}
	return entries, nil
}

func (s *Store) Lists(ctx context.Context) ([]Summary, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT n.name,COUNT(m.restaurant_id),n.created_at,n.updated_at FROM tabelog_notebooks n LEFT JOIN tabelog_memberships m ON m.list_name=n.name GROUP BY n.name,n.created_at,n.updated_at ORDER BY n.created_at,n.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	lists := make([]Summary, 0)
	for rows.Next() {
		var summary Summary
		if err := rows.Scan(&summary.Name, &summary.Count, &summary.CreatedAt, &summary.UpdatedAt); err != nil {
			return nil, err
		}
		lists = append(lists, summary)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	return lists, nil
}

func (s *Store) Note(ctx context.Context, name, id, note string) error {
	if err := validateNote(note); err != nil {
		return err
	}
	return s.mutateMember(ctx, name, id, `UPDATE tabelog_memberships SET note=?,updated_at=? WHERE list_name=? AND restaurant_id=?`, note)
}

func (s *Store) Remove(ctx context.Context, name, id string) error {
	return s.mutateMember(ctx, name, id, `DELETE FROM tabelog_memberships WHERE list_name=? AND restaurant_id=?`)
}

func (s *Store) mutateMember(ctx context.Context, name, id, statement string, note ...string) error {
	name, err := validateName(name)
	if err != nil {
		return err
	}
	if err := validateID(id); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	args := []any{name, id}
	if len(note) > 0 {
		args = []any{note[0], now, name, id}
	}
	result, err := tx.ExecContext(ctx, statement, args...)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrMemberNotFound
	}
	if _, err := tx.ExecContext(ctx, `UPDATE tabelog_notebooks SET updated_at=? WHERE name=?`, now, name); err != nil {
		return err
	}
	return tx.Commit()
}
