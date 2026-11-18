// Copyright 2026 JimPresting and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"fmt"
	"io"
	"regexp"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/rewe/internal/rewe"
)

// productIDRe matches REWE product ids: a known id with no snapshots yet is an
// empty history, while an unknown name is reported as not found.
var productIDRe = regexp.MustCompile(`^[0-9]{5,10}$`)

type priceRow struct {
	ProductID  string `json:"product_id"`
	Name       string `json:"name"`
	PriceCents int    `json:"price_cents"`
	Price      string `json:"price"`
	Source     string `json:"source"`
	Market     string `json:"market"`
	SeenAt     string `json:"seen_at"`
}

func newNovelPriceHistoryCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "history [product]",
		Short: "Price changes of a product seen in searches and offers over time.",
		Long: "Reads the local price snapshots that products search, basket add, list add and offers record.\n" +
			"Pass a product id (e.g. 7381513) or part of a name. Only changes are printed (consecutive\n" +
			"identical prices are collapsed).",
		Example:     "  rewe-pp-cli price history 7381513 --json\n  rewe-pp-cli price history hafer",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local", "pp:happy-args": "product=7381513"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "price history")
			}
			db, err := openReweDB(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()
			rows, err := db.DB().QueryContext(cmd.Context(), `SELECT product_id, COALESCE(name,''), price_cents, source, COALESCE(market,''), seen_at FROM rewe_prices
WHERE product_id = ? OR product_id = ? OR name LIKE ? ORDER BY product_id, seen_at`, args[0], "nan:"+args[0], "%"+args[0]+"%")
			if err != nil {
				return err
			}
			defer rows.Close()
			out := []priceRow{}
			last := map[string]int{}
			for rows.Next() {
				var r priceRow
				if err := rows.Scan(&r.ProductID, &r.Name, &r.PriceCents, &r.Source, &r.Market, &r.SeenAt); err != nil {
					return err
				}
				if p, ok := last[r.ProductID]; ok && p == r.PriceCents {
					continue
				}
				last[r.ProductID] = r.PriceCents
				r.Price = rewe.FormatCents(r.PriceCents)
				out = append(out, r)
			}
			if len(out) == 0 && !productIDRe.MatchString(args[0]) {
				return notFoundErr(fmt.Errorf("no prices recorded for %q yet; run products search, basket add or offers first", args[0]))
			}
			if limit > 0 && len(out) > limit {
				out = out[len(out)-limit:]
			}
			return emit(cmd, flags, out, func(w io.Writer) {
				if len(out) == 0 {
					fmt.Fprintln(w, "No prices recorded yet; run products search, basket add or offers first.")
				}
				for _, r := range out {
					fmt.Fprintf(w, "%s  %-40s %9s  (%s)\n", r.SeenAt[:10], trunc(r.Name, 40), r.Price, r.Source)
				}
			})
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "Show at most the last N changes")
	return cmd
}
