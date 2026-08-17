// Copyright 2026 Micah Baldwin and contributors. Licensed under Apache-2.0. See LICENSE.
package lrcat

import (
	"context"
	"database/sql"
	"fmt"
)

// NamedCount is a catalog entity (collection, keyword) with an image count.
type NamedCount struct {
	Name       string `json:"name"`
	ImageCount int64  `json:"image_count"`
}

// Gear is a camera body or lens with usage counts and first/last-seen dates.
type Gear struct {
	Name       string `json:"name"`
	ImageCount int64  `json:"image_count"`
	FirstSeen  string `json:"first_seen,omitempty"`
	LastSeen   string `json:"last_seen,omitempty"`
}

func (c *Catalog) namedCounts(ctx context.Context, q string, args ...any) ([]NamedCount, error) {
	rows, err := c.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	out := make([]NamedCount, 0)
	for rows.Next() {
		var n NamedCount
		var name sql.NullString
		if err := rows.Scan(&name, &n.ImageCount); err != nil {
			_ = rows.Close()
			return nil, err
		}
		n.Name = name.String
		if n.Name == "" {
			continue
		}
		out = append(out, n)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	return out, rows.Close()
}

// Collections lists user collections with image counts (system collections excluded).
func (c *Catalog) Collections(ctx context.Context) ([]NamedCount, error) {
	return c.namedCounts(ctx, `
		SELECT c.name, count(ci.image)
		FROM AgLibraryCollection c
		LEFT JOIN AgLibraryCollectionImage ci ON ci.collection = c.id_local
		WHERE c.systemOnly = 0
		GROUP BY c.id_local
		ORDER BY count(ci.image) DESC, c.name`)
}

// Keywords lists keywords with image counts, including zero-use keywords.
func (c *Catalog) Keywords(ctx context.Context) ([]NamedCount, error) {
	return c.namedCounts(ctx, `
		SELECT k.name, count(ki.image)
		FROM AgLibraryKeyword k
		LEFT JOIN AgLibraryKeywordImage ki ON ki.tag = k.id_local
		WHERE k.name IS NOT NULL
		GROUP BY k.id_local
		ORDER BY count(ki.image) DESC, k.name`)
}

func (c *Catalog) gear(ctx context.Context, refCol, table string) ([]Gear, error) {
	// #nosec G201 -- refCol/table are package-internal string literals from the two
	// fixed call sites (Cameras, Lenses); no user input reaches this query text.
	q := fmt.Sprintf(`
		SELECT g.value, count(i.id_local),
		       COALESCE(min(substr(i.captureTime,1,10)),''),
		       COALESCE(max(substr(i.captureTime,1,10)),'')
		FROM %s g
		JOIN AgHarvestedExifMetadata em ON em.%s = g.id_local
		JOIN Adobe_images i ON i.id_local = em.image
		GROUP BY g.id_local
		ORDER BY count(i.id_local) DESC, g.value`, table, refCol)
	rows, err := c.DB.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	out := make([]Gear, 0)
	for rows.Next() {
		var g Gear
		var name sql.NullString
		if err := rows.Scan(&name, &g.ImageCount, &g.FirstSeen, &g.LastSeen); err != nil {
			_ = rows.Close()
			return nil, err
		}
		g.Name = name.String
		if g.Name == "" {
			continue
		}
		out = append(out, g)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	return out, rows.Close()
}

// Cameras lists camera bodies with counts and first/last-seen capture dates.
func (c *Catalog) Cameras(ctx context.Context) ([]Gear, error) {
	return c.gear(ctx, "cameraModelRef", "AgInternedExifCameraModel")
}

// Lenses lists lenses with counts and first/last-seen capture dates.
func (c *Catalog) Lenses(ctx context.Context) ([]Gear, error) {
	return c.gear(ctx, "lensRef", "AgInternedExifLens")
}
