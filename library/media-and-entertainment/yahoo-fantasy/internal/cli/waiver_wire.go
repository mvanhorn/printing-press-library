// Copyright 2026 BenHof and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source auto

package cli

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/yahoo-fantasy/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/yahoo-fantasy/internal/yahoofantasy"
	"github.com/spf13/cobra"
)

func newNovelWaiverWireCmd(flags *rootFlags) *cobra.Command {
	var flagLeague string
	var flagTeam string
	var flagPos string
	var flagLimit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "waiver-wire",
		Short: "Rank available free agents and waiver players by net points-per-game upgrade over your weakest bench player.",
		Long: `Join available free agents (FA) and waiver (W) players against your team's current roster
and scoring stats to rank waiver targets by net points-per-game upgrade over your weakest
rostered bench player at each eligible position.`,
		Example: strings.Trim(`
  # Rank top 10 RB waiver upgrades for your team
  yahoo-fantasy-pp-cli waiver-wire --league 449.l.365083 --team 449.l.365083.t.1 --pos RB --limit 10 --agent

  # Inspect top waiver upgrades across all positions from local SQLite mirror
  yahoo-fantasy-pp-cli waiver-wire --league 449.l.365083 --team 449.l.365083.t.1 --data-source local --json
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:data-source":      "auto",
			"pp:happy-args":       "--league=449.l.365083;--team=449.l.365083.t.1;--pos=RB;--limit=10",
			"pp:typed-exit-codes": "0,2,3",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "waiver-wire")
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

			if dbPath == "" {
				dbPath = defaultDBPath("yahoo-fantasy-pp-cli")
			}

			var candidates []yahoofantasy.PlayerRecord
			var roster []yahoofantasy.PlayerRecord
			hasDB := false

			if flags.dataSource != "live" {
				blobs, dbExists, err := loadLocalResourceBlobs(ctx, cmd, flags, dbPath, "players", "rosters", "leagues", "teams", "users")
				if err != nil {
					return err
				}
				hasDB = dbExists
				for _, p := range extractLeaguePlayersFromBlobs(blobs, leagueKey) {
					if p.OwnerTeamKey == teamKey || (p.OwnerTeamKey == "" && p.SelectedPosition != "" && p.OwnershipStatus != "FA" && p.OwnershipStatus != "W") {
						roster = append(roster, p)
					} else if p.OwnerTeamKey == "" {
						candidates = append(candidates, p)
					}
				}
			}

			if len(candidates) == 0 && flags.dataSource != "local" && (hasLiveAuthConfigured(flags) || cliutil.IsVerifyEnv()) {
				c, err := flags.newClient()
				if err != nil {
					return err
				}
				pos := strings.ToUpper(strings.TrimSpace(flagPos))
				if pos == "" {
					pos = "O"
				}
				faPath := fmt.Sprintf("/league/%s/players;status=A;position=%s;sort=PTS;start=0;count=25/percent_owned", leagueKey, pos)
				rawFA, getErr := c.Get(ctx, faPath, map[string]string{"format": "json"})
				if getErr != nil {
					return classifyAPIError(cmd.OutOrStdout(), getErr, flags)
				}
				candidates = yahoofantasy.ExtractPlayers(rawFA)

				rosterPath := fmt.Sprintf("/team/%s/roster;week=1", teamKey)
				rawRoster, rErr := c.Get(ctx, rosterPath, map[string]string{"format": "json"})
				if rErr != nil {
					return classifyAPIError(cmd.OutOrStdout(), rErr, flags)
				}
				roster = yahoofantasy.ExtractPlayers(rawRoster)

				// Enrich candidates and roster with season stats (points_total + games_played -> PPG) when not already populated
				var needStats []string
				seenKey := map[string]bool{}
				for _, p := range append(append([]yahoofantasy.PlayerRecord{}, candidates...), roster...) {
					if p.PlayerKey != "" && p.PointsPerGame == 0 && p.ProjectedPoints == 0 && !seenKey[p.PlayerKey] {
						seenKey[p.PlayerKey] = true
						needStats = append(needStats, p.PlayerKey)
					}
				}
				statsByKey := map[string]yahoofantasy.PlayerRecord{}
				const batchSize = 25
				for start := 0; start < len(needStats); start += batchSize {
					end := start + batchSize
					if end > len(needStats) {
						end = len(needStats)
					}
					joined := strings.Join(needStats[start:end], ",")
					statsPath := fmt.Sprintf("/league/%s/players;player_keys=%s/stats;type=season;week=1", leagueKey, joined)
					rawStats, statsErr := c.Get(ctx, statsPath, map[string]string{"format": "json"})
					if statsErr != nil {
						return classifyAPIError(cmd.OutOrStdout(), statsErr, flags)
					}
					for _, sp := range yahoofantasy.ExtractPlayers(rawStats) {
						statsByKey[sp.PlayerKey] = sp
					}
				}
				if len(statsByKey) > 0 {
					for i := range candidates {
						if sp, ok := statsByKey[candidates[i].PlayerKey]; ok {
							if sp.PointsTotal > 0 {
								candidates[i].PointsTotal = sp.PointsTotal
							}
							if sp.GamesPlayed > 0 {
								candidates[i].GamesPlayed = sp.GamesPlayed
							}
							if sp.PointsPerGame > 0 {
								candidates[i].PointsPerGame = sp.PointsPerGame
							}
						}
					}
					for i := range roster {
						if sp, ok := statsByKey[roster[i].PlayerKey]; ok {
							if sp.PointsTotal > 0 {
								roster[i].PointsTotal = sp.PointsTotal
							}
							if sp.GamesPlayed > 0 {
								roster[i].GamesPlayed = sp.GamesPlayed
							}
							if sp.PointsPerGame > 0 {
								roster[i].PointsPerGame = sp.PointsPerGame
							}
						}
					}
				}
			}

			if !hasDB && len(candidates) == 0 {
				if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
					fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: yahoo-fantasy-pp-cli sync --resources users --db %s\n", dbPath, dbPath)
				}
			}

			limit := flagLimit
			if limit <= 0 {
				limit = 10
			}
			rows := yahoofantasy.RankWaiverTargets(leagueKey, teamKey, candidates, roster, flagPos, limit)
			if rows == nil {
				rows = make([]yahoofantasy.WaiverUpgrade, 0)
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No matching waiver wire upgrades found.")
				return nil
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
			fmt.Fprintf(tw, "RANK\tADD PLAYER\tPOS\tSTATUS\tOWN%%\tPPG\tDROP BENCH\tDROP PPG\tNET UPGRADE\n")
			for _, r := range rows {
				fmt.Fprintf(tw, "%d\t%s (%s)\t%s\t%s\t%.1f%%\t%.2f\t%s\t%.2f\t%+.2f\n",
					r.Rank, r.AddPlayerName, r.AddPlayerKey, r.Position, r.OwnershipStatus, r.PercentOwned, r.CandidatePPG, r.DropPlayerName, r.DropPlayerPPG, r.NetUpgradePPG)
			}
			return tw.Flush()
		},
	}

	cmd.Flags().StringVar(&flagLeague, "league", "", "Yahoo league key in {game_id}.l.{league_id} format (e.g. 449.l.365083)")
	cmd.Flags().StringVar(&flagTeam, "team", "", "Yahoo team key in {game_id}.l.{league_id}.t.{team_id} format (e.g. 449.l.365083.t.1)")
	cmd.Flags().StringVar(&flagPos, "pos", "", "Filter waiver targets by position (e.g. QB, RB, WR, TE, K, DEF)")
	cmd.Flags().IntVar(&flagLimit, "limit", 10, "Maximum number of ranked waiver recommendations to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to local SQLite database mirror")
	return cmd
}
