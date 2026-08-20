package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/sumble/internal/store"
)

// creditEnvelope captures the credit accounting Sumble attaches to every v6
// JSON response. Pointers distinguish an absent field from a real zero.
type creditEnvelope struct {
	CreditsUsed      *int `json:"credits_used"`
	CreditsRemaining *int `json:"credits_remaining"`
}

// endpointCost describes how a Sumble v6 endpoint bills so the CLI can estimate
// spend before dialing. perRow is the credit cost for each returned row; flat is
// a fixed cost regardless of rows.
type endpointCost struct {
	key           string
	perRow        int
	perRowWithDsc int // postings.find: cost per row when descriptions are included
	flat          int // intelligence-brief: fixed cost
	emailPerRow   int // people.enrich: per-person email reveal
	phonePerRow   int // people.enrich: per-person phone reveal
	note          string
}

// creditCosts is the canonical per-endpoint billing table, transcribed from the
// Sumble v6 docs. It is the single source of truth for cost-estimate.
//
// pp:novel-static-reference
var creditCosts = map[string]endpointCost{
	"organizations.find":               {key: "organizations.find", perRow: 5, note: "5 credits per organization returned"},
	"organizations.enrich":             {key: "organizations.enrich", perRow: 5, note: "5 credits per technology found"},
	"organizations.match":              {key: "organizations.match", perRow: 1, note: "1 credit per matched org; unmatched are free"},
	"organizations.intelligence-brief": {key: "organizations.intelligence-brief", flat: 50, note: "50 credits when complete; pending (202) is free"},
	"people.find":                      {key: "people.find", perRow: 1, note: "1 credit per person returned"},
	"people.find-related-people":       {key: "people.find-related-people", perRow: 1, note: "1 credit per person returned"},
	"people.enrich":                    {key: "people.enrich", emailPerRow: 10, phonePerRow: 80, note: "10 credits per email, 80 per phone (first reveal only)"},
	"postings.find":                    {key: "postings.find", perRow: 2, perRowWithDsc: 3, note: "2 credits per job, 3 with descriptions"},
	"postings.get":                     {key: "postings.get", flat: 1, note: "1 credit per job"},
	"postings.find-related-people":     {key: "postings.find-related-people", perRow: 1, note: "1 credit per person returned"},
	"technologies.find":                {key: "technologies.find", flat: 1, note: "1 credit if at least one match, else free"},
	"organization-lists.list":          {key: "organization-lists.list", perRow: 1, note: "1 credit per list returned"},
	"organization-lists.get":           {key: "organization-lists.get", perRow: 1, note: "1 credit per organization returned"},
	"contact-lists.list":               {key: "contact-lists.list", perRow: 1, note: "1 credit per list returned"},
	"contact-lists.get":                {key: "contact-lists.get", perRow: 1, note: "1 credit per person returned"},
}

// costEstimate holds a computed worst-case spend for one endpoint call.
type costEstimate struct {
	Endpoint         string `json:"endpoint"`
	Rows             int    `json:"rows"`
	EstimatedCredits int    `json:"estimated_credits"`
	Note             string `json:"note"`
}

// estimateCost computes the worst-case credit spend for an endpoint call. rows
// is the number of rows the caller expects (e.g. --limit). For people.enrich,
// rows is the number of people; email/phone toggle the per-person surcharges.
func estimateCost(key string, rows int, withDescriptions, withEmail, withPhone bool) (costEstimate, error) {
	c, ok := creditCosts[key]
	if !ok {
		return costEstimate{}, fmt.Errorf("unknown endpoint %q (valid: %s)", key, strings.Join(sortedCostKeys(), ", "))
	}
	if rows < 0 {
		return costEstimate{}, fmt.Errorf("--rows must be >= 0, got %d", rows)
	}
	total := c.flat
	switch key {
	case "people.enrich":
		per := 0
		if withEmail {
			per += c.emailPerRow
		}
		if withPhone {
			per += c.phonePerRow
		}
		total += per * rows
	case "postings.find":
		per := c.perRow
		if withDescriptions {
			per = c.perRowWithDsc
		}
		total += per * rows
	default:
		total += c.perRow * rows
	}
	return costEstimate{Endpoint: key, Rows: rows, EstimatedCredits: total, Note: c.note}, nil
}

