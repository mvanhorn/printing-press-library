// Copyright 2026 Matt Anderson and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/actual-budget/internal/actual"
)

// defaultScheduleTolerancePct is the amount-drift tolerance when --tolerance is unset.
const defaultScheduleTolerancePct = 5

func newNovelAuditSchedulesCmd(flags *rootFlags) *cobra.Command {
	var flagTolerance float64
	var flagAsOf string
	var flagAll bool
	var flagLimit int

	cmd := &cobra.Command{
		Use:   "schedules",
		Short: "List bills that are overdue with nothing posted and schedules whose amounts drifted.",
		Long: `Checks every active, non-completed schedule against the transactions linked to
it (transactions.schedule) and reports findings:

  overdue        the schedule's next date is before --as-of and nothing linked
                 to it posted on or after that date
  amount-drift   the latest linked posting differs from the schedule's amount
                 by more than --tolerance percent (outside the range for
                 "is between" schedules)
  never-posted   no transaction has ever been linked to the schedule
  postings-unavailable
                 the mirror has no transactions.schedule column, so postings
                 cannot be linked; the other checks are skipped

Payee, account and expected amount come from the schedule's rule conditions.
Amounts are integer minor units (cents). By default only schedules with
findings are shown; --all shows every active schedule. Reads the local mirror
only.`,
		Example: strings.Trim(`
  actual-budget-pp-cli audit schedules
  actual-budget-pp-cli audit schedules --tolerance 5 --agent
  actual-budget-pp-cli audit schedules --as-of 2026-09-30 --all --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if done, err := mirrorPreflight(cmd, flags, "audit schedules"); done {
				return err
			}
			tol := flagTolerance
			if math.IsNaN(tol) || math.IsInf(tol, 0) || tol < 0 {
				return usageErr(fmt.Errorf("--tolerance must be a percentage >= 0, got %v", tol))
			}
			asOf := actual.DateInt(time.Now())
			if d, ok, err := parseAsOfFlag(flagAsOf); err != nil {
				return err
			} else if ok {
				asOf = d
			}
			if err := checkLimit(flagLimit); err != nil {
				return err
			}
			return withMirror(cmd, flags, make([]actual.ScheduleAudit, 0), func(ctx context.Context, h *mirrorHandle) error {
				all, err := h.Ledger.AuditSchedules(ctx, asOf, tol)
				if err != nil {
					return err
				}
				rows := keepFindings(all, func(r actual.ScheduleAudit) []string { return r.Findings }, flagAll, flagLimit)
				return emitRows(cmd, flags, rows, func() error {
					w := cmd.OutOrStdout()
					if len(rows) == 0 {
						fmt.Fprintln(w, "All schedules look healthy.")
						return nil
					}
					table := make([][]string, 0, len(rows))
					for _, r := range rows {
						last := ""
						if r.LastAmount != nil {
							last = amt(*r.LastAmount)
						}
						table = append(table, []string{r.Name, strings.Join(r.Findings, ", "), r.Payee, amt(r.ExpectedAmount), r.NextDate, r.LastPosted, last})
					}
					return printOrderedTable(w, []string{"SCHEDULE", "FINDINGS", "PAYEE", "EXPECTED", "NEXT_DATE", "LAST_POSTED", "LAST_AMOUNT"}, table)
				})
			})
		},
	}
	cmd.Flags().Float64Var(&flagTolerance, "tolerance", defaultScheduleTolerancePct, "Allowed difference between posted and scheduled amount, in percent")
	cmd.Flags().StringVar(&flagAsOf, "as-of", "", "Date to judge overdue against, YYYY-MM-DD or YYYYMMDD (default today)")
	cmd.Flags().BoolVar(&flagAll, "all", false, "Show every active schedule, not only those with findings")
	cmd.Flags().IntVar(&flagLimit, "limit", 50, "Maximum schedules to return (0 = all)")
	return cmd
}
