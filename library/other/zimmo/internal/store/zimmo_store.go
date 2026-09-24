// Copyright 2026 sambassio and contributors. Licensed under Apache-2.0. See LICENSE.

// Hand-authored Zimmo domain tables. Kept in their own file so
// `generate --force` preserves them. Tables are created lazily by
// EnsureZimmoSchema, which every hand-written command calls after opening
// the store.

package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/zimmo/internal/zimmo"
)

const zimmoSchema = `
CREATE TABLE IF NOT EXISTS zm_listings (
	code TEXT PRIMARY KEY,
	id TEXT,
	status TEXT, type TEXT, subtype TEXT,
	postal_code TEXT, locality TEXT, addr_key TEXT,
	lat REAL, lng REAL,
	price REAL, surface REAL, bedrooms INTEGER,
	epc TEXT, epc_kwh REAL, renovation TEXT, rented INTEGER DEFAULT 0,
	agency_id TEXT,
	published_at TEXT,
	data TEXT NOT NULL,
	detail_at TEXT,
	first_seen TEXT NOT NULL, last_seen TEXT NOT NULL, gone_at TEXT
);
CREATE INDEX IF NOT EXISTS zm_listings_pc ON zm_listings(postal_code, status, type);
CREATE INDEX IF NOT EXISTS zm_listings_addr ON zm_listings(addr_key);
CREATE TABLE IF NOT EXISTS zm_price_obs (
	code TEXT NOT NULL,
	observed_at TEXT NOT NULL,
	price REAL NOT NULL,
	PRIMARY KEY (code, observed_at)
);
CREATE TABLE IF NOT EXISTS zm_saved (
	name TEXT PRIMARY KEY,
	criteria TEXT NOT NULL,
	created_at TEXT NOT NULL,
	last_run_at TEXT
);
CREATE TABLE IF NOT EXISTS zm_search_seen (
	search_name TEXT NOT NULL,
	code TEXT NOT NULL,
	first_seen TEXT NOT NULL,
	last_seen TEXT NOT NULL,
	last_price REAL,
	PRIMARY KEY (search_name, code)
);
CREATE TABLE IF NOT EXISTS zm_shortlist (
	code TEXT PRIMARY KEY,
	added_at TEXT NOT NULL,
	note TEXT
);
CREATE TABLE IF NOT EXISTS zm_places (
	query TEXT PRIMARY KEY,
	data TEXT NOT NULL,
	fetched_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS zm_locality_price (
	place_id INTEGER NOT NULL,
	sub INTEGER NOT NULL,
	data TEXT NOT NULL,
	fetched_at TEXT NOT NULL,
	PRIMARY KEY (place_id, sub)
);
`

// ZimmoResource is the resource_type used for FTS indexing of listings in
// the framework resources table.
const ZimmoResource = "listings"

// EnsureZimmoSchema creates the Zimmo tables when missing.
func (s *Store) EnsureZimmoSchema(ctx context.Context) error {
	s.lockForWrite()
	defer s.unlockAfterWrite()
	if _, err := s.db.ExecContext(ctx, zimmoSchema); err != nil {
		return fmt.Errorf("creating zimmo tables: %w", err)
	}
	return nil
}

func nullF(p *float64) any {
	if p == nil {
		return nil
	}
	return *p
}

