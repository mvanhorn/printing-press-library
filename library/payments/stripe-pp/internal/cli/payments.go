// Copyright 2026 primiasolutions. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

// newPaymentsCmd is the user-facing `payments` parent — it shells novel
// payment-oriented analytics that operate over PaymentIntents under the hood.
func newPaymentsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "payments",
		Short: "Payment analytics (operates over PaymentIntents)",
	}
	cmd.AddCommand(newPaymentsFailedCmd(flags))
	return cmd
}
