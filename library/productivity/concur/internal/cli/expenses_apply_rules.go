// Copyright 2026 Allen Lew and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written command: absorbed feature #25 from the absorb manifest,
// ported from proven prior art (private expense-report-filer's
// apply_expense_type_rules / ExpenseTypeRule). This is the mutating
// counterpart to 'reports validate' (which only reports issues) -- this
// command actually fills the business-purpose field when a rule matches
// and it's currently empty.
// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/productivity/concur/internal/types"
	"github.com/spf13/cobra"
)

// expenseRule is one entry in the --config JSON file: a Concur Expense Type
// code mapped to the business purpose to fill when empty, and an optional
// reimbursement cap to flag overages against.
type expenseRule struct {
	BusinessPurpose  string   `json:"business_purpose"`
	ReimbursementCap *float64 `json:"reimbursement_cap,omitempty"`
}

// expenseRulesConfig maps a Concur Expense Type code to its rule. Reconstructed
// 2026-08-19 after reports_validate.go (its original home, sharing this type
// with this command) was removed; this file is now its only user.
type expenseRulesConfig map[string]expenseRule

type appliedRuleChange struct {
	ExpenseId   string `json:"expense_id"`
	ExpenseType string `json:"expense_type"`
	ChangeType  string `json:"change_type"`
	Detail      string `json:"detail"`
}

type applyRulesResult struct {
	ReportId string              `json:"report_id"`
	Changes  []appliedRuleChange `json:"changes"`
}

