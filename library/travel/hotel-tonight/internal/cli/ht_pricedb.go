// HotelTonight price-history store: a hand-authored time-series layer on top
// of the generated SQLite store. The app deliberately erases prior prices, so
// every deal fetch can append a snapshot row here, turning the ephemeral feed
// into a queryable, diff-able history. Survives `generate --force`.
package cli

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/hotel-tonight/internal/store"
)

const htSnapshotSchema = `
CREATE TABLE IF NOT EXISTS ht_snapshots (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	observed_at TEXT NOT NULL,
	check_in TEXT,
	hotel_id INTEGER,
	hotel_name TEXT NOT NULL,
	neighborhood TEXT,
	market_id INTEGER,
	market_name TEXT,
	latitude REAL,
	longitude REAL,
	deal_type TEXT,
	category TEXT,
	customer_price REAL,
	strikethrough_price REAL,
	pct_off INTEGER,
	num_remaining INTEGER,
	sold_out INTEGER
);
CREATE INDEX IF NOT EXISTS idx_ht_snap_hotel ON ht_snapshots(hotel_name, check_in, observed_at);
CREATE INDEX IF NOT EXISTS idx_ht_snap_market ON ht_snapshots(market_id, observed_at);
`

// snapshotInsertSQL is shared by recordSnapshots and recordDailyDrop so the
// column list and placeholders can't drift between the two writers. Callers
// must pass args in this column order.
const snapshotInsertSQL = `
	INSERT INTO ht_snapshots
	  (observed_at, check_in, hotel_id, hotel_name, neighborhood, market_id, market_name,
	   latitude, longitude, deal_type, category, customer_price, strikethrough_price,
	   pct_off, num_remaining, sold_out)
	VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`

// openPriceStore opens the shared SQLite store and ensures the snapshot table
// exists. Callers must Close the returned store.
func openPriceStore(ctx context.Context) (*store.Store, error) {
	s, err := store.OpenWithContext(ctx, defaultDBPath("hotel-tonight-pp-cli"))
	if err != nil {
		return nil, fmt.Errorf("opening price store: %w", err)
	}
	if _, err := s.DB().ExecContext(ctx, htSnapshotSchema); err != nil {
		s.Close()
		return nil, fmt.Errorf("creating snapshot table: %w", err)
	}
	return s, nil
}

// recordSnapshots appends one row per deal observed in inv. observedAt is a
// single timestamp shared by the whole batch so a run's snapshots group
// cleanly. Returns the number of rows written.
func recordSnapshots(ctx context.Context, db *sql.DB, inv *htInventory, observedAt time.Time) (int, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin snapshot tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.PrepareContext(ctx, snapshotInsertSQL)
	if err != nil {
		return 0, fmt.Errorf("prepare snapshot insert: %w", err)
	}
	defer stmt.Close()

	ts := observedAt.UTC().Format(time.RFC3339)
	n := 0
	for _, r := range inv.Rooms {
		d := r.toDeal()
		soldOut := 0
		if d.SoldOut {
			soldOut = 1
		}
		if _, err := stmt.ExecContext(ctx, ts, inv.CurrentDay, d.HotelID, d.HotelName,
			d.Neighborhood, numI(inv.PrimaryMarket.ID), inv.PrimaryMarket.CityName,
			d.Latitude, d.Longitude, d.DealType, d.Category, d.Price, d.Was,
			d.PctOff, d.NumRemaining, soldOut); err != nil {
			return 0, fmt.Errorf("insert snapshot: %w", err)
		}
		n++
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit snapshots: %w", err)
	}
	return n, nil
}

// htSnapshotRow is one observed price point for a hotel.
type htSnapshotRow struct {
	ObservedAt   string  `json:"observed_at"`
	CheckIn      string  `json:"check_in"`
	DealType     string  `json:"deal_type"`
	Category     string  `json:"category"`
	Price        float64 `json:"price"`
	Was          float64 `json:"was,omitempty"`
	PctOff       int     `json:"pct_off"`
	NumRemaining int64   `json:"num_remaining"`
	SoldOut      bool    `json:"sold_out"`
}

