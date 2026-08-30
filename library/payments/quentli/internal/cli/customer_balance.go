// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// Customer balance drill-down: single-customer financial snapshot from the local mirror.
// pp:data-source local
package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/payments/quentli/internal/store"
	"github.com/spf13/cobra"
)

type cbInvoice struct {
	ID          string  `json:"id"`
	CustomerID  string  `json:"customerId"`
	TotalAmount float64 `json:"totalAmount"`
	AmountPaid  float64 `json:"amountPaid"`
	Currency    string  `json:"currency"`
	IsPaid      bool    `json:"isPaid"`
}

type cbSubscription struct {
	ID         string `json:"id"`
	CustomerID string `json:"customerId"`
	Status     string `json:"status"`
	IsActive   bool   `json:"isActive"`
}

type cbPayment struct {
	ID          string  `json:"id"`
	CustomerID  string  `json:"customerId"`
	Amount      float64 `json:"amount"`
	Currency    string  `json:"currency"`
	Status      string  `json:"status"`
	IsCompleted bool    `json:"isCompleted"`
}

type cbTaxInvoice struct {
	ID         string `json:"id"`
	CustomerID string `json:"customerId"`
	Status     string `json:"status"`
}

type cbSnapshot struct {
	CustomerID           string  `json:"customer_id"`
	CustomerName         string  `json:"customer_name,omitempty"`
	Email                string  `json:"email,omitempty"`
	Outstanding          float64 `json:"outstanding"`
	OutstandingFormatted string  `json:"outstanding_formatted"`
	PaidTotal            float64 `json:"paid_total"`
	PaidTotalFormatted   string  `json:"paid_total_formatted"`
	ActiveSubscriptions  int     `json:"active_subscriptions"`
	TotalSubscriptions   int     `json:"total_subscriptions"`
	ValidTaxInvoices     int     `json:"valid_tax_invoices"`
	Currency             string  `json:"currency"`
	Invoices             int     `json:"invoices"`
	OpenInvoices         int     `json:"open_invoices"`
}

func newNovelCustomerBalanceCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:         "balance <customerId>",
		Short:       "Render a one-screen financial snapshot for a single customer from the local mirror",
		Example:     "  quentli-pp-cli customer balance cus_abc123 --json",
		Long:        "For one customer's financial snapshot. Use 'dunning' for the whole-portfolio collection queue.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("customer balance requires a customer id (e.g. quentli-pp-cli customer balance cus_abc123)"))
			}
			cid := strings.TrimSpace(args[0])
			if dryRunOK(flags) {
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					env := map[string]any{
						"dry_run":     true,
						"action":      "customer balance",
						"customer_id": cid,
						"would":       "render financial snapshot for customer " + cid,
					}
					return printJSONFiltered(cmd.OutOrStdout(), env, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "would render financial snapshot for customer %s\n", cid)
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("quentli-pp-cli")
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: quentli-pp-cli sync --resources customers,invoices,subscriptions,payments,tax-invoices --db %s\n", dbPath, dbPath)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), cbSnapshot{CustomerID: cid}, flags)
				}
				return nil
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()
			hintIfUnsynced(cmd, db, "invoices")

			invs, err := loadAll[cbInvoice](db, "invoices")
			if err != nil {
				return err
			}
			subs, err := loadAll[cbSubscription](db, "subscriptions")
			if err != nil {
				return err
			}
			pays, err := loadAll[cbPayment](db, "payments")
			if err != nil {
				return err
			}
			txis, err := loadAll[cbTaxInvoice](db, "tax-invoices")
			if err != nil {
				return err
			}

			snap := cbSnapshot{CustomerID: cid, Currency: "MXN"}
			var outstanding float64
			var paid float64
			for _, inv := range invs {
				if inv.CustomerID != cid {
					continue
				}
				snap.Invoices++
				if inv.Currency != "" {
					snap.Currency = inv.Currency
				}
				if inv.IsPaid {
					paid += inv.AmountPaid
				} else {
					outstanding += inv.TotalAmount - inv.AmountPaid
					snap.OpenInvoices++
				}
			}
			for _, p := range pays {
				if p.CustomerID == cid && p.IsCompleted {
					paid += p.Amount
				}
			}
			for _, s := range subs {
				if s.CustomerID != cid {
					continue
				}
				snap.TotalSubscriptions++
				if s.IsActive || strings.EqualFold(s.Status, "ACTIVE") {
					snap.ActiveSubscriptions++
				}
			}
			for _, tx := range txis {
				if tx.CustomerID == cid && strings.EqualFold(tx.Status, "VALID") {
					snap.ValidTaxInvoices++
				}
			}
			snap.Outstanding = outstanding
			snap.OutstandingFormatted = formatMoneyMinor(outstanding, snap.Currency)
			snap.PaidTotal = paid
			snap.PaidTotalFormatted = formatMoneyMinor(paid, snap.Currency)

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), snap, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Customer %s\n", cid)
			fmt.Fprintf(cmd.OutOrStdout(), "  Outstanding:           %s\n", snap.OutstandingFormatted)
			fmt.Fprintf(cmd.OutOrStdout(), "  Paid total:            %s\n", snap.PaidTotalFormatted)
			fmt.Fprintf(cmd.OutOrStdout(), "  Invoices:              %d (%d open)\n", snap.Invoices, snap.OpenInvoices)
			fmt.Fprintf(cmd.OutOrStdout(), "  Active subscriptions:  %d / %d\n", snap.ActiveSubscriptions, snap.TotalSubscriptions)
			fmt.Fprintf(cmd.OutOrStdout(), "  Valid tax invoices:    %d\n", snap.ValidTaxInvoices)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "path to the local database")
	return cmd
}
