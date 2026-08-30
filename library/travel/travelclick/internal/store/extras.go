// Copyright 2026 Allen Lew and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// migrateExtras runs after the generated store migrations and before the
// schema-version stamp. It is the canonical place for novel-feature auxiliary
// tables that need to live in the local store.
//
// Edit this file when adding tables for novel commands. Keep migrations
// idempotent with CREATE TABLE IF NOT EXISTS / CREATE INDEX IF NOT EXISTS so
// every store open can safely re-run them.
func (s *Store) migrateExtras(ctx context.Context, conn *sql.Conn) error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS hotel_aliases (
			alias TEXT PRIMARY KEY,
			hotel_id TEXT NOT NULL,
			created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS rate_snapshots (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			hotel_id TEXT NOT NULL,
			check_in TEXT NOT NULL,
			check_out TEXT NOT NULL,
			room_type_code TEXT,
			room_type_name TEXT,
			rate_plan_code TEXT,
			rate_plan_name TEXT,
			nightly_rate REAL,
			currency TEXT,
			captured_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_rate_snapshots_hotel ON rate_snapshots(hotel_id, captured_at)`,
		`CREATE TABLE IF NOT EXISTS code_checks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			hotel_id TEXT NOT NULL,
			code_type TEXT NOT NULL,
			code TEXT NOT NULL,
			valid INTEGER NOT NULL,
			error_code TEXT,
			error_message TEXT,
			checked_at TEXT NOT NULL
		)`,
	}
	for _, m := range migrations {
		if _, err := conn.ExecContext(ctx, m); err != nil {
			return fmt.Errorf("extra migration failed: %w", err)
		}
	}
	return nil
}

type HotelAlias struct {
	Alias     string `json:"alias"`
	HotelID   string `json:"hotel_id"`
	CreatedAt string `json:"created_at"`
}

func (s *Store) UpsertHotelAlias(ctx context.Context, alias string, hotelID string) error {
	s.lockForWrite()
	defer s.unlockAfterWrite()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO hotel_aliases (alias, hotel_id, created_at)
		 VALUES (?, ?, ?)
		 ON CONFLICT(alias) DO UPDATE SET hotel_id = excluded.hotel_id, created_at = excluded.created_at`,
		alias, hotelID, time.Now().UTC().Format(time.RFC3339),
	)
	return err
}

func (s *Store) ResolveHotelID(ctx context.Context, aliasOrID string) (string, error) {
	aliasOrID = strings.TrimSpace(aliasOrID)
	if aliasOrID == "" {
		return "", fmt.Errorf("empty hotel alias or ID")
	}
	isAllDigits := true
	for _, r := range aliasOrID {
		if r < '0' || r > '9' {
			isAllDigits = false
			break
		}
	}
	if isAllDigits {
		return aliasOrID, nil
	}
	var hotelID string
	err := s.db.QueryRowContext(ctx, `SELECT hotel_id FROM hotel_aliases WHERE alias = ?`, aliasOrID).Scan(&hotelID)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("hotel alias %q not found", aliasOrID)
	}
	if err != nil {
		return "", err
	}
	return hotelID, nil
}

func (s *Store) ListHotelAliases(ctx context.Context) ([]HotelAlias, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT alias, hotel_id, created_at FROM hotel_aliases ORDER BY alias ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []HotelAlias
	for rows.Next() {
		var a HotelAlias
		if err := rows.Scan(&a.Alias, &a.HotelID, &a.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, a)
	}
	return list, rows.Err()
}

