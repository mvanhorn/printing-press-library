// Copyright 2026 Greg Cole and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: CookUnity per-week menu snapshot table for `drift`. Preserved on regen.
//
// The generated `meals` table is keyed by bare meal id, so syncing a new week
// overwrites the previous week's rows. `drift` needs two weeks side by side, so
// this table keys meals by (delivery_date, meal_id) to retain per-week history.
package store

import (
	"encoding/json"
	"fmt"
	"time"
)

// SyncMeals atomically replaces the entire current-week `meals` table AND writes
// the per-week `meal_snapshots` rows for deliveryDate in a single transaction:
// every prior meals row is deleted, every new meal inserted, and every snapshot
// upserted under one commit. Either the whole sync lands or nothing changes, so
// an interrupted sync can never expose a mix of old and new weeks nor a menu
// that disagrees with its snapshot. Returns the number of meals stored.
func (s *Store) SyncMeals(deliveryDate string, items []json.RawMessage) (int, error) {
	// DDL runs outside the data transaction; CREATE TABLE IF NOT EXISTS is
	// idempotent and cheap.
	if err := s.EnsureMealSnapshots(); err != nil {
		return 0, err
	}

	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM "meals"`); err != nil {
		return 0, fmt.Errorf("clearing meals: %w", err)
	}
	stored := 0
	for _, item := range items {
		obj, err := DecodeJSONObject(item)
		if err != nil {
			continue
		}
		id := ExtractResourceID("meals", obj)
		if id == "" {
			continue
		}
		storageID := resourceStorageID("meals", id, obj)
		if err := s.upsertGenericResourceTx(tx, "meals", storageID, item); err != nil {
			return 0, fmt.Errorf("insert meal %s: %w", storageID, err)
		}
		if err := s.upsertMealsTx(tx, storageID, obj, item); err != nil {
			return 0, fmt.Errorf("insert typed meal %s: %w", storageID, err)
		}
		// Snapshot in the SAME transaction so history can never diverge from
		// the current menu on an interrupt.
		if _, err := tx.Exec(
			`INSERT INTO "meal_snapshots" ("delivery_date","meal_id","name","final_price","calories","data")
			 VALUES (?,?,?,?,?,?)
			 ON CONFLICT("delivery_date","meal_id") DO UPDATE SET
			   "name"=excluded."name","final_price"=excluded."final_price",
			   "calories"=excluded."calories","data"=excluded."data",
			   "synced_at"=CURRENT_TIMESTAMP`,
			deliveryDate, snapshotID(obj), snapshotStr(obj, "name"),
			snapshotFloat(obj, "finalPrice"), snapshotInt(obj, "calories"), string(item),
		); err != nil {
			return 0, fmt.Errorf("snapshot meal %s: %w", storageID, err)
		}
		stored++
	}
	// Write the sync-state metadata in the SAME transaction so the recorded
	// last-synced count/timestamp can never lag the committed catalog data.
	if _, err := tx.Exec(
		`INSERT INTO sync_state (resource_type, last_cursor, last_synced_at, total_count)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(resource_type) DO UPDATE SET last_cursor = excluded.last_cursor,
		 last_synced_at = excluded.last_synced_at, total_count = excluded.total_count`,
		"meals", deliveryDate, time.Now().UTC().Format(time.RFC3339), stored,
	); err != nil {
		return 0, fmt.Errorf("recording sync state: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return stored, nil
}

func snapshotID(obj map[string]any) string {
	if v, ok := obj["id"]; ok {
		return ResourceIDString(v)
	}
	return ""
}
func snapshotStr(obj map[string]any, k string) string {
	if v, ok := obj[k].(string); ok {
		return v
	}
	return ""
}
func snapshotFloat(obj map[string]any, k string) float64 {
	if v, ok := obj[k].(float64); ok {
		return v
	}
	return 0
}
func snapshotInt(obj map[string]any, k string) int {
	if v, ok := obj[k].(float64); ok {
		return int(v)
	}
	return 0
}

// EnsureMealSnapshots lazily creates the per-week snapshot table. Safe to call
// repeatedly.
func (s *Store) EnsureMealSnapshots() error {
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS "meal_snapshots" (
		"delivery_date" TEXT NOT NULL,
		"meal_id"       TEXT NOT NULL,
		"name"          TEXT,
		"final_price"   REAL,
		"calories"      INTEGER,
		"data"          JSON NOT NULL,
		"synced_at"     DATETIME DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY ("delivery_date", "meal_id")
	)`)
	if err != nil {
		return fmt.Errorf("creating meal_snapshots: %w", err)
	}
	return nil
}

// UpsertMealSnapshot writes one meal into the per-week snapshot table.
func (s *Store) UpsertMealSnapshot(deliveryDate, mealID, name string, finalPrice float64, calories int, data json.RawMessage) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.Exec(
		`INSERT INTO "meal_snapshots" ("delivery_date","meal_id","name","final_price","calories","data")
		 VALUES (?,?,?,?,?,?)
		 ON CONFLICT("delivery_date","meal_id") DO UPDATE SET
		   "name"=excluded."name","final_price"=excluded."final_price",
		   "calories"=excluded."calories","data"=excluded."data",
		   "synced_at"=CURRENT_TIMESTAMP`,
		deliveryDate, mealID, name, finalPrice, calories, string(data),
	)
	if err != nil {
		return fmt.Errorf("upserting meal snapshot %s/%s: %w", deliveryDate, mealID, err)
	}
	return nil
}

// SnapshotDates returns the distinct delivery dates that have snapshots, newest first.
func (s *Store) SnapshotDates() ([]string, error) {
	if err := s.EnsureMealSnapshots(); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(`SELECT DISTINCT "delivery_date" FROM "meal_snapshots" ORDER BY "delivery_date" DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var dates []string
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return nil, err
		}
		dates = append(dates, d)
	}
	return dates, rows.Err()
}

// SnapshotMeals returns all meal JSON rows for a given delivery date.
func (s *Store) SnapshotMeals(deliveryDate string) ([]json.RawMessage, error) {
	if err := s.EnsureMealSnapshots(); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(`SELECT "data" FROM "meal_snapshots" WHERE "delivery_date"=? ORDER BY "meal_id"`, deliveryDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []json.RawMessage
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		out = append(out, json.RawMessage(data))
	}
	return out, rows.Err()
}
