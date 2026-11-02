// Copyright 2026 david and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/gusto/internal/gusto"
)

func newNovelPayrollsMileageCmd(flags *rootFlags) *cobra.Command {
	var t draftTarget
	var flagEmployee string
	var flagMiles string
	var flagRate string

	cmd := &cobra.Command{
		Use:   "mileage",
		Short: "Add a mileage reimbursement to the open draft payroll from miles and a per-mile rate, with a clear paystub description.",
		Long: "Add a mileage reimbursement to a draft payroll. The amount is miles x rate, rounded to\n" +
			"the cent, and the paystub description is \"Mileage <miles> mi x $<rate>\". Running the same\n" +
			"command again replaces that line instead of adding a duplicate.\n\n" +
			"Use this command to turn a mileage figure into a reimbursement. Do NOT use it for a\n" +
			"non-mileage reimbursement with a fixed amount; use 'payrolls reimburse add' instead. Do NOT\n" +
			"use it to load a whole week's inputs; use 'payrolls prep' instead.",
		Example:     "  gusto-pp-cli payrolls mileage --period-end 2026-11-01 --employee 'Ava Example' --miles 5.2 --rate 0.60 --dry-run",
		Annotations: map[string]string{"mcp:read-only": "false", "pp:data-source": "live", "pp:happy-args": "--employee=example-value;--miles=0;--rate=0.60", "pp:verified-by-proof": "payrolls-write-lifecycle.md"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			miles, mErr := strconv.ParseFloat(strings.TrimSpace(flagMiles), 64)
			rate, rErr := strconv.ParseFloat(strings.TrimPrefix(strings.TrimSpace(flagRate), "$"), 64)
			if dryRunOK(flags) {
				if mErr == nil && rErr == nil && miles > 0 && rate > 0 {
					amt, desc := gusto.Mileage(miles, rate)
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "action": "payrolls mileage", "would": fmt.Sprintf("add %s %q", money(amt), desc), "amount": amt, "description": desc}, flags)
				}
				return writeDryRun(cmd.OutOrStdout(), flags, "payrolls mileage")
			}
			if mErr != nil || miles <= 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--miles must be a positive number, e.g. --miles 5.2"))
			}
			if rErr != nil || rate <= 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--rate must be a positive dollars-per-mile figure, e.g. --rate 0.60"))
			}
			amt, desc := gusto.Mileage(miles, rate)
			if amt <= 0 {
				return usageErr(fmt.Errorf("%.2f miles at $%.2f rounds to $0.00; nothing to add", miles, rate))
			}
			return addReimbursementLine(cmd, flags, t, flagEmployee, gusto.Reimbursement{Amount: amt, Description: desc})
		},
	}
	cmd.Flags().StringVar(&t.periodEnd, "period-end", "", periodEndFlagHelp)
	cmd.Flags().StringVar(&t.draftID, "draft-id", "", draftIDFlagHelp)
	cmd.Flags().StringVar(&flagEmployee, "employee", "", "Employee name (full or first) or uuid; optional when the draft has one employee")
	cmd.Flags().StringVar(&flagMiles, "miles", "", "Miles driven in the pay period")
	cmd.Flags().StringVar(&flagRate, "rate", "", "Reimbursement rate in dollars per mile, e.g. 0.60")
	return cmd
}