func nullI(p *int) any {
	if p == nil {
		return nil
	}
	return *p
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// StoredListing is a listing plus its local bookkeeping.
type StoredListing struct {
	zimmo.Listing
	FirstSeen string `json:"first_seen"`
	LastSeen  string `json:"last_seen"`
	GoneAt    string `json:"gone_at,omitempty"`
	DetailAt  string `json:"detail_at,omitempty"`
}

// UpsertZimmoListings writes listings observed at `at`, appends a price
// observation when the price changed, and indexes them for `search`.
// detail=true stamps detail_at (the row came from a per-listing fetch).
func (s *Store) UpsertZimmoListings(ctx context.Context, listings []zimmo.Listing, at time.Time, detail bool) error {
	if len(listings) == 0 {
		return nil
	}
	now := at.UTC().Format(time.RFC3339)
	var detailAt any
	if detail {
		detailAt = now
	}
	s.lockForWrite()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		s.unlockAfterWrite()
		return err
	}
	type indexed struct {
		code string
		data json.RawMessage
	}
	toIndex := make([]indexed, 0, len(listings))
	fail := func(err error) error {
		_ = tx.Rollback()
		s.unlockAfterWrite()
		return err
	}
	for _, l := range listings {
		if l.Code == "" {
			continue
		}
		var prev string
		hasPrev := tx.QueryRowContext(ctx, `SELECT data FROM zm_listings WHERE code = ?`, l.Code).Scan(&prev) == nil
		// Keep geocoded or rooftop coordinates when the incoming ones are
		// coarser (a search result after an enrich geocode).
		if hasPrev && l.GeoPrecision != "ROOFTOP" {
			var old zimmo.Listing
			if json.Unmarshal([]byte(prev), &old) == nil && old.Lat != nil && (old.GeoPrecision == "ROOFTOP" || old.GeoPrecision == "GEOCODED") {
				l.Lat, l.Lng, l.GeoPrecision = old.Lat, old.Lng, old.GeoPrecision
			}
		}
		data, err := json.Marshal(l)
		if err != nil {
			return fail(err)
		}
		// A search result is thinner than an enriched listing: fields the new
		// payload lacks (description, documents, price history, flags...) are
		// kept from the stored JSON instead of being erased.
		if hasPrev {
			data = mergeListingJSON([]byte(prev), data)
		}
		var lastPrice sql.NullFloat64
		_ = tx.QueryRowContext(ctx, `SELECT price FROM zm_price_obs WHERE code = ? ORDER BY observed_at DESC LIMIT 1`, l.Code).Scan(&lastPrice)
		// A listing that left the market (sold/rented) is marked gone.
		var gone any
		if l.Status == "SOLD" || l.Status == "RENTED" {
			gone = now
		}
		_, err = tx.ExecContext(ctx, `
INSERT INTO zm_listings (code, id, status, type, subtype, postal_code, locality, addr_key, lat, lng,
	price, surface, bedrooms, epc, epc_kwh, renovation, rented, agency_id, published_at, data, detail_at,
	first_seen, last_seen, gone_at)
VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
ON CONFLICT(code) DO UPDATE SET
	id=excluded.id, status=excluded.status, type=excluded.type, subtype=excluded.subtype,
	postal_code=excluded.postal_code, locality=excluded.locality,
	addr_key=COALESCE(NULLIF(excluded.addr_key,''), zm_listings.addr_key),
	lat=COALESCE(excluded.lat, zm_listings.lat), lng=COALESCE(excluded.lng, zm_listings.lng),
	price=excluded.price, surface=COALESCE(excluded.surface, zm_listings.surface),
	bedrooms=COALESCE(excluded.bedrooms, zm_listings.bedrooms),
	epc=COALESCE(NULLIF(excluded.epc,''), zm_listings.epc), epc_kwh=COALESCE(excluded.epc_kwh, zm_listings.epc_kwh),
	renovation=COALESCE(NULLIF(excluded.renovation,''), zm_listings.renovation),
	rented=excluded.rented, agency_id=excluded.agency_id,
	published_at=COALESCE(excluded.published_at, zm_listings.published_at),
	data=excluded.data, detail_at=COALESCE(excluded.detail_at, zm_listings.detail_at),
	last_seen=excluded.last_seen,
	gone_at=CASE WHEN excluded.gone_at IS NOT NULL THEN COALESCE(zm_listings.gone_at, excluded.gone_at) ELSE NULL END`,
			l.Code, l.ID, l.Status, l.Type, l.SubType, l.PostalCode, l.Locality, AddrKey(l.PostalCode, l.Street+" "+l.Number),
			nullF(l.Lat), nullF(l.Lng), nullF(l.Price), nullF(l.Surface), nullI(l.Bedrooms),
			l.EPC, nullF(l.EPCKWh), l.RenovationDuty, b2i(l.Rented), l.AgencyID, nullS(l.PublishedAt), string(data), detailAt,
			now, now, gone)
		if err != nil {
			return fail(fmt.Errorf("upserting listing %s: %w", l.Code, err))
		}
		if l.Price != nil && *l.Price > 0 && (!lastPrice.Valid || lastPrice.Float64 != *l.Price) {
			if _, err := tx.ExecContext(ctx, `INSERT OR REPLACE INTO zm_price_obs (code, observed_at, price) VALUES (?,?,?)`, l.Code, now, *l.Price); err != nil {
				return fail(err)
			}
		}
		toIndex = append(toIndex, indexed{l.Code, data})
	}
	if err := tx.Commit(); err != nil {
		s.unlockAfterWrite()
		return err
	}
	s.unlockAfterWrite()
	// FTS indexing through the framework helper (own transaction).
	for _, it := range toIndex {
		if err := s.Upsert(ZimmoResource, it.code, it.data); err != nil {
			return fmt.Errorf("indexing listing %s: %w", it.code, err)
		}
	}
	return nil
}

