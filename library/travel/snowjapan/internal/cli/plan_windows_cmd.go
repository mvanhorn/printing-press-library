// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// pp:data-source local
package cli

import "github.com/spf13/cobra"

func newNovelPlanWindowsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "windows"}
	configureSnowPlan(cmd, flags, "windows")
	return cmd
}
