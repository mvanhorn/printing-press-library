// Copyright 2026 github-actionsbot and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto

package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

type queueEstimateResult struct {
	QueueFile        string `json:"queue_file"`
	Shots            int    `json:"shots"`
	EstimatedCredits int    `json:"estimated_credits"`
	RemainingCredits *int   `json:"remaining_credits,omitempty"`
	PlanTier         string `json:"plan_tier,omitempty"`
	Fits             *bool  `json:"fits,omitempty"`
	Note             string `json:"note,omitempty"`
}

func newNovelQueueEstimateCmd(flags *rootFlags) *cobra.Command {
	var flagAPIKey string

	cmd := &cobra.Command{
		Use:   "estimate <queue-file>",
		Short: "See whether a prepared batch of generations fits your remaining Flow credits before you spend a single one.",
		Long: "Sums a prepared queue file's (from script draft-prompts / episode import) per-shot estimated_credits and, " +
			"when --api-key is given, checks it against your live Flow credit balance (GET /credits, the same call " +
			"`flow-pp-cli credits` makes). Without --api-key the check is local-only: the total is reported but not " +
			"compared against a live balance.",
		Example: "  flow-pp-cli queue estimate episode3-queue.json",
		// pp:happy-args: the matrix's generic positional-arg synthesis skips
		// a single non-id-shaped positional ("queue-file") rather than
		// guessing; declare the real shared root fixture explicitly so the
		// leaf-level happy path actually runs instead of being skipped.
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "queue-file=episode3-queue.json",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && !hasChangedLocalFlags(cmd) {
				return cmd.Help()
			}
			if len(args) == 0 {
				return usageErr(fmt.Errorf("queue estimate requires a queue file path; run %q for usage", cmd.CommandPath()+" --help"))
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, fmt.Sprintf("estimate credit cost of %s", args[0]))
			}

			raw, err := os.ReadFile(args[0])
			if err != nil {
				return usageErr(fmt.Errorf("reading queue file: %w", err))
			}
			var queue promptQueue
			if err := json.Unmarshal(raw, &queue); err != nil {
				return usageErr(fmt.Errorf("parsing queue file: %w", err))
			}

			result := queueEstimateResult{QueueFile: args[0], Shots: len(queue.Shots)}
			for _, s := range queue.Shots {
				result.EstimatedCredits += s.EstimatedCredits
			}

			if flagAPIKey == "" {
				result.Note = "pass --api-key to compare against your live credit balance (same value flow-pp-cli credits requires)"
			} else {
				ctx, cancel := boundCtx(cmd.Context(), flags)
				defer cancel()
				c, err := flags.newClient()
				if err != nil {
					return err
				}
				data, err := c.Get(ctx, "/credits", map[string]string{"key": flagAPIKey})
				if err != nil {
					return classifyAPIError(cmd.OutOrStdout(), err, flags)
				}
				var balance struct {
					RemainingCredits int    `json:"remainingCredits"`
					PlanTier         string `json:"planTier"`
				}
				if err := json.Unmarshal(data, &balance); err != nil {
					return apiErr(fmt.Errorf("parsing credit balance response: %w", err))
				}
				result.RemainingCredits = &balance.RemainingCredits
				result.PlanTier = balance.PlanTier
				fits := balance.RemainingCredits >= result.EstimatedCredits
				result.Fits = &fits
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "%s: %d shot(s), ~%d credits\n", args[0], result.Shots, result.EstimatedCredits)
				if result.RemainingCredits != nil {
					verdict := "fits"
					if !*result.Fits {
						verdict = "does NOT fit"
					}
					fmt.Fprintf(cmd.OutOrStdout(), "remaining balance: %d credits (%s plan) -- %s\n", *result.RemainingCredits, result.PlanTier, verdict)
				} else if result.Note != "" {
					fmt.Fprintln(cmd.OutOrStdout(), result.Note)
				}
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&flagAPIKey, "api-key", "", "Browser API key sent alongside the Bearer token (harvested) -- same value flow-pp-cli credits requires; omit to estimate offline")
	return cmd
}
