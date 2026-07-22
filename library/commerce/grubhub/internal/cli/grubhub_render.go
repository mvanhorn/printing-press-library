// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// wantsJSON reports whether output should be machine JSON rather than a human
// table (explicit --json/--agent, or a non-terminal stdout like a pipe).
func wantsJSON(cmd *cobra.Command, flags *rootFlags) bool {
	return flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout())
}

// emitJSON writes v as JSON honoring --select/--compact/--quiet/--csv.
func emitJSON(cmd *cobra.Command, flags *rootFlags, v any) error {
	return printJSONFiltered(cmd.OutOrStdout(), v, flags)
}

// renderRestaurantTable prints a compact human comparison table of rows.
func renderRestaurantTable(cmd *cobra.Command, rows []restaurantRow) error {
	out := cmd.OutOrStdout()
	if len(rows) == 0 {
		fmt.Fprintln(out, "No restaurants found.")
		return nil
	}
	tw := newTabWriter(out)
	fmt.Fprintln(tw, "NAME\tFEE\tMIN\tETA\tRATING\tMILES\tDEALS\tID")
	for _, r := range rows {
		rating := "-"
		if r.Rating > 0 {
			rating = fmt.Sprintf("%.1f", r.Rating)
		}
		deals := ""
		if r.Deals > 0 {
			deals = fmt.Sprintf("%d", r.Deals)
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%dm\t%s\t%.2f\t%s\t%s\n",
			truncate(r.Name, 32), r.DeliveryFee, r.Minimum, r.ETAMinutes, rating, r.DistanceMiles, deals, r.ID)
	}
	return tw.Flush()
}

func joinCuisines(c []string, max int) string {
	if len(c) > max {
		c = c[:max]
	}
	return strings.Join(c, ", ")
}
