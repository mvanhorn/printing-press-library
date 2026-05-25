// Copyright 2026 startos00. Licensed under Apache-2.0. See LICENSE.

// This file extends the generated store with a per-instance "properties"
// table used by the hand-built novel commands (list/export/land-export/
// enrich/compare). It follows the same patterns as store.go: CREATE TABLE IF
// NOT EXISTS on demand, writeMu-serialized writes, WAL reads. It is kept
// separate from the generated store.go (marked DO NOT EDIT) so regeneration
// does not clobber it.

package store

import (
	"encoding/json"
	"fmt"
)

// propertiesDDL creates the full-detail per-instance property table. Keyed by
// (instance, id) so the same property id from different land banks coexists.
// The full hydrated record is stored as JSON; commonly-filtered fields are
// promoted to columns for offline classification and export without a JSON
// scan per row.
const propertiesDDL = `CREATE TABLE IF NOT EXISTS properties (
	instance        TEXT NOT NULL,
	id              TEXT NOT NULL,
	data            JSON NOT NULL,
	structure_type  TEXT,
	property_class  TEXT,
	latitude        REAL,
	longitude       REAL,
	asking_price    REAL,
	synced_at       DATETIME DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (instance, id)
)`

// PropertyRow is a hydrated full-detail property as stored locally.
type PropertyRow struct {
	Instance      string          `json:"instance"`
	ID            string          `json:"id"`
	Data          json.RawMessage `json:"data"`
	StructureType string          `json:"structure_type"`
	PropertyClass string          `json:"property_class"`
}

// ensurePropertiesTable lazily creates the properties table. Safe to call on
// every write/read path; CREATE TABLE IF NOT EXISTS is a no-op once present.
func (s *Store) ensurePropertiesTable() error {
	_, err := s.db.Exec(propertiesDDL)
	if err != nil {
		return fmt.Errorf("creating properties table: %w", err)
	}
	return nil
}

// UpsertFullProperty stores a hydrated property's full JSON keyed by
// (instance, id). structureType / propertyClass / lat / lng / askingPrice are
// promoted to columns for fast offline filtering. data is the full returnVal
// object.
func (s *Store) UpsertFullProperty(instance, id string, data json.RawMessage) error {
	if err := s.ensurePropertiesTable(); err != nil {
		return err
	}

	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return fmt.Errorf("unmarshaling property %s/%s: %w", instance, id, err)
	}

	structureType, _ := obj["structureType"].(string)
	propertyClass, _ := obj["propertyClass"].(string)
	lat := floatOrNil(obj["latitude"])
	lng := floatOrNil(obj["longitude"])
	price := floatOrNil(obj["askingPrice"])

	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.Exec(
		`INSERT INTO properties (instance, id, data, structure_type, property_class, latitude, longitude, asking_price, synced_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(instance, id) DO UPDATE SET
		   data=excluded.data, structure_type=excluded.structure_type,
		   property_class=excluded.property_class, latitude=excluded.latitude,
		   longitude=excluded.longitude, asking_price=excluded.asking_price,
		   synced_at=CURRENT_TIMESTAMP`,
		instance, id, string(data), structureType, propertyClass, lat, lng, price,
	)
	if err != nil {
		return fmt.Errorf("upserting property %s/%s: %w", instance, id, err)
	}
	return nil
}

// ListFullProperties returns all hydrated property records for an instance.
// When instance is empty, returns rows across all instances. limit <= 0 means
// no limit.
func (s *Store) ListFullProperties(instance string, limit int) ([]PropertyRow, error) {
	if err := s.ensurePropertiesTable(); err != nil {
		return nil, err
	}
	query := `SELECT instance, id, data, COALESCE(structure_type,''), COALESCE(property_class,'') FROM properties`
	var args []any
	if instance != "" {
		query += ` WHERE instance = ?`
		args = append(args, instance)
	}
	query += ` ORDER BY id`
	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	}
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []PropertyRow
	for rows.Next() {
		var r PropertyRow
		var data string
		if err := rows.Scan(&r.Instance, &r.ID, &data, &r.StructureType, &r.PropertyClass); err != nil {
			return nil, err
		}
		r.Data = json.RawMessage(data)
		out = append(out, r)
	}
	return out, rows.Err()
}

// CountFullProperties returns how many hydrated properties exist for an
// instance (empty instance = all).
func (s *Store) CountFullProperties(instance string) (int, error) {
	if err := s.ensurePropertiesTable(); err != nil {
		return 0, err
	}
	var n int
	var err error
	if instance == "" {
		err = s.db.QueryRow(`SELECT COUNT(*) FROM properties`).Scan(&n)
	} else {
		err = s.db.QueryRow(`SELECT COUNT(*) FROM properties WHERE instance = ?`, instance).Scan(&n)
	}
	return n, err
}

func floatOrNil(v any) any {
	if f, ok := v.(float64); ok {
		return f
	}
	return nil
}
