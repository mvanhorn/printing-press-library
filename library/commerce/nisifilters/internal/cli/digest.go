// Copyright 2026 chiotas and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored novel command: digest. Summarizes the mirrored site from the
// local SQLite store — content counts by type, top categories, portfolio and
// product totals, product price range, and the newest content — none of which
// any single upstream endpoint returns.

// pp:data-source local

package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

func newNovelDigestCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "digest",
		Short: "Summarize the mirrored site: counts by type, top categories, portfolio and product totals, price range, newest content",
		Long: "Summarize the locally-mirrored site in one shot: content counts per type,\n" +
			"the busiest post categories, portfolio and shop totals, the product price\n" +
			"range, and the newest piece of content. Computed entirely from the local\n" +
			"store — run `sync` first.",
		Example: "  nisifilters-pp-cli digest\n" +
			"  nisifilters-pp-cli digest --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := fgOpenStore(cmd.Context())
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			defer db.Close()

			counts, err := db.Status()
			if err != nil {
				return fmt.Errorf("reading store status: %w", err)
			}

			total := 0
			for _, n := range counts {
				total += n
			}

			view := map[string]any{
				"counts":      counts,
				"total_items": total,
			}

			// Top categories by their WordPress count field.
			type catCount struct {
				Name  string `json:"name"`
				Count int    `json:"count"`
			}
			var cats []catCount
			if rows, err := db.List("categories", 1000); err == nil {
				for _, raw := range rows {
					obj := fgDecode(raw)
					name := fgString(obj, "name")
					n, _ := fgInt(obj, "count")
					if name != "" {
						cats = append(cats, catCount{Name: name, Count: n})
					}
				}
			}
			sort.Slice(cats, func(i, j int) bool {
				if cats[i].Count != cats[j].Count {
					return cats[i].Count > cats[j].Count
				}
				return cats[i].Name < cats[j].Name
			})
			if len(cats) > 10 {
				cats = cats[:10]
			}
			if len(cats) > 0 {
				view["top_categories"] = cats
			}

			// Product price range.
			if rows, err := db.List("products", 2000); err == nil && len(rows) > 0 {
				var min, max float64
				var currency string
				have := false
				for _, raw := range rows {
					obj := fgDecode(raw)
					if disp, val, ok := fgWooPrice(obj["prices"]); ok {
						if !have || val < min {
							min = val
						}
						if !have || val > max {
							max = val
						}
						if currency == "" {
							currency = fgWooCurrency(disp)
						}
						have = true
					}
				}
				prod := map[string]any{"count": len(rows)}
				if have {
					prod["price_min"] = round2(min)
					prod["price_max"] = round2(max)
					if currency != "" {
						prod["currency"] = currency
					}
				}
				view["products"] = prod
			}

			// Newest content across posts + portfolio (ISO timestamps sort lexically).
			newest := ""
			newestRef := ""
			for _, rt := range []string{"posts", "pages"} {
				rows, err := db.List(rt, 2000)
				if err != nil {
					continue
				}
				for _, raw := range rows {
					obj := fgDecode(raw)
					m := fgString(obj, "modified")
					if m == "" {
						m = fgString(obj, "date")
					}
					if m > newest {
						newest = m
						newestRef = fmt.Sprintf("%s: %s", rt, fgPlainTitle(obj))
					}
				}
			}
			if newest != "" {
				view["newest"] = map[string]any{"modified": newest, "item": newestRef}
			}

			if total == 0 {
				view["note"] = fgEmptyStoreNote("content")
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}

			w := cmd.OutOrStdout()
			if total == 0 {
				fmt.Fprintln(w, fgEmptyStoreNote("content"))
				return nil
			}
			fmt.Fprintln(w, bold("Site digest"))
			fmt.Fprintf(w, "Total mirrored items: %d\n\n", total)
			fmt.Fprintln(w, "Counts by type:")
			keys := make([]string, 0, len(counts))
			for k := range counts {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				fmt.Fprintf(w, "  %-20s %d\n", k, counts[k])
			}
			if len(cats) > 0 {
				fmt.Fprintln(w, "\nTop categories:")
				for _, c := range cats {
					fmt.Fprintf(w, "  %-30s %d\n", c.Name, c.Count)
				}
			}
			if p, ok := view["products"].(map[string]any); ok {
				fmt.Fprintf(w, "\nShop: %v products", p["count"])
				if mn, ok := p["price_min"]; ok {
					fmt.Fprintf(w, " (%v–%v %v)", mn, p["price_max"], p["currency"])
				}
				fmt.Fprintln(w)
			}
			if newest != "" {
				fmt.Fprintf(w, "\nNewest: %s (%s)\n", newestRef, newest)
			}
			return nil
		},
	}
	return cmd
}

// fgWooCurrency extracts the currency code from a formatted "120.00 EUR" string.
func fgWooCurrency(display string) string {
	for i := len(display) - 1; i >= 0; i-- {
		if display[i] == ' ' {
			return display[i+1:]
		}
	}
	return ""
}

func round2(f float64) float64 {
	return float64(int64(f*100+0.5)) / 100
}
