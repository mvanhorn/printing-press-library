// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// pp:data-source live
package cli

import (
	"fmt"
	"github.com/spf13/cobra"
)

func newNovelBusServicesCmd(flags *rootFlags) *cobra.Command {
	var o busOptions
	var limit, offset int
	cmd := &cobra.Command{Use: "services", Short: "Check dated inventory with explicit overnight arrival dates", Args: cobra.NoArgs, Annotations: busAnnotations("--route=12200160001"), Example: "  japan-bus-online-pp-cli bus services --route 12200160001 --direction 0 --date 2026-10-10 --agent", RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "bus services")
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		if flags.dataSource == "local" {
			return usageErr(fmt.Errorf("this bus command requires live source data"))
		}
		c, e := busClient(cmd, o, flags)
		if e != nil {
			return e
		}
		if limit < 1 || limit > 100 || offset < 0 {
			return usageErr(fmt.Errorf("limit must be 1-100; offset must be nonnegative"))
		}
		out, sv, _, e := c.Services(ctx, o.route, o.direction, o.date)
		if e != nil {
			return classifyAPIError(cmd.OutOrStdout(), e, flags)
		}
		out["total_services"] = len(sv)
		start := min(offset, len(sv))
		end := min(start+limit, len(sv))
		out["services"] = sv[start:end]
		out["offset"] = offset
		out["limit"] = limit
		out["has_more"] = end < len(sv)
		return busEmit(cmd, flags, out)
	}}
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum dated service rows to return, 1-100")
	cmd.Flags().IntVar(&offset, "offset", 0, "Offset within source service rows for this day")
	busFlags(cmd, &o, true)
	return cmd
}
