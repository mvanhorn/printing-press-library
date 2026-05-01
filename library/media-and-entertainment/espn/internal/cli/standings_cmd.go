package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newStandingsCmd(flags *rootFlags) *cobra.Command {
	var season int

	cmd := &cobra.Command{
		Use:   "standings <sport> <league>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Conference/division standings for a sport and league",
		Example: `  espn-pp-cli standings football nfl
  espn-pp-cli standings basketball nba --season 2024
  espn-pp-cli standings hockey nhl --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return cmd.Help()
			}
			if len(args) < 2 {
				return usageErr(fmt.Errorf("league is required\nUsage: standings <sport> <league>"))
			}
			sport, league := args[0], args[1]

			// Standings uses a different base URL than the main ESPN API
			standingsURL := fmt.Sprintf("https://site.web.api.espn.com/apis/v2/sports/%s/%s/standings", sport, league)
			if season > 0 {
				standingsURL += fmt.Sprintf("?season=%d", season)
			}

			httpClient := &http.Client{Timeout: flags.timeout}
			if flags.timeout == 0 {
				httpClient.Timeout = 30 * time.Second
			}

			resp, err := httpClient.Get(standingsURL)
			if err != nil {
				return apiErr(fmt.Errorf("fetching standings: %w", err))
			}
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return apiErr(fmt.Errorf("reading standings response: %w", err))
			}

			if resp.StatusCode >= 400 {
				return apiErr(fmt.Errorf("standings API returned HTTP %d: %s", resp.StatusCode, truncate(string(body), 200)))
			}

			// JSON output
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !humanFriendly) {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				var raw json.RawMessage
				if err := json.Unmarshal(body, &raw); err != nil {
					return err
				}
				return enc.Encode(raw)
			}

			// Parse and display standings
			return renderStandings(cmd.OutOrStdout(), body)
		},
	}

	cmd.Flags().IntVar(&season, "season", 0, "Season year (e.g. 2025)")

	return cmd
}

type standingsEntry struct {
	Team   string
	Abbr   string
	Wins   string
	Losses string
	Ties   string
	Pct    string
	GB     string
	Diff   string
	Streak string
	PF     string
	PA     string
}

func renderStandings(w io.Writer, data []byte) error {
	var resp struct {
		Children []struct {
			Name     string `json:"name"`
			Children []struct {
				Name      string `json:"name"`
				Standings struct {
					Entries []struct {
						Team struct {
							DisplayName  string `json:"displayName"`
							Abbreviation string `json:"abbreviation"`
						} `json:"team"`
						Stats []struct {
							Name         string  `json:"name"`
							DisplayValue string  `json:"displayValue"`
							Value        float64 `json:"value"`
						} `json:"stats"`
					} `json:"entries"`
				} `json:"standings"`
			} `json:"children"`
			Standings struct {
				Entries []struct {
					Team struct {
						DisplayName  string `json:"displayName"`
						Abbreviation string `json:"abbreviation"`
					} `json:"team"`
					Stats []struct {
						Name         string  `json:"name"`
						DisplayValue string  `json:"displayValue"`
						Value        float64 `json:"value"`
					} `json:"stats"`
				} `json:"entries"`
			} `json:"standings"`
		} `json:"children"`
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return fmt.Errorf("parsing standings: %w", err)
	}

	for _, conf := range resp.Children {
		// Conference may have divisions (children) or direct standings
		if len(conf.Children) > 0 {
			fmt.Fprintf(w, "\n%s\n", bold(strings.ToUpper(conf.Name)))
			for _, div := range conf.Children {
				entries := parseStandingsEntries(div.Standings.Entries)
				if len(entries) > 0 {
					fmt.Fprintf(w, "\n  %s\n", bold(div.Name))
					printStandingsTable(w, entries)
				}
			}
		} else if len(conf.Standings.Entries) > 0 {
			fmt.Fprintf(w, "\n%s\n", bold(strings.ToUpper(conf.Name)))
			entries := parseStandingsEntries(conf.Standings.Entries)
			printStandingsTable(w, entries)
		}
	}

	return nil
}

func parseStandingsEntries(raw []struct {
	Team struct {
		DisplayName  string `json:"displayName"`
		Abbreviation string `json:"abbreviation"`
	} `json:"team"`
	Stats []struct {
		Name         string  `json:"name"`
		DisplayValue string  `json:"displayValue"`
		Value        float64 `json:"value"`
	} `json:"stats"`
}) []standingsEntry {
	var entries []standingsEntry
	for _, e := range raw {
		se := standingsEntry{
			Team: e.Team.DisplayName,
			Abbr: e.Team.Abbreviation,
		}
		for _, s := range e.Stats {
			switch s.Name {
			case "wins":
				se.Wins = s.DisplayValue
			case "losses":
				se.Losses = s.DisplayValue
			case "ties":
				se.Ties = s.DisplayValue
			case "winPercent", "winPct":
				se.Pct = s.DisplayValue
			case "gamesBehind":
				se.GB = s.DisplayValue
			case "differential", "pointDifferential":
				se.Diff = s.DisplayValue
			case "streak":
				se.Streak = s.DisplayValue
			case "pointsFor":
				se.PF = s.DisplayValue
			case "pointsAgainst":
				se.PA = s.DisplayValue
			}
		}
		entries = append(entries, se)
	}
	return entries
}

func printStandingsTable(w io.Writer, entries []standingsEntry) {
	tw := newTabWriter(w)
	fmt.Fprintf(tw, "  %s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
		bold("TEAM"), bold("W"), bold("L"), bold("T"), bold("PCT"), bold("GB"), bold("DIFF"), bold("STRK"), bold("PF"), bold("PA"))
	for _, e := range entries {
		fmt.Fprintf(tw, "  %s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			e.Abbr, e.Wins, e.Losses, e.Ties, e.Pct, e.GB, e.Diff, e.Streak, e.PF, e.PA)
	}
	tw.Flush()
}
