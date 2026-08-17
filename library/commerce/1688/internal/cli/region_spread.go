// Hand-authored transcendence command. Not generator-emitted.
// pp:data-source local
package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

type regionStat struct {
	Province          string  `json:"province"`
	OfferCount        int     `json:"offer_count"`
	PriceMin          float64 `json:"price_min"`
	PriceMedian       float64 `json:"price_median"`
	PriceMax          float64 `json:"price_max"`
	TotalTransactions int     `json:"total_transactions"`
}

func median(sorted []float64) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	if n%2 == 1 {
		return sorted[n/2]
	}
	return (sorted[n/2-1] + sorted[n/2]) / 2
}

func newNovelRegionSpreadCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:         "region-spread <keyword>",
		Short:       "Group stored offers for a keyword by Chinese province and report min, median, and max price per region",
		Long:        "Group stored offers for a keyword by supplier province and report offer count, min/median/max price, and total transactions per region, surfacing cheaper sourcing geographies. Reads the local store; run 'sync <keyword>' first.",
		Example:     "  1688-pp-cli region-spread 手机壳",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would group stored offers by province")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("keyword is required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if dbPath == "" {
				dbPath = defaultDBPath("1688-pp-cli")
			}
			db, err := openLocalStore(ctx, cmd, flags, dbPath)
			if err != nil {
				return err
			}
			if db == nil {
				return nil
			}
			defer db.Close()

			raws, err := db.OffersByKeyword(ctx, args[0], 0)
			if err != nil {
				return err
			}
			offers := decodeStoredOffers(raws)

			order := []string{}
			prices := map[string][]float64{}
			txByProvince := map[string]int{}
			countByProvince := map[string]int{}
			for _, o := range offers {
				prov := o.Province
				if prov == "" {
					prov = "(unknown)"
				}
				if _, ok := countByProvince[prov]; !ok {
					order = append(order, prov)
				}
				countByProvince[prov]++
				if o.PriceCNY > 0 {
					prices[prov] = append(prices[prov], o.PriceCNY)
				}
				txByProvince[prov] += o.TransactionCount
			}

			stats := make([]regionStat, 0, len(order))
			for _, prov := range order {
				ps := append([]float64(nil), prices[prov]...)
				sort.Float64s(ps)
				st := regionStat{
					Province:          prov,
					OfferCount:        countByProvince[prov],
					TotalTransactions: txByProvince[prov],
				}
				if len(ps) > 0 {
					st.PriceMin = ps[0]
					st.PriceMax = ps[len(ps)-1]
					st.PriceMedian = median(ps)
				}
				stats = append(stats, st)
			}
			sort.SliceStable(stats, func(i, j int) bool {
				return stats[i].OfferCount > stats[j].OfferCount
			})
			if len(stats) == 0 {
				return emit(cmd, flags, map[string]any{
					"keyword": args[0],
					"results": []regionStat{},
					"note":    fmt.Sprintf("no stored offers for %q; run: 1688-pp-cli sync %s", args[0], args[0]),
				})
			}
			return emit(cmd, flags, stats)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store database path")
	return cmd
}
