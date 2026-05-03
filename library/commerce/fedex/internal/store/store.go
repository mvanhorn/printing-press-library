// Local SQLite ledger for fedex-pp-cli. Schema is purpose-built for the
// SMB shipping workflow: shipments are write-only history (FedEx has no
// list-shipments endpoint), tracking_events are append-only per tracking
// number, address_validations is a TTL/cost cache, addresses is the
// hand-curated address book.
package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db   *sql.DB
	path string
}

const Schema = `
CREATE TABLE IF NOT EXISTS shipments (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  tracking_number TEXT NOT NULL UNIQUE,
  master_tracking_number TEXT,
  account TEXT,
  service_type TEXT NOT NULL,
  packaging_type TEXT,
  shipper_name TEXT,
  shipper_postal TEXT,
  shipper_country TEXT,
  recipient_name TEXT,
  recipient_address TEXT,
  recipient_city TEXT,
  recipient_state TEXT,
  recipient_postal TEXT,
  recipient_country TEXT,
  weight_value REAL,
  weight_units TEXT,
  reference TEXT,
  net_charge_amount REAL,
  net_charge_currency TEXT,
  list_charge_amount REAL,
  label_path TEXT,
  raw_response TEXT,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_shipments_created ON shipments(created_at);
CREATE INDEX IF NOT EXISTS idx_shipments_service ON shipments(service_type);
CREATE INDEX IF NOT EXISTS idx_shipments_account ON shipments(account);
CREATE INDEX IF NOT EXISTS idx_shipments_recipient ON shipments(recipient_name, recipient_postal);

CREATE VIRTUAL TABLE IF NOT EXISTS shipments_fts USING fts5(
  tracking_number, recipient_name, recipient_address, recipient_city,
  recipient_state, recipient_postal, reference, service_type,
  content='shipments', content_rowid='id', tokenize='porter unicode61'
);

CREATE TRIGGER IF NOT EXISTS shipments_ai AFTER INSERT ON shipments BEGIN
  INSERT INTO shipments_fts(rowid, tracking_number, recipient_name, recipient_address, recipient_city, recipient_state, recipient_postal, reference, service_type)
  VALUES (new.id, coalesce(new.tracking_number,''), coalesce(new.recipient_name,''), coalesce(new.recipient_address,''), coalesce(new.recipient_city,''), coalesce(new.recipient_state,''), coalesce(new.recipient_postal,''), coalesce(new.reference,''), coalesce(new.service_type,''));
END;

CREATE TABLE IF NOT EXISTS rate_quotes (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  origin_postal TEXT NOT NULL,
  origin_country TEXT NOT NULL DEFAULT 'US',
  dest_postal TEXT NOT NULL,
  dest_country TEXT NOT NULL DEFAULT 'US',
  weight_value REAL NOT NULL,
  weight_units TEXT NOT NULL DEFAULT 'LB',
  service_type TEXT NOT NULL,
  packaging_type TEXT,
  list_amount REAL,
  net_amount REAL,
  currency TEXT,
  transit_days INTEGER,
  delivery_day_of_week TEXT,
  selected INTEGER NOT NULL DEFAULT 0,
  raw_response TEXT,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_rates_lane ON rate_quotes(origin_postal, dest_postal);
CREATE INDEX IF NOT EXISTS idx_rates_created ON rate_quotes(created_at);
CREATE INDEX IF NOT EXISTS idx_rates_service ON rate_quotes(service_type);

CREATE TABLE IF NOT EXISTS tracking_events (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  tracking_number TEXT NOT NULL,
  carrier_code TEXT,
  event_timestamp TIMESTAMP,
  event_type TEXT,
  event_description TEXT,
  status_code TEXT,
  status_locale TEXT,
  scan_location_city TEXT,
  scan_location_state TEXT,
  scan_location_country TEXT,
  delivery_attempts INTEGER,
  raw TEXT,
  fetched_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE(tracking_number, event_timestamp, event_type, scan_location_city)
);
CREATE INDEX IF NOT EXISTS idx_events_tracking ON tracking_events(tracking_number, event_timestamp DESC);

CREATE TABLE IF NOT EXISTS address_validations (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  cache_key TEXT NOT NULL UNIQUE,
  street TEXT NOT NULL,
  city TEXT NOT NULL,
  state TEXT,
  postal TEXT NOT NULL,
  country TEXT NOT NULL DEFAULT 'US',
  classification TEXT,
  resolved_street TEXT,
  resolved_city TEXT,
  resolved_state TEXT,
  resolved_postal TEXT,
  raw_response TEXT,
  validated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS addresses (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL UNIQUE,
  contact_name TEXT,
  company TEXT,
  phone TEXT,
  email TEXT,
  street TEXT NOT NULL,
  street2 TEXT,
  city TEXT NOT NULL,
  state TEXT,
  postal TEXT NOT NULL,
  country TEXT NOT NULL DEFAULT 'US',
  is_residential INTEGER NOT NULL DEFAULT 0,
  notes TEXT,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_addresses_postal ON addresses(postal);

CREATE TABLE IF NOT EXISTS poll_state (
  scope TEXT PRIMARY KEY,
  last_polled_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  watermark TEXT
);
`

