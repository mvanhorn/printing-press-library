// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelOnewayCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "oneway",
		Short:       "Rental planning",
		Example:     "  toyota-rentacar-pp-cli oneway quote --pickup-shop 63601:01V --dropoff-shop 63601:095 --family standard --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelOnewayQuoteCmd(flags))
	return cmd
}
