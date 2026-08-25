// Copyright 2026 Derik Parkinson and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command parent: events. Subcommands live in events_exceptions.go and
// events_update.go.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelEventsCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "events",
		Short:       "events subcommands: exceptions, update",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelEventsExceptionsCmd(flags))
	cmd.AddCommand(newNovelEventsUpdateCmd(flags))
	return cmd
}