func sortedCostKeys() []string {
	keys := make([]string, 0, len(creditCosts))
	for k := range creditCosts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// openCreditStore opens the local store and ensures the credit-economy tables
// exist. Callers must Close the returned store.
func openCreditStore() (*store.Store, error) {
	db, err := store.Open(defaultDBPath("sumble-pp-cli"))
	if err != nil {
		return nil, err
	}
	if err := ensureCreditTables(db.DB()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

// ensureCreditTables creates the credit ledger, settings, and org-tech cache
// tables. Idempotent via IF NOT EXISTS; lives outside the generated store
// migrations so a regen cannot drop it.
func ensureCreditTables(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS credit_ledger (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			ts DATETIME DEFAULT CURRENT_TIMESTAMP,
			endpoint TEXT NOT NULL,
			credits_used INTEGER NOT NULL DEFAULT 0,
			credits_remaining INTEGER,
			note TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_credit_ledger_ts ON credit_ledger(ts)`,
		`CREATE TABLE IF NOT EXISTS cli_settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS org_tech_cache (
			domain TEXT PRIMARY KEY,
			technologies TEXT NOT NULL,
			synced_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("ensuring credit tables: %w", err)
		}
	}
	return nil
}

// recordLedger appends a billed-call row. credits_remaining may be nil when the
// response did not carry it.
func recordLedger(db *sql.DB, endpoint string, used int, remaining *int, note string) error {
	var rem sql.NullInt64
	if remaining != nil {
		rem = sql.NullInt64{Int64: int64(*remaining), Valid: true}
	}
	_, err := db.Exec(
		`INSERT INTO credit_ledger (endpoint, credits_used, credits_remaining, note) VALUES (?, ?, ?, ?)`,
		endpoint, used, rem, note,
	)
	return err
}

// recordEnvelope records a ledger row from a parsed credit envelope, returning
// the credits_remaining value (or nil).
func recordEnvelope(db *sql.DB, endpoint string, env creditEnvelope, note string) *int {
	used := 0
	if env.CreditsUsed != nil {
		used = *env.CreditsUsed
	}
	_ = recordLedger(db, endpoint, used, env.CreditsRemaining, note)
	return env.CreditsRemaining
}

// parseEnvelope extracts the credit accounting from a raw Sumble response.
func parseEnvelope(raw json.RawMessage) creditEnvelope {
	var env creditEnvelope
	_ = json.Unmarshal(raw, &env)
	return env
}

// latestBalance returns the most recent credits_remaining recorded in the
// ledger, or (0, false) when no row carries one.
func latestBalance(db *sql.DB) (int, bool, error) {
	row := db.QueryRow(`SELECT credits_remaining FROM credit_ledger WHERE credits_remaining IS NOT NULL ORDER BY ts DESC, id DESC LIMIT 1`)
	var rem sql.NullInt64
	if err := row.Scan(&rem); err != nil {
		if err == sql.ErrNoRows {
			return 0, false, nil
		}
		return 0, false, err
	}
	if !rem.Valid {
		return 0, false, nil
	}
	return int(rem.Int64), true, nil
}

const budgetSettingKey = "budget_ceiling"

// getBudget returns the configured per-call credit ceiling, or (0, false) when
// no budget is set.
func getBudget(db *sql.DB) (int, bool, error) {
	row := db.QueryRow(`SELECT value FROM cli_settings WHERE key = ?`, budgetSettingKey)
	var v sql.NullString
	if err := row.Scan(&v); err != nil {
		if err == sql.ErrNoRows {
			return 0, false, nil
		}
		return 0, false, err
	}
	if !v.Valid || strings.TrimSpace(v.String) == "" {
		return 0, false, nil
	}
	var n int
	if _, err := fmt.Sscanf(v.String, "%d", &n); err != nil {
		return 0, false, nil
	}
	return n, true, nil
}

func setBudget(db *sql.DB, n int) error {
	_, err := db.Exec(
		`INSERT INTO cli_settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		budgetSettingKey, fmt.Sprintf("%d", n),
	)
	return err
}

func clearBudget(db *sql.DB) error {
	_, err := db.Exec(`DELETE FROM cli_settings WHERE key = ?`, budgetSettingKey)
	return err
}
