// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// pp:data-source auto
func newAskCmd(flags *rootFlags) *cobra.Command {
	var model string
	var seed int64
	var showThinking bool
	var dbPath string
	cmd := &cobra.Command{
		Use:         "ask <question>",
		Short:       "Ask Mixlayer and save the prompt, answer, and reasoning locally",
		Example:     `  mixlayer-pp-cli ask "Draft a migration plan" --model qwen/qwen3.5-27b --show-thinking --json`,
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
			run, err := chatAndSave(cmd.Context(), flags, s, "ask", args[0], model, seed, showThinking)
			if err != nil {
				return err
			}
			if flags.asJSON || flags.agent {
				return outputJSON(cmd, run)
			}
			if showThinking && run.Reasoning != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\n\n", run.Reasoning)
			}
			fmt.Fprintln(cmd.OutOrStdout(), run.Answer)
			fmt.Fprintf(cmd.ErrOrStderr(), "saved run %s\n", run.ID)
			return nil
		},
	}
	cmd.Flags().StringVar(&model, "model", defaultFrontierModel, "Model to use")
	cmd.Flags().Int64Var(&seed, "seed", 0, "Best-effort deterministic seed")
	cmd.Flags().BoolVar(&showThinking, "show-thinking", false, "Request and display reasoning_content")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path")
	return cmd
}

func newReplayCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:         "replay <run-id>",
		Short:       "Re-run a saved prompt with its original model and seed",
		Example:     `  mixlayer-pp-cli replay run_abc123 --json`,
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
			orig, err := s.GetRun(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			run, err := chatAndSave(cmd.Context(), flags, s, "replay", orig.Prompt, orig.Model, orig.Seed, orig.Reasoning != "")
			if err != nil {
				return err
			}
			return outputJSON(cmd, map[string]any{"original": orig, "replay": run, "same_answer": orig.Answer == run.Answer})
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path")
	return cmd
}

func newGrepCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int
	cmd := &cobra.Command{
		Use:         "grep <pattern>",
		Short:       "Search saved prompts, answers, and reasoning",
		Example:     `  mixlayer-pp-cli grep "migration" --limit 10 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			s, err := openMixStore(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer s.Close()
			results, err := s.SearchRuns(cmd.Context(), args[0], limit)
			if err != nil {
				return err
			}
			return outputJSON(cmd, results)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum results")
	return cmd
}
