package pricebook

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/servicetitan-pricebook/internal/store"
)

// sku_cost_history is the append-only change log that makes cost-drift a
// one-shot query. The generated ServiceTitan API exposes no cost history;
// this table is the entire reason cost-drift and the "did price follow
// cost?" check exist. It lives in the same SQLite file as the synced
// resources but is owned entirely by this package — the generated store.go
// never touches it.
const createCostHistory = `
CREATE TABLE IF NOT EXISTS sku_cost_history (
	snapshot_at  TEXT    NOT NULL,
	sku_kind     TEXT    NOT NULL,
	sku_id       INTEGER NOT NULL,
	code         TEXT,
	display_name TEXT,
	cost         REAL,
	price        REAL,
	vendor_part  TEXT,
	modified_on  TEXT,
	PRIMARY KEY (snapshot_at, sku_kind, sku_id)
);`

// EnsureCostHistory creates the sku_cost_history table if it does not exist.
// Safe to call on every command invocation.
func EnsureCostHistory(db *store.Store) error {
	if _, err := db.DB().Exec(createCostHistory); err != nil {
		return fmt.Errorf("creating sku_cost_history: %w", err)
	}
	return nil
}

// histKey identifies one SKU across snapshots.
type histKey struct {
	kind SKUKind
	id   int64
}

type histRow struct {
	cost       float64
	price      float64
	vendorPart string
}

