// Copyright 2026 prisco-faiella. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/diretta/internal/parser"
	"github.com/spf13/cobra"
)

func newRawCmd(flags *rootFlags) *cobra.Command {
	var flagDecode bool

	cmd := &cobra.Command{
		Use:   "raw <path>",
		Short: "Dump raw FlashScore field-value pairs for any feed path — for debugging and protocol discovery.",
		Long: `Fetches a FlashScore feed path and prints the raw key÷value pairs in a
human-readable format. Useful for exploring undocumented fields and
reverse-engineering new endpoints.

Examples of valid paths:
  /x/feed/f_1_0_3_it_1        today's football
  /x/feed/dc_1_MATCHID        match detail
  /x/feed/df_st_1_MATCHID     match stats`,
		Example: `  diretta-pp-cli raw /x/feed/f_1_0_3_it_1
  diretta-pp-cli raw /x/feed/dc_1_vVn0EQM5 --decode`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return usageErr(fmt.Errorf("path argument required\nUsage: %s <path>", cmd.CommandPath()))
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			// Always bypass cache for raw dumps
			c.NoCache = true
			raw, _, err := resolveRead(cmd.Context(), c, flags, "raw", false, args[0], map[string]string{}, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if flagDecode {
				records := parser.ParseFeed([]byte(raw))
				w := cmd.OutOrStdout()
				for i, rec := range records {
					fmt.Fprintf(w, "--- record %d ---\n", i+1)
					for k, v := range rec {
						fmt.Fprintf(w, "  %s: %s\n", k, v)
					}
				}
				return nil
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(raw))
			return nil
		},
	}
	cmd.Flags().BoolVar(&flagDecode, "decode", false, "Decode and print key-value pairs instead of raw bytes")
	return cmd
}
