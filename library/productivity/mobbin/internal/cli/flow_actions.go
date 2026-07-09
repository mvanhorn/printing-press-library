// Copyright 2026 Darin Kishore and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "github.com/spf13/cobra"

func newFlowActionsCmd(flags *rootFlags) *cobra.Command {
	return newProjectionCmd(flags, "flow-actions", "flowActions", "List flow actions such as onboarding.")
}
