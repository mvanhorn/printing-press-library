// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/steam-web/internal/client"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/steam-web/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/steam-web/internal/store"
)

const steamRateLimitPerSec = 20.0

// openOwnedGamesStore opens the local SQLite store for owned-games caching.
func openOwnedGamesStore() (*store.Store, error) {
	return store.Open(defaultDBPath("steam-web-pp-cli"))
}

type ownedGame struct {
	Appid           int    `json:"appid"`
	Name            string `json:"name"`
	PlaytimeForever int    `json:"playtime_forever"`
	Playtime2Weeks  int    `json:"playtime_2weeks"`
	HasIcon         string `json:"img_icon_url"`
}

type ownedGamesResponse struct {
	Response struct {
		GameCount int         `json:"game_count"`
		Games     []ownedGame `json:"games"`
	} `json:"response"`
}

type playerSummary struct {
	SteamID                  string `json:"steamid"`
	PersonaName              string `json:"personaname"`
	PersonaState             int    `json:"personastate"`
	CommunityVisibilityState int    `json:"communityvisibilitystate"`
	ProfileURL               string `json:"profileurl"`
	GameID                   string `json:"gameid,omitempty"`
	GameExtraInfo            string `json:"gameextrainfo,omitempty"`
	LastLogoff               int    `json:"lastlogoff,omitempty"`
}

type playerSummariesResponse struct {
	Response struct {
		Players []playerSummary `json:"players"`
	} `json:"response"`
}

type friend struct {
	SteamID      string `json:"steamid"`
	Relationship string `json:"relationship"`
	FriendSince  int    `json:"friend_since"`
}

type friendListResponse struct {
	Friendslist struct {
		Friends []friend `json:"friends"`
	} `json:"friendslist"`
}

type achievementSchema struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Description string `json:"description"`
	Hidden      int    `json:"hidden"`
	Icon        string `json:"icon"`
}

type schemaForGameResponse struct {
	Game struct {
		GameName           string `json:"gameName"`
		AvailableGameStats struct {
			Achievements []achievementSchema `json:"achievements"`
		} `json:"availableGameStats"`
	} `json:"game"`
}