// DefaultPath returns the user-level SQLite path for the FedEx ledger.
func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "fedex.db"
	}
	return filepath.Join(home, ".local", "share", "fedex-pp-cli", "fedex.db")
}

// Open opens (creating if needed) the local SQLite ledger and applies the
// schema. Caller is responsible for Close.
func Open(path string) (*Store, error) {
	if path == "" {
		path = DefaultPath()
	}
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("creating store dir: %w", err)
		}
	}
	dsn := path + "?_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", path, err)
	}
	db.SetMaxOpenConns(1) // SQLite + WAL: single writer pattern keeps things simple
	if _, err := db.Exec(Schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("applying schema: %w", err)
	}
	return &Store{db: db, path: path}, nil
}

func (s *Store) DB() *sql.DB  { return s.db }
func (s *Store) Path() string { return s.path }
func (s *Store) Close() error { return s.db.Close() }

// Shipment captures the SMB-relevant fields we persist after a successful
// ship create. The raw FedEx response is preserved for forensic JSON.
type Shipment struct {
	TrackingNumber       string
	MasterTrackingNumber string
	Account              string
	ServiceType          string
	PackagingType        string
	ShipperName          string
	ShipperPostal        string
	ShipperCountry       string
	RecipientName        string
	RecipientAddress     string
	RecipientCity        string
	RecipientState       string
	RecipientPostal      string
	RecipientCountry     string
	WeightValue          float64
	WeightUnits          string
	Reference            string
	NetChargeAmount      float64
	NetChargeCurrency    string
	ListChargeAmount     float64
	LabelPath            string
	RawResponse          string
	CreatedAt            time.Time
}

