// Hand-authored Immoweb domain tables. Kept in their own file so
// `generate --force` preserves them. Tables are created lazily by
// EnsureImmoSchema, which every novel command calls after opening the store.

package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/immoweb/internal/immo"
)

const immoSchema = `
CREATE TABLE IF NOT EXISTS immo_listings (
	id INTEGER PRIMARY KEY,
	deal TEXT, type TEXT, subtype TEXT, title TEXT,
	locality TEXT, postal_code TEXT, province TEXT, street TEXT,
	lat REAL, lng REAL,
	price REAL, rent_costs REAL, bedrooms INTEGER, surface REAL, land REAL,
	agency TEXT, private INTEGER DEFAULT 0, under_option INTEGER DEFAULT 0,
	new_price INTEGER DEFAULT 0, flag TEXT, epc TEXT,
	created_at TEXT, modified_at TEXT,
	first_seen TEXT NOT NULL, last_seen TEXT NOT NULL, gone_at TEXT,
	views INTEGER, bookmarks INTEGER, detail_at TEXT,
	data TEXT, old_price REAL
);
CREATE INDEX IF NOT EXISTS immo_listings_pc ON immo_listings(postal_code, deal, type);
CREATE TABLE IF NOT EXISTS immo_price_obs (
	listing_id INTEGER NOT NULL,
	observed_at TEXT NOT NULL,
	price REAL NOT NULL,
	PRIMARY KEY (listing_id, observed_at)
);
CREATE TABLE IF NOT EXISTS immo_saved (
	name TEXT PRIMARY KEY,
	criteria TEXT NOT NULL,
	created_at TEXT NOT NULL,
	last_run_at TEXT
);
CREATE TABLE IF NOT EXISTS immo_search_seen (
	search_name TEXT NOT NULL,
	listing_id INTEGER NOT NULL,
	first_seen TEXT NOT NULL,
	last_seen TEXT NOT NULL,
	last_price REAL,
	PRIMARY KEY (search_name, listing_id)
);
CREATE TABLE IF NOT EXISTS immo_hidden (
	listing_id INTEGER PRIMARY KEY,
	hidden_at TEXT NOT NULL,
	reason TEXT
);
CREATE TABLE IF NOT EXISTS immo_shortlist (
	listing_id INTEGER PRIMARY KEY,
	added_at TEXT NOT NULL,
	note TEXT
);
CREATE TABLE IF NOT EXISTS immo_pulls (
	scope TEXT PRIMARY KEY,
	pulled_at TEXT NOT NULL,
	listings INTEGER NOT NULL
);
`

// ImmoResource is the generic resource_type used for FTS indexing of
// listings in the framework `resources` table.
const ImmoResource = "listings"

