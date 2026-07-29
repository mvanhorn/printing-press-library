// Copyright 2026 alon-auto and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: customer summary — one local join for a customer's whole
// picture: invoiced totals with age buckets, open orders, recent invoices.

package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"database/sql"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/priority/internal/priorityx"
	"github.com/mvanhorn/printing-press-library/library/commerce/priority/internal/store"
)

// pp:data-source local

type summaryInvoice struct {
	IVNum   string  `json:"ivnum"`
	IVDate  string  `json:"ivdate"`
	Total   float64 `json:"total"`
	AgeDays int     `json:"age_days"`
}

type summaryOrder struct {
	OrdName string  `json:"ordname"`
	Status  string  `json:"status"`
	Date    string  `json:"date"`
	Total   float64 `json:"total"`
}

type customerSummaryView struct {
	CustName       string             `json:"custname"`
	CustDes        string             `json:"custdes,omitempty"`
	Status         string             `json:"status,omitempty"`
	InvoicedTotal  float64            `json:"invoiced_total"`
	InvoiceCount   int                `json:"invoice_count"`
	AgingBuckets   map[string]float64 `json:"aging_buckets_by_invoice_date"`
	OpenOrders     []summaryOrder     `json:"open_orders"`
	RecentInvoices []summaryInvoice   `json:"recent_invoices"`
	LastActivity   string             `json:"last_activity,omitempty"`
	Note           string             `json:"note,omitempty"`
}

