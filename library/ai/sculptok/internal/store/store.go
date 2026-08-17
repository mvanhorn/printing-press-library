// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored local SQLite store. The generator does not emit a store for
// SculptOK because its api-open surface is list-only/async-job-shaped (no
// get-by-id mirror model), so the "compounding local state" features
// (generate job persistence, sync, search, analytics, reconcile) build on this
// instead. Pure-Go driver (modernc.org/sqlite) keeps the binary cgo-free.

package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

// Store wraps a SQLite database of SculptOK jobs, credit events, and drawings.
type Store struct {
	db *sql.DB
}

// DB exposes the underlying handle for read-only ad-hoc queries.
func (s *Store) DB() *sql.DB { return s.db }

// DefaultPath returns the default on-disk location for the local mirror.
func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "sculptok.db"
	}
	return filepath.Join(home, ".config", "sculptok-pp-cli", "sculptok.db")
}

// Open opens (creating if needed) the store at path and runs migrations.
func Open(ctx context.Context, path string) (*Store, error) {
	if path == "" {
		path = DefaultPath()
	}
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, fmt.Errorf("creating store dir: %w", err)
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("opening store: %w", err)
	}
	s := &Store{db: db}
	if err := s.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// OpenReadOnly opens an existing store without creating tables. Returns
// (nil, false, nil) when the file does not exist yet.
func OpenReadOnly(ctx context.Context, path string) (*Store, bool, error) {
	if path == "" {
		path = DefaultPath()
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, false, nil
	}
	s, err := Open(ctx, path)
	if err != nil {
		return nil, false, err
	}
	return s, true, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS jobs (
			prompt_id   TEXT PRIMARY KEY,
			kind        TEXT NOT NULL DEFAULT '',
			status      TEXT NOT NULL DEFAULT '',
			image_url   TEXT NOT NULL DEFAULT '',
			params      TEXT NOT NULL DEFAULT '{}',
			result_urls TEXT NOT NULL DEFAULT '[]',
			credit_cost INTEGER NOT NULL DEFAULT 0,
			created_at  TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS credit_events (
			id           TEXT PRIMARY KEY,
			action_type  INTEGER NOT NULL DEFAULT 0,
			remain_value INTEGER NOT NULL DEFAULT 0,
			change_num   INTEGER NOT NULL DEFAULT 0,
			remarks      TEXT NOT NULL DEFAULT '',
			create_date  TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS drawings (
			id          TEXT PRIMARY KEY,
			img_url     TEXT NOT NULL DEFAULT '',
			create_date TEXT NOT NULL DEFAULT ''
		)`,
	}
	for _, q := range stmts {
		if _, err := s.db.ExecContext(ctx, q); err != nil {
			return fmt.Errorf("migrating store: %w", err)
		}
	}
	return nil
}

// Job is a locally recorded draw.
type Job struct {
	PromptID   string `json:"promptId"`
	Kind       string `json:"kind"`
	Status     string `json:"status"`
	ImageURL   string `json:"imageUrl"`
	Params     string `json:"params"`
	ResultURLs string `json:"resultUrls"`
	CreditCost int    `json:"creditCost"`
	CreatedAt  string `json:"createdAt"`
}

// UpsertJob inserts or updates a job by prompt_id.
func (s *Store) UpsertJob(ctx context.Context, j Job) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO jobs (prompt_id, kind, status, image_url, params, result_urls, credit_cost, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(prompt_id) DO UPDATE SET
			kind=excluded.kind, status=excluded.status, image_url=excluded.image_url,
			params=excluded.params, result_urls=excluded.result_urls,
			credit_cost=excluded.credit_cost`,
		j.PromptID, j.Kind, j.Status, j.ImageURL, nz(j.Params, "{}"), nz(j.ResultURLs, "[]"), j.CreditCost, j.CreatedAt)
	return err
}

func nz(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}

func scanJobs(rows *sql.Rows) ([]Job, error) {
	defer rows.Close()
	out := make([]Job, 0)
	for rows.Next() {
		var j Job
		var kind, status, img, params, results, created sql.NullString
		var cost sql.NullInt64
		if err := rows.Scan(&j.PromptID, &kind, &status, &img, &params, &results, &cost, &created); err != nil {
			continue
		}
		j.Kind, j.Status, j.ImageURL = kind.String, status.String, img.String
		j.Params, j.ResultURLs, j.CreatedAt = params.String, results.String, created.String
		j.CreditCost = int(cost.Int64)
		out = append(out, j)
	}
	return out, rows.Err()
}

// ListJobs returns the most recent jobs.
func (s *Store) ListJobs(ctx context.Context, limit int) ([]Job, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx, `SELECT prompt_id, kind, status, image_url, params, result_urls, credit_cost, created_at FROM jobs ORDER BY created_at DESC, rowid DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	return scanJobs(rows)
}

// likeEscaper escapes LIKE metacharacters so a search term is matched
// literally rather than as a wildcard pattern.
var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// SearchJobs does a simple LIKE search across kind/status/params/prompt_id.
func (s *Store) SearchJobs(ctx context.Context, term string, limit int) ([]Job, error) {
	if limit <= 0 {
		limit = 20
	}
	// Escape % and _ so they match literally; ESCAPE '\' tells SQLite how.
	like := "%" + likeEscaper.Replace(term) + "%"
	rows, err := s.db.QueryContext(ctx, `
		SELECT prompt_id, kind, status, image_url, params, result_urls, credit_cost, created_at FROM jobs
		WHERE kind LIKE ? ESCAPE '\' OR status LIKE ? ESCAPE '\' OR params LIKE ? ESCAPE '\' OR prompt_id LIKE ? ESCAPE '\' OR image_url LIKE ? ESCAPE '\'
		ORDER BY created_at DESC, rowid DESC LIMIT ?`, like, like, like, like, like, limit)
	if err != nil {
		return nil, err
	}
	return scanJobs(rows)
}

// CountJobs returns the number of stored jobs.
func (s *Store) CountJobs(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM jobs`).Scan(&n)
	return n, err
}

