// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

// pp:data-source live

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var budgetExpenseCategories = map[string]bool{
	"flights": true, "lodging": true, "carRental": true, "publicTransit": true,
	"food": true, "drinks": true, "sightseeing": true, "activities": true,
	"shopping": true, "gas": true, "groceries": true, "other": true,
}

var budgetSplitModes = map[string]bool{"noOne": true, "everyone": true, "individuals": true}

func newNovelPlanBudgetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "budget", Short: "Read and edit Wanderlog trip budget expenses and payments", RunE: parentNoSubcommandRunE(flags)}
	cmd.AddCommand(newNovelPlanBudgetSummaryCmd(flags))
	cmd.AddCommand(newNovelPlanBudgetCSVCommand(flags))
	cmd.AddCommand(newNovelPlanBudgetSetCmd(flags))
	cmd.AddCommand(newNovelPlanBudgetExpenseCmd(flags))
	cmd.AddCommand(newNovelPlanBudgetPaymentCmd(flags))
	return cmd
}

func newNovelPlanBudgetSummaryCmd(flags *rootFlags) *cobra.Command {
	opts := planEditOptions{clientSchemaVersion: 2}
	cmd := &cobra.Command{Use: "summary", Short: "Summarize a Wanderlog trip budget", Example: "  wanderlog-pp-cli plan budget summary --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --agent", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		c, err := planLiveClient(flags)
		if err != nil {
			return err
		}
		key, err := resolveEditablePlanKey(opts)
		if err != nil {
			return usageErr(err)
		}
		trip, _, err := fetchPlanSnapshotViaShareDB(ctx, c, key, opts.clientSchemaVersion)
		if err != nil {
			return err
		}
		budget := ensureBudget(mapField(trip, "itinerary"))
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"command": "plan budget summary", "target_key": key, "budget": summarizeBudget(budget)}, flags)
	}}
	addPlanTargetFlags(cmd, &opts)
	cmd.Flags().IntVar(&opts.clientSchemaVersion, "client-schema-version", 2, "Wanderlog client schema version")
	return cmd
}

func newNovelPlanBudgetCSVCommand(flags *rootFlags) *cobra.Command {
	opts := planEditOptions{}
	cmd := &cobra.Command{Use: "csv", Short: "Export Wanderlog budget expenses as CSV", Example: "  wanderlog-pp-cli plan budget csv --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --agent", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		c, err := planLiveClient(flags)
		if err != nil {
			return err
		}
		key, err := resolveEditablePlanKey(opts)
		if err != nil {
			return usageErr(err)
		}
		data, err := c.GetNoCache(ctx, "/api/tripPlans/"+key+"/expensesAsCSV", nil)
		if err != nil {
			return classifyAPIError(err, flags)
		}
		if flags.asJSON || flags.agent || flags.compact || flags.selectFields != "" {
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"command": "plan budget csv", "target_key": key, "csv": string(data)}, flags)
		}
		_, err = cmd.OutOrStdout().Write(data)
		return err
	}}
	addPlanTargetFlags(cmd, &opts)
	return cmd
}

