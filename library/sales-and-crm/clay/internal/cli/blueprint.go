// Copyright 2026 Ade Amos and contributors. Licensed under Apache-2.0. See LICENSE.
// Group command for the blueprint family; the leaves carry the behavior.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelBlueprintCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "blueprint",
		Short:       "Design as code",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:parent-group": "true", "pp:typed-exit-codes": "0,2"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelBlueprintApplyCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelBlueprintExportCmd(flags))
	return cmd
}
