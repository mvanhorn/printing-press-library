package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/espn/internal/store"
	"github.com/spf13/cobra"
)

// newH2hCmd returns deep head-to-head detail between two teams. Differs from
// `rivals` (league-wide pairwise records): h2h focuses on one specific pair
// with average score and recent meetings detail.
func newH2hCmd(flags *rootFlags) *cobra.Command {
	var sport, league, dbPath string

	cmd := &cobra.Command{
		Use:   "h2h <team1> <team2>",
		Short: "Head-to-head detail between two teams",
		Example: `  espn-pp-cli h2h KC BUF --sport football --league nfl
  espn-pp-cli h2h LAL BOS --sport basketball --league nba --agent
  espn-pp-cli h2h NYY BOS --sport baseball --league mlb --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return usageErr(fmt.Errorf("two team abbreviations are required\nUsage: h2h <team1> <team2> --sport <s> --league <l>"))
			}
			if sport == "" || league == "" {
				return usageErr(fmt.Errorf("--sport and --league are required"))
			}
			teamA := strings.ToUpper(args[0])
			teamB := strings.ToUpper(args[1])

			if dbPath == "" {
				dbPath = defaultDBPath("espn-pp-cli")
			}

			db, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w\nhint: run 'espn-pp-cli sync' first", err)
			}
			defer db.Close()

			events, err := db.ListEvents(sport, league, teamA, 1000, true)
			if err != nil {
				return fmt.Errorf("querying events: %w", err)
			}

			type meeting struct {
				Date   string `json:"date"`
				Score  string `json:"score"`
				Winner string `json:"winner"`
				ScoreA int    `json:"score_a"`
				ScoreB int    `json:"score_b"`
			}
			var meetings []meeting
			aWins, bWins, ties := 0, 0, 0
			var sumA, sumB int

			for _, raw := range events {
				var ev map[string]any
				if json.Unmarshal(raw, &ev) != nil {
					continue
				}
				comps, ok := ev["competitions"].([]any)
				if !ok || len(comps) == 0 {
					continue
				}
				comp, _ := comps[0].(map[string]any)
				if comp == nil {
					continue
				}
				competitors, ok := comp["competitors"].([]any)
				if !ok {
					continue
				}
				var hasA, hasB bool
				var scoreA, scoreB int
				var aWinner, bWinner bool
				for _, c := range competitors {
					t, _ := c.(map[string]any)
					if t == nil {
						continue
					}
					var abbr string
					if teamObj, ok := t["team"].(map[string]any); ok {
						abbr = jsonStrAny(teamObj, "abbreviation")
					}
					score := atoiSafe(jsonStrAny(t, "score"))
					winner := false
					if w, ok := t["winner"].(bool); ok {
						winner = w
					}
					if strings.EqualFold(abbr, teamA) {
						hasA = true
						scoreA = score
						aWinner = winner
					} else if strings.EqualFold(abbr, teamB) {
						hasB = true
						scoreB = score
						bWinner = winner
					}
				}
				if !hasA || !hasB {
					continue
				}

				date := jsonStrAny(ev, "date")
				if len(date) > 10 {
					date = date[:10]
				}
				m := meeting{
					Date:   date,
					Score:  fmt.Sprintf("%s %d - %s %d", teamA, scoreA, teamB, scoreB),
					ScoreA: scoreA,
					ScoreB: scoreB,
				}
				switch {
				case aWinner:
					m.Winner = teamA
					aWins++
				case bWinner:
					m.Winner = teamB
					bWins++
				default:
					m.Winner = "TIE"
					ties++
				}
				sumA += scoreA
				sumB += scoreB
				meetings = append(meetings, m)
			}

			if len(meetings) == 0 {
				return notFoundErr(fmt.Errorf("no meetings between %s and %s in synced %s/%s. Run 'espn-pp-cli sync' first", teamA, teamB, sport, league))
			}

			avgA := float64(sumA) / float64(len(meetings))
			avgB := float64(sumB) / float64(len(meetings))

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !humanFriendly) {
				out := map[string]any{
					"team_a":   teamA,
					"team_b":   teamB,
					"sport":    sport,
					"league":   league,
					"a_wins":   aWins,
					"b_wins":   bWins,
					"ties":     ties,
					"total":    len(meetings),
					"avg_a":    avgA,
					"avg_b":    avgB,
					"meetings": meetings,
				}
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(out)
			}

			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "%s vs %s  (%s %s)\n", bold(teamA), bold(teamB), strings.ToUpper(sport), strings.ToUpper(league))
			fmt.Fprintf(w, "Record: %s %d - %d %s", teamA, aWins, bWins, teamB)
			if ties > 0 {
				fmt.Fprintf(w, " (%d ties)", ties)
			}
			fmt.Fprintf(w, "  (%d games)\n", len(meetings))
			fmt.Fprintf(w, "Avg score: %s %.1f, %s %.1f\n\n", teamA, avgA, teamB, avgB)

			tw := newTabWriter(w)
			fmt.Fprintf(tw, "%s\t%s\t%s\n", bold("DATE"), bold("SCORE"), bold("WINNER"))
			for _, m := range meetings {
				fmt.Fprintf(tw, "%s\t%s\t%s\n", m.Date, m.Score, m.Winner)
			}
			return tw.Flush()
		},
	}

	cmd.Flags().StringVar(&sport, "sport", "", "Sport slug (required)")
	cmd.Flags().StringVar(&league, "league", "", "League slug (required)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func atoiSafe(s string) int {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return n
		}
		n = n*10 + int(r-'0')
	}
	return n
}
