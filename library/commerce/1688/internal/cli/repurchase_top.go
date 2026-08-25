// Hand-authored transcendence command. Not generator-emitted.
// pp:data-source local
package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

func newNovelRepurchaseTopCmd(flags *rootFlags) *cobra.Command {
	var minTx, top int
	var dbPath string

	cmd := &cobra.Command{
		Use:         "repurchase-top <keyword>",
		Short:       "Rank synced offers and suppliers by 回头率 (buyer reorder rate)",
		Long:        "Rank stored offers for a keyword by reorder rate (回头率), with a minimum-transaction floor so a 100% rate over 2 orders does not outrank a 40% rate over 5,000. Reads the local store; run 'sync <keyword>' first.",
		Example:     "  1688-pp-cli repurchase-top 手机壳 --min-tx 100",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would rank stored offers by reorder rate")
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
			filtered := offers[:0]
			for _, o := range offers {
				if o.TransactionCount >= minTx {
					filtered = append(filtered, o)
				}
			}
			sort.SliceStable(filtered, func(i, j int) bool {
				if filtered[i].RepurchasePct != filtered[j].RepurchasePct {
					return filtered[i].RepurchasePct > filtered[j].RepurchasePct
				}
				return filtered[i].TransactionCount > filtered[j].TransactionCount
			})
			if top > 0 && len(filtered) > top {
				filtered = filtered[:top]
			}
			if len(filtered) == 0 {
				return emit(cmd, flags, map[string]any{
					"keyword": args[0],
					"results": []any{},
					"note":    fmt.Sprintf("no stored offers for %q at min-tx %d; run: 1688-pp-cli sync %s", args[0], minTx, args[0]),
				})
			}
			return emit(cmd, flags, filtered)
		},
	}
	cmd.Flags().IntVar(&minTx, "min-tx", 0, "Minimum 30-day transaction count to qualify")
	cmd.Flags().IntVar(&top, "top", 0, "Return only the top N offers (0 = all)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store database path")
	return cmd
}
