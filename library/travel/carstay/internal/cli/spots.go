// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelSpotsCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "spots",
		Short:       "Work with spots",
		Example:     "  carstay-pp-cli spots audit 632c59b82b614b99a252d1b2 --check-in 2026-10-10 --check-out 2026-10-11 --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelSpotsAuditCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelSpotsCompareCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelSpotsCoverageCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelSpotsFitCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelSpotsNearCmd(flags))
	return cmd
}
