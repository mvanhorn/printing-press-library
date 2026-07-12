// Copyright 2026 Mayank Lavania and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// pp:client-call — calls the hand-written sibling client (internal/niftyindices or internal/indiapassivefunds) via a package-local newXClient() helper, not the generated internal/client package.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newNovelIndexFundsCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "funds <index>",
		Short:       "See every ETF and index fund that tracks a given NSE index in one call.",
		Long:        "Use for \"what tracks index X\" lookups. For cost/fidelity comparison of those funds, use 'index tracking' instead; for a single fund-vs-single-index side-by-side, use 'compare'.",
		Example:     "  passive-indices-pp-cli index funds \"NIFTY 50\" --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would join index to tracking funds")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("index name is required, e.g. \"NIFTY 50\""))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c := newIndiaPassiveFundsClient(flags)
			rows, matched, err := resolveIndexTrackers(ctx, c, args[0])
			if err != nil {
				return classifyAPIError(err, flags)
			}
			out := map[string]any{
				"index":          args[0],
				"matched_index":  matched,
				"tracking_funds": rows,
				"count":          len(rows),
			}
			return flags.printJSON(cmd, out)
		},
	}
	return cmd
}
