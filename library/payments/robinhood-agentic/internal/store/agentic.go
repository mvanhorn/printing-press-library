// Copyright 2026 Kevin Magnan and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-written store layer for the transcendence features. NOT generated —
// these tables and accessors back the local-only capabilities the Robinhood
// Agentic MCP cannot provide: an append-only portfolio time series, a write
// journal for agent accountability, a tool-surface change log, and the guard
// policy. Tables are created lazily on first use (CREATE TABLE IF NOT EXISTS)
// so this file needs no edit to the generated migrate() and survives regen.

package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

var agenticSchemaOnce sync.Once

const agenticSchema = `
CREATE TABLE IF NOT EXISTS portfolio_snapshots (
	id             INTEGER PRIMARY KEY AUTOINCREMENT,
	account_number TEXT NOT NULL,
	captured_at    TEXT NOT NULL,
	total_value    TEXT,
	equity_value   TEXT,
	options_value  TEXT,
	cash           TEXT,
	buying_power   TEXT,
	raw            TEXT
);
CREATE INDEX IF NOT EXISTS idx_portfolio_snapshots_acct_time
	ON portfolio_snapshots(account_number, captured_at);

CREATE TABLE IF NOT EXISTS write_journal (
	id                 INTEGER PRIMARY KEY AUTOINCREMENT,
	ts                 TEXT NOT NULL,
	tool               TEXT NOT NULL,
	action             TEXT NOT NULL,
	account            TEXT,
	symbol             TEXT,
	ref_id             TEXT,
	review_fingerprint TEXT,
	outcome            TEXT NOT NULL,
	detail             TEXT
);
CREATE INDEX IF NOT EXISTS idx_write_journal_ts ON write_journal(ts);

CREATE TABLE IF NOT EXISTS tool_surface_snapshots (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	captured_at TEXT NOT NULL,
	tool_count  INTEGER NOT NULL,
	tools       TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS guard_policy (
	key   TEXT PRIMARY KEY,
	value TEXT
);
`

func (s *Store) ensureAgenticSchema() error {
	var err error
	agenticSchemaOnce.Do(func() {
		_, err = s.DB().Exec(agenticSchema)
	})
	return err
}

// isNoSuchTable reports whether an error is SQLite's "no such table". Read
// accessors are opened read-only (they can't CREATE TABLE), so on a fresh
// install where no write has yet created the agentic tables, a SELECT fails
// with this — and an absent table is simply "no rows yet", not an error.
func isNoSuchTable(err error) bool {
	return err != nil && strings.Contains(err.Error(), "no such table")
}

// --- portfolio snapshots ---------------------------------------------------

// PortfolioSnapshot is one captured point in the local portfolio time series.
type PortfolioSnapshot struct {
	AccountNumber string    `json:"account_number"`
	CapturedAt    time.Time `json:"captured_at"`
	TotalValue    string    `json:"total_value"`
	EquityValue   string    `json:"equity_value"`
	OptionsValue  string    `json:"options_value"`
	Cash          string    `json:"cash"`
	BuyingPower   string    `json:"buying_power"`
}

