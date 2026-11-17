// Copyright 2026 Josselin and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelNewsCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "news",
		Short:       "Work with news",
		Example:     "  autom-pp-cli news watch Autom --country fr --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:parent-group": "true", "pp:typed-exit-codes": "0,2"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelNewsWatchCmd(flags))
	return cmd
}
