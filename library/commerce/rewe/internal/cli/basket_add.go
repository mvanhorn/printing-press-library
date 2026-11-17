// Copyright 2026 JimPresting and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/rewe/internal/rewe"
)

func newNovelBasketAddCmd(flags *rootFlags) *cobra.Command {
	var qty int
	cmd := &cobra.Command{
		Use:   "add [query]",
		Short: "Add products by search text and quantity without knowing listing ids.",
		Long: "Searches the selected market for the text, picks the best matching orderable product and\n" +
			"sets its quantity in the basket. --dry-run shows the chosen product without changing the basket.",
		Example:     "  rewe-pp-cli basket add hafermilch --qty 2 --json\n  rewe-pp-cli basket add \"ja! Butter\" --dry-run",
		Annotations: map[string]string{"pp:data-source": "live", "pp:happy-args": "query=hafermilch;--qty=1", "pp:live-happy-path": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if done, err := verifyDryRun(cmd, flags, "basket add"); done {
				return err
			}
			if len(args) == 0 {
				return cmd.Help()
			}
			if qty < 1 || qty > 99 {
				return usageErr(fmt.Errorf("--qty must be between 1 and 99"))
			}
			query := strings.Join(args, " ")
			sh, c, err := openShop(cmd, flags)
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			products, err := sh.SearchProducts(ctx, query, 20)
			if err != nil {
				return reweErr(err)
			}
			recordPrices(ctx, products, c.SelectedMarket)
			p, ok := rewe.BestMatch(query, products)
			if !ok {
				return notFoundErr(fmt.Errorf("no orderable product matches %q in market %s", query, c.SelectedMarket))
			}
			out := map[string]any{"query": query, "product": p, "quantity": qty, "added": false}
			if flags.dryRun {
				return emit(cmd, flags, out, func(w io.Writer) {
					fmt.Fprintf(w, "Would add %dx %s (%s each)\n", qty, p.Name, rewe.FormatCents(p.PriceCents))
				})
			}
			b, err := sh.SetQuantity(ctx, p.ListingID, qty)
			if err != nil {
				return reweErr(err)
			}
			_ = sh.Persist()
			out["added"] = true
			out["basket"] = viewBasket(b)
			return emit(cmd, flags, out, func(w io.Writer) {
				fmt.Fprintf(w, "Added %dx %s (%s each). Basket total %s.\n", qty, p.Name, rewe.FormatCents(p.PriceCents), rewe.FormatCents(b.Summary.TotalPrice))
			})
		},
	}
	cmd.Flags().IntVar(&qty, "qty", 1, "Quantity to set")
	return cmd
}
