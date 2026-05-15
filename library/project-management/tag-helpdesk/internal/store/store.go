// Copyright 2026 andrea-m-piovesana. Licensed under Apache-2.0. See LICENSE.

// Package store manages the local SQLite cache for helpdesk tickets.
package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// DB wraps the SQLite connection.
type DB struct {
	conn *sql.DB
	Path string
}

// TicketRow is the flattened row stored in SQLite.
type TicketRow struct {
	ID              int
	Number          string
	Name            string
	DescriptionText string // HTML stripped to plain text
	PartnerID       int
	PartnerName     string
	PartnerEmail    string
	UserID          int
	UserName        string
	TeamID          int
	TeamName        string
	StageID         int
	StageName       string
	Priority        string
	KanbanState     string
	CategoryID      int
	CategoryName    string
	ChannelID       int
	ChannelName     string
	AssignedDate    string
	ClosedDate      string
	Closed          bool
	Unattended      bool
	LastStageUpdate string
	WriteDate       string
	Active          bool
	SyncedAt        string
}

// Open opens (creating if needed) the SQLite database at the given path.
func Open(path string) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("creating store directory: %w", err)
	}
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("opening SQLite: %w", err)
	}
	conn.SetMaxOpenConns(1)
	db := &DB{conn: conn, Path: path}
	if err := db.migrate(); err != nil {
		conn.Close()
		return nil, err
	}
	return db, nil
}

// Close closes the database connection.
func (db *DB) Close() error {
	return db.conn.Close()
}

func (db *DB) migrate() error {
	_, err := db.conn.Exec(`
PRAGMA journal_mode=WAL;
PRAGMA foreign_keys=ON;

CREATE TABLE IF NOT EXISTS tickets (
  id               INTEGER PRIMARY KEY,
  number           TEXT NOT NULL DEFAULT '',
  name             TEXT NOT NULL DEFAULT '',
  description_text TEXT NOT NULL DEFAULT '',
  partner_id       INTEGER NOT NULL DEFAULT 0,
  partner_name     TEXT NOT NULL DEFAULT '',
  partner_email    TEXT NOT NULL DEFAULT '',
  user_id          INTEGER NOT NULL DEFAULT 0,
  user_name        TEXT NOT NULL DEFAULT '',
  team_id          INTEGER NOT NULL DEFAULT 0,
  team_name        TEXT NOT NULL DEFAULT '',
  stage_id         INTEGER NOT NULL DEFAULT 0,
  stage_name       TEXT NOT NULL DEFAULT '',
  priority         TEXT NOT NULL DEFAULT '1',
  kanban_state     TEXT NOT NULL DEFAULT 'normal',
  category_id      INTEGER NOT NULL DEFAULT 0,
  category_name    TEXT NOT NULL DEFAULT '',
  channel_id       INTEGER NOT NULL DEFAULT 0,
  channel_name     TEXT NOT NULL DEFAULT '',
  assigned_date    TEXT NOT NULL DEFAULT '',
  closed_date      TEXT NOT NULL DEFAULT '',
  closed           INTEGER NOT NULL DEFAULT 0,
  unattended       INTEGER NOT NULL DEFAULT 0,
  last_stage_update TEXT NOT NULL DEFAULT '',
  write_date       TEXT NOT NULL DEFAULT '',
  active           INTEGER NOT NULL DEFAULT 1,
  synced_at        TEXT NOT NULL DEFAULT ''
);

CREATE VIRTUAL TABLE IF NOT EXISTS tickets_fts USING fts5(
  number, name, description_text, partner_name, partner_email,
  content='tickets', content_rowid='id'
);

CREATE TRIGGER IF NOT EXISTS tickets_ai AFTER INSERT ON tickets BEGIN
  INSERT INTO tickets_fts(rowid, number, name, description_text, partner_name, partner_email)
  VALUES (new.id, new.number, new.name, new.description_text, new.partner_name, new.partner_email);
END;

CREATE TRIGGER IF NOT EXISTS tickets_ad AFTER DELETE ON tickets BEGIN
  INSERT INTO tickets_fts(tickets_fts, rowid, number, name, description_text, partner_name, partner_email)
  VALUES ('delete', old.id, old.number, old.name, old.description_text, old.partner_name, old.partner_email);
END;

CREATE TRIGGER IF NOT EXISTS tickets_au AFTER UPDATE ON tickets BEGIN
  INSERT INTO tickets_fts(tickets_fts, rowid, number, name, description_text, partner_name, partner_email)
  VALUES ('delete', old.id, old.number, old.name, old.description_text, old.partner_name, old.partner_email);
  INSERT INTO tickets_fts(rowid, number, name, description_text, partner_name, partner_email)
  VALUES (new.id, new.number, new.name, new.description_text, new.partner_name, new.partner_email);
END;

CREATE TABLE IF NOT EXISTS sync_meta (
  key   TEXT PRIMARY KEY,
  value TEXT NOT NULL
);
`)
	return err
}

