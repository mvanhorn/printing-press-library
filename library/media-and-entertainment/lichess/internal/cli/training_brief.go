// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves this managed safety implementation.

package cli

// pp:data-source live

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"

	"github.com/spf13/cobra"
)

type puzzleWeakness struct {
	Theme       string  `json:"theme"`
	Attempts    int     `json:"attempts"`
	Performance float64 `json:"performance"`
	FollowUp    string  `json:"follow_up"`
}

type trainingBriefReport struct {
	Days             int               `json:"days"`
	Username         string            `json:"username"`
	PuzzleWeaknesses []puzzleWeakness  `json:"puzzle_weaknesses"`
	LossEvidence     lossPatternReport `json:"loss_evidence"`
	Caveat           string            `json:"caveat"`
}

func newNovelTrainingBriefCmd(flags *rootFlags) *cobra.Command {
	var days int
	var maxGames int

	cmd := &cobra.Command{
		Use:         "training-brief",
		Short:       "Choose puzzle practice from official puzzle themes and completed-game loss evidence.",
		Example:     "  lichess-pp-cli training-brief --days 30 --max 30",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"days": days, "max_games": maxGames, "dry_run": true}, flags)
			}
			if days < 1 {
				return usageErr(fmt.Errorf("--days must be at least 1"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			accountData, err := c.Get(cmd.Context(), "/api/account", nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var account map[string]any
			if err := json.Unmarshal(accountData, &account); err != nil {
				return fmt.Errorf("decode account response: %w", err)
			}
			username := stringValue(account["id"])
			if username == "" {
				return fmt.Errorf("account response did not contain a user id")
			}
			losses, err := collectLossPatterns(cmd, c, username, maxGames)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			dashboardData, err := c.Get(cmd.Context(), "/api/puzzle/dashboard/"+strconv.Itoa(days), nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			weaknesses, err := lowestPuzzleThemes(dashboardData)
			if err != nil {
				return err
			}
			report := trainingBriefReport{
				Days: days, Username: username, PuzzleWeaknesses: weaknesses, LossEvidence: losses,
				Caveat: "Puzzle-theme performance and game-loss judgments are separate official evidence streams. This report does not claim a puzzle theme caused a game mistake.",
			}
			return printJSONFiltered(cmd.OutOrStdout(), report, flags)
		},
	}
	cmd.Flags().IntVar(&days, "days", 30, "Puzzle-dashboard lookback window in days")
	cmd.Flags().IntVar(&maxGames, "max", 30, "Maximum completed games to inspect for loss evidence (1-50)")
	return cmd
}

func lowestPuzzleThemes(data []byte) ([]puzzleWeakness, error) {
	var dashboard struct {
		Themes map[string]struct {
			Results struct {
				NB          int     `json:"nb"`
				Performance float64 `json:"performance"`
			} `json:"results"`
		} `json:"themes"`
	}
	if err := json.Unmarshal(data, &dashboard); err != nil {
		return nil, fmt.Errorf("decode puzzle dashboard: %w", err)
	}
	weaknesses := make([]puzzleWeakness, 0, len(dashboard.Themes))
	for theme, record := range dashboard.Themes {
		if record.Results.NB == 0 {
			continue
		}
		weaknesses = append(weaknesses, puzzleWeakness{
			Theme: theme, Attempts: record.Results.NB, Performance: record.Results.Performance,
		})
	}
	sort.Slice(weaknesses, func(i, j int) bool {
		if weaknesses[i].Performance != weaknesses[j].Performance {
			return weaknesses[i].Performance < weaknesses[j].Performance
		}
		return weaknesses[i].Theme < weaknesses[j].Theme
	})
	if len(weaknesses) > 3 {
		weaknesses = weaknesses[:3]
	}
	if len(weaknesses) > 0 {
		// A brief offers exactly one follow-up. The remaining ranked themes are
		// context, not additional puzzle-fetch instructions.
		weaknesses[0].FollowUp = "lichess-pp-cli puzzle next --angle " + weaknesses[0].Theme
	}
	return weaknesses, nil
}
