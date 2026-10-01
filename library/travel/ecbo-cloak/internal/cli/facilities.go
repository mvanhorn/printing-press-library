// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelFacilitiesCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "facilities",
		Short:       "Work with facilities",
		Example:     "  ecbo-cloak-pp-cli facilities get 0c3fb1e9-ad5a-42de-bfda-3027ebe4921e --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelFacilitiesGetCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelFacilitiesNearCmd(flags))
	return cmd
}
