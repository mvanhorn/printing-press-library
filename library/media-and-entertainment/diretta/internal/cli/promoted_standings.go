// Copyright 2026 prisco-faiella. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/diretta/internal/parser"
	"github.com/spf13/cobra"
)

func newStandingsPromotedCmd(flags *rootFlags) *cobra.Command {
	var flagSeason string
	var flagPage string
	var flagLocale string

	cmd := &cobra.Command{
		Use:         "standings <tournamentId>",
		Short:       "League table for a tournament.",
		Long:        "League standings for a tournament. Pass the tournament ID (ZEE field from 'matches today' output).",
		Example:     "  diretta-pp-cli standings naYhNOaA\n  diretta-pp-cli standings naYhNOaA --season 2024 --json",
		Annotations: map[string]string{"pp:endpoint": "standings.table", "pp:method": "GET", "pp:path": "/x/feed/tr_{tournamentId}_{season}_{page}_3_{locale}_1", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"error": "tournamentId is required",
						"usage": fmt.Sprintf("%s <tournamentId>", cmd.CommandPath()),
					}, flags)
				}
				return usageErr(fmt.Errorf("tournamentId is required\nUsage: %s <tournamentId>", cmd.CommandPath()))
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			path := "/x/feed/tr_{tournamentId}_{season}_{page}_3_{locale}_1"
			path = replacePathParam(path, "tournamentId", args[0])
			path = replacePathParam(path, "season", flagSeason)
			path = replacePathParam(path, "page", flagPage)
			path = replacePathParam(path, "locale", flagLocale)
			raw, prov, err := resolveRead(cmd.Context(), c, flags, "standings", false, path, map[string]string{}, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			rows := parser.ParseStandings([]byte(raw))
			data, _ := json.Marshal(rows)
			jdata := json.RawMessage(data)
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				printProvenance(cmd, len(rows), prov)
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
						fmt.Fprintf(os.Stderr, "\nShowing %d rows.\n", len(items))
					}
					return nil
				}
				// Standings parse is best-effort; fall back to raw display
				fmt.Fprintln(os.Stderr, "Note: standings field codes for this tournament may differ. Use --json to see raw parsed records.")
				return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), jdata, flags)
		},
	}
	cmd.Flags().StringVar(&flagSeason, "season", "2024", "Season year (e.g. 2024 for 2024/25 season)")
	cmd.Flags().StringVar(&flagPage, "page", "3", "Page number for standings")
	cmd.Flags().StringVar(&flagLocale, "locale", "it", "Locale code (it=Italian, en=English)")
	return cmd
}
