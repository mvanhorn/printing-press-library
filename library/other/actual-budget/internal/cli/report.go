// Copyright 2026 Matt Anderson and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/actual-budget/internal/actual"
)

func newReportCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Spending, cashflow, budget, net-worth, trend and recurring-charge reports from the local mirror",
		Long: `Offline reports computed from your local budget mirror (refresh it with
'mirror pull'). All amounts are integer minor units (cents); spending is reported
as positive numbers. Reports use leaf transactions, so a split is counted once
through its parts, and transfers between your own accounts are never income or
spending. On-budget accounts only unless --include-offbudget.`,
		Example: strings.Trim(`
  actual-budget-pp-cli report spending --month 2026-09
  actual-budget-pp-cli report budget-vs-actual --month 2026-09 --agent
  actual-budget-pp-cli report net-worth --months 12`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local", "pp:typed-exit-codes": "0,2"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newReportSpendingCmd(flags), newReportCashflowCmd(flags), newReportBudgetVsActualCmd(flags),
		newReportNetWorthCmd(flags), newReportTrendsCmd(flags), newReportRecurringCmd(flags))
	return cmd
}

// reportOpts holds the flags shared by report subcommands.
type reportOpts struct {
	month, from, to, account string
	includeOffBudget         bool
	months                   int
}

func (o *reportOpts) bindRange(cmd *cobra.Command, monthHelp string) {
	cmd.Flags().StringVar(&o.month, "month", "", monthHelp)
	cmd.Flags().StringVar(&o.from, "from", "", "Start date, inclusive (YYYY-MM-DD); use instead of --month")
	cmd.Flags().StringVar(&o.to, "to", "", "End date, inclusive (YYYY-MM-DD); use instead of --month")
}

func (o *reportOpts) bindAccount(cmd *cobra.Command) {
	cmd.Flags().StringVar(&o.account, "account", "", "Only this account (name or id)")
	cmd.Flags().BoolVar(&o.includeOffBudget, "include-offbudget", false, "Include off-budget (tracking) accounts")
}

// window resolves a multi-month range: --from/--to when given, otherwise the
// o.months months ending at --month (default: current month).
func (o *reportOpts) window() (int, int, error) {
	if o.month != "" && (o.from != "" || o.to != "") {
		return 0, 0, usageErr(errors.New("use either --month or --from/--to, not both"))
	}
	if o.months < 1 {
		return 0, 0, usageErr(errors.New("--months must be >= 1"))
	}
	if o.from != "" || o.to != "" {
		f, t, _, err := parseRange("", o.from, o.to)
		if err != nil {
			return 0, 0, err
		}
		if t == 0 {
			t = actual.DateInt(time.Now())
		}
		if f == 0 {
			f = actual.MonthStart(actual.AddMonths(actual.YMFromDate(t), -(o.months - 1)))
		}
		if f > t {
			return 0, 0, usageErr(errors.New("--from is after --to"))
		}
		return f, t, nil
	}
	month := o.month
	if month == "" {
		month = time.Now().Format("2006-01")
	}
	_, end, ym, err := actual.MonthRange(month)
	if err != nil {
		return 0, 0, usageErr(fmt.Errorf("--month: %w", err))
	}
	return actual.MonthStart(actual.AddMonths(ym, -(o.months - 1))), end, nil
}

func (o *reportOpts) scope(ctx context.Context, h *mirrorHandle, from, to int) (actual.ReportScope, error) {
	acctID, err := ledgerResolveAccount(ctx, h, o.account)
	if err != nil {
		return actual.ReportScope{}, err
	}
	return actual.ReportScope{From: from, To: to, AccountID: acctID, IncludeOffBudget: o.includeOffBudget}, nil
}

func pct(v float64) string { return fmt.Sprintf("%.1f%%", v*100) }

// ---------------------------------------------------------------------------

