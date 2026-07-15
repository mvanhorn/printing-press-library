// Copyright 2026 Darin Kishore and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "github.com/spf13/cobra"

func newPatternsCmd(flags *rootFlags) *cobra.Command {
	return newProjectionCmd(flags, "patterns", "screenPatterns", "List screen patterns such as paywall.")
}
