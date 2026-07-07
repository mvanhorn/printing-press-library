// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored ht-ml.app data layer. ht-ml.app is accountless: there is no
// list endpoint and the per-site update_key is returned only once at creation.
// This file is the local system-of-record that makes the registry, key
// recovery, and version history possible. Survives `generate --force`.

package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// SiteRecord is the joined view of a published ht-ml.app site: the redacted
// registry metadata (mirrored into the generated "sites" table so framework
// search/sql see it) plus the secret update_key and password (kept out of the
// searchable store, in ht_secrets).
type SiteRecord struct {
	SiteID    string `json:"site_id"`
	URL       string `json:"url"`
	Status    string `json:"status"`
	Title     string `json:"title,omitempty"`
	Alias     string `json:"alias,omitempty"`
	UpdateKey string `json:"update_key,omitempty"`
	Password  string `json:"password,omitempty"`
	HasKey    bool   `json:"has_key"`
	CreatedAt string `json:"created_at,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

// SiteVersion is one stored HTML snapshot of a site.
type SiteVersion struct {
	SiteID    string `json:"site_id"`
	Version   int    `json:"version"`
	HTML      string `json:"html"`
	CreatedAt string `json:"created_at"`
}

// AssetRef is a referenced asset path for a site (the API's parse of the HTML).
type AssetRef struct {
	SiteID       string `json:"site_id"`
	RelativePath string `json:"relative_path"`
	AssetType    string `json:"asset_type,omitempty"`
}

// EnsureHTMLSchema creates the ht-ml.app side tables if they do not exist.
// Secrets, version history, and referenced-asset paths live here; the redacted
// site registry lives in the generated "sites" table (via UpsertSites) so the
// framework search/sql commands can see it without exposing the update_key.
func (s *Store) EnsureHTMLSchema() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS ht_secrets (
			site_id    TEXT PRIMARY KEY,
			update_key TEXT,
			password   TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS ht_versions (
			site_id    TEXT NOT NULL,
			version    INTEGER NOT NULL,
			html       TEXT NOT NULL,
			created_at TEXT NOT NULL,
			PRIMARY KEY (site_id, version)
		)`,
		`CREATE TABLE IF NOT EXISTS ht_assets (
			site_id       TEXT NOT NULL,
			relative_path TEXT NOT NULL,
			asset_type    TEXT,
			PRIMARY KEY (site_id, relative_path)
		)`,
	}
	for _, q := range stmts {
		if _, err := s.db.Exec(q); err != nil {
			return fmt.Errorf("ht-ml schema: %w", err)
		}
	}
	return nil
}

// SaveSite upserts a site: the redacted registry row (generated sites table),
// the secret update_key/password (ht_secrets), and, when html != "" and
// newVersion is true, a new ht_versions snapshot. created_at is preserved
// across updates; updated_at is always refreshed.
func (s *Store) SaveSite(rec SiteRecord, html string, newVersion bool) error {
	now := time.Now().UTC().Format(time.RFC3339)
	created := rec.CreatedAt
	if created == "" {
		if existing, _ := s.GetSite(rec.SiteID); existing != nil && existing.CreatedAt != "" {
			created = existing.CreatedAt
		} else {
			created = now
		}
	}

	// Redacted registry mirror (no secrets). id == site_id so the generated
	// extractObjectID resolves the storage key.
	registry := map[string]any{
		"id":         rec.SiteID,
		"site_id":    rec.SiteID,
		"url":        rec.URL,
		"status":     rec.Status,
		"title":      rec.Title,
		"alias":      rec.Alias,
		"created_at": created,
		"updated_at": now,
	}
	data, err := json.Marshal(registry)
	if err != nil {
		return err
	}
	if err := s.UpsertSites(data); err != nil {
		return err
	}

	if _, err := s.db.Exec(
		`INSERT INTO ht_secrets (site_id, update_key, password)
		 VALUES (?, ?, ?)
		 ON CONFLICT(site_id) DO UPDATE SET
		   update_key = COALESCE(NULLIF(excluded.update_key, ''), ht_secrets.update_key),
		   password   = excluded.password`,
		rec.SiteID, rec.UpdateKey, rec.Password,
	); err != nil {
		return fmt.Errorf("save secret: %w", err)
	}

	if newVersion && html != "" {
		if err := s.appendSiteVersion(rec.SiteID, html, now); err != nil {
			return err
		}
	}
	return nil
}

