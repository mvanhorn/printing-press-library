// Copyright 2026 Mayank Lavania and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/passive-indices/internal/indiapassivefunds"
)

func newFundSearchCmd(flags *rootFlags) *cobra.Command {
	var pageSize int
	var instrumentType string

	cmd := &cobra.Command{
		Use:     "search <query>",
		Short:   "Search funds/schemes by name fragment",
		Example: "  passive-indices-pp-cli fund search \"nifty 50\" --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would search funds")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("search query is required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c := newIndiaPassiveFundsClient(flags)
			env, err := c.SymbolLookup(ctx, indiapassivefunds.SymbolLookupParams{
				SearchTerm:     args[0],
				PageSize:       pageSize,
				InstrumentType: instrumentType,
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			rows := decodeEnvelopeRows(env)
			return flags.printJSON(cmd, rows)
		},
	}
	cmd.Flags().IntVar(&pageSize, "limit", 20, "maximum results to return")
	cmd.Flags().StringVar(&instrumentType, "instrument-type", "all", "filter by instrument type (all, etf, fund)")
	return cmd
}

// decodeEnvelopeRows flattens every row in a ListEnvelope from field codes
// to displayName-keyed maps.
func decodeEnvelopeRows(env *indiapassivefunds.ListEnvelope) []map[string]any {
	rows := make([]map[string]any, 0, len(env.Data))
	for _, row := range env.Data {
		rows = append(rows, env.Decode(row))
	}
	return rows
}
