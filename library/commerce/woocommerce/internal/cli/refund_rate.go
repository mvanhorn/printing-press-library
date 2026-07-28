// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/woocommerce/internal/store"
)

type refundRateItem struct {
	ProductID     string  `json:"product_id"`
	Name          string  `json:"name"`
	SKU           string  `json:"sku,omitempty"`
	UnitsSold     int     `json:"units_sold"`
	UnitsRefunded int     `json:"units_refunded"`
	RefundRate    float64 `json:"refund_rate"`
	RefundedValue float64 `json:"refunded_value"`
}

type refundRateView struct {
	Window        string           `json:"window"`
	Attribution   string           `json:"attribution"`
	ScannedOrders int              `json:"scanned_orders"`
	Items         []refundRateItem `json:"items"`
	Note          string           `json:"note,omitempty"`
}

// refundLineItems is the subset of a refund record needed for attribution.
type refundLineItems struct {
	LineItems []wooLineItem `json:"line_items"`
	Amount    string        `json:"amount"`
}

func newNovelRefundRateCmd(flags *rootFlags) *cobra.Command {
	var flagWindow string
	var flagLimit int
	var flagMinUnits int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "refund-rate",
		Short: "Refunded units over sold units for every product",
		Long: "Joins refunds back to the line items of their parent orders so return-prone products surface\n" +
			"by rate rather than raw count. Refunds and orders are separate endpoints and neither carries a\n" +
			"per-product rate, so this requires the local mirror.\n\n" +
			"Use this command to find which products are refunded most often, as a rate against units sold.\n" +
			"Do NOT use this command to explain a revenue change over time; use 'revenue explain' instead.",
		Example: "  woocommerce-pp-cli refund-rate --window 90d --limit 20 --agent",
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:happy-args":  "--window=90d",
			"pp:data-source": "local",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would compute refund rates from the local mirror")
				return nil
			}
			if err := rejectLiveDataSource(flags, "refund-rate"); err != nil {
				return err
			}
			window, err := windowOrDefault(flagWindow, "90d")
			if err != nil {
				return err
			}
			if flagLimit <= 0 {
				return usageErr(fmt.Errorf("--limit must be greater than zero"))
			}
			if flagMinUnits < 0 {
				return usageErr(fmt.Errorf("--min-units cannot be negative"))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			dbPath = resolveDBPath(dbPath)
			if localStoreMissing(cmd, flags, dbPath, "orders,order-refunds") {
				return nil
			}
			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			if !hintIfUnsynced(cmd, db, "orders") {
				hintIfStale(cmd, db, "orders", flags.maxAge)
			}

			// Stage 1: drain orders, accumulating sold units inside the window.
			orders, err := loadWooOrders(ctx, db)
			if err != nil {
				return err
			}
			cutoff := time.Now().UTC().Add(-window)
			sold := make(map[string]int)
			inWindow := make(map[string]bool)
			scanned := 0
			for _, o := range orders {
				t, ok := o.EffectiveTime()
				if !ok || t.Before(cutoff) {
					continue
				}
				// Match the status gate the other analytics commands use, so
				// `scanned_orders` and the sold denominator are comparable.
				if !revenueBearingStatuses[o.Status] {
					continue
				}
				scanned++
				inWindow[o.ID] = true
				for _, li := range o.LineItems {
					if li.ProductID == 0 {
						continue
					}
					sold[fmt.Sprintf("%d", li.ProductID)] += li.Quantity
				}
			}

			// Stage 2: parent rows are closed; attribute refunds.
			refunded, refundValue, attribution, err := attributeRefunds(ctx, db, orders, inWindow)
			if err != nil {
				return err
			}
			labels, err := loadProductLabels(ctx, db)
			if err != nil {
				return err
			}

			view := buildRefundRateView(sold, refunded, refundValue, labels, flagWindow, attribution, scanned, flagLimit, flagMinUnits)
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&flagWindow, "window", "90d", "Look-back window (90d, 12w, 2160h)")
	cmd.Flags().IntVar(&flagLimit, "limit", 50, "Maximum products returned")
	cmd.Flags().IntVar(&flagMinUnits, "min-units", 1, "Ignore products with fewer than this many units sold")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// attributeRefunds prefers per-line-item attribution from the order_refunds
