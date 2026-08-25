// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// CFDI tax-invoice reconciliation: flag completed/refunded payments missing a valid SAT timbre.
// pp:data-source local
package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/quentli/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/payments/quentli/internal/store"
	"github.com/spf13/cobra"
)

type reconcilePayment struct {
	ID          string  `json:"id"`
	Amount      float64 `json:"amount"`
	Currency    string  `json:"currency"`
	Status      string  `json:"status"`
	IsCompleted bool    `json:"isCompleted"`
	Type        string  `json:"type"`
	CreatedAt   string  `json:"createdAt"`
}

type reconcileTaxInvoice struct {
	ID        string `json:"id"`
	PaymentID string `json:"paymentId"`
	Status    string `json:"status"`
	InvoiceID string `json:"invoiceId"`
}

type reconcileRow struct {
	PaymentID       string  `json:"payment_id"`
	Amount          float64 `json:"amount"`
	AmountFormatted string  `json:"amount_formatted"`
	Currency        string  `json:"currency"`
	CreatedAt       string  `json:"created_at"`
	Issue           string  `json:"issue"`
}

type reconcileView struct {
	Period             string         `json:"period"`
	CompletedPayments  int            `json:"completed_payments"`
	WithValidCFDI      int            `json:"with_valid_cfdi"`
	MissingValidCFDI   []reconcileRow `json:"missing_valid_cfdi"`
	RefundedUnresolved int            `json:"refunded_unresolved"`
}

func newNovelReconcileCmd(flags *rootFlags) *cobra.Command {
	var flagPeriod string
	var dbPath string
	cmd := &cobra.Command{
		Use:         "reconcile",
		Short:       "Cross-check completed or refunded payments and paid invoices against SAT tax-invoice status for timbre gaps",
		Example:     "  quentli-pp-cli reconcile --period 1m --json",
		Long:        "Use before SAT filing to catch completed payments missing a VALID CFDI instead of eyeballing two exported lists.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "reconcile")
			}
			var period time.Duration
			if flagPeriod != "" {
				parsed, err := cliutil.ParseDurationLoose(flagPeriod)
				if err != nil {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("invalid --period %q: %w", flagPeriod, err))
				}
				period = parsed
			}
			if dbPath == "" {
				dbPath = defaultDBPath("quentli-pp-cli")
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: quentli-pp-cli sync --resources payments,tax-invoices --db %s\n", dbPath, dbPath)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), reconcileView{}, flags)
				}
				return nil
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()
			hintIfUnsynced(cmd, db, "payments")

			dbg, err := loadAll[reconcilePayment](db, "payments")
			if err != nil {
				return err
			}
			txis, err := loadAll[reconcileTaxInvoice](db, "tax-invoices")
			if err != nil {
				return err
			}
			valid := map[string]bool{}
			for _, tx := range txis {
				if tx.Status == "VALID" {
					valid[tx.PaymentID] = true
				}
			}

			now := time.Now().UTC()
			var missing []reconcileRow
			completed := 0
			withValid := 0
			refundUnresolved := 0
			for _, p := range dbg {
				if period > 0 && p.CreatedAt != "" {
					if ct := cliutil.ParseStoredTime(p.CreatedAt); !ct.IsZero() && now.Sub(ct) > period {
						continue
					}
				}
				if p.IsCompleted || strings.EqualFold(p.Status, "COMPLETED") {
					completed++
					if valid[p.ID] {
						withValid++
					} else {
						missing = append(missing, reconcileRow{
							PaymentID:       p.ID,
							Amount:          p.Amount,
							AmountFormatted: formatMoneyMinor(p.Amount, p.Currency),
							Currency:        p.Currency,
							CreatedAt:       p.CreatedAt,
							Issue:           "no valid CFDI",
						})
					}
				}
				if strings.EqualFold(p.Type, "refund") || p.Amount < 0 {
					if !valid[p.ID] && !strings.EqualFold(p.Status, "CANCELED") {
						refundUnresolved++
					}
				}
			}
			sort.Slice(missing, func(i, j int) bool { return missing[i].Amount > missing[j].Amount })
			view := reconcileView{
				Period:             flagPeriod,
				CompletedPayments:  completed,
				WithValidCFDI:      withValid,
				MissingValidCFDI:   missing,
				RefundedUnresolved: refundUnresolved,
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Completed payments: %d | with valid CFDI: %d | missing: %d\n", completed, withValid, len(missing))
			for _, r := range missing {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s  %s  %s (pay %s)\n", r.AmountFormatted, r.Currency, r.Issue, r.PaymentID)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagPeriod, "period", "", "only consider payments in this window (e.g. 1m, 30d)")
	cmd.Flags().StringVar(&dbPath, "db", "", "path to the local database")
	return cmd
}
