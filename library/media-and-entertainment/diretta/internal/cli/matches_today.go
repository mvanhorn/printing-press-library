// Copyright 2026 prisco-faiella. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/diretta/internal/parser"
	"github.com/spf13/cobra"
)

func newMatchesTodayCmd(flags *rootFlags) *cobra.Command {
	var flagSport string

	cmd := &cobra.Command{
		Use:         "today",
		Short:       "Today's football matches grouped by tournament.",
		Example:     "  diretta-pp-cli matches today\n  diretta-pp-cli matches today --json",
		Annotations: map[string]string{"pp:endpoint": "matches.today", "pp:method": "GET", "pp:path": "/x/feed/f_1_0_3_it_1", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := "/x/feed/f_1_0_3_it_1"
			if flagSport != "" {
				path = "/x/feed/f_" + flagSport + "_0_3_it_1"
			}
			params := map[string]string{}
			raw, prov, err := resolveRead(cmd.Context(), c, flags, "matches", false, path, params, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			matches := parser.ParseMatches([]byte(raw))
			data, merr := json.Marshal(matches)
			if merr != nil {
				return merr
			}
			jdata := json.RawMessage(data)

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				printProvenance(cmd, len(matches), prov)
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				filtered := jdata
				if flags.selectFields != "" {
					filtered = filterFields(filtered, flags.selectFields)
				} else if flags.compact {
					filtered = compactFields(filtered)
				}
				wrapped, wrapErr := wrapWithProvenance(filtered, prov)
				if wrapErr != nil {
					return wrapErr
				}
				return printOutput(cmd.OutOrStdout(), wrapped, true)
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				var items []map[string]any
				if json.Unmarshal(jdata, &items) == nil && len(items) > 0 {
					if err := printAutoTable(cmd.OutOrStdout(), items); err != nil {
						return err
					}
					if len(items) >= 25 {
						fmt.Fprintf(os.Stderr, "\nShowing %d results.\n", len(items))
					}
					return nil
				}
			}
			return printOutputWithFlags(cmd.OutOrStdout(), jdata, flags)
		},
	}
	cmd.Flags().StringVar(&flagSport, "sport", "", "Sport code (1=football, 2=all, 3=tennis …). Default: football (1)")
	return cmd
}
