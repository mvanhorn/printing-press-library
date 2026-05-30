// Copyright 2026 Wade Carpenter and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelBlueprintCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "blueprint",
		Short:       "Blueprint operations: sync to a git repo, promote dev → prod with ID remap, diff, restore",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelBlueprintDiffCmd(flags))
	cmd.AddCommand(newNovelBlueprintPromoteCmd(flags))
	cmd.AddCommand(newNovelBlueprintSyncCmd(flags))
	cmd.AddCommand(newNovelBlueprintRestoreCmd(flags))
	return cmd
}
