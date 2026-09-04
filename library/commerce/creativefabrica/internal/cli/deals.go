// Hand-authored transcendence: rank on-sale items by true discount depth.
// pp:data-source live
package cli

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/creativefabrica/internal/cliutil"
	"github.com/spf13/cobra"
)

type dealView struct {
	productView
	DiscountPct  float64 `json:"discount_pct"`
	SavingsValue float64 `json:"savings"`
}

func newNovelDealsCmd(flags *rootFlags) *cobra.Command {
	var itemType, category string
	var pod bool
	var limit, maxScanPages int
	cmd := &cobra.Command{
		Use:   "deals [query]",
		Short: "Rank on-sale items by their actual regular-to-sale discount depth",
		Long: `Find the deepest genuine discounts. Unlike 'find --on-sale' (which only filters
on the promotions flag), this computes each item's real regular-to-sale percent
drop locally and ranks by it.

Use this for the deepest real discounts. To simply filter on-sale items use
'find --on-sale'.`,
		Example:     strings.Trim("\n  creativefabrica-pp-cli deals \"font bundle\" --agent\n  creativefabrica-pp-cli deals --type Graphics --limit 15 --select name,price,regular_price,discount_pct", "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			query := ""
			if len(args) > 0 {
				query = args[0]
			}
			// Curtail pagination under live-dogfood to stay within the shared
			// public search key's rate limit.
			if cliutil.IsDogfoodEnv() && maxScanPages > 1 {
				maxScanPages = 1
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c := newAlgoliaClient(flags)
			q := catalogQuery{query: query, itemType: itemType, category: category, onSale: true, pod: pod, sortBy: "relevance"}

			var deals []dealView
			scanned := 0
			for page := 0; page < maxScanPages && len(deals) < limit*3+limit; page++ {
				req := q.request()
				req.Page = page
				req.HitsPerPage = 100
				results, err := c.Search(ctx, req)
				if err != nil {
					return apiErr(err)
				}
				if len(results) == 0 || len(results[0].Hits) == 0 {
					break
				}
				for _, h := range results[0].Hits {
					scanned++
					reg, err := strconv.ParseFloat(strings.TrimSpace(h.RegularPrice.String()), 64)
					price := h.Price.Float()
					if err != nil || reg <= 0 || price >= reg {
						continue
					}
					pct := (reg - price) / reg * 100
					deals = append(deals, dealView{
						productView:  toView(h),
						DiscountPct:  round1(pct),
						SavingsValue: round2(reg - price),
					})
				}
				if page+1 >= results[0].NbPages {
					break
				}
			}
			sort.SliceStable(deals, func(i, j int) bool { return deals[i].DiscountPct > deals[j].DiscountPct })
			if limit > 0 && len(deals) > limit {
				deals = deals[:limit]
			}
			if flags.asJSON || flags.agent || !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return flags.printJSON(cmd, deals)
			}
			rows := make([][]string, 0, len(deals))
			for _, d := range deals {
				rows = append(rows, []string{
					truncate(d.Name, 40),
					fmt.Sprintf("%.0f%%", d.DiscountPct),
					"$" + strconv.FormatFloat(d.Price, 'f', 2, 64),
					"$" + d.RegularPrice,
					d.ObjectID,
				})
			}
			return flags.printTable(cmd, []string{"NAME", "OFF", "PRICE", "WAS", "ID"}, rows)
		},
	}
	cmd.Flags().StringVar(&itemType, "type", "", "Product type filter")
	cmd.Flags().StringVar(&category, "category", "", "Category filter")
	cmd.Flags().BoolVar(&pod, "pod", false, "Only POD / commercial-license deals")
	cmd.Flags().IntVar(&limit, "limit", 20, "Max deals to return")
	cmd.Flags().IntVar(&maxScanPages, "max-scan-pages", 3, "Max list pages to scan for discounts")
	return cmd
}

func round1(f float64) float64 { return float64(int(f*10+0.5)) / 10 }
func round2(f float64) float64 { return float64(int(f*100+0.5)) / 100 }
