// Copyright 2026 Mayank Lavania and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newIndexTRICmd(flags *rootFlags) *cobra.Command {
	var from, to string

	cmd := &cobra.Command{
		Use:     "tri <name>",
		Short:   "Total Return Index (TRI) historical series for a date range",
		Example: "  passive-indices-pp-cli index tri \"NIFTY 50\" --from 01-Jun-2026 --to 01-Jul-2026 --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch TRI series")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("index name is required"))
			}
			fromT, toT, err := parseHistoryRange(from, to)
			if err != nil {
				return usageErr(err)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c := newNiftyIndicesClient(flags)
			rows, err := c.TRI(ctx, args[0], fromT, toT)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return flags.printJSON(cmd, rows)
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "start date (DD-Mon-YYYY), defaults to 90 days before --to")
	cmd.Flags().StringVar(&to, "to", "", "end date (DD-Mon-YYYY), defaults to today")
	return cmd
}
