// Copyright 2026 Chris Rodriguez and contributors. Licensed under Apache-2.0. See LICENSE.

// PATCH: add `mrr-trend` analytics command. Computes MRR/ARR from active +
// trialing subscriptions joined to their prices, normalizing each item's
// (unit_amount * quantity) to monthly via recurring.interval + interval_count.
// Local-only — reads from the SQLite mirror. Math ported from the
// normalizeToMonthly + subscriptionMonthlyRevenue + aggregateSubscriptions
// pattern documented in .printing-press-patches.json.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/payments/stripe/internal/store"

	"github.com/spf13/cobra"
)

// mrrCurrencyRow is the per-currency rollup. Stripe stores unit_amount as
// integer minor units (cents for USD), and currencies cannot be summed
// directly — surface them separately.
type mrrCurrencyRow struct {
	Currency string `json:"currency"`
	MRRCents int64  `json:"mrr_cents"`
	ARRCents int64  `json:"arr_cents"`
}

// mrrProductRow is the optional per-product rollup. Product name is pulled
// from the local products table; the id is always present.
type mrrProductRow struct {
	ProductID   string `json:"product_id"`
	ProductName string `json:"product_name,omitempty"`
	Currency    string `json:"currency"`
	MRRCents    int64  `json:"mrr_cents"`
	Subscribers int    `json:"subscribers"`
}

// mrrSnapshot is the top-level JSON response shape.
type mrrSnapshot struct {
	Currency             string           `json:"currency"`
	MRRCents             int64            `json:"mrr_cents"`
	ARRCents             int64            `json:"arr_cents"`
	ActiveSubscriptions  int              `json:"active_subscriptions"`
	TrialingSubs         int              `json:"trialing"`
	IncludedTrialing     bool             `json:"included_trialing"`
	ByCurrency           []mrrCurrencyRow `json:"by_currency"`
	ByProduct            []mrrProductRow  `json:"by_product,omitempty"`
}

func newMRRTrendCmd(flags *rootFlags) *cobra.Command {
	var currency string
	var includeTrialing bool
	var byProduct bool
	var dbPath string

	cmd := &cobra.Command{
		Use:   "mrr-trend",
		Short: "Compute MRR/ARR from active + trialing subscriptions",
		Long: `Walk every active (and optionally trialing) subscription, sum each item's
(unit_amount * quantity) normalized to a monthly figure using recurring.interval
(day/week/month/year) and recurring.interval_count. MRR is reported per currency;
the primary --currency value is surfaced at the top level, others appear under
by_currency. Use --by product to also break down per-product MRR.`,
		Example: `  # Total MRR/ARR in USD (default)
  stripe-pp-cli mrr-trend --json

  # Break down per product, exclude trialing
  stripe-pp-cli mrr-trend --by product --include-trialing=false`,
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

			snapshot, err := computeMRR(db.DB(), strings.ToLower(currency), includeTrialing, byProduct)
			if err != nil {
				return apiErr(err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), snapshot, flags)
		},
	}

	cmd.Flags().StringVar(&currency, "currency", "USD", "Primary currency for top-level mrr_cents/arr_cents (others shown in by_currency)")
	cmd.Flags().BoolVar(&includeTrialing, "include-trialing", true, "Include trialing subscriptions in MRR")
	cmd.Flags().BoolVar(&byProduct, "by", false, "When true, populate by_product breakdown (equivalent to --by product)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/stripe-pp-cli/data.db)")

	// Allow --by product (string form) for symmetry with refund-rate/dispute-win-rate.
	// We parse it manually because cobra's bool flag form expects --by alone.
	cmd.Flags().Lookup("by").NoOptDefVal = "true"

	return cmd
}

