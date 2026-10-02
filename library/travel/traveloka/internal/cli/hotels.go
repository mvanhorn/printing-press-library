// Copyright 2026 Jet Sng and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelHotelsCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "hotels",
		Short:       "Work with hotels",
		Example:     "  traveloka-pp-cli hotels date-grid --property-id 9000000001714 --stays 2027-01-06:2027-01-08,2027-01-13:2027-01-15 --adults 2 --rooms 1 --limit 2 --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelHotelsDateGridCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelHotelsFlexibilityCmd(flags))
	return cmd
}
