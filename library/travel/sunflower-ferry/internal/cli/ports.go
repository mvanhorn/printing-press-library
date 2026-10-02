// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// pp:data-source live
package cli

import (
	"github.com/mvanhorn/printing-press-library/library/travel/sunflower-ferry/internal/ferry"
	"github.com/spf13/cobra"
)

func newNovelPortsCmd(flags *rootFlags) *cobra.Command {
	var route, direction string
	cmd := &cobra.Command{Use: "ports", Short: "Read distinct source terminal addresses, access guidance and boarding cautions", Example: "  sunflower-ferry-pp-cli ports --route osaka-beppu --agent", Annotations: ferryAnnotations("live", "--route=osaka-beppu"), RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "ports")
		}
		if e := validateDataSourceStrategy(flags, "live"); e != nil {
			return usageErr(e)
		}
		if e := ferryNoArgs(args); e != nil {
			return e
		}
		r, line, e := ferryResolve(route, direction)
		if e != nil {
			return e
		}
		ctx, cancel := ferryCtx(cmd, flags)
		defer cancel()
		c := ferry.New(flags.rateLimit)
		data, e := c.Ports(ctx, r)
		if e != nil {
			return classifyAPIError(cmd.OutOrStdout(), e, flags)
		}
		if line == r.InboundLine {
			data[0], data[1] = data[1], data[0]
		}
		return ferryWrite(cmd, flags, map[string]any{"route_id": r.ID, "line": line, "departure_terminal": data[0], "arrival_terminal": data[1], "checkin_guide_url": ferry.PublicBase + "/en/reservation/"}, c.Meta("en"), "Access times and ground-transport fares in the operator English guidance are undated reference information; confirm current ground transport. English boarding guidance recommends arrival at least 60 minutes before departure and warns boarding may be refused within 30 minutes; confirm vehicle/seasonal requirements.")
	}}
	cmd.Flags().StringVar(&route, "route", "osaka-beppu", "Official route slug or source direction code (see routes list)")
	cmd.Flags().StringVar(&direction, "direction", "outbound", "Route direction: outbound or inbound; source line IDs already fix direction")
	return cmd
}
