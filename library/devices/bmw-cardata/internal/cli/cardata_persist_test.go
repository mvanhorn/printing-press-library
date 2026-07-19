// Copyright 2026 jvm and contributors. Licensed under Apache-2.0.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/devices/bmw-cardata/internal/store"
)

func TestPersistCardataTelematicDataStoresMissingTimestampAsNull(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "cardata.db")
	persistCardataTelematicData(dbPath, "WBA00000000000000", json.RawMessage(`{
		"telematicData": {
			"vehicle.drivetrain.batteryManagement.header": {"value": "67", "unit": "%"}
		}
	}`))

	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()

	var timestamp sql.NullString
	if err := db.DB().QueryRow(
		`SELECT ts FROM cardata_telematic_snapshots WHERE vin = ? AND descriptor = ?`,
		"WBA00000000000000", "vehicle.drivetrain.batteryManagement.header",
	).Scan(&timestamp); err != nil {
		t.Fatalf("query persisted timestamp: %v", err)
	}
	if timestamp.Valid {
		t.Fatalf("missing BMW timestamp stored as %q; want SQL NULL", timestamp.String)
	}
}

func TestTelematicSnapshotDedupNormalizesMissingTimestamps(t *testing.T) {
	db, err := store.OpenWithContext(context.Background(), filepath.Join(t.TempDir(), "cardata.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()

	insert := func(descriptor string, timestamp any, fetchedAt string) int64 {
		t.Helper()
		result, err := db.DB().Exec(
			`INSERT OR IGNORE INTO cardata_telematic_snapshots
			 (vin, descriptor, value, unit, ts, fetched_at) VALUES(?,?,?,?,?,?)`,
			"WBA00000000000000", descriptor, "67", "%", timestamp, fetchedAt,
		)
		if err != nil {
			t.Fatalf("insert %s: %v", descriptor, err)
		}
		n, err := result.RowsAffected()
		if err != nil {
			t.Fatalf("rows affected: %v", err)
		}
		return n
	}

	if got := insert("missing-ts", "", "2026-07-19T10:00:00Z"); got != 1 {
		t.Fatalf("legacy empty timestamp first insert affected %d rows; want 1", got)
	}
	if got := insert("missing-ts", nil, "2026-07-19T10:01:00Z"); got != 1 {
		t.Fatalf("fresh missing-timestamp observation affected %d rows; want 1", got)
	}
	if got := insert("missing-ts", nil, "2026-07-19T10:01:00Z"); got != 0 {
		t.Fatalf("same-fetch missing-timestamp replay affected %d rows; want 0", got)
	}

	if got := insert("timestamped", "2026-07-19T09:59:00Z", "2026-07-19T10:00:00Z"); got != 1 {
		t.Fatalf("timestamped first insert affected %d rows; want 1", got)
	}
	if got := insert("timestamped", "2026-07-19T09:59:00Z", "2026-07-19T10:01:00Z"); got != 0 {
		t.Fatalf("timestamped replay affected %d rows; want 0", got)
	}
}
