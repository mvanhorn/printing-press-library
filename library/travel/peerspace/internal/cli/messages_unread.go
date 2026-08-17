// Copyright 2026 nspage and contributors. Licensed under Apache-2.0. See LICENSE.
// Messaging v2 unread count from listing/contact flow HAR.
// GET /v2/messaging/messages/inbox/unread-count

package cli

import (
	"github.com/spf13/cobra"
)

func newMessagesUnreadCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "unread",
		Short:   "Inbox unread thread count (GET /v2/messaging/messages/inbox/unread-count).",
		Example: `  peerspace-pp-cli messages unread --agent`,
		Annotations: map[string]string{
			"pp:endpoint":   "messages.unread_count",
			"pp:method":     "GET",
			"pp:path":       "/v2/messaging/messages/inbox/unread-count",
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, _, err := resolveReadWithStrategyAndResponsePath(
				cmd.Context(), c, flags, "live", "messages", false,
				"/v2/messaging/messages/inbox/unread-count", nil, listingAuthHeaders(c), "", cmd.ErrOrStderr(),
			)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printJSONFiltered(cmd.OutOrStdout(), data, flags)
		},
	}
	return cmd
}
