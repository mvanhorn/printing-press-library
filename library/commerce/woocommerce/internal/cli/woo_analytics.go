// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.

// Shared helpers for the local-store analytics commands (orders triage, stock
// velocity, revenue explain, refund-rate, customers ltv).
//
// Hand-authored: kept in its own file so `generate --force` preserves it.
package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/woocommerce/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/woocommerce/internal/store"
)

// revenueBearingStatuses are the order statuses that represent real money.
// cancelled/failed/refunded/trash/checkout-draft are excluded so revenue math
// is not inflated by orders that never settled.
var revenueBearingStatuses = map[string]bool{
	"completed":  true,
	"processing": true,
	"on-hold":    true,
}

// wooLineItem is one row of an order's line_items array.
type wooLineItem struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	ProductID   int64  `json:"product_id"`
	VariationID int64  `json:"variation_id"`
	Quantity    int    `json:"quantity"`
	SKU         string `json:"sku"`
	Subtotal    string `json:"subtotal"`
	Total       string `json:"total"`
}

type wooCouponLine struct {
	Code     string `json:"code"`
	Discount string `json:"discount"`
}

type wooOrderRefundStub struct {
	ID     json.Number `json:"id"`
	Total  string      `json:"total"`
	Reason string      `json:"reason"`
}

// wooOrderData is the subset of the stored order JSON the analytics commands read.
type wooBilling struct {
	Email string `json:"email"`
}

type wooOrderData struct {
	DateModified string               `json:"date_modified"`
	Billing      wooBilling           `json:"billing"`
	LineItems    []wooLineItem        `json:"line_items"`
	CouponLines  []wooCouponLine      `json:"coupon_lines"`
	Refunds      []wooOrderRefundStub `json:"refunds"`
}

// wooOrder is a fully-scanned order row. Scanning completes before any
// follow-up query runs (SQLite allows one connection, so a query issued while
// a parent *sql.Rows is open will deadlock or fail).
type wooOrder struct {
	ID            string
	Number        string
	Status        string
	Currency      string
	Total         float64
	CustomerID    int64
	PaymentMethod string
	Created       time.Time
	CreatedOK     bool
	Modified      time.Time
	ModifiedOK    bool
	LineItems     []wooLineItem
	CouponLines   []wooCouponLine
	Refunds       []wooOrderRefundStub
	BillingEmail  string
}

// EffectiveTime prefers date_modified and falls back to date_created, which is
// what the order-age buckets need.
func (o wooOrder) EffectiveTime() (time.Time, bool) {
	if o.ModifiedOK {
		return o.Modified, true
	}
	return o.Created, o.CreatedOK
}

// parseWooTime parses the date shapes WooCommerce actually emits. The REST API
// documents ISO8601 but omits the zone on non-GMT fields, so a single layout is
// not enough.
func parseWooTime(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), true
		}
	}
	return time.Time{}, false
}

// parseMoney reads a WooCommerce Admin-API monetary value. Amounts arrive as
// decimal strings ("49.00"), never as JSON numbers, so a numeric decode would
// silently zero them.
func parseMoney(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return v
}

// round2 keeps emitted currency amounts readable without pretending to more
// precision than the source data has.
func round2(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	r := math.Round(v*100) / 100
	if r == 0 {
		// Avoid emitting negative zero, which renders as "-0" in JSON.
		return 0
	}
	return r
}

func round4(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	r := math.Round(v*10000) / 10000
	if r == 0 {
		return 0
	}
	return r
}

// rejectLiveDataSource enforces the `pp:data-source local` contract: these
// commands read the mirror and have no live equivalent.
func rejectLiveDataSource(flags *rootFlags, command string) error {
	if flags != nil && flags.dataSource == "live" {
		return usageErr(fmt.Errorf("no live equivalent for %q: it reads the local mirror; run 'woocommerce-pp-cli sync' then retry without --data-source live", command))
	}
	return nil
}

