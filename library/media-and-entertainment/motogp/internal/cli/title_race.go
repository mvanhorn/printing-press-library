// Copyright 2026 waterpig and contributors. Licensed under Apache-2.0.
// Novel feature: championship points progression round-by-round.

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/motogp/internal/cliutil"

	"github.com/spf13/cobra"
)

// pp:data-source computed
func newNovelTitleRaceCmd(flags *rootFlags) *cobra.Command {
	var maxRounds int
	cmd := &cobra.Command{
		Use:   "title-race <year> [class]",
		Short: "See how championship points evolved round-by-round across a season.",
		Long: "Replays each finished round's race classification and accumulates championship\n" +
			"points per rider, producing a points-over-rounds table no single endpoint returns.",
		Example: strings.Trim(`
  motogp-pp-cli title-race 2024 motogp --agent
  motogp-pp-cli title-race 2024 moto2 --rounds 5`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if len(args) < 1 {
				return usageErr(fmt.Errorf("need <year>, e.g. title-race 2024 motogp"))
			}
			year, err := parseYearArg(args[0])
			if err != nil {
				return usageErr(err)
			}
			class := "motogp"
			if len(args) >= 2 {
				class = args[1]
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()

			season, err := resolveSeason(ctx, c, flags, year)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			cat, err := resolveCategory(ctx, c, flags, season.ID, class)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			events, err := seasonEvents(ctx, c, flags, season.ID, true)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			sortEventsByStart(events)

			// Curtail expensive multi-round replay under the dogfood matrix.
			limit := maxRounds
			if cliutil.IsDogfoodEnv() && (limit == 0 || limit > 2) {
				limit = 2
			}
			if limit > 0 && limit < len(events) {
				events = events[:limit]
			}

			// Accumulate points per stable rider key (UUID/legacy/number),
			// never per display name: the API can emit a rider's name in
			// different casing/spacing (or flat vs composed) between rounds,
			// and a name key would split one rider into two totals. names maps
			// each key back to a display name for output and the leader line.
			cumulative := map[string]int{}
			names := map[string]string{}
			type roundOut struct {
				Round        int            `json:"round"`
				Event        string         `json:"event"`
				Winner       string         `json:"winner"`
				Leader       string         `json:"leader"`
				LeaderPoints int            `json:"leader_points"`
				Standings    map[string]int `json:"standings"`
			}
			var rounds []roundOut
			for i, ev := range events {
				sess, err := resolveSession(ctx, c, flags, ev.ID, cat.ID, "race")
				if err != nil {
					continue // some events (tests) have no race
				}
				rows, err := sessionClassification(ctx, c, flags, sess.ID)
				if err != nil {
					continue
				}
				winner := ""
				for _, r := range rows {
					key := r.Rider.stableKey()
					name := r.Rider.fullName()
					cumulative[key] += r.Points
					if name != "" {
						names[key] = name
					}
					if r.Position == 1 {
						winner = name
					}
				}
				// Snapshot is keyed by display name for output; leader is
				// computed from it so ties break on name (deterministic),
				// while aggregation above stayed on the stable key.
				snapshot := map[string]int{}
				for k, v := range cumulative {
					display := names[k]
					if display == "" {
						display = k
					}
					snapshot[display] = v
				}
				leader, leaderPts := leaderOf(snapshot)
				rounds = append(rounds, roundOut{
					Round:        i + 1,
					Event:        ev.label(),
					Winner:       winner,
					Leader:       leader,
					LeaderPoints: leaderPts,
					Standings:    snapshot,
				})
			}

			out := struct {
				Year   int        `json:"year"`
				Class  string     `json:"class"`
				Rounds []roundOut `json:"rounds"`
			}{Year: year, Class: strings.ReplaceAll(cat.Name, "™", ""), Rounds: rounds}

			if flags.asJSON || flags.compact || flags.selectFields != "" || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, out)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%d %s title race (%d rounds)\n", out.Year, out.Class, len(rounds))
			tableRows := make([][]string, 0, len(rounds))
			for _, r := range rounds {
				tableRows = append(tableRows, []string{
					fmt.Sprintf("%d", r.Round),
					r.Event,
					r.Winner,
					fmt.Sprintf("%s (%d)", r.Leader, r.LeaderPoints),
				})
			}
			return flags.printTable(cmd, []string{"RND", "EVENT", "WINNER", "CHAMPIONSHIP LEADER"}, tableRows)
		},
	}
	cmd.Flags().IntVar(&maxRounds, "rounds", 0, "Limit to the first N finished rounds (0 = all)")
	return cmd
}

func leaderOf(points map[string]int) (string, int) {
	best := ""
	bestPts := -1
	names := make([]string, 0, len(points))
	for k := range points {
		names = append(names, k)
	}
	sort.Strings(names) // deterministic tie-break
	for _, n := range names {
		if points[n] > bestPts {
			best = n
			bestPts = points[n]
		}
	}
	if bestPts < 0 {
		bestPts = 0
	}
	return best, bestPts
}

func sortEventsByStart(events []mgpEvent) {
	sort.SliceStable(events, func(i, j int) bool {
		return events[i].DateStart < events[j].DateStart
	})
}
