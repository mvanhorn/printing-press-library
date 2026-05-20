// Copyright 2026 mitch-nick. Licensed under Apache-2.0.

package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// MigrateDomainSchema creates the domain-goat-specific tables alongside the
// generic resources/sync_state tables. Idempotent; safe to call on every Open.
func (s *Store) MigrateDomainSchema(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS tlds (
			tld TEXT PRIMARY KEY,
			kind TEXT NOT NULL DEFAULT 'gTLD',
			rdap_base TEXT NOT NULL DEFAULT '',
			whois_server TEXT NOT NULL DEFAULT '',
			has_rdap INTEGER NOT NULL DEFAULT 0,
			prestige INTEGER NOT NULL DEFAULT 0,
			updated_at TEXT NOT NULL DEFAULT (datetime('now'))
		)`,
		`CREATE TABLE IF NOT EXISTS domains (
			fqdn TEXT PRIMARY KEY,
			ascii TEXT NOT NULL,
			label TEXT NOT NULL,
			tld TEXT NOT NULL,
			length INTEGER NOT NULL,
			score INTEGER NOT NULL DEFAULT 0,
			score_json TEXT NOT NULL DEFAULT '{}',
			status TEXT NOT NULL DEFAULT 'unknown',
			source TEXT NOT NULL DEFAULT '',
			premium INTEGER NOT NULL DEFAULT 0,
			created_at TEXT,
			expires_at TEXT,
			drop_at TEXT,
			last_checked_at TEXT,
			updated_at TEXT NOT NULL DEFAULT (datetime('now'))
		)`,
		`CREATE INDEX IF NOT EXISTS idx_domains_tld ON domains(tld)`,
		`CREATE INDEX IF NOT EXISTS idx_domains_status ON domains(status)`,
		`CREATE INDEX IF NOT EXISTS idx_domains_drop_at ON domains(drop_at)`,
		`CREATE INDEX IF NOT EXISTS idx_domains_score ON domains(score)`,
		`CREATE TABLE IF NOT EXISTS lists (
			name TEXT PRIMARY KEY,
			description TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT (datetime('now'))
		)`,
		`CREATE TABLE IF NOT EXISTS candidates (
			list_name TEXT NOT NULL,
			fqdn TEXT NOT NULL,
			notes TEXT NOT NULL DEFAULT '',
			tags TEXT NOT NULL DEFAULT '',
			killed INTEGER NOT NULL DEFAULT 0,
			kill_reason TEXT NOT NULL DEFAULT '',
			added_at TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at TEXT NOT NULL DEFAULT (datetime('now')),
			PRIMARY KEY (list_name, fqdn)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_candidates_fqdn ON candidates(fqdn)`,
		`CREATE INDEX IF NOT EXISTS idx_candidates_killed ON candidates(killed)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS candidates_fts USING fts5(
			fqdn, list_name, notes, tags, kill_reason,
			content='candidates', content_rowid='rowid', tokenize='porter unicode61'
		)`,
		`CREATE TABLE IF NOT EXISTS watches (
			fqdn TEXT PRIMARY KEY,
			cadence_hours INTEGER NOT NULL DEFAULT 24,
			last_run_at TEXT,
			last_status TEXT NOT NULL DEFAULT '',
			alert_channel TEXT NOT NULL DEFAULT 'none',
			created_at TEXT NOT NULL DEFAULT (datetime('now'))
		)`,
		`CREATE TABLE IF NOT EXISTS whois_records (
			fqdn TEXT NOT NULL,
			raw TEXT NOT NULL,
			parsed_json TEXT NOT NULL DEFAULT '{}',
			source TEXT NOT NULL DEFAULT 'port-43',
			fetched_at TEXT NOT NULL DEFAULT (datetime('now')),
			PRIMARY KEY (fqdn, fetched_at)
		)`,
		`CREATE TABLE IF NOT EXISTS rdap_records (
			fqdn TEXT NOT NULL,
			raw_json TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT '',
			events_json TEXT NOT NULL DEFAULT '[]',
			fetched_at TEXT NOT NULL DEFAULT (datetime('now')),
			PRIMARY KEY (fqdn, fetched_at)
		)`,
		`CREATE TABLE IF NOT EXISTS pricing_snapshots (
			tld TEXT NOT NULL,
			registrar TEXT NOT NULL DEFAULT 'porkbun',
			registration_price REAL,
			renewal_price REAL,
			transfer_price REAL,
			fetched_at TEXT NOT NULL DEFAULT (datetime('now')),
			PRIMARY KEY (tld, registrar)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_pricing_registrar ON pricing_snapshots(registrar)`,
	}

	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for _, stmt := range stmts {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("migrate stmt: %w (sql: %s)", err, stmt)
		}
	}
	return tx.Commit()
}

// TLDRow represents a TLD with metadata.
type TLDRow struct {
	TLD         string `json:"tld"`
	Kind        string `json:"kind"`
	RDAPBase    string `json:"rdap_base"`
	WHOISServer string `json:"whois_server"`
	HasRDAP     bool   `json:"has_rdap"`
	Prestige    int    `json:"prestige"`
}

// UpsertTLD inserts or updates a TLD row.
func (s *Store) UpsertTLD(ctx context.Context, t TLDRow) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	hasRDAP := 0
	if t.HasRDAP {
		hasRDAP = 1
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO tlds (tld, kind, rdap_base, whois_server, has_rdap, prestige, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, datetime('now'))
		ON CONFLICT(tld) DO UPDATE SET
			kind=excluded.kind,
			rdap_base=excluded.rdap_base,
			whois_server=excluded.whois_server,
			has_rdap=excluded.has_rdap,
			prestige=excluded.prestige,
			updated_at=datetime('now')
	`, strings.ToLower(t.TLD), t.Kind, t.RDAPBase, t.WHOISServer, hasRDAP, t.Prestige)
	return err
}

// GetTLD fetches a single TLD row.
func (s *Store) GetTLD(ctx context.Context, tld string) (*TLDRow, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT tld, kind, rdap_base, whois_server, has_rdap, prestige
		FROM tlds WHERE tld = ?`, strings.ToLower(tld))
	t := &TLDRow{}
	var hasRDAP int
	if err := row.Scan(&t.TLD, &t.Kind, &t.RDAPBase, &t.WHOISServer, &hasRDAP, &t.Prestige); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	t.HasRDAP = hasRDAP == 1
	return t, nil
}

// ListTLDs returns all TLDs ordered by name.
func (s *Store) ListTLDs(ctx context.Context) ([]TLDRow, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT tld, kind, rdap_base, whois_server, has_rdap, prestige
		FROM tlds ORDER BY tld`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TLDRow
	for rows.Next() {
		var t TLDRow
		var hasRDAP int
		if err := rows.Scan(&t.TLD, &t.Kind, &t.RDAPBase, &t.WHOISServer, &hasRDAP, &t.Prestige); err != nil {
			return nil, err
		}
		t.HasRDAP = hasRDAP == 1
		out = append(out, t)
	}
	return out, rows.Err()
}

// PricingRow represents a single TLD's pricing snapshot from a registrar.
type PricingRow struct {
	TLD          string  `json:"tld"`
	Registrar    string  `json:"registrar"`
	Registration float64 `json:"registration"`
	Renewal      float64 `json:"renewal"`
	Transfer     float64 `json:"transfer"`
}

// UpsertPricing writes a pricing snapshot.
func (s *Store) UpsertPricing(ctx context.Context, p PricingRow) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO pricing_snapshots (tld, registrar, registration_price, renewal_price, transfer_price, fetched_at)
		VALUES (?, ?, ?, ?, ?, datetime('now'))
		ON CONFLICT(tld, registrar) DO UPDATE SET
			registration_price=excluded.registration_price,
			renewal_price=excluded.renewal_price,
			transfer_price=excluded.transfer_price,
			fetched_at=datetime('now')
	`, strings.ToLower(p.TLD), p.Registrar, p.Registration, p.Renewal, p.Transfer)
	return err
}

// GetPricing fetches pricing for a TLD from a registrar.
func (s *Store) GetPricing(ctx context.Context, tld, registrar string) (*PricingRow, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT tld, registrar, registration_price, renewal_price, transfer_price
		FROM pricing_snapshots WHERE tld = ? AND registrar = ?`,
		strings.ToLower(tld), registrar)
	p := &PricingRow{}
	if err := row.Scan(&p.TLD, &p.Registrar, &p.Registration, &p.Renewal, &p.Transfer); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return p, nil
}

