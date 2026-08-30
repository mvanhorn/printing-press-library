// Copyright 2026 and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelRoadmapCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "roadmap",
		Short:       "Release version roadmap reports (burndown: open/closed counts, completion, logged hours)",
		Example:     "  redmine-pp-cli roadmap burndown 1.0 --project demo --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelRoadmapBurndownCmd(flags))
	return cmd
}
