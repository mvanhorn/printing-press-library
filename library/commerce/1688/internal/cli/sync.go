// Hand-authored 1688 sync. Replaces the generic spec-driven sync template,
// which does not fit the signed-mtop search surface. Not generator-emitted.
// pp:data-source live
package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/commerce/1688/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/1688/internal/mtop"
	"github.com/mvanhorn/printing-press-library/library/commerce/1688/internal/store"

	"github.com/spf13/cobra"
)

func newSyncCmd(flags *rootFlags) *cobra.Command {
	var maxPages, limit int
	var priceMin, priceMax, province, sort, dbPath, resources string
	var full bool

	cmd := &cobra.Command{
		Use:         "sync <keyword>",
		Short:       "Fetch 1688 offers for a keyword into the local store (builds drift history)",
		Long:        "Page through live search results for a keyword and persist offers, suppliers, and price/reorder snapshots into the local store. Run repeatedly over time to build the history that drift, watch, and the ranking commands read.",
		Example:     "  1688-pp-cli sync 手机壳 --max-pages 3",
		Annotations: map[string]string{"pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would sync 1688 offers into the local store")
				return nil
			}
			if len(args) == 0 {
				// 1688 sync is keyword-driven (there is no resource list to
				// walk). A flag-only invocation (e.g. framework-style
				// `sync --full`) is a clean no-op, not a hard error.
				fmt.Fprintln(cmd.ErrOrStderr(), "nothing to sync: provide a keyword, e.g. 1688-pp-cli sync 手机壳")
				return emit(cmd, flags, map[string]any{
					"keyword":       "",
					"pages_fetched": 0,
					"offers_synced": 0,
					"note":          "provide a keyword to sync, e.g. 1688-pp-cli sync 手机壳",
				})
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

			if full && maxPages < 50 {
				maxPages = 50
			}
			// Curtail under live-dogfood so the matrix's per-command timeout holds.
			if cliutil.IsDogfoodEnv() && maxPages > 1 {
				maxPages = 1
			}

			client := flags.newMtopClient()
			keyword := args[0]
			offersSynced := 0
			pagesFetched := 0
			for page := 1; page <= maxPages; page++ {
				res, err := client.Search(ctx, mtop.SearchParams{
					Keyword:  keyword,
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
				if err := persistSearch(ctx, db, res); err != nil {
					return err
				}
				pagesFetched++
				offersSynced += len(res.Offers)
				if !res.HasMore || len(res.Offers) == 0 {
					break
				}
			}

			// Record sync state so doctor/freshness and subsequent runs can
			// report when offers were last synced and for which query.
			_ = db.SaveSyncState("offer", keyword, offersSynced)

			return emit(cmd, flags, map[string]any{
				"keyword":       keyword,
				"pages_fetched": pagesFetched,
				"offers_synced": offersSynced,
				"db":            dbPath,
			})
		},
	}
	cmd.Flags().IntVar(&maxPages, "max-pages", 3, "Maximum result pages to fetch")
	cmd.Flags().IntVar(&limit, "limit", 20, "Results per page (max 60)")
	cmd.Flags().BoolVar(&full, "full", false, "Fetch all available pages (up to the page-size cap)")
	cmd.Flags().StringVar(&resources, "resources", "", "Accepted for framework compatibility; 1688 sync is keyword-driven")
	cmd.Flags().StringVar(&priceMin, "price-min", "", "Minimum wholesale price in CNY")
	cmd.Flags().StringVar(&priceMax, "price-max", "", "Maximum wholesale price in CNY")
	cmd.Flags().StringVar(&province, "province", "", "Supplier province in Chinese characters (e.g. 广东)")
	cmd.Flags().StringVar(&sort, "sort", "", "Sort order: price-asc | price-desc | booked | newest")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store database path")
	return cmd
}
