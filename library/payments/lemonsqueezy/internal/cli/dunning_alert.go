// Copyright 2026 Joseph Alvin Castillo and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source local

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/payments/lemonsqueezy/internal/store"
	"github.com/spf13/cobra"
)

type dunningRow struct {
	InvoiceID         string  `json:"invoice_id"`
	SubscriptionID    string  `json:"subscription_id"`
	CustomerID        string  `json:"customer_id"`
	CustomerEmail     string  `json:"customer_email,omitempty"`
	SubscriptionState string  `json:"subscription_status"`
	InvoiceCreatedAt  string  `json:"invoice_created_at"`
	AmountUSD         float64 `json:"amount_usd"`
	BillingReason     string  `json:"billing_reason,omitempty"`
}

type dunningAlertView struct {
	Rows           []dunningRow `json:"rows"`
	Count          int          `json:"count"`
	RecoverableUSD float64      `json:"recoverable_usd"`
	Note           string       `json:"note,omitempty"`
}

var recoverableSubStates = map[string]bool{
	"active":   true,
	"past_due": true,
}

func newNovelDunningAlertCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "dunning-alert",
		Short: "Failed invoices whose subscription is still active or past_due — the recoverable window",
		Long: `Lists subscription-invoices with status='failed' whose parent subscription is
still in a recoverable state (active or past_due). This is the dollar window
where a Slack ping or grace email can save the renewal.

Use this command for invoice-level failed-charge recovery. For status-change
events on the subscription itself (already cancelled/expired), use
'churn-watch' instead.

Data source: local. Run 'sync --resources subscriptions,subscription-invoices,customers' first.`,
		Example: "  lemonsqueezy-pp-cli sync --resources subscriptions,subscription-invoices,customers\n  lemonsqueezy-pp-cli dunning-alert --json",
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "local",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Read-only local query — no mutation to suppress; we still run
			// so --dry-run --json emits a real view instead of empty stdout.
			if dbPath == "" {
				dbPath = defaultDBPath("lemonsqueezy-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			hintIfMultiUnsynced(cmd, db, "subscription-invoices",
				[]string{"subscriptions", "customers"}, flags.maxAge)

			view, err := buildDunningAlert(db)
			if err != nil {
				return err
			}
			return emitDunningAlert(cmd, flags, view)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Local SQLite database path")
	return cmd
}

func emitDunningAlert(cmd *cobra.Command, flags *rootFlags, view dunningAlertView) error {
	if len(view.Rows) > 0 && wantsHumanTable(cmd.OutOrStdout(), flags) {
		items := make([]map[string]any, 0, len(view.Rows))
		for _, r := range view.Rows {
			items = append(items, map[string]any{
				"invoice_id":         r.InvoiceID,
				"subscription_id":    r.SubscriptionID,
				"customer_email":     r.CustomerEmail,
				"sub_status":         r.SubscriptionState,
				"invoice_created_at": r.InvoiceCreatedAt,
				"amount_usd":         fmt.Sprintf("%.2f", r.AmountUSD),
			})
		}
		if err := printAutoTable(cmd.OutOrStdout(), items); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "\nRecoverable: $%.2f across %d failed invoice(s).\n",
			view.RecoverableUSD, view.Count)
		if view.Note != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "Note: %s\n", view.Note)
		}
		return nil
	}
	return flags.printJSON(cmd, view)
}

func buildDunningAlert(db *store.Store) (dunningAlertView, error) {
	view := dunningAlertView{Rows: []dunningRow{}}

	subStates := loadSubscriptionStates(db)
	customerEmails := loadCustomerEmails(db)

	// dunningInvoiceScanCap caps the subscription-invoices scan. Hitting the
	// cap surfaces a warning so callers can distinguish "no recoverable
	// failures" from "recoverable failures may exist beyond the scan window".
	const dunningInvoiceScanCap = 500000
	rows, err := db.Query(
		`SELECT data FROM resources WHERE resource_type = 'subscription-invoices' LIMIT ?`,
		dunningInvoiceScanCap,
	)
	if err != nil {
		return view, fmt.Errorf("querying subscription-invoices: %w", err)
	}
	defer rows.Close()
	scannedInvoices := 0

	for rows.Next() {
		scannedInvoices++
		var data sql.NullString
		if err := rows.Scan(&data); err != nil {
			continue
		}
		if !data.Valid {
			continue
		}
		var env struct {
			ID         string `json:"id"`
			Attributes struct {
				SubscriptionID any    `json:"subscription_id"`
				CustomerID     any    `json:"customer_id"`
				Status         string `json:"status"`
				CreatedAt      string `json:"created_at"`
				Total          any    `json:"total"`
				TotalUSD       any    `json:"total_usd"`
				BillingReason  string `json:"billing_reason"`
			} `json:"attributes"`
		}
		if err := json.Unmarshal([]byte(data.String), &env); err != nil {
			continue
		}
		if env.Attributes.Status != "failed" {
			continue
		}
		subID := toStringLS(env.Attributes.SubscriptionID)
		subState, ok := subStates[subID]
		if !ok || !recoverableSubStates[subState] {
			continue
		}
		amt := toFloatLS(env.Attributes.TotalUSD)
		if amt == 0 {
			amt = toFloatLS(env.Attributes.Total)
		}
		customerID := toStringLS(env.Attributes.CustomerID)
		row := dunningRow{
			InvoiceID:         env.ID,
			SubscriptionID:    subID,
			SubscriptionState: subState,
			CustomerID:        customerID,
			CustomerEmail:     customerEmails[customerID],
			InvoiceCreatedAt:  env.Attributes.CreatedAt,
			AmountUSD:         amt / 100.0,
			BillingReason:     env.Attributes.BillingReason,
		}
		view.Rows = append(view.Rows, row)
		view.RecoverableUSD += row.AmountUSD
	}
	if scannedInvoices >= dunningInvoiceScanCap {
		fmt.Fprintf(os.Stderr, "warning: dunning-alert hit the %d-invoice scan cap; recoverable failed invoices may exist beyond the window\n", dunningInvoiceScanCap)
	}
	sort.Slice(view.Rows, func(i, j int) bool {
		return view.Rows[i].AmountUSD > view.Rows[j].AmountUSD
	})
	view.Count = len(view.Rows)
	if scannedInvoices >= dunningInvoiceScanCap {
		view.Note = fmt.Sprintf("hit the %d-invoice scan cap; recoverable failed invoices may exist beyond this window. Open an issue if your invoice volume routinely exceeds this.", dunningInvoiceScanCap)
	} else if view.Count == 0 {
		view.Note = "no recoverable failed invoices; no subscriptions in active or past_due state have a failed invoice in the local mirror"
	}
	return view, nil
}
