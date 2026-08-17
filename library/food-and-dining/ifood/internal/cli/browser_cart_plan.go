// Copyright 2026 Matheus Coêlho and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newNovelBrowserCartPlanCmd(flags *rootFlags) *cobra.Command {
	var flagInput string

	cmd := &cobra.Command{
		Use:         "cart-plan",
		Short:       "Produce the exact selected merchant, products, quantities, and expected total before any cart interaction.",
		Example:     "  ifood-pp-cli browser cart-plan --json --no-learn browser cart-plan --input ./ifood-quote.json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			// validate required flags here
			if dryRunOK(flags) {
				return nil
			}
			return fmt.Errorf("TODO: implement novel feature %q", "browser cart-plan")
		},
	}
	cmd.Flags().StringVar(&flagInput, "input", "", "TODO: describe --input")
	return cmd
}
