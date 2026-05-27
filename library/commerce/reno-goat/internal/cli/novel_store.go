// Copyright 2026 h179922. Licensed under Apache-2.0. See LICENSE.
// Novel command: local SQLite store for watch, project, saved, and history features.

package cli

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

// novelDBPath returns the path to the novel-features SQLite database.
// Separate from the sync store so novel tables don't interfere with
// the generator's store migrations.
func novelDBPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "reno-goat-pp-cli", "novel.db")
}

// openNovelDB opens (or creates) the novel-features database and runs
// all table migrations. Callers must defer db.Close().
func openNovelDB() (*sql.DB, error) {
	dbPath := novelDBPath()
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("creating novel db directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_synchronous=NORMAL&_busy_timeout=5000&_foreign_keys=ON")
	if err != nil {
		return nil, fmt.Errorf("opening novel database: %w", err)
	}

	if err := migrateNovelDB(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrating novel database: %w", err)
	}

	return db, nil
}

// migrateNovelDB creates all novel-feature tables if they don't exist.
func migrateNovelDB(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS watches (
			id INTEGER PRIMARY KEY,
			product_url TEXT UNIQUE NOT NULL,
			source TEXT NOT NULL,
			title TEXT,
			threshold_pct REAL DEFAULT 10.0,
			original_price REAL,
			current_price REAL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_checked DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS price_history (
			id INTEGER PRIMARY KEY,
			watch_id INTEGER REFERENCES watches(id) ON DELETE CASCADE,
			price REAL NOT NULL,
			on_sale BOOLEAN DEFAULT 0,
			checked_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS projects (
			id INTEGER PRIMARY KEY,
			name TEXT UNIQUE NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS project_items (
			id INTEGER PRIMARY KEY,
			project_id INTEGER REFERENCES projects(id) ON DELETE CASCADE,
			product_url TEXT NOT NULL,
			source TEXT NOT NULL,
			title TEXT,
			price REAL,
			quantity INTEGER DEFAULT 1,
			added_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(project_id, product_url)
		)`,
		`CREATE TABLE IF NOT EXISTS saved_products (
			id INTEGER PRIMARY KEY,
			product_url TEXT UNIQUE NOT NULL,
			source TEXT NOT NULL,
			title TEXT,
			price REAL,
			in_stock BOOLEAN DEFAULT 1,
			saved_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_checked DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS search_history (
			id INTEGER PRIMARY KEY,
			query TEXT NOT NULL,
			categories TEXT,
			sources_queried TEXT,
			result_count INTEGER,
			searched_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("executing migration: %w", err)
		}
	}
	return nil
}

// inferSource extracts a source identifier from a product URL based on known domains.
// For Shopify DTC stores, returns "shopify-dtc/<store-domain>" to match the
// normalizeShopify output in fan-out search results.
func inferSource(productURL string) string {
	// Shopify DTC: extract store domain from <store>.myshopify.com URLs
	// to produce "shopify-dtc/<store>" matching normalizeShopify's Source field.
	if idx := strings.Index(productURL, ".myshopify.com"); idx > 0 {
		// Walk backward from idx to find the subdomain start (after "://").
		prefix := productURL[:idx]
		if slashIdx := strings.LastIndex(prefix, "/"); slashIdx >= 0 {
			return "shopify-dtc/" + prefix[slashIdx+1:]
		}
	}

	sources := map[string]string{
		"fergusonhome.com":      "ferguson",
		"fergusonshowrooms.com": "ferguson",
		"ferguson.com":          "ferguson",
		"westelm.com":           "west-elm",
		"rejuvenation.com":      "rejuvenation",
		"article.com":           "article",
		"lowes.com":             "lowes",
		"homedepot.com":         "home-depot",
	}
	for domain, source := range sources {
		if containsDomain(productURL, domain) {
			return source
		}
	}
	return "unknown"
}

// truncateBody truncates a byte slice to a reasonable length for error messages.
// Used by novel commands that make HTTP requests and need to include response
// bodies in error strings without flooding logs.
func truncateBody(body []byte) string {
	const maxLen = 200
	if len(body) <= maxLen {
		return string(body)
	}
	return string(body[:maxLen]) + "..."
}

// containsDomain checks if a URL string contains the given domain.
func containsDomain(rawURL, domain string) bool {
	return len(rawURL) > 0 && (strings.Contains(rawURL, "://"+domain) ||
		strings.Contains(rawURL, "://www."+domain) ||
		strings.Contains(rawURL, "."+domain))
}
