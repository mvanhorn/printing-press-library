// Copyright 2026 nicolas-correa. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/commerce/iqos/internal/store"
)

// iqosMigrate creates IQOS-specific tables in the existing SQLite database.
// Uses IF NOT EXISTS so it is safe to call on every command that needs the tables.
func iqosMigrate(ctx context.Context, db *store.Store) error {
	migrations := []string{
		// Products per country: slug is the unique product identifier within a country
		`CREATE TABLE IF NOT EXISTS iqos_products (
			slug     TEXT NOT NULL,
			country  TEXT NOT NULL,
			name     TEXT,
			url      TEXT,
			category TEXT,
			price    TEXT,
			data     TEXT NOT NULL DEFAULT '{}',
			synced_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (slug, country)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_iqos_products_country ON iqos_products(country)`,
		`CREATE INDEX IF NOT EXISTS idx_iqos_products_category ON iqos_products(category)`,
		`CREATE INDEX IF NOT EXISTS idx_iqos_products_synced ON iqos_products(synced_at)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS iqos_products_fts USING fts5(
			slug, country, name, category, data,
			tokenize='porter unicode61'
		)`,
		// Store locations with GPS coordinates (for nearest-store query)
		`CREATE TABLE IF NOT EXISTS iqos_stores (
			id      TEXT NOT NULL,
			country TEXT NOT NULL,
			name    TEXT,
			address TEXT,
			city    TEXT,
			lat     REAL,
			lon     REAL,
			data    TEXT NOT NULL DEFAULT '{}',
			synced_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (id, country)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_iqos_stores_country ON iqos_stores(country)`,
		// Snapshots table for change detection (one row per product+country per sync run)
		`CREATE TABLE IF NOT EXISTS iqos_snapshots (
			slug       TEXT NOT NULL,
			country    TEXT NOT NULL,
			price      TEXT,
			present    INTEGER NOT NULL DEFAULT 1,
			snapshot_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (slug, country, snapshot_at)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_iqos_snapshots_at ON iqos_snapshots(snapshot_at)`,
	}
	rawDB := db.DB()
	for _, m := range migrations {
		if _, err := rawDB.ExecContext(ctx, m); err != nil {
			return fmt.Errorf("iqos migration: %w", err)
		}
	}
	return nil
}

// iqosOpenDB opens the default IQOS store and runs schema migration.
func iqosOpenDB(ctx context.Context) (*store.Store, error) {
	dbPath := defaultDBPath("iqos-pp-cli")
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}
	if err := iqosMigrate(ctx, db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}
