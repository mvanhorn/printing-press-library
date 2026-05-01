package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

func newWatchCmd(flags *rootFlags) *cobra.Command {
	var eventID string
	var interval time.Duration

	cmd := &cobra.Command{
		Use:   "watch <sport> <league>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Live score updates for a game (polls every 30s)",
		Example: `  espn-pp-cli watch football nfl --event 401547417
  espn-pp-cli watch basketball nba --event 401584793 --interval 15s`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return cmd.Help()
			}
			if len(args) < 2 {
				return usageErr(fmt.Errorf("league is required\nUsage: watch <sport> <league> --event <id>"))
			}
			if eventID == "" {
				return usageErr(fmt.Errorf("--event is required"))
			}
			sport, league := args[0], args[1]

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := fmt.Sprintf("/%s/%s/scoreboard", sport, league)
			w := cmd.OutOrStdout()

			fmt.Fprintf(w, "Watching event %s (%s/%s) — polling every %s. Press Ctrl+C to stop.\n\n",
				eventID, sport, league, interval)

			var lastScore string
			for {
				data, err := c.Get(path, nil)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "[%s] error: %v\n", time.Now().Format("15:04:05"), err)
					time.Sleep(interval)
					continue
				}

				event := findEventByID(data, eventID)
				if event == nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "[%s] event %s not found in scoreboard\n", time.Now().Format("15:04:05"), eventID)
					time.Sleep(interval)
					continue
				}

				ev := parseOneScoreEvent(event)
				if ev == nil {
					time.Sleep(interval)
					continue
				}

				currentScore := fmt.Sprintf("%s %s - %s %s", ev.AwayTeam, ev.AwayScore, ev.HomeTeam, ev.HomeScore)

				// JSON mode: emit every poll
				if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !humanFriendly) {
					out := map[string]any{
						"timestamp":  time.Now().UTC().Format(time.RFC3339),
						"event_id":   ev.ID,
						"away_team":  ev.AwayTeam,
						"away_score": ev.AwayScore,
						"home_team":  ev.HomeTeam,
						"home_score": ev.HomeScore,
						"status":     ev.Status,
						"detail":     ev.Detail,
					}
					enc := json.NewEncoder(w)
					enc.Encode(out)
				} else {
					// Only print if score changed or first poll
					if currentScore != lastScore {
						fmt.Fprintf(w, "[%s] %s  %s\n",
							time.Now().Format("15:04:05"),
							currentScore,
							ev.Detail)
						lastScore = currentScore
					}
				}

				// If game is finished, exit
				if ev.Status == "post" {
					fmt.Fprintf(w, "\nFinal: %s\n", currentScore)
					return nil
				}

				time.Sleep(interval)
			}
		},
	}

	cmd.Flags().StringVar(&eventID, "event", "", "ESPN event/game ID (required)")
	cmd.Flags().DurationVar(&interval, "interval", 30*time.Second, "Poll interval (e.g. 15s, 1m)")

	return cmd
}

func findEventByID(data json.RawMessage, eventID string) json.RawMessage {
	var resp struct {
		Events []json.RawMessage `json:"events"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil
	}
	for _, raw := range resp.Events {
		var ev struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(raw, &ev) == nil && ev.ID == eventID {
			return raw
		}
	}
	return nil
}
