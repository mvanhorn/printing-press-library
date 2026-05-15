// Copyright 2026 primiasolutions. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// newPayoutsExplainCmd implements `stripe-pp-pp-cli payouts explain <po_id>`.
// Decomposes a payout into balance_transactions and groups by type, with
// optional per-customer attribution for the charge-typed entries.
func newPayoutsExplainCmd(flags *rootFlags) *cobra.Command {
	var flagTopCustomers int

	cmd := &cobra.Command{
		Use:   "explain [payout-id]",
		Short: "Decompose a payout into balance_transactions with type and customer attribution",
		Long: strings.TrimSpace(`
Payout explain. Returns one JSON document with:
  - the payout amount, arrival_date, currency, and status
  - a breakdown by balance_transaction type (charge, refund, fee, adjustment…)
  - the top N customers contributing to charge-typed transactions

Replaces the Dashboard CSV export + spreadsheet pivot pattern with one call.
`),
		Example: strings.Trim(`
  stripe-pp-pp-cli payouts explain po_1NXabc --json
  stripe-pp-pp-cli payouts explain po_1NXabc --json --select amount,arrival_date,breakdown,top_customers
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
			poID := args[0]

			// Fetch the payout
			poRaw, err := c.Get("/v1/payouts/"+poID, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var payout map[string]any
			_ = json.Unmarshal(poRaw, &payout)

			// Fetch all balance_transactions for this payout (paginate up to 1000).
			breakdown := map[string]int64{}
			var counts = map[string]int{}
			var customerSums = map[string]int64{}
			var customerNames = map[string]string{}
			var totalNet int64

			startingAfter := ""
			for i := 0; i < 10; i++ {
				params := map[string]string{"payout": poID, "limit": "100"}
				if startingAfter != "" {
					params["starting_after"] = startingAfter
				}
				btRaw, err := c.Get("/v1/balance_transactions", params)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				var env struct {
					Data    []map[string]any `json:"data"`
					HasMore bool             `json:"has_more"`
				}
				if err := json.Unmarshal(btRaw, &env); err != nil {
					return apiErr(err)
				}
				for _, bt := range env.Data {
					t, _ := bt["type"].(string)
					if amt, ok := bt["net"].(float64); ok {
						breakdown[t] += int64(amt)
						totalNet += int64(amt)
					}
					counts[t]++
					if flagTopCustomers > 0 {
						if source, ok := bt["source"].(string); ok && strings.HasPrefix(source, "ch_") {
							if cust := lookupChargeCustomer(c, source); cust != "" {
								if amt, ok := bt["amount"].(float64); ok {
									customerSums[cust] += int64(amt)
								}
								if customerNames[cust] == "" {
									customerNames[cust] = lookupCustomerName(c, cust)
								}
							}
						}
					}
				}
				if !env.HasMore || len(env.Data) == 0 {
					break
				}
				lastID, _ := env.Data[len(env.Data)-1]["id"].(string)
				if lastID == "" {
					break
				}
				startingAfter = lastID
			}

			breakdownOut := []map[string]any{}
			for t, amt := range breakdown {
				breakdownOut = append(breakdownOut, map[string]any{
					"type":  t,
					"net":   amt,
					"count": counts[t],
				})
			}
			sort.Slice(breakdownOut, func(i, j int) bool {
				return breakdownOut[i]["net"].(int64) > breakdownOut[j]["net"].(int64)
			})

			topCustomers := []map[string]any{}
			if flagTopCustomers > 0 {
				type entry struct {
					id, name string
					sum      int64
				}
				entries := make([]entry, 0, len(customerSums))
				for id, sum := range customerSums {
					entries = append(entries, entry{id: id, name: customerNames[id], sum: sum})
				}
				sort.Slice(entries, func(i, j int) bool { return entries[i].sum > entries[j].sum })
				n := flagTopCustomers
				if n > len(entries) {
					n = len(entries)
				}
				for _, e := range entries[:n] {
					topCustomers = append(topCustomers, map[string]any{
						"customer": e.id,
						"name":     e.name,
						"amount":   e.sum,
					})
				}
			}

			out := map[string]any{
				"id":            payout["id"],
				"amount":        payout["amount"],
				"currency":      payout["currency"],
				"status":        payout["status"],
				"arrival_date":  payout["arrival_date"],
				"created":       payout["created"],
				"breakdown":     breakdownOut,
				"top_customers": topCustomers,
				"total_net":     totalNet,
			}
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().IntVar(&flagTopCustomers, "top-customers", 0, "Include the top N customers by charge amount (extra API calls per charge)")
	return cmd
}

// lookupChargeCustomer returns the customer ID for a charge or "" on any failure.
// Best-effort; failures don't break the breakdown.
func lookupChargeCustomer(c apiClient, chargeID string) string {
	raw, err := c.Get("/v1/charges/"+chargeID, nil)
	if err != nil {
		return ""
	}
	var ch map[string]any
	if json.Unmarshal(raw, &ch) != nil {
		return ""
	}
	if s, ok := ch["customer"].(string); ok {
		return s
	}
	return ""
}

func lookupCustomerName(c apiClient, custID string) string {
	if custID == "" {
		return ""
	}
	raw, err := c.Get("/v1/customers/"+custID, nil)
	if err != nil {
		return ""
	}
	var cu map[string]any
	if json.Unmarshal(raw, &cu) != nil {
		return ""
	}
	if v, ok := cu["name"].(string); ok && v != "" {
		return v
	}
	if v, ok := cu["email"].(string); ok {
		return v
	}
	return ""
}

// apiClient is the subset of *client.Client used by the explain helpers.
// Declaring it here keeps the helpers easy to swap for tests later.
type apiClient interface {
	Get(path string, params map[string]string) (json.RawMessage, error)
}

// Compile-time assertion (kept inside a function so unused-import warnings stay clean
// in builds that don't currently exercise the interface).
var _ = fmt.Sprintf