func newReportSpendingCmd(flags *rootFlags) *cobra.Command {
	var o reportOpts
	var by string
	var limit int
	cmd := &cobra.Command{
		Use:   "spending",
		Short: "Where the money went in a period, by category, payee or category group",
		Long: `Use for where money went in a period. Do NOT use for budget targets; use 'report budget-vs-actual'.

Net spending: outflows (negative leaf amounts) minus refunds (inflows into an
expense category), excluding uncategorized transfers, income categories and
starting balances — the same rule as cashflow and budget-vs-actual. Grouped by
category (default), payee or group. Each row has spent (positive minor units;
a refund-heavy bucket can be negative), count of contributing transactions and
share of the total (0..1); sorted by spent, largest first. Transactions without
a category land in "Uncategorized".`,
		Example: strings.Trim(`
  actual-budget-pp-cli report spending --month 2026-09
  actual-budget-pp-cli report spending --by payee --from 2026-07-01 --to 2026-09-30 --limit 10
  actual-budget-pp-cli report spending --by group --account "Visa Card" --json`, "\n"),
		Annotations: map[string]string{"pp:data-source": "local", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if done, err := mirrorPreflight(cmd, flags, "report spending"); done {
				return err
			}
			by = strings.ToLower(strings.TrimSpace(by))
			if by != "category" && by != "payee" && by != "group" {
				return usageErr(fmt.Errorf("--by must be category, payee or group (got %q)", by))
			}
			if limit < 0 {
				return usageErr(errors.New("--limit must be >= 0"))
			}
			from, to, _, err := parseRange(o.month, o.from, o.to)
			if err != nil {
				return err
			}
			empty := actual.SpendingResult{From: actual.FormatDate(from), To: actual.FormatDate(to), By: by, Rows: make([]actual.SpendingRow, 0)}
			return withMirror(cmd, flags, empty, func(ctx context.Context, h *mirrorHandle) error {
				s, err := o.scope(ctx, h, from, to)
				if err != nil {
					return err
				}
				res, err := h.Ledger.Spending(ctx, s, by)
				if err != nil {
					return err
				}
				if limit > 0 && len(res.Rows) > limit {
					res.Rows = res.Rows[:limit]
				}
				return emitRows(cmd, flags, res, func() error {
					w := cmd.OutOrStdout()
					fmt.Fprintf(w, "Spending %s..%s by %s: %s total\n", res.From, res.To, by, amt(res.Total))
					if len(res.Rows) == 0 {
						fmt.Fprintln(w, "No spending in this period.")
						return nil
					}
					items := make([]map[string]any, 0, len(res.Rows))
					for _, r := range res.Rows {
						items = append(items, map[string]any{"name": r.Name, "spent": amt(r.Spent), "count": r.Count, "share": pct(r.Share)})
					}
					return printAutoTable(w, items)
				})
			})
		},
	}
	o.bindRange(cmd, "Month to report (YYYY-MM, default current month)")
	o.bindAccount(cmd)
	cmd.Flags().StringVar(&by, "by", "category", "Group by: category, payee or group")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum rows (0 = no limit)")
	return cmd
}

// ---------------------------------------------------------------------------

func newReportCashflowCmd(flags *rootFlags) *cobra.Command {
	o := reportOpts{months: 6}
	cmd := &cobra.Command{
		Use:   "cashflow",
		Short: "Income, spending and net per month",
		Long: `Per-month income, spending and net (income - spending) over the --months months
ending at --month (default: the last 6 months up to the current month), or over
--from/--to. Income is positive amounts in income categories plus uncategorized
inflows; spending is net activity outside income categories: outflows minus
refunds (inflows into an expense category), so a refund-heavy month can show
negative spending. Uncategorized transfers and starting balances are excluded;
categorized transfers to off-budget accounts count as spending, as in Actual.
Integer minor units (cents).`,
		Example: strings.Trim(`
  actual-budget-pp-cli report cashflow
  actual-budget-pp-cli report cashflow --month 2026-09 --months 3 --json
  actual-budget-pp-cli report cashflow --from 2026-01-01 --to 2026-09-30`, "\n"),
		Annotations: map[string]string{"pp:data-source": "local", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if done, err := mirrorPreflight(cmd, flags, "report cashflow"); done {
				return err
			}
			from, to, err := o.window()
			if err != nil {
				return err
			}
			return withMirror(cmd, flags, make([]actual.CashflowMonth, 0), func(ctx context.Context, h *mirrorHandle) error {
				s, err := o.scope(ctx, h, from, to)
				if err != nil {
					return err
				}
				rows, err := h.Ledger.Cashflow(ctx, s)
				if err != nil {
					return err
				}
				return emitRows(cmd, flags, rows, func() error {
					items := make([]map[string]any, 0, len(rows))
					for _, r := range rows {
						items = append(items, map[string]any{"month": r.Month, "income": amt(r.Income), "spending": amt(r.Spending), "net": amt(r.Net)})
					}
					return printAutoTable(cmd.OutOrStdout(), items)
				})
			})
		},
	}
	o.bindRange(cmd, "Last month of the window (YYYY-MM, default current month)")
	o.bindAccount(cmd)
	cmd.Flags().IntVar(&o.months, "months", 6, "Number of months ending at --month")
	return cmd
}

