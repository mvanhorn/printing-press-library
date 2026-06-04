// Copyright 2026 Joseph Alvin Castillo and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source local

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/payments/revenuecat/internal/store"
	"github.com/spf13/cobra"
)

// recoverableSubStates are the subscription statuses where a billing issue can
// still be recovered before access is lost: in_grace_period (access retained)
// and in_billing_retry (billing being retried).
var recoverableSubStates = map[string]bool{
	"in_grace_period":  true,
	"in_billing_retry": true,
}

type dunningRow struct {
	SubscriptionID string  `json:"subscription_id"`
	CustomerID     string  `json:"customer_id"`
	Status         string  `json:"subscription_status"`
	ProductID      string  `json:"product_id,omitempty"`
	UnpaidInvoices int     `json:"unpaid_invoices"`
	RecoverableUSD float64 `json:"recoverable_usd"`
}

type dunningAlertView struct {
	ProjectID      string       `json:"project_id"`
	Rows           []dunningRow `json:"rows"`
	Count          int          `json:"count"`
	RecoverableUSD float64      `json:"recoverable_usd"`
	Note           string       `json:"note,omitempty"`
}

func newNovelDunningAlertCmd(flags *rootFlags) *cobra.Command {
	var projectFlag string
	var dbPath string
	cmd := &cobra.Command{
		Use:   "dunning-alert",
		Short: "Subscriptions in grace / billing-retry joined with their unpaid invoices, ranked by recoverable amount",
		Long: `Local join of the 'subscriptions' mirror filtered to recoverable billing states
(in_grace_period, in_billing_retry) against the customer's unpaid 'invoices'
(an invoice with a null paid_at), ranked by the dollar amount still recoverable.
This is the window where a dunning email or in-app prompt can save the renewal.

Use this command for the recoverable failed-billing window. Do NOT use it for
already-expired churned subscriptions; use 'churn-watch' instead.

Data source: local. Run 'sync' for subscriptions and customer invoices first.`,
		Example: "  revenuecat-pp-cli dunning-alert --project proj1ab2c3d4 --json",
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "local",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if err := validateDataSourceStrategy(flags, "local"); err != nil {
				return usageErr(err)
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would join recoverable subscriptions against their unpaid invoices")
				return nil
			}
			projectID, err := resolveProjectID(projectFlag)
			if err != nil {
				return err
			}
			if dbPath == "" {
				dbPath = defaultDBPath("revenuecat-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			hintIfMultiUnsynced(cmd, db, "subscriptions", []string{"invoices"}, flags.maxAge)

			view, err := buildDunningAlert(db, projectID)
			if err != nil {
				return err
			}
			return emitDunningAlert(cmd, flags, view)
		},
	}
	cmd.Flags().StringVar(&projectFlag, "project", "", "RevenueCat project id (or set REVENUECAT_PROJECT_ID)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local SQLite database path")
	return cmd
}

func emitDunningAlert(cmd *cobra.Command, flags *rootFlags, view dunningAlertView) error {
	if len(view.Rows) > 0 && wantsHumanTable(cmd.OutOrStdout(), flags) {
		items := make([]map[string]any, 0, len(view.Rows))
		for _, r := range view.Rows {
			items = append(items, map[string]any{
				"subscription_id": r.SubscriptionID,
				"customer_id":     r.CustomerID,
				"status":          r.Status,
				"product_id":      r.ProductID,
				"unpaid_invoices": r.UnpaidInvoices,
				"recoverable_usd": fmt.Sprintf("%.2f", r.RecoverableUSD),
			})
		}
		if err := printAutoTable(cmd.OutOrStdout(), items); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "\nRecoverable: $%.2f across %d subscription(s).\n",
			view.RecoverableUSD, view.Count)
		if view.Note != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "Note: %s\n", view.Note)
		}
		return nil
	}
	return flags.printJSON(cmd, view)
}

// dunningSubScanCap bounds the subscriptions scan.
// Aligned with loadSubscriptionStatusCap (rc_helpers.go) so the same
// subscriptions scan is truncated identically across commands.
const dunningSubScanCap = 500000

