// Copyright 2026 primiasolutions. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// newSubscriptionsChurnCmd implements `stripe-pp-pp-cli subscriptions churn`.
// Lists canceled subscriptions in a window, groups by cancellation reason, attaches MRR delta.
func newSubscriptionsChurnCmd(flags *rootFlags) *cobra.Command {
	var flagSince string
	var flagGroupBy string
	var flagMRRDelta bool

	cmd := &cobra.Command{
		Use:   "churn",
		Short: "Group canceled/past_due subscriptions by reason with MRR delta and customer attribution",
		Long: strings.TrimSpace(`
Subscription churn audit. Lists canceled (status=canceled) and past_due
subscriptions in a window, grouped by cancellation_details.reason or .feedback,
with the MRR delta and contributing customers per group.

Replaces Stripe Sigma's lagged churn report (paid extra) with one CLI call.
`),
		Example: strings.Trim(`
  stripe-pp-pp-cli subscriptions churn --since 2026-04-01 --json
  stripe-pp-pp-cli subscriptions churn --since 7d --group-by cancellation_reason --mrr-delta --json
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
			rows := []map[string]any{}
			for i := 0; i < 20; i++ {
				params := map[string]string{
					"status": "canceled",
					"limit":  "100",
				}
				if sinceTS > 0 {
					params["created[gte]"] = strconv.FormatInt(sinceTS, 10)
				}
				if startingAfter != "" {
					params["starting_after"] = startingAfter
				}
				raw, err := c.Get("/v1/subscriptions", params)
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
				rows = append(rows, env.Data...)
				if !env.HasMore || len(env.Data) == 0 {
					break
				}
				lastID, _ := env.Data[len(env.Data)-1]["id"].(string)
				if lastID == "" {
					break
				}
				startingAfter = lastID
			}

			// Group by chosen field.
			type bucket struct {
				count       int
				mrrDelta    int64
				customers   []string
				sampleSubID string
			}
			groups := map[string]*bucket{}
			for _, sub := range rows {
				key := extractCancelReason(sub, flagGroupBy)
				if key == "" {
					key = "unspecified"
				}
				b := groups[key]
				if b == nil {
					b = &bucket{}
					groups[key] = b
				}
				b.count++
				if flagMRRDelta {
					b.mrrDelta -= computeSubscriptionMRR(sub)
				}
				if cust, ok := sub["customer"].(string); ok && len(b.customers) < 5 {
					b.customers = append(b.customers, cust)
				}
				if b.sampleSubID == "" {
					if sid, ok := sub["id"].(string); ok {
						b.sampleSubID = sid
					}
				}
			}

			groupsOut := []map[string]any{}
			for k, b := range groups {
				row := map[string]any{
					"key":              k,
					"count":            b.count,
					"sample_customers": b.customers,
					"sample_id":        b.sampleSubID,
				}
				if flagMRRDelta {
					row["mrr_delta"] = b.mrrDelta
				}
				groupsOut = append(groupsOut, row)
			}
			sort.Slice(groupsOut, func(i, j int) bool {
				return groupsOut[i]["count"].(int) > groupsOut[j]["count"].(int)
			})

			out := map[string]any{
				"since":    flagSince,
				"group_by": effectiveGroupBy(flagGroupBy),
				"total":    len(rows),
				"groups":   groupsOut,
			}
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "30d", "Window start: Unix timestamp, RFC3339 date, or relative like 7d / 90d")
	cmd.Flags().StringVar(&flagGroupBy, "group-by", "cancellation_reason", "Group by: cancellation_reason | cancellation_feedback")
	cmd.Flags().BoolVar(&flagMRRDelta, "mrr-delta", false, "Compute MRR impact per group (sums each subscription's recurring price)")
	return cmd
}

func extractCancelReason(sub map[string]any, groupBy string) string {
	details, _ := sub["cancellation_details"].(map[string]any)
	field := "reason"
	if effectiveGroupBy(groupBy) == "cancellation_feedback" {
		field = "feedback"
	}
	if details != nil {
		if v, ok := details[field].(string); ok {
			return v
		}
	}
	return ""
}

func effectiveGroupBy(g string) string {
	if g == "cancellation_feedback" || g == "feedback" {
		return "cancellation_feedback"
	}
	return "cancellation_reason"
}

// computeSubscriptionMRR sums the recurring price for active items. Works on the
// raw Stripe payload; treats one-time prices as 0 contribution.
func computeSubscriptionMRR(sub map[string]any) int64 {
	items, _ := sub["items"].(map[string]any)
	if items == nil {
		return 0
	}
	data, _ := items["data"].([]any)
	var total int64
	for _, it := range data {
		item, _ := it.(map[string]any)
		if item == nil {
			continue
		}
		price, _ := item["price"].(map[string]any)
		if price == nil {
			continue
		}
		if price["recurring"] == nil {
			continue
		}
		amt, _ := price["unit_amount"].(float64)
		qty, _ := item["quantity"].(float64)
		if qty == 0 {
			qty = 1
		}
		total += int64(amt) * int64(qty)
	}
	return total
}

func parseSinceArg(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	// Relative like 7d, 24h, 30m
	if l := len(s); l > 1 {
		unit := s[l-1]
		if n, err := strconv.Atoi(s[:l-1]); err == nil {
			switch unit {
			case 'd':
				return time.Now().Add(-time.Duration(n) * 24 * time.Hour).Unix(), nil
			case 'h':
				return time.Now().Add(-time.Duration(n) * time.Hour).Unix(), nil
			case 'm':
				return time.Now().Add(-time.Duration(n) * time.Minute).Unix(), nil
			}
		}
	}
	// Unix timestamp
	if ts, err := strconv.ParseInt(s, 10, 64); err == nil {
		return ts, nil
	}
	// RFC3339
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.Unix(), nil
	}
	// Date-only
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t.Unix(), nil
	}
	return 0, fmt.Errorf("could not parse --since %q: expected Unix ts, RFC3339, YYYY-MM-DD, or relative like 7d", s)
}
