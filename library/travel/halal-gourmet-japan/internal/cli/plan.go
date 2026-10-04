// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelPlanCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "plan",
		Short:       "Work with plan",
		Example:     "  halal-gourmet-japan-pp-cli plan changes --restaurants 300739 --prayer 838884 --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:typed-exit-codes": "0,2"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelPlanChangesCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelPlanCompareCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelPlanGapsCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelPlanMatchCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelPlanPairCmd(flags))
	return cmd
}
