// Copyright 2026 chiotas and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored novel command: filters. Discovers the filter sizes/types
// available across the NiSi catalog by aggregating product attributes from the
// local mirror, and finds products matching a given attribute value. No single
// storefront endpoint returns this cross-product view.

// pp:data-source auto

package cli

import (
	"encoding/json"
	"fmt"
	"html"
	"sort"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/nisifilters/internal/cliutil"

	"github.com/spf13/cobra"
)

func newNovelFiltersCmd(flags *rootFlags) *cobra.Command {
	var attrFlag string
	var valueFlag string
	var limit int

	cmd := &cobra.Command{
		Use:   "filters",
		Short: "Discover available filter sizes/types from product attributes and find matching products",
		Long: "Aggregate product attributes across the catalog to answer two questions:\n" +
			"what filter sizes/types exist (e.g. the Dimensione values 49mm, 82mm, 95mm),\n" +
			"and which products carry a given value. Reads the local mirror first\n" +
			"(run `sync`); falls back to a live fetch.\n\n" +
			"No flags: list every attribute and its values with product counts.\n" +
			"--attribute NAME: narrow to one attribute.\n" +
			"--attribute NAME --value VAL: list products that have that value.",
		Example: "  nisifilters-pp-cli filters\n" +
			"  nisifilters-pp-cli filters --attribute Dimensione --json\n" +
			"  nisifilters-pp-cli filters --attribute Dimensione --value 82mm",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if cliutil.IsDogfoodEnv() && limit > 20 {
				limit = 20
			}

			rows := fgLoadProducts(cmd, flags)
			if len(rows) == 0 {
				if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"attributes": []any{},
						"note":       fgEmptyStoreNote("products"),
					}, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), fgEmptyStoreNote("products"))
				return nil
			}

			attrNeedle := strings.ToLower(strings.TrimSpace(attrFlag))
			valNeedle := strings.ToLower(strings.TrimSpace(valueFlag))

			// Product-filter mode: --attribute + --value -> matching products.
			if attrNeedle != "" && valNeedle != "" {
				type prodRef struct {
					ID    int    `json:"id"`
					Name  string `json:"name"`
					Price string `json:"price,omitempty"`
					Link  string `json:"permalink,omitempty"`
				}
				matches := make([]prodRef, 0)
				for _, raw := range rows {
					obj := fgDecode(raw)
					if !productHasAttrValue(obj, attrNeedle, valNeedle) {
						continue
					}
					pr := prodRef{Name: fgPlainTitle(obj), Link: fgString(obj, "permalink")}
					if id, ok := fgInt(obj, "id"); ok {
						pr.ID = id
					}
					if disp, _, ok := fgWooPrice(obj["prices"]); ok {
						pr.Price = disp
					}
					matches = append(matches, pr)
					if limit > 0 && len(matches) >= limit {
						break
					}
				}
				if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"attribute": attrFlag,
						"value":     valueFlag,
						"count":     len(matches),
						"products":  matches,
					}, flags)
				}
				w := cmd.OutOrStdout()
				if len(matches) == 0 {
					fmt.Fprintf(w, "No products with %s = %q.\n", attrFlag, valueFlag)
					return nil
				}
				for _, m := range matches {
					price := m.Price
					if price == "" {
						price = "—"
					}
					fmt.Fprintf(w, "%-10s %s\n", price, m.Name)
				}
				return nil
			}

			// Discovery mode: aggregate attribute -> value -> product count.
			type valCount struct {
				Value string `json:"value"`
				Count int    `json:"count"`
			}
			counts := map[string]map[string]int{} // attr -> value -> count
			order := []string{}                   // attr first-seen order
			for _, raw := range rows {
				obj := fgDecode(raw)
				for attrName, values := range productAttrValues(obj) {
					if attrNeedle != "" && !strings.Contains(strings.ToLower(attrName), attrNeedle) {
						continue
					}
					if _, ok := counts[attrName]; !ok {
						counts[attrName] = map[string]int{}
						order = append(order, attrName)
					}
					for v := range values {
						counts[attrName][v]++
					}
				}
			}
			sort.Strings(order)

			type attrView struct {
				Name   string     `json:"name"`
				Values []valCount `json:"values"`
			}
			out := make([]attrView, 0, len(order))
			for _, name := range order {
				vc := make([]valCount, 0, len(counts[name]))
				for v, n := range counts[name] {
					vc = append(vc, valCount{Value: v, Count: n})
				}
				sort.Slice(vc, func(i, j int) bool {
					if vc[i].Count != vc[j].Count {
						return vc[i].Count > vc[j].Count
					}
					return vc[i].Value < vc[j].Value
				})
				out = append(out, attrView{Name: name, Values: vc})
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"attribute_count": len(out),
					"attributes":      out,
				}, flags)
			}
			w := cmd.OutOrStdout()
			if len(out) == 0 {
				fmt.Fprintln(w, "No matching attributes found.")
				return nil
			}
			for _, a := range out {
				fmt.Fprintln(w, bold(a.Name))
				for _, v := range a.Values {
					fmt.Fprintf(w, "  %-28s %d\n", v.Value, v.Count)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&attrFlag, "attribute", "", "Attribute name to narrow to (substring, e.g. Dimensione)")
	cmd.Flags().StringVar(&valueFlag, "value", "", "With --attribute, list products that have this value (substring)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum products to return in product-filter mode")
	return cmd
}

