// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelRoutesCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "routes",
		Short:       "Approve, unapprove, and review subnet routes and exit nodes",
		Example:     "  tailscale-pp-cli routes overview --pending",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:typed-exit-codes": "0,2"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelRoutesApproveCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelRoutesOverviewCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelRoutesUnapproveCmd(flags))
	return cmd
}
