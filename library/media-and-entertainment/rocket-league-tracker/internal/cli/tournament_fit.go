// Copyright 2026 addisonk. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/spf13/cobra"
)

// tierBracketKeywords maps short bracket tokens to canonical tier names so a
// tournament's "bracket": "Plat" string can be matched against the player's
// "Platinum 1" tier. We pick the lower bracket for each prefix to be inclusive.
var tierBracketKeywords = map[string]string{
	"bronze":            "Bronze 1",
	"silver":            "Silver 1",
	"gold":              "Gold 1",
	"plat":              "Platinum 1",
	"platinum":          "Platinum 1",
	"diamond":           "Diamond 1",
	"champ":             "Champion 1",
	"champion":          "Champion 1",
	"gc":                "Grand Champion 1",
	"grand champion":    "Grand Champion 1",
	"grandchampion":     "Grand Champion 1",
	"ssl":               "Supersonic Legend",
	"supersonic legend": "Supersonic Legend",
}

// newTournamentFitCmd implements `tournament-fit <player>` — calls
// /ranks/{playerId} and /tournaments concurrently, then filters tournaments
// whose skill bracket includes the player's MMR for the relevant playlist.
func newTournamentFitCmd(flags *rootFlags) *cobra.Command {
	var platform string
	cmd := &cobra.Command{
		Use:         "tournament-fit <player>",
		Short:       "Tournaments whose skill bracket matches the player's current MMR.",
		Long:        "Concurrently fetches /ranks/{playerId} and /tournaments, infers each tournament's tier bracket from a `bracket`/`tier` field, and emits the subset that include the player's current MMR for any playlist.",
		Example:     "  rocket-league-tracker-pp-cli tournament-fit 550e8400-e29b-41d4-a716-446655440000",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			player := args[0]
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			var (
				wg                 sync.WaitGroup
				ranksRaw, tournRaw json.RawMessage
				ranksErr, tournErr error
			)
			wg.Add(2)
			go func() {
				defer wg.Done()
				ranksRaw, ranksErr = c.Get("/ranks/"+pathEscapePlayer(player), nil)
			}()
			go func() {
				defer wg.Done()
				tournRaw, tournErr = c.Get("/tournaments", nil)
			}()
			wg.Wait()
			if ranksErr != nil {
				return classifyAPIError(ranksErr, flags)
			}
			if tournErr != nil {
				return classifyAPIError(tournErr, flags)
			}

			snaps := extractRankPlaylists(extractResponseData(ranksRaw))
			byPlaylist := map[string]rankSnapshot{}
			for _, s := range snaps {
				byPlaylist[strings.ToLower(s.Playlist)] = s
			}

			tournData := extractResponseData(tournRaw)
			var tournaments []map[string]any
			if err := json.Unmarshal(tournData, &tournaments); err != nil {
				var obj map[string]json.RawMessage
				if json.Unmarshal(tournData, &obj) == nil {
					for _, k := range []string{"tournaments", "items", "results", "data"} {
						if raw, ok := obj[k]; ok {
							_ = json.Unmarshal(raw, &tournaments)
							break
						}
					}
				}
			}

			type fit struct {
				Tournament  map[string]any `json:"tournament"`
				MatchedTier string         `json:"matched_tier"`
				Playlist    string         `json:"matched_playlist,omitempty"`
				PlayerMMR   int            `json:"player_mmr,omitempty"`
			}
			out := []fit{}
			for _, t := range tournaments {
				tierName := tournamentTier(t)
				if tierName == "" {
					continue
				}
				bracketLower := tierLowerBound(tierName)
				// Match against the highest-MMR playlist that's >= bracketLower.
				bestMMR := 0
				bestPlaylist := ""
				for k, s := range byPlaylist {
					if s.MMR >= bracketLower && s.MMR > bestMMR {
						bestMMR = s.MMR
						bestPlaylist = k
					}
				}
				if bestPlaylist == "" {
					continue
				}
				out = append(out, fit{
					Tournament:  t,
					MatchedTier: tierName,
					Playlist:    bestPlaylist,
					PlayerMMR:   bestMMR,
				})
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no tournaments matched the player's MMR")
				return nil
			}
			for _, f := range out {
				name := firstStr(f.Tournament, "name", "title", "id")
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t(%s @ %d MMR)\n", name, f.MatchedTier, f.Playlist, f.PlayerMMR)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&platform, "platform", "epic", "Platform: epic, steam, psn, xbox")
	_ = platform
	return cmd
}

// tournamentTier inspects a tournament object for any field that looks like a
// tier bracket (bracket, tier, skill, division, rank) and matches it against
// known tier keywords. Returns the canonical tier name or "" when unknown.
func tournamentTier(t map[string]any) string {
	candidates := []string{"bracket", "tier", "skill", "division", "rank", "level"}
	for _, k := range candidates {
		if v, ok := t[k]; ok {
			if s, ok := v.(string); ok && s != "" {
				low := strings.ToLower(s)
				for kw, canonical := range tierBracketKeywords {
					if strings.Contains(low, kw) {
						return canonical
					}
				}
				if c, ok := resolveTier(s); ok {
					return c
				}
			}
		}
	}
	return ""
}
