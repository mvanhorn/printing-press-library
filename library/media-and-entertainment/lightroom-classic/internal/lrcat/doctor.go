// Copyright 2026 Micah Baldwin and contributors. Licensed under Apache-2.0. See LICENSE.
package lrcat

import (
	"context"
	"os"
)

// HealthReport is the catalog hygiene sweep. Strictly read-only: paths are
// stat'ed but never opened for writing.
type HealthReport struct {
	CatalogPath      string   `json:"catalog_path"`
	TotalPhotos      int64    `json:"total_photos"`
	MissingMasters   []string `json:"missing_masters"`
	MissingCount     int      `json:"missing_count"`
	NoCaptureTime    int64    `json:"no_capture_time"`
	OrphanKeywords   []string `json:"orphan_keywords"`
	EmptyCollections []string `json:"empty_collections"`
	ScannedFiles     int      `json:"scanned_files"`
}

// Health runs the catalog hygiene sweep. maxMissing bounds how many missing
// paths are listed (all are counted).
func (c *Catalog) Health(ctx context.Context, maxMissing int) (*HealthReport, error) {
	rep := &HealthReport{
		CatalogPath:      c.Path,
		MissingMasters:   []string{},
		OrphanKeywords:   []string{},
		EmptyCollections: []string{},
	}
	if maxMissing <= 0 {
		maxMissing = 50
	}
	if err := c.DB.QueryRowContext(ctx, "SELECT count(*) FROM Adobe_images").Scan(&rep.TotalPhotos); err != nil {
		return nil, err
	}
	if err := c.DB.QueryRowContext(ctx,
		"SELECT count(*) FROM Adobe_images WHERE captureTime IS NULL OR captureTime = ''").Scan(&rep.NoCaptureTime); err != nil {
		return nil, err
	}

	// Missing masters: resolve every path, stat on disk. Drain rows fully
	// before stat-ing to keep the single SQLite connection free.
	rows, err := c.DB.QueryContext(ctx, `
		SELECT COALESCE(rf.absolutePath,'') || COALESCE(fo.pathFromRoot,'') || COALESCE(fl.idx_filename,'')
		FROM AgLibraryFile fl
		JOIN AgLibraryFolder fo ON fo.id_local = fl.folder
		JOIN AgLibraryRootFolder rf ON rf.id_local = fo.rootFolder`)
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, rep.TotalPhotos)
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			_ = rows.Close()
			return nil, err
		}
		if p != "" {
			paths = append(paths, p)
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for _, p := range paths {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		rep.ScannedFiles++
		if _, err := os.Stat(p); os.IsNotExist(err) {
			rep.MissingCount++
			if len(rep.MissingMasters) < maxMissing {
				rep.MissingMasters = append(rep.MissingMasters, p)
			}
		}
	}

	// Orphan keywords: named keywords applied to zero images.
	kws, err := c.Keywords(ctx)
	if err != nil {
		return nil, err
	}
	for _, k := range kws {
		if k.ImageCount == 0 {
			rep.OrphanKeywords = append(rep.OrphanKeywords, k.Name)
		}
	}

	// Empty collections.
	cols, err := c.Collections(ctx)
	if err != nil {
		return nil, err
	}
	for _, col := range cols {
		if col.ImageCount == 0 {
			rep.EmptyCollections = append(rep.EmptyCollections, col.Name)
		}
	}
	return rep, nil
}
