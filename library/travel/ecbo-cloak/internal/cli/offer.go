// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelOfferCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "offer",
		Short:       "Work with offer",
		Example:     "  ecbo-cloak-pp-cli offer inspect 0c3fb1e9-ad5a-42de-bfda-3027ebe4921e --from 2026-10-03T19:00 --to 2026-10-03T21:00 --small 1 --large 1 --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelOfferInspectCmd(flags))
	return cmd
}
