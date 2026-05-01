package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"
)

// dashboardConfig is the on-disk shape for the [favorites] section of
// ~/.config/espn-pp-cli/config.toml.
//
// Schema:
//
//	[favorites]
//	nfl   = ["KC", "BAL"]
//	nba   = ["LAL"]
//
// Keys are league slugs (lowercase). Values are team abbreviations (uppercase).
// Unknown keys are preserved for forward compatibility.
type dashboardConfig struct {
	Favorites map[string][]string `toml:"favorites"`
}

func loadDashboardFavorites(path string) (map[string][]string, string, error) {
	if path == "" {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, ".config", "espn-pp-cli", "config.toml")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, path, nil
		}
		return nil, path, err
	}
	var cfg dashboardConfig
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, path, fmt.Errorf("parsing %s: %w", path, err)
	}
	// Normalize keys/values.
	out := map[string][]string{}
	for league, teams := range cfg.Favorites {
		key := strings.ToLower(league)
		var ts []string
		for _, t := range teams {
			ts = append(ts, strings.ToUpper(t))
		}
		out[key] = ts
	}
	return out, path, nil
}

// newDashboardCmd renders favorited teams' status grouped by league.
func newDashboardCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dashboard",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Your favorite teams' status at a glance (reads [favorites] in config.toml)",
		Example: `  espn-pp-cli dashboard
  espn-pp-cli dashboard --agent
  espn-pp-cli dashboard --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			favs, path, err := loadDashboardFavorites(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			if len(favs) == 0 {
				w := cmd.OutOrStdout()
				if flags.asJSON || (!isTerminal(w) && !humanFriendly) {
					fmt.Fprintln(w, "[]")
					fmt.Fprintf(cmd.ErrOrStderr(), "No favorites configured. Add a [favorites] block to %s\n", path)
					return nil
				}
				fmt.Fprintf(w, "No favorites configured. Add a [favorites] block to %s, e.g.:\n\n", path)
				fmt.Fprintln(w, "  [favorites]")
				fmt.Fprintln(w, `  nfl = ["KC", "BAL"]`)
				fmt.Fprintln(w, `  nba = ["LAL"]`)
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			type leagueRow struct {
				League string         `json:"league"`
				Teams  []dashboardRow `json:"teams"`
				Err    string         `json:"error,omitempty"`
			}

			leagues := make([]string, 0, len(favs))
			for k := range favs {
				leagues = append(leagues, k)
			}
			sort.Strings(leagues)

			results := make([]leagueRow, len(leagues))
			var wg sync.WaitGroup
			for i, league := range leagues {
				wg.Add(1)
				go func(idx int, league string) {
					defer wg.Done()
					sport := sportForLeague(league)
					if sport == "" {
						results[idx] = leagueRow{League: league, Err: "unknown league slug"}
						return
					}
					path := fmt.Sprintf("/%s/%s/scoreboard", sport, league)
					data, fetchErr := c.Get(path, nil)
					if fetchErr != nil {
						results[idx] = leagueRow{League: league, Err: fetchErr.Error()}
						return
					}
					rows := dashboardRowsFor(data, favs[league])
					results[idx] = leagueRow{League: league, Teams: rows}
				}(i, league)
			}
			wg.Wait()

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !humanFriendly) {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(results)
			}

			w := cmd.OutOrStdout()
			for _, r := range results {
				fmt.Fprintf(w, "\n%s\n", bold(strings.ToUpper(r.League)))
				if r.Err != "" {
					fmt.Fprintf(w, "  error: %s\n", r.Err)
					continue
				}
				if len(r.Teams) == 0 {
					fmt.Fprintln(w, "  (no games for favorited teams in current scoreboard)")
					continue
				}
				tw := newTabWriter(w)
				fmt.Fprintf(tw, "  %s\t%s\t%s\t%s\n",
					bold("TEAM"), bold("MATCHUP"), bold("SCORE"), bold("STATUS"))
				for _, row := range r.Teams {
					fmt.Fprintf(tw, "  %s\t%s\t%s\t%s\n",
						row.Team, row.Matchup, row.Score, row.Status)
				}
				tw.Flush()
			}
			return nil
		},
	}
	return cmd
}

type dashboardRow struct {
	Team    string `json:"team"`
	Matchup string `json:"matchup"`
	Score   string `json:"score"`
	Status  string `json:"status"`
}

func dashboardRowsFor(data json.RawMessage, favorites []string) []dashboardRow {
	if len(favorites) == 0 {
		return nil
	}
	favSet := map[string]bool{}
	for _, t := range favorites {
		favSet[strings.ToUpper(t)] = true
	}
	events := parseScoreEvents(data)
	var rows []dashboardRow
	for _, e := range events {
		if !favSet[strings.ToUpper(e.HomeTeam)] && !favSet[strings.ToUpper(e.AwayTeam)] {
			continue
		}
		team := e.HomeTeam
		if !favSet[strings.ToUpper(e.HomeTeam)] {
			team = e.AwayTeam
		}
		rows = append(rows, dashboardRow{
			Team:    team,
			Matchup: e.Matchup,
			Score:   fmt.Sprintf("%s %s - %s %s", e.AwayTeam, e.AwayScore, e.HomeTeam, e.HomeScore),
			Status:  firstNonEmpty(e.Detail, e.Status),
		})
	}
	return rows
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