// RecordPortfolioSnapshot appends a portfolio value point. The API has no
// portfolio-history endpoint, so this local series is the only record of past
// value. account is the account number; portfolio is a get_portfolio result
// object (the `data` payload).
func (s *Store) RecordPortfolioSnapshot(account string, portfolio json.RawMessage) error {
	if err := s.ensureAgenticSchema(); err != nil {
		return err
	}
	var p struct {
		TotalValue   string          `json:"total_value"`
		EquityValue  string          `json:"equity_value"`
		OptionsValue string          `json:"options_value"`
		Cash         string          `json:"cash"`
		BuyingPower  json.RawMessage `json:"buying_power"`
	}
	_ = json.Unmarshal(portfolio, &p)
	bp := ""
	if len(p.BuyingPower) > 0 {
		// buying_power may be a nested object {buying_power: "..."} or a scalar.
		var nested struct {
			BuyingPower string `json:"buying_power"`
		}
		if json.Unmarshal(p.BuyingPower, &nested) == nil && nested.BuyingPower != "" {
			bp = nested.BuyingPower
		} else {
			bp = strings.Trim(string(p.BuyingPower), `"`)
		}
	}
	_, err := s.DB().Exec(
		`INSERT INTO portfolio_snapshots
			(account_number, captured_at, total_value, equity_value, options_value, cash, buying_power, raw)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		account, time.Now().UTC().Format(time.RFC3339), p.TotalValue, p.EquityValue, p.OptionsValue, p.Cash, bp, string(portfolio),
	)
	return err
}

// PortfolioSnapshots returns snapshots for an account (all accounts if account
// is empty) captured at or after `since`, oldest first.
func (s *Store) PortfolioSnapshots(account string, since time.Time) ([]PortfolioSnapshot, error) {
	q := `SELECT account_number, captured_at, total_value, equity_value, options_value, cash, buying_power
		  FROM portfolio_snapshots WHERE captured_at >= ?`
	argv := []any{since.UTC().Format(time.RFC3339)}
	if account != "" {
		q += ` AND account_number = ?`
		argv = append(argv, account)
	}
	q += ` ORDER BY captured_at ASC`
	rows, err := s.DB().Query(q, argv...)
	if isNoSuchTable(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PortfolioSnapshot
	for rows.Next() {
		var snap PortfolioSnapshot
		var ts string
		if err := rows.Scan(&snap.AccountNumber, &ts, &snap.TotalValue, &snap.EquityValue, &snap.OptionsValue, &snap.Cash, &snap.BuyingPower); err != nil {
			return nil, err
		}
		snap.CapturedAt, _ = time.Parse(time.RFC3339, ts)
		out = append(out, snap)
	}
	return out, rows.Err()
}

// --- write journal ---------------------------------------------------------

// WriteJournalEntry records one mutating tool call attempt and its outcome.
type WriteJournalEntry struct {
	ID                int64     `json:"id,omitempty"`
	Timestamp         time.Time `json:"timestamp"`
	Tool              string    `json:"tool"`
	Action            string    `json:"action"`
	Account           string    `json:"account,omitempty"`
	Symbol            string    `json:"symbol,omitempty"`
	RefID             string    `json:"ref_id,omitempty"`
	ReviewFingerprint string    `json:"review_fingerprint,omitempty"`
	Outcome           string    `json:"outcome"`
	Detail            string    `json:"detail,omitempty"`
}

// RecordWrite appends a write-journal entry. Journaling failures are the
// caller's to swallow — never let audit bookkeeping block a trade.
func (s *Store) RecordWrite(e WriteJournalEntry) error {
	if err := s.ensureAgenticSchema(); err != nil {
		return err
	}
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now().UTC()
	}
	_, err := s.DB().Exec(
		`INSERT INTO write_journal
			(ts, tool, action, account, symbol, ref_id, review_fingerprint, outcome, detail)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.Timestamp.UTC().Format(time.RFC3339), e.Tool, e.Action, e.Account, e.Symbol, e.RefID, e.ReviewFingerprint, e.Outcome, e.Detail,
	)
	return err
}

// WriteJournalFilter narrows a journal query. Zero-value fields are ignored.
type WriteJournalFilter struct {
	Since      time.Time
	Outcome    string // exact match, e.g. "placed", or prefix "blocked" via OutcomePrefix
	OutcomePfx string // outcome LIKE '<prefix>%'
	Tool       string
}

// WriteJournal returns journal entries matching the filter, newest first.
func (s *Store) WriteJournal(f WriteJournalFilter) ([]WriteJournalEntry, error) {
	q := `SELECT id, ts, tool, action, account, symbol, ref_id, review_fingerprint, outcome, detail
		  FROM write_journal WHERE 1=1`
	var argv []any
	if !f.Since.IsZero() {
		q += ` AND ts >= ?`
		argv = append(argv, f.Since.UTC().Format(time.RFC3339))
	}
	if f.Outcome != "" {
		q += ` AND outcome = ?`
		argv = append(argv, f.Outcome)
	}
	if f.OutcomePfx != "" {
		q += ` AND outcome LIKE ?`
		argv = append(argv, f.OutcomePfx+"%")
	}
	if f.Tool != "" {
		q += ` AND tool = ?`
		argv = append(argv, f.Tool)
	}
	q += ` ORDER BY id DESC`
	rows, err := s.DB().Query(q, argv...)
	if isNoSuchTable(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WriteJournalEntry
	for rows.Next() {
		var e WriteJournalEntry
		var ts string
		if err := rows.Scan(&e.ID, &ts, &e.Tool, &e.Action, &e.Account, &e.Symbol, &e.RefID, &e.ReviewFingerprint, &e.Outcome, &e.Detail); err != nil {
			return nil, err
		}
		e.Timestamp, _ = time.Parse(time.RFC3339, ts)
		out = append(out, e)
	}
	return out, rows.Err()
}

// SumPlacedNotionalSince returns the total notional of orders whose outcome is
// "placed" since t, for the daily-cap guard. Notional is stored in the detail
// column as a bare decimal string prefixed "notional=".
func (s *Store) SumPlacedNotionalSince(t time.Time) (float64, error) {
	entries, err := s.WriteJournal(WriteJournalFilter{Since: t, Outcome: "placed"})
	if err != nil {
		return 0, err
	}
	var total float64
	for _, e := range entries {
		total += parseNotionalDetail(e.Detail)
	}
	return total, nil
}

func parseNotionalDetail(detail string) float64 {
	for _, part := range strings.Split(detail, ";") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "notional=") {
			if f, err := strconv.ParseFloat(strings.TrimPrefix(part, "notional="), 64); err == nil {
				return f
			}
		}
	}
	return 0
}

