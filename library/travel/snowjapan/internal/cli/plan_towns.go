// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// pp:data-source local
package cli

import "github.com/spf13/cobra"

func newNovelPlanTownsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "towns"}
	configureSnowPlan(cmd, flags, "towns")
	return cmd
}
