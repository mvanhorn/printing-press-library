// Copyright 2026 JimPresting and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/edeka/internal/edeka"
)

func newNovelBasketAddCmd(flags *rootFlags) *cobra.Command {
	var qty, grams int
	var market string
	cmd := &cobra.Command{
		Use:   "add <article-id-or-words>",
		Short: "Add a product by id or by plain words; the CLI picks the best match in your market and adds it.",
		Long: "Use this command to add an item by product id or by free text. Do NOT use it to add a whole list; use 'list add' instead.\n\n" +
			"With an article id (from 'products search') the article is added directly. With words the market's search runs\n" +
			"and the best-matching product is added. The quantity is added on top of what is already in the basket.\n" +
			"Weight articles (meat and cheese counter) need --grams. --dry-run shows the choice without changing the basket.",
		Example:     "  edeka-pp-cli basket add hafermilch --qty 2 --dry-run\n  edeka-pp-cli basket add 5915407005MXST --qty 1",
		Annotations: map[string]string{"mcp:read-only": "false", "pp:data-source": "live", "pp:happy-args": "query=milch;--qty=1", "pp:live-happy-path": "true", "pp:verified-by-proof": "basket-add-lifecycle.md"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			cmd.SetContext(ctx)
			if done, err := verifyDryRun(cmd, flags, "basket add"); done {
				return err
			}
			if len(args) == 0 {
				return cmd.Help()
			}
			if qty < 1 || qty > 99 {
				return usageErr(fmt.Errorf("--qty must be between 1 and 99"))
			}
			sh, err := openShop(cmd, market)
			if err != nil {
				return err
			}
			res, err := addToBasket(cmd.Context(), sh, joinArgs(args), qty, grams, flags.dryRun)
			_ = sh.Persist()
			if err != nil {
				return err
			}
			return emit(cmd, flags, res, func(w io.Writer) { printAddResult(w, res) })
		},
	}
	cmd.Flags().IntVar(&qty, "qty", 1, "How many to add (1-99)")
	cmd.Flags().IntVar(&grams, "grams", 0, "Weight in grams for weight articles (meat/cheese counter)")
	cmd.Flags().StringVar(&market, "market", "", marketFlagUsage)
	return cmd
}

// addResult is the outcome of one basket add (also used by list add).
type addResult struct {
	Query        string         `json:"query"`
	Product      *edeka.Product `json:"product,omitempty"`
	Quantity     int            `json:"quantity"`
	NewQuantity  int            `json:"new_quantity"`
	Added        bool           `json:"added"`
	DryRun       bool           `json:"dry_run,omitempty"`
	Error        string         `json:"error,omitempty"`
	BasketTotal  string         `json:"basket_total,omitempty"`
	BasketPieces int            `json:"basket_pieces,omitempty"`
}

// addToBasket resolves query (article id or words) and adds qty.
func addToBasket(ctx context.Context, sh *edeka.Shop, query string, qty, grams int, dryRun bool) (*addResult, error) {
	res := &addResult{Query: query, Quantity: qty, DryRun: dryRun}
	var p edeka.Product
	if edeka.LooksLikeArticleID(query) {
		p = edeka.Product{ID: query}
		b, err := sh.Basket(ctx)
		if err != nil {
			return nil, edekaErr(err)
		}
		for _, it := range b.Items {
			if it.ID == query {
				p.InBasket = it.Quantity
				p.Title = it.Title
			}
		}
		if p.Title == "" {
			if d, err := sh.Product(ctx, query); err == nil {
				p.Title, p.PriceCents, p.Price, p.URL = d.Title, d.PriceCents, d.Price, d.URL
			} else {
				return nil, notFoundErr(fmt.Errorf("article %s not found in market %s", query, sh.Host))
			}
		}
	} else {
		g, err := sh.Search(ctx, query, 0)
		if err != nil {
			return nil, edekaErr(err)
		}
		recordPrices(ctx, g.Products, sh.Host, "search")
		best, ok := edeka.BestMatch(query, g.Products)
		if alt := edeka.AlternativeQuery(query); alt != "" && (!ok || edeka.IsNonFood(best.URL)) {
			if g2, err := sh.Search(ctx, alt, 0); err == nil {
				recordPrices(ctx, g2.Products, sh.Host, "search")
				if b2, ok2 := edeka.BestMatch(alt, g2.Products); ok2 && !edeka.IsNonFood(b2.URL) {
					best, ok = b2, true
				}
			}
		}
		if !ok {
			return nil, notFoundErr(fmt.Errorf("no product matches %q in market %s", query, sh.Host))
		}
		p = best
	}
	if p.WeightArticle && grams <= 0 {
		return nil, usageErr(fmt.Errorf("%s is sold by weight: pass --grams (for example --grams 250)", p.Title))
	}
	res.Product = &p
	res.NewQuantity = p.InBasket + qty
	if dryRun {
		return res, nil
	}
	r, err := sh.SetQuantity(ctx, p.ID, res.NewQuantity, grams)
	if err != nil {
		return nil, edekaErr(err)
	}
	res.Added = true
	res.BasketTotal = edeka.FormatCents(int(r.BruttoSum*100 + 0.5))
	res.BasketPieces = r.ProductCount
	return res, nil
}

func printAddResult(w io.Writer, r *addResult) {
	if r.Product == nil {
		fmt.Fprintf(w, "%q: %s\n", r.Query, r.Error)
		return
	}
	verb := "Added"
	if r.DryRun {
		verb = "Would add"
	}
	fmt.Fprintf(w, "%s %dx %s (%s) -> %d in basket", verb, r.Quantity, r.Product.Title, r.Product.Price, r.NewQuantity)
	if r.BasketTotal != "" {
		fmt.Fprintf(w, ", basket total %s", r.BasketTotal)
	}
	fmt.Fprintln(w)
}
