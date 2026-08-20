// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

// Hand-authored store extensions for here.now novel features. This file is NOT
// generated and must survive a regeneration merge. It adds two auxiliary
// tables — claim_vault and publish_state — that back the `claims`, `publish
// dir`, and `publish resume` commands, plus typed accessors over them.
package store

import (
	"database/sql"
	"fmt"
)

// hereNowTablesSQL holds the idempotent DDL for the novel-feature tables.
// CREATE TABLE IF NOT EXISTS keeps EnsureHereNowTables safe to call on every
// command invocation.
var hereNowTablesSQL = []string{
	`CREATE TABLE IF NOT EXISTS claim_vault (
		slug TEXT PRIMARY KEY,
		claim_token TEXT,
		url TEXT,
		published_at TEXT,
		expires_at TEXT,
		claimed INTEGER NOT NULL DEFAULT 0
	)`,
	`CREATE TABLE IF NOT EXISTS publish_state (
		slug TEXT PRIMARY KEY,
		version_id TEXT,
		dir TEXT,
		uploads_json TEXT,
		finalized INTEGER NOT NULL DEFAULT 0,
		created_at TEXT
	)`,
}

// EnsureHereNowTables creates the novel-feature tables if they do not already
// exist. Idempotent: safe to call before every read or write.
func (s *Store) EnsureHereNowTables() error {
	for _, ddl := range hereNowTablesSQL {
		if _, err := s.db.Exec(ddl); err != nil {
			return fmt.Errorf("ensure here.now tables: %w", err)
		}
	}
	return nil
}

// ClaimRecord is a row in claim_vault: an anonymous publish's claim metadata.
type ClaimRecord struct {
	Slug        string
	ClaimToken  string
	URL         string
	PublishedAt string
	ExpiresAt   string
	Claimed     bool
}

// PublishStateRecord is a row in publish_state: a publish's upload progress.
type PublishStateRecord struct {
	Slug        string
	VersionID   string
	Dir         string
	UploadsJSON string
	Finalized   bool
	CreatedAt   string
}

// SaveClaim inserts or replaces a claim_vault row.
func (s *Store) SaveClaim(rec ClaimRecord) error {
	if err := s.EnsureHereNowTables(); err != nil {
		return err
	}
	claimed := 0
	if rec.Claimed {
		claimed = 1
	}
	_, err := s.db.Exec(
		`INSERT INTO claim_vault (slug, claim_token, url, published_at, expires_at, claimed)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(slug) DO UPDATE SET
		   claim_token = excluded.claim_token,
		   url = excluded.url,
		   published_at = excluded.published_at,
		   expires_at = excluded.expires_at,
		   claimed = excluded.claimed`,
		rec.Slug, rec.ClaimToken, rec.URL, rec.PublishedAt, rec.ExpiresAt, claimed,
	)
	if err != nil {
		return fmt.Errorf("save claim %q: %w", rec.Slug, err)
	}
	return nil
}