// UpsertTickets inserts or replaces a batch of tickets.
func (db *DB) UpsertTickets(rows []TicketRow) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
INSERT INTO tickets (
  id, number, name, description_text,
  partner_id, partner_name, partner_email,
  user_id, user_name, team_id, team_name,
  stage_id, stage_name, priority, kanban_state,
  category_id, category_name, channel_id, channel_name,
  assigned_date, closed_date, closed, unattended,
  last_stage_update, write_date, active, synced_at
) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
ON CONFLICT(id) DO UPDATE SET
  number=excluded.number, name=excluded.name,
  description_text=excluded.description_text,
  partner_id=excluded.partner_id, partner_name=excluded.partner_name,
  partner_email=excluded.partner_email,
  user_id=excluded.user_id, user_name=excluded.user_name,
  team_id=excluded.team_id, team_name=excluded.team_name,
  stage_id=excluded.stage_id, stage_name=excluded.stage_name,
  priority=excluded.priority, kanban_state=excluded.kanban_state,
  category_id=excluded.category_id, category_name=excluded.category_name,
  channel_id=excluded.channel_id, channel_name=excluded.channel_name,
  assigned_date=excluded.assigned_date, closed_date=excluded.closed_date,
  closed=excluded.closed, unattended=excluded.unattended,
  last_stage_update=excluded.last_stage_update, write_date=excluded.write_date,
  active=excluded.active, synced_at=excluded.synced_at
`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	now := time.Now().UTC().Format(time.RFC3339)
	for _, r := range rows {
		syncedAt := r.SyncedAt
		if syncedAt == "" {
			syncedAt = now
		}
		closedInt := 0
		if r.Closed {
			closedInt = 1
		}
		unattendedInt := 0
		if r.Unattended {
			unattendedInt = 1
		}
		activeInt := 1
		if !r.Active {
			activeInt = 0
		}
		_, err := stmt.Exec(
			r.ID, r.Number, r.Name, r.DescriptionText,
			r.PartnerID, r.PartnerName, r.PartnerEmail,
			r.UserID, r.UserName, r.TeamID, r.TeamName,
			r.StageID, r.StageName, r.Priority, r.KanbanState,
			r.CategoryID, r.CategoryName, r.ChannelID, r.ChannelName,
			r.AssignedDate, r.ClosedDate, closedInt, unattendedInt,
			r.LastStageUpdate, r.WriteDate, activeInt, syncedAt,
		)
		if err != nil {
			return fmt.Errorf("upserting ticket %d: %w", r.ID, err)
		}
	}
	return tx.Commit()
}

// SetMeta stores a key-value pair in sync_meta.
func (db *DB) SetMeta(key, value string) error {
	_, err := db.conn.Exec(`INSERT INTO sync_meta(key, value) VALUES (?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value)
	return err
}

// GetMeta retrieves a value from sync_meta. Returns "" if not found.
func (db *DB) GetMeta(key string) string {
	var val string
	db.conn.QueryRow(`SELECT value FROM sync_meta WHERE key=?`, key).Scan(&val)
	return val
}

// Filter parameters for ListTickets.
type Filter struct {
	Closed     *bool
	Unattended *bool
	UserName   string
	TeamName   string
	Priority   string
	Active     *bool
	Limit      int
	Offset     int
}

