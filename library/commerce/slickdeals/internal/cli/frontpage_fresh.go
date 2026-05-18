// Copyright 2026 david. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/slickdeals/internal/rss"

	"github.com/spf13/cobra"
)

// newFrontpageFreshCmd hits the live Slickdeals frontpage RSS feed and
// returns the FRESH drops (~25 most recent items). This differs from v0.1's
// `frontpage list-json`, which reads the Nuxt carousel — a server-rendered
// payload that can lag behind the actual feed by minutes. RSS is the
// authoritative "what hit the frontpage right now" surface.
func newFrontpageFreshCmd(flags *rootFlags) *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "frontpage-fresh",
		Short: "Fresh Slickdeals frontpage RSS feed (live, unfiltered)",
		Long: `Pulls today's drops from the live Slickdeals frontpage RSS feed.

Unlike 'frontpage list-json' (v0.1), which reads the Nuxt-cached
carousel and may lag behind by minutes, this command consumes the
canonical RSS feed at /newsearch.php?mode=frontpage&rss=1. Items are
in feed order (newest first) and truncated to --limit.`,
		Example: strings.Trim(`
  slickdeals-pp-cli frontpage-fresh --json --limit 10
  slickdeals-pp-cli frontpage-fresh --limit 5 --json --select title,link,pub_date,thumbs
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			items, err := rss.LiveFrontpage(cmd.Context(), nil)
			if err != nil {
				return apiErr(fmt.Errorf("fetching frontpage RSS: %w", err))
			}
			if limit > 0 && len(items) > limit {
				items = items[:limit]
			}

			prov := DataProvenance{
				Source:       "live",
				Reason:       "user_requested",
				ResourceType: "frontpage-fresh",
			}

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
	return cmd
}
