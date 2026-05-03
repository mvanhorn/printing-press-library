// Helpers for the Craigslist tables defined in cl_tables.go. Lives in the same
// package so unexported Store fields are reachable.

package store

import (
	"context"
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// CLListing is the typed shape stored in the listings table. Mirrors the source
// package's Listing but with the fields the store cares about.
type CLListing struct {
	PID          int64             `json:"pid"`
	UUID         string            `json:"uuid"`
	Site         string            `json:"site"`
	Subarea      string            `json:"subarea,omitempty"`
	Neighborhood string            `json:"neighborhood,omitempty"`
	CategoryAbbr string            `json:"categoryAbbr,omitempty"`
	CategoryID   int               `json:"categoryId,omitempty"`
	Title        string            `json:"title"`
	Body         string            `json:"body,omitempty"`
	BodyText     string            `json:"bodyText,omitempty"`
	Price        int               `json:"price"`
	PriceDisplay string            `json:"priceDisplay,omitempty"`
	Lat          float64           `json:"lat,omitempty"`
	Lng          float64           `json:"lng,omitempty"`
	Images       []string          `json:"images,omitempty"`
	Attributes   map[string]string `json:"attributes,omitempty"`
	CanonicalURL string            `json:"canonicalUrl,omitempty"`
	Slug         string            `json:"slug,omitempty"`
	PostedAt     int64             `json:"postedAt,omitempty"`
	UpdatedAt    int64             `json:"updatedAt,omitempty"`
}

// UpsertListing writes or updates a single listing row, captures a snapshot of
// (price, title, body_hash) for drift/repost detection, and updates the FTS index.
func (s *Store) UpsertListing(ctx context.Context, l CLListing) error {
	if l.PID == 0 {
		return fmt.Errorf("UpsertListing: empty pid")
	}
	now := time.Now().Unix()
	imgsJSON, _ := json.Marshal(l.Images)
	attrsJSON, _ := json.Marshal(l.Attributes)
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("upsert listing: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO listings (
			pid, uuid, site, subarea, neighborhood, category_abbr, category_id,
			title, body, body_text, price, price_display, lat, lng,
			image_count, images_json, attributes_json, canonical_url, slug,
			posted_at, updated_at, first_seen_at, last_seen_at, status
		) VALUES (?,?,?,?,?,?,?, ?,?,?,?,?,?,?, ?,?,?,?,?, ?,?,?,?, 'active')
		ON CONFLICT(pid) DO UPDATE SET
			uuid          = excluded.uuid,
			subarea       = excluded.subarea,
			neighborhood  = excluded.neighborhood,
			category_abbr = excluded.category_abbr,
			category_id   = excluded.category_id,
			title         = excluded.title,
			body          = excluded.body,
			body_text     = excluded.body_text,
			price         = excluded.price,
			price_display = excluded.price_display,
			lat           = excluded.lat,
			lng           = excluded.lng,
			image_count   = excluded.image_count,
			images_json   = excluded.images_json,
			attributes_json = excluded.attributes_json,
			canonical_url = excluded.canonical_url,
			slug          = excluded.slug,
			updated_at    = excluded.updated_at,
			last_seen_at  = excluded.last_seen_at
	`,
		l.PID, l.UUID, l.Site, l.Subarea, l.Neighborhood, l.CategoryAbbr, l.CategoryID,
		l.Title, l.Body, l.BodyText, l.Price, l.PriceDisplay, l.Lat, l.Lng,
		len(l.Images), string(imgsJSON), string(attrsJSON), l.CanonicalURL, l.Slug,
		l.PostedAt, l.UpdatedAt, now, now,
	)
	if err != nil {
		return fmt.Errorf("upsert listings row: %w", err)
	}

	// Snapshot for drift detection. We snapshot every observation so price/title
	// timelines are preserved.
	bodyHash := sha1Hex(l.BodyText)
	_, err = tx.ExecContext(ctx, `
		INSERT OR IGNORE INTO listing_snapshots(pid, observed_at, price, title, body_hash, status)
		VALUES (?, ?, ?, ?, ?, 'active')
	`, l.PID, now, l.Price, l.Title, bodyHash)
	if err != nil {
		return fmt.Errorf("insert snapshot: %w", err)
	}

	// Image refs: replace-all. Cheap because typical row has < 30 images.
	_, _ = tx.ExecContext(ctx, `DELETE FROM listing_images WHERE pid = ?`, l.PID)
	for i, img := range l.Images {
		_, err = tx.ExecContext(ctx, `INSERT INTO listing_images(pid, idx, image_id) VALUES (?, ?, ?)`, l.PID, i, img)
		if err != nil {
			return fmt.Errorf("insert image: %w", err)
		}
	}

	// FTS index: insert a row per upsert. We don't try to delete the previous
	// FTS row because FTS5's MATCH already handles staleness via the post_id
	// column join. We compose searchable text from title + body + attributes.
	attrsText := strings.Builder{}
	for k, v := range l.Attributes {
		attrsText.WriteString(k)
		attrsText.WriteString(": ")
		attrsText.WriteString(v)
		attrsText.WriteString(" ")
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO listings_fts(rowid, title, body_text, attributes_text)
		VALUES (?, ?, ?, ?)
	`, l.PID, l.Title, l.BodyText, attrsText.String())
	if err != nil {
		// Tolerate duplicate-rowid errors on re-upsert by retrying with delete.
		_, _ = tx.ExecContext(ctx, `DELETE FROM listings_fts WHERE rowid = ?`, l.PID)
		_, err = tx.ExecContext(ctx, `
			INSERT INTO listings_fts(rowid, title, body_text, attributes_text)
			VALUES (?, ?, ?, ?)
		`, l.PID, l.Title, l.BodyText, attrsText.String())
		if err != nil {
			return fmt.Errorf("fts upsert: %w", err)
		}
	}
	return tx.Commit()
}

