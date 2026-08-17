// Copyright 2026 Shoffner and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored novel command group. Snapshots forecasts into local SQLite
// (the API keeps no personal history) and reviews them over time.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelJournalCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "journal",
		Short:       "Snapshot forecasts to a local journal and review them over time (log, show).",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelJournalLogCmd(flags))
	cmd.AddCommand(newNovelJournalShowCmd(flags))
	return cmd
}