// EnsureImmoSchema creates the Immoweb tables when missing.
func (s *Store) EnsureImmoSchema(ctx context.Context) error {
	s.lockForWrite()
	defer s.unlockAfterWrite()
	if _, err := s.db.ExecContext(ctx, immoSchema); err != nil {
		return fmt.Errorf("creating immoweb tables: %w", err)
	}
	// Columns added after the first schema; a concurrent ALTER may win the race.
	for _, col := range []struct{ table, name, decl string }{
		{"immo_listings", "old_price", "REAL"},
		{"immo_search_seen", "last_price", "REAL"},
	} {
		var n int
		if err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM pragma_table_info('`+col.table+`') WHERE name = ?`, col.name).Scan(&n); err == nil && n == 0 {
			if _, err := s.db.ExecContext(ctx, `ALTER TABLE `+col.table+` ADD COLUMN `+col.name+` `+col.decl); err != nil && !strings.Contains(err.Error(), "duplicate column") {
				return fmt.Errorf("adding %s.%s column: %w", col.table, col.name, err)
			}
		}
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

// UpsertImmoListings writes listings observed at `at` and appends a price
// observation whenever the price differs from the last one. Raw JSON is also indexed into the framework
// resources table (outside the write transaction) for offline FTS search.
func (s *Store) UpsertImmoListings(ctx context.Context, listings []immo.Listing, raws []json.RawMessage, at time.Time) error {
	if len(listings) == 0 {
		return nil
	}
	now := at.UTC().Format(time.RFC3339)
	s.lockForWrite()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		s.unlockAfterWrite()
		return err
	}
	for i, l := range listings {
		var lastPrice sql.NullFloat64
		_ = tx.QueryRowContext(ctx, `SELECT price FROM immo_price_obs WHERE listing_id = ? ORDER BY observed_at DESC LIMIT 1`, l.ID).Scan(&lastPrice)
		var raw any
		if i < len(raws) && len(raws[i]) > 0 {
			raw = string(raws[i])
		}
		_, err := tx.ExecContext(ctx, `
INSERT INTO immo_listings (id, deal, type, subtype, title, locality, postal_code, province, street, lat, lng,
	price, rent_costs, bedrooms, surface, land, agency, private, under_option, new_price, flag, epc,
	created_at, modified_at, first_seen, last_seen, gone_at, data, old_price)
VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,NULL,?,?)
ON CONFLICT(id) DO UPDATE SET
	deal=excluded.deal, type=excluded.type, subtype=excluded.subtype, title=COALESCE(NULLIF(excluded.title,''), immo_listings.title),
	locality=excluded.locality, postal_code=excluded.postal_code, province=excluded.province, street=excluded.street,
	lat=COALESCE(excluded.lat, immo_listings.lat), lng=COALESCE(excluded.lng, immo_listings.lng),
	price=excluded.price, rent_costs=COALESCE(excluded.rent_costs, immo_listings.rent_costs),
	bedrooms=COALESCE(excluded.bedrooms, immo_listings.bedrooms), surface=COALESCE(excluded.surface, immo_listings.surface),
	land=COALESCE(excluded.land, immo_listings.land), agency=excluded.agency, private=excluded.private,
	under_option=excluded.under_option, new_price=MAX(excluded.new_price, immo_listings.new_price), flag=COALESCE(NULLIF(excluded.flag,''), immo_listings.flag),
	epc=COALESCE(NULLIF(excluded.epc,''), immo_listings.epc),
	created_at=COALESCE(NULLIF(excluded.created_at,''), immo_listings.created_at),
	modified_at=COALESCE(NULLIF(excluded.modified_at,''), immo_listings.modified_at),
	last_seen=excluded.last_seen, gone_at=NULL, data=COALESCE(excluded.data, immo_listings.data),
	old_price=COALESCE(excluded.old_price, immo_listings.old_price)`,
			l.ID, l.Deal, l.Type, l.Subtype, l.Title, l.Locality, l.PostalCode, l.Province, l.Street, nullF(l.Lat), nullF(l.Lng),
			nullF(l.Price), nullF(l.RentCosts), nullI(l.Bedrooms), nullF(l.Surface), nullF(l.Land), l.Agency, b2i(l.Private), b2i(l.UnderOption), b2i(l.NewPrice), l.Flag, l.EPC,
			l.CreatedAt, l.ModifiedAt, now, now, raw, nullF(l.OldPrice))
		if err != nil {
			_ = tx.Rollback()
			s.unlockAfterWrite()
			return fmt.Errorf("upserting listing %d: %w", l.ID, err)
		}
		if l.Price != nil && *l.Price > 0 {
			if !lastPrice.Valid || lastPrice.Float64 != *l.Price {
				if _, err := tx.ExecContext(ctx, `INSERT OR REPLACE INTO immo_price_obs (listing_id, observed_at, price) VALUES (?,?,?)`, l.ID, now, *l.Price); err != nil {
					_ = tx.Rollback()
					s.unlockAfterWrite()
					return err
				}
			}
		}
	}
	if err := tx.Commit(); err != nil {
		s.unlockAfterWrite()
		return err
	}
	s.unlockAfterWrite()
	// FTS indexing through the framework helper (own transaction).
	for i, l := range listings {
		if i < len(raws) && len(raws[i]) > 0 {
			if err := s.Upsert(ImmoResource, fmt.Sprint(l.ID), raws[i]); err != nil {
				return fmt.Errorf("indexing listing %d: %w", l.ID, err)
			}
		}
	}
	return nil
}

// SaveDetail stores detail-only fields (EPC, views, bookmarks, created date)
// and indexes the full detail JSON for offline search. A sold/rented flag
// keeps the first gone date.
func (s *Store) SaveDetail(ctx context.Context, d immo.Detail, raw json.RawMessage, at time.Time) error {
	var prevGone sql.NullString
	_ = s.db.QueryRowContext(ctx, `SELECT gone_at FROM immo_listings WHERE id = ?`, d.ID).Scan(&prevGone)
	if err := s.UpsertImmoListings(ctx, []immo.Listing{d.Listing}, nil, at); err != nil {
		return err
	}
	now := at.UTC().Format(time.RFC3339)
	gone := any(nil)
	if d.Sold {
		gone = now
		if prevGone.Valid && prevGone.String != "" {
			gone = prevGone.String
		}
	}
	s.lockForWrite()
	_, err := s.db.ExecContext(ctx, `UPDATE immo_listings SET epc=COALESCE(NULLIF(?,''), epc), views=?, bookmarks=?,
		created_at=COALESCE(NULLIF(?,''), created_at), detail_at=?, gone_at=? WHERE id=?`,
		d.EPC, nullI(d.Views), nullI(d.Bookmarks), d.CreatedAt, now, gone, d.ID)
	s.unlockAfterWrite()
	if err != nil {
		return err
	}
	if len(raw) > 0 {
		return s.Upsert(ImmoResource, fmt.Sprint(d.ID), raw)
	}
	return nil
}

// StoredListing is a listing row plus local bookkeeping.
type StoredListing struct {
	immo.Listing
	FirstSeen string `json:"first_seen"`
	LastSeen  string `json:"last_seen"`
	GoneAt    string `json:"gone_at,omitempty"`
	Views     *int   `json:"views,omitempty"`
	Bookmarks *int   `json:"bookmarks,omitempty"`
}

// ListingFilter narrows local listing queries. Empty fields match all.
type ListingFilter struct {
	Deal        string
	Types       []string
	PostalCodes []string // bare 1050 or BE-1050
	IDs         []int64
	IncludeGone bool
	GoneSince   string // RFC3339; only gone listings gone after this
}

const listingCols = `id, COALESCE(deal,''), COALESCE(type,''), COALESCE(subtype,''), COALESCE(title,''), COALESCE(locality,''),
	COALESCE(postal_code,''), COALESCE(province,''), COALESCE(street,''), lat, lng, price, rent_costs, bedrooms, surface, land,
	COALESCE(agency,''), COALESCE(private,0), COALESCE(under_option,0), COALESCE(new_price,0), COALESCE(flag,''), COALESCE(epc,''),
	COALESCE(created_at,''), COALESCE(modified_at,''), first_seen, last_seen, COALESCE(gone_at,''), views, bookmarks, old_price`

func scanStored(rows *sql.Rows) (StoredListing, error) {
	var l StoredListing
	var lat, lng, price, costs, surface, land, oldPrice sql.NullFloat64
	var beds, views, bookmarks sql.NullInt64
	var priv, uo, np int
	err := rows.Scan(&l.ID, &l.Deal, &l.Type, &l.Subtype, &l.Title, &l.Locality, &l.PostalCode, &l.Province, &l.Street,
		&lat, &lng, &price, &costs, &beds, &surface, &land, &l.Agency, &priv, &uo, &np, &l.Flag, &l.EPC,
		&l.CreatedAt, &l.ModifiedAt, &l.FirstSeen, &l.LastSeen, &l.GoneAt, &views, &bookmarks, &oldPrice)
	if err != nil {
		return l, err
	}
	f := func(n sql.NullFloat64) *float64 {
		if !n.Valid {
			return nil
		}
		v := n.Float64
		return &v
	}
	i := func(n sql.NullInt64) *int {
		if !n.Valid {
			return nil
		}
		v := int(n.Int64)
		return &v
	}
	l.Lat, l.Lng, l.Price, l.RentCosts, l.Surface, l.Land, l.OldPrice = f(lat), f(lng), f(price), f(costs), f(surface), f(land), f(oldPrice)
	l.Bedrooms, l.Views, l.Bookmarks = i(beds), i(views), i(bookmarks)
	l.Private, l.UnderOption, l.NewPrice = priv == 1, uo == 1, np == 1
	l.URL = immo.ListingURL(l.ID)
	l.PricePerSqm = immo.PricePerSqm(l.Deal, l.Price, l.Surface)
	return l, nil
}

// QueryListings returns stored listings matching the filter (drain-first).
func (s *Store) QueryListings(ctx context.Context, f ListingFilter) ([]StoredListing, error) {
	where := []string{"1=1"}
	args := []any{}
	if f.Deal != "" {
		where = append(where, "deal = ?")
		args = append(args, f.Deal)
	}
	if len(f.Types) > 0 {
		where = append(where, "type IN ("+placeholders(len(f.Types))+")")
		for _, t := range f.Types {
			args = append(args, t)
		}
	}
	if len(f.PostalCodes) > 0 {
		where = append(where, "postal_code IN ("+placeholders(len(f.PostalCodes))+")")
		for _, pc := range f.PostalCodes {
			args = append(args, immo.BarePostalCode(pc))
		}
	}
	if len(f.IDs) > 0 {
		where = append(where, "id IN ("+placeholders(len(f.IDs))+")")
		for _, id := range f.IDs {
			args = append(args, id)
		}
	}
	if f.GoneSince != "" {
		where = append(where, "gone_at IS NOT NULL AND gone_at >= ?")
		args = append(args, f.GoneSince)
	} else if !f.IncludeGone {
		where = append(where, "gone_at IS NULL")
	}
	// #nosec G202 -- listingCols and every where fragment are string constants; filter values travel only as ? args.
	rows, err := s.db.QueryContext(ctx, "SELECT "+listingCols+" FROM immo_listings WHERE "+strings.Join(where, " AND ")+" ORDER BY last_seen DESC, id DESC", args...)
	if err != nil {
		return nil, err
	}
	out := make([]StoredListing, 0)
	for rows.Next() {
		l, err := scanStored(rows)
		if err != nil {
			_ = rows.Close()
			return nil, err
		}
		out = append(out, l)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	return out, rows.Close()
}

func placeholders(n int) string {
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}

// PriceObs is one recorded asking price.
type PriceObs struct {
	ObservedAt string  `json:"observed_at"`
	Price      float64 `json:"price"`
}

// PriceHistory returns the recorded prices of one listing, oldest first.
func (s *Store) PriceHistory(ctx context.Context, id int64) ([]PriceObs, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT observed_at, price FROM immo_price_obs WHERE listing_id = ? ORDER BY observed_at`, id)
	if err != nil {
		return nil, err
	}
	out := make([]PriceObs, 0)
	for rows.Next() {
		var p PriceObs
		if err := rows.Scan(&p.ObservedAt, &p.Price); err != nil {
			_ = rows.Close()
			return nil, err
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	return out, rows.Close()
}

// AllPriceHistories returns price observations for every listing that has at
// least two distinct recorded prices.
func (s *Store) AllPriceHistories(ctx context.Context) (map[int64][]PriceObs, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT listing_id, observed_at, price FROM immo_price_obs
		WHERE listing_id IN (SELECT listing_id FROM immo_price_obs GROUP BY listing_id HAVING COUNT(1) > 1)
		ORDER BY listing_id, observed_at`)
	if err != nil {
		return nil, err
	}
	out := map[int64][]PriceObs{}
	for rows.Next() {
		var id int64
		var p PriceObs
		if err := rows.Scan(&id, &p.ObservedAt, &p.Price); err != nil {
			_ = rows.Close()
			return nil, err
		}
		out[id] = append(out[id], p)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	return out, rows.Close()
}

// SavedSearch is a named local search.
type SavedSearch struct {
	Name      string        `json:"name"`
	Criteria  immo.Criteria `json:"criteria"`
	CreatedAt string        `json:"created_at"`
	LastRunAt string        `json:"last_run_at,omitempty"`
}

// SaveSearch creates or replaces a named search. Changing the criteria of
// an existing name resets its watch history so the next watch starts from a
// fresh baseline instead of reporting every listing as new or gone.
func (s *Store) SaveSearch(ctx context.Context, name string, c immo.Criteria) error {
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
	defer tx.Rollback()
	var prev sql.NullString
	_ = tx.QueryRowContext(ctx, `SELECT criteria FROM immo_saved WHERE name = ?`, name).Scan(&prev)
	if prev.Valid && prev.String != string(b) {
		if _, err := tx.ExecContext(ctx, `UPDATE immo_saved SET last_run_at = NULL WHERE name = ?`, name); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM immo_search_seen WHERE search_name = ?`, name); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO immo_saved (name, criteria, created_at) VALUES (?,?,?)
		ON CONFLICT(name) DO UPDATE SET criteria=excluded.criteria`, name, string(b), time.Now().UTC().Format(time.RFC3339)); err != nil {
		return err
	}
	return tx.Commit()
}

// GetSearch loads a saved search; sql.ErrNoRows when missing.
func (s *Store) GetSearch(ctx context.Context, name string) (SavedSearch, error) {
	var ss SavedSearch
	var crit string
	var last sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT name, criteria, created_at, last_run_at FROM immo_saved WHERE name = ?`, name).Scan(&ss.Name, &crit, &ss.CreatedAt, &last)
	if err != nil {
		return ss, err
	}
	ss.LastRunAt = last.String
	return ss, json.Unmarshal([]byte(crit), &ss.Criteria)
}