// ListTickets returns tickets from the local store matching the filter.
func (db *DB) ListTickets(f Filter) ([]TicketRow, error) {
	where := []string{}
	args := []interface{}{}

	if f.Active != nil {
		where = append(where, "active=?")
		if *f.Active {
			args = append(args, 1)
		} else {
			args = append(args, 0)
		}
	}
	if f.Closed != nil {
		where = append(where, "closed=?")
		if *f.Closed {
			args = append(args, 1)
		} else {
			args = append(args, 0)
		}
	}
	if f.Unattended != nil {
		where = append(where, "unattended=?")
		if *f.Unattended {
			args = append(args, 1)
		} else {
			args = append(args, 0)
		}
	}
	if f.UserName != "" {
		where = append(where, "LOWER(user_name) LIKE LOWER(?)")
		args = append(args, "%"+f.UserName+"%")
	}
	if f.TeamName != "" {
		where = append(where, "LOWER(team_name) LIKE LOWER(?)")
		args = append(args, "%"+f.TeamName+"%")
	}
	if f.Priority != "" {
		where = append(where, "priority=?")
		args = append(args, f.Priority)
	}

	q := "SELECT * FROM tickets"
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	q += " ORDER BY priority DESC, id DESC"
	if f.Limit > 0 {
		q += fmt.Sprintf(" LIMIT %d OFFSET %d", f.Limit, f.Offset)
	}
	return db.queryTickets(q, args...)
}

// GetTicket returns a single ticket by ID from the local store.
func (db *DB) GetTicket(id int) (*TicketRow, error) {
	rows, err := db.queryTickets("SELECT * FROM tickets WHERE id=?", id)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("ticket %d not found in local store (run sync first)", id)
	}
	return &rows[0], nil
}

// GetTicketByNumber returns a ticket by its number field.
func (db *DB) GetTicketByNumber(number string) (*TicketRow, error) {
	rows, err := db.queryTickets("SELECT * FROM tickets WHERE number=?", number)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("ticket %q not found in local store (run sync first)", number)
	}
	return &rows[0], nil
}

// SearchTickets performs FTS5 full-text search.
func (db *DB) SearchTickets(query string, limit int) ([]TicketRow, error) {
	q := `
SELECT t.* FROM tickets t
JOIN tickets_fts f ON t.id = f.rowid
WHERE tickets_fts MATCH ?
ORDER BY rank
LIMIT ?`
	return db.queryTickets(q, query, limit)
}

// StaleTickets returns open tickets with no write_date activity in the last N days.
func (db *DB) StaleTickets(days int) ([]TicketRow, error) {
	cutoff := time.Now().UTC().AddDate(0, 0, -days).Format("2006-01-02")
	q := `SELECT * FROM tickets WHERE closed=0 AND active=1 AND (write_date='' OR write_date < ?) ORDER BY write_date ASC`
	return db.queryTickets(q, cutoff)
}

// UnattendedTickets returns tickets in unattended stages.
func (db *DB) UnattendedTickets() ([]TicketRow, error) {
	return db.queryTickets("SELECT * FROM tickets WHERE unattended=1 AND closed=0 AND active=1 ORDER BY last_stage_update ASC")
}

// OverdueTickets returns high/urgent open tickets older than N days.
func (db *DB) OverdueTickets(days int) ([]TicketRow, error) {
	cutoff := time.Now().UTC().AddDate(0, 0, -days).Format("2006-01-02")
	q := `SELECT * FROM tickets WHERE closed=0 AND active=1 AND priority IN ('2','3') AND (write_date='' OR write_date < ?) ORDER BY priority DESC, write_date ASC`
	return db.queryTickets(q, cutoff)
}

// AgentSummary holds per-agent ticket stats.
type AgentSummary struct {
	UserName   string
	UserID     int
	OpenCount  int
	TotalCount int
	AvgAgeDays float64
}