func newNovelPlanBudgetSetCmd(flags *rootFlags) *cobra.Command {
	opts := planEditOptions{clientSchemaVersion: 2, applyRetries: 2}
	var amount float64
	var currency string
	var simplifyDebt bool
	cmd := &cobra.Command{Use: "set", Short: "Set budget total, currency, or debt simplification", Example: "  wanderlog-pp-cli plan budget set --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --amount 500 --currency USD --dry-run --agent", RunE: func(cmd *cobra.Command, args []string) error {
		if !cmd.Flags().Changed("amount") && !cmd.Flags().Changed("currency") && !cmd.Flags().Changed("simplify-debt") {
			return usageErr(errors.New("set at least one of --amount, --currency, or --simplify-debt"))
		}
		currency = strings.ToUpper(strings.TrimSpace(currency))
		if cmd.Flags().Changed("currency") && currency == "" {
			return usageErr(errors.New("--currency cannot be empty"))
		}
		return runPlanEdit(cmd, flags, opts, "plan budget set", func(target map[string]any) (planEditBuildResult, error) {
			budget, budgetExists := editableBudget(target)
			if cmd.Flags().Changed("amount") || cmd.Flags().Changed("currency") {
				amountMap := cloneJSONMap(mapField(budget, "amount"))
				if amountMap == nil {
					amountMap = map[string]any{"amount": 0, "currencyCode": "USD"}
				}
				if cmd.Flags().Changed("amount") {
					amountMap["amount"] = amount
				}
				if cmd.Flags().Changed("currency") {
					amountMap["currencyCode"] = currency
				}
				budget["amount"] = amountMap
			}
			if cmd.Flags().Changed("simplify-debt") {
				budget["simplifyDebt"] = simplifyDebt
			}
			ops := budgetSetOps(budget, budgetExists, mapField(target, "itinerary"), cmd.Flags().Changed("amount") || cmd.Flags().Changed("currency"), cmd.Flags().Changed("simplify-debt"))
			report := baseEditReport("plan budget set", opts, target)
			report.Operation = "set budget fields"
			report.OpPaths = opPaths(ops)
			report.Budget = summarizeBudget(budget)
			return planEditBuildResult{Ops: ops, Report: report}, nil
		})
	}}
	addPlanTargetFlags(cmd, &opts)
	cmd.Flags().Float64Var(&amount, "amount", 0, "Total trip budget amount")
	cmd.Flags().StringVar(&currency, "currency", "", "Budget currency code, e.g. USD, SGD, JPY")
	cmd.Flags().BoolVar(&simplifyDebt, "simplify-debt", false, "Enable or disable debt simplification")
	cmd.Flags().BoolVar(&opts.apply, "apply", false, "Apply through Wanderlog ShareDB; default is preview only")
	cmd.Flags().IntVar(&opts.clientSchemaVersion, "client-schema-version", 2, "Wanderlog client schema version")
	return cmd
}

func newNovelPlanBudgetExpenseCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "expense", Short: "List, add, edit, or remove budget expenses", RunE: parentNoSubcommandRunE(flags)}
	cmd.AddCommand(newNovelPlanBudgetExpenseListCmd(flags))
	cmd.AddCommand(newNovelPlanBudgetExpenseAddCmd(flags))
	cmd.AddCommand(newNovelPlanBudgetExpenseEditCmd(flags))
	cmd.AddCommand(newNovelPlanBudgetExpenseRemoveCmd(flags))
	return cmd
}

func newNovelPlanBudgetExpenseListCmd(flags *rootFlags) *cobra.Command {
	opts := planEditOptions{clientSchemaVersion: 2}
	cmd := &cobra.Command{Use: "list", Short: "List Wanderlog budget expenses", Example: "  wanderlog-pp-cli plan budget expense list --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --agent", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		c, err := planLiveClient(flags)
		if err != nil {
			return err
		}
		key, err := resolveEditablePlanKey(opts)
		if err != nil {
			return usageErr(err)
		}
		trip, _, err := fetchPlanSnapshotViaShareDB(ctx, c, key, opts.clientSchemaVersion)
		if err != nil {
			return err
		}
		budget := ensureBudget(mapField(trip, "itinerary"))
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"command": "plan budget expense list", "target_key": key, "expenses": budgetArray(budget, "expenses")}, flags)
	}}
	addPlanTargetFlags(cmd, &opts)
	cmd.Flags().IntVar(&opts.clientSchemaVersion, "client-schema-version", 2, "Wanderlog client schema version")
	return cmd
}

