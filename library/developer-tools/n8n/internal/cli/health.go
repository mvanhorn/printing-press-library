// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

func newHealthCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "health",
		Short: "Health checks and cross-instance comparison",
	}
	cmd.AddCommand(newHealthCompareCmd(flags))
	return cmd
}
