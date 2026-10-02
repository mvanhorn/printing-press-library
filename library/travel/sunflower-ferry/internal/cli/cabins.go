// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// pp:data-source live
package cli

import (
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/sunflower-ferry/internal/ferry"
	"github.com/spf13/cobra"
	"strings"
)

func newNovelCabinsCmd(flags *rootFlags) *cobra.Command {
	var route, direction, query string
	var limit int
	cmd := &cobra.Command{Use: "cabins", Short: "Inspect source cabin type and occupancy, preserving shared-room versus private-room facts", Example: "  sunflower-ferry-pp-cli cabins --route osaka-shibushi --agent", Annotations: ferryAnnotations("live", "--route=osaka-shibushi"), RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "cabins")
		}
		if e := validateDataSourceStrategy(flags, "live"); e != nil {
			return usageErr(e)
		}
		if e := ferryNoArgs(args); e != nil {
			return e
		}
		r, _, e := ferryResolve(route, direction)
		if e != nil {
			return e
		}
		if limit < 1 || limit > 40 || len([]rune(query)) > 100 {
			return usageErr(fmt.Errorf("--limit must be 1..40 and query at most 100 characters"))
		}
		ctx, cancel := ferryCtx(cmd, flags)
		defer cancel()
		c := ferry.New(flags.rateLimit)
		data, e := c.Cabins(ctx, r)
		if e != nil {
			return classifyAPIError(cmd.OutOrStdout(), e, flags)
		}
		out := []ferry.Cabin{}
		matched := 0
		for _, d := range data {
			if strings.Contains(strings.ToLower(d.Name), strings.ToLower(query)) {
				matched++
				if len(out) < limit {
					out = append(out, d)
				}
			}
		}
		return ferryWrite(cmd, flags, map[string]any{"route_id": r.ID, "cabins": out, "scanned_cabins": len(data), "matched_cabins": matched, "truncated": matched > len(out)}, c.Meta("en"), "Published occupancy does not prove dated eligibility or availability. Dormitory section/shared-room capacity retains source wording; use quote for the entered party.")
	}}
	cmd.Flags().StringVar(&route, "route", "osaka-beppu", "Official route slug or source direction code (see routes list)")
	cmd.Flags().StringVar(&direction, "direction", "outbound", "Route direction: outbound or inbound; source line IDs already fix direction")
	cmd.Flags().StringVar(&query, "query", "", "Filter source cabin names by case-insensitive text")
	cmd.Flags().IntVar(&limit, "limit", 40, "Maximum cabin categories returned, between 1 and 40")
	return cmd
}
