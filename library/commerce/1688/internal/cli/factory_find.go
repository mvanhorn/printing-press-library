// Hand-authored transcendence command. Not generator-emitted.
// pp:data-source local
package cli

import (
	"fmt"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/commerce/1688/internal/mtop"

	"github.com/spf13/cobra"
)

type scoredOffer struct {
	mtop.Offer
	FactoryConfidence float64 `json:"factory_confidence"`
	FactoryLabel      string  `json:"factory_label"`
}

// factoryScore weights verification flags, the 深度验厂 tag, reorder rate, and
// supplier trade score into one confidence number and a coarse label.
func factoryScore(o mtop.Offer) (float64, string) {
	s := 0.0
	if o.SuperFactory {
		s += 3
	}
	if o.FactoryInspection {
		s += 3
	}
	if hasServiceTag(o.ServiceTags, "深度验厂") {
		s += 2
	}
	if o.BusinessInspection {
		s += 1
	}
	s += o.RepurchasePct / 10.0 // reorder rate, capped softly by its own range
	s += o.TradeComposite / 2.0 // composite trade score is 0-5 -> 0-2.5

	label := "trader"
	switch {
	case o.SuperFactory || o.FactoryInspection || hasServiceTag(o.ServiceTags, "深度验厂"):
		label = "verified-factory"
	case o.BusinessInspection || o.RepurchasePct >= 20:
		label = "likely-factory"
	}
	return s, label
}

func newNovelFactoryFindCmd(flags *rootFlags) *cobra.Command {
	var top int
	var minTrade float64
	var dbPath string

	cmd := &cobra.Command{
		Use:         "factory-find <keyword>",
		Short:       "Rank wholesale offers by how likely the seller is the real factory, not a reseller",
		Long:        "Rank stored offers for a keyword by a factory-confidence score (verification flags + 深度验厂 tag + reorder rate + supplier trade score) and label each trader / likely-factory / verified-factory. Reads the local store; run 'sync <keyword>' first.",
		Example:     "  1688-pp-cli factory-find 手机壳 --top 10",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would rank stored offers by factory confidence")
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
			ranked := make([]scoredOffer, 0, len(offers))
			for _, o := range offers {
				if minTrade > 0 && o.TradeComposite < minTrade {
					continue
				}
				score, label := factoryScore(o)
				ranked = append(ranked, scoredOffer{Offer: o, FactoryConfidence: score, FactoryLabel: label})
			}
			sort.SliceStable(ranked, func(i, j int) bool {
				return ranked[i].FactoryConfidence > ranked[j].FactoryConfidence
			})
			if top > 0 && len(ranked) > top {
				ranked = ranked[:top]
			}
			if len(ranked) == 0 {
				return emit(cmd, flags, map[string]any{
					"keyword": args[0],
					"results": []scoredOffer{},
					"note":    fmt.Sprintf("no stored offers for %q; run: 1688-pp-cli sync %s", args[0], args[0]),
				})
			}
			return emit(cmd, flags, ranked)
		},
	}
	cmd.Flags().IntVar(&top, "top", 0, "Return only the top N offers by factory confidence (0 = all)")
	cmd.Flags().Float64Var(&minTrade, "min-trade", 0, "Minimum supplier composite trade score (0-5)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store database path")
	return cmd
}
