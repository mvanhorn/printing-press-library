// Copyright 2026 Micah Baldwin and contributors. Licensed under Apache-2.0. See LICENSE.
package lrcat

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
)

// Photo is one catalog image with resolved metadata and disk path.
type Photo struct {
	ID          int64    `json:"id"`
	CaptureTime string   `json:"capture_time,omitempty"`
	Rating      *float64 `json:"rating,omitempty"`
	Pick        int      `json:"pick"`
	ColorLabel  string   `json:"color_label,omitempty"`
	FileFormat  string   `json:"file_format,omitempty"`
	FileName    string   `json:"file_name,omitempty"`
	Camera      string   `json:"camera,omitempty"`
	Lens        string   `json:"lens,omitempty"`
	ISO         *float64 `json:"iso,omitempty"`
	Aperture    string   `json:"aperture,omitempty"`
	Shutter     string   `json:"shutter,omitempty"`
	FocalLength *float64 `json:"focal_length,omitempty"`
	Path        string   `json:"path"`
}

// FindFilters are the criteria for FindPhotos. Zero values mean "no filter".
type FindFilters struct {
	Since      string // YYYY-MM-DD inclusive
	Until      string // YYYY-MM-DD inclusive
	Date       string // exact day
	Rating     string // e.g. ">=4", "5", ">0", "unrated"
	Picked     bool
	Rejected   bool
	Label      string
	Keyword    string
	Collection string
	Camera     string
	Lens       string
	ISO        string // e.g. ">=1600"
	Format     string // RAW, JPG, ...
	Limit      int
}

const photoSelect = `
SELECT i.id_local, COALESCE(i.captureTime,''), i.rating, CAST(i.pick AS INTEGER),
       COALESCE(i.colorLabels,''), COALESCE(i.fileFormat,''), COALESCE(fl.idx_filename,''),
       COALESCE(cm.value,''), COALESCE(ln.value,''),
       em.isoSpeedRating, em.aperture, em.shutterSpeed, em.focalLength,
       COALESCE(rf.absolutePath,'') || COALESCE(fo.pathFromRoot,'') || COALESCE(fl.idx_filename,'')
FROM Adobe_images i
JOIN AgLibraryFile fl ON fl.id_local = i.rootFile
JOIN AgLibraryFolder fo ON fo.id_local = fl.folder
JOIN AgLibraryRootFolder rf ON rf.id_local = fo.rootFolder
LEFT JOIN AgHarvestedExifMetadata em ON em.image = i.id_local
LEFT JOIN AgInternedExifCameraModel cm ON cm.id_local = em.cameraModelRef
LEFT JOIN AgInternedExifLens ln ON ln.id_local = em.lensRef
`

// opClause parses a compact comparison filter like ">=4", "<800", "5" into a
// SQL clause on col. Returns error on junk input so bad flags fail loudly.
func opClause(col, expr string) (string, []any, error) {
	e := strings.TrimSpace(expr)
	op := "="
	for _, cand := range []string{">=", "<=", "!=", ">", "<", "="} {
		if strings.HasPrefix(e, cand) {
			op = cand
			e = strings.TrimSpace(e[len(cand):])
			break
		}
	}
	v, err := strconv.ParseFloat(e, 64)
	if err != nil {
		return "", nil, fmt.Errorf("invalid comparison %q: expected forms like '>=4', '5', '>0'", expr)
	}
	return fmt.Sprintf("%s %s ?", col, op), []any{v}, nil
}

