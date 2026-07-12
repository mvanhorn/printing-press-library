// Copyright 2026 Mayank Lavania and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

func newNovelIndexSectorsCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "sectors <index>",
		Short:       "See an index's constituents grouped by sector, with real per-constituent and per-sector weights.",
		Long:        "Reads niftyindices' live sector-weight feed (liveindexsa.niftyindices.com), which publishes actual weights per constituent and per sector — unlike the /IndexConstituent/ CSV (used by 'index constituents'), which carries no weight field at all. This feed also covers strategy indices (e.g. NIFTY ALPHA 50) that have no published constituent CSV.",
		Example:     "  passive-indices-pp-cli index sectors \"NIFTY 50\" --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch sector weights")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("index name is required, e.g. \"NIFTY 50\""))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c := newNiftyIndicesClient(flags)
			groups, err := c.SectorWeights(ctx, args[0])
			if err != nil {
				return classifyAPIError(err, flags)
			}

			sort.Slice(groups, func(i, j int) bool { return groups[i].Weight > groups[j].Weight })
			totalConstituents := 0
			for _, g := range groups {
				totalConstituents += len(g.Constituents)
			}

			return flags.printJSON(cmd, map[string]any{
				"index":              args[0],
				"total_constituents": totalConstituents,
				"sectors":            groups,
			})
		},
	}
	return cmd
}
