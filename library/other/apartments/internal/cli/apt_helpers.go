// Copyright 2026 rderwin. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/apartments/internal/apt"
	"github.com/mvanhorn/printing-press-library/library/other/apartments/internal/store"
)

// cityStateFromListingURL extracts the trailing (city, state) tokens
// from an apartments.com listing URL. Listing URLs are shaped like:
//
//	https://www.apartments.com/{property-name}-{city}-{state}/{id}/
//
// where state is a two-letter US abbreviation. Returns ("","") when
// the trailing two-letter slug doesn't look like a state. Multi-word
// cities (san-francisco, new-york) require a dictionary to fully
// disambiguate; this helper returns the LAST hyphenated token before
// the state, which is correct for most single-word cities.
func cityStateFromListingURL(u string) (city, state string) {
	if u == "" {
		return "", ""
	}
	idx := strings.Index(u, "://")
	if idx >= 0 {
		u = u[idx+3:]
	}
	if hostEnd := strings.Index(u, "/"); hostEnd >= 0 {
		u = u[hostEnd+1:]
	}
	u = strings.Trim(u, "/")
	parts := strings.Split(u, "/")
	if len(parts) == 0 {
		return "", ""
	}
	tokens := strings.Split(parts[0], "-")
	if len(tokens) < 2 {
		return "", ""
	}
	candState := strings.ToLower(tokens[len(tokens)-1])
	if len(candState) != 2 {
		return "", ""
	}
	candCity := strings.ToLower(tokens[len(tokens)-2])
	return candCity, candState
}

// lookupListingSnapshot returns the most recent placard observation for
// a listing URL from the local snapshots table. Used as a fallback when
// the live listing-detail fetch returns 403 (apartments.com listing
// pages have stricter bot protection than search pages).
func lookupListingSnapshot(ctx context.Context, listingURL string) (*apt.Listing, error) {
	s, err := openAptStore(ctx)
	if err != nil {
		return nil, err
	}
	defer s.Close()
	rows, err := apt.SnapshotsForURL(s.DB(), listingURL)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	r := rows[len(rows)-1] // SnapshotsForURL returns oldest-first; take the latest
	li := &apt.Listing{
		URL:        listingURL,
		PropertyID: apt.ListingURLToPropertyID(listingURL),
		Beds:       r.Beds,
		Baths:      r.Baths,
		MaxRent:    r.MaxRent,
	}
	return li, nil
}

// openAptStore opens the local SQLite store and ensures the apt
// extension schema is present. Callers must Close() the returned
// store.
func openAptStore(ctx context.Context) (*store.Store, error) {
	s, err := store.OpenWithContext(ctx, defaultDBPath("apartments-pp-cli"))
	if err != nil {
		return nil, fmt.Errorf("opening local store: %w", err)
	}
	if err := apt.EnsureExtSchema(s.DB()); err != nil {
		s.Close()
		return nil, fmt.Errorf("apt schema: %w", err)
	}
	return s, nil
}

// listingsRow is one cached listing as we read it back from the
// generic listing(id, data JSON) table.
type listingsRow struct {
	ID   string
	Data apt.Listing
}

// loadCachedListings returns every listing currently cached in the
// local store, decoded into apt.Listing. Filter is best-effort and
// applied in Go after the SQL fetch.
func loadCachedListings(db *sql.DB) ([]listingsRow, error) {
	rows, err := db.Query(`SELECT id, data FROM listing`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []listingsRow
	for rows.Next() {
		var (
			id   string
			data string
		)
		if err := rows.Scan(&id, &data); err != nil {
			return nil, err
		}
		var li apt.Listing
		if err := json.Unmarshal([]byte(data), &li); err != nil {
			continue
		}
		out = append(out, listingsRow{ID: id, Data: li})
	}
	return out, rows.Err()
}

// latestObservationPerURL returns the most recent SnapshotRow for each
// listing_url present in listing_snapshots. Used by drops/stale/etc.
func latestObservationPerURL(db *sql.DB) ([]apt.SnapshotRow, error) {
	rows, err := db.Query(
		`SELECT s.listing_url, s.property_id, s.saved_search, s.observed_at,
		        s.max_rent, s.beds, s.baths, s.available_at, s.fetch_status
		 FROM listing_snapshots s
		 INNER JOIN (
		   SELECT listing_url, MAX(observed_at) AS max_obs
		   FROM listing_snapshots
		   GROUP BY listing_url
		 ) m
		   ON s.listing_url = m.listing_url AND s.observed_at = m.max_obs`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []apt.SnapshotRow
	for rows.Next() {
		var (
			r           apt.SnapshotRow
			ts          string
			propertyID  sql.NullString
			savedSearch sql.NullString
			availableAt sql.NullString
			maxRent     sql.NullInt64
			beds        sql.NullInt64
			baths       sql.NullFloat64
			fetchStatus sql.NullInt64
		)
		if err := rows.Scan(&r.ListingURL, &propertyID, &savedSearch, &ts, &maxRent, &beds, &baths, &availableAt, &fetchStatus); err != nil {
			return nil, err
		}
		r.PropertyID = propertyID.String
		r.SavedSearch = savedSearch.String
		r.ObservedAt = parseSnapshotTime(ts)
		r.MaxRent = int(maxRent.Int64)
		r.Beds = int(beds.Int64)
		r.Baths = baths.Float64
		r.AvailableAt = availableAt.String
		r.FetchStatus = int(fetchStatus.Int64)
		out = append(out, r)
	}
	return out, rows.Err()
}

// parseSnapshotTime mirrors apt.parseStoredTime; we keep a copy here so
// the apt package stays consumable as a pure-data layer.
func parseSnapshotTime(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999 -0700 MST",
		"2006-01-02 15:04:05.999999 -0700 MST",
		"2006-01-02 15:04:05.999 -0700 MST",
		"2006-01-02 15:04:05 -0700 MST",
		"2006-01-02 15:04:05.999999999 -0700",
		"2006-01-02 15:04:05 -0700",
		"2006-01-02 15:04:05",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// parseDurationLoose understands time.ParseDuration plus a "Nd" suffix
// (days, integer N) so flags like --since 7d work without a custom flag
// type. Returns 0 + nil when the input is empty.
func parseDurationLoose(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	if strings.HasSuffix(s, "d") {
		var days int
		if _, err := fmt.Sscanf(s, "%dd", &days); err == nil && days >= 0 {
			return time.Duration(days) * 24 * time.Hour, nil
		}
	}
	return time.ParseDuration(s)
}

// median returns the middle value of a sorted-in-place copy of xs.
// Returns 0 for empty input.
func median(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	cp := make([]float64, len(xs))
	copy(cp, xs)
	sort.Float64s(cp)
	mid := len(cp) / 2
	if len(cp)%2 == 0 {
		return (cp[mid-1] + cp[mid]) / 2
	}
	return cp[mid]
}

// percentile returns the p-th percentile (0..100) using linear
// interpolation on a sorted copy of xs.
func percentile(xs []float64, p float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	cp := make([]float64, len(xs))
	copy(cp, xs)
	sort.Float64s(cp)
	if p <= 0 {
		return cp[0]
	}
	if p >= 100 {
		return cp[len(cp)-1]
	}
	idx := (p / 100.0) * float64(len(cp)-1)
	lo := int(idx)
	frac := idx - float64(lo)
	if lo+1 >= len(cp) {
		return cp[lo]
	}
	return cp[lo] + frac*(cp[lo+1]-cp[lo])
}