// ---------------------------------------------------------------------------

func newReportBudgetVsActualCmd(flags *rootFlags) *cobra.Command {
	var month, account string
	cmd := &cobra.Command{
		Use:   "budget-vs-actual",
		Short: "Budgeted vs spent per category for a month, flagging overspending",
		Long: `For one month, each expense category's budgeted amount, what was spent
(category activity on on-budget accounts, positive = spent, refunds reduce it),
remaining = budgeted - spent, and over_budget when spent exceeds the budget.
Categorized transfers to off-budget accounts count as spending, as in Actual.
Integer minor units (cents). For where money went regardless of budget, use
'report spending'.

--account narrows only the spent side: budgeted amounts are always the
budget-wide figures (Actual budgets per category, not per account), so with
--account, remaining and over_budget compare that account's activity against
the whole category budget.`,
		Example: strings.Trim(`
  actual-budget-pp-cli report budget-vs-actual --month 2026-09
  actual-budget-pp-cli report budget-vs-actual --month 2026-09 --agent --select rows.category,rows.over_budget`, "\n"),
		Annotations: map[string]string{"pp:data-source": "local", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if done, err := mirrorPreflight(cmd, flags, "report budget-vs-actual"); done {
				return err
			}
			if month == "" {
				month = time.Now().Format("2006-01")
			}
			_, _, ym, err := actual.MonthRange(month)
			if err != nil {
				return usageErr(fmt.Errorf("--month: %w", err))
			}
			empty := actual.BudgetVsActualResult{Month: actual.FormatYM(ym), Rows: make([]actual.BudgetLine, 0)}
			return withMirror(cmd, flags, empty, func(ctx context.Context, h *mirrorHandle) error {
				acctID, err := ledgerResolveAccount(ctx, h, account)
				if err != nil {
					return err
				}
				res, err := h.Ledger.BudgetVsActual(ctx, ym, acctID)
				if err != nil {
					return err
				}
				return emitRows(cmd, flags, res, func() error {
					w := cmd.OutOrStdout()
					fmt.Fprintf(w, "%s: budgeted %s, spent %s, remaining %s\n", res.Month, amt(res.TotalBudgeted), amt(res.TotalSpent), amt(res.TotalRemaining))
					items := make([]map[string]any, 0, len(res.Rows))
					for _, r := range res.Rows {
						flag := ""
						if r.OverBudget {
							flag = "OVER"
						}
						items = append(items, map[string]any{"group": r.Group, "category": r.Category, "budgeted": amt(r.Budgeted), "spent": amt(r.Spent), "remaining": amt(r.Remaining), "status": flag})
					}
					return printAutoTable(w, items)
				})
			})
		},
	}
	cmd.Flags().StringVar(&month, "month", "", "Budget month (YYYY-MM, default current month)")
	cmd.Flags().StringVar(&account, "account", "", "Only count activity in this account (name or id); budgeted stays budget-wide")
	return cmd
}

// ---------------------------------------------------------------------------

