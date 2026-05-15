// Copyright 2026 yaooooooooooooooo. Licensed under Apache-2.0. See LICENSE.
// Hand-written novel store — not generated.

// Package store is the local SQLite mirror of the Operon API. It backs the
// `sync`, `demand stale`, `demand health`, `placement replay`, `placement
// watch`, `auction explain`, `campaign trust-history`, and `campaign
// group-by-wallet` commands. The store is a pure-Go SQLite (no CGO) so the
// CLI cross-compiles cleanly on the same toolchain that builds every other
// command.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// Store wraps a *sql.DB with per-resource helpers.
type Store struct {
	db   *sql.DB
	path string
}

// Open opens a SQLite database at path, creating parent directories and
// running the migrations if needed. The caller is responsible for Close.
func Open(ctx context.Context, path string) (*Store, error) {
	if path == "" {
		return nil, errors.New("store: empty path")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("store: mkdir parent: %w", err)
	}
	// modernc.org/sqlite uses the "sqlite" driver name (not "sqlite3").
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("store: open %s: %w", path, err)
	}
	// Single connection — SQLite is single-writer and our access pattern is
	// always serial per Store. This also avoids "database is locked" on
	// Windows where the pool would otherwise rotate handles unpredictably.
	db.SetMaxOpenConns(1)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("store: ping: %w", err)
	}
	if _, err := db.ExecContext(ctx, Schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("store: migrate: %w", err)
	}
	return &Store{db: db, path: path}, nil
}

// DefaultPath returns the conventional store location for the named CLI.
// Falls back to the working dir if UserCacheDir is unavailable.
func DefaultPath(cliName string) string {
	dir, err := os.UserCacheDir()
	if err != nil || dir == "" {
		dir = "."
	}
	return filepath.Join(dir, cliName, "store.db")
}

// DB exposes the underlying handle for callers that need raw SQL access
// (debug, ad-hoc inspection). Most callers should use the per-resource
// helpers.
func (s *Store) DB() *sql.DB { return s.db }

// Path returns the on-disk location of this store.
func (s *Store) Path() string { return s.path }

// Close releases the database handle. Safe to call on nil.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}