// hotelHistory returns observed snapshots for a hotel whose name matches
// (case-insensitive substring) within the last `days`, newest first. A days
// value <= 0 means no time bound.
func hotelHistory(ctx context.Context, db *sql.DB, name string, days int) ([]htSnapshotRow, error) {
	query := `
		SELECT observed_at, check_in, deal_type, category, customer_price,
		       strikethrough_price, pct_off, num_remaining, sold_out
		FROM ht_snapshots
		WHERE hotel_name LIKE ? COLLATE NOCASE`
	args := []any{"%" + name + "%"}
	if days > 0 {
		query += ` AND observed_at >= ?`
		args = append(args, time.Now().UTC().AddDate(0, 0, -days).Format(time.RFC3339))
	}
	query += ` ORDER BY observed_at DESC`

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query history: %w", err)
	}
	defer rows.Close()

	var out []htSnapshotRow
	for rows.Next() {
		var (
			observedAt, checkIn, dealType, category sql.NullString
			price, strike                           sql.NullFloat64
			pctOffV, numRemaining                   sql.NullInt64
			soldOut                                 sql.NullInt64
		)
		if err := rows.Scan(&observedAt, &checkIn, &dealType, &category, &price,
			&strike, &pctOffV, &numRemaining, &soldOut); err != nil {
			return nil, fmt.Errorf("scan history row: %w", err)
		}
		out = append(out, htSnapshotRow{
			ObservedAt:   observedAt.String,
			CheckIn:      checkIn.String,
			DealType:     dealType.String,
			Category:     category.String,
			Price:        price.Float64,
			Was:          strike.Float64,
			PctOff:       int(pctOffV.Int64),
			NumRemaining: numRemaining.Int64,
			SoldOut:      soldOut.Int64 == 1,
		})
	}
	return out, rows.Err()
}

// htPriceStats summarizes a hotel's observed price distribution.
type htPriceStats struct {
	HotelName    string  `json:"hotel_name"`
	Observations int     `json:"observations"`
	Low          float64 `json:"low"`
	Median       float64 `json:"median"`
	High         float64 `json:"high"`
	P25          float64 `json:"p25"`
	P75          float64 `json:"p75"`
}

// hotelPriceStats computes low/median/high and the 25th/75th percentile of a
// hotel's recorded customer prices. Returns ok=false when no priced
// observations exist.
func hotelPriceStats(ctx context.Context, db *sql.DB, name string) (htPriceStats, bool, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT customer_price, hotel_name FROM ht_snapshots
		WHERE hotel_name LIKE ? COLLATE NOCASE AND customer_price > 0
		ORDER BY customer_price ASC`, "%"+name+"%")
	if err != nil {
		return htPriceStats{}, false, fmt.Errorf("query stats: %w", err)
	}
	defer rows.Close()

	var prices []float64
	resolvedName := name
	for rows.Next() {
		var price sql.NullFloat64
		var hn sql.NullString
		if err := rows.Scan(&price, &hn); err != nil {
			return htPriceStats{}, false, fmt.Errorf("scan stats row: %w", err)
		}
		if price.Valid && price.Float64 > 0 {
			prices = append(prices, price.Float64)
			if hn.Valid && hn.String != "" {
				resolvedName = hn.String
			}
		}
	}
	if err := rows.Err(); err != nil {
		return htPriceStats{}, false, err
	}
	if len(prices) == 0 {
		return htPriceStats{}, false, nil
	}
	sort.Float64s(prices)
	return htPriceStats{
		HotelName:    resolvedName,
		Observations: len(prices),
		Low:          prices[0],
		High:         prices[len(prices)-1],
		Median:       percentile(prices, 0.50),
		P25:          percentile(prices, 0.25),
		P75:          percentile(prices, 0.75),
	}, true, nil
}

// percentile returns the value at fraction p (0..1) of a sorted slice using
// nearest-rank. Assumes sorted, non-empty input.
func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 1 {
		return sorted[0]
	}
	idx := int(float64(len(sorted)-1) * p)
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

// classifyPrice buckets a current price against a hotel's recorded
// distribution: at/below the 25th percentile is "cheap", at/above the 75th is
// "expensive", otherwise "typical". When the recorded prices show no spread
// (every observation equal), there is no basis to call a price cheap or
// expensive, so it is "typical".
func classifyPrice(current float64, stats htPriceStats) string {
	if stats.Low == stats.High {
		return "typical"
	}
	switch {
	case current <= stats.P25:
		return "cheap"
	case current >= stats.P75:
		return "expensive"
	default:
		return "typical"
	}
}

// lastPriceByHotel returns the most recent recorded price per hotel observed
// strictly before `before`, used by `watch` to detect drops since the last
// run. Keyed by hotel name.
func lastPriceByHotel(ctx context.Context, db *sql.DB, before time.Time) (map[string]float64, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT hotel_name, customer_price
		FROM ht_snapshots s
		WHERE observed_at < ? AND customer_price > 0
		  AND observed_at = (
		    SELECT MAX(observed_at) FROM ht_snapshots s2
		    WHERE s2.hotel_name = s.hotel_name AND s2.observed_at < ? AND s2.customer_price > 0
		  )`, before.UTC().Format(time.RFC3339), before.UTC().Format(time.RFC3339))
	if err != nil {
		return nil, fmt.Errorf("query last prices: %w", err)
	}
	defer rows.Close()

	out := map[string]float64{}
	for rows.Next() {
		var name sql.NullString
		var price sql.NullFloat64
		if err := rows.Scan(&name, &price); err != nil {
			return nil, fmt.Errorf("scan last price: %w", err)
		}
		if name.Valid && price.Valid {
			out[name.String] = price.Float64
		}
	}
	return out, rows.Err()
}
