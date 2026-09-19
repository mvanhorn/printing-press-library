// Copyright 2026 matthew.martin and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelHavenCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "haven",
		Short:       "Browse and compare saved Haven Hot Chicken menus",
		Example:     "  haven-hot-chicken-pp-cli haven changes --location 14208 --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local", "pp:parent-group": "true", "pp:typed-exit-codes": "0,2"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelHavenChangesCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelHavenCommonCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelHavenCompareCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelHavenNearbyCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelHavenSubtotalCmd(flags))
	cmd.AddCommand(newNovelHavenRefreshCmd(flags))
	cmd.AddCommand(newNovelHavenMenuCmd(flags))
	cmd.AddCommand(newNovelHavenLocationsCmd(flags))
	return cmd
}
