// Copyright 2026 Chirantan Rajhans and contributors. Licensed under Apache-2.0. See LICENSE.
// PATCH(reader-subscriptions-list): hand-authored reader sub-tree; see .printing-press-patches/.

package cli

import (
	"github.com/spf13/cobra"
)

func newReaderCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "reader",
		Short:       "Reader-side commands — subscriptions, following, and reading list",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}

	cmd.AddCommand(newReaderSubscriptionsCmd(flags))
	return cmd
}
