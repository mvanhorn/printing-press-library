// Copyright 2026 prisco-faiella. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/diretta/internal/parser"
	"github.com/spf13/cobra"
)

func newInsightsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "insights <matchId>",
		Short: "Derive analytical insights for a match: H2H win rates, BTTS, over/under, form signals.",
		Long: `Fetches H2H history and match detail for a given match ID, then computes
derived analytics beyond the raw feed data:

  • head-to-head win/draw/loss percentages
  • both-teams-scored rate (BTTS)
  • over-2.5-goals rate
  • average goals per H2H game
  • per-team last-5 form string
  • plain-language signals (e.g. "home_dominant", "high_scoring_h2h", "btts_likely")
  • confidence level based on H2H sample size`,
		Example:     "  diretta-pp-cli insights vVn0EQM5\n  diretta-pp-cli insights vVn0EQM5 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return usageErr(fmt.Errorf("matchId is required\nUsage: %s <matchId>", cmd.CommandPath()))
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			matchID := args[0]

			// Fetch H2H
			h2hPath := replacePathParam("/x/feed/df_hh_1_{matchId}", "matchId", matchID)
			h2hRaw, prov, err := resolveRead(cmd.Context(), c, flags, "match", false, h2hPath, map[string]string{}, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			h2h := parser.ParseH2H([]byte(h2hRaw))

			// Fetch match detail to get team names and current status
			detailPath := replacePathParam("/x/feed/dc_1_{matchId}", "matchId", matchID)
			detailRaw, _, detailErr := resolveRead(cmd.Context(), c, flags, "match", false, detailPath, map[string]string{}, nil)
			detail := map[string]any{}
			if detailErr == nil {
				detail = parser.ParseMatchDetail([]byte(detailRaw))
			}

			homeTeam, _ := detail["home_team"].(string)
			awayTeam, _ := detail["away_team"].(string)
			// Fall back to first H2H record if detail missing
			if homeTeam == "" && len(h2h) > 0 {
				homeTeam, _ = h2h[0]["home_team"].(string)
				awayTeam, _ = h2h[0]["away_team"].(string)
			}

			result := computeInsights(matchID, homeTeam, awayTeam, h2h)
			data, _ := json.Marshal(result)
			jdata := json.RawMessage(data)

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				printProvenance(cmd, 1, prov)
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				wrapped, wrapErr := wrapWithProvenance(jdata, prov)
				if wrapErr != nil {
					return wrapErr
				}
				return printOutput(cmd.OutOrStdout(), wrapped, true)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), jdata, flags)
		},
	}
	return cmd
}

type matchInsights struct {
	MatchID    string `json:"match_id"`
	HomeTeam   string `json:"home_team"`
	AwayTeam   string `json:"away_team"`
	Confidence string `json:"confidence"`

	// H2H aggregate stats
	H2HTotalGames int     `json:"h2h_total_games"`
	HomeWinPct    float64 `json:"home_win_pct"`
	DrawPct       float64 `json:"draw_pct"`
	AwayWinPct    float64 `json:"away_win_pct"`
	BTTSPct       float64 `json:"btts_pct"`
	AvgGoals      float64 `json:"avg_goals_per_game"`
	Over25Pct     float64 `json:"over_2_5_pct"`

	// Form (last 5 in the H2H set for each team)
	HomeForm string `json:"home_form_last5"`
	AwayForm string `json:"away_form_last5"`

	// Plain-language signals
	Signals []string `json:"signals"`
}

