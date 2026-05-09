// Copyright 2026 rick-van-de-laar. Licensed under Apache-2.0. See LICENSE.

package cli

// PATCH transcendence-commands: hand-built — exit 0 (under) / 8 (over). Honors --dry-run.

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

func newBudgetCheckCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "check <cron-name>",
		Short:       "Check spend against budget; exit 0 (under) or 8 (over)",
		Example:     "  openrouter-pp-cli budget check scan-pipeline",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			name := args[0]
			budgets, err := loadBudgets()
			if err != nil {
				return apiErr(err)
			}
			budget, ok := budgets[name]
			if !ok {
				return notFoundErr(fmt.Errorf("no budget set for %q (run 'openrouter-pp-cli budget set %s <usd>')", name, name))
			}
			since := time.Now().Add(-7 * 24 * time.Hour)
			entries, err := loadToolCallEntries(since)
			if err != nil {
				return apiErr(err)
			}
			actual := 0.0
			for _, e := range entries {
				if e.groupValue("cron") == name {
					actual += e.cost()
				}
			}
			over := actual > budget
			result := map[string]any{
				"cron":      name,
				"budget":    budget,
				"actual":    actual,
				"remaining": budget - actual,
				"over":      over,
			}
			if flags.asJSON {
				if err := printJSONFiltered(cmd.OutOrStdout(), result, flags); err != nil {
					return err
				}
			} else {
				fmt.Fprintf(cmd.OutOrStdout(),
					"cron=%s budget=$%.4f actual=$%.4f remaining=$%.4f over=%v\n",
					name, budget, actual, budget-actual, over)
			}
			if over {
				return &cliError{code: 8, err: fmt.Errorf("over budget")}
			}
			return nil
		},
	}
	return cmd
}
