// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/steam-web/internal/cliutil"
)

func newFriendsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "friends",
		Short: "Operate over your friend list",
	}
	cmd.AddCommand(newFriendsCompareCmd(flags))
	return cmd
}

type friendsCompareRow struct {
	SteamID         string  `json:"steamid"`
	PersonaName     string  `json:"persona_name"`
	OwnsApp         bool    `json:"owns_app"`
	PlaytimeMinutes int     `json:"playtime_minutes"`
	PlaytimeHours   float64 `json:"playtime_hours"`
}

type friendsCompareOutput struct {
	MySteamID string              `json:"my_steamid"`
	Appid     string              `json:"appid"`
	Filter    string              `json:"filter,omitempty"`
	Friends   []friendsCompareRow `json:"results"`
}

func newFriendsCompareCmd(flags *rootFlags) *cobra.Command {
	var mySteamID, appidArg, filter string
	cmd := &cobra.Command{
		Use:         "compare [appid]",
		Short:       "Rank your friends by playtime in one app (throttled fan-out)",
		Long:        "Throttled fan-out across your friend list to rank everyone by hours in a given app.",
		Example:     "  steam-web-pp-cli friends compare 1245620 --my-steamid 76561197960287930 --filter owners --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				appidArg = args[0]
			}
			if appidArg == "" && mySteamID == "" {
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
			if mySteamID == "" {
				return fmt.Errorf("--my-steamid is required")
			}
			appidNum, _ := strconv.Atoi(appidArg)
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			myID, err := resolveSteamID(c, mySteamID)
			if err != nil {
				return err
			}
			friends, err := fetchFriendList(c, myID)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if len(friends) == 0 {
				return printJSONFiltered(cmd.OutOrStdout(), friendsCompareOutput{MySteamID: myID, Appid: appidArg, Filter: filter}, flags)
			}
			ids := make([]string, 0, len(friends))
			for _, f := range friends {
				ids = append(ids, f.SteamID)
			}
			limiter := cliutil.NewAdaptiveLimiter(steamRateLimitPerSec)
			fanout := fanOutOwnedGames(cmd.Context(), c, limiter, ids)
			summaries, _ := fetchPlayerSummaries(c, ids)
			personas := make(map[string]string, len(summaries))
			for _, s := range summaries {
				personas[s.SteamID] = s.PersonaName
			}
			rows := make([]friendsCompareRow, 0, len(fanout))
			for sid, owned := range fanout {
				if owned == nil {
					continue
				}
				row := friendsCompareRow{SteamID: sid, PersonaName: personas[sid]}
				for _, g := range owned.Response.Games {
					if g.Appid == appidNum {
						row.OwnsApp = true
						row.PlaytimeMinutes = g.PlaytimeForever
						row.PlaytimeHours = hoursOf(g.PlaytimeForever)
						break
					}
				}
				if !row.OwnsApp && filter != "" {
					continue
				}
				if filter == "owns-zero-hours" && row.PlaytimeMinutes != 0 {
					continue
				}
				if filter == "owners" && !row.OwnsApp {
					continue
				}
				rows = append(rows, row)
			}
			sort.Slice(rows, func(i, j int) bool { return rows[i].PlaytimeMinutes > rows[j].PlaytimeMinutes })
			return printJSONFiltered(cmd.OutOrStdout(), friendsCompareOutput{
				MySteamID: myID, Appid: appidArg, Filter: filter, Friends: rows,
			}, flags)
		},
	}
	cmd.Flags().StringVar(&mySteamID, "my-steamid", "", "Your Steam ID 64 or vanity URL")
	cmd.Flags().StringVar(&appidArg, "appid", "", "App ID to compare playtime in")
	cmd.Flags().StringVar(&filter, "filter", "", "Filter: owners, owns-zero-hours")
	return cmd
}

type currentlyPlayingRow struct {
	SteamID       string `json:"steamid"`
	PersonaName   string `json:"persona_name"`
	GameID        string `json:"gameid"`
	GameExtraInfo string `json:"game_extra_info"`
}

type currentlyPlayingOutput struct {
	MySteamID string                `json:"my_steamid"`
	Now       []currentlyPlayingRow `json:"in_game_now"`
	Online    int                   `json:"friends_online"`
	Total     int                   `json:"friends_total"`
}