// CreditEvent mirrors a row from /point/page.
type CreditEvent struct {
	ID          string `json:"id"`
	ActionType  int    `json:"actionType"`
	RemainValue int    `json:"remainValue"`
	ChangeNum   int    `json:"changeNum"`
	Remarks     string `json:"remarks"`
	CreateDate  string `json:"createDate"`
}

// UpsertCreditEvent inserts or updates a credit event by id.
func (s *Store) UpsertCreditEvent(ctx context.Context, e CreditEvent) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO credit_events (id, action_type, remain_value, change_num, remarks, create_date)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			action_type=excluded.action_type, remain_value=excluded.remain_value,
			change_num=excluded.change_num, remarks=excluded.remarks, create_date=excluded.create_date`,
		e.ID, e.ActionType, e.RemainValue, e.ChangeNum, e.Remarks, e.CreateDate)
	return err
}

// ListCreditEvents returns the most recent credit events.
func (s *Store) ListCreditEvents(ctx context.Context, limit int) ([]CreditEvent, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, action_type, remain_value, change_num, remarks, create_date FROM credit_events ORDER BY create_date DESC, rowid DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	return scanCreditEvents(rows)
}

// SearchCreditEvents does a LIKE search over credit-event remarks, applying
// limit as a SQL-level result cap (like SearchJobs) so older matching events
// are not silently dropped — unlike a fetch-newest-N-then-filter approach,
// where the limit would act as a search window rather than a result cap. With
// an empty term it returns the most recent events.
func (s *Store) SearchCreditEvents(ctx context.Context, term string, limit int) ([]CreditEvent, error) {
	if limit <= 0 {
		limit = 50
	}
	if strings.TrimSpace(term) == "" {
		return s.ListCreditEvents(ctx, limit)
	}
	// Escape % and _ so they match literally; ESCAPE '\' tells SQLite how.
	like := "%" + likeEscaper.Replace(term) + "%"
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, action_type, remain_value, change_num, remarks, create_date FROM credit_events
		WHERE remarks LIKE ? ESCAPE '\'
		ORDER BY create_date DESC, rowid DESC LIMIT ?`, like, limit)
	if err != nil {
		return nil, err
	}
	return scanCreditEvents(rows)
}

func scanCreditEvents(rows *sql.Rows) ([]CreditEvent, error) {
	defer rows.Close()
	out := make([]CreditEvent, 0)
	for rows.Next() {
		var e CreditEvent
		var remarks, created sql.NullString
		var at, rv, cn sql.NullInt64
		if err := rows.Scan(&e.ID, &at, &rv, &cn, &remarks, &created); err != nil {
			continue
		}
		e.ActionType, e.RemainValue, e.ChangeNum = int(at.Int64), int(rv.Int64), int(cn.Int64)
		e.Remarks, e.CreateDate = remarks.String, created.String
		out = append(out, e)
	}
	return out, rows.Err()
}

// GroupCount is one row of an analytics aggregation.
type GroupCount struct {
	Group       string `json:"group"`
	Count       int    `json:"count"`
	TotalChange int    `json:"totalChange"`
}

// AnalyticsCreditEvents groups credit events by a supported field and sums the
// (negative) change_num so spend is visible. by: actionType | remarks | day.
func (s *Store) AnalyticsCreditEvents(ctx context.Context, by string, limit int) ([]GroupCount, error) {
	if limit <= 0 {
		limit = 50
	}
	var expr string
	switch by {
	case "", "actionType", "action_type":
		expr = "CAST(action_type AS TEXT)"
	case "remarks":
		expr = "remarks"
	case "day", "date", "createDate", "create_date":
		expr = "substr(create_date, 1, 10)"
	default:
		return nil, fmt.Errorf("unsupported --group-by %q (use actionType, remarks, or day)", by)
	}
	// #nosec G201 -- expr is not user input: it is one of three constant
	// SQL fragments chosen by the switch above; any other --group-by value
	// returns an error before reaching here. The LIMIT value is parameterized.
	q := fmt.Sprintf(`SELECT %s AS grp, COUNT(*), COALESCE(SUM(change_num),0) FROM credit_events GROUP BY grp ORDER BY COUNT(*) DESC LIMIT ?`, expr)
	rows, err := s.db.QueryContext(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]GroupCount, 0)
	for rows.Next() {
		var g GroupCount
		var grp sql.NullString
		var cnt, total sql.NullInt64
		if err := rows.Scan(&grp, &cnt, &total); err != nil {
			continue
		}
		g.Group, g.Count, g.TotalChange = grp.String, int(cnt.Int64), int(total.Int64)
		out = append(out, g)
	}
	return out, rows.Err()
}

