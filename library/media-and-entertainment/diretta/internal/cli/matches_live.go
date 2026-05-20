// Copyright 2026 prisco-faiella. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/diretta/internal/parser"
	"github.com/spf13/cobra"
)

func newMatchesLiveCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "live",
		Short:       "Currently live football matches (filtered from today's feed by status).",
		Example:     "  diretta-pp-cli matches live",
		Annotations: map[string]string{"pp:endpoint": "matches.live", "pp:method": "GET", "pp:path": "/x/feed/f_1_0_3_it_1", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			raw, prov, err := resolveRead(cmd.Context(), c, flags, "matches", false, "/x/feed/f_1_0_3_it_1", map[string]string{}, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			all := parser.ParseMatches([]byte(raw))
			matches := parser.FilterLive(all)
			data, _ := json.Marshal(matches)
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
						fmt.Fprintf(os.Stderr, "\nShowing %d live matches.\n", len(items))
					}
					return nil
				}
				fmt.Fprintln(cmd.OutOrStdout(), "No live matches right now.")
			}
			return printOutputWithFlags(cmd.OutOrStdout(), jdata, flags)
		},
	}
	return cmd
}
