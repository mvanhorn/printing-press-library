// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package tabiwa

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	_ "modernc.org/sqlite"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"time"
)

const SavedFilename = "catalog/saved.db"
const MaxSaved = 50
const MaxSavedPayload = 24 << 10

func databaseURI(path string, readOnly bool) string {
	u := url.URL{Scheme: "file", Path: filepath.ToSlash(path)}
	v := url.Values{"_pragma": {"busy_timeout(3000)"}}
	if readOnly {
		v.Set("mode", "ro")
	}
	u.RawQuery = v.Encode()
	return u.String()
}
func Save(ctx context.Context, path string, products []Product) error {
	if len(products) == 0 {
		return nil
	}
	if len(products) > MaxSaved {
		return fmt.Errorf("save exceeds %d product bound", MaxSaved)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	db, err := sql.Open("sqlite", databaseURI(path, false))
	if err != nil {
		return err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS catalog_saved(region TEXT NOT NULL,id TEXT NOT NULL,observed_at TEXT NOT NULL,payload TEXT NOT NULL,PRIMARY KEY(region,id))`); err != nil {
		return err
	}
	for _, p := range products {
		if ValidateID(p.ID) != nil {
			return fmt.Errorf("cannot save invalid product identity")
		}
		if _, err = Region(p.Region.ID); err != nil {
			return err
		}
		incoming, e := observationInstant(p.ObservedAt)
		if e != nil {
			return e
		}
		var storedClock string
		e = tx.QueryRowContext(ctx, `SELECT observed_at FROM catalog_saved WHERE region=? AND id=?`, p.Region.ID, p.ID).Scan(&storedClock)
		if e != nil && !errors.Is(e, sql.ErrNoRows) {
			return e
		}
		if e == nil {
			stored, clockErr := observationInstant(storedClock)
			if clockErr != nil {
				return fmt.Errorf("stored observation clock: %w", clockErr)
			}
			// The read and conditional update share one ordinary SQLite transaction.
			// Equal or delayed observations cannot replace newer cached evidence.
			if !incoming.After(stored) {
				continue
			}
		}
		b, e := json.Marshal(p)
		if e != nil {
			return e
		}
		if len(b) > MaxSavedPayload {
			return fmt.Errorf("normalized product exceeds 24 KiB save bound")
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO catalog_saved(region,id,observed_at,payload) VALUES(?,?,?,?) ON CONFLICT(region,id) DO UPDATE SET observed_at=excluded.observed_at,payload=excluded.payload`, p.Region.ID, p.ID, p.ObservedAt, string(b)); err != nil {
			return err
		}
	}
	if err = trimSaved(ctx, tx); err != nil {
		return err
	}
	return tx.Commit()
}
func Saved(ctx context.Context, path, region string, limit int) ([]Product, error) {
	out := []Product{}
	if _, err := Region(region); err != nil {
		return nil, err
	}
	if limit < 1 || limit > 50 {
		return nil, fmt.Errorf("--limit must be 1..50")
	}
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	if info.Size() > 16<<20 {
		return nil, fmt.Errorf("saved catalog database exceeds 16 MiB read bound")
	}
	db, err := sql.Open("sqlite", databaseURI(path, true))
	if err != nil {
		return nil, err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	var exists int
	if err = db.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_master WHERE type='table' AND name='catalog_saved'`).Scan(&exists); err != nil {
		return nil, err
	}
	if exists == 0 {
		return out, nil
	}
	rows, err := db.QueryContext(ctx, `SELECT region,id,observed_at,payload FROM catalog_saved WHERE region=? LIMIT 51`, region)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var rowRegion, id, observed, payload string
		if err = rows.Scan(&rowRegion, &id, &observed, &payload); err != nil {
			return nil, err
		}
		if len(payload) > MaxSavedPayload {
			return nil, fmt.Errorf("saved payload exceeds 24 KiB read bound")
		}
		var p Product
		if err = json.Unmarshal([]byte(payload), &p); err != nil {
			return nil, fmt.Errorf("saved catalog record is invalid: %w", err)
		}
		if p.ID != id || p.Region.ID != rowRegion || p.ObservedAt != observed {
			return nil, fmt.Errorf("saved catalog identity or clock does not match its row")
		}
		if _, err = observationInstant(p.ObservedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
		if len(out) > MaxSaved {
			return nil, fmt.Errorf("saved catalog exceeds 50-record read bound")
		}
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, _ := observationInstant(out[i].ObservedAt)
		b, _ := observationInstant(out[j].ObservedAt)
		if a.Equal(b) {
			return out[i].ID < out[j].ID
		}
		return a.After(b)
	})
	return out[:min(len(out), limit)], nil
}

func observationInstant(clock string) (time.Time, error) {
	instant, err := time.Parse(time.RFC3339Nano, clock)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid source observation clock %q: expected RFC3339", clock)
	}
	return instant, nil
}

// trimSaved compares real instants rather than timestamp text. At most the
// previous 50 records plus one bounded save batch can be present in this transaction.
func trimSaved(ctx context.Context, tx *sql.Tx) error {
	type row struct {
		region, id string
		instant    time.Time
	}
	rows, err := tx.QueryContext(ctx, `SELECT region,id,observed_at FROM catalog_saved LIMIT 101`)
	if err != nil {
		return err
	}
	records := []row{}
	for rows.Next() {
		var r row
		var clock string
		if err = rows.Scan(&r.region, &r.id, &clock); err != nil {
			return errors.Join(err, rows.Close())
		}
		r.instant, err = observationInstant(clock)
		if err != nil {
			return errors.Join(err, rows.Close())
		}
		records = append(records, r)
	}
	err = errors.Join(rows.Err(), rows.Close())
	if err != nil {
		return err
	}
	if len(records) > MaxSaved*2 {
		return fmt.Errorf("saved catalog exceeds the 100-record transactional bound")
	}
	sort.SliceStable(records, func(i, j int) bool {
		a, b := records[i], records[j]
		if a.instant.Equal(b.instant) {
			return a.region+"/"+a.id < b.region+"/"+b.id
		}
		return a.instant.After(b.instant)
	})
	for _, r := range records[min(len(records), MaxSaved):] {
		if _, err = tx.ExecContext(ctx, `DELETE FROM catalog_saved WHERE region=? AND id=?`, r.region, r.id); err != nil {
			return err
		}
	}
	return nil
}
