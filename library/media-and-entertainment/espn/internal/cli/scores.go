package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

// scoreEvent is a parsed score line from an ESPN scoreboard event.
type scoreEvent struct {
	ID        string `json:"id"`
	Matchup   string `json:"matchup"`
	AwayTeam  string `json:"away_team"`
	AwayScore string `json:"away_score"`
	HomeTeam  string `json:"home_team"`
	HomeScore string `json:"home_score"`
	Status    string `json:"status"`
	Detail    string `json:"detail"`
}

func newScoresCmd(flags *rootFlags) *cobra.Command {
	var dates string
	var limit int

	cmd := &cobra.Command{
		Use:   "scores <sport> <league>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Live scores and results for a sport and league",
		Example: `  espn-pp-cli scores football nfl
  espn-pp-cli scores basketball nba --dates 20250115
  espn-pp-cli scores baseball mlb --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return cmd.Help()
			}
			if len(args) < 2 {
				return usageErr(fmt.Errorf("league is required\nUsage: scores <sport> <league>"))
			}
			sport, league := args[0], args[1]

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := fmt.Sprintf("/%s/%s/scoreboard", sport, league)
			params := map[string]string{}
			if dates != "" {
				params["dates"] = dates
			}
			if limit > 0 {
				params["limit"] = fmt.Sprintf("%d", limit)
			}

			data, err := c.Get(path, params)
			if err != nil {
				return classifyAPIError(err)
			}

			events := parseScoreEvents(data)

			// JSON output when piped or --json
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !humanFriendly) {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(events)
			}

			// Table output
			if len(events) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No games found.")
				return nil
			}

			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
				bold("MATCHUP"), bold("AWAY"), bold("HOME"), bold("STATUS"), bold("DETAIL"))
			for _, e := range events {
				away := fmt.Sprintf("%s %s", e.AwayTeam, e.AwayScore)
				home := fmt.Sprintf("%s %s", e.HomeTeam, e.HomeScore)
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
					e.Matchup, away, home, e.Status, e.Detail)
			}
			return tw.Flush()
		},
	}

	cmd.Flags().StringVar(&dates, "dates", "", "Date filter (YYYYMMDD or YYYYMMDD-YYYYMMDD)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Max events to return")

	return cmd
}

// parseScoreEvents extracts score events from an ESPN scoreboard response.
func parseScoreEvents(data json.RawMessage) []scoreEvent {
	var resp struct {
		Events []json.RawMessage `json:"events"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil
	}

	var events []scoreEvent
	for _, raw := range resp.Events {
		ev := parseOneScoreEvent(raw)
		if ev != nil {
			events = append(events, *ev)
		}
	}
	return events
}

func parseOneScoreEvent(raw json.RawMessage) *scoreEvent {
	var ev map[string]any
	if err := json.Unmarshal(raw, &ev); err != nil {
		return nil
	}

	e := &scoreEvent{
		ID:      jsonStrAny(ev, "id"),
		Matchup: jsonStrAny(ev, "shortName"),
	}

	// Status
	if statusObj, ok := ev["status"].(map[string]any); ok {
		if typeObj, ok := statusObj["type"].(map[string]any); ok {
			e.Status = jsonStrAny(typeObj, "state")
			e.Detail = jsonStrAny(typeObj, "detail")
			if e.Detail == "" {
				e.Detail = jsonStrAny(typeObj, "shortDetail")
			}
		}
	}

	// Competitors
	if comps, ok := ev["competitions"].([]any); ok && len(comps) > 0 {
		comp, _ := comps[0].(map[string]any)
		if comp != nil {
			if competitors, ok := comp["competitors"].([]any); ok {
				for _, c := range competitors {
					team, _ := c.(map[string]any)
					if team == nil {
						continue
					}
					homeAway := jsonStrAny(team, "homeAway")
					score := jsonStrAny(team, "score")
					var abbr string
					if t, ok := team["team"].(map[string]any); ok {
						abbr = jsonStrAny(t, "abbreviation")
					}
					if homeAway == "home" {
						e.HomeTeam = abbr
						e.HomeScore = score
					} else {
						e.AwayTeam = abbr
						e.AwayScore = score
					}
				}
			}
		}
	}

	if e.Matchup == "" {
		e.Matchup = e.AwayTeam + " @ " + e.HomeTeam
	}

	return e
}

func jsonStrAny(obj map[string]any, key string) string {
	if v, ok := obj[key]; ok {
		switch s := v.(type) {
		case string:
			return s
		default:
			return fmt.Sprintf("%v", s)
		}
	}
	return ""
}
