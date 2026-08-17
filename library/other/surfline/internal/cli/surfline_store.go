// Copyright 2026 Shoffner and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored local-store helpers for the journal and alert novel commands.
// These own the surfline_journal and surfline_alert tables (created lazily by
// store.EnsureSurflineTables) so the commands stay thin.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/other/surfline/internal/store"
)

const surflineDBName = "surfline-pp-cli"

type alertRule struct {
	Name         string  `json:"name"`
	SpotID       string  `json:"spot_id"`
	MinSurf      float64 `json:"min_surf,omitempty"`
	MinPeriod    float64 `json:"min_period,omitempty"`
	MaxWind      float64 `json:"max_wind,omitempty"`
	OffshoreOnly bool    `json:"offshore_only,omitempty"`
	MinRating    float64 `json:"min_rating,omitempty"`
}

type journalSnapshot struct {
	ID         int64           `json:"id"`
	SpotID     string          `json:"spot_id"`
	SpotName   string          `json:"spot_name"`
	CapturedAt int64           `json:"captured_at"`
	RatingKey  string          `json:"rating_key"`
	SurfMin    float64         `json:"surf_min"`
	SurfMax    float64         `json:"surf_max"`
	SwellHt    float64         `json:"swell_ft"`
	SwellPer   float64         `json:"swell_period_s"`
	WindKts    float64         `json:"wind_kts"`
	WindType   string          `json:"wind_type"`
	Snapshot   json.RawMessage `json:"snapshot,omitempty"`
}

// openSurflineStore opens the shared SQLite store and ensures the novel tables
// exist. Callers must Close the returned store.
func openSurflineStore(ctx context.Context, dbPath string) (*store.Store, error) {
	if dbPath == "" {
		dbPath = defaultDBPath(surflineDBName)
	}
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}
	if err := db.EnsureSurflineTables(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ensuring surfline tables: %w", err)
	}
	return db, nil
}

func saveAlertRule(ctx context.Context, db *store.Store, r alertRule, createdAt int64) error {
	_, err := db.DB().ExecContext(ctx, `
		INSERT INTO surfline_alert (name, spot_id, min_surf, min_period, max_wind, offshore_only, min_rating, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET
			spot_id=excluded.spot_id, min_surf=excluded.min_surf, min_period=excluded.min_period,
			max_wind=excluded.max_wind, offshore_only=excluded.offshore_only, min_rating=excluded.min_rating`,
		r.Name, r.SpotID, r.MinSurf, r.MinPeriod, r.MaxWind, boolToInt(r.OffshoreOnly), r.MinRating, createdAt)
	return err
}

func listAlertRules(ctx context.Context, db *store.Store, name string) ([]alertRule, error) {
	q := `SELECT name, spot_id, min_surf, min_period, max_wind, offshore_only, min_rating FROM surfline_alert`
	var rows *sql.Rows
	var err error
	if name != "" {
		rows, err = db.DB().QueryContext(ctx, q+" WHERE name = ?", name)
	} else {
		rows, err = db.DB().QueryContext(ctx, q+" ORDER BY name")
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []alertRule
	for rows.Next() {
		var r alertRule
		var offshore int
		var minSurf, minPeriod, maxWind, minRating sql.NullFloat64
		if err := rows.Scan(&r.Name, &r.SpotID, &minSurf, &minPeriod, &maxWind, &offshore, &minRating); err != nil {
			continue
		}
		r.MinSurf = minSurf.Float64
		r.MinPeriod = minPeriod.Float64
		r.MaxWind = maxWind.Float64
		r.MinRating = minRating.Float64
		r.OffshoreOnly = offshore != 0
		out = append(out, r)
	}
	return out, rows.Err()
}

func saveJournalSnapshot(ctx context.Context, db *store.Store, s journalSnapshot) (int64, error) {
	res, err := db.DB().ExecContext(ctx, `
		INSERT INTO surfline_journal
			(spot_id, spot_name, captured_at, rating_key, rating_value, surf_min, surf_max, swell_height, swell_period, wind_speed, wind_direction_type, snapshot)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.SpotID, s.SpotName, s.CapturedAt, s.RatingKey, 0.0, s.SurfMin, s.SurfMax, s.SwellHt, s.SwellPer, s.WindKts, s.WindType, string(s.Snapshot))
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	return id, nil
}

func listJournalSnapshots(ctx context.Context, db *store.Store, spotID string, limit int) ([]journalSnapshot, error) {
	if limit <= 0 {
		limit = 30
	}
	rows, err := db.DB().QueryContext(ctx, `
		SELECT id, spot_id, spot_name, captured_at, rating_key, surf_min, surf_max, swell_height, swell_period, wind_speed, wind_direction_type
		FROM surfline_journal WHERE spot_id = ? ORDER BY captured_at DESC LIMIT ?`, spotID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []journalSnapshot
	for rows.Next() {
		var s journalSnapshot
		var name, ratingKey, windType sql.NullString
		var surfMin, surfMax, swellHt, swellPer, windKts sql.NullFloat64
		if err := rows.Scan(&s.ID, &s.SpotID, &name, &s.CapturedAt, &ratingKey, &surfMin, &surfMax, &swellHt, &swellPer, &windKts, &windType); err != nil {
			continue
		}
		s.SpotName = name.String
		s.RatingKey = ratingKey.String
		s.WindType = windType.String
		s.SurfMin = surfMin.Float64
		s.SurfMax = surfMax.Float64
		s.SwellHt = swellHt.Float64
		s.SwellPer = swellPer.Float64
		s.WindKts = windKts.Float64
		out = append(out, s)
	}
	return out, rows.Err()
}

type spotMatch struct {
	SpotID     string `json:"spot_id"`
	SpotName   string `json:"spot_name"`
	LastLogged int64  `json:"last_logged"`
	Snapshots  int    `json:"snapshots"`
}

// searchJournaledSpots finds spots the user has captured with `journal log`
// whose name or ID matches the query — the offline name→spotId index.
func searchJournaledSpots(ctx context.Context, db *store.Store, query string, limit int) ([]spotMatch, error) {
	if limit <= 0 {
		limit = 20
	}
	like := "%" + query + "%"
	rows, err := db.DB().QueryContext(ctx, `
		SELECT spot_id, COALESCE(spot_name, '') AS name, MAX(captured_at) AS last_logged, COUNT(*) AS n
		FROM surfline_journal
		WHERE spot_name LIKE ? COLLATE NOCASE OR spot_id LIKE ?
		GROUP BY spot_id
		ORDER BY last_logged DESC
		LIMIT ?`, like, like, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []spotMatch
	for rows.Next() {
		var m spotMatch
		var name sql.NullString
		if err := rows.Scan(&m.SpotID, &name, &m.LastLogged, &m.Snapshots); err != nil {
			continue
		}
		m.SpotName = name.String
		out = append(out, m)
	}
	return out, rows.Err()
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
