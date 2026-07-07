// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.
// Local SQLite store for offline search, rollups, and dupe detection.

package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Store wraps a SQLite database with helpers for Expensify entities.
type Store struct {
	DB   *sql.DB
	Path string
}

// Expense is a local row representing an Expensify transaction/expense.
type Expense struct {
	TransactionID string `json:"transaction_id"`
	ReportID      string `json:"report_id"`
	Merchant      string `json:"merchant"`
	Amount        int64  `json:"amount"` // cents
	Currency      string `json:"currency"`
	Category      string `json:"category"`
	Tag           string `json:"tag"`
	Date          string `json:"date"`
	Comment       string `json:"comment"`
	Receipt       string `json:"receipt"`
	PolicyID      string `json:"policy_id"`
	Created       string `json:"created"`
	Billable      bool   `json:"billable"`
	Reimbursable  bool   `json:"reimbursable"`
	RawJSON       string `json:"raw_json,omitempty"`
}

// Report is a local row representing an Expensify report.
type Report struct {
	ReportID     string `json:"report_id"`
	PolicyID     string `json:"policy_id"`
	Title        string `json:"title"`
	Status       string `json:"status"`
	Total        int64  `json:"total"`
	Currency     string `json:"currency"`
	Created      string `json:"created"`
	LastUpdated  string `json:"last_updated"`
	ExpenseCount int    `json:"expense_count"`
	RawJSON      string `json:"raw_json,omitempty"`
	SyncedAt     string `json:"synced_at,omitempty"`
}

// Workspace represents an Expensify policy/workspace.
type Workspace struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	Role       string `json:"role"`
	OwnerEmail string `json:"owner_email"`
	RawJSON    string `json:"raw_json,omitempty"`
	SyncedAt   string `json:"synced_at,omitempty"`
}

// Category row.
type Category struct {
	PolicyID string `json:"policy_id"`
	Name     string `json:"name"`
	Enabled  bool   `json:"enabled"`
	GLCode   string `json:"gl_code"`
}

// Tag row.
type Tag struct {
	PolicyID string `json:"policy_id"`
	Name     string `json:"name"`
	Enabled  bool   `json:"enabled"`
	Level    int    `json:"level"`
}

// DefaultPath returns the default store location under ~/.cache.
func DefaultPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cache", "expensify-pp-cli", "store.sqlite")
}

