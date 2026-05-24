// Copyright 2026 azaaron. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

func newTrainingCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "training",
		Short: "Training analytics computed from synced activity data",
		Long:  "Analyze training load, zone distribution, and effort trends from locally synced Strava data.",
		RunE:  parentNoSubcommandRunE(flags),
	}

	cmd.AddCommand(newTrainingLoadCmd(flags))
	cmd.AddCommand(newTrainingZonesCmd(flags))
	return cmd
}
