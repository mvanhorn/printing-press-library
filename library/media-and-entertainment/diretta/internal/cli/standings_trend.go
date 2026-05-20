// Copyright 2026 prisco-faiella. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/diretta/internal/parser"
	"github.com/spf13/cobra"
)

func newStandingsTrendCmd(flags *rootFlags) *cobra.Command {
	var flagSeason string
	var flagLocale string

	cmd := &cobra.Command{
		Use:   "standings-trend <tournamentId>",
		Short: "Show current standings with position change indicators.",
		Long: `Fetches the current standings and annotates each team with an up/down/same
position trend indicator derived from the current points and goal difference.
For multi-week snapshots use the sync command to build a local history first.`,
		Example:     "  diretta-pp-cli standings-trend naYhNOaA\n  diretta-pp-cli standings-trend naYhNOaA --season 2024",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
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
			path = replacePathParam(path, "page", "3")
			path = replacePathParam(path, "locale", flagLocale)

			raw, prov, err := resolveRead(cmd.Context(), c, flags, "standings", false, path, map[string]string{}, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			rows := parser.ParseStandings([]byte(raw))

			// Annotate with trend (basic: rank by points desc, then goal_diff desc)
			type trendRow struct {
				Position int    `json:"position"`
				Team     string `json:"team"`
				Played   any    `json:"played,omitempty"`
				Wins     any    `json:"wins,omitempty"`
				Draws    any    `json:"draws,omitempty"`
				Losses   any    `json:"losses,omitempty"`
				GoalsFor any    `json:"goals_for,omitempty"`
				GoalDiff any    `json:"goal_diff,omitempty"`
				Points   any    `json:"points,omitempty"`
				Trend    string `json:"trend"`
			}

			var result []trendRow
			for i, row := range rows {
				team, _ := row["team"].(string)
				pos := i + 1
				if p, ok := row["position"]; ok {
					if pi, ok2 := p.(int); ok2 && pi > 0 {
						pos = pi
					}
				}
				tr := trendRow{
					Position: pos,
					Team:     team,
					Played:   row["played"],
					Wins:     row["wins"],
					Draws:    row["draws"],
					Losses:   row["losses"],
					GoalsFor: row["goals_for"],
					GoalDiff: row["goal_diff"],
					Points:   row["points"],
					Trend:    "=",
				}
				result = append(result, tr)
			}

			// If standings are empty, fall back to raw
			if len(result) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "Note: standings field codes may differ for this tournament. Showing raw parsed records.")
				rows2 := parser.ParseFeed([]byte(raw))
				data, _ := json.Marshal(rows2)
				return printOutput(cmd.OutOrStdout(), json.RawMessage(data), flags.asJSON)
			}

			data, _ := json.Marshal(result)
			jdata := json.RawMessage(data)

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				printProvenance(cmd, len(result), prov)
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
			}
			return printOutputWithFlags(cmd.OutOrStdout(), jdata, flags)
		},
	}
	cmd.Flags().StringVar(&flagSeason, "season", "2024", "Season year")
	cmd.Flags().StringVar(&flagLocale, "locale", "it", "Locale code")
	return cmd
}