// GetListing returns a single listing by PID, or sql.ErrNoRows if not found.
func (s *Store) GetListing(ctx context.Context, pid int64) (*CLListing, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT pid, uuid, site, subarea, neighborhood, category_abbr, category_id,
		       title, body, body_text, price, price_display, lat, lng,
		       images_json, attributes_json, canonical_url, slug, posted_at, updated_at
		FROM listings WHERE pid = ?
	`, pid)
	var l CLListing
	var imgsJSON, attrsJSON string
	var subarea, nbh, catAbbr, body, bodyText, priceDisp, canonical, slug, uuid sql.NullString
	var catID sql.NullInt64
	var price sql.NullInt64
	var lat, lng sql.NullFloat64
	var posted, upd sql.NullInt64
	err := row.Scan(&l.PID, &uuid, &l.Site, &subarea, &nbh, &catAbbr, &catID,
		&l.Title, &body, &bodyText, &price, &priceDisp, &lat, &lng,
		&imgsJSON, &attrsJSON, &canonical, &slug, &posted, &upd)
	if err != nil {
		return nil, err
	}
	l.UUID = uuid.String
	l.Subarea = subarea.String
	l.Neighborhood = nbh.String
	l.CategoryAbbr = catAbbr.String
	l.CategoryID = int(catID.Int64)
	l.Body = body.String
	l.BodyText = bodyText.String
	if price.Valid {
		l.Price = int(price.Int64)
	}
	l.PriceDisplay = priceDisp.String
	l.Lat = lat.Float64
	l.Lng = lng.Float64
	l.CanonicalURL = canonical.String
	l.Slug = slug.String
	l.PostedAt = posted.Int64
	l.UpdatedAt = upd.Int64
	if imgsJSON != "" {
		_ = json.Unmarshal([]byte(imgsJSON), &l.Images)
	}
	if attrsJSON != "" {
		_ = json.Unmarshal([]byte(attrsJSON), &l.Attributes)
	}
	return &l, nil
}

// ListingSnapshot is one row from listing_snapshots.
type ListingSnapshot struct {
	PID        int64  `json:"pid"`
	ObservedAt int64  `json:"observedAt"`
	Price      int    `json:"price"`
	Title      string `json:"title"`
	BodyHash   string `json:"bodyHash"`
	Status     string `json:"status"`
}

// GetSnapshots returns price/title history for a listing, oldest first.
func (s *Store) GetSnapshots(ctx context.Context, pid int64) ([]ListingSnapshot, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT pid, observed_at, price, title, body_hash, status
		FROM listing_snapshots
		WHERE pid = ?
		ORDER BY observed_at ASC
	`, pid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ListingSnapshot
	for rows.Next() {
		var sn ListingSnapshot
		var price sql.NullInt64
		if err := rows.Scan(&sn.PID, &sn.ObservedAt, &price, &sn.Title, &sn.BodyHash, &sn.Status); err != nil {
			return out, err
		}
		if price.Valid {
			sn.Price = int(price.Int64)
		}
		out = append(out, sn)
	}
	return out, rows.Err()
}

// SaveArea writes one Area row (replace-on-conflict).
func (s *Store) SaveArea(ctx context.Context, areaID int, abbreviation, hostname, country, region, description, shortDescription string, lat, lng float64, timezone string, parentID int) error {
	now := time.Now().Unix()
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO cl_areas(area_id, abbreviation, hostname, country, region, description, short_description, lat, lng, timezone, parent_area_id, refreshed_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(area_id) DO UPDATE SET
			abbreviation = excluded.abbreviation,
			hostname = excluded.hostname,
			country = excluded.country,
			region = excluded.region,
			description = excluded.description,
			short_description = excluded.short_description,
			lat = excluded.lat,
			lng = excluded.lng,
			timezone = excluded.timezone,
			parent_area_id = excluded.parent_area_id,
			refreshed_at = excluded.refreshed_at
	`, areaID, abbreviation, hostname, country, region, description, shortDescription, lat, lng, timezone, parentID, now)
	return err
}

// SaveCategory writes one Category row.
func (s *Store) SaveCategory(ctx context.Context, categoryID int, abbreviation, description, typ string) error {
	now := time.Now().Unix()
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO cl_categories(category_id, abbreviation, description, type, refreshed_at)
		VALUES (?,?,?,?,?)
		ON CONFLICT(category_id) DO UPDATE SET
			abbreviation = excluded.abbreviation,
			description = excluded.description,
			type = excluded.type,
			refreshed_at = excluded.refreshed_at
	`, categoryID, abbreviation, description, typ, now)
	return err
}

// CountAreas / CountCategories / CountListings — quick metrics for `doctor` and tests.
func (s *Store) CountAreas(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM cl_areas`).Scan(&n)
	return n, err
}
func (s *Store) CountCategories(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM cl_categories`).Scan(&n)
	return n, err
}
func (s *Store) CountListings(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM listings`).Scan(&n)
	return n, err
}

func sha1Hex(s string) string {
	h := sha1.Sum([]byte(s))
	return hex.EncodeToString(h[:])
}