// Open opens the SQLite database, creating parent directories as needed.
// If path is empty, DefaultPath() is used.
func Open(path string) (*Store, error) {
	if path == "" {
		path = DefaultPath()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("creating store dir: %w", err)
	}
	db, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(15000)")
	if err != nil {
		return nil, fmt.Errorf("opening sqlite: %w", err)
	}
	// Serialize writes through a single connection so concurrent commands in the
	// same process don't deadlock on the DDL exclusive lock; cross-process
	// contention is absorbed by the 15s busy_timeout above.
	db.SetMaxOpenConns(1)
	s := &Store{DB: db, Path: path}
	if err := s.Migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// Close closes the underlying database.
func (s *Store) Close() error {
	if s == nil || s.DB == nil {
		return nil
	}
	return s.DB.Close()
}

// Migrate creates the schema if it does not exist.
func (s *Store) Migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS expenses (
			transaction_id TEXT PRIMARY KEY,
			report_id      TEXT,
			merchant       TEXT,
			amount         INTEGER,
			currency       TEXT,
			category       TEXT,
			tag            TEXT,
			date           TEXT,
			comment        TEXT,
			receipt        TEXT,
			policy_id      TEXT,
			created        TEXT,
			billable       INTEGER,
			reimbursable   INTEGER,
			raw_json       TEXT,
			synced_at      TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_expenses_date ON expenses(date)`,
		`CREATE INDEX IF NOT EXISTS idx_expenses_policy ON expenses(policy_id)`,
		`CREATE INDEX IF NOT EXISTS idx_expenses_report ON expenses(report_id)`,
		`CREATE TABLE IF NOT EXISTS reports (
			report_id      TEXT PRIMARY KEY,
			policy_id      TEXT,
			title          TEXT,
			status         TEXT,
			total          INTEGER,
			currency       TEXT,
			created        TEXT,
			last_updated   TEXT,
			expense_count  INTEGER,
			raw_json       TEXT,
			synced_at      TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS workspaces (
			id          TEXT PRIMARY KEY,
			name        TEXT,
			type        TEXT,
			role        TEXT,
			owner_email TEXT,
			raw_json    TEXT,
			synced_at   TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS categories (
			policy_id TEXT,
			name      TEXT,
			enabled   INTEGER,
			gl_code   TEXT,
			PRIMARY KEY (policy_id, name)
		)`,
		`CREATE TABLE IF NOT EXISTS tags (
			policy_id TEXT,
			name      TEXT,
			enabled   INTEGER,
			level     INTEGER,
			PRIMARY KEY (policy_id, name)
		)`,
		`CREATE TABLE IF NOT EXISTS action_log (
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			ts           TEXT,
			command      TEXT,
			target_id    TEXT,
			before_json  TEXT,
			after_json   TEXT
		)`,
		// FTS5 virtual table. Best-effort: if FTS5 is not compiled in, we
		// swallow the error and fall back to LIKE-based search.
		`CREATE VIRTUAL TABLE IF NOT EXISTS expenses_fts USING fts5(
			transaction_id UNINDEXED,
			merchant,
			comment,
			category,
			tag
		)`,
	}
	for _, q := range stmts {
		if _, err := s.DB.Exec(q); err != nil {
			// FTS5 may not be available on some builds — log but continue.
			if strings.Contains(q, "expenses_fts") {
				continue
			}
			return fmt.Errorf("migrate: %w (stmt: %.80s)", err, q)
		}
	}
	return nil
}

// hasFTS reports whether the expenses_fts virtual table exists.
func (s *Store) hasFTS() bool {
	var n int
	row := s.DB.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='expenses_fts'`)
	if err := row.Scan(&n); err != nil {
		return false
	}
	return n > 0
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// UpsertExpense inserts or updates a single expense.
func (s *Store) UpsertExpense(e Expense) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.DB.Exec(`
		INSERT INTO expenses (
			transaction_id, report_id, merchant, amount, currency,
			category, tag, date, comment, receipt, policy_id, created,
			billable, reimbursable, raw_json, synced_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(transaction_id) DO UPDATE SET
			report_id=excluded.report_id,
			merchant=excluded.merchant,
			amount=excluded.amount,
			currency=excluded.currency,
			category=excluded.category,
			tag=excluded.tag,
			date=excluded.date,
			comment=excluded.comment,
			receipt=excluded.receipt,
			policy_id=excluded.policy_id,
			created=excluded.created,
			billable=excluded.billable,
			reimbursable=excluded.reimbursable,
			raw_json=excluded.raw_json,
			synced_at=excluded.synced_at
	`, e.TransactionID, e.ReportID, e.Merchant, e.Amount, e.Currency,
		e.Category, e.Tag, e.Date, e.Comment, e.Receipt, e.PolicyID, e.Created,
		boolToInt(e.Billable), boolToInt(e.Reimbursable), e.RawJSON, now)
	if err != nil {
		return err
	}
	if s.hasFTS() {
		// Keep FTS table in sync — cheapest is delete-then-insert.
		if _, err := s.DB.Exec(`DELETE FROM expenses_fts WHERE transaction_id = ?`, e.TransactionID); err != nil {
			return err
		}
		if _, err := s.DB.Exec(`INSERT INTO expenses_fts (transaction_id, merchant, comment, category, tag) VALUES (?, ?, ?, ?, ?)`,
			e.TransactionID, e.Merchant, e.Comment, e.Category, e.Tag); err != nil {
			return err
		}
	}
	return nil
}

// UpsertReport inserts or updates a single report.
func (s *Store) UpsertReport(r Report) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.DB.Exec(`
		INSERT INTO reports (
			report_id, policy_id, title, status, total, currency,
			created, last_updated, expense_count, raw_json, synced_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(report_id) DO UPDATE SET
			policy_id=excluded.policy_id,
			title=excluded.title,
			status=excluded.status,
			total=excluded.total,
			currency=excluded.currency,
			created=excluded.created,
			last_updated=excluded.last_updated,
			expense_count=excluded.expense_count,
			raw_json=excluded.raw_json,
			synced_at=excluded.synced_at
	`, r.ReportID, r.PolicyID, r.Title, r.Status, r.Total, r.Currency,
		r.Created, r.LastUpdated, r.ExpenseCount, r.RawJSON, now)
	return err
}

// UpsertWorkspace inserts or updates a workspace.
func (s *Store) UpsertWorkspace(w Workspace) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.DB.Exec(`
		INSERT INTO workspaces (id, name, type, role, owner_email, raw_json, synced_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name=excluded.name,
			type=excluded.type,
			role=excluded.role,
			owner_email=excluded.owner_email,
			raw_json=excluded.raw_json,
			synced_at=excluded.synced_at
	`, w.ID, w.Name, w.Type, w.Role, w.OwnerEmail, w.RawJSON, now)
	return err
}

