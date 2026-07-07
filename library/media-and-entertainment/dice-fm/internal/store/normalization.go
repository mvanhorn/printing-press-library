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
		canonical_id       TEXT NOT NULL,
		entity_type        TEXT NOT NULL,
		canonical_name     TEXT NOT NULL,
		parent_canonical_id TEXT,
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
	// entity_attributes is the generic key/value attribute store for any entity
	// type that has no dedicated typed table (ticket_type → tier_attributes,
	// venue → venue_attributes stay authoritative). One row per (canonical_id,
	// attr_key) so an entity carries an arbitrary set of axes.
	`CREATE TABLE IF NOT EXISTS entity_attributes (
		canonical_id       TEXT NOT NULL,
		entity_type        TEXT NOT NULL,
		attr_key           TEXT NOT NULL,
		attr_value         TEXT NOT NULL,
		method             TEXT NOT NULL DEFAULT '',
		classifier_version INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (canonical_id, attr_key)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_entity_attrs_type ON entity_attributes(entity_type, attr_key)`,
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

// EntityAttrRow is a single generic attribute (one key/value pair) for a
// canonical entity in the entity_attributes table. Used for entity types that
// have no dedicated typed attribute table.
type EntityAttrRow struct {
	CanonicalID       string
	EntityType        string
	AttrKey           string
	AttrValue         string
	Method            string
	ClassifierVersion int
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
// Delegates to UpsertCanonicalEntityWithParent with no parent, keeping all
// callers working without change.
func (s *Store) UpsertCanonicalEntity(entityType, canonicalID, canonicalName string) error {
	return s.UpsertCanonicalEntityWithParent(entityType, canonicalID, canonicalName, "")
}

// UpsertCanonicalEntityWithParent inserts or updates a canonical entity record
// and optionally sets a parent relationship. An empty parentID stores SQL NULL
// rather than an empty string so the hierarchy column is unambiguously absent
// for root entities.
func (s *Store) UpsertCanonicalEntityWithParent(entityType, canonicalID, canonicalName, parentID string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.Exec(
		`INSERT INTO canonical_entity (canonical_id, entity_type, canonical_name, parent_canonical_id)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(entity_type, canonical_id) DO UPDATE SET
		 	canonical_name      = excluded.canonical_name,
		 	parent_canonical_id = excluded.parent_canonical_id`,
		canonicalID, entityType, canonicalName, nullString(parentID),
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

// UpsertExternalRef inserts or updates an entity_external_ref row keyed by
// (entity_type, canonical_id, source_system). All writes are serialized through
// writeMu, matching the locking contract of UpsertCrosswalk and UpsertBatch.
func (s *Store) UpsertExternalRef(entityType, canonicalID, sourceSystem, externalID string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.Exec(
		`INSERT INTO entity_external_ref (entity_type, canonical_id, source_system, external_id)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(entity_type, canonical_id, source_system) DO UPDATE SET
		 	external_id = excluded.external_id`,
		entityType, canonicalID, sourceSystem, externalID,
	)
	if err != nil {
		return fmt.Errorf("upsert external ref: %w", err)
	}
	return nil
}

// ListVenueAttributes returns all venue attribute rows for a given entity type
// by joining through entity_crosswalk. Results are ordered by canonical_id.
// Only canonical IDs that have a venue_attributes entry (i.e. matched venues)
// are returned — unmatched crosswalk rows are not included — mirroring
// ListTierAttributes so normalize stats counts are symmetric.
func (s *Store) ListVenueAttributes(entityType string) ([]VenueAttributesRow, error) {
	rows, err := s.db.Query(
		`SELECT va.canonical_id,
			COALESCE(va.complex,''), COALESCE(va.room,''),
			va.classifier_version, va.method
		 FROM venue_attributes va
		 WHERE va.canonical_id IN (
			SELECT DISTINCT canonical_id FROM entity_crosswalk WHERE entity_type = ?
		 )
		 ORDER BY va.canonical_id`,
		entityType,
	)
	if err != nil {
		return nil, fmt.Errorf("list venue attributes: %w", err)
	}
	defer rows.Close()

	var results []VenueAttributesRow
	for rows.Next() {
		var r VenueAttributesRow
		if err := rows.Scan(
			&r.CanonicalID, &r.Complex, &r.Room,
			&r.ClassifierVersion, &r.Method,
		); err != nil {
			return nil, fmt.Errorf("scan venue attributes: %w", err)
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// UpsertEntityAttribute inserts or replaces a single generic attribute row
// keyed by (canonical_id, attr_key). All writes are serialized through writeMu.
//
// entity_type is intentionally NOT updated on conflict: the canonical_id is a
// type-prefixed SHA1 ("<entity_type>:<hash>"), so a given canonical_id belongs
// to exactly one entity type for its whole life. Rewriting entity_type on
// conflict could only do harm — in the (astronomically unlikely) event of a
// minting collision across two types it would silently relabel the row's type
// rather than surfacing the clash — so the column is set once at insert and
// left immutable thereafter.
func (s *Store) UpsertEntityAttribute(canonicalID, entityType, key, value, method string, classifierVersion int) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.Exec(
		`INSERT INTO entity_attributes
			(canonical_id, entity_type, attr_key, attr_value, method, classifier_version)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(canonical_id, attr_key) DO UPDATE SET
			attr_value         = excluded.attr_value,
			method             = excluded.method,
			classifier_version = excluded.classifier_version`,
		canonicalID, entityType, key, value, method, classifierVersion,
	)
	if err != nil {
		return fmt.Errorf("upsert entity attribute: %w", err)
	}
	return nil
}

// ListEntityAttributes returns all generic attribute rows for a given entity
// type. Results are ordered by canonical_id, attr_key for deterministic output.
func (s *Store) ListEntityAttributes(entityType string) ([]EntityAttrRow, error) {
	rows, err := s.db.Query(
		`SELECT canonical_id, entity_type, attr_key, attr_value, method, classifier_version
		 FROM entity_attributes
		 WHERE entity_type = ?
		 ORDER BY canonical_id, attr_key`,
		entityType,
	)
	if err != nil {
		return nil, fmt.Errorf("list entity attributes: %w", err)
	}
	defer rows.Close()

	var results []EntityAttrRow
	for rows.Next() {
		var r EntityAttrRow
		if err := rows.Scan(
			&r.CanonicalID, &r.EntityType, &r.AttrKey, &r.AttrValue,
			&r.Method, &r.ClassifierVersion,
		); err != nil {
			return nil, fmt.Errorf("scan entity attribute: %w", err)
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// ClearNormalization removes all non-manual normalization rows for the given
// entity type from entity_crosswalk and the corresponding attribute tables.
// Rows with method='manual' are preserved so operator overrides survive a
// re-classification run.
//
// The whole multi-table clear runs inside a single transaction so the store is
// never observed in a half-cleared state (e.g. crosswalk rows gone but attribute
// rows still present): either every non-manual row is removed or none is. Writes
// are also serialized through writeMu, matching the locking contract of the
// other store mutators.
func (s *Store) ClearNormalization(entityType string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin clear normalization: %w", err)
	}
	defer tx.Rollback()

	// Collect canonical IDs that are exclusively non-manual before deleting,
	// so we can clean up the attribute tables without touching manual entries.
	rows, err := tx.Query(
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
	if _, err := tx.Exec(
		`DELETE FROM entity_crosswalk WHERE entity_type = ? AND method <> 'manual'`,
		entityType,
	); err != nil {
		return fmt.Errorf("clear crosswalk: %w", err)
	}

	// Delete tier/venue attributes only for canonical IDs that had no manual
	// crosswalk row (collected above).
	for _, id := range ids {
		if _, err := tx.Exec(
			`DELETE FROM tier_attributes WHERE canonical_id = ?`, id,
		); err != nil {
			return fmt.Errorf("clear tier attributes for %s: %w", id, err)
		}
		if _, err := tx.Exec(
			`DELETE FROM venue_attributes WHERE canonical_id = ?`, id,
		); err != nil {
			return fmt.Errorf("clear venue attributes for %s: %w", id, err)
		}
	}

	// Delete non-manual generic attributes for this entity type. Unlike the
	// typed tier/venue tables (which key manual-preservation off the crosswalk
	// method), entity_attributes carries its own per-row method, so manual rows
	// are identified directly and left in place.
	if _, err := tx.Exec(
		`DELETE FROM entity_attributes WHERE entity_type = ? AND method <> 'manual'`,
		entityType,
	); err != nil {
		return fmt.Errorf("clear entity attributes: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit clear normalization: %w", err)
	}
	return nil
}

// nullString converts an empty string to a NULL sql.NullString so optional
// text columns store NULL rather than empty string.
func nullString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}
