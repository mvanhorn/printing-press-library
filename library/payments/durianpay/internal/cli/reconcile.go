// Copyright 2026 ardihanan and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/durianpay/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/payments/durianpay/internal/store"
	"github.com/spf13/cobra"
)

// pp:data-source local

// localOrder / localPayment are the minimal projections the reconcile logic
// needs, scanned NULL-safely out of the resources table.
type localOrder struct {
	ID     string
	Status string
	Amount float64
}

type localPayment struct {
	ID      string
	OrderID string
	Status  string
	Amount  float64
}

type amountMismatch struct {
	OrderID       string  `json:"order_id"`
	PaymentID     string  `json:"payment_id"`
	OrderAmount   float64 `json:"order_amount"`
	PaymentAmount float64 `json:"payment_amount"`
}

type reconcileResult struct {
	MatchedCount         int              `json:"matched_count"`
	OrdersWithoutPayment []localOrder     `json:"orders_without_payment"`
	PaymentsWithoutOrder []localPayment   `json:"payments_without_order"`
	AmountMismatches     []amountMismatch `json:"amount_mismatches"`
	ScannedOrders        int              `json:"scanned_orders"`
	ScannedPayments      int              `json:"scanned_payments"`
	Window               string           `json:"window"`
	Note                 string           `json:"note,omitempty"`
}

// orderLooksPaid reports whether an order's status indicates it was paid or
// completed and therefore should have a matching payment row.
func orderLooksPaid(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "paid", "completed", "settled", "captured", "success", "succeeded":
		return true
	}
	return false
}

// classifyReconcile is the pure row-classification logic over in-memory
// slices. It is unit-tested directly without SQLite.
func classifyReconcile(orders []localOrder, payments []localPayment, window string) reconcileResult {
	res := reconcileResult{
		OrdersWithoutPayment: make([]localOrder, 0),
		PaymentsWithoutOrder: make([]localPayment, 0),
		AmountMismatches:     make([]amountMismatch, 0),
		ScannedOrders:        len(orders),
		ScannedPayments:      len(payments),
		Window:               window,
	}

	// Index payments by the order they reference.
	payByOrder := map[string][]localPayment{}
	for _, p := range payments {
		if p.OrderID != "" {
			payByOrder[p.OrderID] = append(payByOrder[p.OrderID], p)
		}
	}
	orderByID := map[string]localOrder{}
	for _, o := range orders {
		orderByID[o.ID] = o
	}

	for _, o := range orders {
		matches := payByOrder[o.ID]
		if orderLooksPaid(o.Status) && len(matches) == 0 {
			res.OrdersWithoutPayment = append(res.OrdersWithoutPayment, o)
			continue
		}
		if len(matches) > 0 {
			res.MatchedCount++
			for _, p := range matches {
				// IDR amounts parse from strings; compare with a sub-cent epsilon
				// rather than exact float equality to avoid representation artifacts.
				if math.Abs(o.Amount-p.Amount) > 0.005 {
					res.AmountMismatches = append(res.AmountMismatches, amountMismatch{
						OrderID:       o.ID,
						PaymentID:     p.ID,
						OrderAmount:   o.Amount,
						PaymentAmount: p.Amount,
					})
				}
			}
		}
	}

	for _, p := range payments {
		if p.OrderID == "" {
			res.PaymentsWithoutOrder = append(res.PaymentsWithoutOrder, p)
			continue
		}
		if _, ok := orderByID[p.OrderID]; !ok {
			res.PaymentsWithoutOrder = append(res.PaymentsWithoutOrder, p)
		}
	}
	return res
}

// nullFloat parses a SQL string/number value into a float, tolerating NULL
// and non-numeric junk (returns 0).
func nullFloat(s sql.NullString) float64 {
	if !s.Valid {
		return 0
	}
	f, err := strconv.ParseFloat(strings.TrimSpace(s.String), 64)
	if err != nil {
		return 0
	}
	return f
}

// resourceWindowEpoch returns the Unix-second cutoff for the --since window.
const reconcileTimeExpr = `COALESCE(json_extract(data,'$.created_at'), json_extract(data,'$.created'))`

// queryLocalOrders reads order rows from the resources table within the window.
func queryLocalOrders(db *store.Store, cutoff time.Time) ([]localOrder, error) {
	rows, err := db.DB().Query(
		`SELECT id,
		        json_extract(data,'$.status'),
		        json_extract(data,'$.amount'),
		        ` + reconcileTimeExpr + `
		   FROM resources WHERE resource_type = 'orders'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []localOrder
	for rows.Next() {
		var id string
		var status, amount, created sql.NullString
		if err := rows.Scan(&id, &status, &amount, &created); err != nil {
			return nil, err
		}
		if !withinWindow(created, cutoff) {
			continue
		}
		out = append(out, localOrder{ID: id, Status: nullStr(status), Amount: nullFloat(amount)})
	}
	return out, rows.Err()
}

// queryLocalPayments reads payment rows from the resources table within the window.
func queryLocalPayments(db *store.Store, cutoff time.Time) ([]localPayment, error) {
	rows, err := db.DB().Query(
		`SELECT id,
		        json_extract(data,'$.order_id'),
		        json_extract(data,'$.status'),
		        json_extract(data,'$.amount'),
		        ` + reconcileTimeExpr + `
		   FROM resources WHERE resource_type = 'payments'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []localPayment
	for rows.Next() {
		var id string
		var orderID, status, amount, created sql.NullString
		if err := rows.Scan(&id, &orderID, &status, &amount, &created); err != nil {
			return nil, err
		}
		if !withinWindow(created, cutoff) {
			continue
		}
		out = append(out, localPayment{ID: id, OrderID: nullStr(orderID), Status: nullStr(status), Amount: nullFloat(amount)})
	}
	return out, rows.Err()
}

func nullStr(s sql.NullString) string {
	if !s.Valid {
		return ""
	}
	return s.String
}

// withinWindow returns true when a created timestamp is at or after the
// cutoff. Defensive: when the timestamp is missing or unparseable, the row is
// kept (we'd rather over-report than silently drop unparseable rows).
func withinWindow(created sql.NullString, cutoff time.Time) bool {
	if !created.Valid || strings.TrimSpace(created.String) == "" {
		return true
	}
	t, ok := parseLooseTime(created.String)
	if !ok {
		return true
	}
	return !t.Before(cutoff)
}

// parseLooseTime parses common timestamp shapes: RFC3339, epoch seconds, epoch
// millis.
func parseLooseTime(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339, time.RFC3339Nano, "2006-01-02 15:04:05", "2006-01-02T15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		// Heuristic: 13-digit => millis, otherwise seconds.
		if n > 1e12 {
			return time.UnixMilli(n), true
		}
		return time.Unix(n, 0), true
	}
	return time.Time{}, false
}

