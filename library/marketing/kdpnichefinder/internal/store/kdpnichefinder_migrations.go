// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// EnsureKDPSchema idempotently applies the KDP-specific schema additions on
// top of the generated migrations: a `bucket` column on the typed `niches`
// table, and a `niche_snapshots` table that records one revenue/sales snapshot
// per book per day. Safe to call on every command invocation.
func (s *Store) EnsureKDPSchema(ctx context.Context) error {
	if err := s.ensureNichesBucketColumn(ctx); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS niche_snapshots (
		book_id TEXT NOT NULL,
		bucket TEXT NOT NULL DEFAULT '',
		captured_on TEXT NOT NULL,
		estimated_monthly_sales INTEGER,
		estimated_monthly_revenue REAL,
		price TEXT,
		title TEXT,
		PRIMARY KEY (book_id, bucket, captured_on)
	)`); err != nil {
		return fmt.Errorf("create niche_snapshots: %w", err)
	}
	return nil
}

// ensureNichesBucketColumn adds the bucket column to niches if it is not
// already present, guarded by a PRAGMA table_info check so re-runs are no-ops.
func (s *Store) ensureNichesBucketColumn(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `PRAGMA table_info("niches")`)
	if err != nil {
		return fmt.Errorf("table_info niches: %w", err)
	}
	defer rows.Close()
	hasTable := false
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			return fmt.Errorf("scan table_info niches: %w", err)
		}
		hasTable = true
		if name == "bucket" {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate table_info niches: %w", err)
	}
	if !hasTable {
		// The generated migrations create the niches table with the bucket
		// column absent; if for some reason it doesn't exist yet, create a
		// minimal table so the ALTER below isn't needed. The generated
		// migrate() (run by Open) will have created the full table already,
		// so this branch is defensive only.
		if _, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS "niches" (
			"id" TEXT PRIMARY KEY,
			"data" JSON NOT NULL,
			"synced_at" DATETIME DEFAULT CURRENT_TIMESTAMP,
			"title" TEXT,
			"amazon_url" TEXT,
			"image_url" TEXT,
			"price" TEXT,
			"publisher" TEXT,
			"estimated_monthly_sales" INTEGER,
			"estimated_monthly_revenue" REAL,
			"bucket" TEXT
		)`); err != nil {
			return fmt.Errorf("create niches: %w", err)
		}
		return nil
	}
	if _, err := s.db.ExecContext(ctx, `ALTER TABLE "niches" ADD COLUMN "bucket" TEXT`); err != nil {
		if strings.Contains(err.Error(), "duplicate column name") {
			return nil
		}
		return fmt.Errorf("add column niches.bucket: %w", err)
	}
	return nil
}

// UpsertNiche writes a single book into the typed niches table (including its
// bucket and the JSON data column) and the generic resources mirror so the
// existing search/list helpers keep working.
func (s *Store) UpsertNiche(id int, title, amazonURL, imageURL, price, publisher string, sales int, revenue float64, bucket string) error {
	idStr := strconv.Itoa(id)
	data, err := json.Marshal(map[string]any{
		"id":                        id,
		"title":                     title,
		"amazon_url":                amazonURL,
		"image_url":                 imageURL,
		"price":                     price,
		"publisher":                 publisher,
		"estimated_monthly_sales":   sales,
		"estimated_monthly_revenue": revenue,
		"bucket":                    bucket,
	})
	if err != nil {
		return fmt.Errorf("marshal niche: %w", err)
	}

	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := s.upsertGenericResourceTx(tx, "niches", idStr, json.RawMessage(data)); err != nil {
		return err
	}
	if _, err := tx.Exec(
		`INSERT INTO "niches" ("id", "data", "synced_at", "title", "amazon_url", "image_url", "price", "publisher", "estimated_monthly_sales", "estimated_monthly_revenue", "bucket")
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT("id") DO UPDATE SET "data" = excluded."data", "synced_at" = excluded."synced_at", "title" = excluded."title", "amazon_url" = excluded."amazon_url", "image_url" = excluded."image_url", "price" = excluded."price", "publisher" = excluded."publisher", "estimated_monthly_sales" = excluded."estimated_monthly_sales", "estimated_monthly_revenue" = excluded."estimated_monthly_revenue", "bucket" = excluded."bucket"`,
		idStr,
		string(data),
		time.Now().UTC().Format(time.RFC3339),
		title, amazonURL, imageURL, price, publisher, sales, revenue, bucket,
	); err != nil {
		return fmt.Errorf("insert into niches: %w", err)
	}
	return tx.Commit()
}

// RecordSnapshot records one daily snapshot for a book. capturedOn is a
// YYYY-MM-DD date string; re-running on the same day overwrites the row.
func (s *Store) RecordSnapshot(bookID, bucket, capturedOn string, sales int, revenue float64, price, title string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO niche_snapshots
		 (book_id, bucket, captured_on, estimated_monthly_sales, estimated_monthly_revenue, price, title)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		bookID, bucket, capturedOn, sales, revenue, price, title,
	)
	if err != nil {
		return fmt.Errorf("record snapshot: %w", err)
	}
	return nil
}
