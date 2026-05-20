// Copyright 2026 prisco-faiella. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/diretta/internal/parser"
	"github.com/spf13/cobra"
)

func newFormCmd(flags *rootFlags) *cobra.Command {
	var flagN int

	cmd := &cobra.Command{
		Use:   "form <matchId>",
		Short: "Show last-N form for both teams using the match's H2H feed.",
		Long: `Fetches the H2H feed for a match and computes a simple form string
(W/D/L) for each team from the most recent N results, split by home/away.`,
		Example:     "  diretta-pp-cli form vVn0EQM5\n  diretta-pp-cli form vVn0EQM5 --n 10",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return usageErr(fmt.Errorf("matchId is required\nUsage: %s <matchId>", cmd.CommandPath()))
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			path := replacePathParam("/x/feed/df_hh_1_{matchId}", "matchId", args[0])
			raw, prov, err := resolveRead(cmd.Context(), c, flags, "match", false, path, map[string]string{}, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			h2h := parser.ParseH2H([]byte(raw))
			if len(h2h) == 0 {
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"message": "no h2h data"}, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "No H2H data available.")
				return nil
			}

			// Determine the two teams from the first h2h record
			homeTeam, awayTeam := "", ""
			if len(h2h) > 0 {
				homeTeam, _ = h2h[0]["home_team"].(string)
				awayTeam, _ = h2h[0]["away_team"].(string)
			}

			// Limit to N most recent
			all := h2h
			if flagN > 0 && len(all) > flagN {
				all = all[len(all)-flagN:]
			}

			type formRow struct {
				Team         string `json:"team"`
				Form         string `json:"form"`
				FormHome     string `json:"form_home"`
				FormAway     string `json:"form_away"`
				GoalsFor     int    `json:"goals_for"`
				GoalsAgainst int    `json:"goals_against"`
				Wins         int    `json:"wins"`
				Draws        int    `json:"draws"`
				Losses       int    `json:"losses"`
			}

			buildForm := func(team string) formRow {
				var full, home, away strings.Builder
				gf, ga, w, d, l := 0, 0, 0, 0, 0
				for _, m := range all {
					ht, _ := m["home_team"].(string)
					at, _ := m["away_team"].(string)
					if ht == "" && at == "" {
						continue
					}
					hsInt, asInt := toAnyInt(m["home_score"]), toAnyInt(m["away_score"])
					var result string
					if team == ht {
						gf += hsInt
						ga += asInt
						if hsInt > asInt {
							result = "W"
							w++
						} else if hsInt == asInt {
							result = "D"
							d++
						} else {
							result = "L"
							l++
						}
						home.WriteString(result)
					} else if team == at {
						gf += asInt
						ga += hsInt
						if asInt > hsInt {
							result = "W"
							w++
						} else if asInt == hsInt {
							result = "D"
							d++
						} else {
							result = "L"
							l++
						}
						away.WriteString(result)
					} else {
						continue
					}
					full.WriteString(result)
				}
				return formRow{
					Team:         team,
					Form:         full.String(),
					FormHome:     home.String(),
					FormAway:     away.String(),
					GoalsFor:     gf,
					GoalsAgainst: ga,
					Wins:         w,
					Draws:        d,
					Losses:       l,
				}
			}

			rows := []formRow{buildForm(homeTeam), buildForm(awayTeam)}
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
				if json.Unmarshal(jdata, &items) == nil {
					return printAutoTable(cmd.OutOrStdout(), items)
				}
			}
			return printOutputWithFlags(cmd.OutOrStdout(), jdata, flags)
		},
	}
	cmd.Flags().IntVar(&flagN, "n", 5, "Number of recent matches to include in form")
	return cmd
}
