// Copyright 2026 JimPresting and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/rewe/internal/rewe"
)

type listResult struct {
	Item    rewe.ListItem `json:"item"`
	Product *rewe.Product `json:"product,omitempty"`
	Error   string        `json:"error,omitempty"`
}

func newNovelListAddCmd(flags *rootFlags) *cobra.Command {
	var file, text string
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Turn a shopping list (one item per line, optional quantity) into basket items.",
		Long: "Reads a shopping list from --file (or '-' for stdin) or --text. Each line is one item, with an\n" +
			"optional quantity like '2x Hafermilch', '3 Bananen' or 'Butter 2'. Every item is searched in\n" +
			"the selected market, the best match is chosen, and all matches go into the basket in one call.\n" +
			"--dry-run shows the matches only. Unmatched lines are reported, never silently dropped.",
		Example:     "  rewe-pp-cli list add --text \"2x Hafermilch\" --json\n  rewe-pp-cli list add --file einkauf.txt --dry-run",
		Annotations: map[string]string{"pp:data-source": "live", "pp:happy-args": "--text=Hafermilch", "pp:live-happy-path": "true", "mcp:read-flags": "file"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if done, err := verifyDryRun(cmd, flags, "list add"); done {
				return err
			}
			if file == "" && text == "" {
				return cmd.Help()
			}
			src := text
			if file != "" {
				var b []byte
				var err error
				if file == "-" {
					b, err = io.ReadAll(cmd.InOrStdin())
				} else {
					b, err = os.ReadFile(file)
				}
				if err != nil {
					return usageErr(err)
				}
				src = string(b)
			}
			items := rewe.ParseList(src)
			if len(items) == 0 {
				return usageErr(fmt.Errorf("the list has no items"))
			}
			if len(items) > 60 {
				return usageErr(fmt.Errorf("the list has %d items; at most 60 per run", len(items)))
			}
			sh, c, err := openShop(cmd, flags)
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			results := []listResult{}
			lineItems := []rewe.LineItem{}
			for _, it := range items {
				ps, err := sh.SearchProducts(ctx, it.Query, 20)
				if err != nil {
					return reweErr(err)
				}
				recordPrices(ctx, ps, c.SelectedMarket)
				p, ok := rewe.BestMatch(it.Query, ps)
				if !ok {
					results = append(results, listResult{Item: it, Error: "no matching product"})
					continue
				}
				pc := p
				results = append(results, listResult{Item: it, Product: &pc})
				lineItems = append(lineItems, rewe.LineItem{ListingID: p.ListingID, Quantity: it.Quantity})
			}
			out := map[string]any{"results": results, "matched": len(lineItems), "unmatched": len(items) - len(lineItems), "added": false}
			if !flags.dryRun && len(lineItems) > 0 {
				res, err := sh.AddMany(ctx, lineItems, "shopping-list")
				if err != nil {
					return reweErr(err)
				}
				_ = sh.Persist()
				out["added"] = true
				out["add_result"] = res
			}
			return emit(cmd, flags, out, func(w io.Writer) {
				for _, r := range results {
					if r.Product == nil {
						fmt.Fprintf(w, "  ? %-30s no match\n", r.Item.Query)
						continue
					}
					fmt.Fprintf(w, "  %2dx %-28s -> %-40s %s\n", r.Item.Quantity, trunc(r.Item.Query, 28), trunc(r.Product.Name, 40), rewe.FormatCents(r.Product.PriceCents))
				}
				fmt.Fprintf(w, "%d matched, %d unmatched, added: %v\n", out["matched"], out["unmatched"], out["added"])
			})
		},
	}
	cmd.Flags().StringVar(&file, "file", "", "Shopping list file ('-' for stdin)")
	cmd.Flags().StringVar(&text, "text", "", "Shopping list text (newline separated)")
	return cmd
}
