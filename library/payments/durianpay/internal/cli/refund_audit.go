// Copyright 2026 ardihanan and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/durianpay/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/payments/durianpay/internal/store"
	"github.com/spf13/cobra"
)

// pp:data-source local

type localRefund struct {
	ID        string  `json:"id"`
	PaymentID string  `json:"payment_id"`
	Amount    float64 `json:"amount"`
	Status    string  `json:"status"`
}

type overRefund struct {
	PaymentID     string  `json:"payment_id"`
	PaymentAmount float64 `json:"payment_amount"`
	RefundTotal   float64 `json:"refund_total"`
	RefundCount   int     `json:"refund_count"`
}

type duplicateRefund struct {
	PaymentID string   `json:"payment_id"`
	Amount    float64  `json:"amount"`
	RefundIDs []string `json:"refund_ids"`
}

type refundAuditResult struct {
	ScannedRefunds        int               `json:"scanned_refunds"`
	ScannedPayments       int               `json:"scanned_payments"`
	OverRefunds           []overRefund      `json:"over_refunds"`
	RefundsWithoutPayment []localRefund     `json:"refunds_without_payment"`
	DuplicateRefunds      []duplicateRefund `json:"duplicate_refunds"`
	Window                string            `json:"window"`
	Note                  string            `json:"note,omitempty"`
}

// classifyRefundAudit is the pure logic over in-memory slices. paymentAmounts
// maps payment_id -> amount; a missing key means the payment row is absent.
func classifyRefundAudit(refunds []localRefund, paymentAmounts map[string]float64, scannedPayments int, window string) refundAuditResult {
	res := refundAuditResult{
		ScannedRefunds:        len(refunds),
		ScannedPayments:       scannedPayments,
		OverRefunds:           make([]overRefund, 0),
		RefundsWithoutPayment: make([]localRefund, 0),
		DuplicateRefunds:      make([]duplicateRefund, 0),
		Window:                window,
	}

	totalByPayment := map[string]float64{}
	countByPayment := map[string]int{}
	// dup key: payment_id + "|" + amount -> refund ids
	dupGroups := map[string][]string{}
	var dupOrder []string

	for _, r := range refunds {
		if r.PaymentID == "" {
			res.RefundsWithoutPayment = append(res.RefundsWithoutPayment, r)
			continue
		}
		if _, ok := paymentAmounts[r.PaymentID]; !ok {
			res.RefundsWithoutPayment = append(res.RefundsWithoutPayment, r)
			continue
		}
		totalByPayment[r.PaymentID] += r.Amount
		countByPayment[r.PaymentID]++
		dk := r.PaymentID + "|" + strconv.FormatFloat(r.Amount, 'f', -1, 64)
		if _, seen := dupGroups[dk]; !seen {
			dupOrder = append(dupOrder, dk)
		}
		dupGroups[dk] = append(dupGroups[dk], r.ID)
	}

	// Over-refunds: total refunded exceeds the payment amount.
	for pid, total := range totalByPayment {
		amt := paymentAmounts[pid]
		if total > amt {
			res.OverRefunds = append(res.OverRefunds, overRefund{
				PaymentID:     pid,
				PaymentAmount: amt,
				RefundTotal:   total,
				RefundCount:   countByPayment[pid],
			})
		}
	}

	// Duplicate refunds: 2+ refunds with same payment_id and same amount.
	for _, dk := range dupOrder {
		ids := dupGroups[dk]
		if len(ids) < 2 {
			continue
		}
		sep := strings.LastIndex(dk, "|")
		pid := dk[:sep]
		amt, _ := strconv.ParseFloat(dk[sep+1:], 64)
		res.DuplicateRefunds = append(res.DuplicateRefunds, duplicateRefund{
			PaymentID: pid,
			Amount:    amt,
			RefundIDs: ids,
		})
	}
	return res
}

