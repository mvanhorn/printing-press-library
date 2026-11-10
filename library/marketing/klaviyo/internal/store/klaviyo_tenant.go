// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/platform"
	"os"
	"path/filepath"
	"time"
)

func OpenVerifiedWithContext(ctx context.Context, dbPath string, session *platform.Session) (*Store, error) {
	if session == nil || session.GateOutcome != platform.GateVerified {
		return nil, fmt.Errorf("verified client profile session is required")
	}
	return openWithContext(ctx, dbPath, session)
}

func (s *Store) ensureTenantMetadata(ctx context.Context, session *platform.Session) error {
	deadline := time.Now().Add(migrationLockTimeout)
	var conn *sql.Conn
	if err := retryOnBusy(ctx, deadline, "acquiring tenant metadata connection", func() error {
		var err error
		conn, err = s.db.Conn(ctx)
		return err
	}); err != nil {
		return err
	}
	defer conn.Close()
	// Decide ownership under the SQLite write lock, after any competing opener commits.
	if err := withMigrationLock(ctx, conn, deadline, func() error {
		var tables, metadataTables int
		if err := conn.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(SUM(name = 'pp_tenant_metadata' AND type = 'table'), 0)
			FROM sqlite_master WHERE name NOT GLOB 'sqlite_*'`).Scan(&tables, &metadataTables); err != nil {
			return err
		}
		if metadataTables != 0 {
			return nil
		}
		if tables != 0 {
			return platform.ErrTenantMetadataMissing
		}
		return platform.InitializeTenantMetadataOn(ctx, conn, session, "new-profile-store")
	}); err != nil {
		return err
	}
	return retryOnBusy(ctx, deadline, "verifying tenant metadata", func() error {
		return platform.VerifyTenantMetadata(ctx, s.db, session)
	})
}

func InspectResourceHealth(dbPath string, session *platform.Session) ([]platform.ResourceHealth, error) {
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return []platform.ResourceHealth{}, nil
	} else if err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(dbPath)+"?mode=ro")
	if err != nil {
		return nil, err
	}
	defer db.Close()
	if err := platform.VerifyTenantMetadataReadOnly(context.Background(), db, session); err != nil {
		return nil, fmt.Errorf("verifying database tenant metadata: %w", err)
	}
	rows, err := db.Query(`SELECT resource_type, COUNT(*), MAX(updated_at), MAX(synced_at) FROM resources GROUP BY resource_type ORDER BY resource_type`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []platform.ResourceHealth{}
	for rows.Next() {
		var name string
		var count int64
		var newestRaw, syncedRaw sql.NullString
		if err := rows.Scan(&name, &count, &newestRaw, &syncedRaw); err != nil {
			return nil, err
		}
		health := platform.ResourceHealth{Name: name, Rows: count, Freshness: "unknown"}
		if newestRaw.Valid {
			if parsed, err := time.Parse(time.RFC3339, newestRaw.String); err == nil {
				health.NewestRowAt = &parsed
			}
		}
		if syncedRaw.Valid {
			if parsed, err := time.Parse(time.RFC3339, syncedRaw.String); err == nil {
				health.LastSyncedAt = &parsed
				if time.Since(parsed) > 24*time.Hour {
					health.Freshness = "stale"
				} else {
					health.Freshness = "fresh"
				}
			}
		}
		result = append(result, health)
	}
	return result, rows.Err()
}

func OpenVerifiedReadOnlyWithContext(ctx context.Context, dbPath string, session *platform.Session) (*Store, error) {
	if session == nil || session.GateOutcome != platform.GateVerified {
		return nil, fmt.Errorf("verified client profile session is required")
	}
	db, err := OpenReadOnlyContext(ctx, dbPath)
	if err != nil {
		return nil, err
	}
	if err := platform.VerifyTenantMetadataReadOnly(ctx, db.DB(), session); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("verifying database tenant metadata: %w", err)
	}
	return db, nil
}
