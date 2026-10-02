// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelBookingCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "booking",
		Short:       "Rental planning",
		Example:     "  toyota-rentacar-pp-cli booking handoff --pickup-shop 63601:01V --pickup 2026-10-20T09:00 --dropoff 2026-10-21T09:00 --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelBookingHandoffCmd(flags))
	return cmd
}