// fgLoadProducts returns products from the store, or the full live catalog when empty.
func fgLoadProducts(cmd *cobra.Command, flags *rootFlags) []json.RawMessage {
	if db, err := fgOpenStore(cmd.Context()); err == nil {
		defer db.Close()
		if rows, lerr := db.List("products", 5000); lerr == nil && len(rows) > 0 {
			return rows
		}
	}
	c, err := flags.newClient()
	if err != nil {
		return nil
	}
	const perPage = 100
	var all []json.RawMessage
	// ponytail: sequential page walk capped at 50 pages (5000 products, matching the store List cap)
	for page := 1; page <= 50; page++ {
		raw, gerr := c.Get(cmd.Context(), fgWooProductsURL, map[string]string{
			"per_page": strconv.Itoa(perPage),
			"page":     strconv.Itoa(page),
		})
		if gerr != nil || len(raw) == 0 {
			break
		}
		items := fgSplitArray(raw)
		all = append(all, items...)
		if len(items) < perPage {
			break
		}
	}
	return all
}

// productAttrValues returns a product's attributes as attrName -> set of term
// values, with HTML entities decoded.
func productAttrValues(m map[string]json.RawMessage) map[string]map[string]bool {
	out := map[string]map[string]bool{}
	raw, ok := m["attributes"]
	if !ok {
		return out
	}
	var attrs []struct {
		Name  string `json:"name"`
		Terms []struct {
			Name string `json:"name"`
		} `json:"terms"`
	}
	if err := json.Unmarshal(raw, &attrs); err != nil {
		return out
	}
	for _, a := range attrs {
		name := html.UnescapeString(strings.TrimSpace(a.Name))
		if name == "" {
			continue
		}
		if _, ok := out[name]; !ok {
			out[name] = map[string]bool{}
		}
		for _, t := range a.Terms {
			v := html.UnescapeString(strings.TrimSpace(t.Name))
			if v != "" {
				out[name][v] = true
			}
		}
	}
	return out
}

// productHasAttrValue reports whether a product has an attribute whose name
// contains attrNeedle and a term value containing valNeedle (both lowercased).
func productHasAttrValue(m map[string]json.RawMessage, attrNeedle, valNeedle string) bool {
	for name, values := range productAttrValues(m) {
		if !strings.Contains(strings.ToLower(name), attrNeedle) {
			continue
		}
		for v := range values {
			if strings.Contains(strings.ToLower(v), valNeedle) {
				return true
			}
		}
	}
	return false
}