func (s *Store) InsertShipment(ctx context.Context, sp Shipment) (int64, error) {
	res, err := s.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO shipments
		(tracking_number, master_tracking_number, account, service_type, packaging_type,
		 shipper_name, shipper_postal, shipper_country,
		 recipient_name, recipient_address, recipient_city, recipient_state, recipient_postal, recipient_country,
		 weight_value, weight_units, reference,
		 net_charge_amount, net_charge_currency, list_charge_amount,
		 label_path, raw_response)
		VALUES (?, ?, ?, ?, ?,  ?, ?, ?,  ?, ?, ?, ?, ?, ?,  ?, ?, ?,  ?, ?, ?,  ?, ?)
	`,
		sp.TrackingNumber, sp.MasterTrackingNumber, sp.Account, sp.ServiceType, sp.PackagingType,
		sp.ShipperName, sp.ShipperPostal, sp.ShipperCountry,
		sp.RecipientName, sp.RecipientAddress, sp.RecipientCity, sp.RecipientState, sp.RecipientPostal, sp.RecipientCountry,
		sp.WeightValue, sp.WeightUnits, sp.Reference,
		sp.NetChargeAmount, sp.NetChargeCurrency, sp.ListChargeAmount,
		sp.LabelPath, sp.RawResponse)
	if err != nil {
		return 0, fmt.Errorf("insert shipment: %w", err)
	}
	id, _ := res.LastInsertId()
	return id, nil
}

// RateQuote is one row in a rate-shop result. `Selected` is set when the
// quote is the one used to actually create a shipment (ship bulk path).
type RateQuote struct {
	OriginPostal      string
	OriginCountry     string
	DestPostal        string
	DestCountry       string
	WeightValue       float64
	WeightUnits       string
	ServiceType       string
	PackagingType     string
	ListAmount        float64
	NetAmount         float64
	Currency          string
	TransitDays       int
	DeliveryDayOfWeek string
	Selected          bool
	RawResponse       string
}

func (s *Store) InsertRateQuote(ctx context.Context, q RateQuote) error {
	sel := 0
	if q.Selected {
		sel = 1
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO rate_quotes
		(origin_postal, origin_country, dest_postal, dest_country, weight_value, weight_units,
		 service_type, packaging_type, list_amount, net_amount, currency, transit_days, delivery_day_of_week,
		 selected, raw_response)
		VALUES (?, ?, ?, ?, ?, ?,  ?, ?, ?, ?, ?, ?, ?,  ?, ?)
	`,
		q.OriginPostal, q.OriginCountry, q.DestPostal, q.DestCountry, q.WeightValue, q.WeightUnits,
		q.ServiceType, q.PackagingType, q.ListAmount, q.NetAmount, q.Currency, q.TransitDays, q.DeliveryDayOfWeek,
		sel, q.RawResponse)
	return err
}

// TrackingEvent is one row in the per-shipment event timeline. Uniqueness
// (tracking_number + event_timestamp + event_type + scan_location_city)
// dedupes the same event when the API returns it across multiple polls.
type TrackingEvent struct {
	TrackingNumber      string
	CarrierCode         string
	EventTimestamp      time.Time
	EventType           string
	EventDescription    string
	StatusCode          string
	StatusLocale        string
	ScanLocationCity    string
	ScanLocationState   string
	ScanLocationCountry string
	DeliveryAttempts    int
	Raw                 string
}

// InsertTrackingEvent returns true when the row was actually inserted (new
// event); false when the unique constraint deduped it (already seen). Used
// by track diff to surface only new milestones.
func (s *Store) InsertTrackingEvent(ctx context.Context, ev TrackingEvent) (bool, error) {
	res, err := s.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO tracking_events
		(tracking_number, carrier_code, event_timestamp, event_type, event_description,
		 status_code, status_locale, scan_location_city, scan_location_state, scan_location_country,
		 delivery_attempts, raw)
		VALUES (?, ?, ?, ?, ?,  ?, ?, ?, ?, ?,  ?, ?)
	`,
		ev.TrackingNumber, ev.CarrierCode, ev.EventTimestamp, ev.EventType, ev.EventDescription,
		ev.StatusCode, ev.StatusLocale, ev.ScanLocationCity, ev.ScanLocationState, ev.ScanLocationCountry,
		ev.DeliveryAttempts, ev.Raw)
	if err != nil {
		return false, fmt.Errorf("insert tracking event: %w", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// Address is one row in the local address book. `Name` is the
// agent/operator-facing handle (e.g., "acme") used by `ship --to-saved`.
type Address struct {
	Name          string
	ContactName   string
	Company       string
	Phone         string
	Email         string
	Street        string
	Street2       string
	City          string
	State         string
	Postal        string
	Country       string
	IsResidential bool
	Notes         string
}

func (s *Store) UpsertAddress(ctx context.Context, a Address) error {
	if strings.TrimSpace(a.Name) == "" {
		return fmt.Errorf("address name is required")
	}
	if a.Country == "" {
		a.Country = "US"
	}
	res := 0
	if a.IsResidential {
		res = 1
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO addresses (name, contact_name, company, phone, email, street, street2, city, state, postal, country, is_residential, notes, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(name) DO UPDATE SET
		  contact_name=excluded.contact_name,
		  company=excluded.company,
		  phone=excluded.phone,
		  email=excluded.email,
		  street=excluded.street,
		  street2=excluded.street2,
		  city=excluded.city,
		  state=excluded.state,
		  postal=excluded.postal,
		  country=excluded.country,
		  is_residential=excluded.is_residential,
		  notes=excluded.notes,
		  updated_at=CURRENT_TIMESTAMP
	`, a.Name, a.ContactName, a.Company, a.Phone, a.Email, a.Street, a.Street2, a.City, a.State, a.Postal, a.Country, res, a.Notes)
	if err != nil {
		return fmt.Errorf("upsert address %q: %w", a.Name, err)
	}
	return nil
}

