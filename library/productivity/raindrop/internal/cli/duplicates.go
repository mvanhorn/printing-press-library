// Copyright 2026 srijits and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelDuplicatesCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "duplicates",
		Short:       "Plan and safely apply duplicate cleanup",
		Example:     "  raindrop-pp-cli duplicates plan --canonical richest --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelDuplicatesPlanCmd(flags))
	addNovelCommandIfAbsent(cmd, newDuplicatesApplyCmd(flags))
	return cmd
}
