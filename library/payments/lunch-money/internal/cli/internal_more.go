package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/lunch-money/internal/internalapi"

	"github.com/spf13/cobra"
)

// Wire all the remaining internal-API command groups onto the existing
// `internal` parent set up in internal.go.
func init() {
	internalSubcommandFactories = append(internalSubcommandFactories,
		newAutocategorizeCmd,
		newInternalAssetsCmd,
		newReviewCmd,
		newRecurringCmd,
		newBudgetCmd,
		newAPITokensCmd,
		newInternalMeCmd,
		newInternalBalanceHistoryCmd,
	)
}

// internalSubcommandFactories is populated via init() functions. The internal.go
// parent reads from this slice when building the command tree. See
// addInternalExtras below.
var internalSubcommandFactories []func(*rootFlags) *cobra.Command

// addInternalExtras is called by newInternalCmd to attach init-registered subcommands.
func addInternalExtras(parent *cobra.Command, flags *rootFlags) {
	for _, f := range internalSubcommandFactories {
		parent.AddCommand(f(flags))
	}
}

// ----------------------------------------------------------------------------
// autocategorize (Plaid taxonomy mapping)
// ----------------------------------------------------------------------------

func newAutocategorizeCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "autocategorize",
		Short: "Plaid taxonomy → Lunch Money category mappings",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List Plaid taxonomy with current LM category mappings",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newInternalClient()
			if err != nil {
				return err
			}
			cats, err := c.ListPlaidCategories()
			if err != nil {
				return err
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(cats)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "populate",
		Short: "Refresh the Plaid taxonomy (POST /plaid/categories/populate)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeInternalDryRun(cmd, "POST", "/plaid/categories/populate", map[string]any{})
			}
			c, err := newInternalClient()
			if err != nil {
				return err
			}
			if err := c.PopulatePlaidCategories(); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "ok")
			return nil
		},
	})
	return cmd
}

// ----------------------------------------------------------------------------
// assets — internal /assets (richer than public manual_accounts)
// ----------------------------------------------------------------------------

func newInternalAssetsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "assets",
		Short: "Accounts via internal /assets endpoint (full Plaid + manual)",
	}
	var showSecrets bool
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all accounts. Plaid access_tokens are redacted by default.",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newInternalClient()
			if err != nil {
				return err
			}
			assets, err := c.ListAssets()
			if err != nil {
				return err
			}
			if !showSecrets {
				for i := range assets {
					assets[i].AccessToken = ""
					assets[i].ItemID = ""
				}
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(assets)
		},
	}
	listCmd.Flags().BoolVar(&showSecrets, "show-secrets", false, "Include Plaid access_tokens (⚠ sensitive)")
	cmd.AddCommand(listCmd)

	cmd.AddCommand(&cobra.Command{
		Use:   "status <asset_id>",
		Short: "Check dependencies (hasTransaction/hasRecurring/hasBalanceHistory) before delete",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("usage: assets status <id>")
			}
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return err
			}
			c, err := newInternalClient()
			if err != nil {
				return err
			}
			s, err := c.GetAssetStatus(id)
			if err != nil {
				return err
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(s)
		},
	})

	var keepItems bool
	delCmd := &cobra.Command{
		Use:   "delete <asset_id>",
		Short: "Soft-delete a manual account (or unlink Plaid + keep items)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("usage: assets delete <id>")
			}
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return err
			}
			if dryRunOK(flags) {
				return writeInternalDryRun(cmd, "PUT", fmt.Sprintf("/assets/%d/delete", id), map[string]any{"keep_items": keepItems})
			}
			c, err := newInternalClient()
			if err != nil {
				return err
			}
			if err := c.DeleteAsset(id, keepItems); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "ok")
			return nil
		},
	}
	delCmd.Flags().BoolVar(&keepItems, "keep-items", false, "Unlink Plaid but keep historical transactions")
	cmd.AddCommand(delCmd)

	cmd.AddCommand(&cobra.Command{
		Use:   "subtypes",
		Short: "List valid type/subtype combinations for manual accounts",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newInternalClient()
			if err != nil {
				return err
			}
			s, err := c.AssetSubtypes()
			if err != nil {
				return err
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(s)
		},
	})
	return cmd
}

// ----------------------------------------------------------------------------
// review — transaction status / bulk-edit workflow
// ----------------------------------------------------------------------------

func newReviewCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "review",
		Short: "Review unreviewed transactions and bulk-edit",
	}

	var startDate, endDate string
	var unreviewed, minimal bool
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List transactions with internal-API filters",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newInternalClient()
			if err != nil {
				return err
			}
			res, err := c.ListTransactions(internalapi.TransactionFilter{
				StartDate:    startDate,
				EndDate:      endDate,
				IsUnreviewed: unreviewed,
				Match:        "all",
				Minimal:      minimal,
				Paginate:     true,
			})
			if err != nil {
				return err
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(res)
		},
	}
	listCmd.Flags().StringVar(&startDate, "start-date", "", "YYYY-MM-DD")
	listCmd.Flags().StringVar(&endDate, "end-date", "", "YYYY-MM-DD")
	listCmd.Flags().BoolVar(&unreviewed, "unreviewed", false, "Only unreviewed transactions")
	listCmd.Flags().BoolVar(&minimal, "minimal", false, "Smaller payload (skip joined data)")
	cmd.AddCommand(listCmd)

	// review mark <ids>... --reviewed|--unreviewed
	var markReviewed, markUnreviewed bool
	markCmd := &cobra.Command{
		Use:   "mark <tx_id>...",
		Short: "Mark transactions reviewed/unreviewed (bulk)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("provide at least one transaction id")
			}
			if markReviewed == markUnreviewed {
				return fmt.Errorf("specify exactly one of --reviewed / --unreviewed")
			}
			status := "cleared"
			if markUnreviewed {
				status = "uncleared"
			}
			if dryRunOK(flags) {
				return writeInternalDryRun(cmd, "PUT", "/transactions/bulk_update", map[string]any{
					"transactionIds": args,
					"updateObj":      internalapi.TransactionUpdate{Status: &status},
				})
			}
			c, err := newInternalClient()
			if err != nil {
				return err
			}
			res, err := c.BulkUpdateTransactions(args, internalapi.TransactionUpdate{Status: &status})
			if err != nil {
				return err
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(res)
		},
	}
	markCmd.Flags().BoolVar(&markReviewed, "reviewed", false, "Mark as reviewed (cleared)")
	markCmd.Flags().BoolVar(&markUnreviewed, "unreviewed", false, "Mark as unreviewed (uncleared)")
	cmd.AddCommand(markCmd)

	// review bulk-edit <ids>... --category-id N --notes X
	var bulkCat int64
	var bulkNotes, bulkPayee string
	editCmd := &cobra.Command{
		Use:   "bulk-edit <tx_id>...",
		Short: "Bulk-update writable fields on a set of transactions",
		Example: `  review bulk-edit 12345 12346 --category-id 12345
  review bulk-edit 12345 --payee "Gregory Miller" --notes "lawyer retainer"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("provide at least one transaction id")
			}
			upd := internalapi.TransactionUpdate{}
			if bulkCat != 0 {
				upd.CategoryID = &bulkCat
			}
			if bulkNotes != "" {
				upd.Notes = &bulkNotes
			}
			if bulkPayee != "" {
				upd.Payee = &bulkPayee
			}
			if dryRunOK(flags) {
				return writeInternalDryRun(cmd, "PUT", "/transactions/bulk_update", map[string]any{
					"transactionIds": args,
					"updateObj":      upd,
				})
			}
			c, err := newInternalClient()
			if err != nil {
				return err
			}
			res, err := c.BulkUpdateTransactions(args, upd)
			if err != nil {
				return err
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(res)
		},
	}
	editCmd.Flags().Int64Var(&bulkCat, "category-id", 0, "Set category")
	editCmd.Flags().StringVar(&bulkNotes, "notes", "", "Set notes")
	editCmd.Flags().StringVar(&bulkPayee, "payee", "", "Set payee")
	cmd.AddCommand(editCmd)
	return cmd
}

// ----------------------------------------------------------------------------
// recurring (internal /recurring_items)
// ----------------------------------------------------------------------------

func newRecurringCmd(_ *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "recurring",
		Short: "Recurring items via internal API",
	}
	var startDate, endDate string
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List recurring items",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newInternalClient()
			if err != nil {
				return err
			}
			items, err := c.ListRecurringItems(startDate, endDate)
			if err != nil {
				return err
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(items)
		},
	}
	listCmd.Flags().StringVar(&startDate, "start-date", "", "YYYY-MM-DD")
	listCmd.Flags().StringVar(&endDate, "end-date", "", "YYYY-MM-DD")
	cmd.AddCommand(listCmd)
	return cmd
}

// ----------------------------------------------------------------------------
// budget (uses /summary)
// ----------------------------------------------------------------------------

func newBudgetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "budget",
		Short: "Budget data via internal /summary endpoint",
	}
	var startDate, endDate string
	var includeOccurrences, includeRecurring, includeRollover, includeProperties, includePast bool
	showCmd := &cobra.Command{
		Use:     "show",
		Short:   "Show budget + spend per category for a date range",
		Example: `  budget show --start-date 2026-05-01 --end-date 2026-05-31 --include-recurring`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if startDate == "" || endDate == "" {
				now := time.Now()
				if startDate == "" {
					startDate = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
				}
				if endDate == "" {
					endDate = now.Format("2006-01-02")
				}
			}
			c, err := newInternalClient()
			if err != nil {
				return err
			}
			opts := internalapi.SummaryOptions{StartDate: startDate, EndDate: endDate}
			if includeOccurrences {
				opts.Include("include_occurrences")
			}
			if includeRecurring {
				opts.Include("include_recurring_items")
			}
			if includeRollover {
				opts.Include("include_rollover_pool")
			}
			if includeProperties {
				opts.Include("include_budget_properties")
			}
			if includePast {
				opts.Include("include_past_budget_dates")
			}
			opts.Include("include_totals")
			res, err := c.Summary(opts)
			if err != nil {
				return err
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(res)
		},
	}
	showCmd.Flags().StringVar(&startDate, "start-date", "", "YYYY-MM-DD (defaults to month start)")
	showCmd.Flags().StringVar(&endDate, "end-date", "", "YYYY-MM-DD (defaults to today)")
	showCmd.Flags().BoolVar(&includeOccurrences, "include-occurrences", false, "Include per-month occurrences")
	showCmd.Flags().BoolVar(&includeRecurring, "include-recurring", true, "Include recurring items in totals")
	showCmd.Flags().BoolVar(&includeRollover, "include-rollover", false, "Include rollover pool data")
	showCmd.Flags().BoolVar(&includeProperties, "include-properties", false, "Include budget properties")
	showCmd.Flags().BoolVar(&includePast, "include-past", false, "Include past budget dates")
	cmd.AddCommand(showCmd)

	var setCategoryID int64
	var setStartDate, setAmount, setCurrency, setNotes string
	setCmd := &cobra.Command{
		Use:   "set",
		Short: "Set a category budget for a period",
		RunE: func(cmd *cobra.Command, args []string) error {
			if setCategoryID == 0 {
				return fmt.Errorf("--category-id is required")
			}
			if setStartDate == "" {
				return fmt.Errorf("--start-date is required")
			}
			if setAmount == "" {
				return fmt.Errorf("--amount is required")
			}
			req := internalapi.BudgetUpsert{
				CategoryID: setCategoryID,
				StartDate:  setStartDate,
				Amount:     setAmount,
				Currency:   setCurrency,
				Notes:      setNotes,
			}
			if dryRunOK(flags) {
				return writeInternalDryRun(cmd, "PUT", "/v2/budgets", req)
			}
			c, err := newInternalClient()
			if err != nil {
				return err
			}
			// PATCH: Expose the captured internal-host v2 budget write path.
			res, err := c.UpsertBudget(req)
			if err != nil {
				return err
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(res)
		},
	}
	setCmd.Flags().Int64Var(&setCategoryID, "category-id", 0, "Category ID")
	setCmd.Flags().StringVar(&setStartDate, "start-date", "", "Budget period start date, YYYY-MM-DD")
	setCmd.Flags().StringVar(&setAmount, "amount", "", "Budget amount")
	setCmd.Flags().StringVar(&setCurrency, "currency", "", "Currency, e.g. usd")
	setCmd.Flags().StringVar(&setNotes, "notes", "", "Optional notes")
	cmd.AddCommand(setCmd)

	var clearCategoryID int64
	var clearStartDate string
	clearCmd := &cobra.Command{
		Use:   "clear",
		Short: "Clear a category budget for a period",
		RunE: func(cmd *cobra.Command, args []string) error {
			if clearCategoryID == 0 {
				return fmt.Errorf("--category-id is required")
			}
			if clearStartDate == "" {
				return fmt.Errorf("--start-date is required")
			}
			if dryRunOK(flags) {
				return writeInternalDryRun(cmd, "DELETE", "/v2/budgets", map[string]any{
					"category_id": clearCategoryID,
					"start_date":  clearStartDate,
				})
			}
			c, err := newInternalClient()
			if err != nil {
				return err
			}
			// PATCH: Expose the captured budget delete/clear path.
			if err := c.DeleteBudget(clearCategoryID, clearStartDate); err != nil {
				return err
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{"cleared": true})
		},
	}
	clearCmd.Flags().Int64Var(&clearCategoryID, "category-id", 0, "Category ID")
	clearCmd.Flags().StringVar(&clearStartDate, "start-date", "", "Budget period start date, YYYY-MM-DD")
	cmd.AddCommand(clearCmd)
	return cmd
}

// ----------------------------------------------------------------------------
// api-tokens
// ----------------------------------------------------------------------------

func newAPITokensCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "api-tokens",
		Short: "Manage Lunch Money API tokens issued to your account",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List issued API tokens",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newInternalClient()
			if err != nil {
				return err
			}
			toks, err := c.ListAPITokens()
			if err != nil {
				return err
			}
			if len(toks) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "(no tokens)")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-8s  %-30s  %-12s  %s\n", "ID", "LABEL", "STATUS", "LAST_ACTIVITY")
			for _, t := range toks {
				fmt.Fprintf(cmd.OutOrStdout(), "%-8d  %-30s  %-12s  %s\n",
					t.ID, truncStr(t.Label, 30), t.Status, t.LastActivity.Format(time.RFC3339))
			}
			return nil
		},
	})
	var yes bool
	delCmd := &cobra.Command{
		Use:   "revoke <id>",
		Short: "Revoke an API token by id",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("usage: api-tokens revoke <id>")
			}
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return err
			}
			if dryRunOK(flags) {
				return writeInternalDryRun(cmd, "DELETE", fmt.Sprintf("/api_tokens/%d", id), nil)
			}
			if !yes {
				fmt.Fprintf(cmd.OutOrStdout(), "Re-run with --yes to revoke token %d\n", id)
				return nil
			}
			c, err := newInternalClient()
			if err != nil {
				return err
			}
			if err := c.DeleteAPIToken(id); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "ok: revoked")
			return nil
		},
	}
	delCmd.Flags().BoolVar(&yes, "yes", false, "Confirm")
	cmd.AddCommand(delCmd)

	var newLabel, newReason string
	createCmd := &cobra.Command{
		Use:     "create",
		Short:   "Issue a new public-API token (returns the raw secret once)",
		Example: `  api-tokens create --label "my-script"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if newLabel == "" {
				return fmt.Errorf("--label is required")
			}
			if dryRunOK(flags) {
				return writeInternalDryRun(cmd, "POST", "/api_tokens", map[string]any{
					"label":            newLabel,
					"apiKeyReason":     nil,
					"apiKeyReasonText": newReason,
				})
			}
			c, err := newInternalClient()
			if err != nil {
				return err
			}
			tok, err := c.CreateAPIToken(newLabel, newReason)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "TOKEN: %s\n", tok)
			fmt.Fprintln(cmd.OutOrStderr(), "(save this now — it cannot be retrieved later)")
			return nil
		},
	}
	createCmd.Flags().StringVar(&newLabel, "label", "", "Label for the new token")
	createCmd.Flags().StringVar(&newReason, "reason", "", "Free-text reason (optional)")
	cmd.AddCommand(createCmd)
	return cmd
}

