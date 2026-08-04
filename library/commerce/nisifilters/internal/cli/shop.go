// Copyright 2026 chiotas and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored novel command: shop. Lists WooCommerce Store products joined
// with their categories — name, price, category, stock — from the local mirror
// (live fallback when not synced), sorted by price or name.

// pp:data-source auto

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/nisifilters/internal/cliutil"

	"github.com/spf13/cobra"
)

func newNovelShopCmd(flags *rootFlags) *cobra.Command {
	var flagSort string
	var flagCategory string
	var limit int

	cmd := &cobra.Command{
		Use:   "shop",
		Short: "List shop products joined with their categories, showing name, price, category, and stock, sorted by price",
		Long: "List WooCommerce shop products (prints / workshops) with price, categories,\n" +
			"and stock. Reads the local mirror first (run `sync`); falls back to a live\n" +
			"fetch. Sort by price (default) or name; filter by a category name substring\n" +
			"or numeric category ID.",
		Example: "  nisifilters-pp-cli shop\n" +
			"  nisifilters-pp-cli shop --sort name --json\n" +
			"  nisifilters-pp-cli shop --category prints --limit 10",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			switch flagSort {
			case "", "price", "name":
			default:
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--sort must be price or name"))
			}
			if flagSort == "" {
				flagSort = "price"
			}
			if cliutil.IsDogfoodEnv() && limit > 10 {
				limit = 10
			}

			db, _ := fgOpenStore(cmd.Context())
			if db != nil {
				defer db.Close()
			}

			var rows []json.RawMessage
			if db != nil {
				if r, err := db.List("products", 2000); err == nil {
					rows = r
				}
			}
			if len(rows) == 0 {
				c, err := flags.newClient()
				if err != nil {
					return err
				}
				perPage := limit
				if perPage < 1 || perPage > 100 {
					perPage = 50
				}
				raw, gerr := c.Get(cmd.Context(), fgWooProductsURL, map[string]string{
					"per_page": strconv.Itoa(perPage),
				})
				if gerr != nil {
					return fmt.Errorf("fetching products: %w", gerr)
				}
				rows = fgSplitArray(raw)
			}

			type product struct {
				ID       int    `json:"id"`
				Name     string `json:"name"`
				Price    string `json:"price"`
				priceVal float64
				hasPrice bool
				InStock  bool     `json:"in_stock"`
				Stock    string   `json:"stock"` // in | backorder | out
				OnSale   bool     `json:"on_sale"`
				Category []string `json:"categories"`
				catIDs   []int
				Link     string `json:"permalink"`
			}

			var products []product
			for _, raw := range rows {
				obj := fgDecode(raw)
				// ponytail: slug-prefix heuristic — the Store API exposes no
				// catalog_visibility, so uncategorized "test-*" placeholders
				// (e.g. test-split-1) are filtered by shape; revisit if the
				// shop ever publishes a real uncategorized product named test*.
				if strings.HasPrefix(fgString(obj, "slug"), "test-") && len(fgCategoryNames(obj)) == 0 {
					continue
				}
				p := product{
					Name:     fgPlainTitle(obj),
					InStock:  fgBool(obj, "is_in_stock"),
					Stock:    fgStockStatus(obj),
					OnSale:   fgBool(obj, "on_sale"),
					Category: fgCategoryNames(obj),
					catIDs:   fgProductCategoryIDs(obj),
					Link:     fgString(obj, "permalink"),
				}
				if id, ok := fgInt(obj, "id"); ok {
					p.ID = id
				}
				if disp, val, ok := fgWooPrice(obj["prices"]); ok {
					p.Price = disp
					p.priceVal = val
					p.hasPrice = true
				}
				products = append(products, p)
			}

			// Category filter (numeric ID or name substring).
			if flagCategory != "" {
				var wantID int
				numeric := false
				if n, err := strconv.Atoi(flagCategory); err == nil {
					wantID = n
					numeric = true
				}
				needle := strings.ToLower(flagCategory)
				filtered := products[:0]
				for _, p := range products {
					keep := false
					if numeric {
						keep = containsInt(p.catIDs, wantID)
					} else {
						for _, name := range p.Category {
							if strings.Contains(strings.ToLower(name), needle) {
								keep = true
								break
							}
						}
					}
					if keep {
						filtered = append(filtered, p)
					}
				}
				products = filtered
			}

			sort.SliceStable(products, func(i, j int) bool {
				if flagSort == "name" {
					return strings.ToLower(products[i].Name) < strings.ToLower(products[j].Name)
				}
				// price: priced items first, ascending; unpriced sink to the end.
				if products[i].hasPrice != products[j].hasPrice {
					return products[i].hasPrice
				}
				return products[i].priceVal < products[j].priceVal
			})

			if limit > 0 && len(products) > limit {
				products = products[:limit]
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				view := map[string]any{"count": len(products), "products": products}
				if len(products) == 0 {
					view["note"] = fgEmptyStoreNote("products")
				}
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}

			w := cmd.OutOrStdout()
			if len(products) == 0 {
				fmt.Fprintln(w, fgEmptyStoreNote("products"))
				return nil
			}
			tw := newTabWriter(w)
			fmt.Fprintln(tw, "PRICE\tSTOCK\tCATEGORY\tNAME")
			for _, p := range products {
				stock := p.Stock
				price := p.Price
				if price == "" {
					price = "—"
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", price, stock, strings.Join(p.Category, ", "), p.Name)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&flagSort, "sort", "price", "Sort by: price or name")
	cmd.Flags().StringVar(&flagCategory, "category", "", "Filter by category name substring or numeric category ID")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum products to return")
	return cmd
}

// fgProductCategoryIDs returns the category term IDs of a WooCommerce product.
func fgProductCategoryIDs(m map[string]json.RawMessage) []int {
	raw, ok := m["categories"]
	if !ok {
		return nil
	}
	var cats []struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal(raw, &cats); err != nil {
		return nil
	}
	out := make([]int, 0, len(cats))
	for _, c := range cats {
		out = append(out, c.ID)
	}
	return out
}
