// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelParksCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "parks",
		Short:       "Work with parks",
		Example:     "  kurumatabi-pp-cli parks audit rvpark/712 --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelParksAuditCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelParksCompareCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelParksFitCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelParksMatchCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelParksNearCmd(flags))
	return cmd
}
