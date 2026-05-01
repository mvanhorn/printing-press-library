package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newRecapCmd(flags *rootFlags) *cobra.Command {
	var eventID string

	cmd := &cobra.Command{
		Use:   "recap <sport> <league>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Game recap with box score and leaders",
		Example: `  espn-pp-cli recap football nfl --event 401547417
  espn-pp-cli recap basketball nba --event 401584793 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return cmd.Help()
			}
			if len(args) < 2 {
				return usageErr(fmt.Errorf("league is required\nUsage: recap <sport> <league> --event <id>"))
			}
			if eventID == "" {
				return usageErr(fmt.Errorf("--event is required"))
			}
			sport, league := args[0], args[1]

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := fmt.Sprintf("/%s/%s/summary", sport, league)
			params := map[string]string{"event": eventID}

			data, err := c.Get(path, params)
			if err != nil {
				return classifyAPIError(err)
			}

			// JSON output
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !humanFriendly) {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				var raw json.RawMessage
				if err := json.Unmarshal(data, &raw); err != nil {
					return err
				}
				return enc.Encode(raw)
			}

			return renderRecap(cmd, data)
		},
	}

	cmd.Flags().StringVar(&eventID, "event", "", "ESPN event/game ID (required)")

	return cmd
}

func renderRecap(cmd *cobra.Command, data json.RawMessage) error {
	w := cmd.OutOrStdout()

	var summary map[string]json.RawMessage
	if err := json.Unmarshal(data, &summary); err != nil {
		return fmt.Errorf("parsing summary: %w", err)
	}

	// Header - game info
	if headerRaw, ok := summary["header"]; ok {
		var header struct {
			GameNote string `json:"gameNote"`
			Season   struct {
				Year int `json:"year"`
			} `json:"season"`
			Competitions []struct {
				Date        string `json:"date"`
				NeutralSite bool   `json:"neutralSite"`
				Competitors []struct {
					HomeAway string `json:"homeAway"`
					Winner   bool   `json:"winner"`
					Score    string `json:"score"`
					Team     struct {
						Abbreviation string `json:"abbreviation"`
						DisplayName  string `json:"displayName"`
					} `json:"team"`
					Linescores []struct {
						DisplayValue string `json:"displayValue"`
					} `json:"linescores"`
				} `json:"competitors"`
				Status struct {
					Type struct {
						Detail    string `json:"detail"`
						Completed bool   `json:"completed"`
					} `json:"type"`
				} `json:"status"`
			} `json:"competitions"`
		}
		if err := json.Unmarshal(headerRaw, &header); err == nil && len(header.Competitions) > 0 {
			comp := header.Competitions[0]
			fmt.Fprintf(w, "%s\n", bold(comp.Status.Type.Detail))
			if header.GameNote != "" {
				fmt.Fprintf(w, "%s\n", header.GameNote)
			}
			fmt.Fprintln(w)

			// Score header
			for _, c := range comp.Competitors {
				marker := "  "
				if c.Winner {
					marker = "* "
				}
				var periods []string
				for _, ls := range c.Linescores {
					periods = append(periods, ls.DisplayValue)
				}
				periodStr := ""
				if len(periods) > 0 {
					periodStr = "  (" + strings.Join(periods, " | ") + ")"
				}
				fmt.Fprintf(w, "%s%s %s  %s%s\n", marker, c.Team.Abbreviation, c.Team.DisplayName, c.Score, periodStr)
			}
			fmt.Fprintln(w)
		}
	}

	// Box score
	if boxRaw, ok := summary["boxscore"]; ok {
		var box struct {
			Players []struct {
				Team struct {
					Abbreviation string `json:"abbreviation"`
				} `json:"team"`
				Statistics []struct {
					Name     string   `json:"name"`
					Labels   []string `json:"labels"`
					Athletes []struct {
						Athlete struct {
							DisplayName string `json:"displayName"`
						} `json:"athlete"`
						Stats []string `json:"stats"`
					} `json:"athletes"`
				} `json:"statistics"`
			} `json:"players"`
		}
		if err := json.Unmarshal(boxRaw, &box); err == nil && len(box.Players) > 0 {
			fmt.Fprintf(w, "%s\n", bold("BOX SCORE"))
			for _, team := range box.Players {
				if len(team.Statistics) > 0 {
					stat := team.Statistics[0]
					fmt.Fprintf(w, "\n  %s — %s\n", bold(team.Team.Abbreviation), stat.Name)
					if len(stat.Athletes) > 0 && len(stat.Labels) > 0 {
						tw := newTabWriter(w)
						// Header: PLAYER + stat labels (max 6)
						labels := stat.Labels
						if len(labels) > 6 {
							labels = labels[:6]
						}
						header := "  " + bold("PLAYER")
						for _, l := range labels {
							header += "\t" + bold(l)
						}
						fmt.Fprintln(tw, header)
						// Rows
						maxRows := 5
						for i, ath := range stat.Athletes {
							if i >= maxRows {
								break
							}
							row := "  " + truncate(ath.Athlete.DisplayName, 20)
							for j, s := range ath.Stats {
								if j >= len(labels) {
									break
								}
								row += "\t" + s
							}
							fmt.Fprintln(tw, row)
						}
						tw.Flush()
					}
				}
			}
			fmt.Fprintln(w)
		}
	}

	// Leaders
	if leadersRaw, ok := summary["leaders"]; ok {
		var leaders []struct {
			Name    string `json:"name"`
			Leaders []struct {
				DisplayName string `json:"displayName"`
				Leaders     []struct {
					Athlete struct {
						DisplayName string `json:"displayName"`
						Team        struct {
							Abbreviation string `json:"abbreviation"`
						} `json:"team"`
					} `json:"athlete"`
					DisplayValue string `json:"displayValue"`
				} `json:"leaders"`
			} `json:"leaders"`
		}
		if err := json.Unmarshal(leadersRaw, &leaders); err == nil && len(leaders) > 0 {
			fmt.Fprintf(w, "%s\n", bold("LEADERS"))
			for _, cat := range leaders {
				for _, sub := range cat.Leaders {
					if len(sub.Leaders) > 0 {
						top := sub.Leaders[0]
						fmt.Fprintf(w, "  %-20s %s (%s) — %s\n",
							sub.DisplayName,
							top.Athlete.DisplayName,
							top.Athlete.Team.Abbreviation,
							top.DisplayValue)
					}
				}
			}
		}
	}

	return nil
}