func queryLocalRefunds(db *store.Store, cutoff time.Time) ([]localRefund, error) {
	rows, err := db.DB().Query(
		`SELECT id,
		        json_extract(data,'$.payment_id'),
		        json_extract(data,'$.amount'),
		        json_extract(data,'$.status'),
		        ` + reconcileTimeExpr + `
		   FROM resources WHERE resource_type = 'refunds'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []localRefund
	for rows.Next() {
		var id string
		var paymentID, amount, status, created sql.NullString
		if err := rows.Scan(&id, &paymentID, &amount, &status, &created); err != nil {
			return nil, err
		}
		if !withinWindow(created, cutoff) {
			continue
		}
		out = append(out, localRefund{ID: id, PaymentID: nullStr(paymentID), Amount: nullFloat(amount), Status: nullStr(status)})
	}
	return out, rows.Err()
}

// queryPaymentAmounts returns payment_id -> amount for all payment rows.
func queryPaymentAmounts(db *store.Store) (map[string]float64, error) {
	rows, err := db.DB().Query(
		`SELECT id, json_extract(data,'$.amount') FROM resources WHERE resource_type = 'payments'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]float64{}
	for rows.Next() {
		var id string
		var amount sql.NullString
		if err := rows.Scan(&id, &amount); err != nil {
			return nil, err
		}
		out[id] = nullFloat(amount)
	}
	return out, rows.Err()
}

func newNovelRefundAuditCmd(flags *rootFlags) *cobra.Command {
	var flagSince string
	var dbPath string

	cmd := &cobra.Command{
		Use:         "refund-audit",
		Short:       "Flag refunds that exceed their source payment, target a missing payment, or duplicate an earlier refund.",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "--since=30d"},
		Long: `Audit locally synced refunds against payments. Flags three problems:
  - over-refunds: total refunded for a payment exceeds the payment amount
  - refunds whose payment row is missing locally
  - duplicate refunds (2+ refunds with the same payment_id and amount)

Reads only the local store (sync refunds,payments first). Offline.`,
		Example: strings.TrimLeft(`
  durianpay-pp-cli refund-audit
  durianpay-pp-cli refund-audit --since=7d --json`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := rejectLiveDataSource(cmd, flags, "refund-audit"); err != nil {
				return err
			}
			window, err := cliutil.ParseDurationLoose(flagSince)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid --since %q: %w", flagSince, err))
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.ErrOrStderr(), "dry-run: would query local store (refunds, payments)")
				return nil
			}
			db, err := openLocalStore(dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			hintIfUnsynced(cmd, db, "refunds")
			hintIfUnsynced(cmd, db, "payments")
			hintIfStale(cmd, db, "refunds", flags.maxAge)
			hintIfStale(cmd, db, "payments", flags.maxAge)

			cutoff := time.Now().Add(-window)
			refunds, err := queryLocalRefunds(db, cutoff)
			if err != nil {
				return fmt.Errorf("querying refunds: %w", err)
			}
			payments, err := queryPaymentAmounts(db)
			if err != nil {
				return fmt.Errorf("querying payments: %w", err)
			}

			res := classifyRefundAudit(refunds, payments, len(payments), flagSince)
			if len(refunds) == 0 && len(payments) == 0 {
				res.Note = "no local refunds or payments; run 'durianpay-pp-cli sync --resources refunds,payments --since 30d'"
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}
			return printRefundAuditTable(cmd, res)
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "30d", "Only consider refunds created within this window (e.g. 30d, 24h, 4w)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to the local SQLite store (default: standard data dir)")
	return cmd
}

func printRefundAuditTable(cmd *cobra.Command, res refundAuditResult) error {
	out := cmd.OutOrStdout()
	if res.Note != "" {
		fmt.Fprintln(out, res.Note)
		return nil
	}
	fmt.Fprintf(out, "window=%s  scanned: %d refunds, %d payments\n", res.Window, res.ScannedRefunds, res.ScannedPayments)
	fmt.Fprintf(out, "over-refunds: %d\n", len(res.OverRefunds))
	for _, o := range res.OverRefunds {
		fmt.Fprintf(out, "  payment=%s payment_amount=%s refund_total=%s refunds=%d\n",
			o.PaymentID,
			strconv.FormatFloat(o.PaymentAmount, 'f', -1, 64),
			strconv.FormatFloat(o.RefundTotal, 'f', -1, 64),
			o.RefundCount)
	}
	fmt.Fprintf(out, "refunds without payment: %d\n", len(res.RefundsWithoutPayment))
	for _, r := range res.RefundsWithoutPayment {
		fmt.Fprintf(out, "  %s payment_id=%q amount=%s\n", r.ID, r.PaymentID, strconv.FormatFloat(r.Amount, 'f', -1, 64))
	}
	fmt.Fprintf(out, "duplicate refunds: %d\n", len(res.DuplicateRefunds))
	for _, d := range res.DuplicateRefunds {
		fmt.Fprintf(out, "  payment=%s amount=%s refund_ids=%s\n",
			d.PaymentID, strconv.FormatFloat(d.Amount, 'f', -1, 64), strings.Join(d.RefundIDs, ","))
	}
	return nil
}
