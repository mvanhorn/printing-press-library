// Copyright 2026 Omar Shahine and contributors. Licensed under Apache-2.0. See LICENSE.

package registrydb

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Watch is a registered owner or tail watch.
type Watch struct {
	ID      int64  `json:"id"`
	Kind    string `json:"kind"` // "owner" or "tail"
	Value   string `json:"value"`
	AddedAt string `json:"added_at"`
}

// AddWatch registers an owner-name or tail-number watch.
func (d *DB) AddWatch(ctx context.Context, kind, value string) (*Watch, error) {
	kind = strings.ToLower(strings.TrimSpace(kind))
	if kind != "owner" && kind != "tail" {
		return nil, fmt.Errorf("watch kind must be owner or tail")
	}
	v := strings.ToUpper(strings.TrimSpace(value))
	if kind == "tail" {
		v = NormalizeTail(v)
	}
	if v == "" {
		return nil, fmt.Errorf("watch value must not be empty")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := d.db.ExecContext(ctx, `INSERT OR IGNORE INTO faa_watches (kind, value, added_at) VALUES (?, ?, ?)`, kind, v, now)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, fmt.Errorf("watch already exists for %s %q", kind, value)
	}
	id, _ := res.LastInsertId()
	return &Watch{ID: id, Kind: kind, Value: v, AddedAt: now}, nil
}

// RemoveWatch deletes a watch and its snapshot state.
func (d *DB) RemoveWatch(ctx context.Context, kind, value string) error {
	kind = strings.ToLower(strings.TrimSpace(kind))
	v := strings.ToUpper(strings.TrimSpace(value))
	if kind == "tail" {
		v = NormalizeTail(v)
	}
	var id int64
	err := d.db.QueryRowContext(ctx, `SELECT id FROM faa_watches WHERE kind = ? AND value = ?`, kind, v).Scan(&id)
	if err != nil {
		return fmt.Errorf("no %s watch for %q", kind, value)
	}
	if _, err := d.db.ExecContext(ctx, `DELETE FROM faa_watch_state WHERE watch_id = ?`, id); err != nil {
		return err
	}
	_, err = d.db.ExecContext(ctx, `DELETE FROM faa_watches WHERE id = ?`, id)
	return err
}

// ListWatches returns all registered watches.
func (d *DB) ListWatches(ctx context.Context) ([]Watch, error) {
	rows, err := d.db.QueryContext(ctx, `SELECT id, kind, value, added_at FROM faa_watches ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Watch
	for rows.Next() {
		var w Watch
		if err := rows.Scan(&w.ID, &w.Kind, &w.Value, &w.AddedAt); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// WatchChange is one detected difference for a watch.
type WatchChange struct {
	Change   string    `json:"change"` // "added", "removed", "changed"
	Aircraft *Aircraft `json:"aircraft,omitempty"`
	NNumber  string    `json:"n_number"`
}

// WatchReport is the diff result for one watch.
type WatchReport struct {
	Watch    Watch         `json:"watch"`
	Baseline bool          `json:"baseline"` // true when this run recorded the first snapshot
	Changes  []WatchChange `json:"changes"`
}

// CheckWatches diffs every watch against its stored snapshot and updates the
// snapshots. The first check for a watch records a baseline and reports no
// changes.
func (d *DB) CheckWatches(ctx context.Context) ([]WatchReport, error) {
	if err := d.requireSynced(ctx); err != nil {
		return nil, err
	}
	watches, err := d.ListWatches(ctx)
	if err != nil {
		return nil, err
	}
	var reports []WatchReport
	for _, w := range watches {
		rep, err := d.checkWatch(ctx, w)
		if err != nil {
			return nil, fmt.Errorf("checking %s %q: %w", w.Kind, w.Value, err)
		}
		reports = append(reports, *rep)
	}
	return reports, nil
}

func (d *DB) checkWatch(ctx context.Context, w Watch) (*WatchReport, error) {
	var current []*Aircraft
	var err error
	switch w.Kind {
	case "owner":
		current, err = d.FleetAircraft(ctx, w.Value)
	case "tail":
		var ac *Aircraft
		ac, err = d.LookupTail(ctx, w.Value)
		if ac != nil {
			current = []*Aircraft{ac}
		}
	default:
		return nil, fmt.Errorf("unknown watch kind %q", w.Kind)
	}
	if err != nil {
		return nil, err
	}

	curHashes := map[string]string{}
	curByTail := map[string]*Aircraft{}
	for _, ac := range current {
		n := NormalizeTail(ac.NNumber)
		b, _ := json.Marshal(ac)
		sum := sha256.Sum256(b)
		curHashes[n] = hex.EncodeToString(sum[:])
		curByTail[n] = ac
	}

	prev := map[string]string{}
	rows, err := d.db.QueryContext(ctx, `SELECT n_number, row_hash FROM faa_watch_state WHERE watch_id = ?`, w.ID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var n, h string
		if err := rows.Scan(&n, &h); err != nil {
			rows.Close()
			return nil, err
		}
		prev[n] = h
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	rep := &WatchReport{Watch: w, Baseline: len(prev) == 0, Changes: []WatchChange{}}
	if !rep.Baseline {
		for n, h := range curHashes {
			if ph, ok := prev[n]; !ok {
				rep.Changes = append(rep.Changes, WatchChange{Change: "added", NNumber: "N" + n, Aircraft: curByTail[n]})
			} else if ph != h {
				rep.Changes = append(rep.Changes, WatchChange{Change: "changed", NNumber: "N" + n, Aircraft: curByTail[n]})
			}
		}
		for n := range prev {
			if _, ok := curHashes[n]; !ok {
				rep.Changes = append(rep.Changes, WatchChange{Change: "removed", NNumber: "N" + n})
			}
		}
	}

	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM faa_watch_state WHERE watch_id = ?`, w.ID); err != nil {
		return nil, err
	}
	for n, h := range curHashes {
		if _, err := tx.ExecContext(ctx, `INSERT INTO faa_watch_state (watch_id, n_number, row_hash) VALUES (?, ?, ?)`, w.ID, n, h); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return rep, nil
}
