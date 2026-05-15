// Copyright 2026 erick-holm. Licensed under Apache-2.0. See LICENSE.

// This file is hand-built (NOT generator-emitted). It is the single
// wiring point for the 12 v1.1 novel verbs. root.go calls attachP1Cmds
// once; this keeps the generated root.go free of hand-written command
// registrations.

package cli

import "github.com/spf13/cobra"

// attachP1Cmds registers the 12 hand-written v1.1 novel verbs on the
// root command:
//
//	read-side:  digest, customer-intel, drift, dms-summary, dormant,
//	            attention, who-said, thread-summary
//	write-side: post, schedule
//	plumbing:   channel-find, user-find
//
// All read-side verbs are MCP read-only; post and schedule are mutating
// and intentionally omit the mcp:read-only annotation.
func attachP1Cmds(rootCmd *cobra.Command, flags *rootFlags) {
	rootCmd.AddCommand(newDigestCmd(flags))
	rootCmd.AddCommand(newCustomerIntelCmd(flags))
	rootCmd.AddCommand(newDriftCmd(flags))
	rootCmd.AddCommand(newDMSummaryCmd(flags))
	rootCmd.AddCommand(newDormantCmd(flags))
	rootCmd.AddCommand(newAttentionCmd(flags))
	rootCmd.AddCommand(newWhoSaidCmd(flags))
	rootCmd.AddCommand(newThreadSummaryCmd(flags))
	rootCmd.AddCommand(newPostCmd(flags))
	rootCmd.AddCommand(newScheduleCmd(flags))
	rootCmd.AddCommand(newChannelFindCmd(flags))
	rootCmd.AddCommand(newUserFindCmd(flags))
}