// mergeListingJSON overlays next on prev: every key next carries with a
// non-empty value wins; keys next omits or leaves empty (null, "", [], {})
// keep prev's value. Invalid JSON on either side returns next unchanged.
func mergeListingJSON(prev, next []byte) []byte {
	var p, n map[string]json.RawMessage
	if json.Unmarshal(prev, &p) != nil || json.Unmarshal(next, &n) != nil {
		return next
	}
	for k, v := range p {
		nv, ok := n[k]
		if !ok || isEmptyJSON(nv) {
			n[k] = v
		}
	}
	out, err := json.Marshal(n)
	if err != nil {
		return next
	}
	return out
}

func isEmptyJSON(v json.RawMessage) bool {
	switch strings.TrimSpace(string(v)) {
	case "", "null", `""`, "[]", "{}":
		return true
	}
	return false
}

func nullS(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// UpdateZimmoGeo stores geocoded coordinates (or a failed-geocode marker)
// without touching last_seen, which means "seen by a search".
func (s *Store) UpdateZimmoGeo(ctx context.Context, l zimmo.Listing) error {
	data, err := json.Marshal(l)
	if err != nil {
		return err
	}
	s.lockForWrite()
	defer s.unlockAfterWrite()
	_, err = s.db.ExecContext(ctx, `UPDATE zm_listings SET lat = ?, lng = ?, data = ? WHERE code = ?`, nullF(l.Lat), nullF(l.Lng), string(data), l.Code)
	return err
}

// MarkZimmoGone stamps gone_at for codes that disappeared (404).
func (s *Store) MarkZimmoGone(ctx context.Context, codes []string, at time.Time) error {
	if len(codes) == 0 {
		return nil
	}
	s.lockForWrite()
	defer s.unlockAfterWrite()
	now := at.UTC().Format(time.RFC3339)
	for _, c := range codes {
		if _, err := s.db.ExecContext(ctx, `UPDATE zm_listings SET gone_at = COALESCE(gone_at, ?) WHERE code = ?`, now, c); err != nil {
			return err
		}
	}
	return nil
}

// ListingFilter selects stored listings.
type ListingFilter struct {
	Codes       []string
	IDs         []string // listing UUIDs
	Postcodes   []string
	Statuses    []string
	Types       []string
	MaxPrice    *float64
	MinPrice    *float64
	IncludeGone bool
	OnlyGone    bool
	Limit       int
}

func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}

// QueryZimmoListings returns stored listings matching f, newest first.
func (s *Store) QueryZimmoListings(ctx context.Context, f ListingFilter) ([]StoredListing, error) {
	var where []string
	var args []any
	add := func(col string, vals []string) {
		if len(vals) == 0 {
			return
		}
		where = append(where, col+" IN ("+placeholders(len(vals))+")")
		for _, v := range vals {
			args = append(args, v)
		}
	}
	add("code", f.Codes)
	add("id", f.IDs)
	add("postal_code", f.Postcodes)
	add("status", f.Statuses)
	add("type", f.Types)
	if f.MaxPrice != nil {
		where = append(where, "price <= ?")
		args = append(args, *f.MaxPrice)
	}
	if f.MinPrice != nil {
		where = append(where, "price >= ?")
		args = append(args, *f.MinPrice)
	}
	switch {
	case f.OnlyGone:
		where = append(where, "gone_at IS NOT NULL")
	case !f.IncludeGone:
		where = append(where, "gone_at IS NULL")
	}
	q := `SELECT data, first_seen, last_seen, COALESCE(gone_at,''), COALESCE(detail_at,'') FROM zm_listings`
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ") // #nosec G202 -- fragments are literal column names with ? placeholders; values are bound args.
	}
	q += " ORDER BY COALESCE(NULLIF(published_at,''), first_seen) DESC, code"
	if f.Limit > 0 {
		q += " LIMIT ?"
		args = append(args, f.Limit)
	}
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]StoredListing, 0)
	for rows.Next() {
		var data string
		var sl StoredListing
		if err := rows.Scan(&data, &sl.FirstSeen, &sl.LastSeen, &sl.GoneAt, &sl.DetailAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(data), &sl.Listing); err != nil {
			return nil, fmt.Errorf("decoding stored listing: %w", err)
		}
		sl.Listing.Normalize()
		out = append(out, sl)
	}
	return out, rows.Err()
}