// ListClaims returns every claim_vault row ordered by expiry (soonest first).
func (s *Store) ListClaims() ([]ClaimRecord, error) {
	if err := s.EnsureHereNowTables(); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(
		`SELECT slug, claim_token, url, published_at, expires_at, claimed
		 FROM claim_vault ORDER BY expires_at ASC, slug ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list claims: %w", err)
	}
	defer rows.Close()
	return scanClaims(rows)
}

// GetClaim returns the claim_vault row for slug, or (nil, nil) if none exists.
func (s *Store) GetClaim(slug string) (*ClaimRecord, error) {
	if err := s.EnsureHereNowTables(); err != nil {
		return nil, err
	}
	row := s.db.QueryRow(
		`SELECT slug, claim_token, url, published_at, expires_at, claimed
		 FROM claim_vault WHERE slug = ?`,
		slug,
	)
	rec, err := scanClaim(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get claim %q: %w", slug, err)
	}
	return rec, nil
}

// MarkClaimed sets claimed=1 for slug. A missing slug is not an error.
func (s *Store) MarkClaimed(slug string) error {
	if err := s.EnsureHereNowTables(); err != nil {
		return err
	}
	if _, err := s.db.Exec(`UPDATE claim_vault SET claimed = 1 WHERE slug = ?`, slug); err != nil {
		return fmt.Errorf("mark claimed %q: %w", slug, err)
	}
	return nil
}

// SavePublishState inserts or replaces a publish_state row.
func (s *Store) SavePublishState(rec PublishStateRecord) error {
	if err := s.EnsureHereNowTables(); err != nil {
		return err
	}
	finalized := 0
	if rec.Finalized {
		finalized = 1
	}
	_, err := s.db.Exec(
		`INSERT INTO publish_state (slug, version_id, dir, uploads_json, finalized, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(slug) DO UPDATE SET
		   version_id = excluded.version_id,
		   dir = excluded.dir,
		   uploads_json = excluded.uploads_json,
		   finalized = excluded.finalized,
		   created_at = excluded.created_at`,
		rec.Slug, rec.VersionID, rec.Dir, rec.UploadsJSON, finalized, rec.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("save publish state %q: %w", rec.Slug, err)
	}
	return nil
}

// GetPublishState returns the publish_state row for slug, or (nil, nil) if none.
func (s *Store) GetPublishState(slug string) (*PublishStateRecord, error) {
	if err := s.EnsureHereNowTables(); err != nil {
		return nil, err
	}
	row := s.db.QueryRow(
		`SELECT slug, version_id, dir, uploads_json, finalized, created_at
		 FROM publish_state WHERE slug = ?`,
		slug,
	)
	var (
		rec       PublishStateRecord
		versionID sql.NullString
		dir       sql.NullString
		uploads   sql.NullString
		createdAt sql.NullString
		finalized int
	)
	err := row.Scan(&rec.Slug, &versionID, &dir, &uploads, &finalized, &createdAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get publish state %q: %w", slug, err)
	}
	rec.VersionID = versionID.String
	rec.Dir = dir.String
	rec.UploadsJSON = uploads.String
	rec.CreatedAt = createdAt.String
	rec.Finalized = finalized != 0
	return &rec, nil
}

// MarkFinalized sets finalized=1 for slug. A missing slug is not an error.
func (s *Store) MarkFinalized(slug string) error {
	if err := s.EnsureHereNowTables(); err != nil {
		return err
	}
	if _, err := s.db.Exec(`UPDATE publish_state SET finalized = 1 WHERE slug = ?`, slug); err != nil {
		return fmt.Errorf("mark finalized %q: %w", slug, err)
	}
	return nil
}

// RecentPublishCount returns the number of publish_state rows created at or
// after the given RFC3339 timestamp. Used by the usage meter's publish cadence.
func (s *Store) RecentPublishCount(sinceRFC3339 string) (int, error) {
	if err := s.EnsureHereNowTables(); err != nil {
		return 0, err
	}
	var n int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM publish_state WHERE created_at >= ?`,
		sinceRFC3339,
	).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("recent publish count: %w", err)
	}
	return n, nil
}

func scanClaims(rows *sql.Rows) ([]ClaimRecord, error) {
	var out []ClaimRecord
	for rows.Next() {
		rec, err := scanClaim(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *rec)
	}
	return out, rows.Err()
}

// rowScanner is satisfied by both *sql.Row and *sql.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanClaim(r rowScanner) (*ClaimRecord, error) {
	var (
		rec         ClaimRecord
		claimToken  sql.NullString
		url         sql.NullString
		publishedAt sql.NullString
		expiresAt   sql.NullString
		claimed     int
	)
	if err := r.Scan(&rec.Slug, &claimToken, &url, &publishedAt, &expiresAt, &claimed); err != nil {
		return nil, err
	}
	rec.ClaimToken = claimToken.String
	rec.URL = url.String
	rec.PublishedAt = publishedAt.String
	rec.ExpiresAt = expiresAt.String
	rec.Claimed = claimed != 0
	return &rec, nil
}