// appendSiteVersion computes the next version number and inserts the HTML
// snapshot atomically. The MAX(version) read and the INSERT are wrapped in a
// single transaction held under writeMu so two concurrent callers (e.g. a
// rapid publish followed by a triggered rollback) cannot read the same MAX and
// both attempt the same (site_id, version) pair — which the INSERT OR IGNORE
// would otherwise resolve by silently dropping one snapshot. This mirrors the
// writeMu + transaction pattern used by the generated store writers.
func (s *Store) appendSiteVersion(siteID, html, createdAt string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var maxV sql.NullInt64
	if err := tx.QueryRow(`SELECT MAX(version) FROM ht_versions WHERE site_id = ?`, siteID).Scan(&maxV); err != nil {
		return fmt.Errorf("version lookup: %w", err)
	}
	next := int(maxV.Int64) + 1
	if _, err := tx.Exec(
		`INSERT OR IGNORE INTO ht_versions (site_id, version, html, created_at) VALUES (?, ?, ?, ?)`,
		siteID, next, html, createdAt,
	); err != nil {
		return fmt.Errorf("save version: %w", err)
	}
	return tx.Commit()
}

// GetSite returns the joined registry + secret record for a site, or (nil, nil)
// when the site is not in the local store.
func (s *Store) GetSite(siteID string) (*SiteRecord, error) {
	var data sql.NullString
	err := s.db.QueryRow(`SELECT data FROM "sites" WHERE site_id = ?`, siteID).Scan(&data)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	rec := &SiteRecord{SiteID: siteID}
	if data.Valid && data.String != "" {
		var m map[string]any
		if json.Unmarshal([]byte(data.String), &m) == nil {
			rec.URL, _ = m["url"].(string)
			rec.Status, _ = m["status"].(string)
			rec.Title, _ = m["title"].(string)
			rec.Alias, _ = m["alias"].(string)
			rec.CreatedAt, _ = m["created_at"].(string)
			rec.UpdatedAt, _ = m["updated_at"].(string)
		}
	}
	var key, pw sql.NullString
	if err := s.db.QueryRow(`SELECT update_key, password FROM ht_secrets WHERE site_id = ?`, siteID).Scan(&key, &pw); err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	rec.UpdateKey = key.String
	rec.Password = pw.String
	rec.HasKey = key.Valid && key.String != ""
	return rec, nil
}

// GetUpdateKey returns the stored per-site update_key, or "" when absent.
func (s *Store) GetUpdateKey(siteID string) (string, error) {
	var key sql.NullString
	err := s.db.QueryRow(`SELECT update_key FROM ht_secrets WHERE site_id = ?`, siteID).Scan(&key)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return key.String, nil
}

