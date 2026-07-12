// Copyright 2026 Mayank Lavania and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// pp:client-call — calls the hand-written sibling client (internal/niftyindices or internal/indiapassivefunds) via a package-local newXClient() helper, not the generated internal/client package.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/passive-indices/internal/indiapassivefunds"
)

func newNovelFundNfoTrackingCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:     "tracking <index>",
		Short:   "See upcoming New Fund Offers that mention a specific index by name.",
		Long:    "indiapassivefunds' NFO listing does not expose a clean underlying-index field, so this matches the requested index name as a substring of each NFO's fund name (the naming convention nearly every Indian index fund/ETF follows). Confirm the exact benchmark on the resulting fund's detail page before relying on this for decisions.",
		Example: "  passive-indices-pp-cli fund nfo tracking \"NIFTY Next 50\" --json",
		Annotations: map[string]string{
			"mcp:read-only":          "true",
			"pp:no-error-path-probe": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would filter NFOs by index name")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("index name is required, e.g. \"NIFTY Next 50\""))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c := newIndiaPassiveFundsClient(flags)
			env, err := c.NFO(ctx, indiapassivefunds.NFOParams{PageSize: 200})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			rows := decodeEnvelopeRows(env)

			target := strings.ToLower(args[0])
			matched := make([]map[string]any, 0)
			for _, row := range rows {
				name, _ := row["Name"].(string)
				if name == "" {
					continue
				}
				if strings.Contains(strings.ToLower(name), target) {
					matched = append(matched, row)
				}
			}

			return flags.printJSON(cmd, map[string]any{
				"index":         args[0],
				"match_method":  "fund-name substring match (NFO listing has no underlying-index field)",
				"scanned_nfos":  len(rows),
				"matching_nfos": matched,
			})
		},
	}
	return cmd
}
