// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

// newUsageCmd registers the top-level `usage` parent.
func newUsageCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "usage",
		Short: "Estimate credit cost of planned operations",
	}
	cmd.AddCommand(newUsageEstimateCmd(flags))
	return cmd
}
