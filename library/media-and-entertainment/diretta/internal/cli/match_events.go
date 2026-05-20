// Copyright 2026 prisco-faiella. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/diretta/internal/parser"
	"github.com/spf13/cobra"
)

func newMatchEventsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "events <matchId>",
		Short:       "Match events: goals, cards, substitutions.",
		Example:     "  diretta-pp-cli match events vVn0EQM5",
		Annotations: map[string]string{"pp:endpoint": "match.events", "pp:method": "GET", "pp:path": "/x/feed/df_sui_1_{matchId}", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			path := replacePathParam("/x/feed/df_sui_1_{matchId}", "matchId", args[0])
			raw, prov, err := resolveRead(cmd.Context(), c, flags, "match", false, path, map[string]string{}, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			events := parser.ParseEvents([]byte(raw))
			data, _ := json.Marshal(events)
			jdata := json.RawMessage(data)
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				printProvenance(cmd, len(events), prov)
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
					return printAutoTable(cmd.OutOrStdout(), items)
				}
				fmt.Fprintln(os.Stderr, "No events found.")
			}
			return printOutputWithFlags(cmd.OutOrStdout(), jdata, flags)
		},
	}
	return cmd
}
