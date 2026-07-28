// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/woocommerce/internal/store"
)

// maxProjectedCoverDays bounds the stockout projection. Ten years of cover is
// already "never" for a merchant, and the bound keeps the float->Duration
// conversion inside int64 range.
const maxProjectedCoverDays = 3650.0

type velocityItem struct {
	ProductID     string   `json:"product_id"`
	VariationID   string   `json:"variation_id,omitempty"`
	Name          string   `json:"name"`
	SKU           string   `json:"sku,omitempty"`
	UnitsSold     int      `json:"units_sold"`
	UnitsPerDay   float64  `json:"units_per_day"`
	StockQuantity *int64   `json:"stock_quantity"`
	DaysOfCover   *float64 `json:"days_of_cover"`
	StockoutDate  string   `json:"stockout_date,omitempty"`
}

type velocityView struct {
	Window        string         `json:"window"`
	WindowDays    float64        `json:"window_days"`
	ScannedOrders int            `json:"scanned_orders"`
	Items         []velocityItem `json:"items"`
	Note          string         `json:"note,omitempty"`
}

// soldKey identifies the stock-keeping unit demand is accumulated against.
// WooCommerce tracks stock per variation when one exists, otherwise per product.
type soldKey struct {
	productID   string
	variationID string
}

func newNovelStockVelocityCmd(flags *rootFlags) *cobra.Command {
	var flagWindow string
	var flagLimit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "velocity",
		Short: "Units sold per day per variation divided into current stock",
		Long: "Aggregates order line items over a window to get units sold per day, then divides current\n" +
			"stock by that rate to produce days-of-cover and a reorder list sorted by stockout date.\n" +
			"WooCommerce exposes stock but never a demand denominator, so this needs the local mirror.\n\n" +
			"Use this command to decide what to reorder.\n" +
			"Do NOT use this command to find catalog defects like missing prices; use 'catalog audit' instead.",
		Example: "  woocommerce-pp-cli stock velocity --window 30d --agent",
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:happy-args":  "--window=30d",
			"pp:data-source": "local",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would compute stock velocity from the local mirror")
				return nil
			}
			if err := rejectLiveDataSource(flags, "stock velocity"); err != nil {
				return err
			}
			window, err := windowOrDefault(flagWindow, "30d")
			if err != nil {
				return err
			}
			if flagLimit <= 0 {
				return usageErr(fmt.Errorf("--limit must be greater than zero"))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			dbPath = resolveDBPath(dbPath)
			if localStoreMissing(cmd, flags, dbPath, "orders,products") {
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

			// Stage 1: drain orders fully before touching products.
			orders, err := loadWooOrders(ctx, db)
			if err != nil {
				return err
			}
			cutoff := time.Now().UTC().Add(-window)
			sold := make(map[soldKey]int)
			scanned := 0
			for _, o := range orders {
				t, ok := o.EffectiveTime()
				if !ok || t.Before(cutoff) {
					continue
				}
				if !revenueBearingStatuses[o.Status] {
					continue
				}
				scanned++
				for _, li := range o.LineItems {
					if li.ProductID == 0 && li.VariationID == 0 {
						continue
					}
					k := soldKey{productID: fmt.Sprintf("%d", li.ProductID)}
					if li.VariationID != 0 {
						k.variationID = fmt.Sprintf("%d", li.VariationID)
					}
					sold[k] += li.Quantity
				}
			}

			// Stage 2: parent rows are closed, so follow-up queries are safe now.
			stock, labels, err := loadStockLevels(ctx, db)
			variationsMissing := errors.Is(err, errVariationsUnavailable)
			if err != nil && !variationsMissing {
				return err
			}

			view := buildVelocityView(sold, stock, labels, window, flagWindow, scanned, flagLimit, time.Now().UTC())
			if variationsMissing && view.Note == "" {
				view.Note = "variations are not synced, so results are product-level only; run 'woocommerce-pp-cli sync --resources variations' for per-variation cover"
			}
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&flagWindow, "window", "30d", "Look-back window for demand (30d, 12w, 720h)")
	cmd.Flags().IntVar(&flagLimit, "limit", 50, "Maximum items returned")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// loadStockLevels reads current stock for products and variations. Variation
// stock is keyed by variation id; product stock by product id.
// errVariationsUnavailable signals that the variations table could not be read,
// so variation-level results are absent rather than genuinely empty.
var errVariationsUnavailable = errors.New("variations table unavailable")

func loadStockLevels(ctx context.Context, db *store.Store) (map[soldKey]int64, map[soldKey]productLabel, error) {
	stock := make(map[soldKey]int64)
	labels := make(map[soldKey]productLabel)

	rows, err := db.DB().QueryContext(ctx, `
		SELECT id, COALESCE(name,''), COALESCE(sku,''), stock_quantity FROM products`)
	if err == nil {
		for rows.Next() {
			var id, name, sku string
			var qty sql.NullInt64
			if err := rows.Scan(&id, &name, &sku, &qty); err != nil {
				_ = rows.Close()
				return stock, labels, fmt.Errorf("scanning product stock: %w", err)
			}
			k := soldKey{productID: id}
			labels[k] = productLabel{Name: name, SKU: sku}
			if qty.Valid {
				stock[k] = qty.Int64
			}
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return stock, labels, fmt.Errorf("iterating product stock: %w", err)
		}
		if err := rows.Close(); err != nil {
			return stock, labels, fmt.Errorf("closing product stock rows: %w", err)
		}
	}

	vrows, err := db.DB().QueryContext(ctx, `
		SELECT id, COALESCE(parent_id,''),
		       COALESCE(json_extract(data,'$.sku'),''),
		       json_extract(data,'$.stock_quantity')
		FROM variations`)
	if err != nil {
		// Variations may not be synced yet; product-level velocity still works.
		// Reported rather than swallowed so the caller can say so.
		return stock, labels, errVariationsUnavailable
	}
	for vrows.Next() {
		var id, parent, sku string
		var qty sql.NullInt64
		if err := vrows.Scan(&id, &parent, &sku, &qty); err != nil {
			_ = vrows.Close()
			return stock, labels, fmt.Errorf("scanning variation stock: %w", err)
		}
		k := soldKey{productID: parent, variationID: id}
		parentLabel := labels[soldKey{productID: parent}]
		labels[k] = productLabel{Name: parentLabel.Name, SKU: sku}
		if qty.Valid {
			stock[k] = qty.Int64
		}
	}
	if err := vrows.Err(); err != nil {
		_ = vrows.Close()
		return stock, labels, fmt.Errorf("iterating variation stock: %w", err)
	}
	if err := vrows.Close(); err != nil {
		return stock, labels, fmt.Errorf("closing variation stock rows: %w", err)
	}
	return stock, labels, nil
}

// buildVelocityView is pure so the velocity math stays unit-testable.
func buildVelocityView(sold map[soldKey]int, stock map[soldKey]int64, labels map[soldKey]productLabel,
	window time.Duration, windowLabel string, scanned, limit int, now time.Time) velocityView {

	days := window.Hours() / 24
	if days <= 0 {
		days = 1
	}
	items := make([]velocityItem, 0, len(sold))
	for k, units := range sold {
		perDay := float64(units) / days
		it := velocityItem{
			ProductID:   k.productID,
			VariationID: k.variationID,
			UnitsSold:   units,
			UnitsPerDay: round4(perDay),
		}
		if l, ok := labels[k]; ok {
			it.Name, it.SKU = l.Name, l.SKU
		} else if l, ok := labels[soldKey{productID: k.productID}]; ok {
			it.Name, it.SKU = l.Name, l.SKU
		}
		if q, ok := stock[k]; ok {
			qq := q
			it.StockQuantity = &qq
			if perDay > 0 {
				cover := float64(q) / perDay
				c := round2(cover)
				it.DaysOfCover = &c
				// Only project a date when it is meaningful. Beyond ~10 years the
				// number is noise, and converting a huge float to time.Duration
				// overflows int64 (implementation-defined: saturates on arm64,
				// wraps negative on amd64, yielding a stockout date in 1677).
				if cover >= 0 && cover <= maxProjectedCoverDays {
					it.StockoutDate = now.Add(time.Duration(cover * float64(24*time.Hour))).Format("2006-01-02")
				}
			}
		}
		items = append(items, it)
	}

	// Soonest stockout first; unknown cover sinks to the bottom.
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i].DaysOfCover, items[j].DaysOfCover
		switch {
		case a != nil && b != nil && *a != *b:
			return *a < *b
		case a != nil && b == nil:
			return true
		case a == nil && b != nil:
			return false
		}
		return items[i].UnitsSold > items[j].UnitsSold
	})
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}

	view := velocityView{
		Window:        windowLabel,
		WindowDays:    round2(days),
		ScannedOrders: scanned,
		Items:         items,
	}
	if scanned == 0 {
		view.Note = fmt.Sprintf("no revenue-bearing orders in the last %s, so velocity cannot be computed; widen --window or run 'woocommerce-pp-cli sync --resources orders'", windowLabel)
	}
	return view
}
