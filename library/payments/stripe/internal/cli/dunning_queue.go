// Copyright 2026 chris-rodriguez. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/stripe/internal/store"

	"github.com/spf13/cobra"
)

type dunningRow struct {
	InvoiceID         string `json:"invoice_id"`
	CustomerID        string `json:"customer_id,omitempty"`
	CustomerEmail     string `json:"customer_email,omitempty"`
	AmountDue         int64  `json:"amount_due"`
	Currency          string `json:"currency,omitempty"`
	DaysOverdue       int    `json:"days_overdue"`
	Status            string `json:"status"`
	LastFailureReason string `json:"last_failure_reason,omitempty"`
	HostedInvoiceURL  string `json:"hosted_invoice_url,omitempty"`
}

func newDunningQueueCmd(flags *rootFlags) *cobra.Command {
	var ownerEmail string
	var limit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "dunning-queue",
		Short: "List invoices in past_due/uncollectible state with retry context",
		Long: `Surface every open or past-due invoice with a non-zero amount, ranked by
days overdue (oldest first). Pulls the last failure reason and hosted invoice
URL out of the local mirror — no live API calls.`,
		Example: `  # All accounts
  stripe-pp-cli dunning-queue --json

  # Just one customer
  stripe-pp-cli dunning-queue --owner alice@example.com --limit 20`,
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

			rows, err := readDunningInvoices(db.DB(), ownerEmail)
			if err != nil {
				return apiErr(err)
			}

			sort.SliceStable(rows, func(i, j int) bool {
				return rows[i].DaysOverdue > rows[j].DaysOverdue
			})

			if limit > 0 && len(rows) > limit {
				rows = rows[:limit]
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}

	cmd.Flags().StringVar(&ownerEmail, "owner", "", "Filter to invoices for a single customer email")
	cmd.Flags().IntVar(&limit, "limit", 0, "Cap output to top-N most overdue")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/stripe-pp-cli/data.db)")

	return cmd
}

func readDunningInvoices(db *sql.DB, ownerEmail string) ([]dunningRow, error) {
	q := `SELECT id, data FROM resources WHERE resource_type='invoices'
		AND json_extract(data,'$.status') IN ('open','past_due','uncollectible')
		AND CAST(IFNULL(json_extract(data,'$.amount_due'),0) AS INTEGER) > 0`
	args := []any{}
	if ownerEmail != "" {
		q += ` AND json_extract(data,'$.customer_email') = ?`
		args = append(args, ownerEmail)
	}
	rs, err := db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("querying invoices: %w", err)
	}
	defer rs.Close()

	now := time.Now()
	out := make([]dunningRow, 0)
	for rs.Next() {
		var id, data string
		if err := rs.Scan(&id, &data); err != nil {
			return nil, err
		}
		raw := json.RawMessage(data)
		row := dunningRow{InvoiceID: id}
		if v, ok := jsonGet(raw, "customer"); ok {
			row.CustomerID = v
		}
		if v, ok := jsonGet(raw, "customer_email"); ok {
			row.CustomerEmail = v
		}
		if v, ok := jsonGetInt(raw, "amount_due"); ok {
			row.AmountDue = v
		}
		if v, ok := jsonGet(raw, "currency"); ok {
			row.Currency = v
		}
		if v, ok := jsonGet(raw, "status"); ok {
			row.Status = v
		}
		if v, ok := jsonGet(raw, "hosted_invoice_url"); ok {
			row.HostedInvoiceURL = v
		}
		row.LastFailureReason = extractLastFailureReason(raw)
		row.DaysOverdue = computeDaysOverdue(raw, now)
		out = append(out, row)
	}
	return out, rs.Err()
}

// computeDaysOverdue derives days-overdue from the invoice JSON. We prefer
// 'due_date' (Stripe's authoritative field for invoices that have one set);
// fall back to 'period_end' for usage-based invoices that omit due_date;
// final fallback is 'created'. All three are unix epochs in Stripe.
func computeDaysOverdue(raw json.RawMessage, now time.Time) int {
	for _, field := range []string{"due_date", "period_end", "created"} {
		if ts, ok := jsonGetInt(raw, field); ok && ts > 0 {
			d := int(now.Sub(time.Unix(ts, 0)).Hours() / 24)
			if d < 0 {
				return 0
			}
			return d
		}
	}
	return 0
}

// extractLastFailureReason pulls the most informative string from
// last_finalization_error.message or last_finalization_error.code, with a
// fallback to the top-level invoice 'last_finalization_error' string when
// some integrations flatten the field.
func extractLastFailureReason(raw json.RawMessage) string {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return ""
	}
	if errBlob, ok := obj["last_finalization_error"]; ok {
		var inner map[string]json.RawMessage
		if json.Unmarshal(errBlob, &inner) == nil {
			for _, k := range []string{"message", "code", "decline_code"} {
				if v, ok := inner[k]; ok {
					var s string
					if json.Unmarshal(v, &s) == nil && s != "" {
						return s
					}
				}
			}
		}
		// Sometimes flattened.
		var s string
		if json.Unmarshal(errBlob, &s) == nil && s != "" {
			return s
		}
	}
	return ""
}
