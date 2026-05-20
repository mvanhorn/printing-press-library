// Copyright 2026 prisco-faiella. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/diretta/internal/parser"
	"github.com/spf13/cobra"
)

func newPregameCmd(flags *rootFlags) *cobra.Command {
	var flagDate string
	var flagTournamentID string

	cmd := &cobra.Command{
		Use:   "pregame",
		Short: "For each match on a date, show last-3 H2H and current odds in one table.",
		Long: `Fetches the day's matches (today, yesterday, or tomorrow) and for each one
fetches H2H data, then renders a pregame scout table: match time, teams,
last-3 H2H scores, and home/draw/away win rates.`,
		Example:     "  diretta-pp-cli pregame\n  diretta-pp-cli pregame --tournament naYhNOaA\n  diretta-pp-cli pregame --date yesterday",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Determine feed path
			feedPath := "/x/feed/f_1_0_3_it_1"
			switch flagDate {
			case "yesterday":
				feedPath = "/x/feed/f_1_-1_3_it_1"
			case "tomorrow":
				feedPath = "/x/feed/f_1_1_3_it_1"
			}

			raw, _, err := resolveRead(cmd.Context(), c, flags, "matches", false, feedPath, map[string]string{}, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			allMatches := parser.ParseMatches([]byte(raw))

			// Filter by tournament if specified
			var matches []map[string]any
			for _, m := range allMatches {
				if flagTournamentID != "" {
					tid, _ := m["tournament_id"].(string)
					if tid != flagTournamentID {
						continue
					}
				}
				matches = append(matches, m)
			}

			if len(matches) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No matches found.")
				return nil
			}

			type pregameRow struct {
				Date       string `json:"date"`
				Tournament string `json:"tournament"`
				HomeTeam   string `json:"home_team"`
				AwayTeam   string `json:"away_team"`
				Status     string `json:"status"`
				H2HLast3   string `json:"h2h_last_3"`
				HomeWinPct int    `json:"home_win_pct"`
				DrawPct    int    `json:"draw_pct"`
				AwayWinPct int    `json:"away_win_pct"`
			}

			var rows []pregameRow
			for _, m := range matches {
				matchID, _ := m["id"].(string)
				ht, _ := m["home_team"].(string)
				at, _ := m["away_team"].(string)

				row := pregameRow{
					Date:       fmt.Sprintf("%v", m["date"]),
					Tournament: fmt.Sprintf("%v", m["tournament"]),
					HomeTeam:   ht,
					AwayTeam:   at,
					Status:     fmt.Sprintf("%v", m["status"]),
				}

				// Fetch H2H (best effort — skip on error)
				if matchID != "" {
					h2hPath := replacePathParam("/x/feed/df_hh_1_{matchId}", "matchId", matchID)
					h2hRaw, _, h2hErr := resolveRead(cmd.Context(), c, flags, "match", false, h2hPath, map[string]string{}, nil)
					if h2hErr == nil {
						h2h := parser.ParseH2H([]byte(h2hRaw))
						// Last 3 scores
						last3 := make([]string, 0, 3)
						hw, aw, draws := 0, 0, 0
						for _, hm := range h2h {
							hmt, _ := hm["home_team"].(string)
							score, _ := hm["score"].(string)
							hs := toAnyInt(hm["home_score"])
							as := toAnyInt(hm["away_score"])
							if score == "" {
								score = fmt.Sprintf("%d-%d", hs, as)
							}
							if len(last3) < 3 {
								last3 = append(last3, score)
							}
							if hmt == ht {
								if hs > as {
									hw++
								} else if as > hs {
									aw++
								} else {
									draws++
								}
							} else {
								if as > hs {
									hw++
								} else if hs > as {
									aw++
								} else {
									draws++
								}
							}
						}
						if len(h2h) > 0 {
							total := hw + aw + draws
							if total > 0 {
								row.HomeWinPct = hw * 100 / total
								row.DrawPct = draws * 100 / total
								row.AwayWinPct = aw * 100 / total
							}
						}
						row.H2HLast3 = joinStrings(last3, " | ")
					}
				}
				rows = append(rows, row)
			}

			data, _ := json.Marshal(rows)
			jdata := json.RawMessage(data)

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printOutput(cmd.OutOrStdout(), jdata, true)
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
	cmd.Flags().StringVar(&flagDate, "date", "today", "Date: today, yesterday, or tomorrow")
	cmd.Flags().StringVar(&flagTournamentID, "tournament", "", "Filter by tournament ID (ZEE field from matches output)")
	return cmd
}

func joinStrings(ss []string, sep string) string {
	result := ""
	for i, s := range ss {
		if i > 0 {
			result += sep
		}
		result += s
	}
	return result
}
