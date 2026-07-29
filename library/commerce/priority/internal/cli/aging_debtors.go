// Copyright 2026 alon-auto and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written AR analytics over the local mirror: book-wide invoice-date
// aging buckets and ranked debtors. Priority's OData surface exposes invoices
// (AINVOICES) but not payment application, so both commands present
// invoice-date aging, clearly labeled — not open-AR balances.

package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/priority/internal/priorityx"
	"github.com/mvanhorn/printing-press-library/library/commerce/priority/internal/store"
)

// pp:data-source local

type agingView struct {
	Buckets      map[string]float64 `json:"buckets_by_invoice_date"`
	InvoiceCount int                `json:"invoice_count"`
	Total        float64            `json:"total"`
	WindowDays   int                `json:"window_days"`
	Note         string             `json:"note,omitempty"`
}

func newAgingCmd(flags *rootFlags) *cobra.Command {
	var windowDays int
	cmd := &cobra.Command{
		Use:   "aging",
		Short: "Book-wide AR aging buckets (0-30/31-60/61-90/90+) by invoice date, from the local mirror",
		Long: strings.Trim(`
Buckets synced AINVOICES totals by invoice age. Priority's OData surface does
not expose payment application, so buckets reflect invoice age, not open-AR
balances — treat this as a collections triage view, not a ledger.
For one customer's picture use 'customer summary'; for a ranked list use 'debtors'.`, "\n"),
		Example: strings.Trim(`
  priority-pp-cli aging --agent
  priority-pp-cli aging --window-days 365 --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would bucket synced invoices by invoice age")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			dbPath := defaultDBPath("priority-pp-cli")
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: priority-pp-cli sync --resources invoices --db %s\n", dbPath, dbPath)
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
			view := agingView{Buckets: map[string]float64{}, WindowDays: windowDays}
			for _, b := range priorityx.AgeBuckets {
				view.Buckets[b] = 0
			}
			rows, err := db.DB().QueryContext(ctx, `SELECT COALESCE(ivdate,''), COALESCE(totprice,0) FROM "invoices"`)
			if err != nil {
				return err
			}
			now := time.Now().UTC()
			cutoff := now.AddDate(0, 0, -windowDays)
			for rows.Next() {
				var ivdate string
				var total float64
				if err := rows.Scan(&ivdate, &total); err != nil {
					_ = rows.Close()
					return err
				}
				t, perr := time.Parse(time.RFC3339, ivdate)
				if perr != nil {
					t, perr = time.Parse("2006-01-02", ivdate)
				}
				if perr == nil && t.Before(cutoff) {
					continue
				}
				if b := priorityx.BucketFor(ivdate, now); b >= 0 {
					view.Buckets[priorityx.AgeBuckets[b]] += total
					view.Total += total
					view.InvoiceCount++
				}
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return err
			}
			_ = rows.Close()
			if view.InvoiceCount == 0 {
				view.Note = "no synced invoices in the window; run: priority-pp-cli sync --resources invoices"
			}
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "invoice-date aging over %d invoices (last %d days), total %.2f\n", view.InvoiceCount, view.WindowDays, view.Total)
			w := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(w, "BUCKET\tTOTAL")
			for _, b := range priorityx.AgeBuckets {
				fmt.Fprintf(w, "%s\t%.2f\n", b, view.Buckets[b])
			}
			if err := w.Flush(); err != nil {
				return err
			}
			if view.Note != "" {
				fmt.Fprintln(cmd.OutOrStdout(), view.Note)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&windowDays, "window-days", 730, "only include invoices newer than this many days")
	return cmd
}

type debtorRow struct {
	CustName     string  `json:"custname"`
	CustDes      string  `json:"custdes,omitempty"`
	Invoiced     float64 `json:"invoiced_total"`
	InvoiceCount int     `json:"invoice_count"`
	Oldest       string  `json:"oldest_invoice,omitempty"`
}

type debtorsView struct {
	Rows       []debtorRow `json:"rows"`
	WindowDays int         `json:"window_days"`
	Note       string      `json:"note,omitempty"`
}

func newDebtorsCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var windowDays int
	cmd := &cobra.Command{
		Use:   "debtors",
		Short: "Customers ranked by invoiced totals in a window, from the local mirror",
		Long: strings.Trim(`
Ranks customers by synced AINVOICES totals (invoice-date window). Payment
application is not exposed via OData, so this ranks invoiced volume — the
collections call list, not a ledger of open balances.
For one customer's detail use 'customer summary'; for bucket totals use 'aging'.`, "\n"),
		Example: strings.Trim(`
  priority-pp-cli debtors --limit 10
  priority-pp-cli debtors --window-days 90 --agent`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would rank customers by invoiced totals")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			dbPath := defaultDBPath("priority-pp-cli")
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: priority-pp-cli sync --resources invoices,customers --db %s\n", dbPath, dbPath)
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
			cutoff := time.Now().UTC().AddDate(0, 0, -windowDays).Format(time.RFC3339)
			rows, err := db.DB().QueryContext(ctx,
				`SELECT i.custname, COALESCE(c.custdes, ''), SUM(COALESCE(i.totprice, 0)), COUNT(*), MIN(COALESCE(i.ivdate, ''))
				 FROM "invoices" i LEFT JOIN "customers" c ON c.custname = i.custname
				 WHERE i.custname IS NOT NULL AND i.custname != '' AND COALESCE(i.ivdate, '') >= ?
				 GROUP BY i.custname ORDER BY SUM(COALESCE(i.totprice, 0)) DESC LIMIT ?`, cutoff, limit)
			if err != nil {
				return err
			}
			view := debtorsView{WindowDays: windowDays}
			for rows.Next() {
				var r debtorRow
				if err := rows.Scan(&r.CustName, &r.CustDes, &r.Invoiced, &r.InvoiceCount, &r.Oldest); err != nil {
					_ = rows.Close()
					return err
				}
				view.Rows = append(view.Rows, r)
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return err
			}
			_ = rows.Close()
			sort.SliceStable(view.Rows, func(i, j int) bool { return view.Rows[i].Invoiced > view.Rows[j].Invoiced })
			if view.Rows == nil {
				view.Rows = []debtorRow{}
				view.Note = "no synced invoices in the window; run: priority-pp-cli sync --resources invoices,customers"
			}
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(view.Rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), view.Note)
				return nil
			}
			w := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(w, "CUSTOMER\tNAME\tINVOICED\tINVOICES\tOLDEST")
			for _, r := range view.Rows {
				fmt.Fprintf(w, "%s\t%s\t%.2f\t%d\t%s\n", r.CustName, truncate(r.CustDes, 30), r.Invoiced, r.InvoiceCount, r.Oldest)
			}
			return w.Flush()
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "maximum customers to return")
	cmd.Flags().IntVar(&windowDays, "window-days", 365, "only include invoices newer than this many days")
	return cmd
}
