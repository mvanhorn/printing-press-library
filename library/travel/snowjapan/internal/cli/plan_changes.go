// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// pp:data-source local
package cli

import "github.com/spf13/cobra"

func newNovelPlanChangesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "changes"}
	configureSnowPlan(cmd, flags, "changes")
	return cmd
}
