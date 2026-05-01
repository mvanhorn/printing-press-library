package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/espn/internal/store"
	"github.com/spf13/cobra"
)

func newRivalsCmd(flags *rootFlags) *cobra.Command {
	var teams, dbPath string

	cmd := &cobra.Command{
		Use:   "rivals <sport> <league>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Head-to-head record between two teams from synced data",
		Example: `  espn-pp-cli rivals football nfl --teams KC,BUF
  espn-pp-cli rivals basketball nba --teams LAL,BOS --json
  espn-pp-cli rivals baseball mlb --teams NYY,BOS`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return cmd.Help()
			}
			if len(args) < 2 {
				return usageErr(fmt.Errorf("league is required\nUsage: rivals <sport> <league> --teams <A,B>"))
			}
			if teams == "" {
				return usageErr(fmt.Errorf("--teams is required (comma-separated abbreviations, e.g. KC,BUF)"))
			}

			parts := strings.SplitN(teams, ",", 2)
			if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
				return usageErr(fmt.Errorf("--teams must be two comma-separated abbreviations (e.g. KC,BUF)"))
			}
			teamA := strings.ToUpper(strings.TrimSpace(parts[0]))
			teamB := strings.ToUpper(strings.TrimSpace(parts[1]))

			sport, league := args[0], args[1]

			if dbPath == "" {
				dbPath = defaultDBPath("espn-pp-cli")
			}

			db, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w\nhint: run 'espn-pp-cli sync' first", err)
			}
			defer db.Close()

			// Get all completed events for teamA, then filter for matchups against teamB
			events, err := db.ListEvents(sport, league, teamA, 500, true)
			if err != nil {
				return fmt.Errorf("querying events: %w", err)
			}

			type matchup struct {
				Date     string `json:"date"`
				Score    string `json:"score"`
				Winner   string `json:"winner"`
				Venue    string `json:"venue"`
				HomeTeam string `json:"home_team"`
			}

			var matchups []matchup
			aWins, bWins, ties := 0, 0, 0

			for _, raw := range events {
				var ev map[string]any
				if json.Unmarshal(raw, &ev) != nil {
					continue
				}

				// Check if this game involves both teams
				if comps, ok := ev["competitions"].([]any); ok && len(comps) > 0 {
					comp, _ := comps[0].(map[string]any)
					if comp == nil {
						continue
					}

					var hasA, hasB bool
					var aScore, bScore string
					var aWinner, bWinner bool
					var homeTeam, venueName string

					if venue, ok := comp["venue"].(map[string]any); ok {
						venueName = jsonStrAny(venue, "fullName")
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

							if homeAway == "home" {
								homeTeam = abbr
							}

							if strings.EqualFold(abbr, teamA) {
								hasA = true
								aScore = score
								aWinner = winner
							} else if strings.EqualFold(abbr, teamB) {
								hasB = true
								bScore = score
								bWinner = winner
							}
						}
					}

					if !hasA || !hasB {
						continue
					}

					date := jsonStrAny(ev, "date")
					if len(date) > 10 {
						date = date[:10]
					}

					m := matchup{
						Date:     date,
						Score:    teamA + " " + aScore + " - " + teamB + " " + bScore,
						Venue:    venueName,
						HomeTeam: homeTeam,
					}

					if aWinner {
						m.Winner = teamA
						aWins++
					} else if bWinner {
						m.Winner = teamB
						bWins++
					} else {
						m.Winner = "TIE"
						ties++
					}

					matchups = append(matchups, m)
				}
			}

			if len(matchups) == 0 {
				return fmt.Errorf("no matchups found between %s and %s in %s/%s. Run 'espn-pp-cli sync' to populate data", teamA, teamB, sport, league)
			}

			// JSON output
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !humanFriendly) {
				out := map[string]any{
					"team_a":   teamA,
					"team_b":   teamB,
					"sport":    sport,
					"league":   league,
					"a_wins":   aWins,
					"b_wins":   bWins,
					"ties":     ties,
					"total":    len(matchups),
					"matchups": matchups,
				}
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(out)
			}

			// Table output
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "%s vs %s  (%s %s)\n", bold(teamA), bold(teamB), strings.ToUpper(sport), strings.ToUpper(league))
			fmt.Fprintf(w, "Record: %s %d - %d %s", teamA, aWins, bWins, teamB)
			if ties > 0 {
				fmt.Fprintf(w, " (%d ties)", ties)
			}
			fmt.Fprintf(w, "  (%d games)\n\n", len(matchups))

			tw := newTabWriter(w)
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n",
				bold("DATE"), bold("SCORE"), bold("WINNER"), bold("VENUE"))
			for _, m := range matchups {
				winner := m.Winner
				if winner == teamA {
					winner = green(winner)
				} else if winner == teamB {
					winner = red(winner)
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n",
					m.Date, m.Score, winner, truncate(m.Venue, 30))
			}
			return tw.Flush()
		},
	}

	cmd.Flags().StringVar(&teams, "teams", "", "Two team abbreviations, comma-separated (required, e.g. KC,BUF)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")

	return cmd
}
