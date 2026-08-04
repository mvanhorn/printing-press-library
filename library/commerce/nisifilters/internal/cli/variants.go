// Copyright 2026 chiotas and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored novel command: variants. Lists the WooCommerce variations of a
// variable product — variation id, attribute values, price, and stock — which
// are invisible through wp-json/wp/v2 (it only exposes parent products). The
// parent's variation ids come from the Store API product object; each
// variation is then fetched as its own Store API product to get per-variant
// price and availability. Accepts a numeric product id or a product URL.

// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelVariantsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "variants <product-id-or-url>",
		Short: "List the variations of a variable shop product with per-variant price and stock",
		Long: "List the WooCommerce variations of a variable product: variation id,\n" +
			"attribute values (e.g. filter size), price, and stock status\n" +
			"(in / backorder / out). These are invisible via wp-json/wp/v2, which only\n" +
			"exposes parent products.\n\n" +
			"Accepts a numeric product id or a full product URL\n" +
			"(https://nisifilters.it/prodotto/<slug>/). Always fetches live — variation\n" +
			"stock and price change too often to trust a mirror.",
		Example: "  nisifilters-pp-cli variants 278495\n" +
			"  nisifilters-pp-cli variants https://nisifilters.it/prodotto/jetmag-pro-anelli-adattatori/\n" +
			"  nisifilters-pp-cli variants 278495 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a product id or URL is required"))
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()

			// Resolve the argument to a parent product object.
			var parent map[string]json.RawMessage
			arg := args[0]
			if _, aerr := strconv.Atoi(arg); aerr == nil {
				raw, gerr := c.Get(ctx, fgWooProductsURL+"/"+arg, nil)
				if gerr != nil {
					return notFoundErr(fmt.Errorf("no product with id %s: %w", arg, gerr))
				}
				parent = fgDecode(raw)
			} else {
				slug, serr := fgProductSlugFromURL(arg)
				if serr != nil {
					_ = cmd.Usage()
					return usageErr(serr)
				}
				raw, gerr := c.Get(ctx, fgWooProductsURL, map[string]string{"slug": slug})
				if gerr != nil {
					return fmt.Errorf("looking up product by slug %q: %w", slug, gerr)
				}
				items := fgSplitArray(raw)
				if len(items) == 0 {
					return notFoundErr(fmt.Errorf("no product with slug %q", slug))
				}
				parent = fgDecode(items[0])
			}
			if _, ok := parent["id"]; !ok {
				return notFoundErr(fmt.Errorf("no product found for %q", arg))
			}
			// Slug queries (and user-supplied ids) can land on a variation
			// object; follow its parent to reach the variable product.
			if pid, ok := fgInt(parent, "parent"); ok && pid > 0 {
				raw, gerr := c.Get(ctx, fgWooProductsURL+"/"+strconv.Itoa(pid), nil)
				if gerr != nil {
					return fmt.Errorf("resolving parent product %d: %w", pid, gerr)
				}
				parent = fgDecode(raw)
			}
			parentID, _ := fgInt(parent, "id")

			// Variation ids + attribute values from the parent object.
			var varRefs []struct {
				ID         int `json:"id"`
				Attributes []struct {
					Name  string `json:"name"`
					Value string `json:"value"`
				} `json:"attributes"`
			}
			_ = json.Unmarshal(parent["variations"], &varRefs)
			if len(varRefs) == 0 {
				return notFoundErr(fmt.Errorf("product %d (%s) has no variations — it is a simple product; use `shop` or `read --type products`", parentID, fgPlainTitle(parent)))
			}

			type variant struct {
				ID         int      `json:"variation_id"`
				Attributes []string `json:"attributes"`
				Price      string   `json:"price"`
				Stock      string   `json:"stock"` // in | backorder | out
				InStock    bool     `json:"in_stock"`
				SKU        string   `json:"sku,omitempty"`
			}
			variants := make([]variant, 0, len(varRefs))
			for _, ref := range varRefs {
				v := variant{ID: ref.ID}
				for _, a := range ref.Attributes {
					v.Attributes = append(v.Attributes, a.Value)
				}
				// Each variation is itself a Store API product with its own
				// prices and availability.
				if raw, gerr := c.Get(ctx, fgWooProductsURL+"/"+strconv.Itoa(ref.ID), nil); gerr == nil {
					obj := fgDecode(raw)
					if disp, _, ok := fgWooPrice(obj["prices"]); ok {
						v.Price = disp
					}
					v.Stock = fgStockStatus(obj)
					v.InStock = fgBool(obj, "is_in_stock")
					v.SKU = fgString(obj, "sku")
				}
				variants = append(variants, v)
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"product_id": parentID,
					"name":       fgPlainTitle(parent),
					"permalink":  fgString(parent, "permalink"),
					"count":      len(variants),
					"variants":   variants,
				}, flags)
			}

			w := cmd.OutOrStdout()
			fmt.Fprintln(w, bold(fgPlainTitle(parent)))
			fmt.Fprintf(w, "%d variations · %s\n\n", len(variants), fgString(parent, "permalink"))
			tw := newTabWriter(w)
			fmt.Fprintln(tw, "VARIATION_ID\tATTRIBUTES\tPRICE\tSTOCK")
			for _, v := range variants {
				price := v.Price
				if price == "" {
					price = "—"
				}
				fmt.Fprintf(tw, "%d\t%s\t%s\t%s\n", v.ID, strings.Join(v.Attributes, " / "), price, v.Stock)
			}
			return tw.Flush()
		},
	}
	return cmd
}

// fgProductSlugFromURL extracts the product slug from a nisifilters.it product
// URL (…/prodotto/<slug>/…).
func fgProductSlugFromURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "", fmt.Errorf("argument must be a numeric product id or a product URL, got %q", raw)
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	for i, p := range parts {
		if p == "prodotto" && i+1 < len(parts) && parts[i+1] != "" {
			return parts[i+1], nil
		}
	}
	return "", fmt.Errorf("cannot find /prodotto/<slug>/ in URL %q", raw)
}
