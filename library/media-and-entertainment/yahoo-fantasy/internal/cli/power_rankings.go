// Copyright 2026 BenHof and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source auto

package cli

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/yahoo-fantasy/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/yahoo-fantasy/internal/yahoofantasy"
	"github.com/spf13/cobra"
)

func newNovelPowerRankingsCmd(flags *rootFlags) *cobra.Command {
	var flagLeague string
	var flagThroughWeek int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "power-rankings",
		Short: "Compute every team's true weekly all-play record, schedule luck delta, scoring consistency, and composite power rank.",
		Long: `Compute true weekly all-play W-L-T records across all league teams, schedule luck delta
(actual wins minus expected all-play wins), weekly scoring standard deviation, and composite
power rank from synced SQLite records or live league standings and scoreboards.`,
		Example: strings.Trim(`
  # Compute league power rankings and schedule luck through Week 9
  yahoo-fantasy-pp-cli power-rankings --league 449.l.365083 --through-week 9 --agent

  # Compute power rankings from local SQLite mirror
  yahoo-fantasy-pp-cli power-rankings --league 449.l.365083 --data-source local --json
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:data-source":      "auto",
			"pp:happy-args":       "--league=449.l.365083;--through-week=9",
			"pp:typed-exit-codes": "0,2,3",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "power-rankings")
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if err := validateDataSourceStrategy(flags, "auto"); err != nil {
				return usageErr(err)
			}

			leagueKey := strings.TrimSpace(flagLeague)
			if leagueKey == "" && len(args) > 0 {
				leagueKey = strings.TrimSpace(args[0])
			}
			if leagueKey == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--league is required (e.g. --league 449.l.365083)"))
			}

			if dbPath == "" {
				dbPath = defaultDBPath("yahoo-fantasy-pp-cli")
			}

			var teams []yahoofantasy.TeamRecord
			weeklyScores := map[int]map[string]float64{}
			hasDB := false

			if flags.dataSource != "live" {
				blobs, dbExists, err := loadLocalResourceBlobs(ctx, cmd, flags, dbPath, "teams", "standings", "leagues", "users", "scoreboard", "matchups")
				if err != nil {
					return err
				}
				hasDB = dbExists
				teams = append(teams, extractLeagueTeamsFromBlobs(blobs, leagueKey)...)
				for _, m := range extractLeagueMatchupsFromBlobs(blobs, leagueKey) {
					if flagThroughWeek > 0 && m.Week > flagThroughWeek {
						continue
					}
					if m.Week > 0 {
						if weeklyScores[m.Week] == nil {
							weeklyScores[m.Week] = map[string]float64{}
						}
						if m.TeamKey != "" {
							weeklyScores[m.Week][m.TeamKey] = m.TeamPoints
						}
						if m.OpponentKey != "" {
							weeklyScores[m.Week][m.OpponentKey] = m.OpponentPoints
						}
					}
				}
			}

			if len(teams) == 0 && flags.dataSource != "local" && (hasLiveAuthConfigured(flags) || cliutil.IsVerifyEnv()) {
				c, err := flags.newClient()
				if err != nil {
					return err
				}
				stPath := fmt.Sprintf("/league/%s/standings", leagueKey)
				rawSt, getErr := c.Get(ctx, stPath, map[string]string{"format": "json"})
				if getErr != nil {
					return classifyAPIError(cmd.OutOrStdout(), getErr, flags)
				}
				for _, t := range yahoofantasy.ExtractTeams(rawSt) {
					if yahoofantasy.BelongsToLeague(leagueKey, t.LeagueKey, t.TeamKey) {
						teams = append(teams, t)
					}
				}
				maxW := flagThroughWeek
				if cliutil.IsDogfoodEnv() || cliutil.IsVerifyEnv() {
					maxW = 1
				} else if maxW <= 0 {
					maxW = yahoofantasy.ExtractLeagueCurrentWeek(rawSt)
					if maxW <= 0 {
						lgPath := fmt.Sprintf("/league/%s", leagueKey)
						rawLg, lgErr := c.Get(ctx, lgPath, map[string]string{"format": "json"})
						if lgErr != nil {
							return classifyAPIError(cmd.OutOrStdout(), lgErr, flags)
						}
						maxW = yahoofantasy.ExtractLeagueCurrentWeek(rawLg)
					}
					if maxW <= 0 {
						return usageErr(fmt.Errorf("could not determine current league week; pass --through-week"))
					}
				}
				for w := 1; w <= maxW; w++ {
					sbPath := fmt.Sprintf("/league/%s/scoreboard;week=%s", leagueKey, strconv.Itoa(w))
					rawSB, sbErr := c.Get(ctx, sbPath, map[string]string{"format": "json"})
					if sbErr != nil {
						return classifyAPIError(cmd.OutOrStdout(), sbErr, flags)
					}
					for _, m := range yahoofantasy.ExtractMatchups(rawSB) {
						if !yahoofantasy.BelongsToLeague(leagueKey, m.LeagueKey, m.TeamKey, m.OpponentKey) {
							continue
						}
						if weeklyScores[w] == nil {
							weeklyScores[w] = map[string]float64{}
						}
						if m.TeamKey != "" {
							weeklyScores[w][m.TeamKey] = m.TeamPoints
						}
						if m.OpponentKey != "" {
							weeklyScores[w][m.OpponentKey] = m.OpponentPoints
						}
					}
				}
			}

			if !hasDB && len(teams) == 0 {
				if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
					fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: yahoo-fantasy-pp-cli sync --resources users --db %s\n", dbPath, dbPath)
				}
			}

			rows := yahoofantasy.ComputePowerRankings(teams, weeklyScores)
			if rows == nil {
				rows = make([]yahoofantasy.PowerRankingRow, 0)
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No league teams found for power rankings.")
				return nil
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
			fmt.Fprintf(tw, "POWER\tSTND\tTEAM\tRECORD\tALL-PLAY\tWIN%%\tLUCK\tPTS FOR\tSTDDEV\tSCORE\n")
			for _, r := range rows {
				fmt.Fprintf(tw, "%d\t%d\t%s (%s)\t%s\t%d-%d-%d\t%.3f\t%+.2f\t%.2f\t%.2f\t%.2f\n",
					r.PowerRank, r.StandingRank, r.TeamName, r.TeamKey, r.ActualRecord,
					r.AllPlayWins, r.AllPlayLosses, r.AllPlayTies, r.AllPlayWinPct, r.ScheduleLuckDelta,
					r.PointsFor, r.WeeklyScoringStdDev, r.CompositePowerScore)
			}
			return tw.Flush()
		},
	}

	cmd.Flags().StringVar(&flagLeague, "league", "", "Yahoo league key in {game_id}.l.{league_id} format (e.g. 449.l.365083)")
	cmd.Flags().IntVar(&flagThroughWeek, "through-week", 0, "Include weekly scoreboards up through this NFL week number")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to local SQLite database mirror")
	return cmd
}