// ListSearches returns every saved search.
func (s *Store) ListSearches(ctx context.Context) ([]SavedSearch, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT name, criteria, created_at, COALESCE(last_run_at,'') FROM immo_saved ORDER BY name`)
	if err != nil {
		return nil, err
	}
	out := make([]SavedSearch, 0)
	for rows.Next() {
		var ss SavedSearch
		var crit string
		if err := rows.Scan(&ss.Name, &crit, &ss.CreatedAt, &ss.LastRunAt); err != nil {
			_ = rows.Close()
			return nil, err
		}
		if err := json.Unmarshal([]byte(crit), &ss.Criteria); err != nil {
			_ = rows.Close()
			return nil, err
		}
		out = append(out, ss)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	return out, rows.Close()
}

// DeleteSearch removes a saved search and its seen-set. Returns false when
// the search did not exist.
func (s *Store) DeleteSearch(ctx context.Context, name string) (bool, error) {
	s.lockForWrite()
	defer s.unlockAfterWrite()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	r, err := tx.ExecContext(ctx, `DELETE FROM immo_saved WHERE name = ?`, name)
	if err != nil {
		return false, err
	}
	n, _ := r.RowsAffected()
	if _, err := tx.ExecContext(ctx, `DELETE FROM immo_search_seen WHERE search_name = ?`, name); err != nil {
		return false, err
	}
	return n > 0, tx.Commit()
}

// SeenListing is what a saved search saw for a listing at its last run.
type SeenListing struct {
	LastSeen  string
	LastPrice *float64
}

// SeenIDs returns the listings recorded for a saved search at its last run.
func (s *Store) SeenIDs(ctx context.Context, name string) (map[int64]SeenListing, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT listing_id, last_seen, last_price FROM immo_search_seen WHERE search_name = ?`, name)
	if err != nil {
		return nil, err
	}
	out := map[int64]SeenListing{}
	for rows.Next() {
		var id int64
		var sl SeenListing
		var price sql.NullFloat64
		if err := rows.Scan(&id, &sl.LastSeen, &price); err != nil {
			_ = rows.Close()
			return nil, err
		}
		if price.Valid {
			v := price.Float64
			sl.LastPrice = &v
		}
		out[id] = sl
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	return out, rows.Close()
}

