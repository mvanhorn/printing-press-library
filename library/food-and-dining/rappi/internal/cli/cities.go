// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/rappi/internal/source/rappi"

	"github.com/spf13/cobra"
)

func newCitiesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cities",
		Short: "List Mexican cities served by Rappi with default coordinates",
	}
	cmd.AddCommand(newCitiesListCmd(flags))
	return cmd
}

func newCitiesListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List the Rappi-served cities baked into the CLI",
		Long: `List every Mexican city this CLI ships with as a closed enum.
Each entry includes the canonical Rappi slug, the display name, the
Mexican state, and a default centroid lat/lng used by geo-scoped
commands (restaurants near, stores adjacency) when no --lat/--lng
is provided.`,
		Example:     "  rappi-pp-cli cities list --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			rows := rappi.Cities
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			// Human table.
			out := cmd.OutOrStdout()
			fmt.Fprintln(out, "SLUG               NAME               STATE             LAT      LNG")
			fmt.Fprintln(out, strings.Repeat("-", 68))
			for _, c := range rows {
				fmt.Fprintf(out, "%-18s %-18s %-17s %.4f  %.4f\n",
					c.Slug, c.Name, c.State, c.Latitude, c.Longitude)
			}
			return nil
		},
	}
	return cmd
}
