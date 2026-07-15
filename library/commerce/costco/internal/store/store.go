// Package store is the hand-authored local SQLite store for Costco receipts.
// It is not generator-emitted: this CLI's spec has no syncable REST resources
// (the API is a single GraphQL endpoint), so sync/sql/search persist here.
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	_ "modernc.org/sqlite"
)

// Archive is a SQLite-backed receipt store.
type Archive struct {
	db *sql.DB
}

// Open opens (creating if needed) the archive at path and ensures the schema.
func Open(ctx context.Context, path string) (*Archive, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("opening archive %s: %w", path, err)
	}
	a := &Archive{db: db}
	if err := a.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return a, nil
}

// OpenReadOnly opens an existing archive with SQLite's query_only pragma set on
// every pooled connection, so DELETE/UPDATE/DROP/INSERT/ATTACH-write all fail at
// the engine level regardless of what SQL text the caller supplies. Used by the
// read-only sql/search commands; it does not run migrations (the file must
// already exist).
func OpenReadOnly(_ context.Context, path string) (*Archive, error) {
	dsn := (&url.URL{Scheme: "file", Path: path, RawQuery: "_pragma=query_only(true)"}).String()
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening archive %s read-only: %w", path, err)
	}
	return &Archive{db: db}, nil
}

func (a *Archive) Close() error { return a.db.Close() }

func (a *Archive) migrate(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS receipts (
			id TEXT PRIMARY KEY,
			membership_number TEXT,
			transaction_date TEXT,
			channel TEXT,
			warehouse_name TEXT,
			warehouse_number TEXT,
			item_count INTEGER,
			subtotal REAL,
			taxes REAL,
			instant_savings REAL,
			total REAL,
			barcode TEXT,
			data TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_receipts_date ON receipts(transaction_date)`,
		`CREATE TABLE IF NOT EXISTS items (
			receipt_id TEXT,
			transaction_date TEXT,
			warehouse_name TEXT,
			item_number TEXT,
			upc TEXT,
			description TEXT,
			department TEXT,
			unit_price REAL,
			quantity REAL,
			amount REAL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_items_desc ON items(description)`,
		`CREATE INDEX IF NOT EXISTS idx_items_receipt ON items(receipt_id)`,
	}
	for _, s := range stmts {
		if _, err := a.db.ExecContext(ctx, s); err != nil {
			return fmt.Errorf("migrating archive: %w", err)
		}
	}
	return nil
}

// ReceiptRow is the minimal shape Upsert needs; it mirrors the CLI's receipt
// without importing the cli package (which would be a cycle). Numeric fields are
// plain float64/int — the caller flattens the CLI's tolerant types first.
type ReceiptRow struct {
	ID               string
	MembershipNumber string
	TransactionDate  string
	Channel          string
	WarehouseName    string
	WarehouseNumber  string
	ItemCount        int
	SubTotal         float64
	Taxes            float64
	InstantSavings   float64
	Total            float64
	Barcode          string
	Raw              json.RawMessage
	Items            []ItemRow
}

// ItemRow is one line item to persist.
type ItemRow struct {
	ItemNumber  string
	UPC         string
	Description string
	Department  string
	UnitPrice   float64
	Quantity    float64
	Amount      float64
}

// UpsertResult reports how a sync changed the archive.
type UpsertResult struct {
	Added   int
	Updated int
}

// Upsert writes receipts idempotently keyed by ID (membership+barcode),
// replacing line items for any receipt it touches. Returns added vs updated
// counts. Runs in a single transaction.
func (a *Archive) Upsert(ctx context.Context, rows []ReceiptRow) (UpsertResult, error) {
	var res UpsertResult
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return res, err
	}
	defer func() { _ = tx.Rollback() }()

	for _, r := range rows {
		var existing string
		err := tx.QueryRowContext(ctx, `SELECT id FROM receipts WHERE id = ?`, r.ID).Scan(&existing)
		isNew := err == sql.ErrNoRows
		if err != nil && !isNew {
			return res, err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO receipts (id, membership_number, transaction_date, channel, warehouse_name, warehouse_number, item_count, subtotal, taxes, instant_savings, total, barcode, data)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)
			ON CONFLICT(id) DO UPDATE SET
				membership_number=excluded.membership_number,
				transaction_date=excluded.transaction_date,
				channel=excluded.channel,
				warehouse_name=excluded.warehouse_name,
				warehouse_number=excluded.warehouse_number,
				item_count=excluded.item_count,
				subtotal=excluded.subtotal,
				taxes=excluded.taxes,
				instant_savings=excluded.instant_savings,
				total=excluded.total,
				barcode=excluded.barcode,
				data=excluded.data`,
			r.ID, r.MembershipNumber, r.TransactionDate, r.Channel, r.WarehouseName, r.WarehouseNumber,
			r.ItemCount, r.SubTotal, r.Taxes, r.InstantSavings, r.Total, r.Barcode, string(r.Raw)); err != nil {
			return res, err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM items WHERE receipt_id = ?`, r.ID); err != nil {
			return res, err
		}
		for _, it := range r.Items {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO items (receipt_id, transaction_date, warehouse_name, item_number, upc, description, department, unit_price, quantity, amount)
				VALUES (?,?,?,?,?,?,?,?,?,?)`,
				r.ID, r.TransactionDate, r.WarehouseName, it.ItemNumber, it.UPC, it.Description, it.Department, it.UnitPrice, it.Quantity, it.Amount); err != nil {
				return res, err
			}
		}
		if isNew {
			res.Added++
		} else {
			res.Updated++
		}
	}
	if err := tx.Commit(); err != nil {
		return res, err
	}
	return res, nil
}

