package store

import (
	"database/sql"
	"fmt"
	"time"
)

// Hand-authored extension (Slice 2): journal + snapshot tables for
// `listings bulk-edit` and `listings snapshot`. Tables are created lazily
// (CREATE TABLE IF NOT EXISTS) the first time a Bulk*/Listing* method runs, so
// the generated schema/migration list is never edited.

const bulkSchemaSQL = `
CREATE TABLE IF NOT EXISTS bulk_edit_jobs (
	id TEXT PRIMARY KEY,
	source TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'running',
	total_ops INTEGER NOT NULL DEFAULT 0,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS bulk_edit_items (
	job_id TEXT NOT NULL,
	op_key TEXT NOT NULL,
	listing_id INTEGER NOT NULL,
	kind TEXT NOT NULL,
	status TEXT NOT NULL DEFAULT 'pending',
	request_json TEXT NOT NULL DEFAULT '',
	error TEXT NOT NULL DEFAULT '',
	attempts INTEGER NOT NULL DEFAULT 0,
	snapshot_id INTEGER,
	updated_at TEXT NOT NULL,
	PRIMARY KEY (job_id, op_key)
);
CREATE INDEX IF NOT EXISTS idx_bulk_edit_items_status ON bulk_edit_items(job_id, status);
CREATE TABLE IF NOT EXISTS listing_snapshots (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	listing_id INTEGER NOT NULL,
	taken_at TEXT NOT NULL,
	source TEXT NOT NULL DEFAULT '',
	listing_json TEXT NOT NULL DEFAULT '',
	inventory_json TEXT NOT NULL DEFAULT '',
	personalization_json TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_listing_snapshots_listing ON listing_snapshots(listing_id, taken_at);
`

// Journal item statuses.
const (
	BulkStatusPending = "pending"
	BulkStatusDone    = "done"
	BulkStatusFailed  = "failed"
)

// BulkOp is one journaled write unit of a bulk-edit job.
type BulkOp struct {
	JobID      string `json:"job_id"`
	OpKey      string `json:"op_key"`
	ListingID  int64  `json:"listing_id"`
	Kind       string `json:"kind"`
	Status     string `json:"status"`
	Request    string `json:"request_json,omitempty"`
	Error      string `json:"error,omitempty"`
	Attempts   int    `json:"attempts"`
	SnapshotID int64  `json:"snapshot_id,omitempty"`
	UpdatedAt  string `json:"updated_at"`
}

// ListingSnapshotRow is one stored pre-write snapshot of a listing.
type ListingSnapshotRow struct {
	ID                  int64  `json:"id"`
	ListingID           int64  `json:"listing_id"`
	TakenAt             string `json:"taken_at"`
	Source              string `json:"source"`
	ListingJSON         string `json:"listing_json,omitempty"`
	InventoryJSON       string `json:"inventory_json,omitempty"`
	PersonalizationJSON string `json:"personalization_json,omitempty"`
}

// BulkEnsureTables lazily creates the bulk-edit journal and snapshot tables.
func (s *Store) BulkEnsureTables() error {
	s.lockForWrite()
	defer s.unlockAfterWrite()
	_, err := s.db.Exec(bulkSchemaSQL)
	return err
}

func bulkNow() string { return time.Now().UTC().Format(time.RFC3339Nano) }