// Drawing mirrors a row from /image/page.
type Drawing struct {
	ID         string `json:"id"`
	ImgURL     string `json:"imgUrl"`
	CreateDate string `json:"createDate"`
}

// UpsertDrawing inserts or updates a drawing by id.
func (s *Store) UpsertDrawing(ctx context.Context, d Drawing) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO drawings (id, img_url, create_date) VALUES (?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET img_url=excluded.img_url, create_date=excluded.create_date`,
		d.ID, d.ImgURL, d.CreateDate)
	return err
}

// ListDrawings returns the most recent drawings.
func (s *Store) ListDrawings(ctx context.Context, limit int) ([]Drawing, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, img_url, create_date FROM drawings ORDER BY create_date DESC, rowid DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	return scanDrawings(rows)
}

// SearchDrawings does a LIKE search over drawing id/img_url, applying limit as
// a SQL-level result cap (like SearchCreditEvents) so older matching rows are
// not dropped by a newest-N window. With an empty term it lists recent rows.
func (s *Store) SearchDrawings(ctx context.Context, term string, limit int) ([]Drawing, error) {
	if limit <= 0 {
		limit = 50
	}
	if strings.TrimSpace(term) == "" {
		return s.ListDrawings(ctx, limit)
	}
	// Escape % and _ so they match literally; ESCAPE '\' tells SQLite how.
	like := "%" + likeEscaper.Replace(term) + "%"
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, img_url, create_date FROM drawings
		WHERE id LIKE ? ESCAPE '\' OR img_url LIKE ? ESCAPE '\'
		ORDER BY create_date DESC, rowid DESC LIMIT ?`, like, like, limit)
	if err != nil {
		return nil, err
	}
	return scanDrawings(rows)
}

func scanDrawings(rows *sql.Rows) ([]Drawing, error) {
	defer rows.Close()
	out := make([]Drawing, 0)
	for rows.Next() {
		var d Drawing
		var img, created sql.NullString
		if err := rows.Scan(&d.ID, &img, &created); err != nil {
			continue
		}
		d.ImgURL, d.CreateDate = img.String, created.String
		out = append(out, d)
	}
	return out, rows.Err()
}

// CountDrawings returns the number of stored drawings.
func (s *Store) CountDrawings(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM drawings`).Scan(&n)
	return n, err
}

// ReconcileRow flags a credit charge with no matching local job.
type ReconcileRow struct {
	EventID    string `json:"eventId"`
	ChangeNum  int    `json:"changeNum"`
	Remarks    string `json:"remarks"`
	CreateDate string `json:"createDate"`
	MatchedJob string `json:"matchedJob,omitempty"`
}

// Reconcile finds credit-spend events (change_num < 0) whose remarks reference
// a promptId that has no matching local job row. SculptOK's API Draw remarks
// embed the promptId, so a local join surfaces credits spent outside this CLI.
func (s *Store) Reconcile(ctx context.Context, limit int) ([]ReconcileRow, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, change_num, remarks, create_date FROM credit_events
		WHERE change_num < 0
		ORDER BY create_date DESC, rowid DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type ev struct {
		id, remarks, created string
		change               int
	}
	var events []ev
	for rows.Next() {
		var id string
		var remarks, created sql.NullString
		var change sql.NullInt64
		if err := rows.Scan(&id, &change, &remarks, &created); err != nil {
			continue
		}
		events = append(events, ev{id: id, remarks: remarks.String, created: created.String, change: int(change.Int64)})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Load all known job prompt_ids once (avoids an O(events x jobs) re-query).
	var promptIDs []string
	jobRows, err := s.db.QueryContext(ctx, `SELECT prompt_id FROM jobs`)
	if err != nil {
		return nil, err
	}
	for jobRows.Next() {
		var pid string
		if err := jobRows.Scan(&pid); err != nil {
			continue
		}
		if pid != "" {
			promptIDs = append(promptIDs, pid)
		}
	}
	_ = jobRows.Close()
	if err := jobRows.Err(); err != nil {
		return nil, err
	}

	out := make([]ReconcileRow, 0)
	for _, e := range events {
		matched := false
		for _, pid := range promptIDs {
			if strings.Contains(e.remarks, pid) {
				matched = true
				break
			}
		}
		if !matched {
			out = append(out, ReconcileRow{EventID: e.id, ChangeNum: e.change, Remarks: e.remarks, CreateDate: e.created})
		}
	}
	return out, nil
}
