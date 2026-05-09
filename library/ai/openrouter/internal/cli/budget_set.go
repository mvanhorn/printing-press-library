// Copyright 2026 rick-van-de-laar. Licensed under Apache-2.0. See LICENSE.

package cli

// PATCH transcendence-commands: hand-built — write per-cron USD/week budget.

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newBudgetSetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "set <cron-name> <usd-per-week>",
		Short:   "Set a weekly USD budget for a named cron job",
		Example: "  openrouter-pp-cli budget set scan-pipeline 2usd",
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			amount, err := parseBudgetUSD(args[1])
			if err != nil {
				return usageErr(fmt.Errorf("bad amount %q: %w", args[1], err))
			}
			budgets, err := loadBudgets()
			if err != nil {
				return apiErr(err)
			}
			budgets[name] = amount
			if err := saveBudgets(budgets); err != nil {
				return apiErr(err)
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"cron": name, "budget_usd_per_week": amount}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "set budget for %s = $%.2f/week\n", name, amount)
			return nil
		},
	}
	return cmd
}
