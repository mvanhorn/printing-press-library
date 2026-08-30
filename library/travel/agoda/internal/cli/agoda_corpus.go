// Copyright 2026 Victor Wibisono and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/agoda/internal/agoda"
	"github.com/mvanhorn/printing-press-library/library/travel/agoda/internal/store"
)

// rememberProperties records what a live search saw into the local corpus.
//
// Agoda exposes no bulk catalog endpoint, so there is nothing to "sync". The
// corpus therefore accumulates as a side effect of ordinary searching, which is
// what makes offline `search` possible at all.
//
// Failures here are deliberately non-fatal: a user running a live search should
// never lose their results because the local cache could not be written.
func rememberProperties(ctx context.Context, w io.Writer, dbPath string, cityID int, props []agoda.Property) {
	if dbPath == "" || len(props) == 0 {
		return
	}
	st, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return
	}
	defer st.Close()
	if err := store.EnsureAgodaSchema(ctx, st.DB()); err != nil {
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	tx, err := st.DB().BeginTx(ctx, nil)
	if err != nil {
		return
	}
	stmt, err := tx.PrepareContext(ctx, `
        INSERT INTO properties
            (property_id, name, city_id, address, star_rating, review_score, review_count, latitude, longitude, last_seen_at)
        VALUES (?,?,?,?,?,?,?,?,?,?)
        ON CONFLICT(property_id) DO UPDATE SET
            name=excluded.name, city_id=excluded.city_id, address=excluded.address,
            star_rating=excluded.star_rating, review_score=excluded.review_score,
            review_count=excluded.review_count, latitude=excluded.latitude,
            longitude=excluded.longitude, last_seen_at=excluded.last_seen_at`)
	if err != nil {
		_ = tx.Rollback()
		return
	}
	defer stmt.Close()
	for _, p := range props {
		if p.PropertyID == 0 || p.Name == "" {
			continue
		}
		if _, err := stmt.ExecContext(ctx, p.PropertyID, p.Name, cityID, p.Address,
			p.StarRating, p.ReviewScore, p.ReviewCount, p.Latitude, p.Longitude, now); err != nil {
			_ = tx.Rollback()
			return
		}
	}
	// Reported rather than discarded: a failed commit means the corpus silently
	// stopped growing, which would later look like "offline search found
	// nothing" with no explanation. It still must not fail the user's search.
	if err := tx.Commit(); err != nil && w != nil {
		fmt.Fprintf(w, "warning: could not update the local property corpus: %v\n", err)
	}
}

// corpusRow is one locally-cached property returned by offline search.
type corpusRow struct {
	PropertyID  int     `json:"property_id"`
	Name        string  `json:"name"`
	CityID      int     `json:"city_id"`
	Address     string  `json:"address,omitempty"`
	StarRating  float64 `json:"star_rating,omitempty"`
	ReviewScore float64 `json:"review_score,omitempty"`
	ReviewCount int     `json:"review_count,omitempty"`
	LastSeenAt  string  `json:"last_seen_at,omitempty"`
}

// queryCorpus runs the offline FTS5 lookup.
//
// Every optional column is scanned through a sql.Null* target: SQLite returns
// NULL for absent values, and scanning those into bare strings would error and
// silently drop the row inside the iteration loop.
func queryCorpus(ctx context.Context, db *sql.DB, query string, limit int) ([]corpusRow, error) {
	rows, err := db.QueryContext(ctx, `
        SELECT p.property_id, p.name, p.city_id, p.address, p.star_rating,
               p.review_score, p.review_count, p.last_seen_at
        FROM properties_fts f
        JOIN properties p ON p.property_id = f.rowid
        WHERE properties_fts MATCH ?
        ORDER BY bm25(properties_fts), p.review_count DESC
        LIMIT ?`, query, limit)
	if err != nil {
		return nil, err
	}
	out := make([]corpusRow, 0, limit)
	for rows.Next() {
		var r corpusRow
		var name, addr, seen sql.NullString
		var star, score sql.NullFloat64
		var count, city sql.NullInt64
		if err := rows.Scan(&r.PropertyID, &name, &city, &addr, &star, &score, &count, &seen); err != nil {
			_ = rows.Close()
			return nil, err
		}
		r.Name, r.Address, r.LastSeenAt = name.String, addr.String, seen.String
		r.StarRating, r.ReviewScore = star.Float64, score.Float64
		r.ReviewCount, r.CityID = int(count.Int64), int(city.Int64)
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	return out, nil
}
