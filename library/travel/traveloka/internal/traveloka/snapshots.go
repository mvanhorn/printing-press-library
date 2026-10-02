package traveloka

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/traveloka/internal/store"
)

// MarshalJSON makes empty result lists arrays, while preserving unknown scalar pointers as null.
func (s Snapshot) MarshalJSON() ([]byte, error) {
	type plain Snapshot
	value := plain(s)
	if value.Offers == nil {
		value.Offers = []Offer{}
	}
	if value.Warnings == nil {
		value.Warnings = []string{}
	}
	if value.Query.ChildAges == nil {
		value.Query.ChildAges = []int{}
	}
	return json.Marshal(value)
}
func (o Offer) MarshalJSON() ([]byte, error) {
	type plain Offer
	value := plain(o)
	if value.Legs == nil {
		value.Legs = []Leg{}
	}
	return json.Marshal(value)
}
func (l Leg) MarshalJSON() ([]byte, error) {
	type plain Leg
	value := plain(l)
	if value.Segments == nil {
		value.Segments = []Segment{}
	}
	return json.Marshal(value)
}
func normalizedSnapshot(snapshot *Snapshot) (*Snapshot, []byte, error) {
	if snapshot == nil || snapshot.ID == "" || snapshot.Kind == "" {
		return nil, nil, apiError("INVALID_INPUT", "snapshot ID and kind are required", 0, false)
	}
	if _, err := time.Parse(time.RFC3339Nano, snapshot.RetrievedAt); err != nil {
		return nil, nil, apiError("INVALID_INPUT", "snapshot requires an original RFC3339 retrieval time", 0, false)
	}
	b, err := json.Marshal(snapshot)
	if err != nil {
		return nil, nil, err
	}
	var obj map[string]any
	if err = decodeJSON(b, &obj); err != nil {
		return nil, nil, err
	}
	b, err = json.Marshal(Sanitize(obj))
	if err != nil {
		return nil, nil, err
	}
	var normalized Snapshot
	if err = decodeJSON(b, &normalized); err != nil {
		return nil, nil, err
	}
	normalized.Indicative = true
	if normalized.Freshness == "" {
		normalized.Freshness = "fresh"
	}
	b, err = json.Marshal(normalized)
	return &normalized, b, err
}

const snapshotTable = `CREATE TABLE IF NOT EXISTS traveloka_snapshots (id TEXT PRIMARY KEY, kind TEXT NOT NULL, retrieved_at TEXT NOT NULL, snapshot JSON NOT NULL)`
const snapshotIndex = `CREATE INDEX IF NOT EXISTS traveloka_snapshots_kind_time ON traveloka_snapshots(kind,retrieved_at DESC)`

// SaveSnapshot stores normalized public JSON and searchable generated resource/FTS records atomically.
func SaveSnapshot(ctx context.Context, db *store.Store, snapshot *Snapshot) error {
	if db == nil {
		return fmt.Errorf("snapshot store is required")
	}
	normalized, b, err := normalizedSnapshot(snapshot)
	if err != nil {
		return err
	}
	tx, err := db.DB().BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, snapshotTable); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, snapshotIndex); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO traveloka_snapshots(id,kind,retrieved_at,snapshot) VALUES(?,?,?,?) ON CONFLICT(id) DO UPDATE SET kind=excluded.kind,retrieved_at=excluded.retrieved_at,snapshot=excluded.snapshot`, normalized.ID, normalized.Kind, normalized.RetrievedAt, string(b)); err != nil {
		return err
	}
	if err = indexPublicResource(ctx, tx, "traveloka_snapshots", normalized.ID, b, normalized.RetrievedAt); err != nil {
		return err
	}
	for i, offer := range normalized.Offers {
		offerID := offer.ID
		if offerID == "" {
			offerID = fmt.Sprintf("position-%d", i)
		}
		entity := map[string]any{"snapshot_id": normalized.ID, "retrieved_at": normalized.RetrievedAt, "query": normalized.Query, "offer": offer}
		ob, err := json.Marshal(entity)
		if err != nil {
			return err
		}
		if err = indexPublicResource(ctx, tx, "traveloka_offers", normalized.ID+":"+offerID, ob, normalized.RetrievedAt); err != nil {
			return err
		}
	}
	return tx.Commit()
}
func indexPublicResource(ctx context.Context, tx *sql.Tx, kind, id string, b []byte, retrievedAt string) error {
	if _, err := tx.ExecContext(ctx, `INSERT INTO resources(id,resource_type,data,synced_at,updated_at) VALUES(?,?,?,?,?) ON CONFLICT(resource_type,id) DO UPDATE SET data=excluded.data,synced_at=excluded.synced_at,updated_at=excluded.updated_at`, id, kind, string(b), retrievedAt, retrievedAt); err != nil {
		return err
	}
	// Match the generated store's deterministic FTS row ID derivation.
	h := fnv.New64a()
	_, _ = h.Write([]byte(kind))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(id))
	rowID := int64(h.Sum64() & 0x7fffffffffffffff)
	if _, err := tx.ExecContext(ctx, `DELETE FROM resources_fts WHERE rowid=?`, rowID); err != nil {
		return err
	}
	var value any
	if err := decodeJSON(b, &value); err != nil {
		return err
	}
	var leaves []string
	collectStrings(value, &leaves)
	_, err := tx.ExecContext(ctx, `INSERT INTO resources_fts(rowid,id,resource_type,content) VALUES(?,?,?,?)`, rowID, id, kind, strings.Join(leaves, " "))
	return err
}

// LoadSnapshot returns the original retrieval time; an empty ID selects the latest matching kind.
func LoadSnapshot(ctx context.Context, db *store.Store, id, kind string) (*Snapshot, error) {
	if db == nil {
		return nil, fmt.Errorf("snapshot store is required")
	}
	var exists string
	if err := db.DB().QueryRowContext(ctx, `SELECT name FROM sqlite_master WHERE type='table' AND name='traveloka_snapshots'`).Scan(&exists); err != nil {
		return nil, err
	}
	query := `SELECT snapshot FROM traveloka_snapshots WHERE 1=1`
	args := []any{}
	if id != "" {
		query += ` AND id=?`
		args = append(args, id)
	}
	if kind != "" {
		query += ` AND kind=?`
		args = append(args, kind)
	}
	query += ` ORDER BY julianday(retrieved_at) DESC,rowid DESC LIMIT 1`
	var b string
	if err := db.DB().QueryRowContext(ctx, query, args...).Scan(&b); err != nil {
		return nil, err
	}
	var snapshot Snapshot
	if err := decodeJSON([]byte(b), &snapshot); err != nil {
		return nil, err
	}
	normalized, _, err := normalizedSnapshot(&snapshot)
	if err != nil {
		return nil, err
	}
	normalized.Freshness = "saved_snapshot"
	return normalized, nil
}
func SaveSnapshotFile(path string, snapshot *Snapshot) error {
	_, b, err := normalizedSnapshot(snapshot)
	if err != nil {
		return err
	}
	if path == "" {
		return apiError("INVALID_INPUT", "snapshot output path is required", 0, false)
	}
	return os.WriteFile(path, append(b, '\n'), 0600)
}
func ReadSnapshotFile(path string) (*Snapshot, error) {
	b, err := readBoundedFile(path)
	if err != nil {
		return nil, err
	}
	var snapshot Snapshot
	if err := decodeJSON(b, &snapshot); err != nil {
		return nil, err
	}
	normalized, _, err := normalizedSnapshot(&snapshot)
	if err != nil {
		return nil, err
	}
	normalized.Freshness = "saved_snapshot"
	return normalized, nil
}
