// Copyright 2026 Mayank Lavania and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newFundCompareListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "compare <schemeId1> <schemeId2> [schemeId3...]",
		Short:   "Compare two or more funds' NAV, AUM, expense ratio, and tracking metrics",
		Example: "  passive-indices-pp-cli fund compare 1150 1272 --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would compare funds")
				return nil
			}
			if len(args) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("at least two schemeIds are required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c := newIndiaPassiveFundsClient(flags)
			views := make([]*fundDetailView, 0, len(args))
			var fetchFailures []map[string]string
			for _, id := range args {
				fd, err := c.FundDetail(ctx, id)
				if err != nil {
					fetchFailures = append(fetchFailures, map[string]string{"scheme_id": id, "error": err.Error()})
					continue
				}
				views = append(views, fundDetailToView(fd))
			}
			if len(fetchFailures) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d funds failed to fetch\n", len(fetchFailures), len(args))
			}

			out := map[string]any{"funds": views}
			if len(fetchFailures) > 0 {
				out["fetch_failures"] = fetchFailures
			}
			return flags.printJSON(cmd, out)
		},
	}
	return cmd
}