// CountZimmoListings counts active stored listings.
func (s *Store) CountZimmoListings(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM zm_listings`).Scan(&n)
	return n, err
}

// PriceObs is one local price observation.
type PriceObs struct {
	ObservedAt string  `json:"observed_at"`
	Price      float64 `json:"price"`
}

// AllZimmoPriceHistories returns every code's local observations, oldest first.
func (s *Store) AllZimmoPriceHistories(ctx context.Context) (map[string][]PriceObs, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT code, observed_at, price FROM zm_price_obs ORDER BY code, observed_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]PriceObs{}
	for rows.Next() {
		var code string
		var o PriceObs
		if err := rows.Scan(&code, &o.ObservedAt, &o.Price); err != nil {
			return nil, err
		}
		out[code] = append(out[code], o)
	}
	return out, rows.Err()
}

// SavedSearch is a named criteria set used by watch.
type SavedSearch struct {
	Name      string         `json:"name"`
	Criteria  zimmo.Criteria `json:"criteria"`
	CreatedAt string         `json:"created_at"`
	LastRunAt string         `json:"last_run_at,omitempty"`
}

// SaveZimmoSearch creates or replaces a saved search.
func (s *Store) SaveZimmoSearch(ctx context.Context, name string, c zimmo.Criteria, at time.Time) error {
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	s.lockForWrite()
	defer s.unlockAfterWrite()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var old string
	_ = tx.QueryRowContext(ctx, `SELECT criteria FROM zm_saved WHERE name = ?`, name).Scan(&old)
	if old != "" && old != string(b) {
		// New criteria: the previous seen-set would report every old result
		// as gone and every new one as new. Start over as a first run.
		if _, err := tx.ExecContext(ctx, `DELETE FROM zm_search_seen WHERE search_name = ?`, name); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE zm_saved SET last_run_at = NULL WHERE name = ?`, name); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO zm_saved (name, criteria, created_at) VALUES (?,?,?)
ON CONFLICT(name) DO UPDATE SET criteria=excluded.criteria`, name, string(b), at.UTC().Format(time.RFC3339)); err != nil {
		return err
	}
	return tx.Commit()
}

// ZimmoSavedSearches lists saved searches (all when name is empty).
func (s *Store) ZimmoSavedSearches(ctx context.Context, name string) ([]SavedSearch, error) {
	q := `SELECT name, criteria, created_at, COALESCE(last_run_at,'') FROM zm_saved`
	var args []any
	if name != "" {
		q += ` WHERE name = ?`
		args = append(args, name)
	}
	q += ` ORDER BY name`
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]SavedSearch, 0)
	for rows.Next() {
		var ss SavedSearch
		var crit string
		if err := rows.Scan(&ss.Name, &crit, &ss.CreatedAt, &ss.LastRunAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(crit), &ss.Criteria); err != nil {
			return nil, err
		}
		out = append(out, ss)
	}
	return out, rows.Err()
}

// DeleteZimmoSearch removes a saved search and its seen-set.
func (s *Store) DeleteZimmoSearch(ctx context.Context, name string) (bool, error) {
	s.lockForWrite()
	defer s.unlockAfterWrite()
	res, err := s.db.ExecContext(ctx, `DELETE FROM zm_saved WHERE name = ?`, name)
	if err != nil {
		return false, err
	}
	_, _ = s.db.ExecContext(ctx, `DELETE FROM zm_search_seen WHERE search_name = ?`, name)
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// SeenEntry is a listing a saved search saw before.
type SeenEntry struct {
	Code      string
	LastPrice *float64
}

// ZimmoSearchSeen returns the seen-set of a saved search.
func (s *Store) ZimmoSearchSeen(ctx context.Context, name string) (map[string]SeenEntry, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT code, last_price FROM zm_search_seen WHERE search_name = ?`, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]SeenEntry{}
	for rows.Next() {
		var e SeenEntry
		var p sql.NullFloat64
		if err := rows.Scan(&e.Code, &p); err != nil {
			return nil, err
		}
		if p.Valid {
			v := p.Float64
			e.LastPrice = &v
		}
		out[e.Code] = e
	}
	return out, rows.Err()
}

