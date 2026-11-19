// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelMoneyCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "money",
		Short:       "Work with money",
		Example:     "  etsy-pp-cli money fees --since 30d --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:parent-group": "true", "pp:typed-exit-codes": "0,2"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelMoneyFeesCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelMoneyReconcileCmd(flags))
	return cmd
}