// openLocalStore opens the local SQLite store read-only at dbPath (or the
// default path) for the read-only novel commands.
func openLocalStore(dbPath string) (*store.Store, error) {
	if dbPath == "" {
		dbPath = defaultDBPath("durianpay-pp-cli")
	}
	// Read-only open: reconcile/refund-audit/stuck never write. This returns a
	// clean error when the DB doesn't exist yet (instead of silently creating
	// an empty one) and never contends with a concurrent sync for the migration
	// write lock.
	db, err := store.OpenReadOnly(dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening local database: %w\nRun 'durianpay-pp-cli sync' first.", err)
	}
	return db, nil
}

// rejectLiveDataSource returns a usage error for static/local-only commands
// invoked with --data-source live.
func rejectLiveDataSource(cmd *cobra.Command, flags *rootFlags, name string) error {
	if flags != nil && flags.dataSource == "live" {
		_ = cmd.Usage()
		return usageErr(fmt.Errorf("%s reads the local store and has no live equivalent; drop --data-source live", name))
	}
	return nil
}

func newNovelReconcileCmd(flags *rootFlags) *cobra.Command {
	var flagSince string
	var dbPath string

	cmd := &cobra.Command{
		Use:         "reconcile",
		Short:       "Join synced orders against payments locally to flag charged-but-unsettled orders, payments with no order, and amount mismatches.",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "--since=7d"},
		Long: `Reconcile locally synced orders against payments. Flags three problems:
  - orders that look paid/completed but have no payment referencing them
  - payments with no matching local order
  - order amount vs payment amount mismatches

Reads only the local store (sync orders,payments first). Offline.`,
		Example: strings.TrimLeft(`
  durianpay-pp-cli reconcile
  durianpay-pp-cli reconcile --since=30d --json`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := rejectLiveDataSource(cmd, flags, "reconcile"); err != nil {
				return err
			}
			window, err := cliutil.ParseDurationLoose(flagSince)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid --since %q: %w", flagSince, err))
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.ErrOrStderr(), "dry-run: would query local store (orders, payments)")
				return nil
			}
			db, err := openLocalStore(dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			hintIfUnsynced(cmd, db, "orders")
			hintIfUnsynced(cmd, db, "payments")
			hintIfStale(cmd, db, "orders", flags.maxAge)
			hintIfStale(cmd, db, "payments", flags.maxAge)

			cutoff := time.Now().Add(-window)
			orders, err := queryLocalOrders(db, cutoff)
			if err != nil {
				return fmt.Errorf("querying orders: %w", err)
			}
			payments, err := queryLocalPayments(db, cutoff)
			if err != nil {
				return fmt.Errorf("querying payments: %w", err)
			}

			res := classifyReconcile(orders, payments, flagSince)
			if len(orders) == 0 && len(payments) == 0 {
				res.Note = "no local orders or payments; run 'durianpay-pp-cli sync --resources orders,payments --since 7d'"
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}
			return printReconcileTable(cmd, res)
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "7d", "Only consider rows created within this window (e.g. 7d, 24h, 2w)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to the local SQLite store (default: standard data dir)")
	return cmd
}

func printReconcileTable(cmd *cobra.Command, res reconcileResult) error {
	out := cmd.OutOrStdout()
	if res.Note != "" {
		fmt.Fprintln(out, res.Note)
		return nil
	}
	fmt.Fprintf(out, "window=%s  scanned: %d orders, %d payments  matched=%d\n", res.Window, res.ScannedOrders, res.ScannedPayments, res.MatchedCount)
	fmt.Fprintf(out, "orders without payment: %d\n", len(res.OrdersWithoutPayment))
	for _, o := range res.OrdersWithoutPayment {
		fmt.Fprintf(out, "  %s status=%s amount=%s\n", o.ID, o.Status, strconv.FormatFloat(o.Amount, 'f', -1, 64))
	}
	fmt.Fprintf(out, "payments without order: %d\n", len(res.PaymentsWithoutOrder))
	for _, p := range res.PaymentsWithoutOrder {
		fmt.Fprintf(out, "  %s order_id=%q amount=%s\n", p.ID, p.OrderID, strconv.FormatFloat(p.Amount, 'f', -1, 64))
	}
	fmt.Fprintf(out, "amount mismatches: %d\n", len(res.AmountMismatches))
	for _, m := range res.AmountMismatches {
		fmt.Fprintf(out, "  order=%s payment=%s order_amount=%s payment_amount=%s\n",
			m.OrderID, m.PaymentID,
			strconv.FormatFloat(m.OrderAmount, 'f', -1, 64),
			strconv.FormatFloat(m.PaymentAmount, 'f', -1, 64))
	}
	return nil
}
