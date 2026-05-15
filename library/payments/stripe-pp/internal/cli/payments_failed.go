// Copyright 2026 primiasolutions. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// newPaymentsFailedCmd implements `stripe-pp-pp-cli payments failed`.
// Lists PaymentIntents that need a payment method (failed/requires_payment_method),
// groups them by decline_code, customer, or amount band, with $ at risk.
func newPaymentsFailedCmd(flags *rootFlags) *cobra.Command {
	var flagSince string
	var flagGroupBy string

	cmd := &cobra.Command{
		Use:   "failed",
		Short: "List failed PaymentIntents grouped by decline_code, customer, or amount band",
		Long: strings.TrimSpace(`
Failed-payment triage. Lists PaymentIntents in 'requires_payment_method' (the
canonical failed/needs-retry state) over a window, grouped by:
  - decline_code  (default) — bucket by last_payment_error.decline_code
  - customer                — bucket by the affected customer
  - amount                  — bucket by amount band ($0-$10, $10-$100, …)

Returns counts plus the total $ at risk per bucket. Replaces the dashboard's
ungrouped failures list with one aggregated answer.
`),
		Example: strings.Trim(`
  stripe-pp-pp-cli payments failed --since 24h --json
  stripe-pp-pp-cli payments failed --since 2026-05-01 --group-by decline_code --json --select decline_code,count,total_amount
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			sinceTS, err := parseSinceArg(flagSince)
			if err != nil {
				return usageErr(err)
			}

			startingAfter := ""
			intents := []map[string]any{}
			for i := 0; i < 20; i++ {
				params := map[string]string{"limit": "100"}
				if sinceTS > 0 {
					params["created[gte]"] = strconv.FormatInt(sinceTS, 10)
				}
				if startingAfter != "" {
					params["starting_after"] = startingAfter
				}
				raw, err := c.Get("/v1/payment_intents", params)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				var env struct {
					Data    []map[string]any `json:"data"`
					HasMore bool             `json:"has_more"`
				}
				if err := json.Unmarshal(raw, &env); err != nil {
					return apiErr(err)
				}
				for _, pi := range env.Data {
					status, _ := pi["status"].(string)
					if status == "requires_payment_method" || status == "canceled" {
						intents = append(intents, pi)
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

			type bucket struct {
				count  int
				amount int64
				ids    []string
				custs  []string
			}
			groups := map[string]*bucket{}
			for _, pi := range intents {
				key := bucketKey(pi, flagGroupBy)
				b := groups[key]
				if b == nil {
					b = &bucket{}
					groups[key] = b
				}
				b.count++
				if amt, ok := pi["amount"].(float64); ok {
					b.amount += int64(amt)
				}
				if id, ok := pi["id"].(string); ok && len(b.ids) < 5 {
					b.ids = append(b.ids, id)
				}
				if cust, ok := pi["customer"].(string); ok && cust != "" && len(b.custs) < 5 {
					b.custs = append(b.custs, cust)
				}
			}

			out := []map[string]any{}
			for k, b := range groups {
				out = append(out, map[string]any{
					"key":              k,
					"count":            b.count,
					"total_amount":     b.amount,
					"sample_ids":       b.ids,
					"sample_customers": b.custs,
				})
			}
			sort.Slice(out, func(i, j int) bool {
				return out[i]["count"].(int) > out[j]["count"].(int)
			})

			return flags.printJSON(cmd, map[string]any{
				"since":    flagSince,
				"group_by": effectivePaymentsGroupBy(flagGroupBy),
				"total":    len(intents),
				"groups":   out,
			})
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "30d", "Window start: Unix ts, RFC3339, YYYY-MM-DD, or relative like 7d/24h")
	cmd.Flags().StringVar(&flagGroupBy, "group-by", "decline_code", "Group by: decline_code | customer | amount")
	return cmd
}

func bucketKey(pi map[string]any, groupBy string) string {
	switch effectivePaymentsGroupBy(groupBy) {
	case "customer":
		if v, ok := pi["customer"].(string); ok && v != "" {
			return v
		}
		return "no-customer"
	case "amount":
		amt, _ := pi["amount"].(float64)
		switch {
		case amt < 1000:
			return "$0-$10"
		case amt < 10000:
			return "$10-$100"
		case amt < 100000:
			return "$100-$1000"
		default:
			return "$1000+"
		}
	default:
		if err, ok := pi["last_payment_error"].(map[string]any); ok {
			if v, ok := err["decline_code"].(string); ok && v != "" {
				return v
			}
			if v, ok := err["code"].(string); ok && v != "" {
				return v
			}
		}
		return "unspecified"
	}
}

func effectivePaymentsGroupBy(g string) string {
	switch g {
	case "customer", "amount":
		return g
	default:
		return "decline_code"
	}
}
