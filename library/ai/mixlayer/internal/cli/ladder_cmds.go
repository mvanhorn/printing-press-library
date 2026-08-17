// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/ai/mixlayer/internal/ladder"
	"github.com/mvanhorn/printing-press-library/library/ai/mixlayer/internal/pricing"
	"github.com/mvanhorn/printing-press-library/library/ai/mixlayer/internal/store"
	"github.com/spf13/cobra"
)

// pp:data-source live
func newLadderCmd(flags *rootFlags) *cobra.Command {
	var rungsSpec, dbPath string
	var reasoning bool
	var seed int64
	cmd := &cobra.Command{
		Use:         "ladder <question>",
		Short:       "Run one prompt across selected Mixlayer model rungs",
		Example:     `  mixlayer-pp-cli ladder "Which option is cheapest?" --reasoning --json`,
		Annotations: map[string]string{"mcp:hidden": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			var seedPtr *int64
			if seed != 0 {
				seedPtr = &seed
			}
			rungs := ladder.Rungs(rungsSpec)
			results := ladder.AskAcross(cmd.Context(), c, args[0], rungs, reasoning, seedPtr)
			s, err := openMixStore(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer s.Close()
			groupID := store.NewID("ladder")
			if err := saveLadderGroup(cmd.Context(), s, "ladder", groupID, args[0], rungs, ladder.FirstConfident(results), "", seed, results); err != nil {
				return err
			}
			return outputJSON(cmd, map[string]any{"results": results, "first_confident_model": ladder.FirstConfident(results)})
		},
	}
	cmd.Flags().StringVar(&rungsSpec, "rungs", "all", "Comma-separated model rungs or all")
	cmd.Flags().BoolVar(&reasoning, "reasoning", false, "Request reasoning_content and compare it across rungs")
	cmd.Flags().Int64Var(&seed, "seed", 0, "Best-effort deterministic seed")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path")
	return cmd
}

func newEscalateCmd(flags *rootFlags) *cobra.Command {
	var confidence float64
	var rungsSpec, dbPath string
	cmd := &cobra.Command{
		Use:         "escalate <question>",
		Short:       "Climb the model ladder only until a rung is confident enough",
		Example:     `  mixlayer-pp-cli escalate "Classify these tickets" --confidence 0.85 --json`,
		Annotations: map[string]string{"mcp:hidden": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			configuredRungs := ladder.Rungs(rungsSpec)
			var results []ladder.Result
			for _, rung := range configuredRungs {
				part := ladder.AskAcross(cmd.Context(), c, args[0]+"\n\nReturn a concise answer. End with exactly one line: CONFIDENCE: high, medium, or low. Use low or medium when uncertain.", []string{rung}, false, nil)
				results = append(results, part...)
				if len(part) > 0 && part[0].Error == "" && heuristicConfidence(part[0].Answer) >= confidence {
					break
				}
			}
			total := 0.0
			for _, r := range results {
				total += r.CostUSD
			}
			triedRungs, escalationModel := escalationLedgerMetadata(configuredRungs, results)
			s, err := openMixStore(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer s.Close()
			groupID := store.NewID("ladder")
			if err := saveLadderGroup(cmd.Context(), s, "escalate", groupID, args[0], triedRungs, escalationModel, "", 0, results); err != nil {
				return err
			}
			baseline := pricing.Estimate("qwen/qwen3.5-397b-a17b", 1000, 1000)
			return outputJSON(cmd, map[string]any{"group_id": groupID, "results": results, "cost_usd": total, "frontier_baseline_usd": baseline, "saved_usd": baseline - total})
		},
	}
	cmd.Flags().Float64Var(&confidence, "confidence", 0.85, "Confidence threshold")
	cmd.Flags().StringVar(&rungsSpec, "rungs", "all", "Comma-separated model rungs or all")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path")
	return cmd
}

func escalationLedgerMetadata(configuredRungs []string, results []ladder.Result) ([]string, string) {
	triedCount := len(results)
	if triedCount > len(configuredRungs) {
		triedCount = len(configuredRungs)
	}
	triedRungs := append([]string(nil), configuredRungs[:triedCount]...)
	if len(results) == 0 {
		return triedRungs, ""
	}
	return triedRungs, results[len(results)-1].Model
}

func newCouncilCmd(flags *rootFlags) *cobra.Command {
	var membersSpec, judge, dbPath string
	cmd := &cobra.Command{
		Use:         "council <question>",
		Short:       "Fan out to several rungs and ask a judge model to synthesize",
		Example:     `  mixlayer-pp-cli council "Pick the safest launch plan" --json`,
		Annotations: map[string]string{"mcp:hidden": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			results := ladder.AskAcross(cmd.Context(), c, args[0], ladder.Rungs(membersSpec), true, nil)
			var b strings.Builder
			fmt.Fprintf(&b, "Question: %s\n\nSynthesize these model answers and flag disagreements:\n", args[0])
			for i, r := range results {
				appendCouncilMemberPrompt(&b, i+1, r)
			}
			judgeResult := ladder.AskAcross(cmd.Context(), c, b.String(), []string{judge}, false, nil)
			s, err := openMixStore(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer s.Close()
			groupID := store.NewID("ladder")
			members := ladder.Rungs(membersSpec)
			if err := s.SaveLadder(cmd.Context(), groupID, args[0], append(append([]string{}, members...), judge), "", judge); err != nil {
				return err
			}
			if err := saveLadderRuns(cmd.Context(), s, "council", groupID, args[0], 0, results); err != nil {
				return err
			}
			if err := saveLadderRuns(cmd.Context(), s, "council-judge", groupID, b.String(), 0, judgeResult); err != nil {
				return err
			}
			return outputJSON(cmd, map[string]any{"group_id": groupID, "members": results, "judge": judgeResult})
		},
	}
	cmd.Flags().StringVar(&membersSpec, "members", "qwen/qwen3.5-4b-free,qwen/qwen3.5-27b,qwen/qwen3.5-397b-a17b", "Comma-separated member models")
	cmd.Flags().StringVar(&judge, "judge", defaultFrontierModel, "Judge model")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path")
	return cmd
}

func saveLadderGroup(ctx context.Context, s *store.Store, command, groupID, prompt string, rungs []string, firstConfident, judge string, seed int64, results []ladder.Result) error {
	if err := s.SaveLadder(ctx, groupID, prompt, rungs, firstConfident, judge); err != nil {
		return err
	}
	return saveLadderRuns(ctx, s, command, groupID, prompt, seed, results)
}

func saveLadderRuns(ctx context.Context, s *store.Store, command, groupID, prompt string, seed int64, results []ladder.Result) error {
	for _, res := range results {
		raw, _ := json.Marshal(res)
		if err := s.SaveRun(ctx, store.RunRecord{
			ID:               store.NewID("run"),
			GroupID:          groupID,
			Command:          command,
			Prompt:           prompt,
			Answer:           res.Answer,
			Reasoning:        res.Reasoning,
			Model:            res.Model,
			Seed:             seed,
			RawJSON:          raw,
			PromptTokens:     res.PromptTokens,
			CompletionTokens: res.CompletionTokens,
			TotalTokens:      res.TotalTokens,
			CostUSD:          res.CostUSD,
			LatencyMS:        res.LatencyMS,
		}); err != nil {
			return err
		}
	}
	return nil
}

func appendCouncilMemberPrompt(b *strings.Builder, index int, r ladder.Result) {
	payload := struct {
		Model     string `json:"model"`
		Reasoning string `json:"reasoning,omitempty"`
		Answer    string `json:"answer"`
	}{
		Model:     r.Model,
		Reasoning: r.Reasoning,
		Answer:    r.Answer,
	}
	encoded, _ := json.Marshal(payload)
	fmt.Fprintf(b, "\n---BEGIN MEMBER %d---\n%s\n---END MEMBER %d---\n", index, encoded, index)
}

func heuristicConfidence(answer string) float64 {
	lower := strings.ToLower(answer)
	if strings.TrimSpace(answer) == "" {
		return 0
	}
	if strings.Contains(lower, "not sure") || strings.Contains(lower, "uncertain") || strings.Contains(lower, "cannot") || strings.Contains(lower, "can't determine") {
		return 0.4
	}
	if confidenceLineRe.MatchString(answer) {
		return 0.9
	}
	if mediumConfidenceLineRe.MatchString(answer) {
		return 0.7
	}
	if lowConfidenceLineRe.MatchString(answer) {
		return 0.4
	}
	return 0.65
}

var (
	confidenceLineRe       = regexp.MustCompile(`(?im)^\s*confidence\s*:\s*high\s*$`)
	mediumConfidenceLineRe = regexp.MustCompile(`(?im)^\s*confidence\s*:\s*medium\s*$`)
	lowConfidenceLineRe    = regexp.MustCompile(`(?im)^\s*confidence\s*:\s*low\s*$`)
)
