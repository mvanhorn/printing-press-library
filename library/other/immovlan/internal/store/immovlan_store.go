// Hand-authored Immovlan domain tables. Kept in their own file so
// `generate --force` preserves them. Tables are created lazily by
// EnsureVlanSchema, which every novel command calls after opening the store.

package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/immovlan/internal/immovlan"
)

const vlanSchema = `
CREATE TABLE IF NOT EXISTS vlan_listings (
	id TEXT PRIMARY KEY,
	deal TEXT, type TEXT, subtype TEXT, title TEXT,
	locality TEXT, postal_code TEXT, street TEXT,
	lat REAL, lng REAL,
	price REAL, bedrooms INTEGER, bathrooms INTEGER, surface REAL, land REAL,
	agency TEXT, agency_id TEXT, agency_url TEXT, seller_type TEXT, private INTEGER DEFAULT 0, phone TEXT,
	flag TEXT, epc TEXT, epc_band TEXT,
	condition TEXT, rented INTEGER, cadastral_income REAL, year INTEGER, garden REAL, terrace REAL, heating TEXT, software TEXT,
	description TEXT, pictures TEXT, photo_hash TEXT, addr_key TEXT,
	created_at TEXT, detail_at TEXT,
	first_seen TEXT NOT NULL, last_seen TEXT NOT NULL, gone_at TEXT
);
CREATE INDEX IF NOT EXISTS vlan_listings_pc ON vlan_listings(postal_code, deal, type);
CREATE INDEX IF NOT EXISTS vlan_listings_addr ON vlan_listings(addr_key);
CREATE INDEX IF NOT EXISTS vlan_listings_photo ON vlan_listings(photo_hash);
CREATE TABLE IF NOT EXISTS vlan_price_obs (
	listing_id TEXT NOT NULL,
	observed_at TEXT NOT NULL,
	price REAL NOT NULL,
	PRIMARY KEY (listing_id, observed_at)
);
CREATE TABLE IF NOT EXISTS vlan_saved (
	name TEXT PRIMARY KEY,
	criteria TEXT NOT NULL,
	created_at TEXT NOT NULL,
	last_run_at TEXT
);
CREATE TABLE IF NOT EXISTS vlan_search_seen (
	search_name TEXT NOT NULL,
	listing_id TEXT NOT NULL,
	first_seen TEXT NOT NULL,
	last_seen TEXT NOT NULL,
	last_price REAL,
	PRIMARY KEY (search_name, listing_id)
);
CREATE TABLE IF NOT EXISTS vlan_hidden (
	listing_id TEXT PRIMARY KEY,
	hidden_at TEXT NOT NULL,
	reason TEXT
);
CREATE TABLE IF NOT EXISTS vlan_meta (key TEXT PRIMARY KEY, value TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS vlan_shortlist (
	listing_id TEXT PRIMARY KEY,
	added_at TEXT NOT NULL,
	note TEXT
);
`

// VlanResource is the generic resource_type used for FTS indexing of
// listings in the framework `resources` table.
const VlanResource = "listings"

// EnsureVlanSchema creates the Immovlan tables when missing.
func (s *Store) EnsureVlanSchema(ctx context.Context) error {
	s.lockForWrite()
	defer s.unlockAfterWrite()
	if _, err := s.db.ExecContext(ctx, vlanSchema); err != nil {
		return fmt.Errorf("creating immovlan tables: %w", err)
	}
	return s.migratePhotoHashes(ctx)
}

// photoHashVersion bumps whenever PhotoHash changes its recipe; stored
// hashes are then recomputed from the stored picture lists.
const photoHashVersion = "3" // 2: file-name segment; 3: sha256