// ListPricing returns all pricing rows ordered by TLD.
func (s *Store) ListPricing(ctx context.Context, registrar string, limit int) ([]PricingRow, error) {
	q := `SELECT tld, registrar, registration_price, renewal_price, transfer_price FROM pricing_snapshots`
	args := []any{}
	if registrar != "" {
		q += " WHERE registrar = ?"
		args = append(args, registrar)
	}
	q += " ORDER BY tld"
	if limit > 0 {
		q += " LIMIT ?"
		args = append(args, limit)
	}
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PricingRow
	for rows.Next() {
		var p PricingRow
		if err := rows.Scan(&p.TLD, &p.Registrar, &p.Registration, &p.Renewal, &p.Transfer); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// DomainRow represents a tracked domain.
type DomainRow struct {
	FQDN          string    `json:"fqdn"`
	ASCII         string    `json:"ascii"`
	Label         string    `json:"label"`
	TLD           string    `json:"tld"`
	Length        int       `json:"length"`
	Score         int       `json:"score"`
	ScoreJSON     string    `json:"score_json,omitempty"`
	Status        string    `json:"status"`
	Source        string    `json:"source"`
	Premium       bool      `json:"premium"`
	CreatedAt     string    `json:"created_at,omitempty"`
	ExpiresAt     string    `json:"expires_at,omitempty"`
	DropAt        string    `json:"drop_at,omitempty"`
	LastCheckedAt string    `json:"last_checked_at,omitempty"`
	UpdatedAt     time.Time `json:"updated_at,omitempty"`
}

// UpsertDomain writes a domain row.
func (s *Store) UpsertDomain(ctx context.Context, d DomainRow) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	premium := 0
	if d.Premium {
		premium = 1
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO domains (fqdn, ascii, label, tld, length, score, score_json, status, source, premium,
			created_at, expires_at, drop_at, last_checked_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now'))
		ON CONFLICT(fqdn) DO UPDATE SET
			ascii=excluded.ascii, label=excluded.label, tld=excluded.tld, length=excluded.length,
			score=COALESCE(NULLIF(excluded.score, 0), domains.score),
			score_json=CASE WHEN excluded.score_json != '{}' THEN excluded.score_json ELSE domains.score_json END,
			status=excluded.status, source=excluded.source, premium=excluded.premium,
			created_at=COALESCE(NULLIF(excluded.created_at, ''), domains.created_at),
			expires_at=COALESCE(NULLIF(excluded.expires_at, ''), domains.expires_at),
			drop_at=COALESCE(NULLIF(excluded.drop_at, ''), domains.drop_at),
			last_checked_at=excluded.last_checked_at,
			updated_at=datetime('now')
	`, strings.ToLower(d.FQDN), d.ASCII, d.Label, d.TLD, d.Length, d.Score, defaultStr(d.ScoreJSON, "{}"),
		d.Status, d.Source, premium, d.CreatedAt, d.ExpiresAt, d.DropAt, d.LastCheckedAt)
	return err
}

// GetDomain reads a domain row.
func (s *Store) GetDomain(ctx context.Context, fqdn string) (*DomainRow, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT fqdn, ascii, label, tld, length, score, score_json, status, source, premium,
			COALESCE(created_at, ''), COALESCE(expires_at, ''), COALESCE(drop_at, ''),
			COALESCE(last_checked_at, ''), updated_at
		FROM domains WHERE fqdn = ?`, strings.ToLower(fqdn))
	d := &DomainRow{}
	var premium int
	var updatedAt string
	if err := row.Scan(&d.FQDN, &d.ASCII, &d.Label, &d.TLD, &d.Length, &d.Score, &d.ScoreJSON,
		&d.Status, &d.Source, &premium, &d.CreatedAt, &d.ExpiresAt, &d.DropAt, &d.LastCheckedAt, &updatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	d.Premium = premium == 1
	t, _ := time.Parse("2006-01-02 15:04:05", updatedAt)
	d.UpdatedAt = t
	return d, nil
}

// ListRow represents a candidate list.
type ListRow struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
	Size        int    `json:"size,omitempty"`
}

// CreateList creates or updates a list.
func (s *Store) CreateList(ctx context.Context, name, description string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO lists (name, description, created_at) VALUES (?, ?, datetime('now'))
		ON CONFLICT(name) DO UPDATE SET description=COALESCE(NULLIF(excluded.description, ''), lists.description)`,
		name, description)
	return err
}

// ListLists returns all candidate lists with size counts.
func (s *Store) ListLists(ctx context.Context) ([]ListRow, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT l.name, l.description, l.created_at,
			COALESCE((SELECT COUNT(*) FROM candidates c WHERE c.list_name = l.name AND c.killed = 0), 0)
		FROM lists l ORDER BY l.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ListRow
	for rows.Next() {
		var l ListRow
		if err := rows.Scan(&l.Name, &l.Description, &l.CreatedAt, &l.Size); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// CandidateRow represents a candidate in a list.
type CandidateRow struct {
	ListName   string `json:"list_name"`
	FQDN       string `json:"fqdn"`
	Notes      string `json:"notes,omitempty"`
	Tags       string `json:"tags,omitempty"`
	Killed     bool   `json:"killed"`
	KillReason string `json:"kill_reason,omitempty"`
	AddedAt    string `json:"added_at,omitempty"`
}

// PATCH(store-sticky-kill-and-receiver-shadow): AddCandidate ON CONFLICT preserves killed=1 (sticky-kill) so shortlist promote can't silently revive killed candidates; UpsertBatch local 's' was renamed to 'idStr' so it stops shadowing the *Store receiver.
// AddCandidate inserts a candidate into a list (creating the list if absent).
func (s *Store) AddCandidate(ctx context.Context, c CandidateRow) error {
	if c.ListName == "" {
		return fmt.Errorf("list name required")
	}
	if err := s.CreateList(ctx, c.ListName, ""); err != nil {
		return err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	killed := 0
	if c.Killed {
		killed = 1
	}
	// Sticky-kill: once killed=1, AddCandidate can only re-affirm — never
	// silently revive. Previously, ON CONFLICT set killed=excluded.killed
	// unconditionally, so shortlist promote (which always passes killed=0)
	// would resurrect a candidate the user had explicitly killed via
	// `lists kill`. Reviving a killed candidate requires a dedicated
	// revive path; AddCandidate is for inserts and metadata refresh.
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO candidates (list_name, fqdn, notes, tags, killed, kill_reason, added_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))
		ON CONFLICT(list_name, fqdn) DO UPDATE SET
			notes=CASE WHEN excluded.notes != '' THEN excluded.notes ELSE candidates.notes END,
			tags=CASE WHEN excluded.tags != '' THEN excluded.tags ELSE candidates.tags END,
			killed=CASE WHEN excluded.killed = 1 THEN 1 ELSE candidates.killed END,
			kill_reason=CASE WHEN excluded.kill_reason != '' THEN excluded.kill_reason ELSE candidates.kill_reason END,
			updated_at=datetime('now')
	`, c.ListName, strings.ToLower(c.FQDN), c.Notes, c.Tags, killed, c.KillReason)
	if err != nil {
		return err
	}
	// Refresh FTS index — content table mode means we need explicit rebuild on changes.
	_, _ = s.db.ExecContext(ctx, `INSERT INTO candidates_fts(candidates_fts) VALUES('rebuild')`)
	return nil
}

// ListCandidates returns candidates in a list (or all if name=="").
func (s *Store) ListCandidates(ctx context.Context, listName string, includeKilled bool) ([]CandidateRow, error) {
	q := `SELECT list_name, fqdn, notes, tags, killed, kill_reason, added_at FROM candidates`
	args := []any{}
	conds := []string{}
	if listName != "" {
		conds = append(conds, "list_name = ?")
		args = append(args, listName)
	}
	if !includeKilled {
		conds = append(conds, "killed = 0")
	}
	if len(conds) > 0 {
		q += " WHERE " + strings.Join(conds, " AND ")
	}
	q += " ORDER BY added_at DESC"
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CandidateRow
	for rows.Next() {
		var c CandidateRow
		var killed int
		if err := rows.Scan(&c.ListName, &c.FQDN, &c.Notes, &c.Tags, &killed, &c.KillReason, &c.AddedAt); err != nil {
			return nil, err
		}
		c.Killed = killed == 1
		out = append(out, c)
	}
	return out, rows.Err()
}

// SearchCandidates runs FTS5 over candidates (notes + tags + kill_reason).
func (s *Store) SearchCandidates(ctx context.Context, query string, limit int) ([]CandidateRow, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT c.list_name, c.fqdn, c.notes, c.tags, c.killed, c.kill_reason, c.added_at
		FROM candidates_fts f JOIN candidates c ON c.rowid = f.rowid
		WHERE candidates_fts MATCH ?
		ORDER BY rank LIMIT ?`, query, limit)
	if err != nil {
		// FTS5 raises an error on malformed query — fall back to LIKE
		like := "%" + query + "%"
		rows, err = s.db.QueryContext(ctx, `
			SELECT list_name, fqdn, notes, tags, killed, kill_reason, added_at
			FROM candidates
			WHERE notes LIKE ? OR tags LIKE ? OR kill_reason LIKE ? OR fqdn LIKE ?
			ORDER BY added_at DESC LIMIT ?`, like, like, like, like, limit)
		if err != nil {
			return nil, err
		}
	}
	defer rows.Close()
	var out []CandidateRow
	for rows.Next() {
		var c CandidateRow
		var killed int
		if err := rows.Scan(&c.ListName, &c.FQDN, &c.Notes, &c.Tags, &killed, &c.KillReason, &c.AddedAt); err != nil {
			return nil, err
		}
		c.Killed = killed == 1
		out = append(out, c)
	}
	return out, rows.Err()
}

// WatchRow represents a watch entry.
type WatchRow struct {
	FQDN         string `json:"fqdn"`
	CadenceHours int    `json:"cadence_hours"`
	LastRunAt    string `json:"last_run_at,omitempty"`
	LastStatus   string `json:"last_status,omitempty"`
	AlertChannel string `json:"alert_channel,omitempty"`
}

// AddWatch inserts or updates a watch entry.
func (s *Store) AddWatch(ctx context.Context, w WatchRow) error {
	if w.CadenceHours <= 0 {
		w.CadenceHours = 24
	}
	if w.AlertChannel == "" {
		w.AlertChannel = "none"
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO watches (fqdn, cadence_hours, alert_channel)
		VALUES (?, ?, ?)
		ON CONFLICT(fqdn) DO UPDATE SET
			cadence_hours=excluded.cadence_hours,
			alert_channel=excluded.alert_channel`,
		strings.ToLower(w.FQDN), w.CadenceHours, w.AlertChannel)
	return err
}

