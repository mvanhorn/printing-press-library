// Copyright 2026 Micah Baldwin and contributors. Licensed under Apache-2.0. See LICENSE.
package lrcat

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
)

// Bucket is one histogram row.
type Bucket struct {
	Key   string `json:"key"`
	Count int64  `json:"count"`
}

// StatDimensions are the supported --by values for Stats.
var StatDimensions = []string{"camera", "lens", "year", "month", "weekday", "hour", "iso", "focal", "format"}

var weekdayNames = []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}

// Stats returns a histogram of photos bucketed by the given dimension,
// optionally restricted to captures on/after since (YYYY-MM-DD).
func (c *Catalog) Stats(ctx context.Context, by, since string) ([]Bucket, error) {
	var keyExpr, from, order string
	from = "FROM Adobe_images i"
	order = "ORDER BY count(*) DESC"
	switch by {
	case "camera":
		keyExpr = "COALESCE(cm.value,'(unknown)')"
		from += ` LEFT JOIN AgHarvestedExifMetadata em ON em.image = i.id_local
			LEFT JOIN AgInternedExifCameraModel cm ON cm.id_local = em.cameraModelRef`
	case "lens":
		keyExpr = "COALESCE(ln.value,'(unknown)')"
		from += ` LEFT JOIN AgHarvestedExifMetadata em ON em.image = i.id_local
			LEFT JOIN AgInternedExifLens ln ON ln.id_local = em.lensRef`
	case "year":
		keyExpr = "substr(i.captureTime,1,4)"
		order = "ORDER BY 1 DESC"
	case "month":
		keyExpr = "substr(i.captureTime,1,7)"
		order = "ORDER BY 1 DESC"
	case "weekday":
		keyExpr = "strftime('%w', i.captureTime)"
		order = "ORDER BY 1"
	case "hour":
		keyExpr = "substr(i.captureTime,12,2)"
		order = "ORDER BY 1"
	case "iso":
		keyExpr = "CAST(CAST(em.isoSpeedRating AS INTEGER) AS TEXT)"
		from += " JOIN AgHarvestedExifMetadata em ON em.image = i.id_local AND em.isoSpeedRating IS NOT NULL"
		order = "ORDER BY CAST(em.isoSpeedRating AS INTEGER)"
	case "focal":
		keyExpr = "CAST(CAST(round(em.focalLength) AS INTEGER) AS TEXT) || 'mm'"
		from += " JOIN AgHarvestedExifMetadata em ON em.image = i.id_local AND em.focalLength IS NOT NULL"
		order = "ORDER BY CAST(round(em.focalLength) AS INTEGER)"
	case "format":
		keyExpr = "COALESCE(i.fileFormat,'(unknown)')"
	default:
		return nil, fmt.Errorf("--by must be one of: %v", StatDimensions)
	}
	where := "WHERE i.captureTime IS NOT NULL"
	args := []any{}
	if since != "" {
		where += " AND substr(i.captureTime,1,10) >= ?"
		args = append(args, since)
	}
	// #nosec G201 -- keyExpr/from/order are compile-time fragments selected by the
	// closed switch on `by` above (default errors out); user input binds via ? args.
	q := fmt.Sprintf("SELECT %s k, count(*) %s %s GROUP BY k %s", keyExpr, from, where, order)
	rows, err := c.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	out := make([]Bucket, 0)
	for rows.Next() {
		var b Bucket
		var key sql.NullString
		if err := rows.Scan(&key, &b.Count); err != nil {
			_ = rows.Close()
			return nil, err
		}
		b.Key = key.String
		if b.Key == "" {
			b.Key = "(unknown)"
		}
		out = append(out, b)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if by == "weekday" {
		for i := range out {
			if n := out[i].Key; len(n) == 1 && n[0] >= '0' && n[0] <= '6' {
				out[i].Key = weekdayNames[n[0]-'0']
			}
		}
	}
	return out, nil
}

// FunnelStage is one cull stage with count and percentage of total shot.
type FunnelStage struct {
	Stage   string  `json:"stage"`
	Count   int64   `json:"count"`
	Percent float64 `json:"percent"`
}

// FunnelReport is the keeper funnel, optionally per year.
type FunnelReport struct {
	Year   string        `json:"year,omitempty"`
	Stages []FunnelStage `json:"stages"`
}

func funnelStages(shot, picked, rated, developed, collected int64) []FunnelStage {
	pct := func(n int64) float64 {
		if shot == 0 {
			return 0
		}
		return float64(n) / float64(shot) * 100
	}
	return []FunnelStage{
		{Stage: "shot", Count: shot, Percent: 100},
		{Stage: "picked", Count: picked, Percent: pct(picked)},
		{Stage: "rated_3_plus", Count: rated, Percent: pct(rated)},
		{Stage: "developed", Count: developed, Percent: pct(developed)},
		{Stage: "collected", Count: collected, Percent: pct(collected)},
	}
}

const funnelQuery = `
SELECT %s,
  count(*),
  COALESCE(sum(CASE WHEN i.pick = 1 THEN 1 ELSE 0 END), 0),
  COALESCE(sum(CASE WHEN i.rating >= 3 THEN 1 ELSE 0 END), 0),
  COALESCE(sum(CASE WHEN EXISTS (SELECT 1 FROM Adobe_imageDevelopSettings ds
        WHERE ds.image = i.id_local AND ds.hasDevelopAdjustmentsEx > 0) THEN 1 ELSE 0 END), 0),
  COALESCE(sum(CASE WHEN EXISTS (SELECT 1 FROM AgLibraryCollectionImage ci
        JOIN AgLibraryCollection c ON c.id_local = ci.collection
        WHERE ci.image = i.id_local AND c.systemOnly = 0) THEN 1 ELSE 0 END), 0)
FROM Adobe_images i
WHERE i.captureTime IS NOT NULL
%s`

// Funnel computes shot → picked → rated → developed → collected conversion
// counts. byYear=true returns one report per capture year.
func (c *Catalog) Funnel(ctx context.Context, byYear bool) ([]FunnelReport, error) {
	if !byYear {
		q := fmt.Sprintf(funnelQuery, "'all'", "")
		var key string
		var shot, picked, rated, dev, coll int64
		if err := c.DB.QueryRowContext(ctx, q).Scan(&key, &shot, &picked, &rated, &dev, &coll); err != nil {
			return nil, err
		}
		return []FunnelReport{{Stages: funnelStages(shot, picked, rated, dev, coll)}}, nil
	}
	q := fmt.Sprintf(funnelQuery, "substr(i.captureTime,1,4)", "GROUP BY substr(i.captureTime,1,4)")
	rows, err := c.DB.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	out := make([]FunnelReport, 0)
	for rows.Next() {
		var year string
		var shot, picked, rated, dev, coll int64
		if err := rows.Scan(&year, &shot, &picked, &rated, &dev, &coll); err != nil {
			_ = rows.Close()
			return nil, err
		}
		out = append(out, FunnelReport{Year: year, Stages: funnelStages(shot, picked, rated, dev, coll)})
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Year > out[j].Year })
	return out, nil
}

// Backlog returns keepers (flagged picks, or images rated >= minRating when
// minRating > 0) that have no develop adjustments yet.
func (c *Catalog) Backlog(ctx context.Context, minRating float64, pickedOnly bool, limit int) ([]Photo, error) {
	cond := "i.pick = 1"
	args := []any{}
	if !pickedOnly {
		if minRating <= 0 {
			minRating = 3
		}
		cond = "(i.pick = 1 OR i.rating >= ?)"
		args = append(args, minRating)
	}
	if limit <= 0 {
		limit = 100
	}
	args = append(args, limit)
	// #nosec G202 -- cond is one of two compile-time fragments; all values bind via ? args.
	q := photoSelect + fmt.Sprintf(`
		WHERE %s
		  AND NOT EXISTS (SELECT 1 FROM Adobe_imageDevelopSettings ds
			WHERE ds.image = i.id_local AND ds.hasDevelopAdjustmentsEx > 0)
		ORDER BY i.captureTime DESC LIMIT ?`, cond)
	rows, err := c.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	return scanPhotos(rows)
}
