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

func newNovelDraftRoiCmd(flags *rootFlags) *cobra.Command {
	var flagLeague string
	var flagLimit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "draft-roi",
		Short: "Evaluate snake draft rounds or auction dollar costs against season-to-date fantasy points and current roster retention.",
		Long: `Join league draft results with season-to-date player points and current team rosters
to compute surplus points over round expectations, points per auction dollar, drafter
retention, and steal/value/par/bust verdicts.`,
		Example: strings.Trim(`
  # Evaluate draft steals and busts for a league
  yahoo-fantasy-pp-cli draft-roi --league 449.l.365083 --agent

  # Evaluate draft ROI from local SQLite mirror
  yahoo-fantasy-pp-cli draft-roi --league 449.l.365083 --data-source local --json
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:data-source":      "auto",
			"pp:happy-args":       "--league=449.l.365083",
			"pp:typed-exit-codes": "0,2,3",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "draft-roi")
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

			var picks []yahoofantasy.DraftPickRecord
			playersByKey := map[string]yahoofantasy.PlayerRecord{}
			ownerByPlayer := map[string]string{}
			hasDB := false

			if flags.dataSource != "live" {
				blobs, dbExists, err := loadLocalResourceBlobs(ctx, cmd, flags, dbPath, "leagues", "players", "teams", "draft", "rosters")
				if err != nil {
					return err
				}
				hasDB = dbExists
				for _, b := range blobs {
					for _, pk := range yahoofantasy.ExtractDraftPicks(b) {
						if yahoofantasy.BelongsToLeague(leagueKey, pk.LeagueKey, pk.TeamKey) {
							picks = append(picks, pk)
						}
					}
				}
				for _, p := range extractLeaguePlayersFromBlobs(blobs, leagueKey) {
					playersByKey[p.PlayerKey] = p
					if p.OwnerTeamKey != "" {
						ownerByPlayer[p.PlayerKey] = p.OwnerTeamKey
					}
				}
			}

			if len(picks) == 0 && flags.dataSource != "local" && (hasLiveAuthConfigured(flags) || cliutil.IsVerifyEnv()) {
				c, err := flags.newClient()
				if err != nil {
					return err
				}
				drPath := fmt.Sprintf("/league/%s/draftresults", leagueKey)
				rawDR, getErr := c.Get(ctx, drPath, map[string]string{"format": "json"})
				if getErr != nil {
					return classifyAPIError(cmd.OutOrStdout(), getErr, flags)
				}
				for _, pk := range yahoofantasy.ExtractDraftPicks(rawDR) {
					if yahoofantasy.BelongsToLeague(leagueKey, pk.LeagueKey, pk.TeamKey) {
						picks = append(picks, pk)
					}
				}
				for _, p := range yahoofantasy.ExtractPlayers(rawDR) {
					playersByKey[p.PlayerKey] = p
					if p.OwnerTeamKey != "" {
						ownerByPlayer[p.PlayerKey] = p.OwnerTeamKey
					}
				}

				// Fetch season stats and current league ownership in batches of 25 drafted players
				var draftedKeys []string
				seenKey := map[string]bool{}
				for _, pk := range picks {
					if pk.PlayerKey != "" && !seenKey[pk.PlayerKey] {
						seenKey[pk.PlayerKey] = true
						draftedKeys = append(draftedKeys, pk.PlayerKey)
					}
				}
				const batchSize = 25
				for start := 0; start < len(draftedKeys); start += batchSize {
					end := start + batchSize
					if end > len(draftedKeys) {
						end = len(draftedKeys)
					}
					joined := strings.Join(draftedKeys[start:end], ",")
					statsPath := fmt.Sprintf("/league/%s/players;player_keys=%s/stats;type=season;week=1", leagueKey, joined)
					rawStats, statsErr := c.Get(ctx, statsPath, map[string]string{"format": "json"})
					if statsErr != nil {
						return classifyAPIError(cmd.OutOrStdout(), statsErr, flags)
					}
					for _, p := range yahoofantasy.ExtractPlayers(rawStats) {
						cur := playersByKey[p.PlayerKey]
						if p.FullName != "" {
							cur.FullName = p.FullName
						}
						if p.DisplayPosition != "" {
							cur.DisplayPosition = p.DisplayPosition
						}
						if p.PointsTotal > 0 {
							cur.PointsTotal = p.PointsTotal
						}
						if p.PointsPerGame > 0 {
							cur.PointsPerGame = p.PointsPerGame
						}
						if p.GamesPlayed > 0 {
							cur.GamesPlayed = p.GamesPlayed
						}
						cur.PlayerKey = p.PlayerKey
						playersByKey[p.PlayerKey] = cur
					}

					ownPath := fmt.Sprintf("/league/%s/players;player_keys=%s/ownership", leagueKey, joined)
					rawOwn, ownErr := c.Get(ctx, ownPath, map[string]string{"format": "json"})
					if ownErr != nil {
						return classifyAPIError(cmd.OutOrStdout(), ownErr, flags)
					}
					for _, p := range yahoofantasy.ExtractPlayers(rawOwn) {
						cur := playersByKey[p.PlayerKey]
						if p.OwnerTeamKey != "" {
							cur.OwnerTeamKey = p.OwnerTeamKey
							ownerByPlayer[p.PlayerKey] = p.OwnerTeamKey
						}
						if p.OwnershipStatus != "" {
							cur.OwnershipStatus = p.OwnershipStatus
						}
						cur.PlayerKey = p.PlayerKey
						playersByKey[p.PlayerKey] = cur
					}
				}
			}

			if !hasDB && len(picks) == 0 {
				if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
					fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: yahoo-fantasy-pp-cli sync --resources users --db %s\n", dbPath, dbPath)
				}
			}

			rows := yahoofantasy.ComputeDraftROI(picks, playersByKey, ownerByPlayer)
			if rows == nil {
				rows = make([]yahoofantasy.DraftROIRow, 0)
			}
			if flagLimit > 0 && len(rows) > flagLimit {
				rows = rows[:flagLimit]
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No draft picks found for league.")
				return nil
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
			fmt.Fprintf(tw, "RANK\tROUND (PICK)\tPLAYER\tDRAFTED BY\tPTS\tROUND EXP\tSURPLUS\tRETAINED\tVERDICT\n")
			for _, r := range rows {
				fmt.Fprintf(tw, "%d\tR%d (#%d)\t%s (%s)\t%s\t%.1f\t%.1f\t%+.1f\t%v\t%s\n",
					r.Rank, r.Round, r.Pick, r.PlayerName, r.PlayerKey, r.DraftedByTeamKey,
					r.SeasonPoints, r.RoundExpectedPoints, r.SurplusPoints, r.RetainedByDrafter, r.Verdict)
			}
			return tw.Flush()
		},
	}

	cmd.Flags().StringVar(&flagLeague, "league", "", "Yahoo league key in {game_id}.l.{league_id} format (e.g. 449.l.365083)")
	cmd.Flags().IntVar(&flagLimit, "limit", 0, "Optional maximum number of ranked draft picks to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Path to local SQLite database mirror")
	return cmd
}
