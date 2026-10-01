// Copyright 2026 Matt Anderson and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/actual-budget/internal/actual"
)

func newNovelTemplatesStatusCmd(flags *rootFlags) *cobra.Command {
	var flagMonth string
	var flagStatus string

	cmd := &cobra.Command{
		Use:   "status",
		Short: "See which envelopes are under- or over-funded against the #template and #goal targets in category notes.",
		Long: `Use this command to check envelope funding against the #template/#goal targets written in category notes. Do NOT use this command for budgeted-versus-spent comparisons; use 'report budget-vs-actual' instead.

Templates are read from the category's goal_def column (Actual's newer
template storage; types simple, monthly periodic and goal) or, when that is empty, from
"#template <amount>", "#template up to <amount>" and "#goal <amount>" lines in
the category note. Other template forms (by-date, percentages, schedules) are
listed with status "unsupported". For each category: target, budgeted for the
month, spent this month, balance_hint (budgeted minus spent, ignoring
carryover) and status: underfunded, funded, overfunded, or goal (long-term
savings target). All amounts are integer minor units (cents). Reads the local
mirror only.`,
		Example: strings.Trim(`
  actual-budget-pp-cli templates status
  actual-budget-pp-cli templates status --month 2026-09 --agent
  actual-budget-pp-cli templates status --month 2026-09 --status underfunded --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if done, err := mirrorPreflight(cmd, flags, "templates status"); done {
				return err
			}
			month := strings.TrimSpace(flagMonth)
			if month == "" {
				month = time.Now().Format("2006-01")
			}
			if _, _, _, err := actual.MonthRange(month); err != nil {
				return usageErr(fmt.Errorf("--month: %w", err))
			}
			switch flagStatus {
			case "", "underfunded", "funded", "overfunded", "goal", "unsupported":
			default:
				return usageErr(fmt.Errorf("--status must be one of underfunded, funded, overfunded, goal, unsupported; got %q", flagStatus))
			}
			return withMirror(cmd, flags, make([]actual.TemplateStatus, 0), func(ctx context.Context, h *mirrorHandle) error {
				all, err := h.Ledger.TemplateStatuses(ctx, month)
				if err != nil {
					return err
				}
				rows := make([]actual.TemplateStatus, 0, len(all))
				for _, r := range all {
					if flagStatus == "" || r.Status == flagStatus {
						rows = append(rows, r)
					}
				}
				return emitRows(cmd, flags, rows, func() error {
					w := cmd.OutOrStdout()
					if len(rows) == 0 {
						fmt.Fprintf(w, "No #template or #goal targets found for %s.\n", month)
						return nil
					}
					table := make([][]string, 0, len(rows))
					for _, r := range rows {
						table = append(table, []string{r.Group, r.Category, r.TemplateType, amt(r.Target), amt(r.Budgeted), amt(r.Spent), r.Status})
					}
					return printOrderedTable(w, []string{"GROUP", "CATEGORY", "TEMPLATE", "TARGET", "BUDGETED", "SPENT", "STATUS"}, table)
				})
			})
		},
	}
	cmd.Flags().StringVar(&flagMonth, "month", "", "Budget month YYYY-MM (default current month)")
	cmd.Flags().StringVar(&flagStatus, "status", "", "Only show rows with this status (underfunded, funded, overfunded, goal, unsupported)")
	return cmd
}