func newNovelPlanBudgetExpenseAddCmd(flags *rootFlags) *cobra.Command {
	opts := planEditOptions{clientSchemaVersion: 2, applyRetries: 2}
	var in budgetExpenseFlags
	cmd := &cobra.Command{Use: "add", Short: "Add a Wanderlog budget expense", Example: "  wanderlog-pp-cli plan budget expense add --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --description Lunch --amount 12 --dry-run --agent", RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(in.JSON) == "" && (strings.TrimSpace(in.Description) == "" || !cmd.Flags().Changed("amount")) {
			return usageErr(errors.New("--description and --amount are required unless --json-value is supplied"))
		}
		return runPlanEdit(cmd, flags, opts, "plan budget expense add", func(target map[string]any) (planEditBuildResult, error) {
			budget, budgetExists := editableBudget(target)
			expense, err := buildBudgetExpense(in, target)
			if err != nil {
				return planEditBuildResult{}, err
			}
			ops := budgetAppendOps(budget, budgetExists, "expenses", expense)
			report := baseEditReport("plan budget expense add", opts, target)
			report.Operation = "add budget expense"
			report.OpPaths = opPaths(ops)
			report.Expense = summarizeBudgetExpense(expense)
			return planEditBuildResult{Ops: ops, Report: report}, nil
		})
	}}
	addPlanTargetFlags(cmd, &opts)
	addBudgetExpenseFlags(cmd, &in)
	cmd.Flags().BoolVar(&opts.apply, "apply", false, "Apply through Wanderlog ShareDB; default is preview only")
	cmd.Flags().IntVar(&opts.clientSchemaVersion, "client-schema-version", 2, "Wanderlog client schema version")
	return cmd
}

func newNovelPlanBudgetExpenseEditCmd(flags *rootFlags) *cobra.Command {
	opts := planEditOptions{clientSchemaVersion: 2, applyRetries: 2}
	var in budgetExpenseFlags
	cmd := &cobra.Command{Use: "edit", Short: "Edit a Wanderlog budget expense", Example: "  wanderlog-pp-cli plan budget expense edit --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --expense-index 0 --description Snack --dry-run --agent", RunE: func(cmd *cobra.Command, args []string) error {
		if in.ID == 0 && in.Index < 0 {
			return usageErr(errors.New("--expense-id or --expense-index is required"))
		}
		return runPlanEdit(cmd, flags, opts, "plan budget expense edit", func(target map[string]any) (planEditBuildResult, error) {
			budget, _ := editableBudget(target)
			expenses := budgetArray(budget, "expenses")
			idx, old, err := findBudgetItem(expenses, in.ID, in.Index, "expense")
			if err != nil {
				return planEditBuildResult{}, err
			}
			updated := cloneJSONMap(old)
			if strings.TrimSpace(in.JSON) != "" {
				parsed, err := parseJSONObjectFlag(in.JSON, "--json-value")
				if err != nil {
					return planEditBuildResult{}, err
				}
				updated = parsed
			} else if err := applyBudgetExpenseFlagUpdates(cmd, updated, in); err != nil {
				return planEditBuildResult{}, err
			}
			if err := validateBudgetExpenseBlockID(updated, target); err != nil {
				return planEditBuildResult{}, err
			}
			ops := []map[string]any{{"p": []any{"itinerary", "budget", "expenses", idx}, "ld": old, "li": updated}}
			report := baseEditReport("plan budget expense edit", opts, target)
			report.Operation = "edit budget expense"
			report.OpPaths = opPaths(ops)
			report.Expense = summarizeBudgetExpense(updated)
			return planEditBuildResult{Ops: ops, Report: report}, nil
		})
	}}
	addPlanTargetFlags(cmd, &opts)
	addBudgetExpenseFlags(cmd, &in)
	cmd.Flags().IntVar(&in.ID, "expense-id", 0, "Expense id")
	cmd.Flags().IntVar(&in.Index, "expense-index", -1, "Zero-based expense index")
	cmd.Flags().BoolVar(&opts.apply, "apply", false, "Apply through Wanderlog ShareDB; default is preview only")
	cmd.Flags().IntVar(&opts.clientSchemaVersion, "client-schema-version", 2, "Wanderlog client schema version")
	return cmd
}

