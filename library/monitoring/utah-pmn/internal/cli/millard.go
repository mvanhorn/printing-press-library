// Copyright 2026 Paul Gradeff and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: Millard County land-use sweep.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

// pp:data-source live
func newNovelMillardCmd(flags *rootFlags) *cobra.Command {
	var flagDays int
	var flagLimit int
	var flagAll bool

	cmd := &cobra.Command{
		Use:   "millard",
		Short: "Sweep every Millard County town and keep only land-use approval bodies",
		Long: "Sweep every Millard County town in one command, dedup by notice, and keep only " +
			"land-use approval bodies (planning commissions, councils, the county commission, boards). " +
			"Use --all to keep every body, --days to set the window.",
		Example:     "  utah-pmn-pp-cli millard --days 30 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would sweep Millard County towns for land-use meetings")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			start, end := dateWindow(flagDays)
			notices, err := sweepLocations(ctx, c, millardCityNames(), start, end, flagLimit)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			out := make([]pmnNotice, 0, len(notices))
			for _, n := range notices {
				if flagAll || bodyLooksLandUse(n) {
					out = append(out, n)
				}
			}
			b, err := json.Marshal(out)
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), b, flags)
		},
	}
	cmd.Flags().IntVar(&flagDays, "days", 30, "Days ahead to scan from today")
	cmd.Flags().IntVar(&flagLimit, "limit", 50, "Max notices per town before dedup")
	cmd.Flags().BoolVar(&flagAll, "all", false, "Keep every body, not just land-use approval bodies")
	return cmd
}