func newReportNetWorthCmd(flags *rootFlags) *cobra.Command {
	o := reportOpts{months: 12}
	var byAccount bool
	cmd := &cobra.Command{
		Use:   "net-worth",
		Short: "Net worth at each month end, across every account including off-budget",
		Long: `Sums the balances of every live account — on-budget and off-budget, open and
closed — as of each month end in the window (default: 12 months ending at
--month), with the change from the previous month. --by-account adds each
account's balance. Integer minor units (cents).`,
		Example: strings.Trim(`
  actual-budget-pp-cli report net-worth
  actual-budget-pp-cli report net-worth --month 2026-09 --months 6 --by-account --json`, "\n"),
		Annotations: map[string]string{"pp:data-source": "local", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if done, err := mirrorPreflight(cmd, flags, "report net-worth"); done {
				return err
			}
			from, to, err := o.window()
			if err != nil {
				return err
			}
			months := actual.MonthsBetween(from, to)
			return withMirror(cmd, flags, make([]actual.NetWorthPoint, 0), func(ctx context.Context, h *mirrorHandle) error {
				rows, err := h.Ledger.NetWorth(ctx, months, byAccount)
				if err != nil {
					return err
				}
				return emitRows(cmd, flags, rows, func() error {
					// Fixed column order: month, net_worth, change, then one
					// column per account in ledger order (keyed by id so
					// duplicate names stay separate).
					headers := []string{"month", "net_worth", "change"}
					var acctIDs []string
					seen := map[string]bool{}
					for _, r := range rows {
						for _, a := range r.Accounts {
							if !seen[a.ID] {
								seen[a.ID] = true
								acctIDs = append(acctIDs, a.ID)
								headers = append(headers, a.Name)
							}
						}
					}
					table := make([][]string, 0, len(rows))
					for _, r := range rows {
						bal := map[string]int64{}
						for _, a := range r.Accounts {
							bal[a.ID] = a.Balance
						}
						line := []string{r.Month, amt(r.NetWorth), amt(r.Change)}
						for _, id := range acctIDs {
							line = append(line, amt(bal[id]))
						}
						table = append(table, line)
					}
					return printOrderedTable(cmd.OutOrStdout(), headers, table)
				})
			})
		},
	}
	o.bindRange(cmd, "Last month of the window (YYYY-MM, default current month)")
	cmd.Flags().IntVar(&o.months, "months", 12, "Number of month ends ending at --month")
	cmd.Flags().BoolVar(&byAccount, "by-account", false, "Include each account's balance")
	return cmd
}

// ---------------------------------------------------------------------------

func newReportTrendsCmd(flags *rootFlags) *cobra.Command {
	o := reportOpts{months: 6}
	var category string
	var limit int
	cmd := &cobra.Command{
		Use:   "trends",
		Short: "Per-category spending month by month, with average and latest-vs-average change",
		Long: `For each category, spending (positive minor units) in every month of the window
(default: 6 months ending at --month), the monthly average, the latest month,
and delta = latest - average (delta_pct relative to the average). Same spending
rules as 'report spending'. Sorted by total spending.`,
		Example: strings.Trim(`
  actual-budget-pp-cli report trends
  actual-budget-pp-cli report trends --category Groceries --month 2026-09 --months 3 --json`, "\n"),
		Annotations: map[string]string{"pp:data-source": "local", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if done, err := mirrorPreflight(cmd, flags, "report trends"); done {
				return err
			}
			if limit < 0 {
				return usageErr(errors.New("--limit must be >= 0"))
			}
			from, to, err := o.window()
			if err != nil {
				return err
			}
			return withMirror(cmd, flags, make([]actual.TrendRow, 0), func(ctx context.Context, h *mirrorHandle) error {
				s, err := o.scope(ctx, h, from, to)
				if err != nil {
					return err
				}
				catID, err := ledgerResolveCategory(ctx, h, category)
				if err != nil {
					return err
				}
				rows, err := h.Ledger.Trends(ctx, s, catID)
				if err != nil {
					return err
				}
				if limit > 0 && len(rows) > limit {
					rows = rows[:limit]
				}
				return emitRows(cmd, flags, rows, func() error {
					if len(rows) == 0 {
						fmt.Fprintln(cmd.OutOrStdout(), "No spending in this window.")
						return nil
					}
					// Fixed column order: category, each month oldest first,
					// then average, latest and delta.
					headers := []string{"category"}
					for _, p := range rows[0].Series {
						headers = append(headers, p.Month)
					}
					headers = append(headers, "average", "latest", "delta")
					table := make([][]string, 0, len(rows))
					for _, r := range rows {
						line := []string{r.Category}
						for _, p := range r.Series {
							line = append(line, amt(p.Spent))
						}
						table = append(table, append(line, amt(r.Average), amt(r.Latest), amt(r.Delta)))
					}
					return printOrderedTable(cmd.OutOrStdout(), headers, table)
				})
			})
		},
	}
	o.bindRange(cmd, "Last month of the window (YYYY-MM, default current month)")
	o.bindAccount(cmd)
	cmd.Flags().IntVar(&o.months, "months", 6, "Number of months ending at --month")
	cmd.Flags().StringVar(&category, "category", "", "Only this category (name or id)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum categories (0 = no limit)")
	return cmd
}

