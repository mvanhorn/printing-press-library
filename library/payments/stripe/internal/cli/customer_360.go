// Copyright 2026 chris-rodriguez. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/payments/stripe/internal/store"

	"github.com/spf13/cobra"
)

type customer360 struct {
	Customer            json.RawMessage   `json:"customer"`
	ActiveSubscriptions []json.RawMessage `json:"active_subscriptions"`
	RecentInvoices      []json.RawMessage `json:"recent_invoices"`
	PaymentMethods      []json.RawMessage `json:"payment_methods"`
	RecentCharges       []json.RawMessage `json:"recent_charges"`
	OpenDisputes        []json.RawMessage `json:"open_disputes"`
	LifetimeSpend       lifetimeSpend     `json:"lifetime_spend"`
}

type lifetimeSpend struct {
	Amount   int64  `json:"amount"`
	Currency string `json:"currency,omitempty"`
}

func newCustomer360Cmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "customer-360 <id-or-email>",
		Short: "One-shot dossier: customer + active subs + recent invoices + payment methods + recent charges + open disputes + lifetime spend",
		Example: `  # By ID
  stripe-pp-cli customer-360 cus_NffrFeUfNV2Hib

  # By email
  stripe-pp-cli customer-360 alice@example.com`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			path := transcendenceDBPath(dbPath)
			db, err := store.OpenReadOnly(path)
			if err != nil {
				return configErr(fmt.Errorf("opening local database (%s): %w\nRun 'stripe-pp-cli sync' first.", path, err))
			}
			defer db.Close()

			cid, custRaw, err := resolveCustomer(db.DB(), args[0])
			if err != nil {
				return apiErr(err)
			}
			if custRaw == nil {
				return notFoundErr(fmt.Errorf("customer %q not found in local store", args[0]))
			}

			dossier, err := buildCustomer360(db.DB(), cid, custRaw)
			if err != nil {
				return apiErr(err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), dossier, flags)
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/stripe-pp-cli/data.db)")

	return cmd
}

// resolveCustomer takes either a Stripe customer id (cus_…) or an email
// address and returns the matching customer id and JSON blob. Email lookup
// uses an exact json_extract match — FTS5 isn't reliable on email fragments.
func resolveCustomer(db *sql.DB, idOrEmail string) (string, json.RawMessage, error) {
	if strings.Contains(idOrEmail, "@") {
		var id, data string
		err := db.QueryRow(
			`SELECT id, data FROM resources WHERE resource_type='customers'
			 AND json_extract(data,'$.email')=? LIMIT 1`, idOrEmail,
		).Scan(&id, &data)
		if err == sql.ErrNoRows {
			return "", nil, nil
		}
		if err != nil {
			return "", nil, err
		}
		return id, json.RawMessage(data), nil
	}
	var data string
	err := db.QueryRow(
		`SELECT data FROM resources WHERE resource_type='customers' AND id=?`, idOrEmail,
	).Scan(&data)
	if err == sql.ErrNoRows {
		return "", nil, nil
	}
	if err != nil {
		return "", nil, err
	}
	return idOrEmail, json.RawMessage(data), nil
}

func buildCustomer360(db *sql.DB, cid string, custRaw json.RawMessage) (*customer360, error) {
	d := &customer360{
		Customer:            custRaw,
		ActiveSubscriptions: []json.RawMessage{},
		RecentInvoices:      []json.RawMessage{},
		PaymentMethods:      []json.RawMessage{},
		RecentCharges:       []json.RawMessage{},
		OpenDisputes:        []json.RawMessage{},
	}

	d.ActiveSubscriptions = selectJSON(db,
		`SELECT data FROM resources WHERE resource_type='subscriptions'
		 AND json_extract(data,'$.customer')=?
		 AND json_extract(data,'$.status') IN ('active','trialing','past_due')`,
		[]any{cid})

	d.RecentInvoices = selectJSON(db,
		`SELECT data FROM resources WHERE resource_type='invoices'
		 AND json_extract(data,'$.customer')=?
		 ORDER BY IFNULL(json_extract(data,'$.created'),0) DESC LIMIT 5`,
		[]any{cid})

	d.PaymentMethods = selectJSON(db,
		`SELECT data FROM resources WHERE resource_type='payment_methods'
		 AND json_extract(data,'$.customer')=?`,
		[]any{cid})

	d.RecentCharges = selectJSON(db,
		`SELECT data FROM resources WHERE resource_type='charges'
		 AND json_extract(data,'$.customer')=?
		 ORDER BY IFNULL(json_extract(data,'$.created'),0) DESC LIMIT 10`,
		[]any{cid})

	d.OpenDisputes = selectJSON(db,
		`SELECT data FROM resources WHERE resource_type='disputes'
		 AND json_extract(data,'$.customer')=?
		 AND json_extract(data,'$.status') NOT IN ('won','lost')`,
		[]any{cid})

	// Lifetime spend = sum of succeeded charges. Currency is taken from the
	// first charge — multi-currency customers get an asterisk in JSON via the
	// blank field, which is correct (we don't aggregate across currencies).
	var sum sql.NullInt64
	_ = db.QueryRow(
		`SELECT COALESCE(SUM(CAST(json_extract(data,'$.amount') AS INTEGER)),0)
		 FROM resources WHERE resource_type='charges'
		 AND json_extract(data,'$.customer')=?
		 AND json_extract(data,'$.status')='succeeded'`, cid,
	).Scan(&sum)
	if sum.Valid {
		d.LifetimeSpend.Amount = sum.Int64
	}
	var cur sql.NullString
	_ = db.QueryRow(
		`SELECT json_extract(data,'$.currency') FROM resources WHERE resource_type='charges'
		 AND json_extract(data,'$.customer')=? LIMIT 1`, cid,
	).Scan(&cur)
	if cur.Valid {
		d.LifetimeSpend.Currency = cur.String
	}

	return d, nil
}

func selectJSON(db *sql.DB, query string, args []any) []json.RawMessage {
	rows, err := db.Query(query, args...)
	if err != nil {
		return []json.RawMessage{}
	}
	defer rows.Close()
	out := make([]json.RawMessage, 0)
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err == nil {
			out = append(out, json.RawMessage(data))
		}
	}
	return out
}