func computeInsights(matchID, homeTeam, awayTeam string, h2h []map[string]any) matchInsights {
	ins := matchInsights{
		MatchID:  matchID,
		HomeTeam: homeTeam,
		AwayTeam: awayTeam,
		Signals:  []string{},
	}

	total := len(h2h)
	if total == 0 {
		ins.Confidence = "none"
		ins.Signals = append(ins.Signals, "no_h2h_data")
		return ins
	}

	var homeWins, awayWins, draws int
	var totalGoals, btts, over25 int
	homeFormBuf := make([]byte, 0, 5)
	awayFormBuf := make([]byte, 0, 5)
	homeFormCount, awayFormCount := 0, 0

	for i, m := range h2h {
		ht, _ := m["home_team"].(string)
		hs := toAnyInt(m["home_score"])
		as := toAnyInt(m["away_score"])
		goals := hs + as
		totalGoals += goals

		if hs > 0 && as > 0 {
			btts++
		}
		if goals > 2 {
			over25++
		}

		// Win/draw/loss from perspective of homeTeam in the upcoming match
		if ht == homeTeam {
			if hs > as {
				homeWins++
			} else if as > hs {
				awayWins++
			} else {
				draws++
			}
		} else {
			if as > hs {
				homeWins++
			} else if hs > as {
				awayWins++
			} else {
				draws++
			}
		}

		// Last 5 form for each team (most recent = index 0)
		if i < 5 {
			if ht == homeTeam {
				if hs > as {
					homeFormBuf = append(homeFormBuf, 'W')
				} else if hs < as {
					homeFormBuf = append(homeFormBuf, 'L')
				} else {
					homeFormBuf = append(homeFormBuf, 'D')
				}
				homeFormCount++
				// away team perspective
				if as > hs {
					awayFormBuf = append(awayFormBuf, 'W')
				} else if as < hs {
					awayFormBuf = append(awayFormBuf, 'L')
				} else {
					awayFormBuf = append(awayFormBuf, 'D')
				}
				awayFormCount++
			} else {
				if as > hs {
					homeFormBuf = append(homeFormBuf, 'W')
				} else if as < hs {
					homeFormBuf = append(homeFormBuf, 'L')
				} else {
					homeFormBuf = append(homeFormBuf, 'D')
				}
				homeFormCount++
				if hs > as {
					awayFormBuf = append(awayFormBuf, 'W')
				} else if hs < as {
					awayFormBuf = append(awayFormBuf, 'L')
				} else {
					awayFormBuf = append(awayFormBuf, 'D')
				}
				awayFormCount++
			}
		}
	}

	_ = homeFormCount
	_ = awayFormCount

	pct := func(n int) float64 {
		return math.Round(float64(n)*1000/float64(total)) / 10
	}

	ins.H2HTotalGames = total
	ins.HomeWinPct = pct(homeWins)
	ins.DrawPct = pct(draws)
	ins.AwayWinPct = pct(awayWins)
	ins.BTTSPct = pct(btts)
	ins.AvgGoals = math.Round(float64(totalGoals)/float64(total)*10) / 10
	ins.Over25Pct = pct(over25)
	ins.HomeForm = string(homeFormBuf)
	ins.AwayForm = string(awayFormBuf)

	// Confidence based on sample size
	switch {
	case total >= 10:
		ins.Confidence = "high"
	case total >= 5:
		ins.Confidence = "medium"
	default:
		ins.Confidence = "low"
	}

	// Derive signals
	if ins.HomeWinPct >= 55 {
		ins.Signals = append(ins.Signals, "home_dominant")
	} else if ins.AwayWinPct >= 55 {
		ins.Signals = append(ins.Signals, "away_dominant")
	} else if ins.DrawPct >= 35 {
		ins.Signals = append(ins.Signals, "draw_prone")
	}

	if ins.BTTSPct >= 60 {
		ins.Signals = append(ins.Signals, "btts_likely")
	} else if ins.BTTSPct <= 30 {
		ins.Signals = append(ins.Signals, "clean_sheet_likely")
	}

	if ins.Over25Pct >= 60 {
		ins.Signals = append(ins.Signals, "high_scoring_h2h")
	} else if ins.Over25Pct <= 30 {
		ins.Signals = append(ins.Signals, "low_scoring_h2h")
	}

	if ins.AvgGoals >= 3.0 {
		ins.Signals = append(ins.Signals, "over_2_5_favored")
	} else if ins.AvgGoals <= 1.5 {
		ins.Signals = append(ins.Signals, "under_2_5_favored")
	}

	// Home form streak
	if len(homeFormBuf) >= 3 {
		last3 := string(homeFormBuf[:3])
		if last3 == "WWW" {
			ins.Signals = append(ins.Signals, "home_on_win_streak")
		} else if last3 == "LLL" {
			ins.Signals = append(ins.Signals, "home_on_loss_streak")
		}
	}
	if len(awayFormBuf) >= 3 {
		last3 := string(awayFormBuf[:3])
		if last3 == "WWW" {
			ins.Signals = append(ins.Signals, "away_on_win_streak")
		} else if last3 == "LLL" {
			ins.Signals = append(ins.Signals, "away_on_loss_streak")
		}
	}

	if len(ins.Signals) == 0 {
		ins.Signals = append(ins.Signals, "evenly_matched")
	}

	return ins
}
