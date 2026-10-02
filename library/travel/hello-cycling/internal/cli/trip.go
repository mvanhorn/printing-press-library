// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelTripCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "trip",
		Short:       "Station planning",
		Example:     "  hello-cycling-pp-cli trip compare --from-lat 35.697315 --from-lon 139.704995 --to-lat 35.707252 --to-lon 139.777587 --vehicle-type 2 --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelTripCompareCmd(flags))
	return cmd
}