func newCurrentlyPlayingCmd(flags *rootFlags) *cobra.Command {
	var mySteamID string
	cmd := &cobra.Command{
		Use:         "currently-playing",
		Short:       "Show which friends are in-game right now (one batched API call, no fanout)",
		Long:        "Reads your friend list, batches them into GetPlayerSummaries, and lists every friend whose gameextrainfo / gameid fields are set.",
		Example:     "  steam-web-pp-cli currently-playing --my-steamid 76561197960287930 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if mySteamID == "" {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			myID, err := resolveSteamID(c, mySteamID)
			if err != nil {
				return err
			}
			friends, err := fetchFriendList(c, myID)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			ids := make([]string, 0, len(friends))
			for _, f := range friends {
				ids = append(ids, f.SteamID)
			}
			summaries, err := fetchPlayerSummaries(c, ids)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			out := currentlyPlayingOutput{MySteamID: myID, Total: len(friends)}
			for _, s := range summaries {
				if s.PersonaState > 0 {
					out.Online++
				}
				if s.GameID != "" {
					out.Now = append(out.Now, currentlyPlayingRow{
						SteamID: s.SteamID, PersonaName: s.PersonaName,
						GameID: s.GameID, GameExtraInfo: s.GameExtraInfo,
					})
				}
			}
			sort.Slice(out.Now, func(i, j int) bool { return out.Now[i].PersonaName < out.Now[j].PersonaName })
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&mySteamID, "my-steamid", "", "Your Steam ID 64 or vanity URL")
	return cmd
}

type leaderboardRow struct {
	SteamID         string  `json:"steamid"`
	PersonaName     string  `json:"persona_name"`
	Achieved        int     `json:"achievements_unlocked"`
	Total           int     `json:"achievements_total"`
	PercentComplete float64 `json:"percent_complete"`
}

type leaderboardOutput struct {
	MySteamID string           `json:"my_steamid"`
	Appid     string           `json:"appid"`
	Rows      []leaderboardRow `json:"leaderboard"`
}

func newAchievementLeaderboardCmd(flags *rootFlags) *cobra.Command {
	var mySteamID, appidArg string
	cmd := &cobra.Command{
		Use:         "achievement-leaderboard [appid]",
		Short:       "Rank your friends by achievement completion percentage for one app",
		Long:        "Fans out GetPlayerAchievements across your friend list and ranks by percent complete.",
		Example:     "  steam-web-pp-cli achievement-leaderboard 1245620 --my-steamid 76561197960287930 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				appidArg = args[0]
			}
			if appidArg == "" && mySteamID == "" {
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
			if mySteamID == "" {
				return fmt.Errorf("--my-steamid is required")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			myID, err := resolveSteamID(c, mySteamID)
			if err != nil {
				return err
			}
			friends, err := fetchFriendList(c, myID)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			ids := []string{myID}
			for _, f := range friends {
				ids = append(ids, f.SteamID)
			}
			summaries, _ := fetchPlayerSummaries(c, ids)
			personas := make(map[string]string, len(summaries))
			for _, s := range summaries {
				personas[s.SteamID] = s.PersonaName
			}
			limiter := cliutil.NewAdaptiveLimiter(steamRateLimitPerSec)
			rows := []leaderboardRow{}
			for _, sid := range ids {
				if cmd.Context().Err() != nil {
					break
				}
				limiter.Wait()
				resp, err := fetchPlayerAchievements(c, sid, appidArg)
				if err != nil || resp == nil {
					continue
				}
				limiter.OnSuccess()
				achieved := 0
				for _, a := range resp.Playerstats.Achievements {
					if a.Achieved == 1 {
						achieved++
					}
				}
				total := len(resp.Playerstats.Achievements)
				pct := 0.0
				if total > 0 {
					pct = float64(achieved) / float64(total) * 100.0
				}
				rows = append(rows, leaderboardRow{
					SteamID: sid, PersonaName: personas[sid],
					Achieved: achieved, Total: total, PercentComplete: pct,
				})
			}
			sort.Slice(rows, func(i, j int) bool { return rows[i].PercentComplete > rows[j].PercentComplete })
			return printJSONFiltered(cmd.OutOrStdout(), leaderboardOutput{
				MySteamID: myID, Appid: appidArg, Rows: rows,
			}, flags)
		},
	}
	cmd.Flags().StringVar(&mySteamID, "my-steamid", "", "Your Steam ID 64 or vanity URL")
	cmd.Flags().StringVar(&appidArg, "appid", "", "App ID")
	return cmd
}