func (s *Store) migratePhotoHashes(ctx context.Context) error {
	var v string
	_ = s.db.QueryRowContext(ctx, `SELECT value FROM vlan_meta WHERE key = 'photo_hash_version'`).Scan(&v)
	if v == photoHashVersion {
		return nil
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, pictures FROM vlan_listings WHERE pictures IS NOT NULL AND pictures <> ''`)
	if err != nil {
		return err
	}
	updates := map[string]string{}
	for rows.Next() {
		var id, pics string
		if err := rows.Scan(&id, &pics); err != nil {
			_ = rows.Close()
			return err
		}
		var urls []string
		if json.Unmarshal([]byte(pics), &urls) == nil {
			updates[id] = PhotoHash(urls)
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err // never stamp the version after a partial scan
	}
	if err := rows.Close(); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for id, h := range updates {
		if _, err := tx.ExecContext(ctx, `UPDATE vlan_listings SET photo_hash = NULLIF(?, '') WHERE id = ?`, h, id); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO vlan_meta (key, value) VALUES ('photo_hash_version', ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, photoHashVersion); err != nil {
		return err
	}
	return tx.Commit()
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

func nullB(p *bool) any {
	if p == nil {
		return nil
	}
	if *p {
		return 1
	}
	return 0
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

var addrSeparators = strings.NewReplacer("/", " ", ",", " ", ";", " ")

// houseNumber: the first integer after the street name, allowing a short
// suffix (57bis, 57 a). Numbers that are part of the street name ("avenue du
// 11 novembre") are skipped by streetNumberWords below.
var houseNumber = regexp.MustCompile(`^(.*?[a-z])\s+(\d+)(?:bis|ter|[a-d])?(?:\s|$)`)

var streetNumberWords = regexp.MustCompile(`\b(\d+)\s+(janvier|fevrier|mars|avril|mai|juin|juillet|aout|septembre|octobre|novembre|decembre|januari|februari|maart|april|mei|juni|juli|augustus|oktober|december|er|eme|e|de|ste|de)\b`)

var streetWords = regexp.MustCompile(`\b(rue|avenue|av|chaussee|chee|boulevard|bd|place|pl|square|straat|laan|steenweg|plein|clos|drève|dreve|allee)\b`)

// AddrKey normalises "postcode|street|number" so the same property matches
// across references, portals and spellings. Empty when no house number.
func AddrKey(postcode, street string) string {
	f := immovlan.Fold(addrSeparators.Replace(street))
	f = streetWords.ReplaceAllString(f, " ")
	// "11 novembre", "4 septembre", "1er": glue the number to the word so it
	// is not read as the house number.
	f = streetNumberWords.ReplaceAllString(f, "$1$2")
	f = strings.Join(strings.Fields(f), " ")
	// First integer after the street name: "anspach 1 3", "de la poste 12 14"
	// and "de waterloo 1234 bte 5" all key on the first number, as Immoweb
	// stores the bare number.
	m := houseNumber.FindStringSubmatch(f)
	if m == nil || postcode == "" {
		return ""
	}
	name := strings.TrimSpace(m[1])
	if name == "" {
		return ""
	}
	num := strings.TrimLeft(m[2], "0")
	if num == "" {
		return ""
	}
	return postcode + "|" + name + "|" + num
}

// PhotoHash fingerprints a listing by its first photo file names, which
// survive re-listing under a new reference and cross-portal syndication.
// Immovlan URLs end with a size segment (".../<file>.jpg/Large"), so the
// file name is the last path segment carrying an image extension.
func PhotoHash(urls []string) string {
	stems := []string{}
	for i, u := range urls {
		if i >= 10 {
			break
		}
		if name := photoFileName(u); name != "" {
			stems = append(stems, name)
		}
	}
	if len(stems) == 0 {
		return ""
	}
	// Fingerprint only (never a security boundary); sha256 keeps static
	// analysis quiet and the 8-byte prefix is plenty for de-duplication.
	h := sha256.Sum256([]byte(strings.Join(stems, "|")))
	return hex.EncodeToString(h[:8])
}

func photoFileName(u string) string {
	u = strings.ToLower(strings.SplitN(u, "?", 2)[0])
	segs := strings.Split(u, "/")
	for i := len(segs) - 1; i >= 0; i-- {
		for _, ext := range []string{".jpg", ".jpeg", ".png", ".webp", ".gif"} {
			if strings.HasSuffix(segs[i], ext) {
				return segs[i]
			}
		}
	}
	if len(segs) > 0 {
		return segs[len(segs)-1]
	}
	return ""
}

// UpsertVlanListings writes search-card listings observed at `at` and appends
// a price observation whenever the price differs from the last one.
func (s *Store) UpsertVlanListings(ctx context.Context, listings []immovlan.Listing, at time.Time) error {
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
	raws := make([]json.RawMessage, 0, len(listings))
	for _, l := range listings {
		var lastPrice sql.NullFloat64
		_ = tx.QueryRowContext(ctx, `SELECT price FROM vlan_price_obs WHERE listing_id = ? ORDER BY observed_at DESC LIMIT 1`, l.ID).Scan(&lastPrice)
		_, err := tx.ExecContext(ctx, `
INSERT INTO vlan_listings (id, deal, type, subtype, title, locality, postal_code, street, lat, lng,
	price, bedrooms, bathrooms, surface, land, agency, agency_id, seller_type, private, phone, flag, epc, epc_band,
	description, addr_key, created_at, first_seen, last_seen, gone_at)
VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,NULL)
ON CONFLICT(id) DO UPDATE SET
	deal=COALESCE(NULLIF(excluded.deal,''), vlan_listings.deal), type=COALESCE(NULLIF(excluded.type,''), vlan_listings.type),
	subtype=COALESCE(NULLIF(excluded.subtype,''), vlan_listings.subtype),
	title=COALESCE(NULLIF(excluded.title,''), vlan_listings.title),
	locality=COALESCE(NULLIF(excluded.locality,''), vlan_listings.locality),
	postal_code=COALESCE(NULLIF(excluded.postal_code,''), vlan_listings.postal_code),
	street=COALESCE(NULLIF(excluded.street,''), vlan_listings.street),
	lat=COALESCE(excluded.lat, vlan_listings.lat), lng=COALESCE(excluded.lng, vlan_listings.lng),
	price=COALESCE(excluded.price, vlan_listings.price),
	bedrooms=COALESCE(excluded.bedrooms, vlan_listings.bedrooms), bathrooms=COALESCE(excluded.bathrooms, vlan_listings.bathrooms),
	surface=COALESCE(excluded.surface, vlan_listings.surface), land=COALESCE(excluded.land, vlan_listings.land),
	agency=COALESCE(NULLIF(excluded.agency,''), vlan_listings.agency), agency_id=COALESCE(NULLIF(excluded.agency_id,''), vlan_listings.agency_id),
	seller_type=COALESCE(NULLIF(excluded.seller_type,''), vlan_listings.seller_type),
	private=CASE WHEN excluded.seller_type<>'' THEN excluded.private ELSE vlan_listings.private END,
	phone=COALESCE(NULLIF(excluded.phone,''), vlan_listings.phone),
	flag=COALESCE(NULLIF(excluded.flag,''), vlan_listings.flag),
	epc=COALESCE(NULLIF(excluded.epc,''), vlan_listings.epc), epc_band=COALESCE(NULLIF(excluded.epc_band,''), vlan_listings.epc_band),
	description=COALESCE(NULLIF(excluded.description,''), vlan_listings.description),
	addr_key=COALESCE(NULLIF(excluded.addr_key,''), vlan_listings.addr_key),
	created_at=COALESCE(NULLIF(excluded.created_at,''), vlan_listings.created_at),
	last_seen=excluded.last_seen, gone_at=NULL`,
			l.ID, l.Deal, l.Type, l.Subtype, l.Title, l.Locality, l.PostalCode, l.Street, nullF(l.Lat), nullF(l.Lng),
			nullF(l.Price), nullI(l.Bedrooms), nullI(l.Bathrooms), nullF(l.Surface), nullF(l.Land), l.Agency, l.AgencyID, l.SellerType, b2i(l.Private), l.Phone, l.Flag, l.EPC, l.EPCBand,
			l.Description, AddrKey(l.PostalCode, l.Street), l.CreatedAt, now, now)
		if err != nil {
			_ = tx.Rollback()
			s.unlockAfterWrite()
			return fmt.Errorf("upserting listing %s: %w", l.ID, err)
		}
		if l.Price != nil && *l.Price > 0 && (!lastPrice.Valid || lastPrice.Float64 != *l.Price) {
			if _, err := tx.ExecContext(ctx, `INSERT OR REPLACE INTO vlan_price_obs (listing_id, observed_at, price) VALUES (?,?,?)`, l.ID, now, *l.Price); err != nil {
				_ = tx.Rollback()
				s.unlockAfterWrite()
				return err
			}
		}
		if b, err := json.Marshal(l); err == nil {
			raws = append(raws, b)
		}
	}
	if err := tx.Commit(); err != nil {
		s.unlockAfterWrite()
		return err
	}
	s.unlockAfterWrite()
	// FTS indexing through the framework helper (own transaction).
	for i, l := range listings {
		if i < len(raws) {
			if err := s.Upsert(VlanResource, l.ID, raws[i]); err != nil {
				return fmt.Errorf("indexing listing %s: %w", l.ID, err)
			}
		}
	}
	return nil
}

// SaveVlanDetail stores the detail-only fields of a listing page.
func (s *Store) SaveVlanDetail(ctx context.Context, d immovlan.Detail, at time.Time) error {
	if err := s.UpsertVlanListings(ctx, []immovlan.Listing{d.Listing}, at); err != nil {
		return err
	}
	now := at.UTC().Format(time.RFC3339)
	var pics any
	if len(d.Photos) > 0 {
		b, _ := json.Marshal(d.Photos)
		pics = string(b)
	}
	s.lockForWrite()
	_, err := s.db.ExecContext(ctx, `UPDATE vlan_listings SET
		condition=COALESCE(NULLIF(?,''), condition), rented=COALESCE(?, rented), cadastral_income=COALESCE(?, cadastral_income),
		year=COALESCE(?, year), garden=COALESCE(?, garden), terrace=COALESCE(?, terrace), heating=COALESCE(NULLIF(?,''), heating),
		software=COALESCE(NULLIF(?,''), software), agency_url=COALESCE(NULLIF(?,''), agency_url),
		pictures=COALESCE(?, pictures), photo_hash=COALESCE(NULLIF(?,''), photo_hash), detail_at=? WHERE id=?`,
		d.Condition, nullB(d.Rented), nullF(d.CadastralIncome), nullI(d.Year), nullF(d.Garden), nullF(d.Terrace), d.Heating,
		d.Software, d.AgencyURL, pics, PhotoHash(d.Photos), now, d.ID)
	s.unlockAfterWrite()
	if err != nil {
		return err
	}
	if b, err := json.Marshal(d); err == nil {
		return s.Upsert(VlanResource, d.ID, b)
	}
	return nil
}

// StoredListing is a listing row plus local bookkeeping and detail fields.
type StoredListing struct {
	immovlan.Listing
	Condition       string   `json:"condition,omitempty"`
	Rented          *bool    `json:"rented,omitempty"`
	CadastralIncome *float64 `json:"cadastral_income,omitempty"`
	Year            *int     `json:"construction_year,omitempty"`
	Garden          *float64 `json:"garden_m2,omitempty"`
	Terrace         *float64 `json:"terrace_m2,omitempty"`
	Heating         string   `json:"heating,omitempty"`
	Software        string   `json:"software,omitempty"`
	AgencyURL       string   `json:"agency_url,omitempty"`
	Photos          []string `json:"pictures,omitempty"`
	PhotoHash       string   `json:"photo_hash,omitempty"`
	AddrKey         string   `json:"addr_key,omitempty"`
	DetailAt        string   `json:"detail_at,omitempty"`
	FirstSeen       string   `json:"first_seen"`
	LastSeen        string   `json:"last_seen"`
	GoneAt          string   `json:"gone_at,omitempty"`
}

// ListingFilter narrows local listing queries. Empty fields match all.
type ListingFilter struct {
	Deal        string
	Types       []string
	PostalCodes []string
	IDs         []string
	EPC         []string // letters
	Rented      *bool
	MinSurface  float64
	MinBedrooms int
	MaxPrice    int
	MissingAny  []string // columns that must be NULL/empty for the row to match (enrich)
	IncludeGone bool
	// DetailBefore keeps rows whose detail page was never read or was read
	// before this RFC3339 instant (enrich: do not re-read a page that simply
	// lacks the field). OldestDetailFirst orders never-read rows first, then
	// the oldest reads, so a capped pass makes progress run after run.
	DetailBefore      string
	OldestDetailFirst bool
}

const vlanCols = `id, COALESCE(deal,''), COALESCE(type,''), COALESCE(subtype,''), COALESCE(title,''), COALESCE(locality,''),
	COALESCE(postal_code,''), COALESCE(street,''), lat, lng, price, bedrooms, bathrooms, surface, land,
	COALESCE(agency,''), COALESCE(agency_id,''), COALESCE(agency_url,''), COALESCE(seller_type,''), COALESCE(private,0), COALESCE(phone,''),
	COALESCE(flag,''), COALESCE(epc,''), COALESCE(epc_band,''),
	COALESCE(condition,''), rented, cadastral_income, year, garden, terrace, COALESCE(heating,''), COALESCE(software,''),
	COALESCE(description,''), COALESCE(pictures,''), COALESCE(photo_hash,''), COALESCE(addr_key,''),
	COALESCE(created_at,''), COALESCE(detail_at,''), first_seen, last_seen, COALESCE(gone_at,'')`

func scanVlan(rows *sql.Rows) (StoredListing, error) {
	var l StoredListing
	var lat, lng, price, surface, land, rc, garden, terrace sql.NullFloat64
	var beds, baths, rented, year sql.NullInt64
	var priv int
	var pics string
	err := rows.Scan(&l.ID, &l.Deal, &l.Type, &l.Subtype, &l.Title, &l.Locality, &l.PostalCode, &l.Street,
		&lat, &lng, &price, &beds, &baths, &surface, &land,
		&l.Agency, &l.AgencyID, &l.AgencyURL, &l.SellerType, &priv, &l.Phone, &l.Flag, &l.EPC, &l.EPCBand,
		&l.Condition, &rented, &rc, &year, &garden, &terrace, &l.Heating, &l.Software,
		&l.Description, &pics, &l.PhotoHash, &l.AddrKey, &l.CreatedAt, &l.DetailAt, &l.FirstSeen, &l.LastSeen, &l.GoneAt)
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
	l.Lat, l.Lng, l.Price, l.Surface, l.Land, l.CadastralIncome, l.Garden, l.Terrace = f(lat), f(lng), f(price), f(surface), f(land), f(rc), f(garden), f(terrace)
	l.Bedrooms, l.Bathrooms, l.Year = i(beds), i(baths), i(year)
	if rented.Valid {
		b := rented.Int64 == 1
		l.Rented = &b
	}
	l.Private = priv == 1
	if pics != "" {
		_ = json.Unmarshal([]byte(pics), &l.Photos)
	}
	l.URL = immovlan.ListingURL(l.ID)
	l.PricePerSqm = immovlan.PricePerSqm(l.Deal, l.Price, l.Surface)
	return l, nil
}

var missingCols = map[string]string{
	"epc": "epc", "street": "street", "geo": "lat", "rented": "rented", "cadastral": "cadastral_income",
	"software": "software", "year": "year", "condition": "condition", "photos": "pictures", "detail": "detail_at",
}

// MissingColumn maps an --missing token to its column, false when unknown.
func MissingColumn(token string) (string, bool) {
	c, ok := missingCols[strings.ToLower(strings.TrimSpace(token))]
	return c, ok
}

// QueryVlanListings returns stored listings matching the filter (drain-first).
func (s *Store) QueryVlanListings(ctx context.Context, f ListingFilter) ([]StoredListing, error) {
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
			args = append(args, strings.TrimPrefix(strings.ToUpper(pc), "BE-"))
		}
	}
	if len(f.IDs) > 0 {
		where = append(where, "id IN ("+placeholders(len(f.IDs))+")")
		for _, id := range f.IDs {
			args = append(args, strings.ToUpper(id))
		}
	}
	if len(f.EPC) > 0 {
		where = append(where, "UPPER(epc) IN ("+placeholders(len(f.EPC))+")")
		for _, e := range f.EPC {
			args = append(args, strings.ToUpper(e))
		}
	}
	if f.Rented != nil {
		where = append(where, "rented = ?")
		args = append(args, b2i(*f.Rented))
	}
	if f.MinSurface > 0 {
		where = append(where, "surface >= ?")
		args = append(args, f.MinSurface)
	}
	if f.MinBedrooms > 0 {
		where = append(where, "bedrooms >= ?")
		args = append(args, f.MinBedrooms)
	}
	if f.MaxPrice > 0 {
		where = append(where, "(price IS NULL OR price <= ?)")
		args = append(args, f.MaxPrice)
	}
	if len(f.MissingAny) > 0 {
		ors := []string{}
		for _, tok := range f.MissingAny {
			col, ok := MissingColumn(tok)
			if !ok {
				return nil, fmt.Errorf("unknown --missing field %q", tok)
			}
			ors = append(ors, "("+col+" IS NULL OR "+col+" = '')")
		}
		where = append(where, "("+strings.Join(ors, " OR ")+")")
	}
	if !f.IncludeGone {
		where = append(where, "gone_at IS NULL")
	}
	if f.DetailBefore != "" {
		where = append(where, "(detail_at IS NULL OR detail_at < ?)")
		args = append(args, f.DetailBefore)
	}
	order := " ORDER BY last_seen DESC, id DESC"
	if f.OldestDetailFirst {
		order = " ORDER BY detail_at IS NOT NULL, detail_at ASC, last_seen DESC, id DESC"
	}
	// vlanCols, missingCols values and every where fragment are constants; filter values travel only as ? args.
	q := "SELECT " + vlanCols + " FROM vlan_listings WHERE " + strings.Join(where, " AND ") + order // #nosec G202 -- constant fragments, values bound as ? args
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	out := make([]StoredListing, 0)
	for rows.Next() {
		l, err := scanVlan(rows)
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

// VlanPriceHistory returns the recorded prices of one listing, oldest first.
func (s *Store) VlanPriceHistory(ctx context.Context, id string) ([]PriceObs, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT observed_at, price FROM vlan_price_obs WHERE listing_id = ? ORDER BY observed_at`, strings.ToUpper(id))
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