func (s *Store) GetAddress(ctx context.Context, name string) (*Address, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT name, contact_name, company, phone, email, street, street2, city, state, postal, country, is_residential, notes
		FROM addresses WHERE name = ?
	`, name)
	var a Address
	var res int
	if err := row.Scan(&a.Name, &a.ContactName, &a.Company, &a.Phone, &a.Email, &a.Street, &a.Street2, &a.City, &a.State, &a.Postal, &a.Country, &res, &a.Notes); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	a.IsResidential = res == 1
	return &a, nil
}

func (s *Store) ListAddresses(ctx context.Context) ([]Address, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT name, contact_name, company, phone, email, street, street2, city, state, postal, country, is_residential, notes
		FROM addresses ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Address
	for rows.Next() {
		var a Address
		var res int
		if err := rows.Scan(&a.Name, &a.ContactName, &a.Company, &a.Phone, &a.Email, &a.Street, &a.Street2, &a.City, &a.State, &a.Postal, &a.Country, &res, &a.Notes); err != nil {
			return nil, err
		}
		a.IsResidential = res == 1
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) DeleteAddress(ctx context.Context, name string) (bool, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM addresses WHERE name = ?`, name)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// AddressValidationCache stores SHA-256-keyed prior address-validation
// results so repeat lookups don't re-bill the API. Caller computes the
// stable hash off the normalized inputs.
type AddressValidationCache struct {
	CacheKey       string
	Street         string
	City           string
	State          string
	Postal         string
	Country        string
	Classification string
	ResolvedStreet string
	ResolvedCity   string
	ResolvedState  string
	ResolvedPostal string
	RawResponse    string
}

func (s *Store) GetAddressValidationByKey(ctx context.Context, key string) (*AddressValidationCache, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT cache_key, street, city, state, postal, country, classification,
		       resolved_street, resolved_city, resolved_state, resolved_postal, raw_response
		FROM address_validations WHERE cache_key = ?
	`, key)
	var av AddressValidationCache
	if err := row.Scan(&av.CacheKey, &av.Street, &av.City, &av.State, &av.Postal, &av.Country,
		&av.Classification, &av.ResolvedStreet, &av.ResolvedCity, &av.ResolvedState, &av.ResolvedPostal, &av.RawResponse); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &av, nil
}

func (s *Store) InsertAddressValidation(ctx context.Context, av AddressValidationCache) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT OR REPLACE INTO address_validations
		(cache_key, street, city, state, postal, country, classification,
		 resolved_street, resolved_city, resolved_state, resolved_postal, raw_response)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, av.CacheKey, av.Street, av.City, av.State, av.Postal, av.Country, av.Classification,
		av.ResolvedStreet, av.ResolvedCity, av.ResolvedState, av.ResolvedPostal, av.RawResponse)
	return err
}

// LastPolled returns the last poll timestamp for a logical scope (e.g.
// "track:794633071234"), or zero time if never polled.
func (s *Store) LastPolled(ctx context.Context, scope string) (time.Time, error) {
	var t time.Time
	err := s.db.QueryRowContext(ctx, `SELECT last_polled_at FROM poll_state WHERE scope = ?`, scope).Scan(&t)
	if err == sql.ErrNoRows {
		return time.Time{}, nil
	}
	return t, err
}

func (s *Store) MarkPolled(ctx context.Context, scope string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO poll_state (scope, last_polled_at) VALUES (?, CURRENT_TIMESTAMP)
		ON CONFLICT(scope) DO UPDATE SET last_polled_at = CURRENT_TIMESTAMP
	`, scope)
	return err
}
