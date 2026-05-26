package store

import (
	"database/sql"
	"fmt"
)

// normalizationMigrations are additive CREATE TABLE/INDEX statements for the
// entity-normalization layer. They never modify the raw `resources` schema.
// source_system / external_id columns are present for the future cross-provider
// crosswalk but are DICE-only in phase 1.
var normalizationMigrations = []string{
	`CREATE TABLE IF NOT EXISTS canonical_entity (
		canonical_id   TEXT NOT NULL,
		entity_type    TEXT NOT NULL,
		canonical_name TEXT NOT NULL,
		PRIMARY KEY (entity_type, canonical_id)
	)`,
	`CREATE TABLE IF NOT EXISTS entity_external_ref (
		entity_type   TEXT NOT NULL,
		canonical_id  TEXT NOT NULL,
		source_system TEXT NOT NULL,
		external_id   TEXT NOT NULL,
		PRIMARY KEY (entity_type, canonical_id, source_system)
	)`,
	`CREATE TABLE IF NOT EXISTS entity_crosswalk (
		entity_type       TEXT NOT NULL,
		source_system     TEXT NOT NULL,
		source_value      TEXT NOT NULL,
		source_id         TEXT,
		canonical_id      TEXT NOT NULL,
		method            TEXT NOT NULL,
		classifier_version INTEGER NOT NULL,
		PRIMARY KEY (entity_type, source_system, source_value)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_crosswalk_canonical ON entity_crosswalk(entity_type, canonical_id)`,
	`CREATE TABLE IF NOT EXISTS tier_attributes (
		canonical_id       TEXT PRIMARY KEY,
		access_class       TEXT,
		sales_stage        TEXT,
		entry_window_type  TEXT,
		entry_window_time  TEXT,
		group_size         INTEGER,
		comp_flag          INTEGER,
		classifier_version INTEGER NOT NULL,
		method             TEXT NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_tier_attrs_axes ON tier_attributes(access_class, sales_stage)`,
	`CREATE TABLE IF NOT EXISTS venue_attributes (
		canonical_id       TEXT PRIMARY KEY,
		complex            TEXT,
		room               TEXT,
		classifier_version INTEGER NOT NULL,
		method             TEXT NOT NULL
	)`,
}

// CrosswalkRow is a single entry in the entity_crosswalk table that maps a
// raw source value to a canonical entity ID.
type CrosswalkRow struct {
	EntityType        string
	SourceSystem      string
	SourceValue       string
	SourceID          string // optional; may be empty
	CanonicalID       string
	Method            string
	ClassifierVersion int
}

// TierAttributesRow holds the extracted tier axes for a canonical entity.
type TierAttributesRow struct {
	CanonicalID       string
	AccessClass       string
	SalesStage        string
	EntryWindowType   string
	EntryWindowTime   string
	GroupSize         int
	CompFlag          bool
	ClassifierVersion int
	Method            string
}

// VenueAttributesRow holds the extracted venue parts for a canonical entity.
type VenueAttributesRow struct {
	CanonicalID       string
	Complex           string
	Room              string
	ClassifierVersion int
	Method            string
}

// UpsertCrosswalk inserts or replaces a crosswalk row keyed by
// (entity_type, source_system, source_value). All writes are serialized
// through writeMu to mirror the locking pattern of UpsertBatch/SaveSyncState.
func (s *Store) UpsertCrosswalk(row CrosswalkRow) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.Exec(
		`INSERT INTO entity_crosswalk
			(entity_type, source_system, source_value, source_id, canonical_id, method, classifier_version)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(entity_type, source_system, source_value) DO UPDATE SET
			source_id          = excluded.source_id,
			canonical_id       = excluded.canonical_id,
			method             = excluded.method,
			classifier_version = excluded.classifier_version`,
		row.EntityType, row.SourceSystem, row.SourceValue, nullString(row.SourceID),
		row.CanonicalID, row.Method, row.ClassifierVersion,
	)
	if err != nil {
		return fmt.Errorf("upsert crosswalk: %w", err)
	}
	return nil
}

