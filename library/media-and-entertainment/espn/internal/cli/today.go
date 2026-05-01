package cli

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/spf13/cobra"
)

type sportLeague struct {
	Sport  string
	League string
	Label  string
}

var defaultSports = []sportLeague{
	{"football", "nfl", "NFL"},
	{"basketball", "nba", "NBA"},
	{"baseball", "mlb", "MLB"},
	{"hockey", "nhl", "NHL"},
}

func newTodayCmd(flags *rootFlags) *cobra.Command {
	var sportFilter string

	cmd := &cobra.Command{
		Use:   "today",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Today's scores across all major sports",
		Example: `  espn-pp-cli today
  espn-pp-cli today --sport nba
  espn-pp-cli today --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			sports := defaultSports
			if sportFilter != "" {
				sports = filterSports(sports, sportFilter)
				if len(sports) == 0 {
					return usageErr(fmt.Errorf("unknown sport %q. Valid: nfl, nba, mlb, nhl", sportFilter))
				}
			}

			type result struct {
				SL     sportLeague
				Events []scoreEvent
				Err    error
			}

			results := make([]result, len(sports))
			var wg sync.WaitGroup

			for i, sl := range sports {
				wg.Add(1)
				go func(idx int, sl sportLeague) {
					defer wg.Done()
					path := fmt.Sprintf("/%s/%s/scoreboard", sl.Sport, sl.League)
					data, fetchErr := c.Get(path, nil)
					if fetchErr != nil {
						results[idx] = result{SL: sl, Err: fetchErr}
						return
					}
					results[idx] = result{SL: sl, Events: parseScoreEvents(data)}
				}(i, sl)
			}
			wg.Wait()

			// JSON output
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !humanFriendly) {
				out := make(map[string]any)
				for _, r := range results {
					if r.Err != nil {
						out[r.SL.Label] = map[string]string{"error": r.Err.Error()}
					} else {
						out[r.SL.Label] = r.Events
					}
				}
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(out)
			}

			// Table output grouped by sport
			w := cmd.OutOrStdout()
			totalGames := 0
			for _, r := range results {
				if r.Err != nil {
					fmt.Fprintf(w, "\n%s  (error: %v)\n", bold(r.SL.Label), r.Err)
					continue
				}
				if len(r.Events) == 0 {
					fmt.Fprintf(w, "\n%s  No games today\n", bold(r.SL.Label))
					continue
				}
				fmt.Fprintf(w, "\n%s\n", bold(r.SL.Label))
				tw := newTabWriter(w)
				fmt.Fprintf(tw, "  %s\t%s\t%s\t%s\n",
					bold("MATCHUP"), bold("AWAY"), bold("HOME"), bold("STATUS"))
				for _, e := range r.Events {
					away := fmt.Sprintf("%s %s", e.AwayTeam, e.AwayScore)
					home := fmt.Sprintf("%s %s", e.HomeTeam, e.HomeScore)
					status := e.Detail
					if status == "" {
						status = e.Status
					}
					fmt.Fprintf(tw, "  %s\t%s\t%s\t%s\n",
						e.Matchup, away, home, status)
				}
				tw.Flush()
				totalGames += len(r.Events)
			}
			fmt.Fprintf(w, "\n%d games today\n", totalGames)
			return nil
		},
	}

	cmd.Flags().StringVar(&sportFilter, "sport", "", "Filter to one sport (nfl, nba, mlb, nhl)")

	return cmd
}

func filterSports(sports []sportLeague, filter string) []sportLeague {
	var out []sportLeague
	for _, sl := range sports {
		if sl.League == filter || sl.Sport == filter || sl.Label == filter {
			out = append(out, sl)
		}
	}
	return out
}
