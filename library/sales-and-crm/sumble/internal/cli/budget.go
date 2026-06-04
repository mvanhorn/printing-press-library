package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

func newBudgetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "budget",
		Short: "Manage the per-call credit ceiling enforced by cost-estimate",
		Long: strings.Trim(`
Set a per-call credit ceiling. 'cost-estimate' checks each estimate against this
ceiling and exits non-zero when a call would exceed it, so an agent can stop
before overspending. The ceiling is stored locally in the SQLite store.
`, "\n"),
	}
	cmd.AddCommand(newBudgetSetCmd(flags))
	cmd.AddCommand(newBudgetShowCmd(flags))
	cmd.AddCommand(newBudgetClearCmd(flags))
	return cmd
}

func newBudgetSetCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "set <credits>",
		Short:   "Set the per-call credit ceiling",
		Example: "  sumble-pp-cli budget set 500",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			n, err := strconv.Atoi(args[0])
			if err != nil || n < 0 {
				return usageErr(fmt.Errorf("credits must be a non-negative integer, got %q", args[0]))
			}
			db, derr := openCreditStore()
			if derr != nil {
				return configErr(derr)
			}
			defer db.Close()
			if err := setBudget(db.DB(), n); err != nil {
				return apiErr(err)
			}
			if flags.asJSON {
				return flags.printJSON(cmd, map[string]any{"budget_ceiling": n})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Budget ceiling set to %d credits per call.\n", n)
			return nil
		},
	}
}

func newBudgetShowCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "show",
		Short:       "Show the current credit ceiling",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			db, derr := openCreditStore()
			if derr != nil {
				return configErr(derr)
			}
			defer db.Close()
			n, have, err := getBudget(db.DB())
			if err != nil {
				return apiErr(err)
			}
			if flags.asJSON {
				out := map[string]any{"budget_set": have}
				if have {
					out["budget_ceiling"] = n
				}
				return flags.printJSON(cmd, out)
			}
			if !have {
				fmt.Fprintln(cmd.OutOrStdout(), "No budget ceiling set.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Budget ceiling: %d credits per call.\n", n)
			return nil
		},
	}
}

func newBudgetClearCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "clear",
		Short: "Remove the credit ceiling",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, derr := openCreditStore()
			if derr != nil {
				return configErr(derr)
			}
			defer db.Close()
			if err := clearBudget(db.DB()); err != nil {
				return apiErr(err)
			}
			if flags.asJSON {
				return flags.printJSON(cmd, map[string]any{"budget_set": false})
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Budget ceiling cleared.")
			return nil
		},
	}
}
