// Copyright 2026 JimPresting and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/rewe/internal/rewe"
)

type dealMatch struct {
	Offer   rewe.Offer `json:"offer"`
	Product string     `json:"matches_product"`
	Source  string     `json:"source"`
}

func newNovelDealsMineCmd(flags *rootFlags) *cobra.Command {
	var extra []string
	cmd := &cobra.Command{
		Use:   "mine",
		Short: "Offers that match products you bought before or saved as favourites.",
		Long: "Joins this market's offers with your 'Bereits gekauft' products, your favourites and any\n" +
			"--also terms (e.g. preferences an agent remembers). A match needs a shared significant word\n" +
			"(4+ letters) between the offer title and the product name.",
		Example:     "  rewe-pp-cli deals mine --json\n  rewe-pp-cli deals mine --also kaffee --also hafermilch",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "deals mine")
			}
			sh, _, err := openShop(cmd, flags)
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			names := map[string]string{} // product name -> source
			var fav []struct {
				Items []struct {
					ProductID string `json:"productId"`
					Title     string `json:"title"`
					Name      string `json:"name"`
				} `json:"items"`
			}
			if err := sh.JSON(ctx, http.MethodGet, "/shop/api/favorites", nil, &fav, nil); err == nil {
				for _, l := range fav {
					for _, it := range l.Items {
						if n := firstNonEmpty(it.Title, it.Name); n != "" {
							names[n] = "favorite"
						}
					}
				}
			}
			var mine struct {
				MyProducts []json.RawMessage `json:"myProducts"`
			}
			if err := sh.JSON(ctx, http.MethodGet, "/shop/api/myproducts", nil, &mine, nil); err == nil {
				for _, raw := range mine.MyProducts {
					var p struct {
						ProductName string `json:"productName"`
						Title       string `json:"title"`
						Name        string `json:"name"`
					}
					if json.Unmarshal(raw, &p) == nil {
						if n := firstNonEmpty(p.ProductName, p.Title, p.Name); n != "" {
							names[n] = "bought"
						}
					}
				}
			}
			for _, e := range extra {
				names[e] = "preference"
			}
			page, err := sh.OffersURL(ctx)
			if err != nil {
				return reweErr(err)
			}
			offers, err := sh.Offers(ctx, page, 400)
			if err != nil && len(offers) == 0 {
				return reweErr(err)
			}
			_ = recordOffers(ctx, offers)
			matches := matchDeals(offers, names)
			return emit(cmd, flags, matches, func(w io.Writer) {
				if len(names) == 0 {
					fmt.Fprintln(w, "No purchased products, favourites or --also terms yet.")
				}
				for _, m := range matches {
					fmt.Fprintf(w, "%-40s %-9s  (matches %s: %s)\n", trunc(m.Offer.Title, 40), m.Offer.Price, m.Source, trunc(m.Product, 40))
				}
				fmt.Fprintf(w, "%d matching offers\n", len(matches))
			})
		},
	}
	cmd.Flags().StringArrayVar(&extra, "also", nil, "Extra preference term to match (repeatable)")
	return cmd
}

func firstNonEmpty(s ...string) string {
	for _, x := range s {
		if strings.TrimSpace(x) != "" {
			return x
		}
	}
	return ""
}

var dealStop = map[string]bool{"rewe": true, "beste": true, "wahl": true, "stück": true, "sorten": true, "versch": true}

func sigWords(s string) []string {
	out := []string{}
	for _, w := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return !(r >= 'a' && r <= 'z' || r == 'ä' || r == 'ö' || r == 'ü' || r == 'ß') }) {
		if len([]rune(w)) >= 4 && !dealStop[w] {
			out = append(out, w)
		}
	}
	return out
}

func matchDeals(offers []rewe.Offer, names map[string]string) []dealMatch {
	out := []dealMatch{}
	seen := map[string]bool{}
	for _, o := range offers {
		ow := sigWords(o.Title)
		for name, src := range names {
			hit := false
			for _, a := range ow {
				for _, b := range sigWords(name) {
					if a == b || strings.Contains(a, b) || strings.Contains(b, a) {
						hit = true
					}
				}
			}
			if hit && !seen[o.NAN+o.Week] {
				seen[o.NAN+o.Week] = true
				out = append(out, dealMatch{Offer: o, Product: name, Source: src})
			}
		}
	}
	return out
}
