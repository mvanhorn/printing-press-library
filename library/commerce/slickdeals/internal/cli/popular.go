// Copyright 2026 david. Licensed under Apache-2.0. See LICENSE.

// popular.go implements `slickdeals-pp-cli popular`. Fetches Slickdeals'
// "Popular Deals" RSS feed (mode=popdeals) — community-voted deals that are
// distinct from the editor-curated frontpage. The feed is advertised on
// Slickdeals' own /forums/forumdisplay.php?f=9 HTML as a canonical RSS feed.

package cli

import (
	"encoding/json"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/slickdeals/internal/rss"

	"github.com/spf13/cobra"
)

func newPopularCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var minThumbs int

	cmd := &cobra.Command{
		Use:         "popular",
		Short:       "List Slickdeals Popular Deals (community-voted, separate from frontpage)",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Fetch the live "Popular Deals" RSS feed (mode=popdeals). This feed is
community-voted and distinct from the editor-curated frontpage — items here
are ranked by user thumbs across all Slickdeals forums, so it surfaces
trending deals that may not yet have hit the frontpage.

Pairs with 'hot' (frontpage filtered by thumbs) and 'frontpage-fresh' (raw
frontpage). Use --min-thumbs to drop low-vote items client-side.`,
		Example: strings.Trim(`
  # Top 10 popular deals as JSON
  slickdeals-pp-cli popular --limit 10 --json

  # Only items with >= 50 thumbs
  slickdeals-pp-cli popular --min-thumbs 50 --json --select title,link,thumbs
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			// Pass limit through to the fetch so we don't over-fetch when
			// Slickdeals expands the feed beyond 25. When --min-thumbs is
			// set we DO need to fetch unlimited because the thumb filter
			// runs client-side and any items dropped by the filter would
			// otherwise reduce the final count below the user's --limit.
			fetchLimit := limit
			if minThumbs > 0 {
				fetchLimit = 0
			}
			items, err := rss.LivePopular(cmd.Context(), nil, fetchLimit)
			if err != nil {
				return apiErr(err)
			}
			if minThumbs > 0 {
				kept := items[:0]
				for _, it := range items {
					if it.Thumbs >= minThumbs {
						kept = append(kept, it)
					}
				}
				items = kept
				if limit > 0 && len(items) > limit {
					items = items[:limit]
				}
			}
			data, err := json.Marshal(items)
			if err != nil {
				return err
			}
			prov := DataProvenance{Source: "live", ResourceType: "popular"}
			wrapped, err := wrapWithProvenance(data, prov)
			if err != nil {
				return err
			}
			printProvenance(cmd, len(items), prov)
			return printJSONFiltered(cmd.OutOrStdout(), json.RawMessage(wrapped), flags)
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum deals to return")
	cmd.Flags().IntVar(&minThumbs, "min-thumbs", 0, "Drop items below this thumb threshold (client-side)")

	return cmd
}
