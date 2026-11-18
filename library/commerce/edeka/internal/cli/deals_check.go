// Copyright 2026 JimPresting and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"
	"io"
	"sort"
	"sync"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/edeka/internal/edeka"
)

func newNovelDealsCheckCmd(flags *rootFlags) *cobra.Command {
	var market string
	var perTerm int
	cmd := &cobra.Command{
		Use:   "check <term> [term...]",
		Short: "Which of these items are on offer in your market this week.",
		Long: "Use this command for \"are these items on offer\". Do NOT use it to browse all offers; use 'offers'.\n\n" +
			"Runs the market's search once per term and keeps the hits that carry the shop's offer badge.\n" +
			"Works without login.",
		Example:     "  edeka-pp-cli deals check milch kaffee butter --market doerflinger-ulm --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "term=milch;--market=doerflinger-ulm"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			cmd.SetContext(ctx)
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "deals check")
			}
			if perTerm < 1 || perTerm > 32 {
				return usageErr(fmt.Errorf("--per-term must be between 1 and 32"))
			}
			if len(args) > 20 {
				return usageErr(fmt.Errorf("at most 20 terms per call"))
			}
			sel, err := openPublicShop(market)
			if err != nil {
				return err
			}
			// Anonymous shop: the shop serialises requests of one PHP session,
			// and offer badges do not depend on the login.
			sh, err := edeka.OpenWith(&edeka.Session{}, "", sel.Host)
			if err != nil {
				return err
			}
			type termResult struct {
				Term   string          `json:"term"`
				Offers []edeka.Product `json:"offers"`
			}
			// one search page per term, up to four at a time (each page is a few MB)
			out := make([]termResult, len(args))
			errs := make([]error, len(args))
			seen := make([][]edeka.Product, len(args))
			var wg sync.WaitGroup
			sem := make(chan struct{}, 4)
			for i, term := range args {
				wg.Add(1)
				go func(i int, term string) {
					defer wg.Done()
					sem <- struct{}{}
					defer func() { <-sem }()
					g, err := sh.Search(cmd.Context(), term, 0)
					if err != nil {
						errs[i] = err
						return
					}
					if !anyTitleMatches(term, g.Products) {
						g.Products = nil // unrelated fallback results
					}
					tr := termResult{Term: term, Offers: []edeka.Product{}}
					var offers []edeka.Product
					for _, p := range g.Products {
						if p.OnOffer {
							offers = append(offers, p)
						}
					}
					// best matches first ("butter" before "peanut butter cups");
					// the shop's order breaks ties
					sort.SliceStable(offers, func(a, b int) bool {
						return edeka.Relevance(term, offers[a]) > edeka.Relevance(term, offers[b])
					})
					if len(offers) > perTerm {
						offers = offers[:perTerm]
					}
					tr.Offers = append(tr.Offers, offers...)
					out[i] = tr
					seen[i] = g.Products
				}(i, term)
			}
			wg.Wait()
			total := 0
			for i := range out {
				if errs[i] != nil {
					return edekaErr(errs[i])
				}
				total += len(out[i].Offers)
				recordPrices(cmd.Context(), seen[i], sh.Host, "search")
			}
			res := map[string]any{"market": sh.Host, "terms": out, "offers_found": total}
			matched := false
			for i := range seen {
				if len(seen[i]) > 0 {
					matched = true
				}
			}
			if !matched {
				return emitThen(cmd, flags, res, notFoundErr(fmt.Errorf("none of the terms matches a product in market %s", sh.Host)))
			}
			return emit(cmd, flags, res, func(w io.Writer) {
				for _, tr := range out {
					if len(tr.Offers) == 0 {
						fmt.Fprintf(w, "%s: no offer\n", tr.Term)
						continue
					}
					fmt.Fprintf(w, "%s:\n", tr.Term)
					printProducts(w, tr.Offers)
				}
			})
		},
	}
	cmd.Flags().IntVar(&perTerm, "per-term", 5, "Maximum offers listed per term")
	cmd.Flags().StringVar(&market, "market", "", marketFlagUsage)
	return cmd
}
