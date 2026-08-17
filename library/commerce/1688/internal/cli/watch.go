// Hand-authored transcendence command. Not generator-emitted.
// pp:data-source live
package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/commerce/1688/internal/mtop"
	"github.com/mvanhorn/printing-press-library/library/commerce/1688/internal/store"

	"github.com/spf13/cobra"
)

type watchChange struct {
	OfferID        string  `json:"offer_id"`
	Title          string  `json:"title"`
	PricePrev      float64 `json:"price_prev"`
	PriceNow       float64 `json:"price_now"`
	RepurchasePrev float64 `json:"repurchase_prev"`
	RepurchaseNow  float64 `json:"repurchase_now"`
	TxPrev         int     `json:"tx_prev"`
	TxNow          int     `json:"tx_now"`
}

type watchNew struct {
	OfferID       string  `json:"offer_id"`
	Title         string  `json:"title"`
	PriceCNY      float64 `json:"price_cny"`
	RepurchasePct float64 `json:"repurchase_pct"`
	SupplierName  string  `json:"supplier_name"`
}

func newNovelWatchCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:         "watch <keyword>",
		Short:       "Re-run a saved search, store a fresh snapshot, and print only what changed since last run",
		Long:        "Re-sync a keyword live, persist a fresh snapshot, and report only the delta versus the last run: offers whose price/reorder-rate/transactions moved, plus newly appeared offers. Use 'drift' instead to read existing snapshot history without a fresh fetch.",
		Example:     "  1688-pp-cli watch 手机壳",
		Annotations: map[string]string{"pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would re-sync and report the delta")
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
			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			// Latest prior snapshot per offer (snapshots are oldest-first, so
			// the last write for each offer ID is its most recent).
			snaps, err := db.OfferSnapshots(ctx, args[0])
			if err != nil {
				return err
			}
			prior := map[string]store.OfferSnapshot{}
			for _, s := range snaps {
				prior[s.OfferID] = s
			}

			client := flags.newMtopClient()
			res, err := client.Search(ctx, mtop.SearchParams{Keyword: args[0], PageSize: limit})
			if err != nil {
				return classify1688Err(err, flags)
			}

			newOffers := []watchNew{}
			changed := []watchChange{}
			for _, o := range res.Offers {
				p, ok := prior[o.OfferID]
				if !ok {
					newOffers = append(newOffers, watchNew{
						OfferID: o.OfferID, Title: o.Title, PriceCNY: o.PriceCNY,
						RepurchasePct: o.RepurchasePct, SupplierName: o.SupplierName,
					})
					continue
				}
				if p.PriceCNY != o.PriceCNY || p.RepurchasePct != o.RepurchasePct || p.BookedCount != o.TransactionCount {
					changed = append(changed, watchChange{
						OfferID: o.OfferID, Title: o.Title,
						PricePrev: p.PriceCNY, PriceNow: o.PriceCNY,
						RepurchasePrev: p.RepurchasePct, RepurchaseNow: o.RepurchasePct,
						TxPrev: p.BookedCount, TxNow: o.TransactionCount,
					})
				}
			}

			if err := persistSearch(ctx, db, res); err != nil {
				return err
			}

			return emit(cmd, flags, map[string]any{
				"keyword":       args[0],
				"first_run":     len(prior) == 0,
				"new_count":     len(newOffers),
				"changed_count": len(changed),
				"new":           newOffers,
				"changed":       changed,
			})
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "Results per page to re-fetch (max 60)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store database path")
	return cmd
}
