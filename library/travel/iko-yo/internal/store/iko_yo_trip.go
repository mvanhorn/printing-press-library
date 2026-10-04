// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/travel/iko-yo/internal/trip"
)

func tripTablesExist(ctx context.Context, s *Store) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE type='table' AND name='iko_yo_trip_records'").Scan(&n)
	return n > 0, err
}
func (s *Store) ensureTripTables(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS iko_yo_trip_records (ref TEXT PRIMARY KEY, kind TEXT NOT NULL, data TEXT NOT NULL, observed_at TEXT NOT NULL, detail INTEGER NOT NULL);
 CREATE TABLE IF NOT EXISTS iko_yo_trip_collections (collection_key TEXT PRIMARY KEY, data TEXT NOT NULL, observed_at TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS iko_yo_trip_memberships (ref TEXT NOT NULL, collection_key TEXT NOT NULL, PRIMARY KEY(ref,collection_key));`)
	return err
}

// SaveTripRecords atomically saves factual snapshots and collection provenance.
// Basic list observations never replace older detailed facts or relabel their age.
// Whole detailed snapshots replace older ones so newly unknown/absent facts cannot
// inherit a stale positive assertion through the generic richer-JSON merge.
func (s *Store) SaveTripRecords(ctx context.Context, records []trip.Record) error {
	if len(records) > 75 {
		return fmt.Errorf("Trip save exceeds 75-record source window")
	}
	s.lockForWrite()
	defer s.unlockAfterWrite()
	if err := s.ensureTripTables(ctx); err != nil {
		return fmt.Errorf("initialize Trip cache: %w", err)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, r := range records {
		kind, id, err := trip.ParseReference(r.Ref)
		if err != nil || r.ID != id || r.Kind != kind {
			return fmt.Errorf("invalid normalized Trip cache reference %q", r.Ref)
		}
		data, err := json.Marshal(r)
		if err != nil {
			return err
		}
		var oldData string
		var oldDetail int
		err = tx.QueryRowContext(ctx, "SELECT data,detail FROM iko_yo_trip_records WHERE ref=?", r.Ref).Scan(&oldData, &oldDetail)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if oldDetail == 1 && !r.Detail {
			data = []byte(oldData)
		}
		var chosen trip.Record
		if err := json.Unmarshal(data, &chosen); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO iko_yo_trip_records(ref,kind,data,observed_at,detail) VALUES(?,?,?,?,?) ON CONFLICT(ref) DO UPDATE SET data=excluded.data,observed_at=excluded.observed_at,detail=excluded.detail`, r.Ref, kind, string(data), chosen.ObservedAt, chosen.Detail); err != nil {
			return err
		}
		key := fmt.Sprintf("list/%s/%d/%d", kind, r.Collection.Region, r.Collection.Prefecture)
		if r.Collection.PagesScanned == 0 {
			key = "detail/" + r.Ref
		}
		key += "/" + r.Collection.ObservedAt
		coverage, err := json.Marshal(r.Collection)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO iko_yo_trip_collections(collection_key,data,observed_at) VALUES(?,?,?) ON CONFLICT(collection_key) DO UPDATE SET data=excluded.data,observed_at=excluded.observed_at`, key, string(coverage), r.Collection.ObservedAt); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "INSERT OR IGNORE INTO iko_yo_trip_memberships(ref,collection_key) VALUES(?,?)", r.Ref, key); err != nil {
			return err
		}
		if err := s.upsertGenericResourceTx(tx, kind, id, data); err != nil {
			return fmt.Errorf("mirror Trip facts into searchable resources: %w", err)
		}
	}
	return tx.Commit()
}
func (s *Store) TripRecord(ctx context.Context, ref string) (trip.Record, error) {
	var r trip.Record
	exists, err := tripTablesExist(ctx, s)
	if err != nil {
		return r, err
	}
	if !exists {
		return r, sql.ErrNoRows
	}
	var data string
	if err := s.db.QueryRowContext(ctx, "SELECT data FROM iko_yo_trip_records WHERE ref=?", ref).Scan(&data); err != nil {
		return r, err
	}
	if err := json.Unmarshal([]byte(data), &r); err != nil {
		return r, err
	}
	r.DataSource = "local"
	return r, nil
}

// TripRecords drains and closes snapshot rows before reading collection rows.
func (s *Store) TripRecords(ctx context.Context, kind string, maxScan int) ([]trip.Record, []trip.Coverage, error) {
	records := make([]trip.Record, 0)
	coverage := make([]trip.Coverage, 0)
	if maxScan < 1 || maxScan > 1000 {
		return records, coverage, fmt.Errorf("Trip local scan must be from 1 to 1000 records")
	}
	if kind != "all" && kind != "spots" && kind != "events" {
		return records, coverage, fmt.Errorf("invalid Trip cache kind %q", kind)
	}
	exists, err := tripTablesExist(ctx, s)
	if err != nil || !exists {
		return records, coverage, err
	}
	rows, err := s.db.QueryContext(ctx, "SELECT data FROM iko_yo_trip_records WHERE (?='all' OR kind=?) ORDER BY observed_at DESC,ref LIMIT ?", kind, kind, maxScan)
	if err != nil {
		return records, coverage, err
	}
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return records, coverage, closeTripRows(rows, err)
		}
		var r trip.Record
		if err := json.Unmarshal([]byte(data), &r); err != nil {
			return records, coverage, closeTripRows(rows, err)
		}
		r.DataSource = "local"
		records = append(records, r)
	}
	if err := rows.Err(); err != nil {
		return records, coverage, closeTripRows(rows, err)
	}
	if err := rows.Close(); err != nil {
		return records, coverage, err
	}
	refs := map[string]bool{}
	for _, r := range records {
		refs[r.Ref] = true
	}
	rows, err = s.db.QueryContext(ctx, "SELECT m.ref,c.collection_key,c.data FROM iko_yo_trip_memberships m JOIN iko_yo_trip_collections c USING(collection_key) ORDER BY c.observed_at DESC LIMIT 5000")
	if err != nil {
		return records, coverage, err
	}
	seen := map[string]bool{}
	for rows.Next() {
		var ref, key, data string
		if err := rows.Scan(&ref, &key, &data); err != nil {
			return records, coverage, closeTripRows(rows, err)
		}
		if !refs[ref] || seen[key] {
			continue
		}
		seen[key] = true
		var c trip.Coverage
		if err := json.Unmarshal([]byte(data), &c); err != nil {
			return records, coverage, closeTripRows(rows, err)
		}
		coverage = append(coverage, c)
	}
	if err := rows.Err(); err != nil {
		return records, coverage, closeTripRows(rows, err)
	}
	if err := rows.Close(); err != nil {
		return records, coverage, err
	}
	if len(coverage) > 100 {
		coverage = coverage[:100]
	}
	return records, coverage, nil
}

// TripCacheCount is useful for reporting an unsaved store without inventing a
// full synchronization marker for these deliberately partial observations.
func (s *Store) TripCacheCount(ctx context.Context, kind string) (int, error) {
	exists, err := tripTablesExist(ctx, s)
	if err != nil || !exists {
		return 0, err
	}
	var n int
	err = s.db.QueryRowContext(ctx, "SELECT count(*) FROM iko_yo_trip_records WHERE (?='' OR ?='all' OR kind=?)", kind, kind, kind).Scan(&n)
	return n, err
}

// closeTripRows preserves the primary read error and a secondary close error.
func closeTripRows(rows *sql.Rows, cause error) error {
	if err := rows.Close(); err != nil {
		return errors.Join(cause, err)
	}
	return cause
}
