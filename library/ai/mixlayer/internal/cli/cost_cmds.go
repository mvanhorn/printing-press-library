// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/ai/mixlayer/internal/pricing"
	"github.com/spf13/cobra"
)

// pp:data-source auto
func newSavingsCmd(flags *rootFlags) *cobra.Command {
	var dbPath, vs string
	cmd := &cobra.Command{
		Use:         "savings",
		Short:       "Roll up Mixlayer ledger cost against a frontier baseline",
		Example:     `  mixlayer-pp-cli savings --vs gpt-frontier --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openMixStore(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer s.Close()
			runs, err := s.RecentRuns(cmd.Context(), 10000)
			if err != nil {
				return err
			}
			var actual, baseline float64
			var tokens int
			for _, r := range runs {
				actual += r.CostUSD
				tokens += r.TotalTokens
				baseline += pricing.Baseline(vs, r.TotalTokens)
			}
			return outputJSON(cmd, map[string]any{"runs": len(runs), "tokens": tokens, "actual_usd": actual, "baseline_usd": baseline, "saved_usd": baseline - actual, "baseline": vs})
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path")
	cmd.Flags().StringVar(&vs, "vs", "gpt-frontier", "Baseline: gpt-frontier or claude-frontier")
	return cmd
}

func newCompareCmd(flags *rootFlags) *cobra.Command {
	var cheap, frontier, dbPath string
	cmd := &cobra.Command{
		Use:         "compare <question>",
		Short:       "Compare a cheap rung with the frontier and price the delta",
		Example:     `  mixlayer-pp-cli compare "Summarize this incident" --cheap qwen/qwen3.5-9b --json`,
		Annotations: map[string]string{"mcp:hidden": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			s, err := openMixStore(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer s.Close()
			cheapRun, err := chatAndSave(cmd.Context(), flags, s, "compare", args[0], cheap, 0, false)
			if err != nil {
				return err
			}
			frontierRun, err := chatAndSave(cmd.Context(), flags, s, "compare", args[0], frontier, 0, false)
			if err != nil {
				return err
			}
			note := "No prior ladder history found for an evidence-grounded quality rate."
			history, _ := s.SearchRuns(cmd.Context(), args[0], 50)
			if len(history) > 2 {
				note = fmt.Sprintf("Evidence note: %d related saved runs exist in your local ledger for this prompt pattern.", len(history))
			}
			return outputJSON(cmd, map[string]any{
				"cheap": cheapRun, "frontier": frontierRun,
				"delta_usd":    frontierRun.CostUSD - cheapRun.CostUSD,
				"quality_note": note,
			})
		},
	}
	cmd.Flags().StringVar(&cheap, "cheap", "qwen/qwen3.5-9b", "Cheap rung")
	cmd.Flags().StringVar(&frontier, "frontier", defaultFrontierModel, "Frontier rung")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path")
	return cmd
}