type playerAchievement struct {
	APIName     string `json:"apiname"`
	Achieved    int    `json:"achieved"`
	UnlockTime  int    `json:"unlocktime"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
}

type playerAchievementsResponse struct {
	Playerstats struct {
		SteamID      string              `json:"steamID"`
		GameName     string              `json:"gameName"`
		Achievements []playerAchievement `json:"achievements"`
		Success      bool                `json:"success"`
		Error        string              `json:"error"`
	} `json:"playerstats"`
}

type globalPctEntry struct {
	Name    string  `json:"name"`
	Percent float64 `json:"percent"`
}

type globalPctResponse struct {
	AchievementPercentages struct {
		Achievements []globalPctEntry `json:"achievements"`
	} `json:"achievementpercentages"`
}

func resolveSteamID(c *client.Client, input string) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", errors.New("steamid required")
	}
	if isSteamID64(input) {
		return input, nil
	}
	data, err := c.Get("/ISteamUser/ResolveVanityURL/v1", map[string]string{"vanityurl": input})
	if err != nil {
		return "", fmt.Errorf("resolve vanity %q: %w", input, err)
	}
	var resolved struct {
		Response struct {
			Success int    `json:"success"`
			SteamID string `json:"steamid"`
		} `json:"response"`
	}
	if err := json.Unmarshal(data, &resolved); err != nil {
		return "", fmt.Errorf("parse resolve response: %w", err)
	}
	if resolved.Response.Success != 1 || resolved.Response.SteamID == "" {
		return "", fmt.Errorf("could not resolve %q to a SteamID", input)
	}
	return resolved.Response.SteamID, nil
}

func isSteamID64(s string) bool {
	if len(s) != 17 {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func fetchOwnedGames(c *client.Client, steamid string) (*ownedGamesResponse, error) {
	params := map[string]string{
		"steamid":                   steamid,
		"include_appinfo":           "1",
		"include_played_free_games": "1",
	}
	data, err := c.Get("/IPlayerService/GetOwnedGames/v1", params)
	if err != nil {
		return nil, err
	}
	var resp ownedGamesResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse owned games: %w", err)
	}
	return &resp, nil
}

func fetchFriendList(c *client.Client, steamid string) ([]friend, error) {
	data, err := c.Get("/ISteamUser/GetFriendList/v1", map[string]string{
		"steamid":      steamid,
		"relationship": "friend",
	})
	if err != nil {
		return nil, err
	}
	var resp friendListResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse friend list: %w", err)
	}
	return resp.Friendslist.Friends, nil
}

func fetchPlayerSummaries(c *client.Client, steamids []string) ([]playerSummary, error) {
	if len(steamids) == 0 {
		return nil, nil
	}
	const batchSize = 100
	var all []playerSummary
	for i := 0; i < len(steamids); i += batchSize {
		end := i + batchSize
		if end > len(steamids) {
			end = len(steamids)
		}
		params := map[string]string{"steamids": strings.Join(steamids[i:end], ",")}
		data, err := c.Get("/ISteamUser/GetPlayerSummaries/v2", params)
		if err != nil {
			return nil, err
		}
		var resp playerSummariesResponse
		if err := json.Unmarshal(data, &resp); err != nil {
			return nil, fmt.Errorf("parse player summaries: %w", err)
		}
		all = append(all, resp.Response.Players...)
	}
	return all, nil
}

func fetchSchemaForGame(c *client.Client, appid string) (*schemaForGameResponse, error) {
	data, err := c.Get("/ISteamUserStats/GetSchemaForGame/v2", map[string]string{"appid": appid})
	if err != nil {
		return nil, err
	}
	var resp schemaForGameResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse schema for %s: %w", appid, err)
	}
	return &resp, nil
}

func fetchPlayerAchievements(c *client.Client, steamid, appid string) (*playerAchievementsResponse, error) {
	data, err := c.Get("/ISteamUserStats/GetPlayerAchievements/v1", map[string]string{
		"steamid": steamid,
		"appid":   appid,
	})
	if err != nil {
		return nil, err
	}
	var resp playerAchievementsResponse
	if err := json.Unmarshal(data, &resp); err == nil && !resp.Playerstats.Success {
		return nil, nil
	}
	return &resp, json.Unmarshal(data, &resp)
}

func fetchGlobalPercentages(c *client.Client, appid string) (map[string]float64, error) {
	data, err := c.Get("/ISteamUserStats/GetGlobalAchievementPercentagesForApp/v2", map[string]string{
		"gameid": appid,
	})
	if err != nil {
		return nil, err
	}
	var resp globalPctResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse global percentages: %w", err)
	}
	out := make(map[string]float64, len(resp.AchievementPercentages.Achievements))
	for _, a := range resp.AchievementPercentages.Achievements {
		out[a.Name] = a.Percent
	}
	return out, nil
}

func fanOutOwnedGames(ctx context.Context, c *client.Client, limiter *cliutil.AdaptiveLimiter, steamids []string) map[string]*ownedGamesResponse {
	out := make(map[string]*ownedGamesResponse, len(steamids))
	db, dbErr := openOwnedGamesStore()
	if db != nil {
		defer db.Close()
	}
	for _, sid := range steamids {
		if ctx.Err() != nil {
			return out
		}
		limiter.Wait()
		resp, err := fetchOwnedGames(c, sid)
		if err != nil {
			var rl *cliutil.RateLimitError
			if errors.As(err, &rl) {
				limiter.OnRateLimit()
			}
			continue
		}
		limiter.OnSuccess()
		if resp != nil {
			out[sid] = resp
			if dbErr == nil && db != nil {
				if blob, marshalErr := json.Marshal(resp); marshalErr == nil {
					_ = db.Upsert("owned_games", "owned_games:"+sid, blob)
				}
			}
		}
	}
	return out
}

func hoursOf(minutes int) float64 {
	return float64(minutes) / 60.0
}

type playtimeRow struct {
	SteamID         string  `json:"steamid"`
	PersonaName     string  `json:"persona_name"`
	PlaytimeMinutes int     `json:"playtime_minutes"`
	PlaytimeHours   float64 `json:"playtime_hours"`
}

func sortByHoursDesc(rows []playtimeRow) {
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].PlaytimeMinutes > rows[j].PlaytimeMinutes
	})
}

var _ = sortByHoursDesc
