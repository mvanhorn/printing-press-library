package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/government-data/internal/govcache"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/government-data/internal/govdata"
)

// SaveCatalogSnapshot atomically publishes the immutable evidence and every
// offline search surface. An index failure never consumes the snapshot ID.
func (s *Store) SaveCatalogSnapshot(ctx context.Context, snapshot govdata.Snapshot) error {
	s.lockForWrite()
	defer s.unlockAfterWrite()
	if err := govcache.Init(ctx, s.db); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := govcache.SaveTx(ctx, tx, snapshot); err != nil {
		return err
	}
	for _, d := range snapshot.Datasets {
		data, err := json.Marshal(d)
		if err != nil {
			return err
		}
		obj, err := DecodeJSONObject(data)
		if err != nil {
			return err
		}
		id := resourceStorageID(d.Source, d.ID, obj)
		if err := upsertCatalogIndexTx(ctx, tx, d.Source, id, data); err != nil {
			return err
		}
		switch d.Source {
		case "california":
			err = s.upsertCaliforniaTx(tx, id, obj, data)
		case "federal":
			err = s.upsertFederalTx(tx, id, obj, data)
		default:
			return fmt.Errorf("unsupported catalog source %q", d.Source)
		}
		if err != nil {
			return err
		}
		if err := upsertCatalogIndexTx(ctx, tx, "datasets", d.Source+":"+d.ID, data); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Catalog saves promise searchable evidence, so FTS failures are fatal here.
// Other generated store paths retain their existing best-effort FTS policy.
func upsertCatalogIndexTx(ctx context.Context, tx *sql.Tx, resourceType, id string, data json.RawMessage) error {
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := tx.ExecContext(ctx, `INSERT INTO resources(id,resource_type,data,synced_at,updated_at) VALUES(?,?,?,?,?) ON CONFLICT(resource_type,id) DO UPDATE SET data=excluded.data,synced_at=excluded.synced_at,updated_at=excluded.updated_at`, id, resourceType, string(data), now, now); err != nil {
		return err
	}
	rowid := ftsRowID(resourceType, id)
	if _, err := tx.ExecContext(ctx, `DELETE FROM resources_fts WHERE rowid=?`, rowid); err != nil {
		return fmt.Errorf("catalog search index cleanup: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO resources_fts(rowid,id,resource_type,content) VALUES(?,?,?,?)`, rowid, id, resourceType, searchableResourceContent(data)); err != nil {
		return fmt.Errorf("catalog search index update: %w", err)
	}
	return nil
}
