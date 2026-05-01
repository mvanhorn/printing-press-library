package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/espn/internal/store"
	"github.com/spf13/cobra"
)

func newStreakCmd(flags *rootFlags) *cobra.Command {
	var team, dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:   "streak <sport> <league>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Current win/loss streak for a team from synced data",
		Example: `  espn-pp-cli streak football nfl --team KC
  espn-pp-cli streak basketball nba --team LAL --limit 30
  espn-pp-cli streak baseball mlb --team NYY --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return cmd.Help()
			}
			if len(args) < 2 {
				return usageErr(fmt.Errorf("league is required\nUsage: streak <sport> <league> --team <abbr>"))
			}
			if team == "" {
				return usageErr(fmt.Errorf("--team is required (team abbreviation, e.g. KC, LAL, NYY)"))
			}
			sport, league := args[0], args[1]
			teamAbbr := strings.ToUpper(team)

			if dbPath == "" {
				dbPath = defaultDBPath("espn-pp-cli")
			}

			db, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w\nhint: run 'espn-pp-cli sync' first", err)
			}
			defer db.Close()

			events, err := db.ListEvents(sport, league, teamAbbr, limit, true)
			if err != nil {
				return fmt.Errorf("querying events: %w", err)
			}

			if len(events) == 0 {
				return fmt.Errorf("no completed games found for %s in %s/%s. Run 'espn-pp-cli sync' first", teamAbbr, sport, league)
			}

			// Parse games and compute streak
			type gameResult struct {
				Date     string `json:"date"`
				Opponent string `json:"opponent"`
				Score    string `json:"score"`
				Result   string `json:"result"` // W or L
				HomeAway string `json:"home_away"`
			}

			var games []gameResult
			wins, losses := 0, 0
			var streakChar string
			var streakCount int

			for _, raw := range events {
				var ev map[string]any
				if json.Unmarshal(raw, &ev) != nil {
					continue
				}

				date := jsonStrAny(ev, "date")
				if len(date) > 10 {
					date = date[:10]
				}

				gr := gameResult{Date: date}

				if comps, ok := ev["competitions"].([]any); ok && len(comps) > 0 {
					comp, _ := comps[0].(map[string]any)
					if comp == nil {
						continue
					}
					if competitors, ok := comp["competitors"].([]any); ok {
						for _, c := range competitors {
							t, _ := c.(map[string]any)
							if t == nil {
								continue
							}
							var abbr string
							if teamObj, ok := t["team"].(map[string]any); ok {
								abbr = jsonStrAny(teamObj, "abbreviation")
							}
							homeAway := jsonStrAny(t, "homeAway")
							score := jsonStrAny(t, "score")
							winner := false
							if w, ok := t["winner"].(bool); ok {
								winner = w
							}

							if strings.EqualFold(abbr, teamAbbr) {
								gr.HomeAway = homeAway
								gr.Score = score
								if winner {
									gr.Result = "W"
								} else {
									gr.Result = "L"
								}
							} else {
								gr.Opponent = abbr
								if gr.Score != "" {
									gr.Score = gr.Score + "-" + score
								} else {
									// Will be set after we process the team
									gr.Opponent = abbr
								}
							}
						}
						// Fix score format: "teamScore-oppScore"
						var teamScore, oppScore string
						for _, c := range competitors {
							t, _ := c.(map[string]any)
							if t == nil {
								continue
							}
							var abbr string
							if teamObj, ok := t["team"].(map[string]any); ok {
								abbr = jsonStrAny(teamObj, "abbreviation")
							}
							if strings.EqualFold(abbr, teamAbbr) {
								teamScore = jsonStrAny(t, "score")
							} else {
								oppScore = jsonStrAny(t, "score")
							}
						}
						gr.Score = teamScore + "-" + oppScore
					}
				}

				if gr.Result == "W" {
					wins++
				} else if gr.Result == "L" {
					losses++
				}
				games = append(games, gr)
			}

			// Compute current streak from most recent games
			if len(games) > 0 {
				streakChar = games[0].Result
				streakCount = 0
				for _, g := range games {
					if g.Result == streakChar {
						streakCount++
					} else {
						break
					}
				}
			}

			streakStr := fmt.Sprintf("%s%d", streakChar, streakCount)

			// JSON output
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !humanFriendly) {
				out := map[string]any{
					"team":   teamAbbr,
					"sport":  sport,
					"league": league,
					"streak": streakStr,
					"record": fmt.Sprintf("%d-%d", wins, losses),
					"wins":   wins,
					"losses": losses,
					"games":  games,
				}
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(out)
			}

			// Table output
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "%s  %s  Record: %d-%d  Streak: %s\n\n",
				bold(teamAbbr), strings.ToUpper(league), wins, losses, bold(streakStr))

			tw := newTabWriter(w)
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
				bold("DATE"), bold("OPP"), bold("H/A"), bold("SCORE"), bold("RESULT"))
			for _, g := range games {
				result := g.Result
				if result == "W" {
					result = green("W")
				} else if result == "L" {
					result = red("L")
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
					g.Date, g.Opponent, g.HomeAway, g.Score, result)
			}
			return tw.Flush()
		},
	}

	cmd.Flags().StringVar(&team, "team", "", "Team abbreviation (required, e.g. KC, LAL, NYY)")
	cmd.Flags().IntVar(&limit, "limit", 20, "Number of recent games to analyze")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")

	return cmd
}