// ListCrosswalk returns all crosswalk rows for a given entity type and source
// system. Results are ordered by source_value for deterministic test output.
func (s *Store) ListCrosswalk(entityType, sourceSystem string) ([]CrosswalkRow, error) {
	rows, err := s.db.Query(
		`SELECT entity_type, source_system, source_value, COALESCE(source_id,''), canonical_id, method, classifier_version
		 FROM entity_crosswalk
		 WHERE entity_type = ? AND source_system = ?
		 ORDER BY source_value`,
		entityType, sourceSystem,
	)
	if err != nil {
		return nil, fmt.Errorf("list crosswalk: %w", err)
	}
	defer rows.Close()

	var results []CrosswalkRow
	for rows.Next() {
		var r CrosswalkRow
		if err := rows.Scan(&r.EntityType, &r.SourceSystem, &r.SourceValue, &r.SourceID,
			&r.CanonicalID, &r.Method, &r.ClassifierVersion); err != nil {
			return nil, fmt.Errorf("scan crosswalk: %w", err)
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// UpsertCanonicalEntity inserts or updates a canonical entity record.
func (s *Store) UpsertCanonicalEntity(entityType, canonicalID, canonicalName string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.Exec(
		`INSERT INTO canonical_entity (canonical_id, entity_type, canonical_name)
		 VALUES (?, ?, ?)
		 ON CONFLICT(entity_type, canonical_id) DO UPDATE SET canonical_name = excluded.canonical_name`,
		canonicalID, entityType, canonicalName,
	)
	if err != nil {
		return fmt.Errorf("upsert canonical entity: %w", err)
	}
	return nil
}

// UpsertTierAttributes inserts or replaces tier axis attributes for a
// canonical ID. All writes are serialized through writeMu.
func (s *Store) UpsertTierAttributes(canonicalID string, row TierAttributesRow) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	compInt := 0
	if row.CompFlag {
		compInt = 1
	}
	_, err := s.db.Exec(
		`INSERT INTO tier_attributes
			(canonical_id, access_class, sales_stage, entry_window_type, entry_window_time,
			 group_size, comp_flag, classifier_version, method)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(canonical_id) DO UPDATE SET
			access_class       = excluded.access_class,
			sales_stage        = excluded.sales_stage,
			entry_window_type  = excluded.entry_window_type,
			entry_window_time  = excluded.entry_window_time,
			group_size         = excluded.group_size,
			comp_flag          = excluded.comp_flag,
			classifier_version = excluded.classifier_version,
			method             = excluded.method`,
		canonicalID, nullString(row.AccessClass), nullString(row.SalesStage),
		nullString(row.EntryWindowType), nullString(row.EntryWindowTime),
		row.GroupSize, compInt, row.ClassifierVersion, row.Method,
	)
	if err != nil {
		return fmt.Errorf("upsert tier attributes: %w", err)
	}
	return nil
}

// UpsertVenueAttributes inserts or replaces venue part attributes for a
// canonical ID. All writes are serialized through writeMu.
func (s *Store) UpsertVenueAttributes(canonicalID string, row VenueAttributesRow) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.Exec(
		`INSERT INTO venue_attributes
			(canonical_id, complex, room, classifier_version, method)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(canonical_id) DO UPDATE SET
			complex            = excluded.complex,
			room               = excluded.room,
			classifier_version = excluded.classifier_version,
			method             = excluded.method`,
		canonicalID, nullString(row.Complex), nullString(row.Room),
		row.ClassifierVersion, row.Method,
	)
	if err != nil {
		return fmt.Errorf("upsert venue attributes: %w", err)
	}
	return nil
}

// ListTierAttributes returns all tier attribute rows for a given entity type
// by joining through entity_crosswalk. Results are ordered by canonical_id.
func (s *Store) ListTierAttributes(entityType string) ([]TierAttributesRow, error) {
	rows, err := s.db.Query(
		`SELECT ta.canonical_id,
			COALESCE(ta.access_class,''), COALESCE(ta.sales_stage,''),
			COALESCE(ta.entry_window_type,''), COALESCE(ta.entry_window_time,''),
			ta.group_size, ta.comp_flag, ta.classifier_version, ta.method
		 FROM tier_attributes ta
		 WHERE ta.canonical_id IN (
			SELECT DISTINCT canonical_id FROM entity_crosswalk WHERE entity_type = ?
		 )
		 ORDER BY ta.canonical_id`,
		entityType,
	)
	if err != nil {
		return nil, fmt.Errorf("list tier attributes: %w", err)
	}
	defer rows.Close()

	var results []TierAttributesRow
	for rows.Next() {
		var r TierAttributesRow
		var compInt int
		if err := rows.Scan(
			&r.CanonicalID, &r.AccessClass, &r.SalesStage,
			&r.EntryWindowType, &r.EntryWindowTime,
			&r.GroupSize, &compInt, &r.ClassifierVersion, &r.Method,
		); err != nil {
			return nil, fmt.Errorf("scan tier attributes: %w", err)
		}
		r.CompFlag = compInt != 0
		results = append(results, r)
	}
	return results, rows.Err()
}

// ClearNormalization removes all non-manual normalization rows for the given
// entity type from entity_crosswalk and the corresponding attribute tables.
// Rows with method='manual' are preserved so operator overrides survive a
// re-classification run.
func (s *Store) ClearNormalization(entityType string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	// Collect canonical IDs that are exclusively non-manual before deleting,
	// so we can clean up the attribute tables without touching manual entries.
	rows, err := s.db.Query(
		`SELECT DISTINCT canonical_id FROM entity_crosswalk
		 WHERE entity_type = ? AND method <> 'manual'
		   AND canonical_id NOT IN (
			SELECT canonical_id FROM entity_crosswalk
			WHERE entity_type = ? AND method = 'manual'
		   )`,
		entityType, entityType,
	)
	if err != nil {
		return fmt.Errorf("collecting non-manual canonical IDs: %w", err)
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return fmt.Errorf("scan canonical id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	// Delete non-manual crosswalk rows.
	if _, err := s.db.Exec(
		`DELETE FROM entity_crosswalk WHERE entity_type = ? AND method <> 'manual'`,
		entityType,
	); err != nil {
		return fmt.Errorf("clear crosswalk: %w", err)
	}

	// Delete tier/venue attributes only for canonical IDs that had no manual
	// crosswalk row (collected above).
	for _, id := range ids {
		if _, err := s.db.Exec(
			`DELETE FROM tier_attributes WHERE canonical_id = ?`, id,
		); err != nil {
			return fmt.Errorf("clear tier attributes for %s: %w", id, err)
		}
		if _, err := s.db.Exec(
			`DELETE FROM venue_attributes WHERE canonical_id = ?`, id,
		); err != nil {
			return fmt.Errorf("clear venue attributes for %s: %w", id, err)
		}
	}
	return nil
}

// nullString converts an empty string to a NULL sql.NullString so optional
// text columns store NULL rather than empty string.
func nullString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}