// Snapshot records the current cost/price/vendor-part of every synced
// material and equipment SKU into sku_cost_history — but only for SKUs that
// changed since their most recent prior snapshot, so the table stays a true
// change log instead of growing by the full pricebook on every call. It
// returns the number of change rows written and the number of SKUs
// considered. Callers that need drift (cost-drift, health) call this first
// so the latest state is always captured before the diff runs.
func Snapshot(db *store.Store) (written, considered int, err error) {
	if err = EnsureCostHistory(db); err != nil {
		return 0, 0, err
	}
	prior, err := latestSnapshotByKey(db)
	if err != nil {
		return 0, 0, err
	}
	mats, err := LoadMaterials(db)
	if err != nil {
		return 0, 0, err
	}
	eqs, err := LoadEquipment(db)
	if err != nil {
		return 0, 0, err
	}

	// PATCH: snapshot-nanos-precision (Greptile PR #576). The composite PK on
	// (snapshot_at, sku_kind, sku_id) plus INSERT OR IGNORE silently drops
	// any row whose timestamp collides with one already in the table. Second-
	// precision RFC3339 made same-second concurrent Snapshot() calls (agent-
	// loop --tail, parallel health checks) lose every row that came after
	// the first. RFC3339Nano keeps the same TEXT column shape but makes
	// real collisions astronomically unlikely.
	now := time.Now().UTC().Format(time.RFC3339Nano)
	tx, err := db.DB().Begin()
	if err != nil {
		return 0, 0, fmt.Errorf("begin snapshot tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	stmt, err := tx.Prepare(`INSERT OR IGNORE INTO sku_cost_history
		(snapshot_at, sku_kind, sku_id, code, display_name, cost, price, vendor_part, modified_on)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, 0, fmt.Errorf("prepare snapshot insert: %w", err)
	}
	defer stmt.Close()

	insertIfChanged := func(kind SKUKind, id int64, code, name string, cost, price float64, vp, modOn string) error {
		considered++
		k := histKey{kind, id}
		if p, ok := prior[k]; ok && p.cost == cost && p.price == price && p.vendorPart == vp {
			return nil // unchanged — don't bloat the log
		}
		if _, err := stmt.Exec(now, string(kind), id, code, name, cost, price, vp, modOn); err != nil {
			return fmt.Errorf("insert snapshot row: %w", err)
		}
		written++
		return nil
	}

	for _, m := range mats {
		vp := ""
		if m.PrimaryVendor != nil {
			vp = m.PrimaryVendor.VendorPart
		}
		if err := insertIfChanged(KindMaterial, m.ID, m.Code, m.DisplayName, m.Cost, m.Price, vp, m.ModifiedOn); err != nil {
			return 0, 0, err
		}
	}
	for _, e := range eqs {
		vp := ""
		if e.PrimaryVendor != nil {
			vp = e.PrimaryVendor.VendorPart
		}
		if err := insertIfChanged(KindEquipment, e.ID, e.Code, e.DisplayName, e.Cost, e.Price, vp, e.ModifiedOn); err != nil {
			return 0, 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, 0, fmt.Errorf("commit snapshot: %w", err)
	}
	return written, considered, nil
}

// latestSnapshotByKey returns the most recent snapshot row per SKU.
func latestSnapshotByKey(db *store.Store) (map[histKey]histRow, error) {
	rows, err := db.DB().Query(`
		SELECT h.sku_kind, h.sku_id, h.cost, h.price, COALESCE(h.vendor_part,'')
		FROM sku_cost_history h
		JOIN (
			SELECT sku_kind, sku_id, MAX(snapshot_at) AS mx
			FROM sku_cost_history GROUP BY sku_kind, sku_id
		) latest
		ON h.sku_kind = latest.sku_kind AND h.sku_id = latest.sku_id AND h.snapshot_at = latest.mx`)
	if err != nil {
		return nil, fmt.Errorf("query latest snapshots: %w", err)
	}
	defer rows.Close()
	out := make(map[histKey]histRow)
	for rows.Next() {
		var kind string
		var id int64
		var r histRow
		if err := rows.Scan(&kind, &id, &r.cost, &r.price, &r.vendorPart); err != nil {
			return nil, fmt.Errorf("scan snapshot: %w", err)
		}
		out[histKey{SKUKind(kind), id}] = r
	}
	return out, rows.Err()
}

// DriftRow is one cost-drift finding: a SKU whose cost moved between the
// baseline snapshot and the latest snapshot, plus whether the price moved
// with it.
type DriftRow struct {
	Kind          SKUKind `json:"kind"`
	ID            int64   `json:"id"`
	Code          string  `json:"code"`
	DisplayName   string  `json:"display_name"`
	OldCost       float64 `json:"old_cost"`
	NewCost       float64 `json:"new_cost"`
	OldPrice      float64 `json:"old_price"`
	NewPrice      float64 `json:"new_price"`
	CostDelta     float64 `json:"cost_delta"`
	PriceDelta    float64 `json:"price_delta"`
	PriceFollowed bool    `json:"price_followed"`
	BaselineAt    string  `json:"baseline_at"`
	LatestAt      string  `json:"latest_at"`
}

// CostDrift returns every SKU whose cost changed between a baseline snapshot
// and the latest snapshot. The baseline for each SKU is the latest snapshot
// at or before `since` (RFC3339); if the SKU has no snapshot that old, its
// earliest snapshot is used. PriceFollowed reports whether the price moved
// in the same direction as the cost — the margin-discipline question the ST
// UI cannot answer. Call Snapshot first so the latest state is captured.
func CostDrift(db *store.Store, since string) ([]DriftRow, error) {
	if err := EnsureCostHistory(db); err != nil {
		return nil, err
	}
	rows, err := db.DB().Query(`
		SELECT snapshot_at, sku_kind, sku_id, code, display_name, cost, price
		FROM sku_cost_history
		ORDER BY sku_kind, sku_id, snapshot_at`)
	if err != nil {
		return nil, fmt.Errorf("query cost history: %w", err)
	}
	defer rows.Close()

	type snap struct {
		at          string
		code, name  string
		cost, price float64
	}
	series := make(map[histKey][]snap)
	for rows.Next() {
		var kind string
		var id int64
		var s snap
		if err := rows.Scan(&s.at, &kind, &id, &s.code, &s.name, &s.cost, &s.price); err != nil {
			return nil, fmt.Errorf("scan cost history: %w", err)
		}
		k := histKey{SKUKind(kind), id}
		series[k] = append(series[k], s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var out []DriftRow
	for k, snaps := range series {
		if len(snaps) < 2 {
			continue // need at least two snapshots to see drift
		}
		latest := snaps[len(snaps)-1]
		// PATCH: cost-drift-since-yyyymmdd (Greptile PR #576). s.at is RFC3339
		// ("2026-04-01T08:00:00Z"); --since is documented as YYYY-MM-DD. A
		// bare date sorts BEFORE any same-day RFC3339 string, so a
		// lexicographic s.at <= since would silently drop same-day snapshots
		// from the baseline window. Promote a bare date to end-of-day.
		sinceBound := since
		if len(sinceBound) == 10 {
			sinceBound += "T23:59:59Z"
		}
		baseline := snaps[0]
		for _, s := range snaps {
			if sinceBound != "" && s.at <= sinceBound {
				baseline = s // walk forward to the newest snapshot <= sinceBound
			}
		}
		if baseline.at == latest.at || baseline.cost == latest.cost {
			continue // cost did not move
		}
		costDelta := round2(latest.cost - baseline.cost)
		priceDelta := round2(latest.price - baseline.price)
		followed := priceDelta != 0 && sameSign(costDelta, priceDelta)
		out = append(out, DriftRow{
			Kind: k.kind, ID: k.id, Code: latest.code, DisplayName: latest.name,
			OldCost: baseline.cost, NewCost: latest.cost,
			OldPrice: baseline.price, NewPrice: latest.price,
			CostDelta: costDelta, PriceDelta: priceDelta, PriceFollowed: followed,
			BaselineAt: baseline.at, LatestAt: latest.at,
		})
	}
	SortDriftRows(out)
	return out, nil
}

func sameSign(a, b float64) bool {
	return (a > 0 && b > 0) || (a < 0 && b < 0)
}

// CostHistoryRows returns the number of rows in sku_cost_history — used by
// health and doctor-style status output.
func CostHistoryRows(db *store.Store) (int, error) {
	if err := EnsureCostHistory(db); err != nil {
		return 0, err
	}
	var n int
	err := db.DB().QueryRow(`SELECT COUNT(*) FROM sku_cost_history`).Scan(&n)
	if err != nil && err != sql.ErrNoRows {
		return 0, fmt.Errorf("counting cost history: %w", err)
	}
	return n, nil
}
