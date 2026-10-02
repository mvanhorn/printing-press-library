// Copyright 2026 zjsng. Licensed under Apache-2.0.
package cycling

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// SaveSnapshot atomically replaces the explicit offline snapshot; no background refresh runs.
func SaveSnapshot(path string, s Snapshot) error {
	if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return e
	}
	abs, e := filepath.Abs(path)
	if e != nil {
		return e
	}
	uri := &url.URL{Scheme: "file", Path: filepath.ToSlash(abs)}
	db, e := sql.Open("sqlite", uri.String())
	if e != nil {
		return e
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, e = db.Exec(`PRAGMA busy_timeout=5000`); e != nil {
		return e
	}
	if _, e = db.Exec(`CREATE TABLE IF NOT EXISTS hello_cycling_snapshot (id INTEGER PRIMARY KEY CHECK(id=1), payload BLOB NOT NULL)`); e != nil {
		return e
	}
	b, e := json.Marshal(s)
	if e != nil {
		return e
	}
	_, e = db.Exec(`INSERT INTO hello_cycling_snapshot(id,payload) VALUES(1,?) ON CONFLICT(id) DO UPDATE SET payload=excluded.payload`, b)
	return e
}

// LoadSnapshot opens an existing cache only; a missing cache cannot silently perform network IO.
func LoadSnapshot(path string) (Snapshot, error) {
	var s Snapshot
	if _, e := os.Stat(path); e != nil {
		return s, fmt.Errorf("offline snapshot unavailable; run stations sync first: %w", e)
	}
	abs, e := filepath.Abs(path)
	if e != nil {
		return s, e
	}
	uri := &url.URL{Scheme: "file", Path: filepath.ToSlash(abs), RawQuery: "mode=ro"}
	db, e := sql.Open("sqlite", uri.String())
	if e != nil {
		return s, e
	}
	defer db.Close()
	var b []byte
	if e = db.QueryRow(`SELECT CASE WHEN length(payload)<=? THEN payload ELSE NULL END FROM hello_cycling_snapshot WHERE id=1`, 3*feedLimit).Scan(&b); e != nil {
		return s, fmt.Errorf("offline snapshot unavailable; run stations sync first: %w", e)
	}
	if len(b) == 0 || len(b) > 3*feedLimit {
		return s, fmt.Errorf("offline snapshot exceeds size bound")
	}
	if e = json.Unmarshal(b, &s); e != nil {
		return s, fmt.Errorf("invalid offline snapshot: %w", e)
	}
	return s, nil
}