func newNovelCustomerSummaryCmd(flags *rootFlags) *cobra.Command {
	var recent int
	cmd := &cobra.Command{
		Use:   "summary <custname>",
		Short: "One command for a customer's whole picture: invoiced totals, age buckets, open orders, recent invoices",
		Long: strings.Trim(`
Use this command for one customer's full picture before a call.
Do NOT use it for ranked debtor lists across customers; use 'debtors' instead.
Do NOT use it for book-wide bucket totals; use 'aging' instead.

Age buckets are computed from invoice dates of synced AINVOICES rows. Priority's
OData surface does not expose payment application, so buckets reflect invoice
age, not open-AR balances.`, "\n"),
		Example: strings.Trim(`
  priority-pp-cli customer summary 1011 --agent
  priority-pp-cli customer summary "#098" --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "custname=1011", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would join synced customers, invoices, and orders for one customer")
				return nil
			}
			if len(args) < 1 || strings.TrimSpace(args[0]) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("customer number is required"))
			}
			if recent < 0 {
				recent = 0
			}
			cust := strings.TrimSpace(args[0])
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			dbPath := defaultDBPath("priority-pp-cli")
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: priority-pp-cli sync --resources customers,invoices,orders --db %s\n", dbPath, dbPath)
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
				}
				return nil
			}
			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			if !hintIfUnsynced(cmd, db, "invoices") {
				hintIfStale(cmd, db, "invoices", flags.maxAge)
			}

			view := customerSummaryView{CustName: cust, AgingBuckets: map[string]float64{}}
			for _, b := range priorityx.AgeBuckets {
				view.AgingBuckets[b] = 0
			}

			var des, stat sql.NullString
			err = db.DB().QueryRowContext(ctx,
				`SELECT custdes, statdes FROM "customers" WHERE custname = ?`, cust).Scan(&des, &stat)
			if err != nil && err != sql.ErrNoRows {
				return err
			}
			view.CustDes = nullStr(des)
			view.Status = nullStr(stat)

			// Invoices (drain fully before next query).
			invRows, err := db.DB().QueryContext(ctx,
				`SELECT COALESCE(ivnum,''), COALESCE(ivdate,''), COALESCE(totprice,0) FROM "invoices" WHERE custname = ? ORDER BY ivdate DESC`, cust)
			if err != nil {
				return err
			}
			now := time.Now().UTC()
			var invoices []summaryInvoice
			for invRows.Next() {
				var inv summaryInvoice
				if err := invRows.Scan(&inv.IVNum, &inv.IVDate, &inv.Total); err != nil {
					_ = invRows.Close()
					return err
				}
				invoices = append(invoices, inv)
			}
			if err := invRows.Err(); err != nil {
				_ = invRows.Close()
				return err
			}
			_ = invRows.Close()
			for i := range invoices {
				inv := &invoices[i]
				if t, err := time.Parse(time.RFC3339, inv.IVDate); err == nil {
					inv.AgeDays = int(now.Sub(t).Hours() / 24)
				}
				if b := priorityx.BucketFor(inv.IVDate, now); b >= 0 {
					view.AgingBuckets[priorityx.AgeBuckets[b]] += inv.Total
				}
				view.InvoicedTotal += inv.Total
			}
			view.InvoiceCount = len(invoices)
			if len(invoices) > recent {
				view.RecentInvoices = invoices[:recent]
			} else {
				view.RecentInvoices = invoices
			}
			if view.RecentInvoices == nil {
				view.RecentInvoices = []summaryInvoice{}
			}

			// Open orders.
			ordRows, err := db.DB().QueryContext(ctx,
				`SELECT COALESCE(ordname,''), COALESCE(ordstatusdes,''), COALESCE(curdate,''), COALESCE(qprice,0) FROM "orders" WHERE custname = ? ORDER BY curdate DESC`, cust)
			if err != nil {
				return err
			}
			var allOrders []summaryOrder
			for ordRows.Next() {
				var o summaryOrder
				if err := ordRows.Scan(&o.OrdName, &o.Status, &o.Date, &o.Total); err != nil {
					_ = ordRows.Close()
					return err
				}
				allOrders = append(allOrders, o)
			}
			if err := ordRows.Err(); err != nil {
				_ = ordRows.Close()
				return err
			}
			_ = ordRows.Close()
			for _, o := range allOrders {
				if !closedStatuses[strings.ToLower(o.Status)] {
					view.OpenOrders = append(view.OpenOrders, o)
				}
			}
			if view.OpenOrders == nil {
				view.OpenOrders = []summaryOrder{}
			}

			// Last activity: newest of invoice date / order date.
			var dates []string
			if len(invoices) > 0 {
				dates = append(dates, invoices[0].IVDate)
			}
			if len(allOrders) > 0 {
				dates = append(dates, allOrders[0].Date)
			}
			sort.Strings(dates)
			if len(dates) > 0 {
				view.LastActivity = dates[len(dates)-1]
			}

			if view.CustDes == "" && view.InvoiceCount == 0 && len(allOrders) == 0 {
				view.Note = fmt.Sprintf("customer %q not found in the local mirror; check the number with: priority-pp-cli search %q --type customers, or sync with: priority-pp-cli sync --resources customers,invoices,orders", cust, cust)
			}

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s  %s  [%s]\n", view.CustName, view.CustDes, view.Status)
			fmt.Fprintf(cmd.OutOrStdout(), "invoiced total: %.2f across %d invoices\n", view.InvoicedTotal, view.InvoiceCount)
			fmt.Fprint(cmd.OutOrStdout(), "aging by invoice date:")
			for _, b := range priorityx.AgeBuckets {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s: %.2f", b, view.AgingBuckets[b])
			}
			fmt.Fprintln(cmd.OutOrStdout())
			fmt.Fprintf(cmd.OutOrStdout(), "open orders: %d\n", len(view.OpenOrders))
			for i, o := range view.OpenOrders {
				if i >= 5 {
					fmt.Fprintf(cmd.OutOrStdout(), "  ... and %d more\n", len(view.OpenOrders)-5)
					break
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  %s  %s  %s  %.2f\n", o.OrdName, o.Status, o.Date, o.Total)
			}
			if view.LastActivity != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "last activity: %s\n", view.LastActivity)
			}
			if view.Note != "" {
				fmt.Fprintln(cmd.OutOrStdout(), view.Note)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&recent, "recent", 5, "how many recent invoices to include")
	return cmd
}
