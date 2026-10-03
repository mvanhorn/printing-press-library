// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package hgj

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"
)

const SnapshotTable = "hgj_detail_snapshots"
const maxSavedPlaces = 1000

type Selection struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}
type MissingSnapshot struct {
	Selection
	State string `json:"state"`
	Hint  string `json:"hint"`
}
type LoadedSnapshots struct {
	Places  []Place           `json:"results"`
	Missing []MissingSnapshot `json:"missing_snapshots"`
}

// SaveSnapshot retains the two newest successful observations by source observation time.
func SaveSnapshot(ctx context.Context, db *sql.DB, p Place) error {
	canonical, err := CanonicalURL(p.Kind, p.ID)
	if err != nil {
		return err
	}
	if p.EvidenceScope != "detail" || p.SourceURL != canonical || p.Name == "" {
		return fmt.Errorf("only a successful canonical full-detail inspection can be saved")
	}
	observed, err := time.Parse(time.RFC3339Nano, p.ObservedAt)
	if err != nil {
		return fmt.Errorf("invalid observation time: %w", err)
	}
	raw, err := json.Marshal(p)
	if err != nil {
		return err
	}
	if _, err = db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS hgj_detail_snapshots(kind TEXT NOT NULL,id TEXT NOT NULL,position INTEGER NOT NULL CHECK(position IN(0,1)),observed_at TEXT NOT NULL,data TEXT NOT NULL,PRIMARY KEY(kind,id,position))`); err != nil {
		return fmt.Errorf("create snapshot cache: %w", err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	type observation struct {
		at         time.Time
		stamp, raw string
	}
	candidates := []observation{{observed, p.ObservedAt, string(raw)}}
	rows, err := tx.QueryContext(ctx, `SELECT observed_at,data FROM hgj_detail_snapshots WHERE kind=? AND id=? ORDER BY position`, p.Kind, p.ID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var old observation
		if err = rows.Scan(&old.stamp, &old.raw); err != nil {
			_ = rows.Close()
			return err
		}
		old.at, err = time.Parse(time.RFC3339Nano, old.stamp)
		if err != nil {
			_ = rows.Close()
			return fmt.Errorf("invalid saved observation time: %w", err)
		}
		candidates = append(candidates, old)
	}
	err = rows.Err()
	closeErr := rows.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if len(candidates) == 1 {
		var count int
		if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM hgj_detail_snapshots WHERE position=0`).Scan(&count); err != nil {
			return err
		}
		if count >= maxSavedPlaces {
			return fmt.Errorf("full-detail cache reached its %d-place cap; use --no-cache for an uncached inspection or another --db", maxSavedPlaces)
		}
	}
	// Incoming observations win timestamp ties; saved order breaks remaining ties.
	sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].at.After(candidates[j].at) })
	if len(candidates) > 2 {
		candidates = candidates[:2]
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM hgj_detail_snapshots WHERE kind=? AND id=?`, p.Kind, p.ID); err != nil {
		return err
	}
	for position, observation := range candidates {
		if _, err = tx.ExecContext(ctx, `INSERT INTO hgj_detail_snapshots(kind,id,position,observed_at,data) VALUES(?,?,?,?,?)`, p.Kind, p.ID, position, observation.stamp, observation.raw); err != nil {
			return err
		}
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("save full-detail snapshot: %w", err)
	}
	return nil
}
func Snapshot(ctx context.Context, db *sql.DB, s Selection, position int) (Place, bool, error) {
	if _, err := CanonicalURL(s.Kind, s.ID); err != nil {
		return Place{}, false, err
	}
	if position != 0 && position != 1 {
		return Place{}, false, fmt.Errorf("snapshot position must be 0 or 1")
	}
	var table int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, SnapshotTable).Scan(&table); err != nil {
		return Place{}, false, err
	}
	if table == 0 {
		return Place{}, false, nil
	}
	var raw string
	err := db.QueryRowContext(ctx, `SELECT data FROM hgj_detail_snapshots WHERE kind=? AND id=? AND position=?`, s.Kind, s.ID, position).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return Place{}, false, nil
	}
	if err != nil {
		return Place{}, false, fmt.Errorf("read snapshot: %w", err)
	}
	var p Place
	if err = json.Unmarshal([]byte(raw), &p); err != nil {
		return Place{}, false, fmt.Errorf("corrupt saved snapshot: %w", err)
	}
	canonical, _ := CanonicalURL(s.Kind, s.ID)
	if p.EvidenceScope != "detail" || p.Kind != s.Kind || p.ID != s.ID || p.SourceURL != canonical {
		return Place{}, false, fmt.Errorf("saved record is not a canonical full-detail snapshot; inspect it again")
	}
	return p, true, nil
}
func LoadSnapshots(ctx context.Context, db *sql.DB, selections []Selection) (LoadedSnapshots, error) {
	out := LoadedSnapshots{Places: []Place{}, Missing: []MissingSnapshot{}}
	for _, s := range selections {
		p, ok, err := Snapshot(ctx, db, s, 0)
		if err != nil {
			return out, err
		}
		if !ok {
			resource := "restaurants"
			if s.Kind == Prayer {
				resource = "prayer"
			}
			out.Missing = append(out.Missing, MissingSnapshot{s, "detail_inspection_needed", fmt.Sprintf("run: halal-gourmet-japan-pp-cli %s get %s --data-source live", resource, s.ID)})
			continue
		}
		out.Places = append(out.Places, p)
	}
	return out, nil
}
