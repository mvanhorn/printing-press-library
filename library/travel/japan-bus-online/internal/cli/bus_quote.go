// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// pp:data-source live
package cli

import (
	"fmt"
	"github.com/spf13/cobra"
)

func newNovelBusQuoteCmd(flags *rootFlags) *cobra.Command {
	var o busOptions
	var service, dep, arr string
	var adults, children, plan int
	var includeStops bool
	cmd := &cobra.Command{Use: "quote", Short: "Read actual stop-pair unit fares, party evidence and cancellation fees", Args: cobra.NoArgs, Annotations: busAnnotations("--route=12200160001 --service=0001 --adults=2 --children=1"), Example: "  japan-bus-online-pp-cli bus quote --route 12200160001 --date 2026-10-10 --service 0001 --adults 2 --children 1 --agent", RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "bus quote")
		}
		if adults < 0 || children < 0 || adults > 20 || children > 20 || adults+children < 1 || adults+children > 20 {
			return usageErr(fmt.Errorf("--adults and --children must be nonnegative and request 1-20 seats in total"))
		}
		if plan < 1 || plan > 20 {
			return usageErr(fmt.Errorf("--fare-plan must be 1-20, using a source offered plan"))
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
		out, e := c.Quote(ctx, o.route, o.direction, o.date, service, dep, arr, adults, children, plan)
		if e != nil {
			return classifyAPIError(cmd.OutOrStdout(), e, flags)
		}
		if !includeStops {
			delete(out, "stops")
		}
		return busEmit(cmd, flags, out)
	}}
	cmd.Flags().BoolVar(&includeStops, "include-stops", false, "Include every source boarding and alighting stop identity")
	busFlags(cmd, &o, true)
	cmd.Flags().StringVar(&service, "service", "", "Service ID from bus services; default first available row")
	cmd.Flags().StringVar(&dep, "dep-stop", "", "Boarding stop ID; default first departure stop")
	cmd.Flags().StringVar(&arr, "arr-stop", "", "Alighting stop ID; default last arrival stop")
	cmd.Flags().IntVar(&adults, "adults", 1, "Number of adult seats for one-way arithmetic estimate")
	cmd.Flags().IntVar(&children, "children", 0, "Number of child seats using source age label")
	cmd.Flags().IntVar(&plan, "fare-plan", 1, "One-based source fare-plan choice for this service")
	return cmd
}
func newBusConditionsCmd(flags *rootFlags) *cobra.Command {
	var o busOptions
	cmd := &cobra.Command{Use: "conditions", Short: "Read route/operator baggage, boarding and cancellation conditions", Args: cobra.NoArgs, Annotations: busAnnotations("--route=12200160001"), Example: "  japan-bus-online-pp-cli bus conditions --route 12200160001 --date 2026-10-10 --agent", RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "bus conditions")
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
		out, e := c.Conditions(ctx, o.route, o.direction, o.date)
		if e != nil {
			return classifyAPIError(cmd.OutOrStdout(), e, flags)
		}
		return busEmit(cmd, flags, out)
	}}
	busFlags(cmd, &o, true)
	return cmd
}
