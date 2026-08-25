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

func newNovelBrowserValidateQuoteCmd(flags *rootFlags) *cobra.Command {
	var flagInput string

	cmd := &cobra.Command{
		Use:         "validate-quote",
		Short:       "Validate complete product observations from at least three markets meeting a configurable rating floor.",
		Example:     "  ifood-pp-cli browser validate-quote --json --no-learn browser validate-quote --input ./ifood-quote.json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			// validate required flags here
			if dryRunOK(flags) {
				return nil
			}
			return fmt.Errorf("TODO: implement novel feature %q", "browser validate-quote")
		},
	}
	cmd.Flags().StringVar(&flagInput, "input", "", "TODO: describe --input")
	return cmd
}