// BulkStartJob creates the job row if absent. With reset, any prior journal for
// the job is discarded so the plan re-runs from scratch.
func (s *Store) BulkStartJob(id, source string, totalOps int, reset bool) error {
	if err := s.BulkEnsureTables(); err != nil {
		return err
	}
	s.lockForWrite()
	defer s.unlockAfterWrite()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if reset {
		if _, err := tx.Exec(`DELETE FROM bulk_edit_items WHERE job_id = ?`, id); err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM bulk_edit_jobs WHERE id = ?`, id); err != nil {
			return err
		}
	}
	now := bulkNow()
	if _, err := tx.Exec(`INSERT INTO bulk_edit_jobs (id, source, status, total_ops, created_at, updated_at)
		VALUES (?, ?, 'running', ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET status='running', total_ops=excluded.total_ops, updated_at=excluded.updated_at`,
		id, source, totalOps, now, now); err != nil {
		return err
	}
	return tx.Commit()
}

// BulkRegisterOp inserts a pending journal item unless one already exists
// (a resumed run keeps the prior status).
func (s *Store) BulkRegisterOp(op BulkOp) error {
	s.lockForWrite()
	defer s.unlockAfterWrite()
	_, err := s.db.Exec(`INSERT OR IGNORE INTO bulk_edit_items
		(job_id, op_key, listing_id, kind, status, request_json, updated_at)
		VALUES (?, ?, ?, ?, 'pending', ?, ?)`,
		op.JobID, op.OpKey, op.ListingID, op.Kind, op.Request, bulkNow())
	return err
}

// BulkOpStatus returns the journaled status of an op ("" when unknown).
func (s *Store) BulkOpStatus(jobID, opKey string) (string, error) {
	var st string
	err := s.db.QueryRow(`SELECT status FROM bulk_edit_items WHERE job_id = ? AND op_key = ?`, jobID, opKey).Scan(&st)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return st, err
}

// BulkMarkOp records an op outcome and bumps its attempt counter.
func (s *Store) BulkMarkOp(jobID, opKey, status, errMsg string, snapshotID int64) error {
	s.lockForWrite()
	defer s.unlockAfterWrite()
	var snap any
	if snapshotID > 0 {
		snap = snapshotID
	}
	res, err := s.db.Exec(`UPDATE bulk_edit_items
		SET status = ?, error = ?, attempts = attempts + 1, snapshot_id = COALESCE(?, snapshot_id), updated_at = ?
		WHERE job_id = ? AND op_key = ?`, status, errMsg, snap, bulkNow(), jobID, opKey)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("bulk op %q not registered for job %q", opKey, jobID)
	}
	return nil
}

// BulkFinishJob sets the terminal status of a job.
func (s *Store) BulkFinishJob(jobID, status string) error {
	s.lockForWrite()
	defer s.unlockAfterWrite()
	_, err := s.db.Exec(`UPDATE bulk_edit_jobs SET status = ?, updated_at = ? WHERE id = ?`, status, bulkNow(), jobID)
	return err
}

// BulkJobOps lists a job's journal items ordered by listing then key.
func (s *Store) BulkJobOps(jobID string) ([]BulkOp, error) {
	rows, err := s.db.Query(`SELECT job_id, op_key, listing_id, kind, status, request_json, error, attempts, COALESCE(snapshot_id,0), updated_at
		FROM bulk_edit_items WHERE job_id = ? ORDER BY listing_id, op_key`, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]BulkOp, 0)
	for rows.Next() {
		var o BulkOp
		if err := rows.Scan(&o.JobID, &o.OpKey, &o.ListingID, &o.Kind, &o.Status, &o.Request, &o.Error, &o.Attempts, &o.SnapshotID, &o.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// SaveListingSnapshot appends a snapshot row and returns its id.
func (s *Store) SaveListingSnapshot(listingID int64, source, listingJSON, inventoryJSON, personalizationJSON string) (int64, error) {
	if err := s.BulkEnsureTables(); err != nil {
		return 0, err
	}
	s.lockForWrite()
	defer s.unlockAfterWrite()
	res, err := s.db.Exec(`INSERT INTO listing_snapshots (listing_id, taken_at, source, listing_json, inventory_json, personalization_json)
		VALUES (?, ?, ?, ?, ?, ?)`, listingID, bulkNow(), source, listingJSON, inventoryJSON, personalizationJSON)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ListListingSnapshots returns snapshot history, newest first. listingID 0
// lists every listing. withBodies includes the stored JSON payloads.
func (s *Store) ListListingSnapshots(listingID int64, limit int, withBodies bool) ([]ListingSnapshotRow, error) {
	if err := s.BulkEnsureTables(); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 50
	}
	cols := `id, listing_id, taken_at, source, '', '', ''`
	if withBodies {
		cols = `id, listing_id, taken_at, source, listing_json, inventory_json, personalization_json`
	}
	q := `SELECT ` + cols + ` FROM listing_snapshots`
	args := []any{}
	if listingID > 0 {
		q += ` WHERE listing_id = ?`
		args = append(args, listingID)
	}
	q += ` ORDER BY id DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]ListingSnapshotRow, 0)
	for rows.Next() {
		var r ListingSnapshotRow
		if err := rows.Scan(&r.ID, &r.ListingID, &r.TakenAt, &r.Source, &r.ListingJSON, &r.InventoryJSON, &r.PersonalizationJSON); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// BulkJobStatus returns an empty status when the job is absent.
func (s *Store) BulkJobStatus(id string) (string, error) {
	var status string
	err := s.db.QueryRow(`SELECT status FROM bulk_edit_jobs WHERE id = ?`, id).Scan(&status)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil { // A fresh mirror may not have the lazily created journal yet.
		var n int
		if e := s.db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='bulk_edit_jobs'`).Scan(&n); e == nil && n == 0 {
			return "", nil
		}
	}
	return status, err
}
