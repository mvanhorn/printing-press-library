// Copyright 2026 chris-rodriguez. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/stripe/internal/store"

	"github.com/spf13/cobra"
)

type btRow struct {
	ID             string `json:"id"`
	Amount         int64  `json:"amount"`
	SourceChargeID string `json:"source_charge_id,omitempty"`
	CustomerID     string `json:"customer_id,omitempty"`
	CustomerEmail  string `json:"customer_email,omitempty"`
}

type payoutReport struct {
	PayoutID            string  `json:"payout_id"`
	Amount              int64   `json:"amount"`
	ArrivalDate         int64   `json:"arrival_date,omitempty"`
	BalanceTransactions []btRow `json:"balance_transactions"`
	TotalMatched        int64   `json:"total_matched"`
	TotalUnmatched      int64   `json:"total_unmatched"`
}

func newPayoutReconcileCmd(flags *rootFlags) *cobra.Command {
	var sinceStr string
	var dbPath string
	var asCSV bool

	cmd := &cobra.Command{
		Use:   "payout-reconcile [payout-id]",
		Short: "Join payout > balance_transactions > charges > customers from local SQLite",
		Long: `Walk a payout (or every payout in a window) down to its balance
transactions, source charges, and customer rows. Reads exclusively from the
local mirror — issues separate SELECTs and joins in-memory rather than
relying on SQLite JSON joins.`,
		Example: `  # One specific payout
  stripe-pp-cli payout-reconcile po_1Hh1l2eZvKYlo2Cp

  # Last 7 days, CSV
  stripe-pp-cli payout-reconcile --since 168h --csv`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			path := transcendenceDBPath(dbPath)
			db, err := store.OpenReadOnly(path)
			if err != nil {
				return configErr(fmt.Errorf("opening local database (%s): %w\nRun 'stripe-pp-cli sync' first.", path, err))
			}
			defer db.Close()

			since, err := parseHumanDuration(sinceStr)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --since: %w", err))
			}
			if since == 0 {
				since = 7 * 24 * time.Hour
			}
			payoutIDs, err := selectPayoutIDs(db.DB(), args, since)
			if err != nil {
				return apiErr(err)
			}
			if len(payoutIDs) == 0 {
				if len(args) > 0 {
					return notFoundErr(fmt.Errorf("payout %s not found in local store", args[0]))
				}
				// No payouts in window: still emit an empty array so JSON consumers don't choke.
				return printJSONFiltered(cmd.OutOrStdout(), []payoutReport{}, flags)
			}

			reports := make([]payoutReport, 0, len(payoutIDs))
			for _, pid := range payoutIDs {
				r, err := buildPayoutReport(db.DB(), pid)
				if err != nil {
					return apiErr(err)
				}
				reports = append(reports, r)
			}

			if asCSV {
				return writePayoutCSV(cmd.OutOrStdout(), reports)
			}

			if len(args) > 0 {
				return printJSONFiltered(cmd.OutOrStdout(), reports[0], flags)
			}
			return printJSONFiltered(cmd.OutOrStdout(), reports, flags)
		},
	}

	cmd.Flags().StringVar(&sinceStr, "since", "7d", "Window covered when no payout-id is given (e.g. 7d, 24h, 1w)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/stripe-pp-cli/data.db)")
	cmd.Flags().BoolVar(&asCSV, "csv", false, "Emit a flat CSV instead of nested JSON")

	return cmd
}

func selectPayoutIDs(db *sql.DB, args []string, since time.Duration) ([]string, error) {
	if len(args) > 0 {
		// Confirm presence so we can return notFoundErr cleanly.
		var id string
		err := db.QueryRow(`SELECT id FROM resources WHERE resource_type='payouts' AND id=?`, args[0]).Scan(&id)
		if err == sql.ErrNoRows {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		return []string{id}, nil
	}
	cutoff := time.Now().Add(-since).Unix()
	rows, err := db.Query(`SELECT id FROM resources WHERE resource_type='payouts'
		AND IFNULL(json_extract(data,'$.created'), 0) >= ? ORDER BY json_extract(data,'$.created') DESC`, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func buildPayoutReport(db *sql.DB, pid string) (payoutReport, error) {
	report := payoutReport{PayoutID: pid, BalanceTransactions: []btRow{}}

	// Pull payout data for amount and arrival_date.
	var pdata string
	if err := db.QueryRow(
		`SELECT data FROM resources WHERE resource_type='payouts' AND id=?`, pid,
	).Scan(&pdata); err == nil {
		raw := json.RawMessage(pdata)
		if v, ok := jsonGetInt(raw, "amount"); ok {
			report.Amount = v
		}
		if v, ok := jsonGetInt(raw, "arrival_date"); ok {
			report.ArrivalDate = v
		}
	}

	// Find balance_transactions tied to this payout.
	rows, err := db.Query(
		`SELECT id, data FROM resources WHERE resource_type='balance_transactions'
		 AND json_extract(data,'$.payout') = ?`, pid)
	if err != nil {
		return report, fmt.Errorf("querying balance_transactions: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id, data string
		if err := rows.Scan(&id, &data); err != nil {
			return report, err
		}
		raw := json.RawMessage(data)
		bt := btRow{ID: id}
		if v, ok := jsonGetInt(raw, "amount"); ok {
			bt.Amount = v
		}
		// Resolve source -> charge -> customer when 'source' looks like a charge id.
		if src, ok := jsonGet(raw, "source"); ok && len(src) > 3 && src[:3] == "ch_" {
			bt.SourceChargeID = src
			bt.CustomerID, bt.CustomerEmail = lookupChargeCustomer(db, src)
		}
		report.BalanceTransactions = append(report.BalanceTransactions, bt)
		if bt.SourceChargeID != "" && bt.CustomerID != "" {
			report.TotalMatched += bt.Amount
		} else {
			report.TotalUnmatched += bt.Amount
		}
	}
	return report, rows.Err()
}

func lookupChargeCustomer(db *sql.DB, chargeID string) (string, string) {
	var cdata string
	if err := db.QueryRow(
		`SELECT data FROM resources WHERE resource_type='charges' AND id=?`, chargeID,
	).Scan(&cdata); err != nil {
		return "", ""
	}
	raw := json.RawMessage(cdata)
	cid, _ := jsonGet(raw, "customer")
	if cid == "" {
		return "", ""
	}
	var custData string
	_ = db.QueryRow(
		`SELECT data FROM resources WHERE resource_type='customers' AND id=?`, cid,
	).Scan(&custData)
	if custData == "" {
		return cid, ""
	}
	email, _ := jsonGet(json.RawMessage(custData), "email")
	return cid, email
}

func writePayoutCSV(w io.Writer, reports []payoutReport) error {
	cw := csv.NewWriter(w)
	if err := cw.Write([]string{"payout_id", "bt_id", "bt_amount", "source_charge_id", "customer_id", "customer_email"}); err != nil {
		return err
	}
	for _, r := range reports {
		for _, bt := range r.BalanceTransactions {
			if err := cw.Write([]string{
				r.PayoutID, bt.ID, strconv.FormatInt(bt.Amount, 10),
				bt.SourceChargeID, bt.CustomerID, bt.CustomerEmail,
			}); err != nil {
				return err
			}
		}
	}
	cw.Flush()
	return cw.Error()
}