// ListWatches returns all watches.
func (s *Store) ListWatches(ctx context.Context) ([]WatchRow, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT fqdn, cadence_hours, COALESCE(last_run_at, ''), last_status, alert_channel
		FROM watches ORDER BY fqdn`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WatchRow
	for rows.Next() {
		var w WatchRow
		if err := rows.Scan(&w.FQDN, &w.CadenceHours, &w.LastRunAt, &w.LastStatus, &w.AlertChannel); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// ListDueWatches returns watches whose cadence has elapsed since the last
// run — i.e. last_run_at is NULL or last_run_at + cadence_hours <= now.
// Used by `watch run` so users wiring it into a cron tick honour the
// per-domain cadence rather than re-checking every domain on every tick.
func (s *Store) ListDueWatches(ctx context.Context) ([]WatchRow, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT fqdn, cadence_hours, COALESCE(last_run_at, ''), last_status, alert_channel
		FROM watches
		WHERE last_run_at IS NULL
		   OR datetime(last_run_at, '+' || cadence_hours || ' hours') <= datetime('now')
		ORDER BY fqdn`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WatchRow
	for rows.Next() {
		var w WatchRow
		if err := rows.Scan(&w.FQDN, &w.CadenceHours, &w.LastRunAt, &w.LastStatus, &w.AlertChannel); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// RemoveWatch deletes a watch entry.
func (s *Store) RemoveWatch(ctx context.Context, fqdn string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.ExecContext(ctx, `DELETE FROM watches WHERE fqdn = ?`, strings.ToLower(fqdn))
	return err
}

// UpdateWatchResult records the last status of a watch run.
func (s *Store) UpdateWatchResult(ctx context.Context, fqdn, status string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.ExecContext(ctx, `
		UPDATE watches SET last_run_at = datetime('now'), last_status = ? WHERE fqdn = ?`,
		status, strings.ToLower(fqdn))
	return err
}

// SaveWhoisRecord persists a raw WHOIS response.
func (s *Store) SaveWhoisRecord(ctx context.Context, fqdn, raw, parsedJSON, source string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO whois_records (fqdn, raw, parsed_json, source) VALUES (?, ?, ?, ?)`,
		strings.ToLower(fqdn), raw, defaultStr(parsedJSON, "{}"), defaultStr(source, "port-43"))
	return err
}

// SaveRDAPRecord persists an RDAP response.
func (s *Store) SaveRDAPRecord(ctx context.Context, fqdn, rawJSON, status, eventsJSON string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO rdap_records (fqdn, raw_json, status, events_json) VALUES (?, ?, ?, ?)`,
		strings.ToLower(fqdn), rawJSON, status, defaultStr(eventsJSON, "[]"))
	return err
}

// GetLatestRDAP returns the most recent RDAP record for a domain.
func (s *Store) GetLatestRDAP(ctx context.Context, fqdn string) (rawJSON, status, eventsJSON, fetchedAt string, err error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT raw_json, status, events_json, fetched_at FROM rdap_records
		WHERE fqdn = ? ORDER BY fetched_at DESC LIMIT 1`, strings.ToLower(fqdn))
	err = row.Scan(&rawJSON, &status, &eventsJSON, &fetchedAt)
	if err == sql.ErrNoRows {
		err = nil
	}
	return
}

func defaultStr(s, d string) string {
	if s == "" {
		return d
	}
	return s
}

// PricingForFQDN returns Porkbun pricing for a domain's TLD (most common case).
func (s *Store) PricingForFQDN(ctx context.Context, fqdn string) (*PricingRow, error) {
	idx := strings.Index(fqdn, ".")
	if idx < 0 {
		return nil, nil
	}
	tld := fqdn[idx+1:]
	return s.GetPricing(ctx, tld, "porkbun")
}

// MarshalJSONInline is a helper to convert any value into JSON string.
func MarshalJSONInline(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}
