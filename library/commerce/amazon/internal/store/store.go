// Package store is the SQLite-backed local history layer.
//
// Schema (per-profile DB at ~/.config/amazon-pp-cli/profiles/<name>/history.db):
//
//	orders            (order_id PK, placed_at, total_cents, raw_json)
//	order_items       (order_id FK, asin, title, quantity, unit_price_cents, raw_json)
//	purchased_items   (asin PK, title, purchase_count, last_purchased_at, last_order_id)
//	purchased_items_fts FTS5 virtual table over (title) contentless, synced via triggers
//	sync_meta         (key, value)
//
// purchased_items is a denormalized rollup over order_items, refreshed by
// RebuildPurchasedItems() after an import or sync. The FTS5 table lets the
// add resolver answer "did the user ever buy something matching 'bath tissue'?"
// in milliseconds.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Store wraps the per-profile SQLite handle.
type Store struct {
	db   *sql.DB
	path string
}

// Open opens (or creates) the SQLite DB at path and applies the schema.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("opening sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db, path: path}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }
func (s *Store) DB() *sql.DB  { return s.db }
func (s *Store) Path() string { return s.path }

func (s *Store) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS orders (
			order_id    TEXT PRIMARY KEY,
			placed_at   TEXT NOT NULL,
			total_cents INTEGER,
			raw_json    TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS order_items (
			order_id        TEXT NOT NULL,
			asin            TEXT NOT NULL,
			title           TEXT NOT NULL,
			quantity        INTEGER DEFAULT 1,
			unit_price_cents INTEGER,
			raw_json        TEXT,
			PRIMARY KEY (order_id, asin),
			FOREIGN KEY (order_id) REFERENCES orders(order_id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS purchased_items (
			asin              TEXT PRIMARY KEY,
			title             TEXT NOT NULL,
			purchase_count    INTEGER NOT NULL DEFAULT 0,
			last_purchased_at TEXT NOT NULL,
			last_order_id     TEXT
		)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS purchased_items_fts
			USING fts5(asin UNINDEXED, title, content='purchased_items', content_rowid='rowid')`,
		`CREATE TRIGGER IF NOT EXISTS purchased_items_ai AFTER INSERT ON purchased_items BEGIN
			INSERT INTO purchased_items_fts(rowid, asin, title) VALUES (new.rowid, new.asin, new.title);
		END`,
		`CREATE TRIGGER IF NOT EXISTS purchased_items_ad AFTER DELETE ON purchased_items BEGIN
			INSERT INTO purchased_items_fts(purchased_items_fts, rowid, asin, title)
			VALUES ('delete', old.rowid, old.asin, old.title);
		END`,
		`CREATE TRIGGER IF NOT EXISTS purchased_items_au AFTER UPDATE ON purchased_items BEGIN
			INSERT INTO purchased_items_fts(purchased_items_fts, rowid, asin, title)
			VALUES ('delete', old.rowid, old.asin, old.title);
			INSERT INTO purchased_items_fts(rowid, asin, title) VALUES (new.rowid, new.asin, new.title);
		END`,
		`CREATE TABLE IF NOT EXISTS sync_meta (
			key   TEXT PRIMARY KEY,
			value TEXT
		)`,
	}
	for _, q := range stmts {
		if _, err := s.db.Exec(q); err != nil {
			return fmt.Errorf("migrate %.40q: %w", q, err)
		}
	}
	return nil
}

// Order is one historical order.
type Order struct {
	OrderID    string    `json:"order_id"`
	PlacedAt   time.Time `json:"placed_at"`
	TotalCents int64     `json:"total_cents"`
}

// OrderItem is one line in a historical order.
type OrderItem struct {
	OrderID        string `json:"order_id"`
	ASIN           string `json:"asin"`
	Title          string `json:"title"`
	Quantity       int    `json:"quantity"`
	UnitPriceCents int64  `json:"unit_price_cents"`
}

// PurchasedItem is the denormalized rollup row.
type PurchasedItem struct {
	ASIN            string    `json:"asin"`
	Title           string    `json:"title"`
	PurchaseCount   int       `json:"purchase_count"`
	LastPurchasedAt time.Time `json:"last_purchased_at"`
	LastOrderID     string    `json:"last_order_id"`
}

// UpsertOrder inserts or replaces an order row.
func (s *Store) UpsertOrder(ctx context.Context, o Order, rawJSON string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO orders (order_id, placed_at, total_cents, raw_json)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(order_id) DO UPDATE SET
			placed_at=excluded.placed_at,
			total_cents=excluded.total_cents,
			raw_json=excluded.raw_json
	`, o.OrderID, o.PlacedAt.Format(time.RFC3339), o.TotalCents, rawJSON)
	return err
}

// UpsertOrderItem inserts or replaces an order_items row.
func (s *Store) UpsertOrderItem(ctx context.Context, oi OrderItem, rawJSON string) error {
	if oi.Quantity == 0 {
		oi.Quantity = 1
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO order_items (order_id, asin, title, quantity, unit_price_cents, raw_json)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(order_id, asin) DO UPDATE SET
			title=excluded.title,
			quantity=excluded.quantity,
			unit_price_cents=excluded.unit_price_cents,
			raw_json=excluded.raw_json
	`, oi.OrderID, oi.ASIN, oi.Title, oi.Quantity, oi.UnitPriceCents, rawJSON)
	return err
}

// RebuildPurchasedItems regenerates purchased_items from order_items.
// Called after a batch import so the rollup view is consistent.
func (s *Store) RebuildPurchasedItems(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM purchased_items`); err != nil {
		return err
	}
	// purchase_count = SUM(quantity), last = most recent placed_at among orders containing this ASIN.
	_, err = tx.ExecContext(ctx, `
		INSERT INTO purchased_items (asin, title, purchase_count, last_purchased_at, last_order_id)
		SELECT
			oi.asin,
			MAX(oi.title) AS title,
			SUM(oi.quantity) AS purchase_count,
			MAX(o.placed_at) AS last_purchased_at,
			(SELECT oi2.order_id
			   FROM order_items oi2 JOIN orders o2 ON o2.order_id = oi2.order_id
			  WHERE oi2.asin = oi.asin
			  ORDER BY o2.placed_at DESC LIMIT 1) AS last_order_id
		FROM order_items oi
		JOIN orders o ON o.order_id = oi.order_id
		GROUP BY oi.asin
	`)
	if err != nil {
		return err
	}
	return tx.Commit()
}

// ListPurchasedItems returns the top N purchased items, ordered by purchase_count DESC, last_purchased_at DESC.
func (s *Store) ListPurchasedItems(limit int) ([]PurchasedItem, error) {
	if limit <= 0 {
		limit = 25
	}
	rows, err := s.db.Query(`
		SELECT asin, title, purchase_count, last_purchased_at, COALESCE(last_order_id, '')
		FROM purchased_items
		ORDER BY purchase_count DESC, last_purchased_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPurchasedItems(rows)
}

// MatchQuality describes how strictly the FTS query matched.
//   - "strict": all query tokens matched (AND).
//   - "loose":  AND matched nothing; fallback to OR matched any token.
//
// Loose matches require extra user confirmation — they're the difference
// between "cauli puffs" finding the user's CauliPuffs (compound-word rescue)
// and "garden hose" finding Garden Veggie Snacks (false positive on one token).
type MatchQuality string

const (
	MatchStrict MatchQuality = "strict"
	MatchLoose  MatchQuality = "loose"
)

// SearchPurchasedItems runs an FTS5 query and returns matches ordered by
// purchase_count DESC, last_purchased_at DESC, along with a quality marker.
//
// Two-stage match: AND (all tokens required) first, then OR (any token) as
// fallback. AND gives precision; OR rescues compound-word cases where FTS5
// tokenization differs from user spacing ("cauli puffs" vs indexed "CauliPuffs").
// Callers MUST surface the MatchQuality to the user so loose matches get
// extra scrutiny before any cart write.
func (s *Store) SearchPurchasedItems(query string, limit int) ([]PurchasedItem, MatchQuality, error) {
	if limit <= 0 {
		limit = 10
	}
	andQ, orQ := NormalizeFTSQueries(query)
	if andQ == "" {
		return nil, "", nil
	}
	results, err := s.searchFTS(andQ, limit)
	if err != nil {
		return nil, "", err
	}
	if len(results) > 0 {
		return results, MatchStrict, nil
	}
	// Single-token queries: AND and OR forms are identical, so the OR fallback
	// would just re-run the same query. Skip it — strict-or-nothing is correct
	// for single tokens. Do NOT "fix" this short-circuit to enable single-token
	// loose matches; that would let "protein" loose-match anything with that word.
	if andQ == orQ {
		return nil, "", nil
	}
	// AND yielded nothing — try OR fallback (compound-word safety net).
	results, err = s.searchFTS(orQ, limit)
	if err != nil {
		return nil, "", err
	}
	if len(results) == 0 {
		return nil, "", nil
	}
	return results, MatchLoose, nil
}

func (s *Store) searchFTS(ftsQuery string, limit int) ([]PurchasedItem, error) {
	rows, err := s.db.Query(`
		SELECT p.asin, p.title, p.purchase_count, p.last_purchased_at, COALESCE(p.last_order_id, '')
		FROM purchased_items p
		JOIN purchased_items_fts f ON f.rowid = p.rowid
		WHERE purchased_items_fts MATCH ?
		ORDER BY p.purchase_count DESC, p.last_purchased_at DESC
		LIMIT ?
	`, ftsQuery, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPurchasedItems(rows)
}

// MostRecentOrder returns the orders table's max placed_at, or zero time if empty.
func (s *Store) MostRecentOrder() (Order, error) {
	row := s.db.QueryRow(`
		SELECT order_id, placed_at, COALESCE(total_cents, 0)
		FROM orders
		ORDER BY placed_at DESC LIMIT 1
	`)
	var o Order
	var placedAt string
	if err := row.Scan(&o.OrderID, &placedAt, &o.TotalCents); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Order{}, nil
		}
		return Order{}, err
	}
	t, err := time.Parse(time.RFC3339, placedAt)
	if err != nil {
		return Order{}, err
	}
	o.PlacedAt = t
	return o, nil
}

// OrderItems returns all order_items rows for a given order_id.
func (s *Store) OrderItems(orderID string) ([]OrderItem, error) {
	rows, err := s.db.Query(`
		SELECT order_id, asin, title, quantity, COALESCE(unit_price_cents, 0)
		FROM order_items WHERE order_id = ?
	`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []OrderItem
	for rows.Next() {
		var oi OrderItem
		if err := rows.Scan(&oi.OrderID, &oi.ASIN, &oi.Title, &oi.Quantity, &oi.UnitPriceCents); err != nil {
			return nil, err
		}
		out = append(out, oi)
	}
	return out, rows.Err()
}

// CountOrders returns the orders row count.
func (s *Store) CountOrders() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM orders`).Scan(&n)
	return n, err
}

// CountPurchasedItems returns the purchased_items row count and the maximum
// last_purchased_at across all rows (zero time if empty).
func (s *Store) CountPurchasedItems() (int, time.Time, error) {
	var n int
	var lastPurchased sql.NullString
	err := s.db.QueryRow(`SELECT COUNT(*), MAX(last_purchased_at) FROM purchased_items`).Scan(&n, &lastPurchased)
	if err != nil {
		return 0, time.Time{}, err
	}
	if !lastPurchased.Valid || lastPurchased.String == "" {
		return n, time.Time{}, nil
	}
	t, err := time.Parse(time.RFC3339, lastPurchased.String)
	if err != nil {
		return n, time.Time{}, nil
	}
	return n, t, nil
}

func scanPurchasedItems(rows *sql.Rows) ([]PurchasedItem, error) {
	var out []PurchasedItem
	for rows.Next() {
		var p PurchasedItem
		var lastPurchased string
		if err := rows.Scan(&p.ASIN, &p.Title, &p.PurchaseCount, &lastPurchased, &p.LastOrderID); err != nil {
			return nil, err
		}
		if t, err := time.Parse(time.RFC3339, lastPurchased); err == nil {
			p.LastPurchasedAt = t
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// NormalizeFTSQuery returns the AND form of the query (all tokens required).
// Kept for backwards compatibility with existing callers and tests.
func NormalizeFTSQuery(q string) string {
	andQ, _ := NormalizeFTSQueries(q)
	return andQ
}

// NormalizeFTSQueries turns "bath tissue" into two FTS5 forms: a strict AND
// query (`bath* AND tissue*`) and a permissive OR query (`bath* OR tissue*`).
// The caller tries AND first for precision, then OR as a fallback for
// compound-word cases ("cauli puffs" doesn't AND-match "CauliPuffs" because
// FTS5 tokenizes the indexed title as one word).
func NormalizeFTSQueries(q string) (and, or string) {
	q = strings.TrimSpace(q)
	if q == "" {
		return "", ""
	}
	// Replace any non-letter/digit with whitespace.
	var b strings.Builder
	for _, r := range q {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
		default:
			b.WriteRune(' ')
		}
	}
	tokens := strings.Fields(b.String())
	if len(tokens) == 0 {
		return "", ""
	}
	for i, t := range tokens {
		tokens[i] = t + "*"
	}
	return strings.Join(tokens, " AND "), strings.Join(tokens, " OR ")
}
