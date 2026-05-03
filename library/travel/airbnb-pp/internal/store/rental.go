package store

import (
	"database/sql"
	"time"
)

type WatchlistItem struct {
	ID            int64   `json:"id,omitempty"`
	ListingURL    string  `json:"listing_url,omitempty"`
	ListingID     string  `json:"listing_id,omitempty"`
	Platform      string  `json:"platform,omitempty"`
	MaxPrice      float64 `json:"max_price,omitempty"`
	Checkin       string  `json:"checkin,omitempty"`
	Checkout      string  `json:"checkout,omitempty"`
	AddedAt       int64   `json:"added_at,omitempty"`
	LastCheckedAt int64   `json:"last_checked_at,omitempty"`
	LastPrice     float64 `json:"last_price,omitempty"`
	LastDropAt    int64   `json:"last_drop_at,omitempty"`
}

type PriceSnapshot struct {
	ListingID   string  `json:"listing_id,omitempty"`
	Platform    string  `json:"platform,omitempty"`
	Checkin     string  `json:"checkin,omitempty"`
	Checkout    string  `json:"checkout,omitempty"`
	SnapshotAt  int64   `json:"snapshot_at,omitempty"`
	TotalPrice  float64 `json:"total_price,omitempty"`
	CleaningFee float64 `json:"cleaning_fee,omitempty"`
	ServiceFee  float64 `json:"service_fee,omitempty"`
	Tax         float64 `json:"tax,omitempty"`
}

type HostRecord struct {
	Name         string `json:"name,omitempty"`
	Brand        string `json:"brand,omitempty"`
	Type         string `json:"type,omitempty"`
	DirectURL    string `json:"direct_url,omitempty"`
	LastSeenAt   int64  `json:"last_seen_at,omitempty"`
	ListingCount int    `json:"listing_count,omitempty"`
}

func (s *Store) UpsertWatchlistItem(item WatchlistItem) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	now := time.Now().Unix()
	if item.AddedAt == 0 {
		item.AddedAt = now
	}
	_, err := s.db.Exec(`INSERT INTO watchlist
		(listing_url, listing_id, platform, max_price, checkin, checkout, added_at, last_checked_at, last_price, last_drop_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(listing_url) DO UPDATE SET listing_id=excluded.listing_id, platform=excluded.platform,
		max_price=excluded.max_price, checkin=excluded.checkin, checkout=excluded.checkout`,
		item.ListingURL, item.ListingID, item.Platform, item.MaxPrice, item.Checkin, item.Checkout,
		item.AddedAt, item.LastCheckedAt, item.LastPrice, item.LastDropAt)
	return err
}

func (s *Store) ListWatchlist(since int64) ([]WatchlistItem, error) {
	query := `SELECT id, listing_url, listing_id, platform, max_price, checkin, checkout, added_at, last_checked_at, last_price, last_drop_at FROM watchlist`
	args := []any{}
	if since > 0 {
		query += ` WHERE added_at >= ? OR last_checked_at >= ?`
		args = append(args, since, since)
	}
	query += ` ORDER BY added_at DESC`
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WatchlistItem
	for rows.Next() {
		var item WatchlistItem
		if err := rows.Scan(&item.ID, &item.ListingURL, &item.ListingID, &item.Platform, &item.MaxPrice, &item.Checkin, &item.Checkout, &item.AddedAt, &item.LastCheckedAt, &item.LastPrice, &item.LastDropAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) UpdateWatchPrice(id int64, price float64, hit bool) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	now := time.Now().Unix()
	dropAt := sql.NullInt64{}
	if hit {
		dropAt = sql.NullInt64{Int64: now, Valid: true}
	}
	_, err := s.db.Exec(`UPDATE watchlist SET last_checked_at=?, last_price=?, last_drop_at=COALESCE(?, last_drop_at) WHERE id=?`, now, price, dropAt, id)
	return err
}

func (s *Store) InsertPriceSnapshot(p PriceSnapshot) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if p.SnapshotAt == 0 {
		p.SnapshotAt = time.Now().Unix()
	}
	_, err := s.db.Exec(`INSERT OR REPLACE INTO price_snapshots
		(listing_id, platform, checkin, checkout, snapshot_at, total_price, cleaning_fee, service_fee, tax)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ListingID, p.Platform, p.Checkin, p.Checkout, p.SnapshotAt, p.TotalPrice, p.CleaningFee, p.ServiceFee, p.Tax)
	return err
}

func (s *Store) ListPriceSnapshotsSince(since int64) ([]PriceSnapshot, error) {
	rows, err := s.db.Query(`SELECT listing_id, platform, checkin, checkout, snapshot_at, total_price, cleaning_fee, service_fee, tax
		FROM price_snapshots WHERE snapshot_at >= ? ORDER BY listing_id, snapshot_at`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PriceSnapshot
	for rows.Next() {
		var p PriceSnapshot
		if err := rows.Scan(&p.ListingID, &p.Platform, &p.Checkin, &p.Checkout, &p.SnapshotAt, &p.TotalPrice, &p.CleaningFee, &p.ServiceFee, &p.Tax); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) UpsertHostRecord(h HostRecord) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if h.LastSeenAt == 0 {
		h.LastSeenAt = time.Now().Unix()
	}
	_, err := s.db.Exec(`INSERT INTO hosts (name, brand, type, direct_url, last_seen_at, listing_count)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET brand=excluded.brand, type=excluded.type, direct_url=excluded.direct_url,
		last_seen_at=excluded.last_seen_at, listing_count=excluded.listing_count`,
		h.Name, h.Brand, h.Type, h.DirectURL, h.LastSeenAt, h.ListingCount)
	return err
}

func (s *Store) HostPortfolio(name string) ([]map[string]any, error) {
	rows, err := s.db.Query(`SELECT data FROM resources WHERE lower(data) LIKE '%' || lower(?) || '%' LIMIT 200`, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"raw": raw})
	}
	return out, rows.Err()
}
