// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// Collection / dunning queue: per-customer outstanding + overdue + next action.
// pp:data-source local
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/quentli/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/payments/quentli/internal/store"
	"github.com/spf13/cobra"
)

type dunningCustomer struct {
	ID          string `json:"id"`
	Name        string `json:"name,omitempty"`
	Email       string `json:"email,omitempty"`
	PhoneNumber string `json:"phone_number,omitempty"`
}

type dunningRow struct {
	Customer             dunningCustomer `json:"customer"`
	OutstandingTotal     float64         `json:"outstanding_total"`
	OutstandingFormatted string          `json:"outstanding_formatted"`
	OverdueTotal         float64         `json:"overdue_total"`
	OverdueFormatted     string          `json:"overdue_formatted"`
	InvoiceCount         int             `json:"invoice_count"`
	MaxDaysOverdue       int             `json:"max_days_overdue"`
	OldestDueDate        string          `json:"oldest_due_date,omitempty"`
	Currency             string          `json:"currency"`
	NextAction           string          `json:"next_action"`
}

// dunningGroupKey groups outstanding rows by customer AND currency so that
// amounts in different currencies are never summed under a single label.
// currency is always the normalizeCurrency form, so an invoice with no currency
// and one explicitly in MXN land in the same bucket instead of producing two
// rows (and two footer lines) that both render as "MXN".
type dunningGroupKey struct {
	customer string
	currency string
}

type invoice struct {
	ID          string  `json:"id"`
	CustomerID  string  `json:"customerId"`
	TotalAmount float64 `json:"totalAmount"`
	AmountPaid  float64 `json:"amountPaid"`
	Currency    string  `json:"currency"`
	DueDate     string  `json:"dueDate"`
	IsPaid      bool    `json:"isPaid"`
}

type customer struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Email       string `json:"email"`
	PhoneNumber string `json:"phoneNumber"`
}

func newNovelDunningCmd(flags *rootFlags) *cobra.Command {
	var flagSince string
	var dbPath string

	cmd := &cobra.Command{
		Use:         "dunning",
		Short:       "See every customer with an outstanding or overdue invoice and the next collection action",
		Example:     "  quentli-pp-cli dunning --since 1w --json",
		Long:        "For the whole-portfolio collection queue. Use 'customer balance <id>' for a single customer's drill-down.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "dunning")
			}
			var since time.Duration
			if flagSince != "" {
				parsed, err := cliutil.ParseDurationLoose(flagSince)
				if err != nil {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("invalid --since %q: %w", flagSince, err))
				}
				since = parsed
			}
			if dbPath == "" {
				dbPath = defaultDBPath("quentli-pp-cli")
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: quentli-pp-cli sync --resources customers,invoices --db %s\n", dbPath, dbPath)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), make([]dunningRow, 0), flags)
				}
				return nil
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()
			hintIfUnsynced(cmd, db, "invoices")

			invs, err := loadAll[invoice](db, "invoices")
			if err != nil {
				return err
			}
			customers, err := loadAll[customer](db, "customers")
			if err != nil {
				return err
			}
			cusMap := make(map[string]customer, len(customers))
			for _, c := range customers {
				cusMap[c.ID] = c
			}

			now := time.Now().UTC()
			group := map[dunningGroupKey]*dunningRow{}
			for _, inv := range invs {
				if inv.IsPaid {
					continue
				}
				outstanding := inv.TotalAmount - inv.AmountPaid
				if outstanding <= 0 {
					continue
				}
				due := cliutil.ParseStoredTime(inv.DueDate)
				if due.IsZero() {
					continue
				}
				if since > 0 && now.Sub(due) > since {
					continue
				}
				cur := normalizeCurrency(inv.Currency)
				key := dunningGroupKey{customer: inv.CustomerID, currency: cur}
				tr := group[key]
				if tr == nil {
					tr = &dunningRow{Currency: cur}
					tr.Customer.ID = inv.CustomerID
					if c, ok := cusMap[inv.CustomerID]; ok {
						tr.Customer.Name = c.Name
						tr.Customer.Email = c.Email
						tr.Customer.PhoneNumber = c.PhoneNumber
					}
					group[key] = tr
				}
				tr.OutstandingTotal += outstanding
				tr.InvoiceCount++
				if due.Before(now) {
					days := int(now.Sub(due).Hours() / 24)
					if days > 0 {
						tr.OverdueTotal += outstanding
						if days > tr.MaxDaysOverdue {
							tr.MaxDaysOverdue = days
						}
					}
				}
				if tr.OldestDueDate == "" || due.Before(cliutil.ParseStoredTime(tr.OldestDueDate)) {
					tr.OldestDueDate = inv.DueDate
				}
			}
			rows := make([]dunningRow, 0, len(group))
			for _, tr := range group {
				tr.OutstandingFormatted = formatMoneyMinor(tr.OutstandingTotal, tr.Currency)
				tr.OverdueFormatted = formatMoneyMinor(tr.OverdueTotal, tr.Currency)
				tr.NextAction = "send_reminder"
				if tr.MaxDaysOverdue >= 3 {
					tr.NextAction = "retry_payment_or_resend_link"
				}
				rows = append(rows, *tr)
			}
			sort.Slice(rows, func(i, j int) bool { return rows[i].OutstandingTotal > rows[j].OutstandingTotal })

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No outstanding invoices in the local mirror.")
				return nil
			}
			table := make([]map[string]any, 0, len(rows))
			for _, r := range rows {
				table = append(table, map[string]any{
					"customer":    r.Customer.Name,
					"email":       r.Customer.Email,
					"outstanding": r.OutstandingFormatted,
					"overdue":     r.OverdueFormatted,
					"invoices":    r.InvoiceCount,
					"next_action": r.NextAction,
				})
			}
			if err := printAutoTable(cmd.OutOrStdout(), table); err != nil {
				return err
			}
			totByCur := map[string]float64{}
			for _, r := range rows {
				totByCur[r.Currency] += r.OutstandingTotal
			}
			// One customer may span several currency rows; report distinct customers.
			customerSet := map[string]struct{}{}
			for _, r := range rows {
				customerSet[r.Customer.ID] = struct{}{}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\nTotal outstanding across %d customers:\n", len(customerSet))
			currencies := make([]string, 0, len(totByCur))
			for c := range totByCur {
				currencies = append(currencies, c)
			}
			sort.Strings(currencies)
			for _, c := range currencies {
				// Keys are already normalized at grouping time, so each currency
				// appears exactly once here.
				fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", formatMoneyMinor(totByCur[c], c))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "", "only include invoices with due dates within this window (e.g. 7d, 4w)")
	cmd.Flags().StringVar(&dbPath, "db", "", "path to the local database")
	return cmd
}

// loadAll reads every row of a resource type into T.
func loadAll[T any](db *store.Store, resourceType string) ([]T, error) {
	raw, err := db.List(resourceType, -1)
	if err != nil {
		return nil, fmt.Errorf("loading %s: %w", resourceType, err)
	}
	out := make([]T, 0, len(raw))
	for _, r := range raw {
		var v T
		if err := json.Unmarshal(r, &v); err != nil {
			continue
		}
		out = append(out, v)
	}
	return out, nil
}