func newNovelPlanBudgetExpenseRemoveCmd(flags *rootFlags) *cobra.Command {
	opts := planEditOptions{clientSchemaVersion: 2, applyRetries: 2}
	var id int
	var idxFlag int
	cmd := &cobra.Command{Use: "remove", Short: "Remove a Wanderlog budget expense", Example: "  wanderlog-pp-cli plan budget expense remove --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --expense-index 0 --dry-run --agent", RunE: func(cmd *cobra.Command, args []string) error {
		if id == 0 && idxFlag < 0 {
			return usageErr(errors.New("--expense-id or --expense-index is required"))
		}
		return runPlanEdit(cmd, flags, opts, "plan budget expense remove", func(target map[string]any) (planEditBuildResult, error) {
			budget, _ := editableBudget(target)
			expenses := budgetArray(budget, "expenses")
			idx, old, err := findBudgetItem(expenses, id, idxFlag, "expense")
			if err != nil {
				return planEditBuildResult{}, err
			}
			ops := []map[string]any{{"p": []any{"itinerary", "budget", "expenses", idx}, "ld": old}}
			report := baseEditReport("plan budget expense remove", opts, target)
			report.Operation = "remove budget expense"
			report.OpPaths = opPaths(ops)
			report.Expense = summarizeBudgetExpense(old)
			return planEditBuildResult{Ops: ops, Report: report}, nil
		})
	}}
	addPlanTargetFlags(cmd, &opts)
	cmd.Flags().IntVar(&id, "expense-id", 0, "Expense id")
	cmd.Flags().IntVar(&idxFlag, "expense-index", -1, "Zero-based expense index")
	cmd.Flags().BoolVar(&opts.apply, "apply", false, "Apply through Wanderlog ShareDB; default is preview only")
	cmd.Flags().IntVar(&opts.clientSchemaVersion, "client-schema-version", 2, "Wanderlog client schema version")
	return cmd
}

func newNovelPlanBudgetPaymentCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "payment", Short: "List, add, or remove budget settlement payments", RunE: parentNoSubcommandRunE(flags)}
	cmd.AddCommand(newNovelPlanBudgetPaymentListCmd(flags))
	cmd.AddCommand(newNovelPlanBudgetPaymentAddCmd(flags))
	cmd.AddCommand(newNovelPlanBudgetPaymentRemoveCmd(flags))
	return cmd
}

func newNovelPlanBudgetPaymentListCmd(flags *rootFlags) *cobra.Command {
	opts := planEditOptions{clientSchemaVersion: 2}
	cmd := &cobra.Command{Use: "list", Short: "List Wanderlog budget payments", Example: "  wanderlog-pp-cli plan budget payment list --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --agent", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		c, err := planLiveClient(flags)
		if err != nil {
			return err
		}
		key, err := resolveEditablePlanKey(opts)
		if err != nil {
			return usageErr(err)
		}
		trip, _, err := fetchPlanSnapshotViaShareDB(ctx, c, key, opts.clientSchemaVersion)
		if err != nil {
			return err
		}
		budget := ensureBudget(mapField(trip, "itinerary"))
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"command": "plan budget payment list", "target_key": key, "payments": budgetArray(budget, "payments")}, flags)
	}}
	addPlanTargetFlags(cmd, &opts)
	cmd.Flags().IntVar(&opts.clientSchemaVersion, "client-schema-version", 2, "Wanderlog client schema version")
	return cmd
}

func newNovelPlanBudgetPaymentAddCmd(flags *rootFlags) *cobra.Command {
	opts := planEditOptions{clientSchemaVersion: 2, applyRetries: 2}
	var fromUserID int
	var toUserID int
	var amount float64
	var currency string
	var paidAt string
	var jsonValue string
	cmd := &cobra.Command{Use: "add", Short: "Add a Wanderlog budget settlement payment", Example: "  wanderlog-pp-cli plan budget payment add --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --from-user-id 1 --to-user-id 2 --amount 10 --dry-run --agent", RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(jsonValue) == "" && (fromUserID == 0 || toUserID == 0 || !cmd.Flags().Changed("amount")) {
			return usageErr(errors.New("--from-user-id, --to-user-id, and --amount are required unless --json-value is supplied"))
		}
		return runPlanEdit(cmd, flags, opts, "plan budget payment add", func(target map[string]any) (planEditBuildResult, error) {
			budget, budgetExists := editableBudget(target)
			defaultCurrency := firstNonEmpty(stringField(mapField(budget, "amount"), "currencyCode"), "USD")
			payment, err := buildBudgetPayment(fromUserID, toUserID, amount, currency, defaultCurrency, paidAt, jsonValue)
			if err != nil {
				return planEditBuildResult{}, err
			}
			ops := budgetAppendOps(budget, budgetExists, "payments", payment)
			report := baseEditReport("plan budget payment add", opts, target)
			report.Operation = "add budget payment"
			report.OpPaths = opPaths(ops)
			report.Payment = summarizeBudgetPayment(payment)
			return planEditBuildResult{Ops: ops, Report: report}, nil
		})
	}}
	addPlanTargetFlags(cmd, &opts)
	cmd.Flags().IntVar(&fromUserID, "from-user-id", 0, "Paying user id")
	cmd.Flags().IntVar(&toUserID, "to-user-id", 0, "Receiving user id")
	cmd.Flags().Float64Var(&amount, "amount", 0, "Payment amount")
	cmd.Flags().StringVar(&currency, "currency", "", "Currency code; defaults to budget currency")
	cmd.Flags().StringVar(&paidAt, "paid-at", "", "Payment timestamp/date; defaults to now")
	cmd.Flags().StringVar(&jsonValue, "json-value", "", "Exact payment JSON object")
	cmd.Flags().BoolVar(&opts.apply, "apply", false, "Apply through Wanderlog ShareDB; default is preview only")
	cmd.Flags().IntVar(&opts.clientSchemaVersion, "client-schema-version", 2, "Wanderlog client schema version")
	return cmd
}

