// Copyright 2026 educrvz and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written novel command: basket snapshot diff.
// Snapshots /cart/summary items to local SQLite and diffs across cycles.
// pp:data-source auto

package cli

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/mvanhorn/printing-press-library/library/commerce/shopper/internal/store"
	"github.com/spf13/cobra"
)

type basketDiffResult struct {
	Status          string                `json:"status"`
	Note            string                `json:"note,omitempty"`
	FromSnapshotID  int64                 `json:"from_snapshot_id,omitempty"`
	ToSnapshotID    int64                 `json:"to_snapshot_id,omitempty"`
	FromTakenAt     string                `json:"from_taken_at,omitempty"`
	ToTakenAt       string                `json:"to_taken_at,omitempty"`
	Added           []diffItem            `json:"added"`
	Removed         []diffItem            `json:"removed"`
	QuantityChanged []quantityChangedItem `json:"quantity_changed"`
	PriceChanged    []priceChangedItem    `json:"price_changed"`
}

type diffItem struct {
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	Qty   float64 `json:"qty"`
	Price float64 `json:"price,omitempty"`
}

type quantityChangedItem struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	QtyFrom  float64 `json:"qty_from"`
	QtyTo    float64 `json:"qty_to"`
	QtyDelta float64 `json:"qty_delta"`
}

type priceChangedItem struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	PriceFrom  float64 `json:"price_from"`
	PriceTo    float64 `json:"price_to"`
	PriceDelta float64 `json:"price_delta"`
	PctChange  float64 `json:"pct_change"`
}

func newNovelBasketDiffCmd(flags *rootFlags) *cobra.Command {
	var flagFrom string
	var flagTo string

	cmd := &cobra.Command{
		Use:     "diff",
		Short:   "Compares your current recurring basket against a previous cycle's snapshot to show exactly what was added, dropped",
		Example: "  shopper-pp-cli basket diff --json",
		Long: `Snapshots your current /cart/summary items and diffs them against the previous
snapshot stored locally.

On first run: captures a baseline snapshot and reports "baseline captured".
On subsequent runs: shows what was added, removed, or had quantity/price changes
since the previous snapshot.`,
		Annotations: map[string]string{
			"mcp:read-only":          "true",
			"pp:no-error-path-probe": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), `{"dry_run":true,"would":"snapshot /cart/summary and diff against last snapshot"}`)
				return nil
			}

			if flagFrom == "" {
				flagFrom = "last-snapshot"
			}
			if flagTo == "" {
				flagTo = "current"
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			cartData, err := c.Get(cmd.Context(), "/cart/summary", nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			currentItems := extractCartItems(cartData)

			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("shopper-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			defer db.Close()

			count, err := store.CountCartSnapshots(db.DB())
			if err != nil {
				return fmt.Errorf("counting snapshots: %w", err)
			}

			if count == 0 {
				newID, snapErr := store.SnapshotCart(db.DB(), currentItems)
				if snapErr != nil {
					return fmt.Errorf("saving baseline snapshot: %w", snapErr)
				}
				result := basketDiffResult{
					Status:          "baseline_captured",
					Note:            "Baseline snapshot captured (snapshot #" + fmt.Sprintf("%d", newID) + "). Run again next cycle to see changes.",
					ToSnapshotID:    newID,
					Added:           make([]diffItem, 0),
					Removed:         make([]diffItem, 0),
					QuantityChanged: make([]quantityChangedItem, 0),
					PriceChanged:    make([]priceChangedItem, 0),
				}
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}

			latestSnaps, err := store.LatestCartSnapshots(db.DB(), 1)
			if err != nil {
				return fmt.Errorf("reading latest snapshot: %w", err)
			}
			if len(latestSnaps) == 0 {
				return fmt.Errorf("snapshot count was %d but no rows were returned; remove %s and re-run to rebuild the baseline", count, defaultDBPath("shopper-pp-cli"))
			}
			fromSnap := latestSnaps[0]

			newID, snapErr := store.SnapshotCart(db.DB(), currentItems)
			if snapErr != nil {
				return fmt.Errorf("saving current snapshot: %w", snapErr)
			}

			result := diffCartSnapshots(fromSnap.Items, currentItems)
			result.FromSnapshotID = fromSnap.ID
			result.ToSnapshotID = newID
			result.FromTakenAt = fromSnap.TakenAt.Format("2006-01-02T15:04:05Z")
			result.ToTakenAt = "now"

			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&flagFrom, "from", "last-snapshot", "Reference snapshot for comparison (default: last-snapshot)")
	cmd.Flags().StringVar(&flagTo, "to", "current", "Target snapshot for comparison (default: current live cart)")
	return cmd
}

func extractCartItems(data json.RawMessage) []store.CartSnapshotItem {
	var top map[string]json.RawMessage
	if json.Unmarshal(data, &top) != nil {
		return nil
	}

	var rawItems []json.RawMessage
	for _, key := range []string{"items", "data", "products", "cart_items", "cartItems"} {
		if raw, ok := top[key]; ok {
			if json.Unmarshal(raw, &rawItems) == nil && len(rawItems) > 0 {
				break
			}
		}
	}

	items := make([]store.CartSnapshotItem, 0, len(rawItems))
	for _, raw := range rawItems {
		var obj map[string]any
		if json.Unmarshal(raw, &obj) != nil {
			continue
		}
		item := store.CartSnapshotItem{
			ID:    extractAnyString(obj, "id", "productId", "product_id"),
			Name:  extractAnyString(obj, "name", "title", "description"),
			Qty:   extractAnyFloat(obj, "qty", "quantity", "count"),
			Price: extractAnyFloat(obj, "price", "unit_price", "preco"),
		}
		if item.ID == "" {
			continue
		}
		items = append(items, item)
	}
	return items
}

func extractAnyString(obj map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := obj[k]; ok {
			switch t := v.(type) {
			case string:
				return t
			case float64:
				return fmt.Sprintf("%.0f", t)
			case int:
				return fmt.Sprintf("%d", t)
			}
		}
	}
	return ""
}