// ----------------------------------------------------------------------------
// me / billing / balance-history convenience
// ----------------------------------------------------------------------------

func newInternalMeCmd(_ *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "me",
		Short: "Show the authenticated user (via internal /me)",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newInternalClient()
			if err != nil {
				return err
			}
			me, err := c.GetMe()
			if err != nil {
				return err
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(me)
		},
	}
}

func newInternalBalanceHistoryCmd(_ *rootFlags) *cobra.Command {
	var startDate, endDate string
	cmd := &cobra.Command{
		Use:   "balance-history",
		Short: "Net worth time series (via internal /balance_history)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if startDate == "" {
				startDate = time.Now().AddDate(-1, 0, 0).Format("2006-01-02")
			}
			if endDate == "" {
				endDate = time.Now().Format("2006-01-02")
			}
			c, err := newInternalClient()
			if err != nil {
				return err
			}
			res, err := c.BalanceHistory(startDate, endDate)
			if err != nil {
				return err
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(res)
		},
	}
	cmd.Flags().StringVar(&startDate, "start-date", "", "YYYY-MM-DD")
	cmd.Flags().StringVar(&endDate, "end-date", "", "YYYY-MM-DD")
	return cmd
}

func truncStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

var _ = strings.HasPrefix // keep imports tidy
