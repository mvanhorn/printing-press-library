package store

import "context"

// The Printing Press framework sync mirrors only flat, top-level resources.
// DNS Made Easy records are hierarchical (GET /dns/managed/{domainId}/records),
// and the generated "records" table has no zone association, so it cannot power
// cross-zone features. These extension tables give the transcendence commands
// (where-used, drift, health, export) a zone-tagged record mirror plus a
// point-in-time snapshot history for drift. They are created lazily by
// EnsureDNSMEExtensions, invoked from the hand-written commands that use them,
// so `generate --force` never clobbers them (separate file, own migration).
const (
	zoneRecordsCreateSQL = `CREATE TABLE IF NOT EXISTS zone_records (
		domain_id    TEXT NOT NULL,
		record_id    TEXT NOT NULL,
		domain_name  TEXT NOT NULL DEFAULT '',
		name         TEXT NOT NULL DEFAULT '',
		type         TEXT NOT NULL DEFAULT '',
		value        TEXT NOT NULL DEFAULT '',
		ttl          INTEGER NOT NULL DEFAULT 0,
		gtd_location TEXT NOT NULL DEFAULT '',
		mx_level     INTEGER NOT NULL DEFAULT 0,
		priority     INTEGER NOT NULL DEFAULT 0,
		weight       INTEGER NOT NULL DEFAULT 0,
		port         INTEGER NOT NULL DEFAULT 0,
		data         JSON NOT NULL DEFAULT '{}',
		synced_at    DATETIME DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (domain_id, record_id)
	)`

	zoneRecordsValueIdx = `CREATE INDEX IF NOT EXISTS idx_zone_records_value ON zone_records(value)`
	zoneRecordsTypeIdx  = `CREATE INDEX IF NOT EXISTS idx_zone_records_type ON zone_records(type)`

	// One row per record per snapshot batch. drift compares the two most
	// recent batch_id values.
	recordSnapshotsCreateSQL = `CREATE TABLE IF NOT EXISTS record_snapshots (
		batch_id     TEXT NOT NULL,
		taken_at     DATETIME NOT NULL,
		domain_id    TEXT NOT NULL,
		domain_name  TEXT NOT NULL DEFAULT '',
		record_id    TEXT NOT NULL,
		name         TEXT NOT NULL DEFAULT '',
		type         TEXT NOT NULL DEFAULT '',
		value        TEXT NOT NULL DEFAULT '',
		ttl          INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (batch_id, domain_id, record_id)
	)`

	recordSnapshotsBatchIdx = `CREATE INDEX IF NOT EXISTS idx_record_snapshots_batch ON record_snapshots(batch_id, taken_at)`
)

// EnsureDNSMEExtensions creates the DNS Made Easy zone-record mirror and
// snapshot tables if they do not already exist. Safe to call on every command
// invocation; CREATE TABLE IF NOT EXISTS is a no-op once present.
func (s *Store) EnsureDNSMEExtensions(ctx context.Context) error {
	stmts := []string{
		zoneRecordsCreateSQL,
		zoneRecordsValueIdx,
		zoneRecordsTypeIdx,
		recordSnapshotsCreateSQL,
		recordSnapshotsBatchIdx,
	}
	for _, stmt := range stmts {
		if _, err := s.DB().ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}