// UpsertCategory inserts or updates a category.
func (s *Store) UpsertCategory(c Category) error {
	_, err := s.DB.Exec(`
		INSERT INTO categories (policy_id, name, enabled, gl_code)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(policy_id, name) DO UPDATE SET
			enabled=excluded.enabled,
			gl_code=excluded.gl_code
	`, c.PolicyID, c.Name, boolToInt(c.Enabled), c.GLCode)
	return err
}

// UpsertTag inserts or updates a tag.
func (s *Store) UpsertTag(t Tag) error {
	_, err := s.DB.Exec(`
		INSERT INTO tags (policy_id, name, enabled, level)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(policy_id, name) DO UPDATE SET
			enabled=excluded.enabled,
			level=excluded.level
	`, t.PolicyID, t.Name, boolToInt(t.Enabled), t.Level)
	return err
}

// GetExpense returns a single expense by transaction id.
func (s *Store) GetExpense(id string) (*Expense, error) {
	row := s.DB.QueryRow(`SELECT transaction_id, report_id, merchant, amount, currency,
		category, tag, date, comment, receipt, policy_id, created, billable, reimbursable, raw_json
		FROM expenses WHERE transaction_id = ?`, id)
	e, err := scanExpense(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return e, nil
}

// DeleteExpense removes an expense row.
func (s *Store) DeleteExpense(id string) error {
	_, err := s.DB.Exec(`DELETE FROM expenses WHERE transaction_id = ?`, id)
	if err != nil {
		return err
	}
	if s.hasFTS() {
		_, _ = s.DB.Exec(`DELETE FROM expenses_fts WHERE transaction_id = ?`, id)
	}
	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanExpense(r rowScanner) (*Expense, error) {
	var e Expense
	var billable, reimbursable int
	if err := r.Scan(
		&e.TransactionID, &e.ReportID, &e.Merchant, &e.Amount, &e.Currency,
		&e.Category, &e.Tag, &e.Date, &e.Comment, &e.Receipt, &e.PolicyID,
		&e.Created, &billable, &reimbursable, &e.RawJSON,
	); err != nil {
		return nil, err
	}
	e.Billable = billable != 0
	e.Reimbursable = reimbursable != 0
	return &e, nil
}

// ListExpenses returns expenses matching the given filter map. Keys honored:
// policy_id, since (inclusive), until (inclusive), has_receipt ("true"/"false"),
// limit (string int).
func (s *Store) ListExpenses(filters map[string]string) ([]Expense, error) {
	where, args := buildExpenseFilter(filters)
	q := `SELECT transaction_id, report_id, merchant, amount, currency,
		category, tag, date, comment, receipt, policy_id, created, billable, reimbursable, raw_json
		FROM expenses`
	if where != "" {
		q += " WHERE " + where
	}
	q += " ORDER BY date DESC, transaction_id DESC"
	if lim := filters["limit"]; lim != "" {
		q += " LIMIT ?"
		args = append(args, lim)
	}
	return s.queryExpenses(q, args...)
}

func (s *Store) queryExpenses(q string, args ...any) ([]Expense, error) {
	rows, err := s.DB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Expense
	for rows.Next() {
		e, err := scanExpense(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *e)
	}
	return out, rows.Err()
}

func buildExpenseFilter(f map[string]string) (string, []any) {
	parts := []string{}
	args := []any{}
	if v := f["policy_id"]; v != "" {
		parts = append(parts, "policy_id = ?")
		args = append(args, v)
	}
	if v := f["since"]; v != "" {
		parts = append(parts, "date >= ?")
		args = append(args, v)
	}
	if v := f["until"]; v != "" {
		parts = append(parts, "date <= ?")
		args = append(args, v)
	}
	if v := f["merchant"]; v != "" {
		parts = append(parts, "merchant LIKE ?")
		args = append(args, "%"+v+"%")
	}
	switch f["has_receipt"] {
	case "true":
		parts = append(parts, "receipt IS NOT NULL AND receipt <> ''")
	case "false":
		parts = append(parts, "(receipt IS NULL OR receipt = '')")
	}
	return strings.Join(parts, " AND "), args
}

// SearchExpenses runs an FTS5 MATCH when a query is provided, and falls back
// to a LIKE over merchant/comment/category/tag when FTS5 is unavailable.
func (s *Store) SearchExpenses(query string, filters map[string]string) ([]Expense, error) {
	filtWhere, filtArgs := buildExpenseFilter(filters)
	limit := ""
	var limitArgs []any
	if lim := filters["limit"]; lim != "" {
		limit = " LIMIT ?"
		limitArgs = append(limitArgs, lim)
	}
	if strings.TrimSpace(query) == "" {
		return s.ListExpenses(filters)
	}
	if s.hasFTS() {
		q := `SELECT e.transaction_id, e.report_id, e.merchant, e.amount, e.currency,
				e.category, e.tag, e.date, e.comment, e.receipt, e.policy_id, e.created,
				e.billable, e.reimbursable, e.raw_json
			FROM expenses e
			JOIN expenses_fts f ON f.transaction_id = e.transaction_id
			WHERE expenses_fts MATCH ?`
		args := []any{query}
		if filtWhere != "" {
			q += " AND " + filtWhere
			args = append(args, filtArgs...)
		}
		q += " ORDER BY e.date DESC" + limit
		args = append(args, limitArgs...)
		return s.queryExpenses(q, args...)
	}
	// Fallback: plain LIKE
	like := "%" + query + "%"
	q := `SELECT transaction_id, report_id, merchant, amount, currency,
			category, tag, date, comment, receipt, policy_id, created,
			billable, reimbursable, raw_json
		FROM expenses
		WHERE (merchant LIKE ? OR comment LIKE ? OR category LIKE ? OR tag LIKE ?)`
	args := []any{like, like, like, like}
	if filtWhere != "" {
		q += " AND " + filtWhere
		args = append(args, filtArgs...)
	}
	q += " ORDER BY date DESC" + limit
	args = append(args, limitArgs...)
	return s.queryExpenses(q, args...)
}

// ListUnreportedSince returns expenses that have no report_id and were dated
// on or after the given cutoff. If policyID is non-empty it is matched exactly.
func (s *Store) ListUnreportedSince(since time.Time, policyID string) ([]Expense, error) {
	q := `SELECT transaction_id, report_id, merchant, amount, currency,
			category, tag, date, comment, receipt, policy_id, created,
			billable, reimbursable, raw_json
		FROM expenses
		WHERE (report_id IS NULL OR report_id = '')
		  AND date >= ?`
	args := []any{since.Format("2006-01-02")}
	if policyID != "" {
		q += " AND policy_id = ?"
		args = append(args, policyID)
	}
	q += " ORDER BY date ASC"
	return s.queryExpenses(q, args...)
}

// MissingReceipts returns expenses with no attached receipt.
func (s *Store) MissingReceipts(filters map[string]string) ([]Expense, error) {
	merged := map[string]string{}
	for k, v := range filters {
		merged[k] = v
	}
	merged["has_receipt"] = "false"
	return s.ListExpenses(merged)
}

// DupeGroup represents a cluster of expenses that look like duplicates.
type DupeGroup struct {
	Merchant string    `json:"merchant"`
	Amount   int64     `json:"amount"`
	Expenses []Expense `json:"expenses"`
}

// Dupes scans the local store for expenses sharing merchant + amount within a
// date window of `windowDays`.
func (s *Store) Dupes(windowDays int) ([]DupeGroup, error) {
	if windowDays < 0 {
		windowDays = 0
	}
	all, err := s.ListExpenses(nil)
	if err != nil {
		return nil, err
	}
	// Group by merchant + amount
	buckets := map[string][]Expense{}
	for _, e := range all {
		key := strings.ToLower(e.Merchant) + "|" + fmt.Sprintf("%d", e.Amount)
		buckets[key] = append(buckets[key], e)
	}
	var out []DupeGroup
	for _, bucket := range buckets {
		if len(bucket) < 2 {
			continue
		}
		// Detect at least one pair within the window
		var clustered []Expense
		for i := 0; i < len(bucket); i++ {
			for j := i + 1; j < len(bucket); j++ {
				di, ei1 := parseDate(bucket[i].Date)
				dj, ei2 := parseDate(bucket[j].Date)
				if ei1 != nil || ei2 != nil {
					continue
				}
				diff := di.Sub(dj).Hours() / 24
				if diff < 0 {
					diff = -diff
				}
				if int(diff) <= windowDays {
					clustered = addUnique(clustered, bucket[i], bucket[j])
				}
			}
		}
		if len(clustered) >= 2 {
			out = append(out, DupeGroup{
				Merchant: clustered[0].Merchant,
				Amount:   clustered[0].Amount,
				Expenses: clustered,
			})
		}
	}
	return out, nil
}

func addUnique(dst []Expense, items ...Expense) []Expense {
	for _, e := range items {
		present := false
		for _, d := range dst {
			if d.TransactionID == e.TransactionID {
				present = true
				break
			}
		}
		if !present {
			dst = append(dst, e)
		}
	}
	return dst
}

func parseDate(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, fmt.Errorf("empty date")
	}
	for _, layout := range []string{"2006-01-02", "2006-01-02T15:04:05Z", time.RFC3339} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized date: %s", s)
}

// RollupRow is one aggregated bucket in a rollup report.
type RollupRow struct {
	Key   string `json:"key"`
	Count int    `json:"count"`
	Total int64  `json:"total"`
}

// Rollup groups expenses by the given dimension (category|tag|merchant) for
// a month given as YYYY-MM. Optional policyID filters the result.
func (s *Store) Rollup(month, by, policyID string) ([]RollupRow, error) {
	col := "category"
	switch strings.ToLower(by) {
	case "tag":
		col = "tag"
	case "merchant":
		col = "merchant"
	case "category", "":
		col = "category"
	default:
		return nil, fmt.Errorf("unknown rollup dimension: %s", by)
	}
	args := []any{}
	where := []string{}
	if month != "" {
		where = append(where, "substr(date,1,7) = ?")
		args = append(args, month)
	}
	if policyID != "" {
		where = append(where, "policy_id = ?")
		args = append(args, policyID)
	}
	q := "SELECT " + col + ", count(*), coalesce(sum(amount),0) FROM expenses"
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ") // #nosec G202 -- col is a closed allowlist (category|tag|merchant) set by the switch above; the WHERE clause is joined from constant fragments and every user value is bound via ? placeholders in args.
	}
	q += " GROUP BY " + col + " ORDER BY sum(amount) DESC"
	rows, err := s.DB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RollupRow
	for rows.Next() {
		var r RollupRow
		if err := rows.Scan(&r.Key, &r.Count, &r.Total); err != nil {
			return nil, err
		}
		if r.Key == "" {
			r.Key = "(uncategorized)"
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// StatusBreakdown returns counts and totals grouped by report status
// (and missing receipts) for a given month (YYYY-MM).
type StatusBreakdown struct {
	Expensed        int64
	ExpensedCount   int
	PendingApproval int64
	PendingCount    int
	Approved        int64
	ApprovedCount   int
	Paid            int64
	PaidCount       int
	MissingReceipts int
}

// Damage returns an at-a-glance breakdown by report status for a month.
func (s *Store) Damage(month, policyID string) (StatusBreakdown, error) {
	out := StatusBreakdown{}
	// Get expenses for the month
	args := []any{}
	where := []string{}
	if month != "" {
		where = append(where, "substr(e.date,1,7) = ?")
		args = append(args, month)
	}
	if policyID != "" {
		where = append(where, "e.policy_id = ?")
		args = append(args, policyID)
	}
	q := `SELECT coalesce(r.status,''), count(*), coalesce(sum(e.amount),0)
		FROM expenses e
		LEFT JOIN reports r ON r.report_id = e.report_id`
	// #nosec G202 -- WHERE clause is joined from constant fragments; all user values are bound via ? placeholders in args.
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	q += " GROUP BY coalesce(r.status,'')"
	rows, err := s.DB.Query(q, args...)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var st string
		var count int
		var total int64
		if err := rows.Scan(&st, &count, &total); err != nil {
			return out, err
		}
		s := strings.ToUpper(st)
		switch {
		case s == "" || s == "OPEN":
			out.Expensed += total
			out.ExpensedCount += count
		case strings.HasPrefix(s, "SUBMIT") || s == "PROCESSING":
			out.PendingApproval += total
			out.PendingCount += count
		case strings.HasPrefix(s, "APPROVED"):
			out.Approved += total
			out.ApprovedCount += count
		case strings.HasPrefix(s, "REIMBURSED") || strings.HasPrefix(s, "PAID"):
			out.Paid += total
			out.PaidCount += count
		}
	}
	// Missing receipts
	mq := `SELECT count(*) FROM expenses e WHERE (e.receipt IS NULL OR e.receipt = '')`
	mArgs := []any{}
	if month != "" {
		mq += " AND substr(e.date,1,7) = ?"
		mArgs = append(mArgs, month)
	}
	if policyID != "" {
		mq += " AND e.policy_id = ?"
		mArgs = append(mArgs, policyID)
	}
	if err := s.DB.QueryRow(mq, mArgs...).Scan(&out.MissingReceipts); err != nil {
		return out, err
	}
	return out, nil
}

// RecordAction appends a row to action_log for later undo.
func (s *Store) RecordAction(command, targetID string, before, after any) error {
	b, _ := json.Marshal(before)
	a, _ := json.Marshal(after)
	_, err := s.DB.Exec(`INSERT INTO action_log (ts, command, target_id, before_json, after_json)
		VALUES (?, ?, ?, ?, ?)`,
		time.Now().UTC().Format(time.RFC3339), command, targetID, string(b), string(a))
	return err
}

// LastCategoryForMerchant returns the most recent non-empty category used on
// an expense whose merchant matches (case-insensitive substring).
func (s *Store) LastCategoryForMerchant(merchant string) (string, error) {
	row := s.DB.QueryRow(`SELECT category FROM expenses
		WHERE lower(merchant) LIKE ? AND category <> ''
		ORDER BY date DESC LIMIT 1`, "%"+strings.ToLower(merchant)+"%")
	var out string
	err := row.Scan(&out)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return out, err
}

// ListReports returns all reports in the local store, optionally filtered.
// Supported filter keys: policy_id, status.
func (s *Store) ListReports(filters map[string]string) ([]Report, error) {
	query := `SELECT report_id, policy_id, title, status, total, currency, created, last_updated, expense_count, raw_json, synced_at FROM reports`
	var where []string
	var args []any
	if v, ok := filters["policy_id"]; ok && v != "" {
		where = append(where, "policy_id = ?")
		args = append(args, v)
	}
	if v, ok := filters["status"]; ok && v != "" {
		where = append(where, "status = ?")
		args = append(args, v)
	}
	// #nosec G202 -- WHERE clause is joined from constant fragments; all user values are bound via ? placeholders in args.
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY last_updated DESC"
	rows, err := s.DB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Report
	for rows.Next() {
		var r Report
		if err := rows.Scan(&r.ReportID, &r.PolicyID, &r.Title, &r.Status, &r.Total, &r.Currency, &r.Created, &r.LastUpdated, &r.ExpenseCount, &r.RawJSON, &r.SyncedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ListWorkspaces returns all workspaces in the local store.
func (s *Store) ListWorkspaces() ([]Workspace, error) {
	rows, err := s.DB.Query(`SELECT id, name, type, role, owner_email, raw_json, synced_at FROM workspaces ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Workspace
	for rows.Next() {
		var w Workspace
		if err := rows.Scan(&w.ID, &w.Name, &w.Type, &w.Role, &w.OwnerEmail, &w.RawJSON, &w.SyncedAt); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}
