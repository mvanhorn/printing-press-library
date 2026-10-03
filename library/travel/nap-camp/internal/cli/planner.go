// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelPlannerCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "planner",
		Short:       "Compare pitch evidence, candidate dates and saved observations",
		Example:     "  nap-camp-pp-cli planner changes before.json after.json --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:parent-group": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelPlannerChangesCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelPlannerCompareCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelPlannerFitCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelPlannerSnapshotCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelPlannerWindowsCmd(flags))
	return cmd
}
