// Copyright 2026 Mayank Lavania and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newFundTimeseriesCmd(flags *rootFlags) *cobra.Command {
	var tenure string

	cmd := &cobra.Command{
		Use:     "timeseries <schemeId>",
		Short:   "Get a fund's NAV/AUM time series",
		Example: "  passive-indices-pp-cli fund timeseries 1150 --tenure 1y --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch time series")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("schemeId is required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c := newIndiaPassiveFundsClient(flags)
			raw, err := c.TimeSeries(ctx, args[0], tenure)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var out json.RawMessage = raw
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().StringVar(&tenure, "tenure", "1y", "time range (e.g. 1m, 6m, 1y, 3y, 5y)")
	return cmd
}
