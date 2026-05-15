// Copyright 2026 primiasolutions. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// newCustomersProfileCmd implements `stripe-pp-pp-cli customers profile <cus_id_or_email>`.
// Joins customer + active subscriptions + recent charges/refunds + open invoices + LTV.
// Live API path (works without a sync); the dotted JSON output is `--select`-friendly.
func newCustomersProfileCmd(flags *rootFlags) *cobra.Command {
	var flagSince string

	cmd := &cobra.Command{
		Use:   "profile [customer-id-or-email]",
		Short: "Joined customer record: subscriptions, lifetime charges/refunds, open invoices, LTV",
		Long: strings.TrimSpace(`
Customer 360. Returns one JSON document combining a customer's record with their
active subscriptions, lifetime charges + refunds, open invoices, and computed
lifetime value.

Replaces the dashboard scavenger hunt that splits this across 5 tabs with one
deterministic agent-friendly call.
`),
		Example: strings.Trim(`
  stripe-pp-pp-cli customers profile cus_1234 --json
  stripe-pp-pp-cli customers profile user@acme.co --json --select id,email,lifetime_value
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ident := args[0]

			// 1) Resolve customer: try ID lookup, fall back to email search.
			var customer map[string]any
			if strings.HasPrefix(ident, "cus_") {
				raw, err := c.Get("/v1/customers/"+ident, nil)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				_ = json.Unmarshal(raw, &customer)
			} else if strings.Contains(ident, "@") {
				raw, err := c.Get("/v1/customers", map[string]string{"email": ident, "limit": "1"})
				if err != nil {
					return classifyAPIError(err, flags)
				}
				var list struct {
					Data []map[string]any `json:"data"`
				}
				if err := json.Unmarshal(raw, &list); err == nil && len(list.Data) > 0 {
					customer = list.Data[0]
				}
				if customer == nil {
					return notFoundErr(fmt.Errorf("no customer found for email %q", ident))
				}
			} else {
				return usageErr(fmt.Errorf("expected a customer ID (cus_…) or email, got %q", ident))
			}

			custID, _ := customer["id"].(string)
			params := map[string]string{"customer": custID, "limit": "100"}
			if flagSince != "" {
				params["created[gte]"] = flagSince
			}

			// 2-4) Fetch subscriptions, charges, invoices in series (parallelizable later).
			subsRaw, err := c.Get("/v1/subscriptions", map[string]string{"customer": custID, "limit": "100", "status": "active"})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			chargesRaw, err := c.Get("/v1/charges", params)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			invoicesRaw, err := c.Get("/v1/invoices", map[string]string{"customer": custID, "status": "open", "limit": "100"})
			if err != nil {
				return classifyAPIError(err, flags)
			}

			subs := unmarshalList(subsRaw)
			charges := unmarshalList(chargesRaw)
			invoices := unmarshalList(invoicesRaw)

			// 5) Compute LTV (sum of charge amounts minus refunds) in customer's currency.
			var ltv int64
			var refundedTotal int64
			for _, ch := range charges {
				if amt, ok := ch["amount"].(float64); ok {
					ltv += int64(amt)
				}
				if amt, ok := ch["amount_refunded"].(float64); ok {
					refundedTotal += int64(amt)
					ltv -= int64(amt)
				}
			}
			var openBalance int64
			for _, inv := range invoices {
				if amt, ok := inv["amount_due"].(float64); ok {
					openBalance += int64(amt)
				}
			}

			out := map[string]any{
				"id":                        custID,
				"email":                     customer["email"],
				"name":                      customer["name"],
				"created":                   customer["created"],
				"currency":                  customer["currency"],
				"delinquent":                customer["delinquent"],
				"lifetime_value":            ltv,
				"lifetime_refunded":         refundedTotal,
				"open_invoice_balance":      openBalance,
				"active_subscriptions":      subs,
				"recent_charges":            charges,
				"open_invoices":             invoices,
				"recent_charge_count":       len(charges),
				"active_subscription_count": len(subs),
				"open_invoice_count":        len(invoices),
			}
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "", "Limit recent_charges to those created at or after this Unix timestamp")
	return cmd
}

// unmarshalList parses a Stripe list-response envelope and returns the .data array.
// Returns an empty slice on any error so callers get a stable shape.
func unmarshalList(raw json.RawMessage) []map[string]any {
	var env struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil
	}
	return env.Data
}
