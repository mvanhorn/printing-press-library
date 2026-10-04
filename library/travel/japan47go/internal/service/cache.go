// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	_ "modernc.org/sqlite"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"
)

const CacheFilename = "./japan47go-observations-v1.sqlite"
const Capacity = 200

func openDB(ctx context.Context, path string, write bool) (*sql.DB, error) {
	if write {
		if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
			return nil, e
		}
	} else {
		if _, e := os.Stat(path); e != nil {
			return nil, e
		}
	}
	absolute, e := filepath.Abs(path)
	if e != nil {
		return nil, e
	}
	u := url.URL{Scheme: "file", Path: filepath.ToSlash(absolute)}
	q := url.Values{"_pragma": {"busy_timeout(5000)"}}
	if !write {
		q.Set("mode", "ro")
	}
	u.RawQuery = q.Encode()
	db, e := sql.Open("sqlite", u.String())
	if e != nil {
		return nil, e
	}
	db.SetMaxOpenConns(1)
	if e = db.PingContext(ctx); e != nil {
		_ = db.Close()
		return nil, e
	}
	return db, nil
}
func Save(ctx context.Context, path string, s Service) error {
	if _, e := ID(s.ID); e != nil {
		return e
	}
	instant, err := time.Parse(time.RFC3339Nano, s.ObservedAt)
	if err != nil || instant.Year() < 1970 || instant.Year() > 2200 {
		return fmt.Errorf("invalid observation timestamp")
	}
	s.SourceFailure = nil
	s.CacheWarning = nil
	s.CacheAgeSeconds = 0
	s.Stale = false
	s.Transport = "live"
	payload, e := json.Marshal(s)
	if e != nil {
		return e
	}
	if len(payload) > 65536 {
		return fmt.Errorf("normalized observation exceeds 64KiB")
	}
	db, e := openDB(ctx, path, true)
	if e != nil {
		return e
	}
	defer db.Close()
	if e = os.Chmod(path, 0600); e != nil {
		return e
	}
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if _, e = tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS service_observations(id TEXT PRIMARY KEY, observed_at TEXT NOT NULL, observed_ns INTEGER NOT NULL, payload TEXT NOT NULL)`); e != nil {
		return e
	}
	if _, e = tx.ExecContext(ctx, `INSERT INTO service_observations(id,observed_at,observed_ns,payload) VALUES(?,?,?,?) ON CONFLICT(id) DO UPDATE SET observed_at=excluded.observed_at,observed_ns=excluded.observed_ns,payload=excluded.payload WHERE excluded.observed_ns>=service_observations.observed_ns`, s.ID, s.ObservedAt, instant.UnixNano(), string(payload)); e != nil {
		return e
	}
	if _, e = tx.ExecContext(ctx, `DELETE FROM service_observations WHERE id IN(SELECT id FROM service_observations ORDER BY observed_ns DESC,id LIMIT -1 OFFSET ?)`, Capacity); e != nil {
		return e
	}
	return tx.Commit()
}
func Cached(ctx context.Context, path, query string, limit int) ([]Service, int, error) {
	out := []Service{}
	if limit < 1 || limit > 50 {
		return out, 0, fmt.Errorf("--limit must be 1..50")
	}
	db, e := openDB(ctx, path, false)
	if os.IsNotExist(e) {
		return out, 0, nil
	}
	if e != nil {
		return nil, 0, e
	}
	defer db.Close()
	var exists int
	if e = db.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_master WHERE type='table' AND name='service_observations'`).Scan(&exists); e != nil {
		return nil, 0, e
	}
	if exists == 0 {
		return out, 0, nil
	}
	rows, e := db.QueryContext(ctx, `SELECT id,payload FROM service_observations ORDER BY observed_ns DESC,id LIMIT ?`, Capacity+1)
	if e != nil {
		return nil, 0, e
	}
	defer rows.Close()
	scanned := 0
	for rows.Next() {
		scanned++
		if scanned > Capacity {
			return nil, 0, fmt.Errorf("cache exceeds %d observation bound", Capacity)
		}
		var id, p string
		if e = rows.Scan(&id, &p); e != nil {
			return nil, 0, e
		}
		if len(p) > 65536 {
			return nil, 0, fmt.Errorf("cache record exceeds 64KiB")
		}
		var s Service
		if e = json.Unmarshal([]byte(p), &s); e != nil || s.ID != id {
			return nil, 0, fmt.Errorf("invalid normalized cache record")
		}
		if _, e = ID(s.ID); e != nil {
			return nil, 0, e
		}
		at, e := time.Parse(time.RFC3339Nano, s.ObservedAt)
		if e != nil {
			return nil, 0, fmt.Errorf("invalid observation timestamp")
		}
		s = recheckSavedFacets(s)
		s.Transport = "local"
		s.CacheAgeSeconds = int64(time.Since(at).Seconds())
		if s.CacheAgeSeconds < 0 {
			s.CacheAgeSeconds = 0
		}
		if (query == "" || strings.Contains(strings.ToLower(s.ID+" "+s.NameJA+" "+s.CityJA+" "+s.PrefectureJA), strings.ToLower(query))) && len(out) < limit {
			out = append(out, s)
		}
	}
	if e = rows.Err(); e != nil {
		return nil, 0, e
	}
	return out, scanned, nil
}

// recheckSavedFacets corrects the two formerly overstated derived facets from
// already-recorded evidence. It never writes or advances the source clocks.
func recheckSavedFacets(s Service) Service {
	changed := false
	if strings.TrimSpace(s.Request.Original) != "" {
		request := ParseRequest("【予約期限】" + s.Request.Original)
		if !reflect.DeepEqual(request, s.Request) {
			s.Request = request
			changed = true
		}
	}
	if separateExpenses.MatchString(s.Price.Original + "\n" + s.DescriptionEvidence) {
		price := ParsePrice(s.Price.Original, s.DescriptionEvidence)
		// Existing amounts/units remain recorded facts; only the cost obligation
		// and its bounded original evidence are reinterpreted.
		price.Amounts = s.Price.Amounts
		for _, q := range s.Price.Qualifiers {
			// This derived qualifier was recomputed above; do not restore a waived obligation.
			if q == "expenses" {
				continue
			}
			found := false
			for _, v := range price.Qualifiers {
				if v == q {
					found = true
					break
				}
			}
			if !found {
				price.Qualifiers = append(price.Qualifiers, q)
			}
		}
		if !reflect.DeepEqual(price, s.Price) {
			s.Price = price
			changed = true
		}
	}
	if changed {
		warning := "Reinterpreted saved request/fee evidence; source observation time is unchanged."
		if s.CacheWarning != nil && *s.CacheWarning != "" {
			warning = *s.CacheWarning + " " + warning
		}
		s.CacheWarning = &warning
	}
	return s
}
