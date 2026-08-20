// find: live parametric Zameen search with client-side filters and sort.
// pp:data-source live
package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/zameen/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/zameen/internal/zameen"
)

func newFindCmd(flags *rootFlags) *cobra.Command {
	var sf searchFlags
	cmd := &cobra.Command{
		Use:   "find",
		Short: "Search Zameen listings live with real filters",
		Long: "Search Zameen.com property listings live with real filters (price, beds, baths, " +
			"area in Marla, city/area, purpose) and client-side sort.\n\n" +
			"Zameen encodes filters in the URL path, not query strings, so this command scans " +
			"search pages and filters locally. Use --max-scan-pages to widen the search and " +
			"--limit to cap results.",
		Example: strings.Trim(`
  zameen-pp-cli find --city Islamabad --purpose buy --type Homes --min-beds 3 --max-price 30000000
  zameen-pp-cli find --city Lahore --purpose rent --type Homes --max-scan-pages 10 --csv
  zameen-pp-cli find --location Lahore_DHA_Defence-9 --type Plots --agent --select external_id,title,price,area_marla`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				path := "/{category}/{location}-{page}.html"
				fmt.Fprintf(cmd.OutOrStdout(), "would search Zameen listings via %s\n", path)
				return nil
			}
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

			fmt.Fprintf(cmd.ErrOrStderr(),
				"scanned %d listings across %d page(s); %d total on Zameen; %d matched\n",
				res.Scanned, res.ScanPages, res.TotalHits, len(res.Listings))
			if res.PartialError != "" {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: scan stopped early after a fetch error: %s (results are partial)\n", res.PartialError)
			}
			if len(res.Listings) == 0 && res.ScanCapHit {
				fmt.Fprintf(cmd.ErrOrStderr(),
					"no matches within the scan cap; raise --max-scan-pages to widen the search\n")
			}
			return emitListings(cmd, flags, res.Listings)
		},
	}
	addSearchFlags(cmd, &sf, true)
	return cmd
}
