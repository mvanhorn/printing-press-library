// Copyright 2026 Matias Sanchez Moises and contributors. Licensed under Apache-2.0. See LICENSE.

// Package cli — contacts_db.go
// Local SQLite store for Contacts data synced from Contacts.app via JXA.
// Schema mirrors the Printing Press store pattern: typed columns + sync_state.
package cli

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// ── store ─────────────────────────────────────────────────────────────────────

type contactStore struct {
	db *sql.DB
}

func contactsDBPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "Application Support", "icloud-pp-cli", "contacts.db")
}

func openContactStore() (*contactStore, error) {
	path := contactsDBPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("creating contacts db dir: %w", err)
	}
	db, err := sql.Open("sqlite",
		path+"?_journal_mode=WAL&_synchronous=NORMAL&_busy_timeout=5000&_foreign_keys=ON&_temp_store=MEMORY",
	)
	if err != nil {
		return nil, fmt.Errorf("opening contacts db: %w", err)
	}
	db.SetMaxOpenConns(4)
	s := &contactStore{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("contacts db migrate: %w", err)
	}
	return s, nil
}

func (s *contactStore) Close() error { return s.db.Close() }

func (s *contactStore) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS contacts (
			id           TEXT PRIMARY KEY,
			first_name   TEXT,
			last_name    TEXT,
			middle_name  TEXT,
			organization TEXT,
			job_title    TEXT,
			note         TEXT,
			birthday     TEXT,
			modified_at  TEXT,
			synced_at    DATETIME DEFAULT CURRENT_TIMESTAMP,
			uuid         TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_contacts_org      ON contacts(organization)`,
		`CREATE INDEX IF NOT EXISTS idx_contacts_modified ON contacts(modified_at)`,

		`CREATE TABLE IF NOT EXISTS contact_phones (
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			contact_id   TEXT NOT NULL REFERENCES contacts(id) ON DELETE CASCADE,
			label        TEXT,
			value        TEXT,
			normalized   TEXT,
			country_code TEXT,
			country_iso  TEXT,
			country      TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_phones_contact ON contact_phones(contact_id)`,
		`CREATE INDEX IF NOT EXISTS idx_phones_country ON contact_phones(country_iso)`,

		`CREATE TABLE IF NOT EXISTS contact_emails (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			contact_id TEXT NOT NULL REFERENCES contacts(id) ON DELETE CASCADE,
			label      TEXT,
			value      TEXT,
			domain     TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_emails_contact ON contact_emails(contact_id)`,
		`CREATE INDEX IF NOT EXISTS idx_emails_domain  ON contact_emails(domain)`,

		`CREATE TABLE IF NOT EXISTS contact_addresses (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			contact_id TEXT NOT NULL REFERENCES contacts(id) ON DELETE CASCADE,
			label      TEXT,
			street     TEXT,
			city       TEXT,
			state      TEXT,
			zip        TEXT,
			country    TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_addresses_contact ON contact_addresses(contact_id)`,

		`CREATE TABLE IF NOT EXISTS contact_urls (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			contact_id TEXT NOT NULL REFERENCES contacts(id) ON DELETE CASCADE,
			label      TEXT,
			value      TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_urls_contact ON contact_urls(contact_id)`,

		// FTS5 for fast full-text search across name, org, phones, emails, note.
		// External content table: we manage inserts/deletes manually.
		`CREATE VIRTUAL TABLE IF NOT EXISTS contacts_fts USING fts5(
			id UNINDEXED,
			body,
			tokenize='unicode61'
		)`,

		`CREATE TABLE IF NOT EXISTS sync_state (
			key        TEXT PRIMARY KEY,
			value      TEXT,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("migrate: %w\nstmt: %s", err, stmt[:min(60, len(stmt))])
		}
	}
	// Incremental migrations for existing databases (errors from ALTER TABLE are
	// ignored — SQLite returns "duplicate column name" when column already exists).
	s.db.Exec("ALTER TABLE contacts ADD COLUMN uuid TEXT")
	s.db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_contacts_uuid ON contacts(uuid)")
	return nil
}

// ── sync ──────────────────────────────────────────────────────────────────────

// jxaContact is the JSON shape emitted by the JXA sync script.
type jxaContact struct {
	ID           string      `json:"id"`
	FirstName    interface{} `json:"firstName"`
	LastName     interface{} `json:"lastName"`
	MiddleName   interface{} `json:"middleName"`
	Organization interface{} `json:"organization"`
	JobTitle     interface{} `json:"jobTitle"`
	Note         interface{} `json:"note"`
	Birthday     interface{} `json:"birthday"`
	ModifiedAt   interface{} `json:"modifiedAt"`
	Phones       []jxaMulti  `json:"phones"`
	Emails       []jxaMulti  `json:"emails"`
	Addresses    []jxaAddr   `json:"addresses"`
	URLs         []jxaMulti  `json:"urls"`
}

type jxaMulti struct {
	Label string      `json:"label"`
	Value interface{} `json:"value"`
}

type jxaAddr struct {
	Label   string      `json:"label"`
	Street  interface{} `json:"street"`
	City    interface{} `json:"city"`
	State   interface{} `json:"state"`
	Zip     interface{} `json:"zip"`
	Country interface{} `json:"country"`
}

func str(v interface{}) string {
	if v == nil {
		return ""
	}
	s, _ := v.(string)
	return strings.TrimSpace(s)
}

// extractUUID strips the ":ABPerson" (or similar) suffix from Apple's contact ID,
// returning the bare UUID that serves as a shorter stable identifier.
func extractUUID(appleID string) string {
	if i := strings.LastIndex(appleID, ":"); i >= 0 {
		return appleID[:i]
	}
	return appleID
}

// SyncAll replaces all contact data with the provided JXA-exported list.
func (s *contactStore) SyncAll(contacts []jxaContact) (int, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	// Full replace: delete all child rows first (FK cascade) then contacts.
	for _, t := range []string{"contact_phones", "contact_emails", "contact_addresses", "contact_urls"} {
		if _, err := tx.Exec("DELETE FROM " + t); err != nil {
			return 0, fmt.Errorf("clear %s: %w", t, err)
		}
	}
	if _, err := tx.Exec("DELETE FROM contacts"); err != nil {
		return 0, fmt.Errorf("clear contacts: %w", err)
	}
	if _, err := tx.Exec("DELETE FROM contacts_fts"); err != nil {
		return 0, fmt.Errorf("clear contacts_fts: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)

	for i := range contacts {
		c := &contacts[i]
		fn := str(c.FirstName)
		ln := str(c.LastName)
		mn := str(c.MiddleName)
		org := str(c.Organization)
		jt := str(c.JobTitle)
		note := str(c.Note)
		bday := str(c.Birthday)
		mod := str(c.ModifiedAt)

		_, err := tx.Exec(
			`INSERT INTO contacts (id, uuid, first_name, last_name, middle_name, organization, job_title, note, birthday, modified_at, synced_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			c.ID, extractUUID(c.ID), fn, ln, mn, org, jt, note, bday, mod, now,
		)
		if err != nil {
			return 0, fmt.Errorf("insert contact %s: %w", c.ID, err)
		}

		// Phones
		var phoneTokens []string
		for _, ph := range c.Phones {
			v := str(ph.Value)
			if v == "" {
				continue
			}
			norm := normalizePhone(v)
			code, iso, country := resolvePhoneCountry(norm)
			if _, err := tx.Exec(
				`INSERT INTO contact_phones (contact_id, label, value, normalized, country_code, country_iso, country) VALUES (?, ?, ?, ?, ?, ?, ?)`,
				c.ID, cleanLabel(ph.Label), v, norm, code, iso, country,
			); err != nil {
				return 0, fmt.Errorf("insert phone: %w", err)
			}
			phoneTokens = append(phoneTokens, v)
		}

		// Emails
		var emailTokens []string
		for _, em := range c.Emails {
			v := str(em.Value)
			if v == "" {
				continue
			}
			domain := ""
			if at := strings.LastIndex(v, "@"); at >= 0 {
				domain = strings.ToLower(v[at+1:])
			}
			if _, err := tx.Exec(
				`INSERT INTO contact_emails (contact_id, label, value, domain) VALUES (?, ?, ?, ?)`,
				c.ID, cleanLabel(em.Label), v, domain,
			); err != nil {
				return 0, fmt.Errorf("insert email: %w", err)
			}
			emailTokens = append(emailTokens, v)
		}

		// Addresses
		for _, addr := range c.Addresses {
			if _, err := tx.Exec(
				`INSERT INTO contact_addresses (contact_id, label, street, city, state, zip, country) VALUES (?, ?, ?, ?, ?, ?, ?)`,
				c.ID, cleanLabel(addr.Label),
				str(addr.Street), str(addr.City), str(addr.State), str(addr.Zip), str(addr.Country),
			); err != nil {
				return 0, fmt.Errorf("insert address: %w", err)
			}
		}

		// URLs
		for _, u := range c.URLs {
			v := str(u.Value)
			if v == "" {
				continue
			}
			if _, err := tx.Exec(
				`INSERT INTO contact_urls (contact_id, label, value) VALUES (?, ?, ?)`,
				c.ID, cleanLabel(u.Label), v,
			); err != nil {
				return 0, fmt.Errorf("insert url: %w", err)
			}
		}

		// FTS body
		body := strings.Join(filterEmpty(fn, ln, mn, org, jt, note,
			strings.Join(phoneTokens, " "),
			strings.Join(emailTokens, " ")), " ")
		if _, err := tx.Exec(
			`INSERT INTO contacts_fts (id, body) VALUES (?, ?)`,
			c.ID, body,
		); err != nil {
			return 0, fmt.Errorf("insert fts: %w", err)
		}
	}

	// Update sync_state
	if _, err := tx.Exec(
		`INSERT INTO sync_state (key, value, updated_at) VALUES ('last_synced_at', ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		now,
	); err != nil {
		return 0, fmt.Errorf("update sync_state: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(contacts), nil
}

// ── read ──────────────────────────────────────────────────────────────────────

// Contact is the full contact record returned to CLI commands.
type Contact struct {
	ID           string           `json:"id"`
	UUID         string           `json:"uuid,omitempty"`
	FirstName    string           `json:"first_name"`
	LastName     string           `json:"last_name,omitempty"`
	MiddleName   string           `json:"middle_name,omitempty"`
	Organization string           `json:"organization,omitempty"`
	JobTitle     string           `json:"job_title,omitempty"`
	Note         string           `json:"note,omitempty"`
	Birthday     string           `json:"birthday,omitempty"`
	ModifiedAt   string           `json:"modified_at,omitempty"`
	SyncedAt     string           `json:"synced_at,omitempty"`
	Phones       []ContactPhone   `json:"phones,omitempty"`
	Emails       []ContactEmail   `json:"emails,omitempty"`
	Addresses    []ContactAddress `json:"addresses,omitempty"`
	URLs         []ContactURL     `json:"urls,omitempty"`
}

// DisplayName returns "First Last" or organization as fallback.
func (c *Contact) DisplayName() string {
	parts := filterEmpty(c.FirstName, c.LastName)
	if len(parts) > 0 {
		return strings.Join(parts, " ")
	}
	if c.Organization != "" {
		return c.Organization
	}
	return "(no name)"
}

type ContactPhone struct {
	Label       string `json:"label"`
	Value       string `json:"value"`
	Country     string `json:"country,omitempty"`
	CountryISO  string `json:"country_iso,omitempty"`
	CountryCode string `json:"country_code,omitempty"`
}

type ContactEmail struct {
	Label  string `json:"label"`
	Value  string `json:"value"`
	Domain string `json:"domain,omitempty"`
}

type ContactAddress struct {
	Label   string `json:"label"`
	Street  string `json:"street,omitempty"`
	City    string `json:"city,omitempty"`
	State   string `json:"state,omitempty"`
	Zip     string `json:"zip,omitempty"`
	Country string `json:"country,omitempty"`
}

type ContactURL struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

func (s *contactStore) Count() (int, error) {
	var n int
	return n, s.db.QueryRow("SELECT COUNT(*) FROM contacts").Scan(&n)
}

func (s *contactStore) LastSyncedAt() string {
	var v sql.NullString
	s.db.QueryRow("SELECT value FROM sync_state WHERE key = 'last_synced_at'").Scan(&v)
	if v.Valid {
		return v.String
	}
	return ""
}

func (s *contactStore) List(limit, offset int) ([]Contact, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(
		`SELECT id, uuid, first_name, last_name, organization, synced_at
		 FROM contacts
		 ORDER BY first_name, last_name
		 LIMIT ? OFFSET ?`, limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var cs []Contact
	for rows.Next() {
		var c Contact
		var uuidN, ln, org, sa sql.NullString
		if err := rows.Scan(&c.ID, &uuidN, &c.FirstName, &ln, &org, &sa); err != nil {
			return nil, err
		}
		c.UUID = uuidN.String
		c.LastName = ln.String
		c.Organization = org.String
		c.SyncedAt = sa.String
		cs = append(cs, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range cs {
		if err := s.loadRelations(&cs[i]); err != nil {
			return nil, err
		}
	}
	return cs, nil
}

func (s *contactStore) Get(id string) (*Contact, error) {
	var c Contact
	var uuidN, ln, mn, org, jt, note, bday, mod, sa sql.NullString
	err := s.db.QueryRow(
		`SELECT id, uuid, first_name, last_name, middle_name, organization, job_title, note, birthday, modified_at, synced_at
		 FROM contacts WHERE id = ?`, id,
	).Scan(&c.ID, &uuidN, &c.FirstName, &ln, &mn, &org, &jt, &note, &bday, &mod, &sa)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	c.UUID = uuidN.String
	c.LastName, c.MiddleName, c.Organization = ln.String, mn.String, org.String
	c.JobTitle, c.Note, c.Birthday = jt.String, note.String, bday.String
	c.ModifiedAt, c.SyncedAt = mod.String, sa.String
	return &c, s.loadRelations(&c)
}

// GetByAny resolves a contact by full Apple ID, bare UUID, or UUID prefix.
// Returns an error if the prefix is ambiguous (matches >1 contact).
func (s *contactStore) GetByAny(input string) (*Contact, error) {
	// 1. Exact match on Apple ID (e.g. "UUID:ABPerson")
	if strings.Contains(input, ":") {
		return s.Get(input)
	}
	// 2. Exact UUID match
	var id string
	err := s.db.QueryRow("SELECT id FROM contacts WHERE uuid = ?", input).Scan(&id)
	if err == nil {
		return s.Get(id)
	}
	// 3. UUID prefix match — must be unique. Escape SQLite LIKE wildcards
	// (% and _) in the input so callers can't broaden the match.
	rows, err := s.db.Query(`SELECT id FROM contacts WHERE uuid LIKE ? || '%' ESCAPE '\' LIMIT 3`, escapeLike(input))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var rid string
		if err := rows.Scan(&rid); err != nil {
			return nil, err
		}
		ids = append(ids, rid)
	}
	rows.Close()
	switch len(ids) {
	case 0:
		return nil, fmt.Errorf("no contact found for %q", input)
	case 1:
		return s.Get(ids[0])
	default:
		return nil, fmt.Errorf("ambiguous prefix %q matches %d contacts — use more characters", input, len(ids))
	}
}

func (s *contactStore) Search(query string, limit int) ([]Contact, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(
		`SELECT c.id, c.uuid, c.first_name, c.last_name, c.organization, c.synced_at
		 FROM contacts c
		 JOIN contacts_fts f ON f.id = c.id
		 WHERE contacts_fts MATCH ?
		 ORDER BY rank
		 LIMIT ?`, query, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("fts search: %w", err)
	}
	defer rows.Close()
	var cs []Contact
	for rows.Next() {
		var c Contact
		var uuidN, ln, org, sa sql.NullString
		if err := rows.Scan(&c.ID, &uuidN, &c.FirstName, &ln, &org, &sa); err != nil {
			return nil, err
		}
		c.UUID = uuidN.String
		c.LastName, c.Organization, c.SyncedAt = ln.String, org.String, sa.String
		cs = append(cs, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range cs {
		if err := s.loadRelations(&cs[i]); err != nil {
			return nil, err
		}
	}
	return cs, nil
}

func (s *contactStore) Delete(id string) error {
	_, err := s.db.Exec("DELETE FROM contacts WHERE id = ?", id)
	if err != nil {
		return err
	}
	_, err = s.db.Exec("DELETE FROM contacts_fts WHERE id = ?", id)
	return err
}

// ── duplicates & merge ────────────────────────────────────────────────────────

// DuplicatePair is a pair of contacts that may be duplicates.
type DuplicatePair struct {
	Reason string  `json:"reason"`
	A      Contact `json:"a"`
	B      Contact `json:"b"`
}

// FindDuplicates returns candidate duplicate pairs grouped by detection reason.
// Checks: exact name, same-first-with-prefix-last, same phone, same email.
func (s *contactStore) FindDuplicates() ([]DuplicatePair, error) {
	seen := map[string]bool{} // dedup pair keys (idA+idB)
	var pairs []DuplicatePair

	addPair := func(reason, idA, idB string) {
		key := idA + "|" + idB
		if seen[key] {
			return
		}
		seen[key] = true
		a, err := s.Get(idA)
		if err != nil || a == nil {
			return
		}
		b, err := s.Get(idB)
		if err != nil || b == nil {
			return
		}
		pairs = append(pairs, DuplicatePair{Reason: reason, A: *a, B: *b})
	}

	// Exact name match (case-insensitive, non-empty names)
	rows, err := s.db.Query(`
		SELECT a.id, b.id
		FROM contacts a JOIN contacts b ON a.id < b.id
		WHERE a.first_name != '' AND a.last_name != ''
		  AND LOWER(a.first_name) = LOWER(b.first_name)
		  AND LOWER(a.last_name)  = LOWER(b.last_name)
	`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var idA, idB string
		if err := rows.Scan(&idA, &idB); err != nil {
			return nil, err
		}
		addPair("exact_name", idA, idB)
	}
	rows.Close()

	// Same first name, one last name is a prefix of the other (e.g. "Gomes" vs "Gomes SF")
	rows, err = s.db.Query(`
		SELECT a.id, b.id
		FROM contacts a JOIN contacts b ON a.id < b.id
		WHERE a.first_name != '' AND b.first_name != ''
		  AND LOWER(a.first_name) = LOWER(b.first_name)
		  AND a.last_name != b.last_name
		  AND (LOWER(b.last_name) LIKE LOWER(a.last_name) || ' %'
		    OR LOWER(a.last_name) LIKE LOWER(b.last_name) || ' %')
	`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var idA, idB string
		if err := rows.Scan(&idA, &idB); err != nil {
			return nil, err
		}
		addPair("similar_name", idA, idB)
	}
	rows.Close()

	// Same normalized phone number
	rows, err = s.db.Query(`
		SELECT DISTINCT pa.contact_id, pb.contact_id, pa.normalized
		FROM contact_phones pa
		JOIN contact_phones pb ON pa.normalized = pb.normalized
		  AND pa.contact_id < pb.contact_id
		WHERE pa.normalized != ''
	`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var idA, idB, norm string
		if err := rows.Scan(&idA, &idB, &norm); err != nil {
			return nil, err
		}
		addPair("same_phone:"+norm, idA, idB)
	}
	rows.Close()

	// Same email address (case-insensitive)
	rows, err = s.db.Query(`
		SELECT DISTINCT ea.contact_id, eb.contact_id, LOWER(ea.value)
		FROM contact_emails ea
		JOIN contact_emails eb ON LOWER(ea.value) = LOWER(eb.value)
		  AND ea.contact_id < eb.contact_id
		WHERE ea.value != ''
	`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var idA, idB, email string
		if err := rows.Scan(&idA, &idB, &email); err != nil {
			return nil, err
		}
		addPair("same_email:"+email, idA, idB)
	}
	rows.Close()

	return pairs, nil
}

// MergeIntoStore merges the absorbed contact's data into the primary contact
// in the local SQLite store. Call this after the Contacts.app merge has already
// been performed via AppleScript. Deduplicates phones by normalized number and
// emails by lowercase value; appends all addresses, URLs, and the note.
func (s *contactStore) MergeIntoStore(primaryID, absorbedID string) (*Contact, error) {
	primary, err := s.Get(primaryID)
	if err != nil || primary == nil {
		return nil, fmt.Errorf("primary contact not found: %s", primaryID)
	}
	absorbed, err := s.Get(absorbedID)
	if err != nil || absorbed == nil {
		return nil, fmt.Errorf("absorbed contact not found: %s", absorbedID)
	}

	// Deduplicated phone merge
	phoneSet := map[string]bool{}
	for _, ph := range primary.Phones {
		phoneSet[normalizePhone(ph.Value)] = true
	}
	for _, ph := range absorbed.Phones {
		if norm := normalizePhone(ph.Value); !phoneSet[norm] {
			primary.Phones = append(primary.Phones, ph)
			phoneSet[norm] = true
		}
	}

	// Deduplicated email merge
	emailSet := map[string]bool{}
	for _, em := range primary.Emails {
		emailSet[strings.ToLower(em.Value)] = true
	}
	for _, em := range absorbed.Emails {
		if !emailSet[strings.ToLower(em.Value)] {
			primary.Emails = append(primary.Emails, em)
			emailSet[strings.ToLower(em.Value)] = true
		}
	}

	// Append addresses and URLs without dedup (addresses are complex to compare)
	primary.Addresses = append(primary.Addresses, absorbed.Addresses...)
	primary.URLs = append(primary.URLs, absorbed.URLs...)

	// Append note (separated by a divider)
	if absorbed.Note != "" {
		if primary.Note == "" {
			primary.Note = absorbed.Note
		} else {
			primary.Note = primary.Note + "\n---\n" + absorbed.Note
		}
	}

	if err := s.UpsertOne(primary); err != nil {
		return nil, fmt.Errorf("upsert merged contact: %w", err)
	}
	if err := s.Delete(absorbedID); err != nil {
		return nil, fmt.Errorf("delete absorbed contact from store: %w", err)
	}
	return primary, nil
}

// UpsertOne inserts or replaces a single contact (used after create/update via AppleScript).
func (s *contactStore) UpsertOne(c *Contact) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	now := time.Now().UTC().Format(time.RFC3339)
	_, err = tx.Exec(
		`INSERT OR REPLACE INTO contacts (id, uuid, first_name, last_name, middle_name, organization, job_title, note, birthday, modified_at, synced_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.ID, extractUUID(c.ID), c.FirstName, c.LastName, c.MiddleName, c.Organization, c.JobTitle, c.Note, c.Birthday, c.ModifiedAt, now,
	)
	if err != nil {
		return err
	}
	for _, t := range []string{"contact_phones", "contact_emails", "contact_addresses", "contact_urls"} {
		if _, err := tx.Exec("DELETE FROM "+t+" WHERE contact_id = ?", c.ID); err != nil {
			return fmt.Errorf("upsert: delete %s: %w", t, err)
		}
	}
	if _, err := tx.Exec("DELETE FROM contacts_fts WHERE id = ?", c.ID); err != nil {
		return fmt.Errorf("upsert: delete fts: %w", err)
	}

	var phoneTokens, emailTokens []string
	for _, ph := range c.Phones {
		norm := normalizePhone(ph.Value)
		code, iso, country := resolvePhoneCountry(norm)
		if _, err := tx.Exec(
			`INSERT INTO contact_phones (contact_id, label, value, normalized, country_code, country_iso, country) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			c.ID, ph.Label, ph.Value, norm, code, iso, country,
		); err != nil {
			return fmt.Errorf("upsert: insert phone: %w", err)
		}
		phoneTokens = append(phoneTokens, ph.Value)
	}
	for _, em := range c.Emails {
		domain := ""
		if at := strings.LastIndex(em.Value, "@"); at >= 0 {
			domain = strings.ToLower(em.Value[at+1:])
		}
		if _, err := tx.Exec(
			`INSERT INTO contact_emails (contact_id, label, value, domain) VALUES (?, ?, ?, ?)`,
			c.ID, em.Label, em.Value, domain,
		); err != nil {
			return fmt.Errorf("upsert: insert email: %w", err)
		}
		emailTokens = append(emailTokens, em.Value)
	}
	for _, addr := range c.Addresses {
		if _, err := tx.Exec(
			`INSERT INTO contact_addresses (contact_id, label, street, city, state, zip, country) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			c.ID, addr.Label, addr.Street, addr.City, addr.State, addr.Zip, addr.Country,
		); err != nil {
			return fmt.Errorf("upsert: insert address: %w", err)
		}
	}
	for _, u := range c.URLs {
		if _, err := tx.Exec(
			`INSERT INTO contact_urls (contact_id, label, value) VALUES (?, ?, ?)`,
			c.ID, u.Label, u.Value,
		); err != nil {
			return fmt.Errorf("upsert: insert url: %w", err)
		}
	}

	body := strings.Join(filterEmpty(c.FirstName, c.MiddleName, c.LastName, c.Organization, c.JobTitle, c.Note,
		strings.Join(phoneTokens, " "), strings.Join(emailTokens, " ")), " ")
	if _, err := tx.Exec(`INSERT INTO contacts_fts (id, body) VALUES (?, ?)`, c.ID, body); err != nil {
		return fmt.Errorf("upsert: insert fts: %w", err)
	}

	return tx.Commit()
}

func (s *contactStore) loadRelations(c *Contact) error {
	// phones
	rows, err := s.db.Query(
		`SELECT label, value, country_code, country_iso, country FROM contact_phones WHERE contact_id = ?`, c.ID,
	)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var ph ContactPhone
		var code, iso, country sql.NullString
		if err := rows.Scan(&ph.Label, &ph.Value, &code, &iso, &country); err != nil {
			return err
		}
		ph.CountryCode, ph.CountryISO, ph.Country = code.String, iso.String, country.String
		c.Phones = append(c.Phones, ph)
	}
	rows.Close()

	// emails
	rows, err = s.db.Query(
		`SELECT label, value, domain FROM contact_emails WHERE contact_id = ?`, c.ID,
	)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var em ContactEmail
		var domain sql.NullString
		if err := rows.Scan(&em.Label, &em.Value, &domain); err != nil {
			return err
		}
		em.Domain = domain.String
		c.Emails = append(c.Emails, em)
	}
	rows.Close()

	// addresses
	rows, err = s.db.Query(
		`SELECT label, street, city, state, zip, country FROM contact_addresses WHERE contact_id = ?`, c.ID,
	)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var a ContactAddress
		var street, city, state, zip, country sql.NullString
		if err := rows.Scan(&a.Label, &street, &city, &state, &zip, &country); err != nil {
			return err
		}
		a.Street, a.City, a.State, a.Zip, a.Country = street.String, city.String, state.String, zip.String, country.String
		c.Addresses = append(c.Addresses, a)
	}
	rows.Close()

	// urls
	rows, err = s.db.Query(
		`SELECT label, value FROM contact_urls WHERE contact_id = ?`, c.ID,
	)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var u ContactURL
		if err := rows.Scan(&u.Label, &u.Value); err != nil {
			return err
		}
		c.URLs = append(c.URLs, u)
	}
	return rows.Err()
}

// ── analytics queries ─────────────────────────────────────────────────────────

type CountryCount struct {
	Country string `json:"country"`
	ISO     string `json:"iso"`
	Code    string `json:"country_code"`
	Count   int    `json:"contacts"`
	Phones  int    `json:"phones"`
}

func (s *contactStore) AnalyticsCountries() ([]CountryCount, error) {
	rows, err := s.db.Query(`
		SELECT country, country_iso, country_code,
		       COUNT(DISTINCT contact_id) AS contacts,
		       COUNT(*) AS phones
		FROM contact_phones
		WHERE country != '' AND country IS NOT NULL
		GROUP BY country_iso
		ORDER BY contacts DESC, phones DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CountryCount
	for rows.Next() {
		var r CountryCount
		var iso, code sql.NullString
		if err := rows.Scan(&r.Country, &iso, &code, &r.Count, &r.Phones); err != nil {
			return nil, err
		}
		r.ISO = iso.String
		r.Code = code.String
		out = append(out, r)
	}
	return out, rows.Err()
}

type DomainCount struct {
	Domain string `json:"domain"`
	Count  int    `json:"contacts"`
}

func (s *contactStore) AnalyticsDomains(limit int) ([]DomainCount, error) {
	if limit <= 0 {
		limit = 25
	}
	rows, err := s.db.Query(`
		SELECT domain, COUNT(DISTINCT contact_id) AS contacts
		FROM contact_emails
		WHERE domain != '' AND domain IS NOT NULL
		GROUP BY domain
		ORDER BY contacts DESC
		LIMIT ?`, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DomainCount
	for rows.Next() {
		var r DomainCount
		if err := rows.Scan(&r.Domain, &r.Count); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

type MissingStats struct {
	Total    int `json:"total"`
	NoPhone  int `json:"no_phone"`
	NoEmail  int `json:"no_email"`
	NoOrg    int `json:"no_org"`
	NoName   int `json:"no_name"`
}

func (s *contactStore) AnalyticsMissing() (*MissingStats, error) {
	var ms MissingStats
	err := s.db.QueryRow(`
		SELECT
		  COUNT(*) AS total,
		  SUM(CASE WHEN id NOT IN (SELECT DISTINCT contact_id FROM contact_phones) THEN 1 ELSE 0 END) AS no_phone,
		  SUM(CASE WHEN id NOT IN (SELECT DISTINCT contact_id FROM contact_emails) THEN 1 ELSE 0 END) AS no_email,
		  SUM(CASE WHEN organization IS NULL OR organization = '' THEN 1 ELSE 0 END) AS no_org,
		  SUM(CASE WHEN (first_name IS NULL OR first_name = '') AND (last_name IS NULL OR last_name = '') THEN 1 ELSE 0 END) AS no_name
		FROM contacts
	`).Scan(&ms.Total, &ms.NoPhone, &ms.NoEmail, &ms.NoOrg, &ms.NoName)
	return &ms, err
}

// ── phone country resolver ────────────────────────────────────────────────────

type dialEntry struct {
	prefix  string
	iso     string
	country string
}

// dialPrefixes is sorted longest-first at init time for correct prefix matching.
var dialPrefixes []dialEntry

func init() {
	raw := []dialEntry{
		// 4-digit NANP exceptions (must come before "+1")
		{"+1242", "BS", "Bahamas"}, {"+1246", "BB", "Barbados"}, {"+1264", "AI", "Anguilla"},
		{"+1268", "AG", "Antigua and Barbuda"}, {"+1284", "VG", "British Virgin Islands"},
		{"+1340", "VI", "US Virgin Islands"}, {"+1345", "KY", "Cayman Islands"},
		{"+1441", "BM", "Bermuda"}, {"+1473", "GD", "Grenada"}, {"+1649", "TC", "Turks and Caicos"},
		{"+1664", "MS", "Montserrat"}, {"+1670", "MP", "N. Mariana Islands"}, {"+1671", "GU", "Guam"},
		{"+1684", "AS", "American Samoa"}, {"+1721", "SX", "Sint Maarten"},
		{"+1758", "LC", "Saint Lucia"}, {"+1767", "DM", "Dominica"},
		{"+1784", "VC", "St. Vincent and the Grenadines"}, {"+1787", "PR", "Puerto Rico"},
		{"+1809", "DO", "Dominican Republic"}, {"+1829", "DO", "Dominican Republic"},
		{"+1849", "DO", "Dominican Republic"}, {"+1868", "TT", "Trinidad and Tobago"},
		{"+1869", "KN", "Saint Kitts and Nevis"}, {"+1876", "JM", "Jamaica"},
		{"+1939", "PR", "Puerto Rico"},
		// 3-digit
		{"+355", "AL", "Albania"}, {"+356", "MT", "Malta"}, {"+357", "CY", "Cyprus"},
		{"+358", "FI", "Finland"}, {"+359", "BG", "Bulgaria"}, {"+370", "LT", "Lithuania"},
		{"+371", "LV", "Latvia"}, {"+372", "EE", "Estonia"}, {"+373", "MD", "Moldova"},
		{"+374", "AM", "Armenia"}, {"+375", "BY", "Belarus"}, {"+376", "AD", "Andorra"},
		{"+377", "MC", "Monaco"}, {"+378", "SM", "San Marino"}, {"+380", "UA", "Ukraine"},
		{"+381", "RS", "Serbia"}, {"+382", "ME", "Montenegro"}, {"+385", "HR", "Croatia"},
		{"+386", "SI", "Slovenia"}, {"+387", "BA", "Bosnia and Herzegovina"},
		{"+389", "MK", "North Macedonia"}, {"+420", "CZ", "Czech Republic"},
		{"+421", "SK", "Slovakia"}, {"+423", "LI", "Liechtenstein"},
		{"+501", "BZ", "Belize"}, {"+502", "GT", "Guatemala"}, {"+503", "SV", "El Salvador"},
		{"+504", "HN", "Honduras"}, {"+505", "NI", "Nicaragua"}, {"+506", "CR", "Costa Rica"},
		{"+507", "PA", "Panama"}, {"+509", "HT", "Haiti"}, {"+590", "GP", "Guadeloupe"},
		{"+591", "BO", "Bolivia"}, {"+592", "GY", "Guyana"}, {"+593", "EC", "Ecuador"},
		{"+595", "PY", "Paraguay"}, {"+597", "SR", "Suriname"}, {"+598", "UY", "Uruguay"},
		{"+673", "BN", "Brunei"}, {"+675", "PG", "Papua New Guinea"}, {"+679", "FJ", "Fiji"},
		{"+852", "HK", "Hong Kong"}, {"+853", "MO", "Macau"}, {"+855", "KH", "Cambodia"},
		{"+856", "LA", "Laos"}, {"+880", "BD", "Bangladesh"}, {"+886", "TW", "Taiwan"},
		{"+960", "MV", "Maldives"}, {"+961", "LB", "Lebanon"}, {"+962", "JO", "Jordan"},
		{"+963", "SY", "Syria"}, {"+964", "IQ", "Iraq"}, {"+965", "KW", "Kuwait"},
		{"+966", "SA", "Saudi Arabia"}, {"+967", "YE", "Yemen"}, {"+968", "OM", "Oman"},
		{"+971", "AE", "United Arab Emirates"}, {"+972", "IL", "Israel"},
		{"+973", "BH", "Bahrain"}, {"+974", "QA", "Qatar"}, {"+975", "BT", "Bhutan"},
		{"+976", "MN", "Mongolia"}, {"+977", "NP", "Nepal"}, {"+992", "TJ", "Tajikistan"},
		{"+993", "TM", "Turkmenistan"}, {"+994", "AZ", "Azerbaijan"}, {"+995", "GE", "Georgia"},
		{"+996", "KG", "Kyrgyzstan"}, {"+998", "UZ", "Uzbekistan"},
		// 2-digit
		{"+20", "EG", "Egypt"}, {"+27", "ZA", "South Africa"}, {"+30", "GR", "Greece"},
		{"+31", "NL", "Netherlands"}, {"+32", "BE", "Belgium"}, {"+33", "FR", "France"},
		{"+34", "ES", "Spain"}, {"+36", "HU", "Hungary"}, {"+39", "IT", "Italy"},
		{"+40", "RO", "Romania"}, {"+41", "CH", "Switzerland"}, {"+43", "AT", "Austria"},
		{"+44", "GB", "United Kingdom"}, {"+45", "DK", "Denmark"}, {"+46", "SE", "Sweden"},
		{"+47", "NO", "Norway"}, {"+48", "PL", "Poland"}, {"+49", "DE", "Germany"},
		{"+51", "PE", "Peru"}, {"+52", "MX", "Mexico"}, {"+53", "CU", "Cuba"},
		{"+54", "AR", "Argentina"}, {"+55", "BR", "Brazil"}, {"+56", "CL", "Chile"},
		{"+57", "CO", "Colombia"}, {"+58", "VE", "Venezuela"}, {"+60", "MY", "Malaysia"},
		{"+61", "AU", "Australia"}, {"+62", "ID", "Indonesia"}, {"+63", "PH", "Philippines"},
		{"+64", "NZ", "New Zealand"}, {"+65", "SG", "Singapore"}, {"+66", "TH", "Thailand"},
		{"+81", "JP", "Japan"}, {"+82", "KR", "South Korea"}, {"+84", "VN", "Vietnam"},
		{"+86", "CN", "China"}, {"+90", "TR", "Turkey"}, {"+91", "IN", "India"},
		{"+92", "PK", "Pakistan"}, {"+93", "AF", "Afghanistan"}, {"+94", "LK", "Sri Lanka"},
		{"+95", "MM", "Myanmar"}, {"+98", "IR", "Iran"},
		// 1-digit (catch-all, must be last)
		{"+1", "US", "United States / Canada"}, {"+7", "RU", "Russia"},
	}
	// Sort longest prefix first so we match most-specific first.
	sort.Slice(raw, func(i, j int) bool {
		return len(raw[i].prefix) > len(raw[j].prefix)
	})
	dialPrefixes = raw
}

func normalizePhone(raw string) string {
	raw = strings.TrimSpace(raw)
	var b strings.Builder
	for i, ch := range raw {
		if ch == '+' && i == 0 {
			b.WriteRune(ch)
		} else if ch >= '0' && ch <= '9' {
			b.WriteRune(ch)
		}
	}
	return b.String()
}

// resolvePhoneCountry returns (dialCode, ISO2, countryName) for a normalized phone number.
func resolvePhoneCountry(normalized string) (code, iso, country string) {
	if !strings.HasPrefix(normalized, "+") {
		return "", "", ""
	}
	for _, e := range dialPrefixes {
		if strings.HasPrefix(normalized, e.prefix) {
			return e.prefix, e.iso, e.country
		}
	}
	return "", "", ""
}

// cleanLabel converts Apple's internal label format "_$!<Mobile>!$_" → "mobile".
func cleanLabel(label string) string {
	if strings.HasPrefix(label, "_$!<") && strings.HasSuffix(label, ">!$_") {
		label = label[4 : len(label)-4]
	}
	return strings.ToLower(strings.TrimSpace(label))
}

// ── helpers ───────────────────────────────────────────────────────────────────

func filterEmpty(ss ...string) []string {
	var out []string
	for _, s := range ss {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
