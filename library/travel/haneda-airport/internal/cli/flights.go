// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
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
		Short:       "Flight planning and observations",
		Example:     "  haneda-airport-pp-cli flights rollover --kind international --direction departure --limit 5",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newHanedaFlightsSearchCmd(flags))
	cmd.AddCommand(newHanedaDetailCmd(flags))
	cmd.AddCommand(newHanedaFlightsDisruptionsCmd(flags))
	cmd.AddCommand(newNovelFlightsRolloverCmd(flags))
	return cmd
}
