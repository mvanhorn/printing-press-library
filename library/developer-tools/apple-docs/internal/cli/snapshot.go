// Copyright 2026 joseph-alvin-castillo. Licensed under Apache-2.0. See LICENSE.

// pp:data-source local

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelSnapshotCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "snapshot",
		Short:       "Save and diff frozen copies of a framework index for WWDC-week diffs",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelSnapshotSaveCmd(flags))
	cmd.AddCommand(newNovelSnapshotListCmd(flags))
	cmd.AddCommand(newNovelSnapshotDiffCmd(flags))
	return cmd
}
