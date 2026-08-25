// Copyright 2026 Elliott Jacobs and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel command preserved by generate --force.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelSnapshotCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "snapshot",
		Short:       "snapshot subcommands: diff",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelSnapshotDiffCmd(flags))
	return cmd
}
