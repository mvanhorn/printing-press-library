// Copyright 2026 Micah Baldwin and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: keeper funnel conversion rates.
package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

// pp:data-source local
func newNovelFunnelCmd(flags *rootFlags) *cobra.Command {
	var flagBy string

	cmd := &cobra.Command{
		Use:   "funnel",
		Short: "Conversion rates from shot to picked to rated to developed to collected, optionally per year.",
		Long: "Chains quality signals into cull-stage conversion rates: shot → picked (flag) → rated 3+ → developed\n" +
			"(has adjustments) → in a collection. 'stats' gives single-facet histograms; this gives ratios.",
		Example: strings.Trim(`
  lightroom-classic-pp-cli funnel --json
  lightroom-classic-pp-cli funnel --by year`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "--by=year"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would compute the keeper funnel from the local catalog")
				return nil
			}
			if flagBy != "" && flagBy != "year" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--by only supports 'year'"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			cat, err := openCatalog(ctx, flags)
			if err != nil {
				return err
			}
			defer cat.Close()
			reports, err := cat.Funnel(ctx, flagBy == "year")
			if err != nil {
				return err
			}
			return emitLrcat(cmd, flags, reports, func(w io.Writer) {
				for _, r := range reports {
					if r.Year != "" {
						fmt.Fprintf(w, "%s:\n", r.Year)
					}
					for _, s := range r.Stages {
						fmt.Fprintf(w, "  %-14s %8d  %5.1f%%\n", s.Stage, s.Count, s.Percent)
					}
				}
			})
		},
	}
	cmd.Flags().StringVar(&flagBy, "by", "", "Break down per capture year: --by year")
	return cmd
}
