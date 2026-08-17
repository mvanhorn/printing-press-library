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

func newNovelQuoteCmd(flags *rootFlags) *cobra.Command {
	var flagHelp bool

	cmd := &cobra.Command{
		Use:         "quote",
		Short:       "Compare requested grocery terms across eligible markets and return complete price estimates.",
		Example:     "  ifood-pp-cli quote --help",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			// validate required flags here
			if dryRunOK(flags) {
				return nil
			}
			return fmt.Errorf("TODO: implement novel feature %q", "quote")
		},
	}
	cmd.Flags().BoolVar(&flagHelp, "help", false, "TODO: describe --help")
	return cmd
}
