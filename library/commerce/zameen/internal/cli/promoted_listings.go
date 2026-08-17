// Hand-authored (was a generated html-endpoint scaffold): the low-level
// `listings <category> <location> <page>` command fetches one raw Zameen
// search page and returns its 25 listings. For filtered multi-page search use
// `find`; this command is the single-page primitive.
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

func newListingsPromotedCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "listings <category> <location> <page>",
		Short: "Fetch one raw page of Zameen search results (25 listings)",
		Long: "Fetch a single Zameen search page and return its listings. Category is one of " +
			"Homes, Rentals, Plots, Commercial; location is a Zameen slug with its id " +
			"(e.g. Islamabad-3, Lahore_DHA_Defence-9); page is 1-based.\n\n" +
			"For filtered, multi-page search prefer 'find'.",
		Example: strings.Trim(`
  zameen-pp-cli listings Homes Islamabad-3 1
  zameen-pp-cli listings Rentals Lahore-1 2 --json`, "\n"),
		Annotations: map[string]string{"pp:endpoint": "listings.search", "pp:method": "GET", "pp:path": "/{category}/{location}-{page}.html", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				path := "/{category}/{location}-{page}.html"
				fmt.Fprintf(cmd.OutOrStdout(), "would GET %s\n", path)
				return nil
			}
			if len(args) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("category and location are required, e.g. 'listings Homes Islamabad-3 1'"))
			}
			category := args[0]
			location := args[1]
			page := 1
			if len(args) >= 3 {
				if _, err := fmt.Sscanf(args[2], "%d", &page); err != nil || page < 1 {
					page = 1
				}
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c := zameen.NewClient(flags.timeout)
			listings, nbHits, nbPages, err := c.Page(ctx, category, location, page)
			if err != nil {
				var rl *cliutil.RateLimitError
				if errors.As(err, &rl) {
					return rateLimitErr(err)
				}
				return apiErr(err)
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "page %d of %d; %d total listings on Zameen for %s/%s\n", page, nbPages, nbHits, category, location)
			return emitListings(cmd, flags, listings)
		},
	}
	return cmd
}
