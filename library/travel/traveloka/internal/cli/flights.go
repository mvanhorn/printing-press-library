// Copyright 2026 Jet Sng and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelFlightsCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "flights",
		Short:       "Work with flights",
		Example:     "  traveloka-pp-cli flights date-grid --origin SIN --destination CGK --depart-dates 2026-11-20,2026-11-21 --limit 2 --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelFlightsDateGridCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelFlightsShortlistCmd(flags))
	return cmd
}
