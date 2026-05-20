// Copyright 2026 magoo242. Licensed under Apache-2.0. See LICENSE.

// `edgar-pp-cli accession <ID>` — normalize an SEC accession number between
// the dashed (0000320193-22-000049) and no-dashes (000032019322000049) forms.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newAccessionCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "accession <id>",
		Short: "Normalize an SEC accession number to both with-dashes and no-dashes forms",
		Long: `Normalize an SEC accession number. Accepts either 0000320193-22-000049 (with dashes,
how data.sec.gov returns it) or 000032019322000049 (no dashes, how Archive URLs need it).
Emits both forms so any downstream URL or query can use the right one.`,
		Example: "  edgar-pp-cli accession 0000320193-22-000049",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			withDashes, noDashes, err := normalizeAccession(args[0])
			if err != nil {
				return usageErr(err)
			}
			out := map[string]string{
				"with_dashes": withDashes,
				"no_dashes":   noDashes,
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) || flags.compact {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(out)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "with_dashes: %s\nno_dashes:   %s\n", withDashes, noDashes)
			return nil
		},
	}
	return cmd
}
