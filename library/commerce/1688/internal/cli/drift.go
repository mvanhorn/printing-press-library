// Hand-authored transcendence command. Not generator-emitted.
// pp:data-source local
package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/commerce/1688/internal/store"

	"github.com/spf13/cobra"
)

type offerDrift struct {
	OfferID           string  `json:"offer_id"`
	Snapshots         int     `json:"snapshots"`
	FirstSeen         string  `json:"first_seen"`
	LastSeen          string  `json:"last_seen"`
	PriceFirst        float64 `json:"price_first"`
	PriceLast         float64 `json:"price_last"`
	PriceChange       float64 `json:"price_change"`
	RepurchaseFirst   float64 `json:"repurchase_first"`
	RepurchaseLast    float64 `json:"repurchase_last"`
	RepurchaseChange  float64 `json:"repurchase_change"`
	TransactionFirst  int     `json:"transaction_first"`
	TransactionLast   int     `json:"transaction_last"`
	TransactionChange int     `json:"transaction_change"`
}

func newNovelDriftCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:         "drift <offer-id|keyword>",
		Short:       "Show how an offer's price, reorder rate, and 30-day transaction count moved across your stored snapshots.",
		Long:        "Diff the earliest stored snapshot against the latest for an offer ID or for every offer under a keyword. Reads the local store; sync the same target at least twice over time to accumulate snapshots.",
		Example:     "  1688-pp-cli drift 手机壳",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would diff stored snapshots")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("an offer ID or keyword is required"))
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

			snaps, err := db.OfferSnapshots(ctx, args[0])
			if err != nil {
				return err
			}
			// Snapshots arrive oldest-first; group by offer_id preserving order.
			order := []string{}
			byOffer := map[string][]store.OfferSnapshot{}
			for _, s := range snaps {
				if _, ok := byOffer[s.OfferID]; !ok {
					order = append(order, s.OfferID)
				}
				byOffer[s.OfferID] = append(byOffer[s.OfferID], s)
			}

			drifts := make([]offerDrift, 0, len(order))
			for _, id := range order {
				g := byOffer[id]
				if len(g) < 2 {
					continue
				}
				first, last := g[0], g[len(g)-1]
				drifts = append(drifts, offerDrift{
					OfferID:           id,
					Snapshots:         len(g),
					FirstSeen:         first.SyncedAt,
					LastSeen:          last.SyncedAt,
					PriceFirst:        first.PriceCNY,
					PriceLast:         last.PriceCNY,
					PriceChange:       last.PriceCNY - first.PriceCNY,
					RepurchaseFirst:   first.RepurchasePct,
					RepurchaseLast:    last.RepurchasePct,
					RepurchaseChange:  last.RepurchasePct - first.RepurchasePct,
					TransactionFirst:  first.BookedCount,
					TransactionLast:   last.BookedCount,
					TransactionChange: last.BookedCount - first.BookedCount,
				})
			}
			if len(drifts) == 0 {
				return emit(cmd, flags, map[string]any{
					"target":  args[0],
					"results": []offerDrift{},
					"note":    "need at least 2 snapshots per offer; sync the same target again later to build drift history",
				})
			}
			return emit(cmd, flags, drifts)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store database path")
	return cmd
}
