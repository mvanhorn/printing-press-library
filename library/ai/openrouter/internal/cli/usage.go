// Copyright 2026 rick-van-de-laar. Licensed under Apache-2.0. See LICENSE.

package cli

// PATCH transcendence-commands: hand-built parent for usage cost-by + usage anomaly.

import (
	"github.com/spf13/cobra"
)

func newUsageCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "usage",
		Short: "Local-derived usage analytics (cost-by, anomaly)",
	}
	cmd.AddCommand(newUsageCostByCmd(flags))
	cmd.AddCommand(newUsageAnomalyCmd(flags))
	return cmd
}