// ListSites returns every site in the local registry, newest first. The
// UpdateKey field is left empty; use GetSite or AllSecrets for key material.
func (s *Store) ListSites() ([]SiteRecord, error) {
	rows, err := s.db.Query(`SELECT data FROM "sites"`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SiteRecord
	for rows.Next() {
		var data sql.NullString
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		if !data.Valid || data.String == "" {
			continue
		}
		var m map[string]any
		if json.Unmarshal([]byte(data.String), &m) != nil {
			continue
		}
		rec := SiteRecord{}
		rec.SiteID, _ = m["site_id"].(string)
		if rec.SiteID == "" {
			rec.SiteID, _ = m["id"].(string)
		}
		rec.URL, _ = m["url"].(string)
		rec.Status, _ = m["status"].(string)
		rec.Title, _ = m["title"].(string)
		rec.Alias, _ = m["alias"].(string)
		rec.CreatedAt, _ = m["created_at"].(string)
		rec.UpdatedAt, _ = m["updated_at"].(string)
		if rec.SiteID != "" {
			out = append(out, rec)
		}
	}
	// Mark which sites still have a recoverable key.
	for i := range out {
		if k, _ := s.GetUpdateKey(out[i].SiteID); k != "" {
			out[i].HasKey = true
		}
	}
	return out, rows.Err()
}

// AllSecrets returns every site's full record including the secret update_key
// and password, joined with registry url/title. Used by `keys export`.
func (s *Store) AllSecrets() ([]SiteRecord, error) {
	rows, err := s.db.Query(`SELECT site_id, update_key, password FROM ht_secrets`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SiteRecord
	for rows.Next() {
		var id string
		var key, pw sql.NullString
		if err := rows.Scan(&id, &key, &pw); err != nil {
			return nil, err
		}
		rec := SiteRecord{SiteID: id, UpdateKey: key.String, Password: pw.String, HasKey: key.Valid && key.String != ""}
		if site, _ := s.GetSite(id); site != nil {
			rec.URL = site.URL
			rec.Title = site.Title
			rec.Status = site.Status
			rec.Alias = site.Alias
			rec.CreatedAt = site.CreatedAt
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

// ResolveAlias returns the site_id bound to a local alias, or "" when unbound.
func (s *Store) ResolveAlias(alias string) (string, error) {
	var id string
	err := s.db.QueryRow(`SELECT site_id FROM "sites" WHERE json_extract(data, '$.alias') = ?`, alias).Scan(&id)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return id, nil
}

// SaveAssets replaces the cached referenced-asset paths for a site.
func (s *Store) SaveAssets(siteID string, assets []AssetRef) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM ht_assets WHERE site_id = ?`, siteID); err != nil {
		return err
	}
	for _, a := range assets {
		if a.RelativePath == "" {
			continue
		}
		if _, err := tx.Exec(
			`INSERT OR REPLACE INTO ht_assets (site_id, relative_path, asset_type) VALUES (?, ?, ?)`,
			siteID, a.RelativePath, a.AssetType,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ListAssets returns the cached referenced-asset paths for a site.
func (s *Store) ListAssets(siteID string) ([]AssetRef, error) {
	rows, err := s.db.Query(`SELECT relative_path, asset_type FROM ht_assets WHERE site_id = ? ORDER BY relative_path`, siteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AssetRef
	for rows.Next() {
		a := AssetRef{SiteID: siteID}
		var at sql.NullString
		if err := rows.Scan(&a.RelativePath, &at); err != nil {
			return nil, err
		}
		a.AssetType = at.String
		out = append(out, a)
	}
	return out, rows.Err()
}

// AssetCount returns the number of cached referenced assets for a site.
func (s *Store) AssetCount(siteID string) (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM ht_assets WHERE site_id = ?`, siteID).Scan(&n)
	return n, err
}

// ListVersions returns a site's stored HTML snapshots, newest version first.
func (s *Store) ListVersions(siteID string) ([]SiteVersion, error) {
	rows, err := s.db.Query(`SELECT version, html, created_at FROM ht_versions WHERE site_id = ? ORDER BY version DESC`, siteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SiteVersion
	for rows.Next() {
		v := SiteVersion{SiteID: siteID}
		if err := rows.Scan(&v.Version, &v.HTML, &v.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// GetVersion returns a specific stored version, or (nil, nil) when absent.
func (s *Store) GetVersion(siteID string, version int) (*SiteVersion, error) {
	v := SiteVersion{SiteID: siteID, Version: version}
	err := s.db.QueryRow(`SELECT html, created_at FROM ht_versions WHERE site_id = ? AND version = ?`, siteID, version).Scan(&v.HTML, &v.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &v, nil
}