// localStoreMissing prints the sync hint and emits an empty JSON result when the
// mirror has not been created yet. A missing mirror is an empty-cache state, not
// a failure, so callers return nil after this reports true.
func localStoreMissing(cmd *cobra.Command, flags *rootFlags, dbPath, resources string) bool {
	if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
		return false
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: woocommerce-pp-cli sync --resources %s --db %s\n", dbPath, resources, dbPath)
	if flags != nil && (flags.asJSON || flags.agent) {
		fmt.Fprintln(cmd.OutOrStdout(), "[]")
	}
	return true
}

// loadWooOrders scans every order row into memory before returning. Callers may
// then run follow-up queries safely.
func loadWooOrders(ctx context.Context, db *store.Store) ([]wooOrder, error) {
	rows, err := db.DB().QueryContext(ctx, `
		SELECT id,
		       COALESCE(number, ''),
		       COALESCE(status, ''),
		       COALESCE(currency, ''),
		       COALESCE(total, ''),
		       COALESCE(customer_id, 0),
		       COALESCE(payment_method_title, ''),
		       COALESCE(date_created, ''),
		       COALESCE(data, '')
		FROM orders`)
	if err != nil {
		return nil, fmt.Errorf("querying orders: %w", err)
	}
	orders := make([]wooOrder, 0)
	for rows.Next() {
		var (
			id, number, status, currency, total string
			customerID                          sql.NullInt64
			payment, created, raw               string
		)
		if err := rows.Scan(&id, &number, &status, &currency, &total, &customerID, &payment, &created, &raw); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scanning order row: %w", err)
		}
		o := wooOrder{
			ID:            id,
			Number:        number,
			Status:        status,
			Currency:      currency,
			Total:         parseMoney(total),
			CustomerID:    customerID.Int64,
			PaymentMethod: strings.TrimSpace(payment),
		}
		o.Created, o.CreatedOK = parseWooTime(created)
		if raw != "" {
			var d wooOrderData
			if err := json.Unmarshal([]byte(raw), &d); err == nil {
				o.LineItems = d.LineItems
				o.CouponLines = d.CouponLines
				o.Refunds = d.Refunds
				o.Modified, o.ModifiedOK = parseWooTime(d.DateModified)
				o.BillingEmail = strings.ToLower(strings.TrimSpace(d.Billing.Email))
			}
		}
		orders = append(orders, o)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("iterating orders: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("closing orders rows: %w", err)
	}
	return orders, nil
}

// productLabel is the name/sku pair used to make numeric product IDs readable.
type productLabel struct {
	Name string
	SKU  string
}

// loadProductLabels reads id -> name/sku for products and variations. Safe to
// call only after every parent *sql.Rows has been closed.
func loadProductLabels(ctx context.Context, db *store.Store) (map[string]productLabel, error) {
	labels := make(map[string]productLabel)
	rows, err := db.DB().QueryContext(ctx, `SELECT id, COALESCE(name,''), COALESCE(sku,'') FROM products`)
	if err != nil {
		return labels, nil // products may not be synced yet; labels are cosmetic
	}
	for rows.Next() {
		var id, name, sku string
		if err := rows.Scan(&id, &name, &sku); err != nil {
			_ = rows.Close()
			return labels, fmt.Errorf("scanning product label: %w", err)
		}
		labels[id] = productLabel{Name: name, SKU: sku}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return labels, fmt.Errorf("iterating product labels: %w", err)
	}
	if err := rows.Close(); err != nil {
		return labels, fmt.Errorf("closing product label rows: %w", err)
	}
	return labels, nil
}

// resolveDBPath centralizes the --db default so every analytics command agrees.
func resolveDBPath(dbPath string) string {
	if strings.TrimSpace(dbPath) != "" {
		return dbPath
	}
	return defaultDBPath("woocommerce-pp-cli")
}

// windowOrDefault parses a loose duration flag, falling back to def when empty.
func windowOrDefault(raw, def string) (time.Duration, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		s = def
	}
	d, err := cliutil.ParseDurationLoose(s)
	if err != nil {
		return 0, usageErr(fmt.Errorf("invalid duration %q: use a form like 24h, 7d, or 4w", raw))
	}
	if d <= 0 {
		return 0, usageErr(fmt.Errorf("duration %q must be positive", raw))
	}
	return d, nil
}