func (s *Store) RemoveHotelAlias(ctx context.Context, alias string) (bool, error) {
	s.lockForWrite()
	defer s.unlockAfterWrite()
	res, err := s.db.ExecContext(ctx, `DELETE FROM hotel_aliases WHERE alias = ?`, alias)
	if err != nil {
		return false, err
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return rowsAffected > 0, nil
}

type RateSnapshot struct {
	ID           int64   `json:"id,omitempty"`
	HotelID      string  `json:"hotel_id"`
	CheckIn      string  `json:"check_in"`
	CheckOut     string  `json:"check_out"`
	RoomTypeCode string  `json:"room_type_code"`
	RoomTypeName string  `json:"room_type_name"`
	RatePlanCode string  `json:"rate_plan_code"`
	RatePlanName string  `json:"rate_plan_name"`
	NightlyRate  float64 `json:"nightly_rate"`
	Currency     string  `json:"currency"`
	CapturedAt   string  `json:"captured_at"`
}

func (s *Store) InsertRateSnapshot(ctx context.Context, snapshot *RateSnapshot) error {
	s.lockForWrite()
	defer s.unlockAfterWrite()
	capturedAt := snapshot.CapturedAt
	if capturedAt == "" {
		capturedAt = time.Now().UTC().Format(time.RFC3339)
	}
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO rate_snapshots (hotel_id, check_in, check_out, room_type_code, room_type_name, rate_plan_code, rate_plan_name, nightly_rate, currency, captured_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		snapshot.HotelID, snapshot.CheckIn, snapshot.CheckOut, snapshot.RoomTypeCode, snapshot.RoomTypeName, snapshot.RatePlanCode, snapshot.RatePlanName, snapshot.NightlyRate, snapshot.Currency, capturedAt,
	)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err == nil {
		snapshot.ID = id
		snapshot.CapturedAt = capturedAt
	}
	return nil
}

func (s *Store) QueryRateSnapshots(ctx context.Context, hotelID string) ([]RateSnapshot, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, hotel_id, check_in, check_out, room_type_code, room_type_name, rate_plan_code, rate_plan_name, nightly_rate, currency, captured_at
		 FROM rate_snapshots
		 WHERE hotel_id = ?
		 ORDER BY captured_at ASC, id ASC`,
		hotelID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []RateSnapshot
	for rows.Next() {
		var sn RateSnapshot
		var rtc, rtn, rpc, rpn sql.NullString
		err := rows.Scan(
			&sn.ID, &sn.HotelID, &sn.CheckIn, &sn.CheckOut,
			&rtc, &rtn, &rpc, &rpn,
			&sn.NightlyRate, &sn.Currency, &sn.CapturedAt,
		)
		if err != nil {
			return nil, err
		}
		sn.RoomTypeCode = rtc.String
		sn.RoomTypeName = rtn.String
		sn.RatePlanCode = rpc.String
		sn.RatePlanName = rpn.String
		list = append(list, sn)
	}
	return list, rows.Err()
}

type CodeCheck struct {
	ID           int64  `json:"id,omitempty"`
	HotelID      string `json:"hotel_id"`
	CodeType     string `json:"code_type"`
	Code         string `json:"code"`
	Valid        int    `json:"valid"` // 1 = valid, 0 = invalid
	ErrorCode    string `json:"error_code,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
	CheckedAt    string `json:"checked_at"`
}

func (s *Store) InsertCodeCheck(ctx context.Context, check *CodeCheck) error {
	s.lockForWrite()
	defer s.unlockAfterWrite()
	checkedAt := check.CheckedAt
	if checkedAt == "" {
		checkedAt = time.Now().UTC().Format(time.RFC3339)
	}
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO code_checks (hotel_id, code_type, code, valid, error_code, error_message, checked_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		check.HotelID, check.CodeType, check.Code, check.Valid, check.ErrorCode, check.ErrorMessage, checkedAt,
	)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err == nil {
		check.ID = id
		check.CheckedAt = checkedAt
	}
	return nil
}

func (s *Store) ListLatestCodeChecks(ctx context.Context, code string) ([]CodeCheck, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT c1.id, c1.hotel_id, c1.code_type, c1.code, c1.valid, c1.error_code, c1.error_message, c1.checked_at
		 FROM code_checks c1
		 INNER JOIN (
		     SELECT hotel_id, MAX(checked_at) as max_checked_at
		     FROM code_checks
		     WHERE code = ?
		     GROUP BY hotel_id
		 ) c2 ON c1.hotel_id = c2.hotel_id AND c1.checked_at = c2.max_checked_at
		 WHERE c1.code = ?
		 ORDER BY c1.hotel_id ASC`,
		code, code,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []CodeCheck
	for rows.Next() {
		var cc CodeCheck
		var ec, em sql.NullString
		err := rows.Scan(
			&cc.ID, &cc.HotelID, &cc.CodeType, &cc.Code, &cc.Valid,
			&ec, &em, &cc.CheckedAt,
		)
		if err != nil {
			return nil, err
		}
		cc.ErrorCode = ec.String
		cc.ErrorMessage = em.String
		list = append(list, cc)
	}
	return list, rows.Err()
}