// ---------------------------------------------------------------------------

func newReportRecurringCmd(flags *rootFlags) *cobra.Command {
	o := reportOpts{months: 12}
	var minOccurrences, limit int
	cmd := &cobra.Command{
		Use:   "recurring",
		Short: "Detect subscriptions and bills: payees charging similar amounts month after month",
		Long: fmt.Sprintf(`Finds payees with outflows in at least --min-occurrences distinct months of the
window (default: 12 months ending at --month) whose amounts vary by at most %.0f%%
(stddev/mean). Each row has occurrences, avg_amount (positive minor units),
cadence_days (average gap), last_date, next_expected, and scheduled = true when
any of its transactions was posted by an Actual schedule — scheduled=false rows
are recurring charges you are not tracking yet.`, actual.RecurringMaxVariation*100),
		Example: strings.Trim(`
  actual-budget-pp-cli report recurring
  actual-budget-pp-cli report recurring --month 2026-09 --min-occurrences 4 --json`, "\n"),
		Annotations: map[string]string{"pp:data-source": "local", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if done, err := mirrorPreflight(cmd, flags, "report recurring"); done {
				return err
			}
			if minOccurrences < 2 {
				return usageErr(errors.New("--min-occurrences must be >= 2"))
			}
			if limit < 0 {
				return usageErr(errors.New("--limit must be >= 0"))
			}
			from, to, err := o.window()
			if err != nil {
				return err
			}
			return withMirror(cmd, flags, make([]actual.RecurringPayee, 0), func(ctx context.Context, h *mirrorHandle) error {
				s, err := o.scope(ctx, h, from, to)
				if err != nil {
					return err
				}
				rows, err := h.Ledger.Recurring(ctx, s, minOccurrences)
				if err != nil {
					return err
				}
				if limit > 0 && len(rows) > limit {
					rows = rows[:limit]
				}
				return emitRows(cmd, flags, rows, func() error {
					if len(rows) == 0 {
						fmt.Fprintln(cmd.OutOrStdout(), "No recurring payees detected.")
						return nil
					}
					items := make([]map[string]any, 0, len(rows))
					for _, r := range rows {
						items = append(items, map[string]any{"payee": r.Payee, "occurrences": r.Occurrences, "avg_amount": amt(r.AvgAmount),
							"cadence_days": r.CadenceDays, "last_date": r.LastDate, "next_expected": r.NextExpected, "scheduled": r.Scheduled})
					}
					return printAutoTable(cmd.OutOrStdout(), items)
				})
			})
		},
	}
	o.bindRange(cmd, "Last month of the window (YYYY-MM, default current month)")
	o.bindAccount(cmd)
	cmd.Flags().IntVar(&o.months, "months", 12, "Number of months ending at --month")
	cmd.Flags().IntVar(&minOccurrences, "min-occurrences", 3, "Minimum distinct months with a charge")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum rows (0 = no limit)")
	return cmd
}