// ReplaceZimmoSearchSeen records the current result set of a saved search.
func (s *Store) ReplaceZimmoSearchSeen(ctx context.Context, name string, current []zimmo.Listing, at time.Time) error {
	now := at.UTC().Format(time.RFC3339)
	s.lockForWrite()
	defer s.unlockAfterWrite()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	// Delete departed codes one by one: a single NOT IN list would hit
	// SQLite's bound-variable limit on large searches.
	keep := map[string]bool{}
	for _, l := range current {
		keep[l.Code] = true
	}
	rows, err := tx.QueryContext(ctx, `SELECT code FROM zm_search_seen WHERE search_name = ?`, name)
	if err != nil {
		return err
	}
	var drop []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			_ = rows.Close()
			return err
		}
		if !keep[c] {
			drop = append(drop, c)
		}
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, c := range drop {
		if _, err := tx.ExecContext(ctx, `DELETE FROM zm_search_seen WHERE search_name = ? AND code = ?`, name, c); err != nil {
			return err
		}
	}
	for _, l := range current {
		if _, err := tx.ExecContext(ctx, `INSERT INTO zm_search_seen (search_name, code, first_seen, last_seen, last_price) VALUES (?,?,?,?,?)
ON CONFLICT(search_name, code) DO UPDATE SET last_seen=excluded.last_seen, last_price=COALESCE(excluded.last_price, zm_search_seen.last_price)`,
			name, l.Code, now, now, nullF(l.Price)); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE zm_saved SET last_run_at = ? WHERE name = ?`, now, name); err != nil {
		return err
	}
	return tx.Commit()
}

// MergeZimmoSearchSeen records the listings an incomplete run saw without
// dropping the others, then prunes entries unseen for 30 days so a search
// that never completes cannot grow without bound.
func (s *Store) MergeZimmoSearchSeen(ctx context.Context, name string, current []zimmo.Listing, at time.Time) error {
	now := at.UTC().Format(time.RFC3339)
	s.lockForWrite()
	defer s.unlockAfterWrite()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, l := range current {
		if _, err := tx.ExecContext(ctx, `INSERT INTO zm_search_seen (search_name, code, first_seen, last_seen, last_price) VALUES (?,?,?,?,?)
ON CONFLICT(search_name, code) DO UPDATE SET last_seen=excluded.last_seen, last_price=COALESCE(excluded.last_price, zm_search_seen.last_price)`,
			name, l.Code, now, now, nullF(l.Price)); err != nil {
			return err
		}
	}
	cutoff := at.Add(-30 * 24 * time.Hour).UTC().Format(time.RFC3339)
	if _, err := tx.ExecContext(ctx, `DELETE FROM zm_search_seen WHERE search_name = ? AND last_seen < ?`, name, cutoff); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE zm_saved SET last_run_at = ? WHERE name = ?`, now, name); err != nil {
		return err
	}
	return tx.Commit()
}

// ShortlistEntry is a starred listing.
type ShortlistEntry struct {
	Code    string `json:"zimmo_code"`
	AddedAt string `json:"added_at"`
	Note    string `json:"note,omitempty"`
}

// ShortlistAdd stars a listing with an optional note.
func (s *Store) ShortlistAdd(ctx context.Context, code, note string, at time.Time) error {
	s.lockForWrite()
	defer s.unlockAfterWrite()
	_, err := s.db.ExecContext(ctx, `INSERT INTO zm_shortlist (code, added_at, note) VALUES (?,?,?)
ON CONFLICT(code) DO UPDATE SET note=CASE WHEN excluded.note<>'' THEN excluded.note ELSE zm_shortlist.note END`, code, at.UTC().Format(time.RFC3339), note)
	return err
}

