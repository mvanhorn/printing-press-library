// Copyright 2026 USER and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

func newExtractCmd() *cobra.Command {
	opts := defaultSourceOptions()
	cmd := &cobra.Command{
		Use:   "extract",
		Short: "Fetch public NYNJ World Cup Concierge sources and emit normalized JSON candidates for Trip Control Tower and other agents.",
		Example: `  nynj-world-cup-concierge-pp-cli extract --agent
  nynj-world-cup-concierge-pp-cli extract --agent --category "Fan Experiences" --category "Watch Parties" --date-window-start 2026-07-02 --date-window-end 2026-07-06 --exclude-undated`,
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := buildPayload(opts)
			if err != nil {
				return err
			}
			return printJSONWithIndent(data, opts.Pretty && !opts.Agent)
		},
	}
	bindCommonSourceFlags(cmd, &opts)
	bindExtractFilterFlags(cmd, &opts)
	return cmd
}
