// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

// Package vtstore provides VirusTotal-specific SQLite intelligence storage.
// Extends the base store package with IOC-specific tables and relationships.
package vtstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// VTStore wraps SQLite with VirusTotal IOC intelligence schema.
type VTStore struct {
	db   *sql.DB
	path string
}

const vtSchemaVersion = 1

// Open creates or opens the VT intelligence store at ~/.virustotal/cache.db
func Open() (*VTStore, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("getting home dir: %w", err)
	}
	dbPath := filepath.Join(homeDir, ".virustotal", "cache.db")
	return OpenPath(dbPath)
}

// OpenPath opens the VT store at a specific path
func OpenPath(dbPath string) (*VTStore, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("creating db directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_synchronous=NORMAL&_busy_timeout=5000&_foreign_keys=ON&_temp_store=MEMORY&_mmap_size=268435456")
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	db.SetMaxOpenConns(4)

	s := &VTStore{db: db, path: dbPath}
	if err := s.migrate(context.Background()); err != nil {
		db.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	return s, nil
}

func (s *VTStore) Close() error {
	return s.db.Close()
}

func (s *VTStore) Path() string {
	return s.path
}

func (s *VTStore) migrate(ctx context.Context) error {
	migrations := []string{
		// Files table
		`CREATE TABLE IF NOT EXISTS files (
			sha256 TEXT PRIMARY KEY,
			md5 TEXT,
			sha1 TEXT,
			size INTEGER,
			type_description TEXT,
			first_seen INTEGER,
			last_seen INTEGER,
			times_submitted INTEGER,
			malicious_votes INTEGER,
			harmless_votes INTEGER,
			detection_ratio TEXT,
			data JSON NOT NULL,
			fetched_at INTEGER DEFAULT (strftime('%s', 'now')),
			updated_at INTEGER DEFAULT (strftime('%s', 'now'))
		)`,
		`CREATE INDEX IF NOT EXISTS idx_files_md5 ON files(md5)`,
		`CREATE INDEX IF NOT EXISTS idx_files_sha1 ON files(sha1)`,
		`CREATE INDEX IF NOT EXISTS idx_files_first_seen ON files(first_seen)`,
		`CREATE INDEX IF NOT EXISTS idx_files_fetched ON files(fetched_at)`,

		// Domains table
		`CREATE TABLE IF NOT EXISTS domains (
			domain TEXT PRIMARY KEY,
			reputation INTEGER,
			categories TEXT,
			last_dns_records TEXT,
			creation_date INTEGER,
			last_update_date INTEGER,
			data JSON NOT NULL,
			fetched_at INTEGER DEFAULT (strftime('%s', 'now')),
			updated_at INTEGER DEFAULT (strftime('%s', 'now'))
		)`,
		`CREATE INDEX IF NOT EXISTS idx_domains_reputation ON domains(reputation)`,
		`CREATE INDEX IF NOT EXISTS idx_domains_fetched ON domains(fetched_at)`,

		// IP addresses table
		`CREATE TABLE IF NOT EXISTS ip_addresses (
			ip TEXT PRIMARY KEY,
			reputation INTEGER,
			asn INTEGER,
			as_owner TEXT,
			country TEXT,
			continent TEXT,
			data JSON NOT NULL,
			fetched_at INTEGER DEFAULT (strftime('%s', 'now')),
			updated_at INTEGER DEFAULT (strftime('%s', 'now'))
		)`,
		`CREATE INDEX IF NOT EXISTS idx_ips_country ON ip_addresses(country)`,
		`CREATE INDEX IF NOT EXISTS idx_ips_asn ON ip_addresses(asn)`,
		`CREATE INDEX IF NOT EXISTS idx_ips_fetched ON ip_addresses(fetched_at)`,

		// URLs table
		`CREATE TABLE IF NOT EXISTS urls (
			url_id TEXT PRIMARY KEY,
			url TEXT NOT NULL,
			scan_id TEXT,
			positives INTEGER,
			total INTEGER,
			scan_date INTEGER,
			data JSON NOT NULL,
			fetched_at INTEGER DEFAULT (strftime('%s', 'now')),
			updated_at INTEGER DEFAULT (strftime('%s', 'now'))
		)`,
		`CREATE INDEX IF NOT EXISTS idx_urls_scan_date ON urls(scan_date)`,
		`CREATE INDEX IF NOT EXISTS idx_urls_fetched ON urls(fetched_at)`,

		// Relationships table (pivot graph edges)
		`CREATE TABLE IF NOT EXISTS relationships (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			source_type TEXT NOT NULL,
			source_id TEXT NOT NULL,
			relationship_type TEXT NOT NULL,
			target_type TEXT NOT NULL,
			target_id TEXT NOT NULL,
			created_at INTEGER DEFAULT (strftime('%s', 'now')),
			UNIQUE(source_type, source_id, relationship_type, target_type, target_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_relationships_source ON relationships(source_type, source_id)`,
		`CREATE INDEX IF NOT EXISTS idx_relationships_target ON relationships(target_type, target_id)`,
		`CREATE INDEX IF NOT EXISTS idx_relationships_type ON relationships(relationship_type)`,

		// FTS5 search across all IOCs
		`CREATE VIRTUAL TABLE IF NOT EXISTS iocs_fts USING fts5(
			ioc_id, ioc_type, content, tokenize='porter unicode61'
		)`,

		// Schema version tracking
		`CREATE TABLE IF NOT EXISTS schema_meta (
			key TEXT PRIMARY KEY,
			value TEXT
		)`,
		fmt.Sprintf(`INSERT OR IGNORE INTO schema_meta (key, value) VALUES ('version', '%d')`, vtSchemaVersion),
	}

	for _, m := range migrations {
		if _, err := s.db.ExecContext(ctx, m); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
	}

	return nil
}

// FileReport represents a VirusTotal file report
type FileReport struct {
	SHA256           string         `json:"sha256"`
	MD5              string         `json:"md5"`
	SHA1             string         `json:"sha1"`
	Size             int64          `json:"size"`
	TypeDescription  string         `json:"type_description"`
	FirstSeen        int64          `json:"first_seen"`
	LastSeen         int64          `json:"last_seen"`
	TimesSubmitted   int            `json:"times_submitted"`
	MaliciousVotes   int            `json:"malicious_votes"`
	HarmlessVotes    int            `json:"harmless_votes"`
	DetectionRatio   string         `json:"detection_ratio"`
	Data             json.RawMessage `json:"data"`
}

// StoreFile upserts a file report
func (s *VTStore) StoreFile(report *FileReport) error {
	_, err := s.db.Exec(`
		INSERT INTO files (sha256, md5, sha1, size, type_description, first_seen, last_seen, times_submitted, malicious_votes, harmless_votes, detection_ratio, data, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, strftime('%s', 'now'))
		ON CONFLICT(sha256) DO UPDATE SET
			md5 = excluded.md5,
			sha1 = excluded.sha1,
			size = excluded.size,
			type_description = excluded.type_description,
			last_seen = excluded.last_seen,
			times_submitted = excluded.times_submitted,
			malicious_votes = excluded.malicious_votes,
			harmless_votes = excluded.harmless_votes,
			detection_ratio = excluded.detection_ratio,
			data = excluded.data,
			updated_at = strftime('%s', 'now')
	`, report.SHA256, report.MD5, report.SHA1, report.Size, report.TypeDescription,
		report.FirstSeen, report.LastSeen, report.TimesSubmitted,
		report.MaliciousVotes, report.HarmlessVotes, report.DetectionRatio, string(report.Data))

	if err != nil {
		return fmt.Errorf("storing file: %w", err)
	}

	// Update FTS index
	_ = s.updateFTS("file", report.SHA256, string(report.Data))

	return nil
}

// GetFile retrieves a file report by hash (supports SHA256, MD5, SHA1)
func (s *VTStore) GetFile(hash string) (*FileReport, error) {
	var report FileReport
	var data string

	query := `SELECT sha256, md5, sha1, size, type_description, first_seen, last_seen, times_submitted, malicious_votes, harmless_votes, detection_ratio, data
		FROM files WHERE sha256 = ? OR md5 = ? OR sha1 = ?`

	err := s.db.QueryRow(query, hash, hash, hash).Scan(
		&report.SHA256, &report.MD5, &report.SHA1, &report.Size, &report.TypeDescription,
		&report.FirstSeen, &report.LastSeen, &report.TimesSubmitted,
		&report.MaliciousVotes, &report.HarmlessVotes, &report.DetectionRatio, &data,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	report.Data = json.RawMessage(data)
	return &report, nil
}

// StoreDomain upserts a domain report
func (s *VTStore) StoreDomain(domain string, data json.RawMessage) error {
	// Parse key fields from JSON for indexed columns
	var parsed map[string]any
	json.Unmarshal(data, &parsed)

	reputation := 0
	if rep, ok := parsed["reputation"].(float64); ok {
		reputation = int(rep)
	}

	_, err := s.db.Exec(`
		INSERT INTO domains (domain, reputation, data, updated_at)
		VALUES (?, ?, ?, strftime('%s', 'now'))
		ON CONFLICT(domain) DO UPDATE SET
			reputation = excluded.reputation,
			data = excluded.data,
			updated_at = strftime('%s', 'now')
	`, domain, reputation, string(data))

	if err != nil {
		return fmt.Errorf("storing domain: %w", err)
	}

	_ = s.updateFTS("domain", domain, string(data))
	return nil
}

// GetDomain retrieves a domain report
func (s *VTStore) GetDomain(domain string) (json.RawMessage, error) {
	var data string
	err := s.db.QueryRow(`SELECT data FROM domains WHERE domain = ?`, domain).Scan(&data)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return json.RawMessage(data), nil
}

// StoreIP upserts an IP address report
func (s *VTStore) StoreIP(ip string, data json.RawMessage) error {
	var parsed map[string]any
	json.Unmarshal(data, &parsed)

	reputation := 0
	if rep, ok := parsed["reputation"].(float64); ok {
		reputation = int(rep)
	}

	_, err := s.db.Exec(`
		INSERT INTO ip_addresses (ip, reputation, data, updated_at)
		VALUES (?, ?, ?, strftime('%s', 'now'))
		ON CONFLICT(ip) DO UPDATE SET
			reputation = excluded.reputation,
			data = excluded.data,
			updated_at = strftime('%s', 'now')
	`, ip, reputation, string(data))

	if err != nil {
		return fmt.Errorf("storing IP: %w", err)
	}

	_ = s.updateFTS("ip", ip, string(data))
	return nil
}

// GetIP retrieves an IP address report
func (s *VTStore) GetIP(ip string) (json.RawMessage, error) {
	var data string
	err := s.db.QueryRow(`SELECT data FROM ip_addresses WHERE ip = ?`, ip).Scan(&data)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return json.RawMessage(data), nil
}

// StoreRelationship records a relationship edge for pivot workflows
func (s *VTStore) StoreRelationship(sourceType, sourceID, relType, targetType, targetID string) error {
	_, err := s.db.Exec(`
		INSERT OR IGNORE INTO relationships (source_type, source_id, relationship_type, target_type, target_id)
		VALUES (?, ?, ?, ?, ?)
	`, sourceType, sourceID, relType, targetType, targetID)
	return err
}

// GetRelationships retrieves all relationships for a given source
func (s *VTStore) GetRelationships(sourceType, sourceID string) ([]Relationship, error) {
	rows, err := s.db.Query(`
		SELECT relationship_type, target_type, target_id FROM relationships
		WHERE source_type = ? AND source_id = ?
	`, sourceType, sourceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rels []Relationship
	for rows.Next() {
		var rel Relationship
		if err := rows.Scan(&rel.Type, &rel.TargetType, &rel.TargetID); err != nil {
			continue
		}
		rels = append(rels, rel)
	}
	return rels, rows.Err()
}

// Relationship represents a graph edge
type Relationship struct {
	Type       string
	TargetType string
	TargetID   string
}

// SearchIOCs performs FTS5 search across all cached IOCs
func (s *VTStore) SearchIOCs(query string, limit int) ([]SearchResult, error) {
	if limit <= 0 {
		limit = 50
	}

	rows, err := s.db.Query(`
		SELECT ioc_type, ioc_id FROM iocs_fts
		WHERE iocs_fts MATCH ?
		ORDER BY rank
		LIMIT ?
	`, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var res SearchResult
		if err := rows.Scan(&res.Type, &res.ID); err != nil {
			continue
		}
		results = append(results, res)
	}
	return results, rows.Err()
}

// SearchResult represents an FTS5 match
type SearchResult struct {
	Type string
	ID   string
}

func (s *VTStore) updateFTS(iocType, iocID, content string) error {
	// Delete existing entry
	_, _ = s.db.Exec(`DELETE FROM iocs_fts WHERE ioc_type = ? AND ioc_id = ?`, iocType, iocID)

	// Insert new entry
	_, err := s.db.Exec(`
		INSERT INTO iocs_fts (ioc_type, ioc_id, content)
		VALUES (?, ?, ?)
	`, iocType, iocID, content)

	return err
}

// Stats returns cache statistics
func (s *VTStore) Stats() (map[string]int, error) {
	stats := make(map[string]int)

	tables := []string{"files", "domains", "ip_addresses", "urls", "relationships"}
	for _, table := range tables {
		var count int
		err := s.db.QueryRow(fmt.Sprintf(`SELECT COUNT(*) FROM %s`, table)).Scan(&count)
		if err != nil {
			return nil, err
		}
		stats[table] = count
	}

	return stats, nil
}

// GetFilesByDetectionRatio returns files with detection ratio above threshold
func (s *VTStore) GetFilesByDetectionRatio(minRatio float64) ([]*FileReport, error) {
	rows, err := s.db.Query(`
		SELECT sha256, md5, sha1, size, type_description, first_seen, last_seen,
			times_submitted, malicious_votes, harmless_votes, detection_ratio, data
		FROM files
		WHERE CAST(malicious_votes AS REAL) / NULLIF(malicious_votes + harmless_votes, 0) >= ?
		ORDER BY malicious_votes DESC
		LIMIT 100
	`, minRatio)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reports []*FileReport
	for rows.Next() {
		var r FileReport
		var data string
		if err := rows.Scan(&r.SHA256, &r.MD5, &r.SHA1, &r.Size, &r.TypeDescription,
			&r.FirstSeen, &r.LastSeen, &r.TimesSubmitted, &r.MaliciousVotes,
			&r.HarmlessVotes, &r.DetectionRatio, &data); err != nil {
			continue
		}
		r.Data = json.RawMessage(data)
		reports = append(reports, &r)
	}
	return reports, rows.Err()
}
