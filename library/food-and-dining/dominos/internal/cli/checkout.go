// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "github.com/spf13/cobra"

func newCheckoutCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "checkout",
		Short: "Inspect checkout readiness without placing an order",
	}
	cmd.AddCommand(newCheckoutPreviewCmd(flags))
	return cmd
}
