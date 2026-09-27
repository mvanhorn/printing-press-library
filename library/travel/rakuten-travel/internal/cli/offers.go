// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source live
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelOffersCmd(flags *rootFlags) *cobra.Command {

	offersGroupCmd := &cobra.Command{
		Use:         "offers",
		Short:       "Inspect real dated room and plan offers",
		Example:     "  rakuten-travel-pp-cli offers search --hotel 51870 --checkin 2026-11-08 --checkout 2026-11-10 --rooms 1 --adults-per-room 2",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"},
		RunE:        publicTravelGroup,
	}
	offersGroupCmd.AddCommand(newNovelOffersSearchCmd(flags))
	offersGroupCmd.AddCommand(newNovelOffersShowCmd(flags))
	return offersGroupCmd
}
