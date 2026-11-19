// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/store"
	"github.com/spf13/cobra"
)

func newNovelInventoryAuditCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var threshold float64
	cmd := &cobra.Command{
		Use:         "audit",
		Short:       "Spot zero-quantity variations, missing or duplicate SKUs and price spread across variations.",
		Example:     "  etsy-pp-cli inventory audit --agent",
		Long:        "Use this command for structural gaps (SKUs, price spread, zero-quantity offerings). Do NOT use it for a simple low-stock list; use 'inventory low-stock' instead.",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "inventory audit")
			}
			if threshold < 0 || math.IsNaN(threshold) || math.IsInf(threshold, 0) {
				return usageErr(fmt.Errorf("--spread-threshold must be finite and >= 0"))
			}
			db, err := shopxOpenDB(cmd, dbPath)
			if err != nil {
				return err
			}
			if db != nil {
				defer db.Close()
			}
			familyNotes := shopxCommandNotes(db, "listings_inventory")
			rows, notes := shopxInventoryFindings(db, shopxShopID(cmd), threshold)
			notes = append(familyNotes, notes...)
			return shopxEmit(cmd, flags, rows, notes)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	cmd.Flags().Float64Var(&threshold, "spread-threshold", 0.5, "Minimum relative price spread (strictly greater)")
	return cmd
}

type shopxInventoryFinding map[string]any

type shopxInventoryPrice struct {
	value           float64
	amount, divisor int64
	currency        string
}

func shopxInventoryPriceString(p shopxInventoryPrice) string {
	// Preserve the precision implied by the Money divisor, including zero-decimal currencies.
	digits := 0
	for d := p.divisor; d > 1; d /= 10 {
		digits++
	}
	return strconv.FormatFloat(p.value, 'f', digits, 64)
}

func shopxInventoryFindings(db *store.Store, shop string, spreadThreshold float64) ([]shopxInventoryFinding, []string) {
	if shop == "" {
		return []shopxInventoryFinding{}, []string{shopxNoteShopUnset}
	}
	notes := make([]string, 0)
	listings, byID, dropped := shopxListingsByID(db, shop)
	if dropped > 0 {
		notes = append(notes, shopxForeignNote(dropped))
	}
	if len(listings) == 0 && len(shopxRows(db, shopxInventoryTypes...)) > 0 {
		return []shopxInventoryFinding{}, append(notes, "listings not synced; cannot attribute inventory rows to your shop")
	}
	out := make([]shopxInventoryFinding, 0)
	seen := make(map[[2]int64]bool)
	skus := make(map[string][][2]int64)
	spellings := make(map[[2]int64]string)
	prices := make(map[int64][]shopxInventoryPrice)
	for _, row := range shopxRows(db, shopxInventoryTypes...) {
		lid := shopxLinkedListingID(row)
		if _, ok := byID[lid]; !ok {
			continue
		}
		products := shopxList(row["products"])
		if _, ok := row["product_id"]; ok {
			products = []any{row}
		}
		for _, raw := range products {
			p := shopxMap(raw)
			pid, _ := shopxInt(p["product_id"])
			key := [2]int64{lid, pid}
			if seen[key] {
				continue
			}
			seen[key] = true
			if deleted, _ := shopxBool(p["is_deleted"]); deleted {
				continue
			}
			sku := shopxStr(p["sku"])
			if sku == "" {
				out = append(out, shopxInventoryFinding{"type": "empty_sku", "listing_id": lid, "product_id": pid})
			} else {
				normalized := strings.ToLower(sku)
				spellings[[2]int64{key[0], key[1]}] = sku
				skus[normalized] = append(skus[normalized], key)
			}
			for _, rawOffering := range shopxList(p["offerings"]) {
				o := shopxMap(rawOffering)
				if deleted, _ := shopxBool(o["is_deleted"]); deleted {
					continue
				}
				if enabled, present := shopxBool(o["is_enabled"]); present && !enabled {
					continue
				}
				if qty, present := shopxInt(o["quantity"]); present && qty == 0 {
					oid, _ := shopxInt(o["offering_id"])
					out = append(out, shopxInventoryFinding{"type": "zero_quantity", "listing_id": lid, "product_id": pid, "sku": sku, "offering_id": oid})
				}
				if a, d, c, ok := shopxMoney(o["price"]); ok {
					prices[lid] = append(prices[lid], shopxInventoryPrice{float64(a) / float64(d), a, d, c})
				}
			}
		}
	}
	for _, pairs := range skus {
		if len(pairs) < 2 {
			continue
		}
		// Report the spelling of the lowest (listing_id, product_id) pair so the
		// output never depends on mirror scan order.
		first := pairs[0]
		for _, pair := range pairs[1:] {
			if pair[0] < first[0] || (pair[0] == first[0] && pair[1] < first[1]) {
				first = pair
			}
		}
		lids, pids := make([]int64, 0), make([]int64, 0)
		unique := make(map[int64]bool)
		for _, pair := range pairs {
			if !unique[pair[0]] {
				lids = append(lids, pair[0])
				unique[pair[0]] = true
			}
			pids = append(pids, pair[1])
		}
		sort.Slice(lids, func(i, j int) bool { return lids[i] < lids[j] })
		sort.Slice(pids, func(i, j int) bool { return pids[i] < pids[j] })
		out = append(out, shopxInventoryFinding{"type": "duplicate_sku", "sku": spellings[first], "listing_ids": lids, "product_ids": pids})
	}
	for lid, ps := range prices {
		min, max := ps[0], ps[0]
		for _, p := range ps[1:] {
			if p.value < min.value {
				min = p
			}
			if p.value > max.value {
				max = p
			}
		}
		if min.value <= 0 {
			continue
		}
		spread := (max.value - min.value) / min.value
		if spread > spreadThreshold {
			out = append(out, shopxInventoryFinding{"type": "price_spread", "listing_id": lid, "min_price": shopxInventoryPriceString(min), "max_price": shopxInventoryPriceString(max), "spread": math.Round(spread*1e4) / 1e4, "currency": min.currency})
		}
	}
	order := map[string]int{"zero_quantity": 0, "empty_sku": 1, "duplicate_sku": 2, "price_spread": 3}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if order[a["type"].(string)] != order[b["type"].(string)] {
			return order[a["type"].(string)] < order[b["type"].(string)]
		}
		al, _ := shopxInt(a["listing_id"])
		bl, _ := shopxInt(b["listing_id"])
		if al != bl {
			return al < bl
		}
		ap, _ := shopxInt(a["product_id"])
		bp, _ := shopxInt(b["product_id"])
		if ap != bp {
			return ap < bp
		}
		if a["type"] == "duplicate_sku" {
			return shopxStr(a["sku"]) < shopxStr(b["sku"])
		}
		ao, _ := shopxInt(a["offering_id"])
		bo, _ := shopxInt(b["offering_id"])
		return ao < bo
	})
	return out, notes
}
