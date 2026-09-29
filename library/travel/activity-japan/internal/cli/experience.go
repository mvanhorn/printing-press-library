// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelExperienceCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "experience",
		Short:       "Work with experience",
		Example:     "  activity-japan-pp-cli experience brief 62375 --date 2026-10-08 --adults 2 --lang en --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelExperienceBriefCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelExperienceCompareCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelExperienceDatesCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelExperiencePriceCmd(flags))
	return cmd
}
