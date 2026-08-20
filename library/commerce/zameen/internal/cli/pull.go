// pull: mirror Zameen search results into the local SQLite store so offline
// search, comps/deals/aging/agencies, and watch diffing work.
// pp:data-source live
package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/zameen/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/zameen/internal/store"
	"github.com/mvanhorn/printing-press-library/library/commerce/zameen/internal/zameen"
)

func newPullCmd(flags *rootFlags) *cobra.Command {
	var sf searchFlags
	var maxPages int
	var dbPath string
	cmd := &cobra.Command{
		Use:   "pull",
		Short: "Mirror Zameen search results into the local store",
		Long: "Scan Zameen search pages for a city/purpose/type and upsert every listing into the " +
			"local SQLite store (deduplicated by listing id). Run this before comps, deals, aging, " +
			"agencies, and the offline search/sql commands.",
		Example: strings.Trim(`
  zameen-pp-cli pull --city Islamabad --purpose buy --type Homes --max-pages 5
  zameen-pp-cli pull --city Lahore --purpose buy --type Plots --area DHA_Defence --max-pages 10`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				path := "/{category}/{location}-{page}.html"
				fmt.Fprintf(cmd.OutOrStdout(), "would mirror Zameen listings from %s into the local store\n", path)
				return nil
			}
			// Reuse the filter machinery but scan many pages and keep all.
			sf.maxScanPages = maxPages
			sf.limit = maxPages * zameen.PageSize
			params, err := sf.toParams()
			if err != nil {
				_ = cmd.Usage()
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c := zameen.NewClient(flags.timeout)
			res, err := c.Search(ctx, params)
			if err != nil {
				var rl *cliutil.RateLimitError
				if errors.As(err, &rl) {
					return rateLimitErr(err)
				}
				return apiErr(err)
			}

			if dbPath == "" {
				dbPath = defaultDBPath("zameen-pp-cli")
			}
			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			// Store each listing under its Zameen external id. Upsert (not
			// UpsertBatch) so the id comes from external_id explicitly rather
			// than the generic ID-field heuristic, which does not recognize
			// external_id and would skip every row.
			stored := 0
			var lastErr error
			for _, l := range res.Listings {
				if l.ExternalId == "" {
					continue
				}
				raw, mErr := json.Marshal(l)
				if mErr != nil {
					continue
				}
				if err := db.UpsertListing(l.ExternalId, raw); err != nil {
					lastErr = err
					continue
				}
				stored++
			}
			if res.PartialError != "" {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: scan stopped early after a fetch error: %s (mirror is partial)\n", res.PartialError)
			}
			if stored == 0 && lastErr != nil {
				return fmt.Errorf("saving listings: %w", lastErr)
			}
			if err := db.SaveSyncState(listingsResource, "", stored); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not record sync state: %v\n", err)
			}

			summary := map[string]any{
				"pulled":        len(res.Listings),
				"stored":        stored,
				"scanned":       res.Scanned,
				"total_hits":    res.TotalHits,
				"db":            dbPath,
				"city":          sf.city,
				"purpose":       params.Purpose,
				"property_type": sf.propertyType,
			}
			fmt.Fprintf(cmd.ErrOrStderr(),
				"pulled %d listings, stored %d into %s\n",
				len(res.Listings), stored, dbPath)
			return emitObject(cmd, flags, summary)
		},
	}
	addSearchFlags(cmd, &sf, false)
	cmd.Flags().IntVar(&maxPages, "max-pages", 5, "Maximum pages to scan and store (25 listings/page)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: standard data dir)")
	return cmd
}