// AllVlanPriceHistories returns observations for listings with 2+ prices.
func (s *Store) AllVlanPriceHistories(ctx context.Context) (map[string][]PriceObs, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT listing_id, observed_at, price FROM vlan_price_obs
		WHERE listing_id IN (SELECT listing_id FROM vlan_price_obs GROUP BY listing_id HAVING COUNT(1) > 1)
		ORDER BY listing_id, observed_at`)
	if err != nil {
		return nil, err
	}
	out := map[string][]PriceObs{}
	for rows.Next() {
		var id string
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
	Name      string            `json:"name"`
	Criteria  immovlan.Criteria `json:"criteria"`
	CreatedAt string            `json:"created_at"`
	LastRunAt string            `json:"last_run_at,omitempty"`
}

// SaveSearch creates or replaces a named search; changed criteria reset the
// watch history so the next run is a fresh baseline.
func (s *Store) SaveSearch(ctx context.Context, name string, c immovlan.Criteria) error {
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
	_ = tx.QueryRowContext(ctx, `SELECT criteria FROM vlan_saved WHERE name = ?`, name).Scan(&prev)
	changed := false
	if prev.Valid {
		var old immovlan.Criteria
		changed = json.Unmarshal([]byte(prev.String), &old) != nil || old.Key() != c.Key()
	}
	if changed {
		if _, err := tx.ExecContext(ctx, `UPDATE vlan_saved SET last_run_at = NULL WHERE name = ?`, name); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM vlan_search_seen WHERE search_name = ?`, name); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO vlan_saved (name, criteria, created_at) VALUES (?,?,?)
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
	err := s.db.QueryRowContext(ctx, `SELECT name, criteria, created_at, last_run_at FROM vlan_saved WHERE name = ?`, name).Scan(&ss.Name, &crit, &ss.CreatedAt, &last)
	if err != nil {
		return ss, err
	}
	ss.LastRunAt = last.String
	return ss, json.Unmarshal([]byte(crit), &ss.Criteria)
}

