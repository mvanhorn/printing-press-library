// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package ticket

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

const MaxCached = 100
const MaxPayload = 32 * 1024
const CacheFilename = "./tickets.sqlite"

func openCache(path string, readOnly bool) (*sql.DB, error) {
	a, e := filepath.Abs(path)
	if e != nil {
		return nil, e
	}
	u := url.URL{Scheme: "file", Path: filepath.ToSlash(a)}
	q := url.Values{}
	q.Set("_pragma", "busy_timeout(5000)")
	if readOnly {
		q.Set("mode", "ro")
	}
	u.RawQuery = q.Encode()
	db, e := sql.Open("sqlite", u.String())
	if e != nil {
		return nil, e
	}
	db.SetMaxOpenConns(1)
	return db, nil
}

// Save uses ordinary SQLite transactions, retains newest evidence and caps records/payloads.
func Save(ctx context.Context, path string, t Ticket) error {
	if !ValidID(t.ID) || t.SourceURL != Origin+"/ticket/"+t.ID+".html" {
		return errors.New("cache ticket source identity is invalid")
	}
	at, e := time.Parse(time.RFC3339Nano, t.ObservedAt)
	if e != nil {
		return errors.New("cache observed_at is not RFC3339")
	}
	t.ObservedAt = at.UTC().Format("2006-01-02T15:04:05.000000000Z")
	b, e := json.Marshal(t)
	if e != nil {
		return e
	}
	if len(b) > MaxPayload {
		return fmt.Errorf("normalized ticket exceeds %d-byte cache payload bound", MaxPayload)
	}
	if e = os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return e
	}
	db, e := openCache(path, false)
	if e != nil {
		return e
	}
	defer db.Close()
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if _, e = tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS ticket_observations(id TEXT PRIMARY KEY, name_ja TEXT NOT NULL, observed_at TEXT NOT NULL, data TEXT NOT NULL CHECK(length(CAST(data AS BLOB))<=32768))`); e != nil {
		return e
	}
	if _, e = tx.ExecContext(ctx, `INSERT INTO ticket_observations(id,name_ja,observed_at,data) VALUES(?,?,?,?) ON CONFLICT(id) DO UPDATE SET name_ja=excluded.name_ja, observed_at=excluded.observed_at, data=excluded.data WHERE excluded.observed_at>=ticket_observations.observed_at`, t.ID, t.NameJA, t.ObservedAt, string(b)); e != nil {
		return e
	}
	if _, e = tx.ExecContext(ctx, `DELETE FROM ticket_observations WHERE id NOT IN (SELECT id FROM ticket_observations ORDER BY observed_at DESC,id LIMIT 100)`); e != nil {
		return e
	}
	return tx.Commit()
}

// Cached opens the existing database read-only, without schema creation or refresh.
func Cached(ctx context.Context, path, query string, limit int) ([]Ticket, error) {
	out := []Ticket{}
	if limit < 1 || limit > 50 {
		return out, fmt.Errorf("--limit must be 1..50")
	}
	info, e := os.Stat(path)
	if errors.Is(e, os.ErrNotExist) {
		return out, nil
	}
	if e != nil {
		return out, e
	}
	if !info.Mode().IsRegular() {
		return out, errors.New("ticket cache is not a regular SQLite file")
	}
	if info.Size() > 8*1024*1024 {
		return out, errors.New("ticket cache exceeds 8 MiB read bound")
	}
	db, e := openCache(path, true)
	if e != nil {
		return out, e
	}
	defer db.Close()
	var found int
	e = db.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_master WHERE type='table' AND name='ticket_observations'`).Scan(&found)
	if e != nil {
		return out, e
	}
	if found == 0 {
		return out, nil
	}
	rows, e := db.QueryContext(ctx, `SELECT data FROM ticket_observations WHERE instr(lower(name_ja),lower(?))>0 OR id=? ORDER BY observed_at DESC,id LIMIT ?`, query, query, limit)
	if e != nil {
		return out, e
	}
	defer rows.Close()
	for rows.Next() {
		var b string
		if e = rows.Scan(&b); e != nil {
			return out, e
		}
		if len(b) > MaxPayload {
			return out, errors.New("cached ticket exceeds payload bound")
		}
		var t Ticket
		if e = json.Unmarshal([]byte(b), &t); e != nil {
			return out, fmt.Errorf("invalid cached ticket: %w", e)
		}
		if !ValidID(t.ID) || t.SourceURL != Origin+"/ticket/"+t.ID+".html" {
			return out, errors.New("invalid cached source identity")
		}
		t.Transport = "local"
		if t.Operator != nil {
			t.Operator.Transport = "local"
			t.Operator.Stale = false
			t.Operator.Freshness = "unknown_clock"
			t.Operator.ObservationAgeSeconds = 0
			if t.Operator.ObservedAt != nil {
				if at, e := time.Parse(time.RFC3339Nano, *t.Operator.ObservedAt); e == nil {
					t.Operator.ObservationAgeSeconds = max(0, int64(time.Since(at).Seconds()))
					t.Operator.Freshness = "not_evaluated"
				} else {
					return out, errors.New("invalid cached operator observation clock")
				}
			}
		}
		if at, e := time.Parse(time.RFC3339Nano, t.ObservedAt); e == nil {
			t.CacheAgeSeconds = int64(time.Since(at).Seconds())
			if t.CacheAgeSeconds < 0 {
				t.CacheAgeSeconds = 0
			}
		} else {
			return out, errors.New("invalid cached observation clock")
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
