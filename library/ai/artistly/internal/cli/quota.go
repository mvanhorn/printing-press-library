// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored. `quota` reports the account's generation budget (read from the
// Inertia shared props Artistly embeds in every authenticated page) and, with
// --for, previews whether a planned batch fits before you start burning it.
//
// pp:data-source live

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newNovelQuotaCmd(flags *rootFlags) *cobra.Command {
	var forFile string
	var quantity int

	cmd := &cobra.Command{
		Use:   "quota",
		Short: "Show remaining generation budget and preview whether a planned batch fits.",
		Long: trimLong(`
Report your Artistly generation budget: today's design count and the concurrent
generation limit, read from the authenticated dashboard. With --for <file> and
--quantity, preview a planned batch's total generation count and whether a
single prompt's images fit the concurrent limit (batch submits one prompt at a
time, so the concurrent limit bounds each step, not the whole run).

Note: Artistly enforces an undocumented ~400 generations/day cap where even
failed generations count. This command surfaces the counts the app exposes; it
cannot read the hidden daily ceiling directly.`),
		Example:     "  artistly-pp-cli quota --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			q, err := fetchQuota(ctx, c)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			result := map[string]any{
				"todays_design_count":         q.TodaysCount,
				"concurrent_generation_count": q.ConcurrentCount,
				"concurrent_generation_limit": q.ConcurrentLimit,
			}

			if forFile != "" {
				prompts, perr := readPromptFile(forFile)
				if perr != nil {
					return perr
				}
				qty := quantity
				if qty <= 0 {
					qty = 1
				}
				needed := len(prompts) * qty
				// batch submits one prompt at a time and paces itself between
				// prompts, so the concurrent limit bounds a single submission's
				// image quantity, not the whole run's total volume. Total volume
				// is bounded by the (app-hidden) daily cap instead. Report whether
				// one prompt's images fit the remaining concurrent slots.
				fitsConcurrent := q.ConcurrentLimit == 0 || q.ConcurrentCount+qty <= q.ConcurrentLimit
				result["planned_prompts"] = len(prompts)
				result["planned_quantity"] = qty
				result["planned_generations"] = needed
				result["fits_concurrent_limit"] = fitsConcurrent
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Today's generations: %d\n", q.TodaysCount)
			fmt.Fprintf(cmd.OutOrStdout(), "Concurrent: %d in flight, limit %d\n", q.ConcurrentCount, q.ConcurrentLimit)
			if forFile != "" {
				needed := result["planned_generations"].(int)
				plannedQty := result["planned_quantity"].(int)
				fmt.Fprintf(cmd.OutOrStdout(), "Planned batch: %d prompts x %d = %d generations (submitted one prompt at a time)\n",
					result["planned_prompts"], plannedQty, needed)
				if fits, _ := result["fits_concurrent_limit"].(bool); fits {
					fmt.Fprintln(cmd.OutOrStdout(), "Each prompt's images fit the concurrent limit; the run paces itself between prompts.")
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "Warning: a single prompt's %d image(s) would exceed the concurrent limit of %d.\n", plannedQty, q.ConcurrentLimit)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&forFile, "for", "", "Prompt file to preview against the budget")
	cmd.Flags().IntVar(&quantity, "quantity", 1, "Images per prompt when previewing --for")
	return cmd
}
