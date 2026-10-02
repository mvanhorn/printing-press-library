// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// pp:data-source live
package cli

import (
	"github.com/spf13/cobra"
)

func newNovelBusCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "bus", Short: "Read-only bus schedules, dated inventory, fares and conditions", Example: "  japan-bus-online-pp-cli bus route --route 12200160001 --agent", RunE: parentNoSubcommandRunE(flags)}
	cmd.AddCommand(newNovelBusRouteCmd(flags), newNovelBusServicesCmd(flags), newNovelBusQuoteCmd(flags), newBusConditionsCmd(flags))
	return cmd
}
