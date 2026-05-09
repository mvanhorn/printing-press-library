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

type subAtRiskRow struct {
	CustomerEmail  string `json:"customer_email,omitempty"`
	SubscriptionID string `json:"subscription_id"`
	PlanLookupKey  string `json:"plan_lookup_key,omitempty"`
	MRR            int64  `json:"mrr"`
	Currency       string `json:"currency,omitempty"`
	CardBrand      string `json:"card_brand,omitempty"`
	CardLast4      string `json:"card_last4,omitempty"`
	CardExp        string `json:"card_exp,omitempty"`
}

func newSubsAtRiskCmd(flags *rootFlags) *cobra.Command {
	var withinStr string
	var limit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "subs-at-risk",
		Short: "Subscriptions whose default PM card expires within window, sorted by MRR",
		Long: `Walk every active subscription, find its default_payment_method, and report
on those whose card expires inside the window. Sorted MRR-desc so the most
revenue-impactful at-risk subs surface first.`,
		Example: `  # Cards expiring in next 30 days
  stripe-pp-cli subs-at-risk

  # Next 60 days, top-25 by MRR
  stripe-pp-cli subs-at-risk --within 1440h --limit 25 --json`,
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

			within, err := parseHumanDuration(withinStr)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --within: %w", err))
			}
			if within == 0 {
				within = 30 * 24 * time.Hour
			}
			rows, err := buildSubsAtRisk(db.DB(), within)
			if err != nil {
				return apiErr(err)
			}
			sort.SliceStable(rows, func(i, j int) bool {
				return rows[i].MRR > rows[j].MRR
			})
			if limit > 0 && len(rows) > limit {
				rows = rows[:limit]
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}

	cmd.Flags().StringVar(&withinStr, "within", "30d", "Expiration window from today (e.g. 30d, 6w, 720h)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Cap output to top-N by MRR")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/stripe-pp-cli/data.db)")

	return cmd
}

func buildSubsAtRisk(db *sql.DB, within time.Duration) ([]subAtRiskRow, error) {
	rs, err := db.Query(`SELECT id, data FROM resources WHERE resource_type='subscriptions'
		AND json_extract(data,'$.status') IN ('active','trialing','past_due')`)
	if err != nil {
		return nil, err
	}
	defer rs.Close()

	now := time.Now()
	cutoff := now.Add(within)
	out := make([]subAtRiskRow, 0)
	for rs.Next() {
		var id, data string
		if err := rs.Scan(&id, &data); err != nil {
			return nil, err
		}
		raw := json.RawMessage(data)

		pmID, _ := jsonGet(raw, "default_payment_method")
		if pmID == "" {
			continue
		}
		brand, last4, expMonth, expYear, ok := lookupCardExpiry(db, pmID)
		if !ok {
			continue
		}
		// expMonth/expYear are calendar fields. The card is still valid up
		// to the END of that month; treat the expiry instant as the first
		// of the following month at midnight UTC.
		exp := time.Date(expYear, time.Month(expMonth)+1, 1, 0, 0, 0, 0, time.UTC)
		if exp.After(cutoff) {
			continue
		}

		row := subAtRiskRow{
			SubscriptionID: id,
			CardBrand:      brand,
			CardLast4:      last4,
			CardExp:        fmt.Sprintf("%02d/%04d", expMonth, expYear),
		}
		row.MRR, row.PlanLookupKey, row.Currency = subscriptionMRR(raw)

		// Customer email lookup.
		if cid, ok := jsonGet(raw, "customer"); ok {
			var cdata string
			_ = db.QueryRow(
				`SELECT data FROM resources WHERE resource_type='customers' AND id=?`, cid,
			).Scan(&cdata)
			if cdata != "" {
				if email, ok := jsonGet(json.RawMessage(cdata), "email"); ok {
					row.CustomerEmail = email
				}
			}
		}
		out = append(out, row)
	}
	return out, rs.Err()
}

// lookupCardExpiry pulls a payment method's card brand/last4/exp from the
// store. Returns ok=false for non-card PMs or PMs not in the mirror.
func lookupCardExpiry(db *sql.DB, pmID string) (brand, last4 string, expMonth, expYear int, ok bool) {
	var data string
	if err := db.QueryRow(
		`SELECT data FROM resources WHERE resource_type='payment_methods' AND id=?`, pmID,
	).Scan(&data); err != nil {
		return "", "", 0, 0, false
	}
	var pm map[string]json.RawMessage
	if err := json.Unmarshal([]byte(data), &pm); err != nil {
		return "", "", 0, 0, false
	}
	cardRaw, hasCard := pm["card"]
	if !hasCard {
		return "", "", 0, 0, false
	}
	var card map[string]json.RawMessage
	if err := json.Unmarshal(cardRaw, &card); err != nil {
		return "", "", 0, 0, false
	}
	intField := func(k string) int {
		v, has := card[k]
		if !has {
			return 0
		}
		var n int
		if json.Unmarshal(v, &n) == nil {
			return n
		}
		return 0
	}
	strField := func(k string) string {
		v, has := card[k]
		if !has {
			return ""
		}
		var s string
		if json.Unmarshal(v, &s) == nil {
			return s
		}
		return ""
	}
	expMonth = intField("exp_month")
	expYear = intField("exp_year")
	if expMonth == 0 || expYear == 0 {
		return "", "", 0, 0, false
	}
	return strField("brand"), strField("last4"), expMonth, expYear, true
}

// subscriptionMRR pulls the first item's unit_amount * quantity, returning
// (mrr, plan_lookup_key, currency). Stripe nests items under data.items.data.
func subscriptionMRR(raw json.RawMessage) (int64, string, string) {
	var sub map[string]json.RawMessage
	if err := json.Unmarshal(raw, &sub); err != nil {
		return 0, "", ""
	}
	itemsRaw, ok := sub["items"]
	if !ok {
		return 0, "", ""
	}
	var items map[string]json.RawMessage
	if err := json.Unmarshal(itemsRaw, &items); err != nil {
		return 0, "", ""
	}
	dataRaw, ok := items["data"]
	if !ok {
		return 0, "", ""
	}
	var arr []map[string]json.RawMessage
	if err := json.Unmarshal(dataRaw, &arr); err != nil || len(arr) == 0 {
		return 0, "", ""
	}
	first := arr[0]
	var qty int64 = 1
	if q, ok := first["quantity"]; ok {
		_ = json.Unmarshal(q, &qty)
	}
	priceRaw, ok := first["price"]
	if !ok {
		return 0, "", ""
	}
	var price map[string]json.RawMessage
	if err := json.Unmarshal(priceRaw, &price); err != nil {
		return 0, "", ""
	}
	var unit int64
	if v, ok := price["unit_amount"]; ok {
		_ = json.Unmarshal(v, &unit)
	}
	var cur, lookup string
	if v, ok := price["currency"]; ok {
		_ = json.Unmarshal(v, &cur)
	}
	if v, ok := price["lookup_key"]; ok {
		_ = json.Unmarshal(v, &lookup)
	}
	return unit * qty, lookup, cur
}
