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

func newNovelMatchupRadarCmd(flags *rootFlags) *cobra.Command {
	var flagLeague string
	var flagTeam string
	var flagWeek int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "matchup-radar",
		Short: "Compare both opponents' starting lineups slot-by-slot with projected vs. actual points and injury risk.",
		Long: `Align a weekly head-to-head fantasy matchup slot-by-slot (QB vs QB, WR1 vs WR1, FLEX vs FLEX)
with projected points, actual points, win probability, and active starter injury risk alerts.`,
		Example: strings.Trim(`
  # Inspect Week 9 head-to-head matchup radar for your team
  yahoo-fantasy-pp-cli matchup-radar --league 449.l.365083 --team 449.l.365083.t.1 --week 9 --agent

  # Read weekly matchup radar from local SQLite mirror
  yahoo-fantasy-pp-cli matchup-radar --league 449.l.365083 --team 449.l.365083.t.1 --week 9 --data-source local --json
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:data-source":      "auto",
			"pp:happy-args":       "--league=449.l.365083;--team=449.l.365083.t.1;--week=9",
			"pp:typed-exit-codes": "0,2,3",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "matchup-radar")
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if err := validateDataSourceStrategy(flags, "auto"); err != nil {
				return usageErr(err)
			}

			leagueKey := strings.TrimSpace(flagLeague)
			teamKey := strings.TrimSpace(flagTeam)
			if leagueKey == "" && strings.Contains(teamKey, ".t.") {
				leagueKey = strings.SplitN(teamKey, ".t.", 2)[0]
			}
			if leagueKey == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--league is required (e.g. --league 449.l.365083)"))
			}
			if teamKey == "" {
				teamKey = leagueKey + ".t.1"
			}
			week := flagWeek
			if week <= 0 {
				week = 1
			}

			if dbPath == "" {
				dbPath = defaultDBPath("yahoo-fantasy-pp-cli")
			}

			var matchups []yahoofantasy.MatchupRecord
			localRostersByTeam := map[string][]yahoofantasy.PlayerRecord{}
			hasDB := false
			if flags.dataSource != "live" {
				blobs, dbExists, err := loadLocalResourceBlobs(ctx, cmd, flags, dbPath, "leagues", "teams", "matchups", "scoreboard", "rosters", "players")
				if err != nil {
					return err
				}
				hasDB = dbExists
				matchups = append(matchups, extractLeagueMatchupsFromBlobs(blobs, leagueKey)...)
				for _, t := range extractLeagueTeamsFromBlobs(blobs, leagueKey) {
					if len(t.Roster) > 0 {
						localRostersByTeam[t.TeamKey] = t.Roster
					}
				}
				for _, p := range extractLeaguePlayersFromBlobs(blobs, leagueKey) {
					if p.OwnerTeamKey != "" {
						localRostersByTeam[p.OwnerTeamKey] = append(localRostersByTeam[p.OwnerTeamKey], p)
					}
				}
			}

			if len(matchups) == 0 && flags.dataSource != "local" && (hasLiveAuthConfigured(flags) || cliutil.IsVerifyEnv()) {
				c, err := flags.newClient()
				if err != nil {
					return err
				}
				sbPath := fmt.Sprintf("/league/%s/scoreboard;week=%s", leagueKey, strconv.Itoa(week))
				rawSB, getErr := c.Get(ctx, sbPath, map[string]string{"format": "json"})
				if getErr != nil {
					return classifyAPIError(cmd.OutOrStdout(), getErr, flags)
				}
				for _, m := range yahoofantasy.ExtractMatchups(rawSB) {
					if yahoofantasy.BelongsToLeague(leagueKey, m.LeagueKey, m.TeamKey, m.OpponentKey) {
						matchups = append(matchups, m)
					}
				}
			}

			if !hasDB && len(matchups) == 0 {
				if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
					fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: yahoo-fantasy-pp-cli sync --resources users --db %s\n", dbPath, dbPath)
				}
			}

			views := make([]yahoofantasy.MatchupRadarView, 0, len(matchups))
			for _, m := range matchups {
				if flagWeek > 0 && m.Week > 0 && m.Week != flagWeek {
					continue
				}
				if flagTeam != "" && m.TeamKey != teamKey && m.OpponentKey != teamKey {
					continue
				}
				mWeek := m.Week
				if mWeek <= 0 {
					mWeek = week
				}
				if len(m.TeamStarters) == 0 && m.TeamKey != "" {
					if r := localRostersByTeam[m.TeamKey]; len(r) > 0 {
						m.TeamStarters = yahoofantasy.FilterActiveStarters(r)
					} else if flags.dataSource != "local" && (hasLiveAuthConfigured(flags) || cliutil.IsVerifyEnv()) {
						c, err := flags.newClient()
						if err != nil {
							return err
						}
						rPath := fmt.Sprintf("/team/%s/roster;week=%s", m.TeamKey, strconv.Itoa(mWeek))
						rawR, rErr := c.Get(ctx, rPath, map[string]string{"format": "json"})
						if rErr != nil {
							return classifyAPIError(cmd.OutOrStdout(), rErr, flags)
						}
						m.TeamStarters = yahoofantasy.FilterActiveStarters(yahoofantasy.ExtractPlayers(rawR))
					}
				}
				if len(m.OpponentStarters) == 0 && m.OpponentKey != "" {
					if r := localRostersByTeam[m.OpponentKey]; len(r) > 0 {
						m.OpponentStarters = yahoofantasy.FilterActiveStarters(r)
					} else if flags.dataSource != "local" && (hasLiveAuthConfigured(flags) || cliutil.IsVerifyEnv()) {
						c, err := flags.newClient()
						if err != nil {
							return err
						}
						rPath := fmt.Sprintf("/team/%s/roster;week=%s", m.OpponentKey, strconv.Itoa(mWeek))
						rawR, rErr := c.Get(ctx, rPath, map[string]string{"format": "json"})
						if rErr != nil {
							return classifyAPIError(cmd.OutOrStdout(), rErr, flags)
						}
						m.OpponentStarters = yahoofantasy.FilterActiveStarters(yahoofantasy.ExtractPlayers(rawR))
					}
				}
				views = append(views, yahoofantasy.BuildMatchupRadar(leagueKey, m))
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), views, flags)
			}
			if len(views) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No matching weekly matchups found.")
				return nil
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
			fmt.Fprintf(tw, "WEEK\tTEAM\tPTS (PROJ)\tOPPONENT\tPTS (PROJ)\tPROJ MARGIN\tALERTS\n")
			for _, v := range views {
				fmt.Fprintf(tw, "%d\t%s (%s)\t%.2f (%.2f)\t%s (%s)\t%.2f (%.2f)\t%+.2f\t%d\n",
					v.Week, v.TeamName, v.TeamKey, v.TeamPoints, v.TeamProjected, v.OpponentName, v.OpponentKey, v.OpponentPoints, v.OpponentProjected, v.ProjectedMargin, len(v.InjuryAlerts))
			}
			return tw.Flush()
		},
	}

	cmd.Flags().StringVar(&flagLeague, "league", "", "Yahoo league key in {game_id}.l.{league_id} format (e.g. 449.l.365083)")
	cmd.Flags().StringVar(&flagTeam, "team", "", "Filter matchup radar to a specific Yahoo team key (e.g. 449.l.365083.t.1)")
	cmd.Flags().IntVar(&flagWeek, "week", 0, "Target NFL week number (e.g. 9)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to local SQLite database mirror")
	return cmd
}
