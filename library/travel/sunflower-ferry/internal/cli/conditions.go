// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// pp:data-source live
package cli

import (
	"github.com/mvanhorn/printing-press-library/library/travel/sunflower-ferry/internal/ferry"
	"github.com/spf13/cobra"
)

func newNovelConditionsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "conditions", Short: "Read common baggage, passenger, vehicle, change and normal-ticket cancellation conditions", Example: "  sunflower-ferry-pp-cli conditions --agent", Annotations: ferryAnnotations("live", ""), RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "conditions")
		}
		if e := validateDataSourceStrategy(flags, "live"); e != nil {
			return usageErr(e)
		}
		if e := ferryNoArgs(args); e != nil {
			return e
		}
		ctx, cancel := ferryCtx(cmd, flags)
		defer cancel()
		c := ferry.New(flags.rateLimit)
		data, e := c.Conditions(ctx)
		if e != nil {
			return classifyAPIError(cmd.OutOrStdout(), e, flags)
		}
		return ferryWrite(cmd, flags, data, c.Meta("en/ja; operator-wide passenger carriage conditions"), "Normal passenger-ticket conditions; ticket-specific campaigns, agency terms and vehicle refunds can differ. No cancellation or refund is performed.")
	}}
	return cmd
}
