// Copyright 2026 rderwin. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/redfin/internal/client"
	"github.com/mvanhorn/printing-press-library/library/other/redfin/internal/redfin"
	"github.com/mvanhorn/printing-press-library/library/other/redfin/internal/store"
)

// parseRegionSlug accepts a Redfin region slug ("city/30772/TX/Austin", or
// "/city/30772/TX/Austin"), a "regionID:regionType" pair ("30772:6"), or a
// bare numeric region ID (defaults to city/6) and returns the region_id +
// region_type pair.
//
// Region type codes (from Redfin's Stingray): 6=city, 1=zip, 2=state,
// 4=metro, 11=neighborhood. Slug prefix "city" → 6, "zipcode" → 1, etc.
func parseRegionSlug(s string) (int64, int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, 0, fmt.Errorf("empty region")
	}
	// "30772:6" pair
	if idx := strings.Index(s, ":"); idx > 0 {
		idStr, typStr := s[:idx], s[idx+1:]
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid region id %q: %w", idStr, err)
		}
		typ, err := strconv.Atoi(typStr)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid region type %q: %w", typStr, err)
		}
		return id, typ, nil
	}
	// Bare numeric → assume city
	if id, err := strconv.ParseInt(s, 10, 64); err == nil {
		return id, 6, nil
	}
	// Slug path: city/30772/TX/Austin or /city/30772/TX/Austin
	parts := strings.Split(strings.Trim(s, "/"), "/")
	if len(parts) < 2 {
		return 0, 0, fmt.Errorf("unrecognized region slug %q", s)
	}
	typ := 0
	switch strings.ToLower(parts[0]) {
	case "city":
		typ = 6
	case "zipcode", "zip":
		typ = 1
	case "neighborhood":
		typ = 11
	case "state":
		typ = 2
	case "metro":
		typ = 4
	case "county":
		typ = 5
	default:
		return 0, 0, fmt.Errorf("unknown region kind %q (expected city|zipcode|neighborhood|state|metro|county)", parts[0])
	}
	id, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid region id %q: %w", parts[1], err)
	}
	return id, typ, nil
}

// openRedfinStore opens the canonical SQLite store and ensures the redfin
// extension schema exists. Caller must Close.
func openRedfinStore(ctx context.Context) (*store.Store, error) {
	dbPath := defaultDBPath("redfin-pp-cli")
	s, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, err
	}
	if err := redfin.EnsureExtSchema(s.DB()); err != nil {
		s.Close()
		return nil, err
	}
	return s, nil
}

// parseDurationLoose accepts everything time.ParseDuration accepts, plus the
// extensions "Nd" (days) and "Nw" (weeks) for human-friendly window flags.
func parseDurationLoose(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty duration")
	}
	if strings.HasSuffix(s, "d") {
		n, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err != nil {
			return 0, err
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	if strings.HasSuffix(s, "w") {
		n, err := strconv.Atoi(strings.TrimSuffix(s, "w"))
		if err != nil {
			return 0, err
		}
		return time.Duration(n) * 7 * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}

// lookupListingByURL reads the canonical homes table for a cached listing
// keyed by URL. Returns (nil, nil) when not cached. Falls back to a live
// listing fetch when c is non-nil and the cache misses.
func lookupListingByURL(ctx context.Context, db *sql.DB, c *client.Client, url string) (*redfin.Listing, error) {
	row := db.QueryRowContext(ctx, `SELECT data FROM homes WHERE id = ?`, url)
	var data string
	err := row.Scan(&data)
	if err == nil {
		var l redfin.Listing
		if uerr := json.Unmarshal([]byte(data), &l); uerr == nil && l.URL != "" {
			return &l, nil
		}
	}
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	if c == nil {
		return nil, nil
	}
	// Live fallback: walk the three Stingray endpoints and merge.
	listing, lerr := fetchListingDetailLive(c, url)
	if lerr != nil {
		return nil, lerr
	}
	return &listing, nil
}

// fetchListingDetailLive runs the three home/details/* calls and merges into
// a Listing. Each call is best-effort; partial responses still produce a
// usable record.
func fetchListingDetailLive(c *client.Client, urlPath string) (redfin.Listing, error) {
	initial, err1 := c.Get("/stingray/api/home/details/initialInfo", map[string]string{"path": urlPath})
	if err1 != nil {
		return redfin.Listing{}, err1
	}
	// Pull propertyId / listingId from initialInfo so we can request the rest.
	var env struct {
		Payload struct {
			PropertyID int64 `json:"propertyId"`
			ListingID  int64 `json:"listingId"`
		} `json:"payload"`
	}
	_ = json.Unmarshal(redfin.StripStingrayPrefix(initial), &env)
	pid := strconv.FormatInt(env.Payload.PropertyID, 10)
	lid := strconv.FormatInt(env.Payload.ListingID, 10)
	above, _ := c.Get("/stingray/api/home/details/aboveTheFold", map[string]string{
		"propertyId": pid, "listingId": lid, "accessLevel": "1",
	})
	below, _ := c.Get("/stingray/api/home/details/belowTheFold", map[string]string{
		"propertyId": pid, "listingId": lid, "accessLevel": "1",
	})
	listing, perr := redfin.ParseListingDetail(initial, above, below)
	if perr != nil {
		return redfin.Listing{}, perr
	}
	if listing.URL == "" {
		listing.URL = urlPath
	}
	return listing, nil
}

// listingsFromHomesTable returns every cached redfin.Listing in the homes
// table that matches the given region (use 0/0 to match all).
func listingsFromHomesTable(ctx context.Context, db *sql.DB, regionID int64, regionType int) ([]redfin.Listing, error) {
	q := `SELECT data FROM homes`
	args := []any{}
	if regionID != 0 {
		q += ` WHERE region_id = ?`
		args = append(args, regionID)
		if regionType != 0 {
			q += ` AND region_type = ?`
			args = append(args, regionType)
		}
	}
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []redfin.Listing
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		var l redfin.Listing
		if uerr := json.Unmarshal([]byte(data), &l); uerr == nil {
			out = append(out, l)
		}
	}
	return out, rows.Err()
}

// upsertListingHome canonicalizes a Listing into the homes table by URL.
// Used by sync-search and listing detail to keep the homes table populated.
func upsertListingHome(s *store.Store, l redfin.Listing) error {
	if l.URL == "" {
		return nil
	}
	b, err := json.Marshal(l)
	if err != nil {
		return err
	}
	return s.Upsert("homes", l.URL, b)
}
