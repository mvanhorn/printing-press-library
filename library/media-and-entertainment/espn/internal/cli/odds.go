package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

// newOddsCmd aggregates spread/total/moneyline lines for tonight's slate by
// reading each scoreboard event's competitions[].odds field. This avoids one
// summary call per game.
func newOddsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "odds <sport> <league>",
		Short: "Spread, total, and moneyline lines for tonight's slate",
		Example: `  espn-pp-cli odds basketball nba
  espn-pp-cli odds football nfl --agent
  espn-pp-cli odds baseball mlb --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return cmd.Help()
			}
			if len(args) < 2 {
				return usageErr(fmt.Errorf("league is required\nUsage: odds <sport> <league>"))
			}
			sport, league := args[0], args[1]

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := fmt.Sprintf("/%s/%s/scoreboard", sport, league)
			data, err := c.Get(path, nil)
			if err != nil {
				return classifyAPIError(err)
			}

			lines := extractOdds(data)

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !humanFriendly) {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(lines)
			}

			w := cmd.OutOrStdout()
			if len(lines) == 0 {
				fmt.Fprintln(w, "No games with odds for the current slate.")
				return nil
			}
			tw := newTabWriter(w)
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
				bold("MATCHUP"), bold("SPREAD"), bold("OVER/UNDER"), bold("ML(AWAY)"), bold("ML(HOME)"))
			for _, l := range lines {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
					l.Matchup, l.Spread, l.OverUnder, l.AwayMoneyline, l.HomeMoneyline)
			}
			return tw.Flush()
		},
	}
	return cmd
}

type oddsLine struct {
	EventID       string `json:"event_id"`
	Matchup       string `json:"matchup"`
	Spread        string `json:"spread"`
	OverUnder     string `json:"over_under"`
	AwayMoneyline string `json:"away_moneyline"`
	HomeMoneyline string `json:"home_moneyline"`
}

func extractOdds(data json.RawMessage) []oddsLine {
	var resp struct {
		Events []struct {
			ID           string `json:"id"`
			ShortName    string `json:"shortName"`
			Competitions []struct {
				Odds []struct {
					Provider struct {
						Name string `json:"name"`
					} `json:"provider"`
					Details      string  `json:"details"`
					OverUnder    float64 `json:"overUnder"`
					AwayTeamOdds struct {
						MoneyLine int `json:"moneyLine"`
					} `json:"awayTeamOdds"`
					HomeTeamOdds struct {
						MoneyLine int `json:"moneyLine"`
					} `json:"homeTeamOdds"`
				} `json:"odds"`
			} `json:"competitions"`
		} `json:"events"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil
	}
	var out []oddsLine
	for _, ev := range resp.Events {
		if len(ev.Competitions) == 0 || len(ev.Competitions[0].Odds) == 0 {
			continue
		}
		o := ev.Competitions[0].Odds[0]
		ou := ""
		if o.OverUnder != 0 {
			ou = fmt.Sprintf("%.1f", o.OverUnder)
		}
		out = append(out, oddsLine{
			EventID:       ev.ID,
			Matchup:       ev.ShortName,
			Spread:        o.Details,
			OverUnder:     ou,
			AwayMoneyline: formatMoneyline(o.AwayTeamOdds.MoneyLine),
			HomeMoneyline: formatMoneyline(o.HomeTeamOdds.MoneyLine),
		})
	}
	return out
}

func formatMoneyline(ml int) string {
	if ml == 0 {
		return ""
	}
	if ml > 0 {
		return fmt.Sprintf("+%d", ml)
	}
	return fmt.Sprintf("%d", ml)
}