// ShortlistRemove un-stars a listing.
func (s *Store) ShortlistRemove(ctx context.Context, code string) (bool, error) {
	s.lockForWrite()
	defer s.unlockAfterWrite()
	res, err := s.db.ExecContext(ctx, `DELETE FROM zm_shortlist WHERE code = ?`, code)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// Shortlist returns starred listings, newest first.
func (s *Store) Shortlist(ctx context.Context) ([]ShortlistEntry, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT code, added_at, COALESCE(note,'') FROM zm_shortlist ORDER BY added_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]ShortlistEntry, 0)
	for rows.Next() {
		var e ShortlistEntry
		if err := rows.Scan(&e.Code, &e.AddedAt, &e.Note); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// CachedJSON reads a cached geo/price payload younger than maxAge.
func (s *Store) CachedPlaces(ctx context.Context, query string, maxAge time.Duration) ([]byte, bool) {
	var data, at string
	if err := s.db.QueryRowContext(ctx, `SELECT data, fetched_at FROM zm_places WHERE query = ?`, query).Scan(&data, &at); err != nil {
		return nil, false
	}
	t, err := time.Parse(time.RFC3339, at)
	if err != nil || time.Since(t) > maxAge {
		return nil, false
	}
	return []byte(data), true
}

// PutPlaces caches a places lookup.
func (s *Store) PutPlaces(ctx context.Context, query string, data []byte, at time.Time) error {
	s.lockForWrite()
	defer s.unlockAfterWrite()
	_, err := s.db.ExecContext(ctx, `INSERT OR REPLACE INTO zm_places (query, data, fetched_at) VALUES (?,?,?)`, query, string(data), at.UTC().Format(time.RFC3339))
	return err
}

// CachedLocalityPrice reads a cached €/m² payload younger than maxAge.
func (s *Store) CachedLocalityPrice(ctx context.Context, placeID int, sub bool, maxAge time.Duration) ([]byte, bool) {
	var data, at string
	if err := s.db.QueryRowContext(ctx, `SELECT data, fetched_at FROM zm_locality_price WHERE place_id = ? AND sub = ?`, placeID, b2i(sub)).Scan(&data, &at); err != nil {
		return nil, false
	}
	t, err := time.Parse(time.RFC3339, at)
	if err != nil || time.Since(t) > maxAge {
		return nil, false
	}
	return []byte(data), true
}

// PutLocalityPrice caches a €/m² payload.
func (s *Store) PutLocalityPrice(ctx context.Context, placeID int, sub bool, data []byte, at time.Time) error {
	s.lockForWrite()
	defer s.unlockAfterWrite()
	_, err := s.db.ExecContext(ctx, `INSERT OR REPLACE INTO zm_locality_price (place_id, sub, data, fetched_at) VALUES (?,?,?,?)`, placeID, b2i(sub), string(data), at.UTC().Format(time.RFC3339))
	return err
}

var addrSeparators = strings.NewReplacer("/", " ", ",", " ", ";", " ", "-", " ")

var houseNumber = regexp.MustCompile(`^(.*?[a-z])\s+(\d+)(?:bis|ter|[a-d])?(?:\s|$)`)

var streetNumberWords = regexp.MustCompile(`\b(\d+)\s+(janvier|fevrier|mars|avril|mai|juin|juillet|aout|septembre|octobre|novembre|decembre|januari|februari|maart|april|mei|juni|juli|augustus|oktober|december|er|eme|e|de|ste)\b`)

var streetWords = regexp.MustCompile(`\b(rue|avenue|av|chaussee|chee|boulevard|bd|place|pl|square|straat|laan|steenweg|plein|clos|dreve|allee)\b`)

// AddrKey normalises "postcode|street|number" so the same property
// matches across portals and spellings. same-as computes it for sibling
// stores too, so both sides use this one recipe. Empty when there is
// no house number.
func AddrKey(postcode, street string) string {
	f := zimmo.Fold(addrSeparators.Replace(street))
	f = streetWords.ReplaceAllString(f, " ")
	f = streetNumberWords.ReplaceAllString(f, "$1$2")
	f = strings.Join(strings.Fields(f), " ")
	m := houseNumber.FindStringSubmatch(f)
	if m == nil || postcode == "" {
		return ""
	}
	name := strings.TrimSpace(m[1])
	num := strings.TrimLeft(m[2], "0")
	if name == "" || num == "" {
		return ""
	}
	return postcode + "|" + name + "|" + num
}