// computeMRR walks subscriptions and aggregates monthly recurring revenue.
// includeTrialing=true sums status IN ('active','trialing'); false sums only 'active'.
// byProduct=true populates the by_product slice (requires a join to the local
// products table for display names).
func computeMRR(db *sql.DB, primaryCurrency string, includeTrialing bool, byProduct bool) (mrrSnapshot, error) {
	statuses := "'active'"
	if includeTrialing {
		statuses = "'active','trialing'"
	}
	q := fmt.Sprintf(`SELECT id, data FROM resources WHERE resource_type='subscriptions'
		AND json_extract(data,'$.status') IN (%s)`, statuses)
	rs, err := db.Query(q)
	if err != nil {
		return mrrSnapshot{}, fmt.Errorf("querying subscriptions: %w", err)
	}
	defer rs.Close()

	currencyMRR := make(map[string]int64)
	// productKey = productID + "|" + currency (currencies cannot be summed)
	productMRR := make(map[string]int64)
	productCurrency := make(map[string]string)
	productSubs := make(map[string]int)
	activeCount := 0
	trialingCount := 0

	productCache := make(map[string]string) // productID -> name

	for rs.Next() {
		var id, data string
		if err := rs.Scan(&id, &data); err != nil {
			return mrrSnapshot{}, err
		}
		raw := json.RawMessage(data)

		status, _ := jsonGet(raw, "status")
		switch status {
		case "active":
			activeCount++
		case "trialing":
			trialingCount++
		}

		monthly, currency, byProd := subscriptionMonthlyMRR(raw)
		if monthly == 0 || currency == "" {
			continue
		}
		currencyMRR[currency] += monthly

		if byProduct {
			for productID, amt := range byProd {
				key := productID + "|" + currency
				productMRR[key] += amt
				productCurrency[key] = currency
				productSubs[key]++

				// Lazy product-name lookup. Empty name on miss is fine.
				if _, hit := productCache[productID]; !hit && productID != "" {
					productCache[productID] = lookupProductName(db, productID)
				}
			}
		}
	}
	if err := rs.Err(); err != nil {
		return mrrSnapshot{}, err
	}

	snapshot := mrrSnapshot{
		Currency:            primaryCurrency,
		ActiveSubscriptions: activeCount,
		TrialingSubs:        trialingCount,
		IncludedTrialing:    includeTrialing,
		ByCurrency:          make([]mrrCurrencyRow, 0, len(currencyMRR)),
	}
	if v, ok := currencyMRR[primaryCurrency]; ok {
		snapshot.MRRCents = v
		snapshot.ARRCents = v * 12
	}
	for cur, mrr := range currencyMRR {
		if cur == primaryCurrency {
			// Primary currency is already surfaced at the top level via
			// MRRCents/ARRCents. Emitting it again under ByCurrency contradicts
			// the documented contract ("others appear under by_currency") and
			// gives consumers a false positive when they use `len(by_currency)>0`
			// to detect non-primary currencies.
			continue
		}
		snapshot.ByCurrency = append(snapshot.ByCurrency, mrrCurrencyRow{
			Currency: cur,
			MRRCents: mrr,
			ARRCents: mrr * 12,
		})
	}
	sort.SliceStable(snapshot.ByCurrency, func(i, j int) bool {
		return snapshot.ByCurrency[i].MRRCents > snapshot.ByCurrency[j].MRRCents
	})

	if byProduct {
		for key, mrr := range productMRR {
			parts := strings.SplitN(key, "|", 2)
			pid := parts[0]
			cur := productCurrency[key]
			row := mrrProductRow{
				ProductID:   pid,
				ProductName: productCache[pid],
				Currency:    cur,
				MRRCents:    mrr,
				Subscribers: productSubs[key],
			}
			snapshot.ByProduct = append(snapshot.ByProduct, row)
		}
		sort.SliceStable(snapshot.ByProduct, func(i, j int) bool {
			return snapshot.ByProduct[i].MRRCents > snapshot.ByProduct[j].MRRCents
		})
	}
	return snapshot, nil
}