func newExpensesApplyRulesCmd(flags *rootFlags) *cobra.Command {
	var flagUserId string
	var flagContextType string
	var flagExpenseRulesConfig string

	cmd := &cobra.Command{
		Use:     "apply-rules <report_id>",
		Short:   "Fill per-expense-type business purpose (and flag reimbursement-cap overages) using a config file, ported from proven prior-art business logic",
		Example: "  concur-pp-cli expenses apply-rules 764428DD6A664AF0BFCB --user-id 550e8400-e29b-41d4-a716-446655440000 --agent",
		Long: "For each expense on the report, checks expense_types.json for a rule matching its Concur\n" +
			"Expense Type. Fills the per-expense Business Purpose if it's currently empty. If the rule\n" +
			"has a reimbursement_cap and the actual amount exceeds it, this command does NOT attempt to\n" +
			"itemize the transaction automatically -- Concur's itemization API contract for splitting a\n" +
			"reimbursable portion from a personal remainder is unverified in this CLI, and guessing at it\n" +
			"risks corrupting real financial data. Instead it reports the overage as a change of type\n" +
			"'exceeds_cap_needs_manual_split' so you can itemize it yourself in the Concur UI. Use\n" +
			"--dry-run first to preview what this command would change without writing anything.",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if flags.asJSON {
					if printErr := printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"error": "missing required argument",
						"usage": fmt.Sprintf("%s%s", cmd.CommandPath(), " <report_id>"),
					}, flags); printErr != nil {
						return printErr
					}
				}
				return usageErr(fmt.Errorf("missing required argument\nUsage: %s%s", cmd.CommandPath(), " <report_id>"))
			}
			if !cmd.Flags().Changed("user-id") && !flags.dryRun {
				return fmt.Errorf("required flag \"%s\" not set", "user-id")
			}
			reportID := args[0]

			var config expenseRulesConfig
			if flagExpenseRulesConfig != "" {
				// #nosec G304 -- flagExpenseRulesConfig is a --config path the
				// CLI operator supplies on their own command line to their own
				// machine; there is no remote/untrusted input and no privilege
				// boundary being crossed (the operator already has direct
				// filesystem access to any path they could pass here).
				data, err := os.ReadFile(flagExpenseRulesConfig)
				if err == nil {
					if err := json.Unmarshal(data, &config); err != nil {
						return fmt.Errorf("parsing expense rules config: %w", err)
					}
				} else if !os.IsNotExist(err) {
					return fmt.Errorf("reading expense rules config file: %w", err)
				}
			}
			if len(config) == 0 {
				// Matches prior-art semantics: a missing/empty config is a
				// no-op, not an error -- nothing to apply.
				return printJSONFiltered(cmd.OutOrStdout(), applyRulesResult{ReportId: reportID, Changes: nil}, flags)
			}

			// This command writes to real financial data (business-purpose
			// fields on live expenses) when not a dry-run -- require explicit
			// confirmation, matching the other mutating novel commands
			// (available-expenses link-to-trip, expenses tag).
			if !flags.yes && !flags.dryRun {
				fmt.Fprintf(cmd.ErrOrStderr(), "refusing to apply expense-type rules to report %q without --yes\n", reportID)
				return fmt.Errorf("confirmation required: pass --yes")
			}

			var expenses []types.Expense
			c, err := flags.newClient()
			if err != nil && !flags.dryRun {
				return err
			}

			if err == nil {
				path := "/expensereports/v4/users/{user_id}/context/{context_type}/reports/{report_id}"
				path = replacePathParam(path, "report_id", reportID)
				path = replacePathParam(path, "user_id", formatCLIParamValue(flagUserId))
				path = replacePathParam(path, "context_type", formatCLIParamValue(flagContextType))

				data, _, readErr := resolveReadWithStrategyAndResponsePath(cmd.Context(), c, flags, "live", "reports", false, path, map[string]string{}, nil, "", cmd.ErrOrStderr())
				if readErr != nil {
					if !flags.dryRun {
						return classifyAPIError(readErr, flags)
					}
					fmt.Fprintln(cmd.ErrOrStderr(), "dry-run: could not reach the live API (no/expired session) -- previewing with simulated example data, not your real report")
					expenses = []types.Expense{
						{ExpenseId: "exp-1", ExpenseTypeCode: "Mobile/Cellular Phone", TransactionAmount: 65.00, BusinessPurpose: ""},
						{ExpenseId: "exp-2", ExpenseTypeCode: "Fitness", TransactionAmount: 30.00, BusinessPurpose: "gym"},
					}
				} else {
					var nested struct {
						Expenses []types.Expense `json:"expenses"`
					}
					nestedErr := json.Unmarshal(data, &nested)
					if nestedErr == nil && len(nested.Expenses) > 0 {
						expenses = nested.Expenses
					} else {
						flatErr := json.Unmarshal(data, &expenses)
						if flatErr != nil {
							if flags.dryRun {
								// dry-run's own short-circuit can return an empty/
								// placeholder body with no readErr; that's a valid
								// "nothing to preview" state, not a parse failure.
								fmt.Fprintln(cmd.ErrOrStderr(), "dry-run: live response had no parseable expenses (placeholder dry-run body) -- nothing to preview")
							} else {
								// A real (non-dry-run) financial-data mutation command
								// must fail loud on an unrecognized response shape, not
								// silently report "nothing to apply" -- that would look
								// identical to "already compliant".
								return fmt.Errorf("parsing report %q expenses response (tried both nested {expenses:[...]} and flat list shapes): nested=%v flat=%v", reportID, nestedErr, flatErr)
							}
						}
					}
				}
			} else {
				fmt.Fprintln(cmd.ErrOrStderr(), "dry-run: no client/auth configured -- previewing with simulated example data, not your real report")
				expenses = []types.Expense{
					{ExpenseId: "exp-1", ExpenseTypeCode: "Mobile/Cellular Phone", TransactionAmount: 65.00, BusinessPurpose: ""},
					{ExpenseId: "exp-2", ExpenseTypeCode: "Fitness", TransactionAmount: 30.00, BusinessPurpose: "gym"},
				}
			}

			var changes []appliedRuleChange
			for _, exp := range expenses {
				rule, ok := config[exp.ExpenseTypeCode]
				if !ok {
					continue
				}
				needsPurpose := exp.BusinessPurpose == ""
				exceedsCap := rule.ReimbursementCap != nil && exp.TransactionAmount > *rule.ReimbursementCap

				if !needsPurpose && !exceedsCap {
					continue // already up to date, safe to re-run
				}

				if needsPurpose {
					if flags.dryRun {
						changes = append(changes, appliedRuleChange{
							ExpenseId: exp.ExpenseId, ExpenseType: exp.ExpenseTypeCode,
							ChangeType: "would_set_business_purpose",
							Detail:     fmt.Sprintf("dry-run: would set Business Purpose to %q", rule.BusinessPurpose),
						})
					} else {
						updatePath := "/expensereports/v4/users/{user_id}/context/{context_type}/reports/{report_id}/expenses/{expense_id}"
						updatePath = replacePathParam(updatePath, "user_id", formatCLIParamValue(flagUserId))
						updatePath = replacePathParam(updatePath, "context_type", formatCLIParamValue(flagContextType))
						updatePath = replacePathParam(updatePath, "report_id", reportID)
						updatePath = replacePathParam(updatePath, "expense_id", exp.ExpenseId)
						if _, _, err := c.PatchWithHeaders(cmd.Context(), updatePath, map[string]any{"businessPurpose": rule.BusinessPurpose}, nil); err != nil {
							return fmt.Errorf("setting business purpose on expense %s: %w", exp.ExpenseId, classifyAPIError(err, flags))
						}
						changes = append(changes, appliedRuleChange{
							ExpenseId: exp.ExpenseId, ExpenseType: exp.ExpenseTypeCode,
							ChangeType: "set_business_purpose",
							Detail:     fmt.Sprintf("Business Purpose set to %q", rule.BusinessPurpose),
						})
					}
				}

				if exceedsCap {
					// See the command's Long description: itemization is
					// deliberately not automated without a verified endpoint.
					changes = append(changes, appliedRuleChange{
						ExpenseId: exp.ExpenseId, ExpenseType: exp.ExpenseTypeCode,
						ChangeType: "exceeds_cap_needs_manual_split",
						Detail: fmt.Sprintf("Amount %.2f exceeds reimbursement cap %.2f -- itemize the personal remainder manually in Concur",
							exp.TransactionAmount, *rule.ReimbursementCap),
					})
				}
			}

			return printJSONFiltered(cmd.OutOrStdout(), applyRulesResult{ReportId: reportID, Changes: changes}, flags)
		},
	}

	cmd.Flags().StringVar(&flagUserId, "user-id", "", "Concur user ID (from 'account whoami')")
	cmd.Flags().StringVar(&flagContextType, "context-type", "TRAVELER", "Access level context for the request (one of: TRAVELER, MANAGER, PROCESSOR, PROXY)")
	cmd.Flags().StringVar(&flagExpenseRulesConfig, "config", "expense_types.json", "Path to JSON config file mapping Concur Expense Type -> business_purpose (and optional reimbursement_cap)")

	return cmd
}