// RecordSearchRun stores the listings (and prices) a saved search returned,
// forgets the ones that left the search, and stamps last_run_at. It does not
// decide whether a listing is gone from Immoweb; callers use MarkGone for that.
func (s *Store) RecordSearchRun(ctx context.Context, name string, prices map[int64]*float64, left []int64, at time.Time) error {
	now := at.UTC().Format(time.RFC3339)
	s.lockForWrite()
	defer s.unlockAfterWrite()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for id, price := range prices {
		if _, err := tx.ExecContext(ctx, `INSERT INTO immo_search_seen (search_name, listing_id, first_seen, last_seen, last_price) VALUES (?,?,?,?,?)
			ON CONFLICT(search_name, listing_id) DO UPDATE SET last_seen=excluded.last_seen, last_price=excluded.last_price`, name, id, now, now, nullF(price)); err != nil {
			return err
		}
	}
	for _, id := range left {
		if _, err := tx.ExecContext(ctx, `DELETE FROM immo_search_seen WHERE search_name = ? AND listing_id = ?`, name, id); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE immo_saved SET last_run_at = ? WHERE name = ?`, now, name); err != nil {
		return err
	}
	return tx.Commit()
}

// MarkGone flags listings (e.g. from an area pull) that were not returned by
// a complete harvest of their scope, skipping any listing seen at or after
// seenBefore (the harvest start). It returns how many were marked.
func (s *Store) MarkGone(ctx context.Context, ids []int64, seenBefore, at time.Time) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	now := at.UTC().Format(time.RFC3339)
	cutoff := seenBefore.UTC().Format(time.RFC3339)
	s.lockForWrite()
	defer s.unlockAfterWrite()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	marked := 0
	for _, id := range ids {
		// last_seen < cutoff keeps an overlapping, newer pull that saw the
		// listing after this harvest started from being overwritten.
		res, err := tx.ExecContext(ctx, `UPDATE immo_listings SET gone_at = COALESCE(gone_at, ?) WHERE id = ? AND last_seen < ?`, now, id, cutoff)
		if err != nil {
			return 0, err
		}
		if n, _ := res.RowsAffected(); n > 0 {
			marked++
		}
	}
	return marked, tx.Commit()
}

// HiddenSet returns all hidden listing IDs.
func (s *Store) HiddenSet(ctx context.Context) (map[int64]bool, error) {
	return s.idSet(ctx, `SELECT listing_id FROM immo_hidden`)
}

// ShortlistSet returns all shortlisted listing IDs.
func (s *Store) ShortlistSet(ctx context.Context) (map[int64]bool, error) {
	return s.idSet(ctx, `SELECT listing_id FROM immo_shortlist`)
}

func (s *Store) idSet(ctx context.Context, q string) (map[int64]bool, error) {
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	out := map[int64]bool{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return nil, err
		}
		out[id] = true
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	return out, rows.Close()
}

// SetHidden hides (or un-hides) a listing.
func (s *Store) SetHidden(ctx context.Context, id int64, hide bool, reason string) error {
	s.lockForWrite()
	defer s.unlockAfterWrite()
	var err error
	if hide {
		_, err = s.db.ExecContext(ctx, `INSERT INTO immo_hidden (listing_id, hidden_at, reason) VALUES (?,?,?)
			ON CONFLICT(listing_id) DO UPDATE SET reason=excluded.reason`, id, time.Now().UTC().Format(time.RFC3339), reason)
	} else {
		_, err = s.db.ExecContext(ctx, `DELETE FROM immo_hidden WHERE listing_id = ?`, id)
	}
	return err
}

// ShortlistEntry is one shortlisted listing.
type ShortlistEntry struct {
	ID      int64  `json:"id"`
	AddedAt string `json:"added_at"`
	Note    string `json:"note,omitempty"`
}

// SetShortlist adds (with note) or removes a shortlisted listing.
func (s *Store) SetShortlist(ctx context.Context, id int64, add bool, note string) error {
	s.lockForWrite()
	defer s.unlockAfterWrite()
	var err error
	if add {
		_, err = s.db.ExecContext(ctx, `INSERT INTO immo_shortlist (listing_id, added_at, note) VALUES (?,?,?)
			ON CONFLICT(listing_id) DO UPDATE SET note=COALESCE(NULLIF(excluded.note,''), immo_shortlist.note)`, id, time.Now().UTC().Format(time.RFC3339), note)
	} else {
		_, err = s.db.ExecContext(ctx, `DELETE FROM immo_shortlist WHERE listing_id = ?`, id)
	}
	return err
}

// ListShortlist returns shortlisted entries, newest first.
func (s *Store) ListShortlist(ctx context.Context) ([]ShortlistEntry, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT listing_id, added_at, COALESCE(note,'') FROM immo_shortlist ORDER BY added_at DESC`)
	if err != nil {
		return nil, err
	}
	out := make([]ShortlistEntry, 0)
	for rows.Next() {
		var e ShortlistEntry
		if err := rows.Scan(&e.ID, &e.AddedAt, &e.Note); err != nil {
			_ = rows.Close()
			return nil, err
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	return out, rows.Close()
}

// RecordPull stamps a harvested scope (used for freshness decisions).
func (s *Store) RecordPull(ctx context.Context, scope string, n int, at time.Time) error {
	s.lockForWrite()
	defer s.unlockAfterWrite()
	_, err := s.db.ExecContext(ctx, `INSERT INTO immo_pulls (scope, pulled_at, listings) VALUES (?,?,?)
		ON CONFLICT(scope) DO UPDATE SET pulled_at=excluded.pulled_at, listings=excluded.listings`, scope, at.UTC().Format(time.RFC3339), n)
	return err
}

// LastPull returns when a scope was last harvested (zero time when never).
func (s *Store) LastPull(ctx context.Context, scope string) (time.Time, int, error) {
	var at string
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT pulled_at, listings FROM immo_pulls WHERE scope = ?`, scope).Scan(&at, &n)
	if err == sql.ErrNoRows {
		return time.Time{}, 0, nil
	}
	if err != nil {
		return time.Time{}, 0, err
	}
	t, _ := time.Parse(time.RFC3339, at)
	return t, n, nil
}