func extractAnyFloat(obj map[string]any, keys ...string) float64 {
	for _, k := range keys {
		if v, ok := obj[k]; ok {
			switch t := v.(type) {
			case float64:
				return t
			case string:
				var f float64
				_, _ = fmt.Sscanf(t, "%f", &f) // scan success inferred from zero-value default; #nosec G104
				return f
			}
		}
	}
	return 0
}

func diffCartSnapshots(from, to []store.CartSnapshotItem) basketDiffResult {
	result := basketDiffResult{
		Status:          "diff",
		Added:           make([]diffItem, 0),
		Removed:         make([]diffItem, 0),
		QuantityChanged: make([]quantityChangedItem, 0),
		PriceChanged:    make([]priceChangedItem, 0),
	}

	fromMap := make(map[string]store.CartSnapshotItem, len(from))
	for _, item := range from {
		fromMap[item.ID] = item
	}
	toMap := make(map[string]store.CartSnapshotItem, len(to))
	for _, item := range to {
		toMap[item.ID] = item
	}

	for id, item := range toMap {
		if _, exists := fromMap[id]; !exists {
			result.Added = append(result.Added, diffItem{ID: id, Name: item.Name, Qty: item.Qty, Price: item.Price})
		}
	}

	for id, item := range fromMap {
		if _, exists := toMap[id]; !exists {
			result.Removed = append(result.Removed, diffItem{ID: id, Name: item.Name, Qty: item.Qty, Price: item.Price})
		}
	}

	for id, toItem := range toMap {
		fromItem, exists := fromMap[id]
		if !exists {
			continue
		}
		if fromItem.Qty != toItem.Qty {
			result.QuantityChanged = append(result.QuantityChanged, quantityChangedItem{
				ID: id, Name: toItem.Name,
				QtyFrom: fromItem.Qty, QtyTo: toItem.Qty, QtyDelta: toItem.Qty - fromItem.Qty,
			})
		}
		if fromItem.Price > 0 && toItem.Price > 0 && math.Abs(fromItem.Price-toItem.Price) > 0.005 {
			pct := (toItem.Price - fromItem.Price) / fromItem.Price * 100
			result.PriceChanged = append(result.PriceChanged, priceChangedItem{
				ID: id, Name: toItem.Name,
				PriceFrom: fromItem.Price, PriceTo: toItem.Price,
				PriceDelta: toItem.Price - fromItem.Price,
				PctChange:  math.Round(pct*100) / 100,
			})
		}
	}

	if len(result.Added) == 0 && len(result.Removed) == 0 &&
		len(result.QuantityChanged) == 0 && len(result.PriceChanged) == 0 {
		result.Status = "no_changes"
	}

	return result
}