// Count returns the number of stored receipts.
func (a *Archive) Count(ctx context.Context) (int, error) {
	var n int
	err := a.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM receipts`).Scan(&n)
	return n, err
}

// QueryJSON runs a read-only SELECT and returns rows as []map[string]any.
// Non-SELECT statements are rejected so the command can never mutate the store.
func (a *Archive) QueryJSON(ctx context.Context, query string, limit int) ([]map[string]any, error) {
	trimmed := strings.TrimSpace(query)
	low := strings.ToLower(trimmed)
	if !strings.HasPrefix(low, "select") && !strings.HasPrefix(low, "with") {
		return nil, fmt.Errorf("only read-only SELECT/WITH queries are allowed")
	}
	// Reject any interior statement separator. Strip a single trailing
	// semicolon first; any remaining ';' means a second statement was chained
	// (e.g. "SELECT 1; DROP TABLE receipts;"). This is defense in depth on top
	// of the query_only pragma the read-only connection sets.
	if strings.Contains(strings.TrimRight(trimmed, "; \n\t"), ";") {
		return nil, fmt.Errorf("multiple statements are not allowed")
	}
	rows, err := a.db.QueryContext(ctx, trimmed)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	out := []map[string]any{}
	for rows.Next() {
		if limit > 0 && len(out) >= limit {
			break
		}
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		m := map[string]any{}
		for i, c := range cols {
			m[c] = normalize(vals[i])
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// SearchItems finds line items whose description or UPC matches term
// (case-insensitive substring), newest first.
func (a *Archive) SearchItems(ctx context.Context, term string, limit int) ([]map[string]any, error) {
	if limit <= 0 {
		limit = 100
	}
	like := "%" + strings.ToLower(strings.TrimSpace(term)) + "%"
	rows, err := a.db.QueryContext(ctx, `
		SELECT transaction_date, warehouse_name, item_number, upc, description, unit_price, quantity, amount
		FROM items
		WHERE lower(description) LIKE ? OR lower(upc) LIKE ?
		ORDER BY transaction_date DESC
		LIMIT ?`, like, like, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var date, wh, itemNo, upc, desc sql.NullString
		var unitPrice, qty, amount sql.NullFloat64
		if err := rows.Scan(&date, &wh, &itemNo, &upc, &desc, &unitPrice, &qty, &amount); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{
			"transactionDate": date.String,
			"warehouseName":   wh.String,
			"itemNumber":      itemNo.String,
			"upc":             upc.String,
			"description":     desc.String,
			"unitPrice":       unitPrice.Float64,
			"quantity":        qty.Float64,
			"amount":          amount.Float64,
		})
	}
	return out, rows.Err()
}

// normalize converts driver byte-slice values to strings for clean JSON.
func normalize(v any) any {
	if b, ok := v.([]byte); ok {
		return string(b)
	}
	return v
}
