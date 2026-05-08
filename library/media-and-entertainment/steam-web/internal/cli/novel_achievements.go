// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/steam-web/internal/cliutil"
)

type achievementHuntRow struct {
	APIName     string  `json:"apiname"`
	DisplayName string  `json:"display_name"`
	Description string  `json:"description"`
	Achieved    bool    `json:"achieved"`
	GlobalPct   float64 `json:"global_pct"`
	UnlockTime  int     `json:"unlock_time,omitempty"`
}

type achievementHuntOutput struct {
	Appid        string               `json:"appid"`
	GameName     string               `json:"game_name,omitempty"`
	SteamID      string               `json:"steamid"`
	Achievements []achievementHuntRow `json:"achievements"`
	Counts       struct {
		Total    int `json:"total"`
		Unlocked int `json:"unlocked"`
		Locked   int `json:"locked"`
	} `json:"counts"`
}

func newAchievementHuntCmd(flags *rootFlags) *cobra.Command {
	var steamidArg, appidArg string
	var lockedOnly, rareOnly bool
	cmd := &cobra.Command{
		Use:         "achievement-hunt [appid]",
		Short:       "Per-app achievement workbench: schema + your unlock state + global rarity in one table",
		Long:        "Joins GetSchemaForGame, GetPlayerAchievements, and GetGlobalAchievementPercentagesForApp for one app.",
		Example:     "  steam-web-pp-cli achievement-hunt 1245620 --steamid 76561197960287930 --locked --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				appidArg = args[0]
			}
			if appidArg == "" && steamidArg == "" {
				return cmd.Help()
			}
			// Honor --dry-run before format/required-flag validation so
			// verify probes (which pass synthetic positionals) don't fall
			// through to API-shaped errors.
			if dryRunOK(flags) {
				return nil
			}
			if appidArg != "" {
				if n, err := strconv.Atoi(appidArg); err != nil || n <= 0 {
					return fmt.Errorf("invalid appid %q: must be a positive integer", appidArg)
				}
			}
			if appidArg == "" {
				return fmt.Errorf("appid is required (positional or --appid)")
			}
			if steamidArg == "" {
				return fmt.Errorf("--steamid is required")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			steamid, err := resolveSteamID(c, steamidArg)
			if err != nil {
				return err
			}
			schema, err := fetchSchemaForGame(c, appidArg)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			pct, err := fetchGlobalPercentages(c, appidArg)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			player, _ := fetchPlayerAchievements(c, steamid, appidArg)
			achievedMap := make(map[string]playerAchievement)
			if player != nil {
				for _, a := range player.Playerstats.Achievements {
					achievedMap[a.APIName] = a
				}
			}
			rows := make([]achievementHuntRow, 0, len(schema.Game.AvailableGameStats.Achievements))
			for _, s := range schema.Game.AvailableGameStats.Achievements {
				row := achievementHuntRow{
					APIName: s.Name, DisplayName: s.DisplayName,
					Description: s.Description, GlobalPct: pct[s.Name],
				}
				if pa, ok := achievedMap[s.Name]; ok && pa.Achieved == 1 {
					row.Achieved = true
					row.UnlockTime = pa.UnlockTime
				}
				if lockedOnly && row.Achieved {
					continue
				}
				rows = append(rows, row)
			}
			out := achievementHuntOutput{
				Appid: appidArg, GameName: schema.Game.GameName,
				SteamID: steamid, Achievements: rows,
			}
			out.Counts.Total = len(rows)
			for _, r := range rows {
				if r.Achieved {
					out.Counts.Unlocked++
				} else {
					out.Counts.Locked++
				}
			}
			if rareOnly {
				sort.Slice(rows, func(i, j int) bool { return rows[i].GlobalPct < rows[j].GlobalPct })
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&steamidArg, "steamid", "", "Steam ID 64 or vanity URL")
	cmd.Flags().StringVar(&appidArg, "appid", "", "App ID")
	cmd.Flags().BoolVar(&lockedOnly, "locked", false, "Filter to achievements you don't have")
	cmd.Flags().BoolVar(&rareOnly, "rare", false, "Sort by ascending global pct")
	return cmd
}

type achievementPickRow struct {
	Appid       int     `json:"appid"`
	GameName    string  `json:"game_name"`
	APIName     string  `json:"apiname"`
	DisplayName string  `json:"display_name"`
	Description string  `json:"description"`
	GlobalPct   float64 `json:"global_pct"`
}

type achievementPickOutput struct {
	SteamID      string               `json:"steamid"`
	Picks        []achievementPickRow `json:"picks"`
	GamesScanned int                  `json:"games_scanned"`
	GamesSkipped int                  `json:"games_skipped"`
}

func newNextAchievementCmd(flags *rootFlags) *cobra.Command {
	var steamidArg, appidScope string
	var limit, maxApps int
	cmd := &cobra.Command{
		Use:         "next-achievement",
		Short:       "Easiest still-locked achievement across your whole library (highest global pct still locked)",
		Long:        "Iterates owned games (capped by --max-apps) and surfaces achievements with highest global pct that are still locked.",
		Example:     "  steam-web-pp-cli next-achievement --steamid 76561197960287930 --limit 10 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if steamidArg == "" {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			steamid, err := resolveSteamID(c, steamidArg)
			if err != nil {
				return err
			}
			out := achievementPickOutput{SteamID: steamid}
			limiter := cliutil.NewAdaptiveLimiter(steamRateLimitPerSec)
			var appids []string
			if appidScope != "" {
				appids = []string{appidScope}
			} else {
				owned, err := fetchOwnedGames(c, steamid)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				games := append([]ownedGame{}, owned.Response.Games...)
				sort.Slice(games, func(i, j int) bool {
					if games[i].Playtime2Weeks != games[j].Playtime2Weeks {
						return games[i].Playtime2Weeks > games[j].Playtime2Weeks
					}
					return games[i].PlaytimeForever > games[j].PlaytimeForever
				})
				if maxApps > 0 && len(games) > maxApps {
					games = games[:maxApps]
				}
				for _, g := range games {
					appids = append(appids, strconv.Itoa(g.Appid))
				}
			}
			for _, appid := range appids {
				if cmd.Context().Err() != nil {
					break
				}
				limiter.Wait()
				schema, err := fetchSchemaForGame(c, appid)
				if err != nil || len(schema.Game.AvailableGameStats.Achievements) == 0 {
					out.GamesSkipped++
					continue
				}
				limiter.OnSuccess()
				limiter.Wait()
				pct, err := fetchGlobalPercentages(c, appid)
				if err != nil {
					out.GamesSkipped++
					continue
				}
				limiter.OnSuccess()
				limiter.Wait()
				player, _ := fetchPlayerAchievements(c, steamid, appid)
				if player == nil {
					out.GamesSkipped++
					continue
				}
				limiter.OnSuccess()
				out.GamesScanned++
				achievedSet := make(map[string]bool, len(player.Playerstats.Achievements))
				for _, a := range player.Playerstats.Achievements {
					if a.Achieved == 1 {
						achievedSet[a.APIName] = true
					}
				}
				for _, s := range schema.Game.AvailableGameStats.Achievements {
					if achievedSet[s.Name] {
						continue
					}
					n, _ := strconv.Atoi(appid)
					out.Picks = append(out.Picks, achievementPickRow{
						Appid: n, GameName: schema.Game.GameName,
						APIName: s.Name, DisplayName: s.DisplayName,
						Description: s.Description, GlobalPct: pct[s.Name],
					})
				}
			}
			sort.Slice(out.Picks, func(i, j int) bool { return out.Picks[i].GlobalPct > out.Picks[j].GlobalPct })
			if limit > 0 && len(out.Picks) > limit {
				out.Picks = out.Picks[:limit]
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&steamidArg, "steamid", "", "Steam ID 64 or vanity URL")
	cmd.Flags().StringVar(&appidScope, "app", "", "Scope to one appid")
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum picks (0 unlimited)")
	cmd.Flags().IntVar(&maxApps, "max-apps", 50, "Maximum apps to sweep")
	return cmd
}

func newRareAchievementsCmd(flags *rootFlags) *cobra.Command {
	var steamidArg string
	var limit, maxApps int
	cmd := &cobra.Command{
		Use:         "rare-achievements",
		Short:       "Rarest unlocked achievements across your whole library (lowest global pct that you have)",
		Long:        "Inverse of next-achievement — sweeps owned games and surfaces achievements you've earned with lowest global pct.",
		Example:     "  steam-web-pp-cli rare-achievements --steamid 76561197960287930 --limit 10 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if steamidArg == "" {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			steamid, err := resolveSteamID(c, steamidArg)
			if err != nil {
				return err
			}
			out := achievementPickOutput{SteamID: steamid}
			limiter := cliutil.NewAdaptiveLimiter(steamRateLimitPerSec)
			owned, err := fetchOwnedGames(c, steamid)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			games := append([]ownedGame{}, owned.Response.Games...)
			sort.Slice(games, func(i, j int) bool { return games[i].PlaytimeForever > games[j].PlaytimeForever })
			if maxApps > 0 && len(games) > maxApps {
				games = games[:maxApps]
			}
			for _, g := range games {
				if cmd.Context().Err() != nil {
					break
				}
				appid := strconv.Itoa(g.Appid)
				limiter.Wait()
				schema, err := fetchSchemaForGame(c, appid)
				if err != nil || len(schema.Game.AvailableGameStats.Achievements) == 0 {
					out.GamesSkipped++
					continue
				}
				limiter.OnSuccess()
				limiter.Wait()
				pct, err := fetchGlobalPercentages(c, appid)
				if err != nil {
					out.GamesSkipped++
					continue
				}
				limiter.OnSuccess()
				limiter.Wait()
				player, _ := fetchPlayerAchievements(c, steamid, appid)
				if player == nil {
					out.GamesSkipped++
					continue
				}
				limiter.OnSuccess()
				out.GamesScanned++
				schemaIndex := make(map[string]achievementSchema, len(schema.Game.AvailableGameStats.Achievements))
				for _, s := range schema.Game.AvailableGameStats.Achievements {
					schemaIndex[s.Name] = s
				}
				for _, a := range player.Playerstats.Achievements {
					if a.Achieved != 1 {
						continue
					}
					s := schemaIndex[a.APIName]
					if s.Name == "" {
						continue
					}
					out.Picks = append(out.Picks, achievementPickRow{
						Appid: g.Appid, GameName: schema.Game.GameName,
						APIName: s.Name, DisplayName: s.DisplayName,
						Description: s.Description, GlobalPct: pct[s.Name],
					})
				}
			}
			sort.Slice(out.Picks, func(i, j int) bool { return out.Picks[i].GlobalPct < out.Picks[j].GlobalPct })
			if limit > 0 && len(out.Picks) > limit {
				out.Picks = out.Picks[:limit]
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&steamidArg, "steamid", "", "Steam ID 64 or vanity URL")
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum picks")
	cmd.Flags().IntVar(&maxApps, "max-apps", 50, "Maximum apps to sweep")
	return cmd
}