func newNovelPlanBudgetPaymentRemoveCmd(flags *rootFlags) *cobra.Command {
	opts := planEditOptions{clientSchemaVersion: 2, applyRetries: 2}
	var id int
	var idxFlag int
	cmd := &cobra.Command{Use: "remove", Short: "Remove a Wanderlog budget payment", Example: "  wanderlog-pp-cli plan budget payment remove --plan-url https://wanderlog.com/plan/naertjcoixqrgrfc/morocco-travel-travel-guide/shared --payment-index 0 --dry-run --agent", RunE: func(cmd *cobra.Command, args []string) error {
		if id == 0 && idxFlag < 0 {
			return usageErr(errors.New("--payment-id or --payment-index is required"))
		}
		return runPlanEdit(cmd, flags, opts, "plan budget payment remove", func(target map[string]any) (planEditBuildResult, error) {
			budget, _ := editableBudget(target)
			payments := budgetArray(budget, "payments")
			idx, old, err := findBudgetItem(payments, id, idxFlag, "payment")
			if err != nil {
				return planEditBuildResult{}, err
			}
			ops := []map[string]any{{"p": []any{"itinerary", "budget", "payments", idx}, "ld": old}}
			report := baseEditReport("plan budget payment remove", opts, target)
			report.Operation = "remove budget payment"
			report.OpPaths = opPaths(ops)
			report.Payment = summarizeBudgetPayment(old)
			return planEditBuildResult{Ops: ops, Report: report}, nil
		})
	}}
	addPlanTargetFlags(cmd, &opts)
	cmd.Flags().IntVar(&id, "payment-id", 0, "Payment id")
	cmd.Flags().IntVar(&idxFlag, "payment-index", -1, "Zero-based payment index")
	cmd.Flags().BoolVar(&opts.apply, "apply", false, "Apply through Wanderlog ShareDB; default is preview only")
	cmd.Flags().IntVar(&opts.clientSchemaVersion, "client-schema-version", 2, "Wanderlog client schema version")
	return cmd
}

type budgetExpenseFlags struct {
	ID             int
	Index          int
	Description    string
	Amount         float64
	Currency       string
	Category       string
	Date           string
	BlockID        int
	PaidByUserID   int
	SplitWith      string
	SplitUserIDs   []int
	AssociatedDate string
	JSON           string
}

func addBudgetExpenseFlags(cmd *cobra.Command, in *budgetExpenseFlags) {
	cmd.Flags().StringVar(&in.Description, "description", "", "Expense description")
	cmd.Flags().Float64Var(&in.Amount, "amount", 0, "Expense amount")
	cmd.Flags().StringVar(&in.Currency, "currency", "", "Currency code; defaults to budget currency")
	cmd.Flags().StringVar(&in.Category, "category", "other", "Expense category")
	cmd.Flags().StringVar(&in.Date, "date", "", "Expense date YYYY-MM-DD")
	cmd.Flags().IntVar(&in.BlockID, "block-id", 0, "Optional itinerary block id associated with the expense")
	cmd.Flags().IntVar(&in.PaidByUserID, "paid-by-user-id", 0, "Payer user id; defaults to plan owner/user id")
	cmd.Flags().StringVar(&in.SplitWith, "split-with", "noOne", "Split mode: noOne, everyone, or individuals")
	cmd.Flags().IntSliceVar(&in.SplitUserIDs, "split-user-id", nil, "User id included when --split-with individuals; repeatable or comma-separated")
	cmd.Flags().StringVar(&in.AssociatedDate, "associated-date", "", "Associated itinerary date YYYY-MM-DD; defaults from --date")
	cmd.Flags().StringVar(&in.JSON, "json-value", "", "Exact expense JSON object")
}

