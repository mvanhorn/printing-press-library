// Copyright 2026 Jet Sng and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelQuotesCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "quotes",
		Short:       "Retrieval history",
		Example:     "  traveloka-pp-cli quotes diff --before /private/tmp/traveloka-flight-before.json --after /private/tmp/traveloka-flight-after.json --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelQuotesDiffCmd(flags))
	return cmd
}
