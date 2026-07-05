// Copyright 2026 jimpresting. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

func newInboxCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var cursor string
	cmd := &cobra.Command{
		Use:     "inbox",
		Short:   "List your Airbnb message threads (requires auth login)",
		Example: "  airbnb-outreach-pp-cli inbox --limit 20\n  airbnb-outreach-pp-cli inbox --json --select edges",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c := newAirbnbClient(flags)
			data, err := c.Inbox(limit, cursor)
			if err != nil {
				return classifyAirbnb(err, flags)
			}
			return flags.printJSON(cmd, data)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 15, "Number of threads to return")
	cmd.Flags().StringVar(&cursor, "cursor", "", "Pagination cursor (endCursor from a previous page)")
	return cmd
}

func newThreadCmd(flags *rootFlags) *cobra.Command {
	var messages int
	cmd := &cobra.Command{
		Use:     "thread [thread-id]",
		Short:   "Read a conversation with a host, with its messages",
		Example: "  airbnb-outreach-pp-cli thread 980001234567 --messages 50",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				return cmd.Help()
			}
			c := newAirbnbClient(flags)
			data, err := c.Thread(args[0], messages)
			if err != nil {
				return classifyAirbnb(err, flags)
			}
			return flags.printJSON(cmd, data)
		},
	}
	cmd.Flags().IntVar(&messages, "messages", 50, "Number of messages to fetch")
	return cmd
}