func ensureBudget(itinerary map[string]any) map[string]any {
	budget, _ := budgetFromItinerary(itinerary)
	return budget
}

func editableBudget(target map[string]any) (map[string]any, bool) {
	return budgetFromItinerary(mapField(target, "itinerary"))
}

func budgetFromItinerary(itinerary map[string]any) (map[string]any, bool) {
	budget := mapField(itinerary, "budget")
	if budget == nil {
		return defaultBudget(), false
	}
	return cloneJSONMap(budget), true
}

func defaultBudget() map[string]any {
	return map[string]any{"amount": map[string]any{"amount": 0, "currencyCode": "USD"}, "expenses": []any{}, "payments": []any{}, "simplifyDebt": false}
}

func budgetAppendOps(budget map[string]any, budgetExists bool, field string, item map[string]any) []map[string]any {
	if !budgetExists {
		budget[field] = []any{item}
		return []map[string]any{objectSetOp([]any{"itinerary", "budget"}, nil, false, budget, false)}
	}
	old, listExists := budget[field]
	list, _ := old.([]any)
	if list == nil {
		return []map[string]any{objectSetOp([]any{"itinerary", "budget", field}, old, listExists, []any{item}, false)}
	}
	return []map[string]any{{"p": []any{"itinerary", "budget", field, len(list)}, "li": item}}
}

func budgetSetOps(budget map[string]any, budgetExists bool, itinerary map[string]any, amountChanged bool, simplifyDebtChanged bool) []map[string]any {
	if !budgetExists {
		return []map[string]any{objectSetOp([]any{"itinerary", "budget"}, nil, false, budget, false)}
	}
	ops := []map[string]any{}
	oldBudget := mapField(itinerary, "budget")
	if amountChanged {
		old, exists := oldBudget["amount"]
		ops = append(ops, objectSetOp([]any{"itinerary", "budget", "amount"}, old, exists, budget["amount"], false))
	}
	if simplifyDebtChanged {
		old, exists := oldBudget["simplifyDebt"]
		ops = append(ops, objectSetOp([]any{"itinerary", "budget", "simplifyDebt"}, old, exists, budget["simplifyDebt"], false))
	}
	return ops
}

func budgetArray(budget map[string]any, field string) []any {
	arr, _ := budget[field].([]any)
	if arr == nil {
		return []any{}
	}
	return arr
}

func summarizeBudget(budget map[string]any) map[string]any {
	expenses := budgetArray(budget, "expenses")
	payments := budgetArray(budget, "payments")
	byCategory := map[string]float64{}
	byCurrency := map[string]float64{}
	for _, raw := range expenses {
		exp, _ := raw.(map[string]any)
		if exp == nil {
			continue
		}
		amount := mapField(exp, "amount")
		value := floatAny(amount["amount"])
		currency := firstNonEmpty(stringField(amount, "currencyCode"), stringField(mapField(budget, "amount"), "currencyCode"))
		category := firstNonEmpty(stringField(exp, "category"), "other")
		byCategory[category] += value
		byCurrency[currency] += value
	}
	return map[string]any{"amount": budget["amount"], "expense_count": len(expenses), "payment_count": len(payments), "totals_by_category": byCategory, "totals_by_currency": byCurrency, "simplify_debt": budget["simplifyDebt"]}
}

