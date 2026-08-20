// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "github.com/spf13/cobra"

// pp:data-source live
func newMessagesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "messages",
		Short:       "List your OfferUp message threads (requires login)",
		Example:     "  offerup-pp-cli messages --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAuthRead(cmd, flags, []any{}, func() (any, error) {
				return newOfferupClient(flags).Chats(cmd.Context())
			})
		},
	}
	cmd.AddCommand(newMessagesReadCmd(flags))
	return cmd
}

// pp:data-source live
func newMessagesReadCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "read <discussion-id>",
		Short:   "Read the messages in one conversation",
		Example: "  offerup-pp-cli messages read <discussion-id> --agent",
		// Auth-gated read: under the dogfood harness with no servable session it
		// skips (exit 0), so an invalid-id error-path probe doesn't apply.
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			return runAuthRead(cmd, flags, map[string]any{}, func() (any, error) {
				return newOfferupClient(flags).ChatDiscussion(cmd.Context(), args[0])
			})
		},
	}
}
