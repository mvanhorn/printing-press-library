// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelStationsCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "stations",
		Short:       "Station planning",
		Example:     "  hello-cycling-pp-cli stations nearby --lat 35.697315 --lon 139.704995 --purpose pickup --vehicle-type 2 --limit 1 --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelStationsChangesCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelStationsNearbyCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelStationsSyncCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelStationsFindCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelStationsShowCmd(flags))
	return cmd
}
