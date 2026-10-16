// Copyright 2026 ElizabethcStone and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/rankmath/internal/seohead"
)

// seoSnapshotTimeLayout keeps stored timestamps fixed-width for lexical ordering.
const seoSnapshotTimeLayout = "2006-01-02T15:04:05.000000000Z07:00"
const snapshotColumns = "url,fetched_at,status,title,description,canonical,robots,noindex,schema_types,COALESCE(final_url,'') AS final_url,COALESCE(og_title,'') AS og_title,COALESCE(og_description,'') AS og_description"
const snapshotInsert = `INSERT OR REPLACE INTO seo_snapshots(url,fetched_at,status,title,description,canonical,robots,noindex,schema_types,final_url,og_title,og_description) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`

func (s *Store) EnsureSEOSnapshots(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS seo_snapshots (url TEXT NOT NULL, fetched_at TEXT NOT NULL, status INT, title TEXT, description TEXT, canonical TEXT, robots TEXT, noindex INT, schema_types TEXT, final_url TEXT, og_title TEXT, og_description TEXT, PRIMARY KEY(url, fetched_at))`); err != nil {
		return err
	}
	rows, err := s.db.QueryContext(ctx, `PRAGMA table_info(seo_snapshots)`)
	if err != nil {
		return err
	}
	columns := make(map[string]bool)
	for rows.Next() {
		var cid, notnull, pk int
		var name, typ string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &typ, &notnull, &defaultValue, &pk); err != nil {
			rows.Close()
			return err
		}
		columns[name] = true
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, column := range []struct{ name, declaration string }{{"final_url", "TEXT"}, {"og_title", "TEXT"}, {"og_description", "TEXT"}} {
		if !columns[column.name] {
			if _, err := s.db.ExecContext(ctx, `ALTER TABLE seo_snapshots ADD COLUMN `+column.name+` `+column.declaration); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Store) InsertSEOSnapshot(ctx context.Context, h seohead.Head) error {
	return s.insertSEOSnapshots(ctx, []seohead.Head{h}, false)
}

func (s *Store) InsertSEOSnapshots(ctx context.Context, heads []seohead.Head) error {
	return s.insertSEOSnapshots(ctx, heads, true)
}

func (s *Store) insertSEOSnapshots(ctx context.Context, heads []seohead.Head, transaction bool) error {
	var tx *sql.Tx
	var stmt *sql.Stmt
	var err error
	if transaction {
		tx, err = s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		stmt, err = tx.PrepareContext(ctx, snapshotInsert)
	} else {
		stmt, err = s.db.PrepareContext(ctx, snapshotInsert)
	}
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, h := range heads {
		types, err := json.Marshal(h.SchemaTypes)
		if err != nil {
			return err
		}
		if _, err := stmt.ExecContext(ctx, h.URL, h.FetchedAt.UTC().Format(seoSnapshotTimeLayout), h.Status, h.Title, h.Description, h.Canonical, h.Robots, boolInt(h.Noindex), string(types), h.FinalURL, h.OGTitle, h.OGDescription); err != nil {
			return err
		}
	}
	if tx != nil {
		return tx.Commit()
	}
	return nil
}

func (s *Store) LatestSEOSnapshots(ctx context.Context) ([]seohead.Head, error) {
	return s.rankedSEOSnapshots(ctx, 1)
}

func (s *Store) PreviousSEOSnapshots(ctx context.Context) (map[string]seohead.Head, error) {
	rows, err := s.rankedSEOSnapshots(ctx, 2)
	if err != nil {
		return nil, err
	}
	out := make(map[string]seohead.Head, len(rows))
	for _, h := range rows {
		out[h.URL] = h
	}
	return out, nil
}

func (s *Store) rankedSEOSnapshots(ctx context.Context, rank int) ([]seohead.Head, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+snapshotColumns+` FROM (SELECT `+snapshotColumns+`, ROW_NUMBER() OVER (PARTITION BY url ORDER BY fetched_at DESC) AS snapshot_rank FROM seo_snapshots WHERE status<400) WHERE snapshot_rank=? ORDER BY url`, rank)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]seohead.Head, 0)
	for rows.Next() {
		h, err := scanSnapshot(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

type snapshotScanner interface{ Scan(...any) error }

func scanSnapshot(r snapshotScanner) (seohead.Head, error) {
	var h seohead.Head
	var at, types string
	var ni int
	if err := r.Scan(&h.URL, &at, &h.Status, &h.Title, &h.Description, &h.Canonical, &h.Robots, &ni, &types, &h.FinalURL, &h.OGTitle, &h.OGDescription); err != nil {
		return h, err
	}
	parsedAt, err := time.Parse(time.RFC3339Nano, at)
	if err != nil {
		return h, fmt.Errorf("parse snapshot fetched_at: %w", err)
	}
	h.FetchedAt = parsedAt
	h.Noindex = ni != 0
	if err := json.Unmarshal([]byte(types), &h.SchemaTypes); err != nil {
		return h, fmt.Errorf("decode snapshot schema types: %w", err)
	}
	if h.SchemaTypes == nil {
		h.SchemaTypes = []string{}
	}
	return h, nil
}
func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
