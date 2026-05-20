// Copyright 2026 prisco-faiella. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/diretta/internal/parser"
	"github.com/spf13/cobra"
)

func newTeamCompetitionsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "team-competitions <teamName>",
		Short: "List every active competition a team is playing in, with next fixture date.",
		Long: `Scans today's, yesterday's and tomorrow's match feeds to find all matches
for a given team name (case-insensitive substring match) and returns the
unique list of competitions the team appears in with the next scheduled match.`,
		Example:     "  diretta-pp-cli team-competitions \"Juventus\"\n  diretta-pp-cli team-competitions \"Inter\" --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return usageErr(fmt.Errorf("teamName is required\nUsage: %s <teamName>", cmd.CommandPath()))
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			teamName := args[0]
			paths := []string{
				"/x/feed/f_1_-1_3_it_1",
				"/x/feed/f_1_0_3_it_1",
				"/x/feed/f_1_1_3_it_1",
			}

			// competition_id -> competition info + next match
			type compEntry struct {
				TournamentID  string `json:"tournament_id"`
				Tournament    string `json:"tournament"`
				Country       string `json:"country"`
				NextMatchDate string `json:"next_match_date,omitempty"`
				NextMatchID   string `json:"next_match_id,omitempty"`
				Opponent      string `json:"opponent,omitempty"`
			}
			comps := map[string]*compEntry{}

			for _, path := range paths {
				raw, _, ferr := resolveRead(cmd.Context(), c, flags, "matches", false, path, map[string]string{}, nil)
				if ferr != nil {
					continue
				}
				for _, m := range parser.ParseMatches([]byte(raw)) {
					ht, _ := m["home_team"].(string)
					at, _ := m["away_team"].(string)

					teamMatches := false
					var opponent string
					if containsCI(ht, teamName) {
						teamMatches = true
						opponent = at
					} else if containsCI(at, teamName) {
						teamMatches = true
						opponent = ht
					}

					if !teamMatches {
						continue
					}

					tid, _ := m["tournament_id"].(string)
					if tid == "" {
						continue
					}
					date, _ := m["date"].(string)
					id, _ := m["id"].(string)

					if _, exists := comps[tid]; !exists {
						comps[tid] = &compEntry{
							TournamentID: tid,
							Tournament:   fmt.Sprintf("%v", m["tournament"]),
							Country:      fmt.Sprintf("%v", m["country"]),
						}
					}
					// Update next match if date is newer or not yet set
					ce := comps[tid]
					if ce.NextMatchDate == "" || date > ce.NextMatchDate {
						ce.NextMatchDate = date
						ce.NextMatchID = id
						ce.Opponent = opponent
					}
				}
			}

			var result []*compEntry
			for _, v := range comps {
				result = append(result, v)
			}

			if len(result) == 0 {
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"team":    teamName,
						"message": "No active competitions found in the ±1 day window.",
					}, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "No active competitions found for %q in the ±1 day window.\n", teamName)
				return nil
			}

			data, _ := json.Marshal(result)
			jdata := json.RawMessage(data)

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printOutput(cmd.OutOrStdout(), jdata, true)
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				var items []map[string]any
				if json.Unmarshal(jdata, &items) == nil && len(items) > 0 {
					return printAutoTable(cmd.OutOrStdout(), items)
				}
			}
			return printOutputWithFlags(cmd.OutOrStdout(), jdata, flags)
		},
	}
	return cmd
}

// containsCI does case-insensitive substring match.
func containsCI(s, sub string) bool {
	if len(sub) == 0 {
		return false
	}
	// simple fold comparison
	sl, subl := len(s), len(sub)
	for i := 0; i <= sl-subl; i++ {
		if foldEqual(s[i:i+subl], sub) {
			return true
		}
	}
	return false
}

func foldEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		ac, bc := a[i], b[i]
		if ac == bc {
			continue
		}
		// simple ASCII fold
		if ac >= 'A' && ac <= 'Z' {
			ac += 32
		}
		if bc >= 'A' && bc <= 'Z' {
			bc += 32
		}
		if ac != bc {
			return false
		}
	}
	return true
}
