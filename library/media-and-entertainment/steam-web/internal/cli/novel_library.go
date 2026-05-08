// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

func newLibraryCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "library",
		Short: "Inspect and compare your Steam library",
		Long:  "Tools that operate on your owned-games list as a whole, not on one app at a time.",
	}
	cmd.AddCommand(newLibraryAuditCmd(flags))
	cmd.AddCommand(newLibraryCompareCmd(flags))
	return cmd
}

type libraryAuditRow struct {
	Appid           int     `json:"appid"`
	Name            string  `json:"name"`
	PlaytimeForever int     `json:"playtime_forever_minutes"`
	PlaytimeHours   float64 `json:"playtime_forever_hours"`
	Bucket          string  `json:"bucket"`
}

type libraryAuditOutput struct {
	SteamID       string            `json:"steamid"`
	GameCount     int               `json:"game_count"`
	NeverLaunched []libraryAuditRow `json:"never_launched,omitempty"`
	Bounce        []libraryAuditRow `json:"bounce,omitempty"`
	HoursByBucket map[string]int    `json:"playtime_minutes_by_bucket,omitempty"`
}

func newLibraryAuditCmd(flags *rootFlags) *cobra.Command {
	var steamidFlag string
	var neverLaunched, bounce, genreSpend bool
	cmd := &cobra.Command{
		Use:   "audit [steamid]",
		Short: "Backlog audit on your owned-games list (never-launched, paid-bounce, genre breakdown)",
		Long: `Audit your Steam library for backlog patterns:

  --never-launched: games you own but have never opened (playtime_forever == 0)
  --bounce:        paid-feel titles played for less than 2 hours
  --genre-spend:   playtime broken down by playtime bucket

Without any flag, all three views are returned. Reads live from GetOwnedGames.`,
		Example:     "  steam-web-pp-cli library audit 76561197960287930 --never-launched --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				steamidFlag = args[0]
			}
			if steamidFlag == "" {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			steamid, err := resolveSteamID(c, steamidFlag)
			if err != nil {
				return err
			}
			owned, err := fetchOwnedGames(c, steamid)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			out := libraryAuditOutput{SteamID: steamid, GameCount: owned.Response.GameCount}
			showAll := !neverLaunched && !bounce && !genreSpend
			if showAll || neverLaunched {
				for _, g := range owned.Response.Games {
					if g.PlaytimeForever == 0 {
						out.NeverLaunched = append(out.NeverLaunched, toAuditRow(g, "never_launched"))
					}
				}
				sort.Slice(out.NeverLaunched, func(i, j int) bool { return out.NeverLaunched[i].Name < out.NeverLaunched[j].Name })
			}
			if showAll || bounce {
				for _, g := range owned.Response.Games {
					if g.PlaytimeForever > 0 && g.PlaytimeForever < 120 {
						out.Bounce = append(out.Bounce, toAuditRow(g, "bounce"))
					}
				}
				sort.Slice(out.Bounce, func(i, j int) bool { return out.Bounce[i].PlaytimeForever < out.Bounce[j].PlaytimeForever })
			}
			if showAll || genreSpend {
				out.HoursByBucket = bucketByPlaytime(owned.Response.Games)
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&steamidFlag, "steamid", "", "Steam ID 64 or vanity URL fragment")
	cmd.Flags().BoolVar(&neverLaunched, "never-launched", false, "Show only games you own but never launched")
	cmd.Flags().BoolVar(&bounce, "bounce", false, "Show paid-feel titles with under 2 hours playtime")
	cmd.Flags().BoolVar(&genreSpend, "genre-spend", false, "Show playtime breakdown by activity bucket")
	return cmd
}

func toAuditRow(g ownedGame, bucket string) libraryAuditRow {
	return libraryAuditRow{
		Appid: g.Appid, Name: g.Name,
		PlaytimeForever: g.PlaytimeForever,
		PlaytimeHours:   hoursOf(g.PlaytimeForever),
		Bucket:          bucket,
	}
}

func bucketByPlaytime(games []ownedGame) map[string]int {
	buckets := map[string]int{
		"never_launched": 0, "under_2h": 0, "2h_to_10h": 0, "10h_to_50h": 0, "over_50h": 0,
	}
	for _, g := range games {
		switch {
		case g.PlaytimeForever == 0:
			buckets["never_launched"]++
		case g.PlaytimeForever < 120:
			buckets["under_2h"] += g.PlaytimeForever
		case g.PlaytimeForever < 600:
			buckets["2h_to_10h"] += g.PlaytimeForever
		case g.PlaytimeForever < 3000:
			buckets["10h_to_50h"] += g.PlaytimeForever
		default:
			buckets["over_50h"] += g.PlaytimeForever
		}
	}
	return buckets
}

type libraryCompareRow struct {
	Appid              int     `json:"appid"`
	Name               string  `json:"name"`
	MyPlaytime         int     `json:"my_playtime_minutes"`
	TheirPlaytime      int     `json:"their_playtime_minutes"`
	PlaytimeDeltaHours float64 `json:"playtime_delta_hours"`
}

type libraryCompareOutput struct {
	MySteamID    string              `json:"my_steamid"`
	TheirSteamID string              `json:"their_steamid"`
	MineOnly     []libraryCompareRow `json:"mine_only,omitempty"`
	TheirsOnly   []libraryCompareRow `json:"theirs_only,omitempty"`
	Shared       []libraryCompareRow `json:"shared,omitempty"`
}

func newLibraryCompareCmd(flags *rootFlags) *cobra.Command {
	var theirSteamID, mySteamID string
	var mineOnly, theirsOnly, shared bool
	cmd := &cobra.Command{
		Use:   "compare [their-steamid]",
		Short: "Diff two Steam libraries (mine-only / theirs-only / shared)",
		Long: `Compare your owned-games list against another player's. Without any of
the --mine-only / --theirs-only / --shared flags, all three sets are
included.`,
		Example:     "  steam-web-pp-cli library compare 76561197960287930 --my-steamid 76561197960287930 --shared --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				theirSteamID = args[0]
			}
			if theirSteamID == "" {
				return cmd.Help()
			}
			// Honor --dry-run before required-flag validation so verify
			// probes (which pass synthetic positionals) don't fall through
			// to API-shaped errors.
			if dryRunOK(flags) {
				return nil
			}
			if mySteamID == "" {
				return fmt.Errorf("--my-steamid is required (your Steam ID 64 or vanity URL)")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			myID, err := resolveSteamID(c, mySteamID)
			if err != nil {
				return err
			}
			theirID, err := resolveSteamID(c, theirSteamID)
			if err != nil {
				return err
			}
			mine, err := fetchOwnedGames(c, myID)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			theirs, err := fetchOwnedGames(c, theirID)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			theirIndex := make(map[int]ownedGame, len(theirs.Response.Games))
			for _, g := range theirs.Response.Games {
				theirIndex[g.Appid] = g
			}
			myIndex := make(map[int]ownedGame, len(mine.Response.Games))
			for _, g := range mine.Response.Games {
				myIndex[g.Appid] = g
			}
			out := libraryCompareOutput{MySteamID: myID, TheirSteamID: theirID}
			showAll := !mineOnly && !theirsOnly && !shared
			if showAll || mineOnly {
				for _, g := range mine.Response.Games {
					if _, has := theirIndex[g.Appid]; !has {
						out.MineOnly = append(out.MineOnly, libraryCompareRow{Appid: g.Appid, Name: g.Name, MyPlaytime: g.PlaytimeForever})
					}
				}
				sort.Slice(out.MineOnly, func(i, j int) bool { return out.MineOnly[i].Name < out.MineOnly[j].Name })
			}
			if showAll || theirsOnly {
				for _, g := range theirs.Response.Games {
					if _, has := myIndex[g.Appid]; !has {
						out.TheirsOnly = append(out.TheirsOnly, libraryCompareRow{Appid: g.Appid, Name: g.Name, TheirPlaytime: g.PlaytimeForever})
					}
				}
				sort.Slice(out.TheirsOnly, func(i, j int) bool { return out.TheirsOnly[i].Name < out.TheirsOnly[j].Name })
			}
			if showAll || shared {
				for _, g := range mine.Response.Games {
					if t, has := theirIndex[g.Appid]; has {
						out.Shared = append(out.Shared, libraryCompareRow{
							Appid: g.Appid, Name: g.Name,
							MyPlaytime: g.PlaytimeForever, TheirPlaytime: t.PlaytimeForever,
							PlaytimeDeltaHours: hoursOf(g.PlaytimeForever - t.PlaytimeForever),
						})
					}
				}
				sort.Slice(out.Shared, func(i, j int) bool {
					return abs(out.Shared[i].MyPlaytime-out.Shared[i].TheirPlaytime) > abs(out.Shared[j].MyPlaytime-out.Shared[j].TheirPlaytime)
				})
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&mySteamID, "my-steamid", "", "Your Steam ID 64 or vanity URL")
	cmd.Flags().StringVar(&theirSteamID, "steamid", "", "The other player's Steam ID 64 or vanity URL")
	cmd.Flags().BoolVar(&mineOnly, "mine-only", false, "Show only games in my library that aren't in theirs")
	cmd.Flags().BoolVar(&theirsOnly, "theirs-only", false, "Show only games in their library that aren't in mine")
	cmd.Flags().BoolVar(&shared, "shared", false, "Show only games in both libraries with playtime delta")
	return cmd
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

var _ = json.Marshal