// --- tool-surface snapshots ------------------------------------------------

// ToolSurfaceSnapshot is one captured MCP tools/list export.
type ToolSurfaceSnapshot struct {
	CapturedAt time.Time       `json:"captured_at"`
	ToolCount  int             `json:"tool_count"`
	Tools      json.RawMessage `json:"tools"`
}

// RecordToolSurface stores a tools/list snapshot (an array of {name, ...}).
func (s *Store) RecordToolSurface(tools json.RawMessage) error {
	if err := s.ensureAgenticSchema(); err != nil {
		return err
	}
	var arr []json.RawMessage
	_ = json.Unmarshal(tools, &arr)
	_, err := s.DB().Exec(
		`INSERT INTO tool_surface_snapshots (captured_at, tool_count, tools) VALUES (?, ?, ?)`,
		time.Now().UTC().Format(time.RFC3339), len(arr), string(tools),
	)
	return err
}

// ToolSurfaceSnapshots returns the most recent n snapshots, newest first.
func (s *Store) ToolSurfaceSnapshots(n int) ([]ToolSurfaceSnapshot, error) {
	if n <= 0 {
		n = 2
	}
	rows, err := s.DB().Query(
		`SELECT captured_at, tool_count, tools FROM tool_surface_snapshots ORDER BY id DESC LIMIT ?`, n,
	)
	if isNoSuchTable(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ToolSurfaceSnapshot
	for rows.Next() {
		var snap ToolSurfaceSnapshot
		var ts, tools string
		if err := rows.Scan(&ts, &snap.ToolCount, &tools); err != nil {
			return nil, err
		}
		snap.CapturedAt, _ = time.Parse(time.RFC3339, ts)
		snap.Tools = json.RawMessage(tools)
		out = append(out, snap)
	}
	return out, rows.Err()
}

// --- guard policy ----------------------------------------------------------

// GuardPolicy is the client-side trade policy enforced before any order leaves
// the machine. Robinhood's OAuth scope is all-or-nothing and the platform
// ships no native limits, so this is the only enforceable budget/kill-switch
// layer for the agentic account.
type GuardPolicy struct {
	MaxOrderNotional float64  `json:"max_order_notional,omitempty"`
	DailyCapNotional float64  `json:"daily_cap_notional,omitempty"`
	Allowlist        []string `json:"allowlist,omitempty"`
	Denylist         []string `json:"denylist,omitempty"`
	KillSwitch       bool     `json:"kill_switch,omitempty"`
}

// GetGuardPolicy loads the persisted guard policy (zero value if unset).
func (s *Store) GetGuardPolicy() (GuardPolicy, error) {
	var p GuardPolicy
	var raw string
	err := s.DB().QueryRow(`SELECT value FROM guard_policy WHERE key = 'policy'`).Scan(&raw)
	if err == sql.ErrNoRows || isNoSuchTable(err) {
		return p, nil
	}
	if err != nil {
		return p, err
	}
	if raw != "" {
		_ = json.Unmarshal([]byte(raw), &p)
	}
	return p, nil
}

// SetGuardPolicy persists the guard policy.
func (s *Store) SetGuardPolicy(p GuardPolicy) error {
	if err := s.ensureAgenticSchema(); err != nil {
		return err
	}
	raw, err := json.Marshal(p)
	if err != nil {
		return err
	}
	_, err = s.DB().Exec(
		`INSERT INTO guard_policy (key, value) VALUES ('policy', ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`, string(raw),
	)
	return err
}

// EvaluateOrder checks an order against the policy and returns a non-empty
// reason when the order must be blocked. daySpent is the notional already
// placed today (from the journal). Pure function for straightforward testing.
func (p GuardPolicy) EvaluateOrder(symbol string, notional, daySpent float64) string {
	if p.KillSwitch {
		return "kill switch is engaged (guard set --kill=false to disarm)"
	}
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if len(p.Allowlist) > 0 && !containsFold(p.Allowlist, sym) {
		return fmt.Sprintf("%s is not on the guard allowlist", sym)
	}
	if containsFold(p.Denylist, sym) {
		return fmt.Sprintf("%s is on the guard denylist", sym)
	}
	if p.MaxOrderNotional > 0 && notional > p.MaxOrderNotional {
		return fmt.Sprintf("order notional %.2f exceeds per-order cap %.2f", notional, p.MaxOrderNotional)
	}
	if p.DailyCapNotional > 0 && daySpent+notional > p.DailyCapNotional {
		return fmt.Sprintf("order notional %.2f would exceed daily cap %.2f (already placed %.2f today)", notional, p.DailyCapNotional, daySpent)
	}
	return ""
}

func containsFold(list []string, v string) bool {
	for _, item := range list {
		if strings.EqualFold(strings.TrimSpace(item), v) {
			return true
		}
	}
	return false
}
