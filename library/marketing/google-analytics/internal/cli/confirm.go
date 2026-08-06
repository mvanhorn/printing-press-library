// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// requireExplicitYes gates destructive mutations. It inspects the flag's
// Changed state rather than rootFlags.yes on purpose: --agent implies --yes,
// and agent mode alone must never authorize a destructive call.
func requireExplicitYes(cmd *cobra.Command, action string) error {
	if cmd.Flags().Changed("yes") {
		return nil
	}
	return fmt.Errorf("%s is destructive and irreversible: re-run with an explicit --yes (--agent alone does not authorize it)", action)
}
