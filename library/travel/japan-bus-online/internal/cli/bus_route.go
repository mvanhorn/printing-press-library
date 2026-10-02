// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// pp:data-source live
package cli

import (
	"fmt"
	"github.com/spf13/cobra"
)

func newNovelBusRouteCmd(flags *rootFlags) *cobra.Command {
	var o busOptions
	cmd := &cobra.Command{Use: "route", Short: "Read course/direction published timetables, advertised JPY fares, mapped stops and overnight offsets", Args: cobra.NoArgs, Annotations: busAnnotations("--route=12200160001"), Example: "  japan-bus-online-pp-cli bus route --route 12200160001 --agent", RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "bus route")
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
		out, ds, e := c.Route(ctx, o.route)
		if e != nil {
			return classifyAPIError(cmd.OutOrStdout(), e, flags)
		}
		if cmd.Flags().Changed("direction") {
			selected := ds[:0]
			for _, d := range ds {
				if d.Direction == o.direction {
					selected = append(selected, d)
				}
			}
			out["directions"] = selected
		}
		return busEmit(cmd, flags, out)
	}}
	busFlags(cmd, &o, false)
	return cmd
}