// buildDunningAlert joins recoverable-state subscriptions with their customer's
// unpaid invoices. RevenueCat invoices carry no subscription_id (per the v2
// schema), so the join is at customer granularity: unpaid invoice counts are
// attributed to the customer's recoverable subscription(s).
//
// TODO(verify): confirm invoice "unpaid" detection (null paid_at) and the
// invoice→customer linkage against live data.
func buildDunningAlert(db *store.Store, projectID string) (dunningAlertView, error) {
	view := dunningAlertView{ProjectID: projectID, Rows: []dunningRow{}}

	unpaidByCustomer, recoverableByCustomer := loadUnpaidInvoicesByCustomer(db)

	rows, err := db.Query(
		`SELECT data FROM resources
		 WHERE resource_type IN ('subscriptions','customers_subscriptions') LIMIT ?`,
		dunningSubScanCap,
	)
	if err != nil {
		return view, fmt.Errorf("querying subscriptions: %w", err)
	}
	defer rows.Close()
	scanned := 0
	// A customer's unpaid-invoice total must count toward the project
	// RecoverableUSD only once, even when the customer has several recoverable
	// subscriptions (otherwise the headline total is inflated N×).
	countedCustomers := map[string]bool{}

	for rows.Next() {
		scanned++
		var data sql.NullString
		if rows.Scan(&data) != nil || !data.Valid {
			continue
		}
		var obj map[string]any
		if json.Unmarshal([]byte(data.String), &obj) != nil {
			continue
		}
		status, _ := obj["status"].(string)
		if !recoverableSubStates[status] {
			continue
		}
		customerID := toStringRC(obj["customer_id"])
		row := dunningRow{
			SubscriptionID: toStringRC(obj["id"]),
			CustomerID:     customerID,
			Status:         status,
			ProductID:      toStringRC(obj["product_id"]),
			UnpaidInvoices: unpaidByCustomer[customerID],
		}
		// Recoverable amount: prefer the customer's unpaid-invoice total; fall
		// back to the subscription's own revenue-at-risk when no invoice rows
		// are mirrored (invoice sync is optional).
		if amt, ok := recoverableByCustomer[customerID]; ok && amt > 0 {
			row.RecoverableUSD = amt
			// recoverableByCustomer is keyed only by non-empty customer ids
			// (loadUnpaidInvoicesByCustomer skips empty cid), so add each
			// customer's invoice total to the project total exactly once.
			if !countedCustomers[customerID] {
				view.RecoverableUSD += amt
				countedCustomers[customerID] = true
			}
		} else {
			// Per-subscription fallback is unique to this row; always add it.
			row.RecoverableUSD = monetaryGrossUSD(obj["total_revenue_in_usd"])
			view.RecoverableUSD += row.RecoverableUSD
		}
		view.Rows = append(view.Rows, row)
	}
	if err := rows.Err(); err != nil {
		return view, fmt.Errorf("iterating subscriptions: %w", err)
	}
	if scanned >= dunningSubScanCap {
		fmt.Fprintf(os.Stderr, "warning: dunning-alert hit the %d-subscription scan cap; recoverable subscriptions may exist beyond the window\n", dunningSubScanCap)
		view.Note = fmt.Sprintf("hit the %d-subscription scan cap; results may be incomplete.", dunningSubScanCap)
	}
	sort.Slice(view.Rows, func(i, j int) bool {
		if view.Rows[i].RecoverableUSD != view.Rows[j].RecoverableUSD {
			return view.Rows[i].RecoverableUSD > view.Rows[j].RecoverableUSD
		}
		return view.Rows[i].SubscriptionID < view.Rows[j].SubscriptionID
	})
	view.Count = len(view.Rows)
	if view.Count == 0 && view.Note == "" {
		view.Note = "no subscriptions in a recoverable billing state (in_grace_period / in_billing_retry) in the local mirror"
	}
	return view, nil
}

// loadUnpaidInvoicesByCustomer returns customer-id → count of unpaid invoices
// and customer-id → summed gross amount of those unpaid invoices. An invoice is
// unpaid when its paid_at is missing/zero.
func loadUnpaidInvoicesByCustomer(db *store.Store) (counts map[string]int, amounts map[string]float64) {
	counts = map[string]int{}
	amounts = map[string]float64{}
	rows, err := db.Query(`SELECT customers_id, data FROM "invoices" LIMIT 1000000`)
	if err != nil {
		return counts, amounts
	}
	defer rows.Close()
	for rows.Next() {
		var custID sql.NullString
		var data sql.NullString
		if rows.Scan(&custID, &data) != nil || !data.Valid {
			continue
		}
		var obj map[string]any
		if json.Unmarshal([]byte(data.String), &obj) != nil {
			continue
		}
		paidAt := rcEpochMSToTime(obj["paid_at"])
		if !paidAt.IsZero() {
			continue // paid — not a dunning candidate
		}
		cid := custID.String
		if cid == "" {
			cid = toStringRC(obj["customer_id"])
		}
		if cid == "" {
			continue
		}
		counts[cid]++
		amounts[cid] += monetaryGrossUSD(obj["total_amount"])
	}
	if err := rows.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: iterating invoices failed (%v); dunning totals may be incomplete\n", err)
	}
	return counts, amounts
}