// ByAgent returns open ticket counts and avg age grouped by assigned agent.
func (db *DB) ByAgent() ([]AgentSummary, error) {
	q := `
SELECT user_id, user_name, COUNT(*) as open_count,
  AVG(CASE WHEN write_date != '' THEN JULIANDAY('now') - JULIANDAY(write_date) ELSE 0 END) as avg_age
FROM tickets
WHERE closed=0 AND active=1
GROUP BY user_id, user_name
ORDER BY open_count DESC`
	rows, err := db.conn.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []AgentSummary
	for rows.Next() {
		var s AgentSummary
		if err := rows.Scan(&s.UserID, &s.UserName, &s.OpenCount, &s.AvgAgeDays); err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

// TeamSummary holds per-team ticket stats.
type TeamSummary struct {
	TeamName   string
	TeamID     int
	OpenCount  int
	TotalCount int
}

// ByTeam returns open ticket counts grouped by team.
func (db *DB) ByTeam() ([]TeamSummary, error) {
	q := `
SELECT team_id, team_name, COUNT(*) as open_count
FROM tickets
WHERE closed=0 AND active=1
GROUP BY team_id, team_name
ORDER BY open_count DESC`
	rows, err := db.conn.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []TeamSummary
	for rows.Next() {
		var s TeamSummary
		if err := rows.Scan(&s.TeamID, &s.TeamName, &s.OpenCount); err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

// CategorySummary holds per-category ticket stats.
type CategorySummary struct {
	CategoryName string
	CategoryID   int
	OpenCount    int
}

// ByCategory returns open ticket counts grouped by category.
func (db *DB) ByCategory() ([]CategorySummary, error) {
	q := `
SELECT category_id, COALESCE(NULLIF(category_name,''), '(none)') as category_name, COUNT(*) as open_count
FROM tickets
WHERE closed=0 AND active=1
GROUP BY category_id, category_name
ORDER BY open_count DESC`
	rows, err := db.conn.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []CategorySummary
	for rows.Next() {
		var s CategorySummary
		if err := rows.Scan(&s.CategoryID, &s.CategoryName, &s.OpenCount); err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

// KPISummary holds aggregate KPIs for the summary command.
type KPISummary struct {
	TotalOpen       int
	TotalClosed     int
	TotalActive     int
	Unattended      int
	HighPriority    int
	Unassigned      int
	LastSyncAt      string
	StaleCount      int     // no activity 7+ days
	AvgAgeDaysOpen  float64 // avg write_date age of open tickets
}

// GetKPIs returns aggregate KPI data for all open tickets.
func (db *DB) GetKPIs() (*KPISummary, error) {
	var s KPISummary
	s.LastSyncAt = db.GetMeta("last_sync")

	err := db.conn.QueryRow(`SELECT
    SUM(CASE WHEN closed=0 AND active=1 THEN 1 ELSE 0 END),
    SUM(CASE WHEN closed=1 THEN 1 ELSE 0 END),
    SUM(CASE WHEN active=1 THEN 1 ELSE 0 END),
    SUM(CASE WHEN unattended=1 AND closed=0 AND active=1 THEN 1 ELSE 0 END),
    SUM(CASE WHEN priority IN ('2','3') AND closed=0 AND active=1 THEN 1 ELSE 0 END),
    SUM(CASE WHEN user_id=0 AND closed=0 AND active=1 THEN 1 ELSE 0 END),
    SUM(CASE WHEN closed=0 AND active=1 AND write_date < DATE('now','-7 days') THEN 1 ELSE 0 END),
    AVG(CASE WHEN closed=0 AND active=1 AND write_date != '' THEN JULIANDAY('now') - JULIANDAY(write_date) ELSE NULL END)
  FROM tickets`).Scan(
		&s.TotalOpen, &s.TotalClosed, &s.TotalActive,
		&s.Unattended, &s.HighPriority, &s.Unassigned,
		&s.StaleCount, &s.AvgAgeDaysOpen,
	)
	return &s, err
}

func (db *DB) queryTickets(q string, args ...interface{}) ([]TicketRow, error) {
	rows, err := db.conn.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []TicketRow
	for rows.Next() {
		var r TicketRow
		var closed, unattended, active int
		err := rows.Scan(
			&r.ID, &r.Number, &r.Name, &r.DescriptionText,
			&r.PartnerID, &r.PartnerName, &r.PartnerEmail,
			&r.UserID, &r.UserName, &r.TeamID, &r.TeamName,
			&r.StageID, &r.StageName, &r.Priority, &r.KanbanState,
			&r.CategoryID, &r.CategoryName, &r.ChannelID, &r.ChannelName,
			&r.AssignedDate, &r.ClosedDate, &closed, &unattended,
			&r.LastStageUpdate, &r.WriteDate, &active, &r.SyncedAt,
		)
		if err != nil {
			return nil, err
		}
		r.Closed = closed == 1
		r.Unattended = unattended == 1
		r.Active = active == 1
		result = append(result, r)
	}
	return result, rows.Err()
}

// Count returns the total number of tickets in the store.
func (db *DB) Count() int {
	var n int
	db.conn.QueryRow("SELECT COUNT(*) FROM tickets").Scan(&n)
	return n
}
