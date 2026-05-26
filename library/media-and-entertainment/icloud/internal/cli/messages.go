// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package cli

import "github.com/spf13/cobra"

func newMessagesCmd(f *rootFlags) *cobra.Command {
	messages := &cobra.Command{
		Use:   "messages",
		Short: "Query your iMessage history",
		Long: `Read your macOS Messages database (~/Library/Messages/chat.db) directly.

All commands open chat.db in read-only mode. Full Disk Access is required —
run "icloud-pp-cli doctor" if any command fails with a permission error.`,
	}

	// --messages-db is registered at the root level (see root.go) so that
	// `doctor` and any future sibling command can read the same override.

	messages.AddCommand(newMessagesListChatsCmd(f))
	messages.AddCommand(newMessagesSearchCmd(f))
	messages.AddCommand(newMessagesStatsCmd(f))
	messages.AddCommand(newMessagesExportCmd(f))

	return messages
}