// ListSearches returns every saved search.
func (s *Store) ListSearches(ctx context.Context) ([]SavedSearch, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT name, criteria, created_at, COALESCE(last_run_at,'') FROM vlan_saved ORDER BY name`)
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

// DeleteSearch removes a saved search and its seen-set.
func (s *Store) DeleteSearch(ctx context.Context, name string) (bool, error) {
	s.lockForWrite()
	defer s.unlockAfterWrite()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	r, err := tx.ExecContext(ctx, `DELETE FROM vlan_saved WHERE name = ?`, name)
	if err != nil {
		return false, err
	}
	n, _ := r.RowsAffected()
	if _, err := tx.ExecContext(ctx, `DELETE FROM vlan_search_seen WHERE search_name = ?`, name); err != nil {
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
func (s *Store) SeenIDs(ctx context.Context, name string) (map[string]SeenListing, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT listing_id, last_seen, last_price FROM vlan_search_seen WHERE search_name = ?`, name)
	if err != nil {
		return nil, err
	}
	out := map[string]SeenListing{}
	for rows.Next() {
		var id string
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

// RecordSearchRun stores what a saved search returned and forgets the
// listings that left it; callers use MarkGone to declare a listing gone.
func (s *Store) RecordSearchRun(ctx context.Context, name string, prices map[string]*float64, left []string, at time.Time) error {
	now := at.UTC().Format(time.RFC3339)
	s.lockForWrite()
	defer s.unlockAfterWrite()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for id, price := range prices {
		if _, err := tx.ExecContext(ctx, `INSERT INTO vlan_search_seen (search_name, listing_id, first_seen, last_seen, last_price) VALUES (?,?,?,?,?)
			ON CONFLICT(search_name, listing_id) DO UPDATE SET last_seen=excluded.last_seen, last_price=excluded.last_price`, name, id, now, now, nullF(price)); err != nil {
			return err
		}
	}
	for _, id := range left {
		if _, err := tx.ExecContext(ctx, `DELETE FROM vlan_search_seen WHERE search_name = ? AND listing_id = ?`, name, id); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE vlan_saved SET last_run_at = ? WHERE name = ?`, now, name); err != nil {
		return err
	}
	return tx.Commit()
}

// MarkGoneNow flags listings gone regardless of when they were last seen
// (enrich: the detail page answered 404).
func (s *Store) MarkGoneNow(ctx context.Context, ids []string) (int, error) {
	now := time.Now()
	return s.MarkGone(ctx, ids, now.Add(24*time.Hour), now)
}

// MarkGone flags listings a complete harvest did not return, skipping any
// listing seen at or after seenBefore (the harvest start).
func (s *Store) MarkGone(ctx context.Context, ids []string, seenBefore, at time.Time) (int, error) {
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
		res, err := tx.ExecContext(ctx, `UPDATE vlan_listings SET gone_at = COALESCE(gone_at, ?) WHERE id = ? AND last_seen < ?`, now, id, cutoff)
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
func (s *Store) HiddenSet(ctx context.Context) (map[string]bool, error) {
	return s.idSet(ctx, `SELECT listing_id FROM vlan_hidden`)
}

func (s *Store) idSet(ctx context.Context, q string) (map[string]bool, error) {
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	out := map[string]bool{}
	for rows.Next() {
		var id string
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
func (s *Store) SetHidden(ctx context.Context, id string, hide bool, reason string) error {
	s.lockForWrite()
	defer s.unlockAfterWrite()
	var err error
	if hide {
		_, err = s.db.ExecContext(ctx, `INSERT INTO vlan_hidden (listing_id, hidden_at, reason) VALUES (?,?,?)
			ON CONFLICT(listing_id) DO UPDATE SET reason=excluded.reason`, id, time.Now().UTC().Format(time.RFC3339), reason)
	} else {
		_, err = s.db.ExecContext(ctx, `DELETE FROM vlan_hidden WHERE listing_id = ?`, id)
	}
	return err
}

// ShortlistEntry is one shortlisted listing.
type ShortlistEntry struct {
	ID      string `json:"id"`
	AddedAt string `json:"added_at"`
	Note    string `json:"note,omitempty"`
}

// SetShortlist adds (with note) or removes a shortlisted listing.
func (s *Store) SetShortlist(ctx context.Context, id string, add bool, note string) error {
	s.lockForWrite()
	defer s.unlockAfterWrite()
	var err error
	if add {
		_, err = s.db.ExecContext(ctx, `INSERT INTO vlan_shortlist (listing_id, added_at, note) VALUES (?,?,?)
			ON CONFLICT(listing_id) DO UPDATE SET note=COALESCE(NULLIF(excluded.note,''), vlan_shortlist.note)`, id, time.Now().UTC().Format(time.RFC3339), note)
	} else {
		_, err = s.db.ExecContext(ctx, `DELETE FROM vlan_shortlist WHERE listing_id = ?`, id)
	}
	return err
}

// ListShortlist returns shortlisted entries, newest first.
func (s *Store) ListShortlist(ctx context.Context) ([]ShortlistEntry, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT listing_id, added_at, COALESCE(note,'') FROM vlan_shortlist ORDER BY added_at DESC`)
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

// LocalPostcodesFor maps a locality name to the postcodes seen in the store.
func (s *Store) LocalPostcodesFor(ctx context.Context, name string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT postal_code, locality FROM vlan_listings WHERE postal_code <> ''`)
	if err != nil {
		return nil, err
	}
	want := immovlan.Fold(name)
	if want == "" {
		_ = rows.Close()
		return nil, nil // punctuation-only names must not match empty localities
	}
	out := []string{}
	seen := map[string]bool{}
	for rows.Next() {
		var pc, loc string
		if err := rows.Scan(&pc, &loc); err != nil {
			_ = rows.Close()
			return nil, err
		}
		if immovlan.Fold(loc) == want && !seen[pc] {
			seen[pc] = true
			out = append(out, pc)
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	return out, rows.Close()
}
