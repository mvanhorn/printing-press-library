package offerup

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// itemQuery is the synthetic query key under which item-detail fetches (from
// `listings get`) store their listing, so seller-scan can find owner-linked
// listings without colliding with keyword-search rows.
const itemQuery = ":item"

// Store is the OfferUp-specific SQLite layer. It opens the same database file
// the framework uses (database/sql, the carve-out for novel commands that
// operate on the local SQLite file) and owns the offerup_* tables that hold
// cleaned listings, per-listing price history, and seller reputation — the data
// the price-intelligence commands aggregate over.
type Store struct {
	db *sql.DB
}

// OpenStore opens (creating if needed) the SQLite database and ensures the
// offerup_* tables exist. WAL mode + busy_timeout match the framework store so
// the two handles coexist without lock contention.
func OpenStore(dbPath string) (*Store, error) {
	// Ensure the parent directory exists before sqlite tries to create the file;
	// on a fresh install the XDG data dir is absent and the open fails with
	// SQLITE_CANTOPEN ("unable to open database file"), which strands every
	// store-backed command on first use.
	if dir := filepath.Dir(dbPath); dir != "" {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return nil, err
		}
	}
	db, err := sql.Open("sqlite", "file:"+dbPath+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // serialize writes; modernc.org/sqlite is happiest single-writer
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS offerup_listings (
			query TEXT NOT NULL,
			listing_id TEXT NOT NULL,
			title TEXT,
			price REAL,
			price_text TEXT,
			location TEXT,
			condition TEXT,
			is_firm INTEGER,
			vehicle_miles TEXT,
			flags TEXT,
			owner_id TEXT,
			url TEXT,
			image_url TEXT,
			first_seen TEXT,
			last_seen TEXT,
			last_price REAL,
			PRIMARY KEY (query, listing_id)
		)`,
		`CREATE TABLE IF NOT EXISTS offerup_price_snapshots (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			query TEXT,
			listing_id TEXT,
			price REAL,
			captured_at TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_offerup_snap ON offerup_price_snapshots(query, listing_id, captured_at)`,
		`CREATE INDEX IF NOT EXISTS idx_offerup_owner ON offerup_listings(owner_id)`,
		`CREATE TABLE IF NOT EXISTS offerup_sellers (
			id TEXT PRIMARY KEY,
			name TEXT,
			date_joined TEXT,
			primary_badge TEXT,
			is_business INTEGER,
			is_dealer INTEGER,
			is_premium INTEGER,
			is_truyou INTEGER,
			last_seen TEXT
		)`,
	}
	for _, q := range stmts {
		if _, err := s.db.Exec(q); err != nil {
			return fmt.Errorf("offerup store migration: %w", err)
		}
	}
	return nil
}

// StoredListing is a listing row read back from the store.
type StoredListing struct {
	ListingID    string  `json:"listingId"`
	Query        string  `json:"query,omitempty"`
	Title        string  `json:"title"`
	Price        float64 `json:"price"`
	PriceText    string  `json:"priceText"`
	LocationName string  `json:"locationName"`
	Condition    string  `json:"conditionText,omitempty"`
	IsFirmPrice  bool    `json:"isFirmPrice"`
	OwnerID      string  `json:"ownerId,omitempty"`
	URL          string  `json:"url"`
	FirstSeen    string  `json:"firstSeen,omitempty"`
	LastSeen     string  `json:"lastSeen,omitempty"`
}

// Drop is one detected price reduction for a listing.
type Drop struct {
	ListingID    string  `json:"listingId"`
	Title        string  `json:"title"`
	PriorPrice   float64 `json:"priorPrice"`
	CurrentPrice float64 `json:"currentPrice"`
	DropAmount   float64 `json:"dropAmount"`
	DropPercent  float64 `json:"dropPercent"`
	URL          string  `json:"url"`
}

func nowRFC3339() string { return time.Now().UTC().Format(time.RFC3339) }

// RecordSearch upserts the listings observed for a keyword query, preserving
// each listing's original first_seen, refreshing last_seen, and appending a
// price snapshot whenever the price is new or changed. Returns how many
// listings were seen for the first time.
func (s *Store) RecordSearch(query string, listings []Listing) (int, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	now := nowRFC3339()
	newCount := 0
	for _, l := range listings {
		if l.ListingID == "" {
			continue
		}
		var existing sql.NullFloat64
		row := tx.QueryRow(`SELECT last_price FROM offerup_listings WHERE query=? AND listing_id=?`, query, l.ListingID)
		isNew := row.Scan(&existing) == sql.ErrNoRows
		if isNew {
			newCount++
		}
		if _, err := tx.Exec(`
			INSERT INTO offerup_listings
				(query, listing_id, title, price, price_text, location, condition, is_firm, vehicle_miles, flags, owner_id, url, image_url, first_seen, last_seen, last_price)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
			ON CONFLICT(query, listing_id) DO UPDATE SET
				title=excluded.title, price=excluded.price, price_text=excluded.price_text,
				location=excluded.location, condition=excluded.condition, is_firm=excluded.is_firm,
				vehicle_miles=excluded.vehicle_miles, flags=excluded.flags, url=excluded.url,
				image_url=excluded.image_url, last_seen=excluded.last_seen, last_price=excluded.last_price`,
			query, l.ListingID, l.Title, l.Price, l.PriceText, l.LocationName, l.ConditionText,
			boolToInt(l.IsFirmPrice), l.VehicleMiles, strings.Join(l.Flags, ","), "", l.URL, l.ImageURL,
			now, now, l.Price,
		); err != nil {
			return 0, err
		}
		if isNew || !existing.Valid || existing.Float64 != l.Price {
			if _, err := tx.Exec(`INSERT INTO offerup_price_snapshots (query, listing_id, price, captured_at) VALUES (?,?,?,?)`,
				query, l.ListingID, l.Price, now); err != nil {
				return 0, err
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return newCount, nil
}

// RecordDetail stores one item-detail listing (with owner linkage) so
// seller-scan can find a seller's locally-known inventory.
func (s *Store) RecordDetail(d *ListingDetail) error {
	if d == nil || d.ListingID == "" {
		return nil
	}
	now := nowRFC3339()
	_, err := s.db.Exec(`
		INSERT INTO offerup_listings
			(query, listing_id, title, price, price_text, location, condition, is_firm, vehicle_miles, flags, owner_id, url, image_url, first_seen, last_seen, last_price)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(query, listing_id) DO UPDATE SET
			title=excluded.title, price=excluded.price, price_text=excluded.price_text,
			location=excluded.location, condition=excluded.condition, is_firm=excluded.is_firm,
			owner_id=excluded.owner_id, url=excluded.url, last_seen=excluded.last_seen, last_price=excluded.last_price`,
		itemQuery, d.ListingID, d.Title, d.Price, d.PriceText, d.LocationName, d.ConditionText,
		boolToInt(d.IsFirmPrice), d.VehicleMiles, strings.Join(d.Flags, ","), d.OwnerID, d.URL, d.ImageURL,
		now, now, d.Price,
	)
	return err
}

// RecordSeller upserts a seller's reputation snapshot.
func (s *Store) RecordSeller(sel *Seller) error {
	if sel == nil || sel.ID == "" {
		return nil
	}
	_, err := s.db.Exec(`
		INSERT INTO offerup_sellers (id, name, date_joined, primary_badge, is_business, is_dealer, is_premium, is_truyou, last_seen)
		VALUES (?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET
			name=excluded.name, date_joined=excluded.date_joined, primary_badge=excluded.primary_badge,
			is_business=excluded.is_business, is_dealer=excluded.is_dealer, is_premium=excluded.is_premium,
			is_truyou=excluded.is_truyou, last_seen=excluded.last_seen`,
		sel.ID, sel.Name, sel.DateJoined, sel.PrimaryBadge,
		boolToInt(sel.IsBusinessAccount), boolToInt(sel.IsAutosDealer), boolToInt(sel.IsPremium), boolToInt(sel.IsTruyouVerified),
		nowRFC3339(),
	)
	return err
}

// Listings returns the current stored listings for a keyword query.
func (s *Store) Listings(query string) ([]StoredListing, error) {
	return s.queryListings(`SELECT listing_id, query, title, last_price, price_text, location, condition, is_firm, owner_id, url, first_seen, last_seen
		FROM offerup_listings WHERE query=? ORDER BY last_price ASC`, query)
}

// NewSince returns listings for a query whose first_seen is at or after cutoff.
func (s *Store) NewSince(query string, cutoff time.Time) ([]StoredListing, error) {
	return s.queryListings(`SELECT listing_id, query, title, last_price, price_text, location, condition, is_firm, owner_id, url, first_seen, last_seen
		FROM offerup_listings WHERE query=? AND first_seen >= ? ORDER BY first_seen DESC`,
		query, cutoff.UTC().Format(time.RFC3339))
}

// SellerInventory returns locally-known listings owned by a seller (populated by
// prior `listings get` calls).
func (s *Store) SellerInventory(sellerID string) ([]StoredListing, error) {
	return s.queryListings(`SELECT listing_id, query, title, last_price, price_text, location, condition, is_firm, owner_id, url, first_seen, last_seen
		FROM offerup_listings WHERE owner_id=? ORDER BY last_price ASC`, sellerID)
}

func (s *Store) queryListings(q string, args ...any) ([]StoredListing, error) {
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []StoredListing
	for rows.Next() {
		var (
			l        StoredListing
			price    sql.NullFloat64
			priceTxt sql.NullString
			loc      sql.NullString
			cond     sql.NullString
			isFirm   sql.NullInt64
			owner    sql.NullString
			url      sql.NullString
			first    sql.NullString
			last     sql.NullString
			title    sql.NullString
		)
		if err := rows.Scan(&l.ListingID, &l.Query, &title, &price, &priceTxt, &loc, &cond, &isFirm, &owner, &url, &first, &last); err != nil {
			continue
		}
		l.Title = title.String
		l.Price = price.Float64
		l.PriceText = priceTxt.String
		l.LocationName = loc.String
		l.Condition = cond.String
		l.IsFirmPrice = isFirm.Int64 != 0
		l.OwnerID = owner.String
		l.URL = url.String
		l.FirstSeen = first.String
		l.LastSeen = last.String
		out = append(out, l)
	}
	return out, rows.Err()
}

// Seller returns a stored seller reputation record, or nil if unknown.
func (s *Store) Seller(sellerID string) (*Seller, error) {
	row := s.db.QueryRow(`SELECT id, name, date_joined, primary_badge, is_business, is_dealer, is_premium, is_truyou
		FROM offerup_sellers WHERE id=?`, sellerID)
	var (
		sel                               Seller
		name, joined, badge               sql.NullString
		isBiz, isDealer, isPrem, isTruyou sql.NullInt64
	)
	if err := row.Scan(&sel.ID, &name, &joined, &badge, &isBiz, &isDealer, &isPrem, &isTruyou); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	sel.Name = name.String
	sel.DateJoined = joined.String
	sel.PrimaryBadge = badge.String
	sel.IsBusinessAccount = isBiz.Int64 != 0
	sel.IsAutosDealer = isDealer.Int64 != 0
	sel.IsPremium = isPrem.Int64 != 0
	sel.IsTruyouVerified = isTruyou.Int64 != 0
	return &sel, nil
}

// Drops returns listings for a query whose current price is below the highest
// price recorded for that listing at or after `since`. Empty until at least two
// observations exist with a price reduction between them.
func (s *Store) Drops(query string, since time.Time) ([]Drop, error) {
	rows, err := s.db.Query(`
		SELECT l.listing_id, l.title, l.last_price, l.url,
			(SELECT MAX(sn.price) FROM offerup_price_snapshots sn
			 WHERE sn.query=l.query AND sn.listing_id=l.listing_id AND sn.captured_at >= ?) AS prior_max
		FROM offerup_listings l WHERE l.query=?`,
		since.UTC().Format(time.RFC3339), query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var drops []Drop
	for rows.Next() {
		var (
			id, url, title sql.NullString
			cur, priorMax  sql.NullFloat64
		)
		if err := rows.Scan(&id, &title, &cur, &url, &priorMax); err != nil {
			continue
		}
		if !priorMax.Valid || !cur.Valid || priorMax.Float64 <= cur.Float64 {
			continue
		}
		amt := priorMax.Float64 - cur.Float64
		drops = append(drops, Drop{
			ListingID:    id.String,
			Title:        title.String,
			PriorPrice:   priorMax.Float64,
			CurrentPrice: cur.Float64,
			DropAmount:   amt,
			DropPercent:  round1(amt / priorMax.Float64 * 100),
			URL:          url.String,
		})
	}
	sort.Slice(drops, func(i, j int) bool { return drops[i].DropPercent > drops[j].DropPercent })
	return drops, rows.Err()
}

// ListingsToStored adapts freshly-fetched search listings to the StoredListing
// shape so stats and views can run over a live result set without a round-trip
// through the database.
func ListingsToStored(ls []Listing) []StoredListing {
	out := make([]StoredListing, 0, len(ls))
	for _, l := range ls {
		out = append(out, StoredListing{
			ListingID:    l.ListingID,
			Title:        l.Title,
			Price:        l.Price,
			PriceText:    l.PriceText,
			LocationName: l.LocationName,
			Condition:    l.ConditionText,
			IsFirmPrice:  l.IsFirmPrice,
			URL:          l.URL,
		})
	}
	return out
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func round1(f float64) float64 {
	return float64(int64(f*10+0.5)) / 10
}
