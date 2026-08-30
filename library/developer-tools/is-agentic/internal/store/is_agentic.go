// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0.

package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

type AgenticSnapshot struct {
	ID        int64    `json:"id"`
	Target    string   `json:"target"`
	FetchedAt string   `json:"fetched_at"`
	ScannedAt string   `json:"scanned_at,omitempty"`
	ReportURL string   `json:"report_url,omitempty"`
	Score     *float64 `json:"score,omitempty"`
	Raw       []byte   `json:"-"`
}

func (s *Store) SaveAgenticSnapshot(ctx context.Context, raw []byte, fetchedAt time.Time) (AgenticSnapshot, error) {
	var report struct {
		Target    string   `json:"target"`
		ScannedAt string   `json:"scanned_at"`
		ReportURL string   `json:"report_url"`
		Score     *float64 `json:"score"`
	}
	if err := json.Unmarshal(raw, &report); err != nil {
		return AgenticSnapshot{}, fmt.Errorf("decode report for local store: %w", err)
	}
	if report.Target == "" {
		return AgenticSnapshot{}, fmt.Errorf("report target is empty")
	}
	fetched := fetchedAt.UTC().Format(time.RFC3339Nano)
	res, err := s.DB().ExecContext(ctx, `INSERT INTO agentic_snapshots(target, fetched_at, scanned_at, report_url, score, report_json) VALUES (?, ?, ?, ?, ?, ?)`, report.Target, fetched, report.ScannedAt, report.ReportURL, report.Score, raw)
	if err != nil {
		return AgenticSnapshot{}, fmt.Errorf("save report snapshot: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return AgenticSnapshot{}, err
	}
	return AgenticSnapshot{ID: id, Target: report.Target, FetchedAt: fetched, ScannedAt: report.ScannedAt, ReportURL: report.ReportURL, Score: report.Score, Raw: append([]byte(nil), raw...)}, nil
}

func (s *Store) ListAgenticSnapshots(ctx context.Context, target string, limit int) ([]AgenticSnapshot, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	query := `SELECT id, target, fetched_at, COALESCE(scanned_at,''), COALESCE(report_url,''), score, report_json FROM agentic_snapshots`
	args := []any{}
	if target != "" {
		query += ` WHERE target = ?`
		args = append(args, target)
	}
	query += ` ORDER BY fetched_at DESC, id DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.DB().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]AgenticSnapshot, 0)
	for rows.Next() {
		var item AgenticSnapshot
		var score sql.NullFloat64
		if err := rows.Scan(&item.ID, &item.Target, &item.FetchedAt, &item.ScannedAt, &item.ReportURL, &score, &item.Raw); err != nil {
			return nil, err
		}
		if score.Valid {
			item.Score = &score.Float64
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) AgenticSnapshotByID(ctx context.Context, id int64) (AgenticSnapshot, error) {
	var item AgenticSnapshot
	var score sql.NullFloat64
	err := s.DB().QueryRowContext(ctx, `SELECT id, target, fetched_at, COALESCE(scanned_at,''), COALESCE(report_url,''), score, report_json FROM agentic_snapshots WHERE id = ?`, id).Scan(&item.ID, &item.Target, &item.FetchedAt, &item.ScannedAt, &item.ReportURL, &score, &item.Raw)
	if score.Valid {
		item.Score = &score.Float64
	}
	if err != nil {
		return AgenticSnapshot{}, err
	}
	return item, nil
}

// SearchAgentic searches retained report JSON without requiring a remote search endpoint.
// It complements the generic resources FTS index for this API's local snapshot ledger.
func (s *Store) SearchAgentic(ctx context.Context, query string, limit int) ([]json.RawMessage, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	rows, err := s.DB().QueryContext(ctx, `SELECT report_json FROM agentic_snapshots WHERE instr(lower(report_json), lower(?)) > 0 ORDER BY fetched_at DESC, id DESC LIMIT ?`, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	results := make([]json.RawMessage, 0)
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		results = append(results, json.RawMessage(append([]byte(nil), raw...)))
	}
	return results, rows.Err()
}

func (s *Store) AgenticTargets(ctx context.Context) ([]string, error) {
	rows, err := s.DB().QueryContext(ctx, `SELECT DISTINCT target FROM agentic_snapshots ORDER BY target`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]string, 0)
	for rows.Next() {
		var target string
		if err := rows.Scan(&target); err != nil {
			return nil, err
		}
		items = append(items, target)
	}
	return items, rows.Err()
}

func IsMissingAgenticStore(err error) bool { return err == sql.ErrNoRows }