func buildBudgetExpense(in budgetExpenseFlags, target map[string]any) (map[string]any, error) {
	if strings.TrimSpace(in.JSON) != "" {
		exp, err := parseJSONObjectFlag(in.JSON, "--json-value")
		if err != nil {
			return nil, err
		}
		if err := validateBudgetExpenseBlockID(exp, target); err != nil {
			return nil, err
		}
		return exp, nil
	}
	category := strings.TrimSpace(in.Category)
	if category == "" {
		category = "other"
	}
	if !budgetExpenseCategories[category] {
		return nil, fmt.Errorf("unsupported --category %q", category)
	}
	split, err := budgetSplitWith(in.SplitWith, in.SplitUserIDs)
	if err != nil {
		return nil, err
	}
	budget := ensureBudget(mapField(target, "itinerary"))
	currency := strings.ToUpper(strings.TrimSpace(in.Currency))
	if currency == "" {
		currency = firstNonEmpty(stringField(mapField(budget, "amount"), "currencyCode"), "USD")
	}
	payerID := in.PaidByUserID
	if payerID == 0 {
		payerID = firstNonZero(intAny(target["userId"]), intAny(target["ownerId"]))
	}
	date := strings.TrimSpace(in.Date)
	if date != "" {
		if _, err := time.Parse("2006-01-02", date); err != nil {
			return nil, fmt.Errorf("--date must be YYYY-MM-DD: %w", err)
		}
	}
	associatedDate := strings.TrimSpace(in.AssociatedDate)
	if associatedDate == "" {
		associatedDate = date
	}
	exp := map[string]any{"id": randomWanderlogID(), "amount": map[string]any{"amount": in.Amount, "currencyCode": currency}, "category": category, "description": in.Description, "date": dateOrNil(date), "blockId": intOrNil(in.BlockID), "paidByUserId": payerID, "paidByUser": budgetUserRef(payerID), "splitWith": split}
	if associatedDate != "" {
		exp["associatedDate"] = associatedDate
	}
	if err := validateBudgetExpenseBlockID(exp, target); err != nil {
		return nil, err
	}
	return exp, nil
}

func applyBudgetExpenseFlagUpdates(cmd *cobra.Command, exp map[string]any, in budgetExpenseFlags) error {
	if cmd.Flags().Changed("description") {
		exp["description"] = in.Description
	}
	if cmd.Flags().Changed("amount") || cmd.Flags().Changed("currency") {
		amount := cloneJSONMap(mapField(exp, "amount"))
		if amount == nil {
			amount = map[string]any{}
		}
		if cmd.Flags().Changed("amount") {
			amount["amount"] = in.Amount
		}
		if cmd.Flags().Changed("currency") {
			amount["currencyCode"] = strings.ToUpper(strings.TrimSpace(in.Currency))
		}
		exp["amount"] = amount
	}
	if cmd.Flags().Changed("category") {
		category := strings.TrimSpace(in.Category)
		if category == "" {
			category = "other"
		}
		if !budgetExpenseCategories[category] {
			return fmt.Errorf("unsupported --category %q", category)
		}
		exp["category"] = category
	}
	if cmd.Flags().Changed("date") {
		date := strings.TrimSpace(in.Date)
		if date != "" {
			if _, err := time.Parse("2006-01-02", date); err != nil {
				return fmt.Errorf("--date must be YYYY-MM-DD: %w", err)
			}
		}
		exp["date"] = dateOrNil(date)
	}
	if cmd.Flags().Changed("block-id") {
		exp["blockId"] = intOrNil(in.BlockID)
	}
	if cmd.Flags().Changed("paid-by-user-id") {
		exp["paidByUserId"] = in.PaidByUserID
		exp["paidByUser"] = budgetUserRef(in.PaidByUserID)
	}
	if cmd.Flags().Changed("split-with") || cmd.Flags().Changed("split-user-id") {
		split, err := budgetSplitWith(in.SplitWith, in.SplitUserIDs)
		if err != nil {
			return err
		}
		exp["splitWith"] = split
	}
	if cmd.Flags().Changed("associated-date") {
		date := strings.TrimSpace(in.AssociatedDate)
		if date != "" {
			if _, err := time.Parse("2006-01-02", date); err != nil {
				return fmt.Errorf("--associated-date must be YYYY-MM-DD: %w", err)
			}
		}
		exp["associatedDate"] = date
	}
	return nil
}

func summarizeBudgetExpense(exp map[string]any) map[string]any {
	out := cloneJSONMap(exp)
	if out == nil {
		return nil
	}
	return out
}

func summarizeBudgetPayment(payment map[string]any) map[string]any {
	out := cloneJSONMap(payment)
	if out == nil {
		return nil
	}
	return out
}

