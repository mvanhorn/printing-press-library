// Copyright 2026 david. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/slickdeals/internal/rss"

	"github.com/spf13/cobra"
)

// newHotCmd surfaces the hottest live Slickdeals frontpage deals by thumb
// count. v0.1's `frontpage hot` proxy at forum=9&hotdeals=1&rss=1 returns an
// empty channel — the handoff doc captures that lesson — so we always fetch
// the frontpage RSS and filter client-side.
func newHotCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var minThumbs int

	cmd := &cobra.Command{
		Use:   "hot",
		Short: "Top Slickdeals frontpage deals by thumb count (live RSS)",
		Long: `Pulls the live Slickdeals frontpage RSS feed and surfaces only deals
whose community thumb score meets --min-thumbs. Results are sorted by
thumbs descending and truncated to --limit.

This is the v0.2 replacement for v0.1's broken 'frontpage hot' path
(the forum=9&hotdeals=1 RSS lever returns an empty feed). Filtering
happens client-side against the same frontpage feed 'frontpage-fresh'
uses.`,
		Example: strings.Trim(`
  slickdeals-pp-cli hot --json --limit 10
  slickdeals-pp-cli hot --min-thumbs 50 --json --select title,link,thumbs
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			items, err := rss.LiveHot(cmd.Context(), nil, minThumbs, limit)
			if err != nil {
				return apiErr(fmt.Errorf("fetching frontpage RSS: %w", err))
			}

			prov := DataProvenance{
				Source:       "live",
				Reason:       "user_requested",
				ResourceType: "hot",
			}

			// Mirror the generator-emitted pattern: wrap-and-marshal for JSON
			// output, fall through to the standard pipeline otherwise. The
			// command stays cohesive with web-api_list_missed_deals.go.
			raw, err := json.Marshal(items)
			if err != nil {
				return err
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				filtered := raw
				if flags.selectFields != "" {
					filtered = filterFields(filtered, flags.selectFields)
				} else if flags.compact {
					filtered = compactFields(filtered)
				}
				wrapped, werr := wrapWithProvenance(filtered, prov)
				if werr != nil {
					return werr
				}
				return printOutput(cmd.OutOrStdout(), wrapped, true)
			}
			printProvenance(cmd, len(items), prov)
			return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum results to return")
	cmd.Flags().IntVar(&minThumbs, "min-thumbs", 20, "Minimum community thumb score to include")
	return cmd
}
