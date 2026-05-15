// Copyright 2026 erick-holm. Licensed under Apache-2.0. See LICENSE.

// This file is hand-built (NOT generator-emitted). It wires the 8 P2
// transcendence verbs onto the root command. root.go is generator-owned
// and must not be edited; it calls attachP2Cmds once, which AddCommands
// every P2 verb (with `reactions` and `usergroups` as parent commands
// holding their `summarize`/`list` subcommands).

package cli

import "github.com/spf13/cobra"

// attachP2Cmds registers the 8 hand-written P2 verbs on rootCmd. The
// `reactions` and `usergroups` parents are added as single commands —
// each already carries its subcommand (summarize / list).
func attachP2Cmds(rootCmd *cobra.Command, flags *rootFlags) {
	rootCmd.AddCommand(newCustomerIntelDeepCmd(flags))
	rootCmd.AddCommand(newDMEngagementCmd(flags))
	rootCmd.AddCommand(newActionFollowthroughCmd(flags))
	rootCmd.AddCommand(newGoalChannelPulseCmd(flags))
	rootCmd.AddCommand(newReactionsCmd(flags))
	rootCmd.AddCommand(newUnreadsCmd(flags))
	rootCmd.AddCommand(newUsergroupsCmd(flags))
	rootCmd.AddCommand(newAgentAuditCmd(flags))
}
