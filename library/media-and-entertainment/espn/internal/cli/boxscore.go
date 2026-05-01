package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// newBoxscoreCmd returns the box-score-only view of a game summary.
// Composes the existing summary endpoint and returns just the boxscore subtree,
// avoiding the full payload when the user only wants per-player stats.
//
// Sport+league inference: when --sport/--league are unset we look in the
// per-event scores cache directory for a recent scoreboard hit that contains
// this event id. On miss the user gets a friendly hint to provide flags.
func newBoxscoreCmd(flags *rootFlags) *cobra.Command {
	var sport, league string

	cmd := &cobra.Command{
		Use:   "boxscore <event_id>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Full box score for a specific event id",
		Example: `  espn-pp-cli boxscore 401547417 --sport football --league nfl
  espn-pp-cli boxscore 401584793 --sport basketball --league nba --agent
  espn-pp-cli boxscore 401569551`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return usageErr(fmt.Errorf("event id is required\nUsage: boxscore <event_id> [--sport <s> --league <l>]"))
			}
			eventID := args[0]

			// Try cache inference when sport/league unset.
			if sport == "" || league == "" {
				inferredSport, inferredLeague, ok := inferSportLeagueFromCache(eventID)
				if ok {
					if sport == "" {
						sport = inferredSport
					}
					if league == "" {
						league = inferredLeague
					}
				}
			}
			if sport == "" || league == "" {
				return usageErr(fmt.Errorf("could not infer sport/league for event %s. Pass --sport and --league explicitly (e.g. --sport basketball --league nba)", eventID))
			}

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

			// Extract just the boxscore subtree from the summary payload.
			var summary map[string]json.RawMessage
			if err := json.Unmarshal(data, &summary); err != nil {
				return fmt.Errorf("parsing summary: %w", err)
			}
			boxRaw, ok := summary["boxscore"]
			if !ok {
				return notFoundErr(fmt.Errorf("event %s has no boxscore (game may not have started)", eventID))
			}

			// JSON output
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !humanFriendly) {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(boxRaw)
			}

			return renderBoxscore(cmd, boxRaw)
		},
	}

	cmd.Flags().StringVar(&sport, "sport", "", "Sport slug (inferred from cache if omitted)")
	cmd.Flags().StringVar(&league, "league", "", "League slug (inferred from cache if omitted)")
	return cmd
}

// inferSportLeagueFromCache scans the espn-pp-cli cache directory for a recent
// scoreboard response that contains the given event id, and returns the
// sport+league that produced that scoreboard. Best-effort — returns false on
// any miss.
func inferSportLeagueFromCache(eventID string) (sport, league string, ok bool) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", false
	}
	cacheDir := filepath.Join(home, ".cache", "espn-pp-cli")
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return "", "", false
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(cacheDir, entry.Name()))
		if err != nil {
			continue
		}
		// Quick string search for the event id before parsing.
		if !strings.Contains(string(data), `"id":"`+eventID+`"`) {
			continue
		}
		// Parse to confirm it is a scoreboard payload that lists this event.
		var sb struct {
			Leagues []struct {
				Slug string `json:"slug"`
			} `json:"leagues"`
			Events []struct {
				ID string `json:"id"`
			} `json:"events"`
		}
		if err := json.Unmarshal(data, &sb); err != nil {
			continue
		}
		hasEvent := false
		for _, ev := range sb.Events {
			if ev.ID == eventID {
				hasEvent = true
				break
			}
		}
		if !hasEvent || len(sb.Leagues) == 0 {
			continue
		}
		// leagues[0].slug is something like "nfl" or "nba"; we need to
		// derive the sport. We rely on a small slug→sport map.
		leagueSlug := sb.Leagues[0].Slug
		if leagueSlug == "" {
			continue
		}
		sport = sportForLeague(leagueSlug)
		if sport == "" {
			continue
		}
		return sport, leagueSlug, true
	}
	return "", "", false
}

// sportForLeague returns the canonical sport slug for a league slug, or empty
// if unknown.
func sportForLeague(leagueSlug string) string {
	switch strings.ToLower(leagueSlug) {
	case "nfl", "college-football", "ncaaf":
		return "football"
	case "nba", "wnba", "mens-college-basketball", "womens-college-basketball", "ncaam", "ncaaw":
		return "basketball"
	case "mlb":
		return "baseball"
	case "nhl":
		return "hockey"
	case "mls", "eng.1":
		return "soccer"
	}
	return ""
}

func renderBoxscore(cmd *cobra.Command, boxRaw json.RawMessage) error {
	w := cmd.OutOrStdout()
	var box struct {
		Players []struct {
			Team struct {
				Abbreviation string `json:"abbreviation"`
				DisplayName  string `json:"displayName"`
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
		Teams []struct {
			Team struct {
				Abbreviation string `json:"abbreviation"`
			} `json:"team"`
			Statistics []struct {
				Label        string `json:"label"`
				DisplayValue string `json:"displayValue"`
			} `json:"statistics"`
		} `json:"teams"`
	}
	if err := json.Unmarshal(boxRaw, &box); err != nil {
		return fmt.Errorf("parsing boxscore: %w", err)
	}

	if len(box.Players) == 0 && len(box.Teams) == 0 {
		fmt.Fprintln(w, "Boxscore is empty (game may not have started).")
		return nil
	}

	for _, team := range box.Players {
		for _, stat := range team.Statistics {
			if len(stat.Athletes) == 0 || len(stat.Labels) == 0 {
				continue
			}
			fmt.Fprintf(w, "\n%s %s — %s\n", bold(team.Team.Abbreviation), team.Team.DisplayName, stat.Name)
			tw := newTabWriter(w)
			labels := stat.Labels
			if len(labels) > 6 {
				labels = labels[:6]
			}
			header := bold("PLAYER")
			for _, l := range labels {
				header += "\t" + bold(l)
			}
			fmt.Fprintln(tw, header)
			for _, ath := range stat.Athletes {
				row := truncate(ath.Athlete.DisplayName, 22)
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

	if len(box.Teams) > 0 {
		fmt.Fprintf(w, "\n%s\n", bold("TEAM TOTALS"))
		for _, team := range box.Teams {
			fmt.Fprintf(w, "  %s:\n", bold(team.Team.Abbreviation))
			tw := newTabWriter(w)
			for _, s := range team.Statistics {
				fmt.Fprintf(tw, "    %s\t%s\n", s.Label, s.DisplayValue)
			}
			tw.Flush()
		}
	}

	return nil
}