func validateBudgetExpenseBlockID(exp map[string]any, target map[string]any) error {
	blockID := intAny(exp["blockId"])
	if blockID == 0 || target == nil {
		return nil
	}
	if planHasBlockID(target, blockID) {
		return nil
	}
	return fmt.Errorf("block id %d not found in itinerary; use plan sections and the selected day/block id before associating a budget expense", blockID)
}

func planHasBlockID(target map[string]any, blockID int) bool {
	for _, rawSection := range sections(target) {
		section, _ := rawSection.(map[string]any)
		blocks, _ := section["blocks"].([]any)
		for _, rawBlock := range blocks {
			block, _ := rawBlock.(map[string]any)
			if intAny(block["id"]) == blockID {
				return true
			}
		}
	}
	return false
}

func budgetSplitWith(mode string, userIDs []int) (map[string]any, error) {
	mode = strings.TrimSpace(mode)
	if mode == "" {
		mode = "noOne"
	}
	if !budgetSplitModes[mode] {
		return nil, fmt.Errorf("--split-with must be noOne, everyone, or individuals")
	}
	if mode != "individuals" {
		return map[string]any{"type": mode, "users": []any{}}, nil
	}
	users := []any{}
	for _, id := range userIDs {
		if id <= 0 {
			return nil, errors.New("--split-user-id must be positive")
		}
		users = append(users, budgetUserRef(id))
	}
	return map[string]any{"type": "individuals", "users": users}, nil
}

func buildBudgetPayment(fromUserID int, toUserID int, amount float64, currency string, defaultCurrency string, paidAt string, jsonValue string) (map[string]any, error) {
	if strings.TrimSpace(jsonValue) != "" {
		return parseJSONObjectFlag(jsonValue, "--json-value")
	}
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if currency == "" {
		currency = strings.ToUpper(strings.TrimSpace(defaultCurrency))
	}
	if currency == "" {
		currency = "USD"
	}
	if paidAt == "" {
		paidAt = time.Now().UTC().Format(time.RFC3339)
	}
	return map[string]any{"id": randomWanderlogID(), "amount": map[string]any{"amount": amount, "currencyCode": currency}, "paidAt": paidAt, "fromUser": budgetUserRef(fromUserID), "toUser": budgetUserRef(toUserID)}, nil
}

func findBudgetItem(items []any, id int, idx int, label string) (int, map[string]any, error) {
	if id != 0 {
		for i, raw := range items {
			m, _ := raw.(map[string]any)
			if intAny(m["id"]) == id {
				return i, cloneJSONMap(m), nil
			}
		}
		return 0, nil, fmt.Errorf("%s id %d not found", label, id)
	}
	if idx < 0 || idx >= len(items) {
		return 0, nil, fmt.Errorf("%s index %d out of range", label, idx)
	}
	m, _ := items[idx].(map[string]any)
	if m == nil {
		return 0, nil, fmt.Errorf("%s index %d is not an object", label, idx)
	}
	return idx, cloneJSONMap(m), nil
}

func budgetUserRef(id int) map[string]any { return map[string]any{"type": "registered", "id": id} }

func parseJSONObjectFlag(value string, name string) (map[string]any, error) {
	var parsed map[string]any
	if err := json.Unmarshal([]byte(value), &parsed); err != nil {
		return nil, fmt.Errorf("parse %s: %w", name, err)
	}
	return parsed, nil
}

func dateOrNil(date string) any {
	date = strings.TrimSpace(date)
	if date == "" {
		return nil
	}
	return date
}

func intOrNil(value int) any {
	if value == 0 {
		return nil
	}
	return value
}

func floatAny(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case float32:
		return float64(x)
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case json.Number:
		f, _ := strconv.ParseFloat(string(x), 64)
		return f
	case string:
		f, _ := strconv.ParseFloat(x, 64)
		return f
	default:
		return 0
	}
}

func firstNonZero(values ...int) int {
	for _, v := range values {
		if v != 0 {
			return v
		}
	}
	return 0
}

func firstNonZeroFloat(values ...float64) float64 {
	for _, v := range values {
		if v != 0 {
			return v
		}
	}
	return 0
}
