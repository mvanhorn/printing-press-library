// Hand-authored transcendence command. Not generator-emitted.
// pp:data-source local
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

type supplierReport struct {
	MemberID          string   `json:"member_id"`
	Name              string   `json:"name"`
	Province          string   `json:"province,omitempty"`
	City              string   `json:"city,omitempty"`
	ShopURL           string   `json:"shop_url,omitempty"`
	OfferCount        int      `json:"offer_count"`
	TotalTransactions int      `json:"total_transactions"`
	AvgRepurchasePct  float64  `json:"avg_repurchase_pct"`
	VerifiedFactory   bool     `json:"verified_factory"`
	ServiceTags       []string `json:"service_tags,omitempty"`
	TradeComposite    float64  `json:"trade_score_composite"`
	TradeLogistics    float64  `json:"trade_score_logistics"`
	TradeDispute      float64  `json:"trade_score_dispute"`
	TradeConsultation float64  `json:"trade_score_consultation"`
	PriceMin          float64  `json:"price_min"`
	PriceMax          float64  `json:"price_max"`
}

func newNovelSupplierReportCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:         "supplier-report <member-id>",
		Short:       "Aggregate one shop across all its stored offers: trade scores, average reorder rate, total transactions",
		Long:        "Roll up a supplier's full footprint across every offer of theirs in the local store: trade-service scores, average reorder rate, total transactions, verification badges, offer count, and price range. Operates on a supplier member ID. For a single product use 'offers <offer-id>'.",
		Example:     "  1688-pp-cli supplier-report b2b-2850655109d72ea",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would aggregate a supplier's stored offers")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a supplier member ID is required"))
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

			raws, qerr := db.OffersBySupplier(ctx, args[0])
			if qerr != nil {
				return qerr
			}
			offers := decodeStoredOffers(raws)
			if len(offers) == 0 {
				return emit(cmd, flags, map[string]any{
					"member_id": args[0],
					"note":      "no stored offers for this supplier; sync some searches that surface them first",
				})
			}

			rep := supplierReport{MemberID: args[0]}
			tagSet := map[string]bool{}
			var repurchaseSum float64
			rep.PriceMin = offers[0].PriceCNY
			rep.PriceMax = offers[0].PriceCNY
			for _, o := range offers {
				rep.OfferCount++
				rep.TotalTransactions += o.TransactionCount
				repurchaseSum += o.RepurchasePct
				if o.VerifiedFactory {
					rep.VerifiedFactory = true
				}
				for _, t := range o.ServiceTags {
					tagSet[t] = true
				}
				if o.SupplierName != "" && rep.Name == "" {
					rep.Name = o.SupplierName
				}
				if rep.Province == "" {
					rep.Province = o.Province
				}
				if rep.City == "" {
					rep.City = o.City
				}
				if rep.ShopURL == "" {
					rep.ShopURL = o.ShopURL
				}
				if o.TradeComposite > rep.TradeComposite {
					rep.TradeComposite = o.TradeComposite
				}
				if o.TradeLogistics > rep.TradeLogistics {
					rep.TradeLogistics = o.TradeLogistics
				}
				if o.TradeDispute > rep.TradeDispute {
					rep.TradeDispute = o.TradeDispute
				}
				if o.TradeConsultation > rep.TradeConsultation {
					rep.TradeConsultation = o.TradeConsultation
				}
				if o.PriceCNY > 0 && (rep.PriceMin == 0 || o.PriceCNY < rep.PriceMin) {
					rep.PriceMin = o.PriceCNY
				}
				if o.PriceCNY > rep.PriceMax {
					rep.PriceMax = o.PriceCNY
				}
			}
			if rep.OfferCount > 0 {
				rep.AvgRepurchasePct = repurchaseSum / float64(rep.OfferCount)
			}
			for t := range tagSet {
				rep.ServiceTags = append(rep.ServiceTags, t)
			}
			return emit(cmd, flags, rep)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store database path")
	return cmd
}
