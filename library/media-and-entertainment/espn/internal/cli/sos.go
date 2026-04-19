package cli

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

// newSosCmd derives a strength-of-schedule view from the standings endpoint.
// Uses the same site.web.api.espn.com/.../standings host as standings_cmd.go.
func newSosCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sos <sport> <league>",
		Short: "Strength-of-schedule derived from league standings",
		Example: `  espn-pp-cli sos football nfl
  espn-pp-cli sos basketball nba --agent
  espn-pp-cli sos baseball mlb --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return cmd.Help()
			}
			if len(args) < 2 {
				return usageErr(fmt.Errorf("league is required\nUsage: sos <sport> <league>"))
			}
			sport, league := args[0], args[1]

			url := fmt.Sprintf("https://site.web.api.espn.com/apis/v2/sports/%s/%s/standings", sport, league)
			body, err := espnHTTPGet(flags.timeout, url)
			if err != nil {
				return err
			}

			entries := extractSOS(body)
			sort.Slice(entries, func(i, j int) bool { return entries[i].SOS > entries[j].SOS })

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !humanFriendly) {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(entries)
			}

			w := cmd.OutOrStdout()
			if len(entries) == 0 {
				fmt.Fprintln(w, "No strength-of-schedule data in standings response.")
				return nil
			}
			tw := newTabWriter(w)
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n",
				bold("RANK"), bold("TEAM"), bold("SOS"), bold("REMAINING SOS"))
			for i, e := range entries {
				fmt.Fprintf(tw, "%d\t%s\t%.4f\t%.4f\n",
					i+1, e.TeamAbbr, e.SOS, e.RemainingSOS)
			}
			return tw.Flush()
		},
	}
	return cmd
}

type sosEntry struct {
	TeamAbbr     string  `json:"team"`
	TeamName     string  `json:"team_name"`
	SOS          float64 `json:"strength_of_schedule"`
	RemainingSOS float64 `json:"remaining_strength_of_schedule"`
}

func extractSOS(data []byte) []sosEntry {
	var resp struct {
		Children []struct {
			Children []struct {
				Standings sosStandings `json:"standings"`
			} `json:"children"`
			Standings sosStandings `json:"standings"`
		} `json:"children"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil
	}
	var out []sosEntry
	collect := func(st sosStandings) {
		for _, e := range st.Entries {
			entry := sosEntry{
				TeamAbbr: e.Team.Abbreviation,
				TeamName: e.Team.DisplayName,
			}
			for _, s := range e.Stats {
				switch s.Name {
				case "strengthOfSchedule":
					entry.SOS = s.Value
				case "remainingStrengthOfSchedule":
					entry.RemainingSOS = s.Value
				}
			}
			if entry.SOS != 0 || entry.RemainingSOS != 0 {
				out = append(out, entry)
			}
		}
	}
	for _, conf := range resp.Children {
		if len(conf.Children) > 0 {
			for _, div := range conf.Children {
				collect(div.Standings)
			}
		} else {
			collect(conf.Standings)
		}
	}
	return out
}

type sosStandings struct {
	Entries []struct {
		Team struct {
			DisplayName  string `json:"displayName"`
			Abbreviation string `json:"abbreviation"`
		} `json:"team"`
		Stats []struct {
			Name  string  `json:"name"`
			Value float64 `json:"value"`
		} `json:"stats"`
	} `json:"entries"`
}