func buildFindQuery(f FindFilters) (string, []any, error) {
	var where []string
	var args []any
	add := func(clause string, a ...any) {
		where = append(where, clause)
		args = append(args, a...)
	}
	if f.Date != "" {
		add("substr(i.captureTime,1,10) = ?", f.Date)
	}
	if f.Since != "" {
		add("substr(i.captureTime,1,10) >= ?", f.Since)
	}
	if f.Until != "" {
		add("substr(i.captureTime,1,10) <= ?", f.Until)
	}
	if f.Rating != "" {
		if strings.EqualFold(f.Rating, "unrated") {
			add("i.rating IS NULL")
		} else {
			clause, a, err := opClause("i.rating", f.Rating)
			if err != nil {
				return "", nil, fmt.Errorf("--rating: %w", err)
			}
			add(clause, a...)
		}
	}
	if f.Picked {
		add("i.pick = 1")
	}
	if f.Rejected {
		add("i.pick = -1")
	}
	if f.Label != "" {
		add("lower(i.colorLabels) = lower(?)", f.Label)
	}
	if f.Keyword != "" {
		add(`EXISTS (SELECT 1 FROM AgLibraryKeywordImage ki
			JOIN AgLibraryKeyword k ON k.id_local = ki.tag
			WHERE ki.image = i.id_local AND lower(k.name) = lower(?))`, f.Keyword)
	}
	if f.Collection != "" {
		add(`EXISTS (SELECT 1 FROM AgLibraryCollectionImage ci
			JOIN AgLibraryCollection c ON c.id_local = ci.collection
			WHERE ci.image = i.id_local AND lower(c.name) LIKE lower(?))`, "%"+f.Collection+"%")
	}
	if f.Camera != "" {
		add("cm.value LIKE ?", "%"+f.Camera+"%")
	}
	if f.Lens != "" {
		add("ln.value LIKE ?", "%"+f.Lens+"%")
	}
	if f.ISO != "" {
		clause, a, err := opClause("em.isoSpeedRating", f.ISO)
		if err != nil {
			return "", nil, fmt.Errorf("--iso: %w", err)
		}
		add(clause, a...)
	}
	if f.Format != "" {
		add("lower(i.fileFormat) = lower(?)", f.Format)
	}
	q := photoSelect
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	q += " ORDER BY i.captureTime DESC, i.id_local DESC"
	limit := f.Limit
	if limit <= 0 {
		limit = 100
	}
	q += fmt.Sprintf(" LIMIT %d", limit)
	return q, args, nil
}

func scanPhotos(rows *sql.Rows) ([]Photo, error) {
	out := make([]Photo, 0)
	for rows.Next() {
		var p Photo
		var rating, iso, aperture, shutter, focal sql.NullFloat64
		if err := rows.Scan(&p.ID, &p.CaptureTime, &rating, &p.Pick, &p.ColorLabel,
			&p.FileFormat, &p.FileName, &p.Camera, &p.Lens,
			&iso, &aperture, &shutter, &focal, &p.Path); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scan photo: %w", err)
		}
		if rating.Valid {
			v := rating.Float64
			p.Rating = &v
		}
		if iso.Valid {
			v := iso.Float64
			p.ISO = &v
		}
		if aperture.Valid {
			p.Aperture = ApertureFromAPEX(aperture.Float64)
		}
		if shutter.Valid {
			p.Shutter = ShutterFromAPEX(shutter.Float64)
		}
		if focal.Valid {
			v := focal.Float64
			p.FocalLength = &v
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	return out, rows.Close()
}

// FindPhotos runs a criteria search over the catalog.
func (c *Catalog) FindPhotos(ctx context.Context, f FindFilters) ([]Photo, error) {
	q, args, err := buildFindQuery(f)
	if err != nil {
		return nil, err
	}
	rows, err := c.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("querying photos: %w", err)
	}
	return scanPhotos(rows)
}

// PhotoByIDOrName resolves a photo by numeric id_local or filename.
func (c *Catalog) PhotoByIDOrName(ctx context.Context, ref string) ([]Photo, error) {
	var q string
	var args []any
	if id, err := strconv.ParseInt(ref, 10, 64); err == nil {
		q = photoSelect + " WHERE i.id_local = ?"
		args = []any{id}
	} else {
		q = photoSelect + " WHERE lower(fl.idx_filename) = lower(?) ORDER BY i.captureTime DESC LIMIT 50"
		args = []any{ref}
	}
	rows, err := c.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("querying photo: %w", err)
	}
	return scanPhotos(rows)
}