// subscriptionMonthlyMRR walks ALL items on a subscription (unlike subs_at_risk's
// subscriptionMRR which only takes the first item) and returns the total MRR,
// the currency, and a per-product breakdown. Returns currency = "" when no
// item has a valid recurring price.
func subscriptionMonthlyMRR(raw json.RawMessage) (int64, string, map[string]int64) {
	var sub map[string]json.RawMessage
	if err := json.Unmarshal(raw, &sub); err != nil {
		return 0, "", nil
	}
	itemsRaw, ok := sub["items"]
	if !ok {
		return 0, "", nil
	}
	var items map[string]json.RawMessage
	if err := json.Unmarshal(itemsRaw, &items); err != nil {
		return 0, "", nil
	}
	dataRaw, ok := items["data"]
	if !ok {
		return 0, "", nil
	}
	var arr []map[string]json.RawMessage
	if err := json.Unmarshal(dataRaw, &arr); err != nil {
		return 0, "", nil
	}

	var totalMonthly int64
	currency := ""
	byProduct := make(map[string]int64)

	for _, item := range arr {
		var qty int64 = 1
		if q, ok := item["quantity"]; ok {
			_ = json.Unmarshal(q, &qty)
		}
		priceRaw, ok := item["price"]
		if !ok {
			continue
		}
		var price map[string]json.RawMessage
		if err := json.Unmarshal(priceRaw, &price); err != nil {
			continue
		}
		var unit int64
		if v, ok := price["unit_amount"]; ok {
			_ = json.Unmarshal(v, &unit)
		}
		if unit == 0 {
			continue
		}
		var cur string
		if v, ok := price["currency"]; ok {
			_ = json.Unmarshal(v, &cur)
		}
		if cur == "" {
			cur = "usd"
		}
		if currency == "" {
			currency = strings.ToLower(cur)
		}
		// Recurring info; without it, treat as non-recurring (skip).
		recRaw, ok := price["recurring"]
		if !ok || string(recRaw) == "null" {
			continue
		}
		var rec map[string]json.RawMessage
		if err := json.Unmarshal(recRaw, &rec); err != nil {
			continue
		}
		var interval string
		if v, ok := rec["interval"]; ok {
			_ = json.Unmarshal(v, &interval)
		}
		var intervalCount int64 = 1
		if v, ok := rec["interval_count"]; ok {
			_ = json.Unmarshal(v, &intervalCount)
		}
		if intervalCount <= 0 {
			intervalCount = 1
		}

		monthly := normalizeToMonthlyCents(unit*qty, interval, intervalCount)
		if monthly == 0 {
			continue
		}
		totalMonthly += monthly

		// Product id may be a string or an embedded object.
		pid := ""
		if v, ok := price["product"]; ok {
			var s string
			if json.Unmarshal(v, &s) == nil {
				pid = s
			} else {
				var obj map[string]json.RawMessage
				if json.Unmarshal(v, &obj) == nil {
					if idRaw, ok := obj["id"]; ok {
						_ = json.Unmarshal(idRaw, &pid)
					}
				}
			}
		}
		if pid == "" {
			pid = "untagged"
		}
		byProduct[pid] += monthly
	}
	return totalMonthly, currency, byProduct
}

// normalizeToMonthlyCents normalizes a gross-per-period amount (in minor units)
// to a monthly figure, matching the Baremetrics / ProfitWell convention:
// month → as-is, year → /12, week → *4.33, day → *30. Returns 0 for unknown
// intervals so callers can skip silently.
func normalizeToMonthlyCents(gross int64, interval string, intervalCount int64) int64 {
	if gross <= 0 || intervalCount <= 0 {
		return 0
	}
	switch interval {
	case "month":
		return gross / intervalCount
	case "year":
		return gross / (12 * intervalCount)
	case "week":
		// 52 weeks ÷ 12 months ≈ 4.33 weeks per month.
		// Integer math: (gross * 433) / (100 * intervalCount).
		return (gross * 433) / (100 * intervalCount)
	case "day":
		// ~30 days per month.
		return (gross * 30) / intervalCount
	default:
		return 0
	}
}

// lookupProductName fetches a product's display name from the local store.
// Returns "" on any miss so the caller can fall back to the id.
func lookupProductName(db *sql.DB, productID string) string {
	var data string
	if err := db.QueryRow(
		`SELECT data FROM resources WHERE resource_type='products' AND id=?`, productID,
	).Scan(&data); err != nil {
		return ""
	}
	if name, ok := jsonGet(json.RawMessage(data), "name"); ok {
		return name
	}
	return ""
}
