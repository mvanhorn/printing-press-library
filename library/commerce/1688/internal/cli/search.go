// Hand-authored live offer search. Not generator-emitted.
// pp:data-source live
package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/commerce/1688/internal/mtop"
	"github.com/mvanhorn/printing-press-library/library/commerce/1688/internal/store"

	"github.com/spf13/cobra"
)

func newSearchCmd(flags *rootFlags) *cobra.Command {
	var priceMin, priceMax, province, sort, dbPath string
	var page, limit int
	var noStore bool

	cmd := &cobra.Command{
		Use:   "search <keyword>",
		Short: "Search 1688 wholesale offers by keyword (live, signed mtop request)",
		Long: "Search 1688.com's wholesale catalog live and return structured offers " +
			"(price, supplier, region, transaction count, reorder rate, factory flags). " +
			"Results persist to the local store by default so drift/factory-find/compare " +
			"can read them later. 1688's corpus is Mandarin: translate English terms to " +
			"Simplified Chinese (e.g. 'phone case' -> 手机壳) for rich results.",
		Example:     "  1688-pp-cli search 手机壳 --limit 10 --sort booked",
		Annotations: map[string]string{"pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would search 1688 offers")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("keyword is required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			client := flags.newMtopClient()
			res, err := client.Search(ctx, mtop.SearchParams{
				Keyword:  args[0],
				PriceMin: priceMin,
				PriceMax: priceMax,
				Province: province,
				Sort:     sort,
				Page:     page,
				PageSize: limit,
			})
			if err != nil {
				return classify1688Err(err, flags)
			}

			if !noStore {
				if dbPath == "" {
					dbPath = defaultDBPath("1688-pp-cli")
				}
				db, derr := store.OpenWithContext(ctx, dbPath)
				if derr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: local store not updated: %v\n", derr)
				} else {
					if err := persistSearch(ctx, db, res); err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "warning: local store not updated: %v\n", err)
					}
					if err := db.Close(); err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "warning: local store close failed: %v\n", err)
					}
				}
			}
			return emit(cmd, flags, res)
		},
	}
	cmd.Flags().StringVar(&priceMin, "price-min", "", "Minimum wholesale price in CNY (e.g. 1.5)")
	cmd.Flags().StringVar(&priceMax, "price-max", "", "Maximum wholesale price in CNY (e.g. 20)")
	cmd.Flags().StringVar(&province, "province", "", "Supplier province in Chinese characters (e.g. 广东, 浙江)")
	cmd.Flags().StringVar(&sort, "sort", "", "Sort order: price-asc | price-desc | booked | newest")
	cmd.Flags().IntVar(&page, "page", 1, "Result page (1-based)")
	cmd.Flags().IntVar(&limit, "limit", 20, "Results per page (max 60)")
	cmd.Flags().BoolVar(&noStore, "no-store", false, "Do not persist results to the local store")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store database path")
	return cmd
}