// table. When that table is empty it falls back to distributing each order's
// refund total across its line items proportionally by value, and says so.
func attributeRefunds(ctx context.Context, db *store.Store, orders []wooOrder, inWindow map[string]bool) (map[string]int, map[string]float64, string, error) {
	units := make(map[string]int)
	value := make(map[string]float64)

	rows, err := db.DB().QueryContext(ctx, `SELECT COALESCE(parent_id,''), COALESCE(data,'') FROM order_refunds`)
	if err == nil {
		found := false
		for rows.Next() {
			var parent, raw string
			if err := rows.Scan(&parent, &raw); err != nil {
				_ = rows.Close()
				return units, value, "line_item", fmt.Errorf("scanning refund: %w", err)
			}
			if !inWindow[parent] || raw == "" {
				continue
			}
			var r refundLineItems
			if err := json.Unmarshal([]byte(raw), &r); err != nil {
				continue
			}
			for _, li := range r.LineItems {
				if li.ProductID == 0 {
					continue
				}
				pid := fmt.Sprintf("%d", li.ProductID)
				units[pid] += absInt(li.Quantity)
				value[pid] += math.Abs(parseMoney(li.Total))
				found = true
			}
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return units, value, "line_item", fmt.Errorf("iterating refunds: %w", err)
		}
		if err := rows.Close(); err != nil {
			return units, value, "line_item", fmt.Errorf("closing refund rows: %w", err)
		}
		if found {
			return units, value, "line_item", nil
		}
	}

	units, value = proportionalRefundAttribution(orders, inWindow)
	return units, value, "proportional", nil
}

// proportionalRefundAttribution distributes each order's refund total across
// its line items by value when no per-item refund breakdown exists.
func proportionalRefundAttribution(orders []wooOrder, inWindow map[string]bool) (map[string]int, map[string]float64) {
	units := make(map[string]int)
	value := make(map[string]float64)
	for _, o := range orders {
		if !inWindow[o.ID] {
			continue
		}
		var total float64
		for _, r := range o.Refunds {
			total += math.Abs(parseMoney(r.Total))
		}
		if total == 0 {
			continue
		}
		var lineTotal float64
		for _, li := range o.LineItems {
			lineTotal += parseMoney(li.Total)
		}
		if lineTotal <= 0 {
			continue
		}
		// The refunded FRACTION of the order, not the line's share of it.
		// Scaling units by the line share marks every unit refunded whenever a
		// line spans the whole order, however small the refund was.
		refundFraction := total / lineTotal
		if refundFraction > 1 {
			refundFraction = 1
		}
		for _, li := range o.LineItems {
			if li.ProductID == 0 {
				continue
			}
			pid := fmt.Sprintf("%d", li.ProductID)
			value[pid] += parseMoney(li.Total) * refundFraction
			units[pid] += int(math.Round(float64(li.Quantity) * refundFraction))
		}
	}
	return units, value
}

// buildRefundRateView is pure so the rate math stays unit-testable.
func buildRefundRateView(sold, refunded map[string]int, refundValue map[string]float64,
	labels map[string]productLabel, windowLabel, attribution string, scanned, limit, minUnits int) refundRateView {

	items := make([]refundRateItem, 0, len(sold))
	anyRefund := false
	for pid, units := range sold {
		if units < minUnits {
			continue
		}
		r := refunded[pid]
		if r > 0 {
			anyRefund = true
		}
		var rate float64
		if units > 0 {
			rate = float64(r) / float64(units)
		}
		it := refundRateItem{
			ProductID:     pid,
			UnitsSold:     units,
			UnitsRefunded: r,
			RefundRate:    round4(rate),
			RefundedValue: round2(refundValue[pid]),
		}
		if l, ok := labels[pid]; ok {
			it.Name, it.SKU = l.Name, l.SKU
		}
		items = append(items, it)
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].RefundRate != items[j].RefundRate {
			return items[i].RefundRate > items[j].RefundRate
		}
		return items[i].UnitsRefunded > items[j].UnitsRefunded
	})
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}

	view := refundRateView{
		Window:        windowLabel,
		Attribution:   attribution,
		ScannedOrders: scanned,
		Items:         items,
	}
	switch {
	case scanned == 0:
		view.Note = fmt.Sprintf("no orders in the last %s; widen --window or run 'woocommerce-pp-cli sync --resources orders'", windowLabel)
	case !anyRefund:
		view.Note = fmt.Sprintf("no refunds recorded in the last %s", windowLabel)
	}
	return view
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
