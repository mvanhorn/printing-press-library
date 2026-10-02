// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package cli

import (
	"fmt"
	"github.com/spf13/cobra"
)

// pp:data-source live
func newNovelSameDayCmd(f *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "same-day"}
	var query string
	var limit int
	cmd = luggageCmd(cmd, "Review separate same-day airport route cutoff evidence and live example fees.", "--query Narita --agent", "live", f)
	cmd.Long = "Return selected Narita/Haneda airport-to-hotel service areas and cutoffs from Yamato's versioned same-day PDF only while live bytes match the source fingerprint. Other directions/counters remain explicit full-PDF/directory handoffs. Changed source suppresses the snapshot. Published example fees are separately labeled and are not quotes for every listed route. This command does not check actual hotel or parcel acceptance."
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if dryRunOK(f) {
			return writeDryRun(cmd.OutOrStdout(), f, "same-day")
		}
		if e := validateDataSourceStrategy(f, cmd.Annotations["pp:data-source"]); e != nil {
			return usageErr(e)
		}
		if cmd.Annotations["pp:data-source"] == "computed" && f.dataSource == "live" {
			return usageErr(fmt.Errorf("parcel uses verified local rules; choose --data-source auto or local"))
		}
		if e := noLuggageArgs(args); e != nil {
			return e
		}
		if e := luggageLimit(limit, 0); e != nil {
			return e
		}
		ctx, cancel := boundCtx(cmd.Context(), f)
		defer cancel()
		c := luggageClient(f)
		d, e := c.SameDay(ctx, query, limit)
		if e != nil {
			return luggageError(e)
		}
		return luggageOutput(cmd, f, c, d)
	}
	cmd.Flags().StringVar(&query, "query", "", "Literal selected Narita/Haneda counter name, ID or destination area")
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum verified schedule records returned; bounded1..50, default10")
	return cmd
}
