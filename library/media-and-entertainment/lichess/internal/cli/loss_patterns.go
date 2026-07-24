// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves this managed safety implementation.

package cli

// pp:data-source live

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type lossPattern struct {
	Judgment    string `json:"judgment"`
	Opening     string `json:"opening,omitempty"`
	Performance string `json:"performance,omitempty"`
	Phase       string `json:"phase"`
	Count       int    `json:"count"`
}

type lossPatternReport struct {
	Username     string        `json:"username"`
	GamesScanned int           `json:"games_scanned"`
	Losses       int           `json:"losses"`
	Patterns     []lossPattern `json:"patterns"`
	Safety       string        `json:"safety"`
}

func newNovelLossPatternsCmd(flags *rootFlags) *cobra.Command {
	var maxGames int

	cmd := &cobra.Command{
		Use:         "loss-patterns [username]",
		Short:       "Mechanically group official completed-game judgments by opening, performance, and phase.",
		Example:     "  lichess-pp-cli loss-patterns a-named-player --max 30",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"username": args[0], "max_games": maxGames, "dry_run": true}, flags)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			report, err := collectLossPatterns(cmd, c, args[0], maxGames)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printJSONFiltered(cmd.OutOrStdout(), report, flags)
		},
	}
	cmd.Flags().IntVar(&maxGames, "max", 30, "Maximum completed games to inspect (1-50)")
	return cmd
}

func collectLossPatterns(cmd *cobra.Command, c interface {
	GetWithHeaders(context.Context, string, map[string]string, map[string]string) (json.RawMessage, error)
}, username string, maxGames int) (lossPatternReport, error) {
	if maxGames < 1 || maxGames > 50 {
		return lossPatternReport{}, usageErr(fmt.Errorf("--max must be between 1 and 50"))
	}
	data, err := c.GetWithHeaders(cmd.Context(), "/api/games/user/"+url.PathEscape(username), map[string]string{
		"max": fmt.Sprintf("%d", maxGames), "finished": "true", "ongoing": "false",
		"evals": "true", "opening": "true", "clocks": "true", "division": "true",
	}, map[string]string{"Accept": "application/x-ndjson"})
	if err != nil {
		return lossPatternReport{}, err
	}
	report := lossPatternReport{Username: username, Safety: "Completed games only; uses Lichess-provided judgments and never runs an engine or inspects ongoing games."}
	counts := map[string]int{}
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	// Analysis-rich NDJSON games can exceed Scanner's 64 KiB default token.
	// The command remains bounded to 50 completed games, so an 8 MiB line cap
	// preserves predictable memory use while accommodating large annotations.
	scanner.Buffer(make([]byte, 64*1024), 8*1024*1024)
	for scanner.Scan() {
		var game map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &game); err != nil {
			return lossPatternReport{}, fmt.Errorf("decode Lichess game stream: %w", err)
		}
		report.GamesScanned++
		color := playerColor(game, username)
		winner, _ := game["winner"].(string)
		if color == "" || winner == "" || winner == color {
			continue
		}
		report.Losses++
		opening, performance := gameLabel(game, "opening", "name"), stringValue(game["perf"])
		middle, end := division(game)
		analysis, _ := game["analysis"].([]any)
		for ply, entry := range analysis {
			if (ply%2 == 0 && color != "white") || (ply%2 == 1 && color != "black") {
				continue
			}
			move, _ := entry.(map[string]any)
			judgment, _ := move["judgment"].(map[string]any)
			kind, _ := judgment["name"].(string)
			if !isLossJudgment(kind) {
				continue
			}
			phase := "opening"
			if end > 0 && ply+1 >= end {
				phase = "endgame"
			} else if middle > 0 && ply+1 >= middle {
				phase = "middlegame"
			}
			counts[strings.Join([]string{kind, opening, performance, phase}, "\x00")]++
		}
	}
	if err := scanner.Err(); err != nil {
		return lossPatternReport{}, err
	}
	for key, count := range counts {
		parts := strings.Split(key, "\x00")
		report.Patterns = append(report.Patterns, lossPattern{Judgment: parts[0], Opening: parts[1], Performance: parts[2], Phase: parts[3], Count: count})
	}
	sort.Slice(report.Patterns, func(i, j int) bool {
		if report.Patterns[i].Count != report.Patterns[j].Count {
			return report.Patterns[i].Count > report.Patterns[j].Count
		}
		return report.Patterns[i].Judgment < report.Patterns[j].Judgment
	})
	return report, nil
}

func isLossJudgment(kind string) bool {
	switch kind {
	case "Inaccuracy", "Mistake", "Blunder":
		return true
	default:
		return false
	}
}

func playerColor(game map[string]any, username string) string {
	players, _ := game["players"].(map[string]any)
	for _, color := range []string{"white", "black"} {
		player, _ := players[color].(map[string]any)
		user, _ := player["user"].(map[string]any)
		id := strings.ToLower(stringValue(user["id"]))
		name := strings.ToLower(stringValue(user["name"]))
		if strings.EqualFold(username, id) || strings.EqualFold(username, name) {
			return color
		}
	}
	return ""
}

func gameLabel(game map[string]any, field, child string) string {
	value, _ := game[field].(map[string]any)
	return stringValue(value[child])
}

func division(game map[string]any) (int, int) {
	value, _ := game["division"].(map[string]any)
	return intValue(value["middle"]), intValue(value["end"])
}

func stringValue(value any) string {
	s, _ := value.(string)
	return s
}

func intValue(value any) int {
	n, _ := value.(float64)
	return int(n)
}
