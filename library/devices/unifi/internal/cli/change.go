// Copyright 2026 Ricardo Cabral and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelChangeCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "change",
		Short:       "Preview, apply, list and undo journaled changes to official Network and Protect paths",
		Example:     "  unifi-pp-cli change apply firewall-policy.json --trial --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:typed-exit-codes": "0,2"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelChangeApplyCmd(flags))
	return cmd
}
