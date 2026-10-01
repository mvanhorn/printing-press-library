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

// defaultAuditRulesMonths is the evaluation window when --months is unset.
const defaultAuditRulesMonths = 12

func newNovelAuditRulesCmd(flags *rootFlags) *cobra.Command {
	var flagMonths int
	var flagAll bool
	var flagAsOf string
	var flagLimit int

	cmd := &cobra.Command{
		Use:   "rules",
		Short: "Find rules that never match, are shadowed by another rule, or point at deleted payees and categories.",
		Long: `Evaluates every live rule against live (leaf) transactions from the last --months
months and reports findings per rule:

  never-matches        no transaction in the window satisfies the conditions
  dangling-reference   a condition or action points at a deleted or missing
                       payee, category, account or schedule (see "dangling")
  shadowed-by:<id>     another rule in the same stage has identical conditions
                       but sets a different value for the same field (reported
                       on both rules)
  unevaluated          a condition uses an op/field this audit cannot model
                       (date, tags, ...), so match counts are not computed

Supported ops: is, isNot, contains, doesNotContain, oneOf, notOneOf, matches,
gt/gte/lt/lte, isapprox, isbetween on payee, imported_payee, notes, account,
category and amount. Payee/category conditions compare against each
transaction's RESOLVED id (after payee and category merges): Actual rewrites
rules to the merge target when payees merge, so a rule still pointing at a
merged-away payee never fires. Schedule-linked rules are never reported as
never-matches. By default only rules with findings are shown; --all shows every
rule. Reads the local mirror only.`,
		Example: strings.Trim(`
  actual-budget-pp-cli audit rules
  actual-budget-pp-cli audit rules --months 6 --agent
  actual-budget-pp-cli audit rules --months 0 --all --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if done, err := mirrorPreflight(cmd, flags, "audit rules"); done {
				return err
			}
			months := flagMonths
			if months < 0 {
				return usageErr(fmt.Errorf("--months must be a whole number >= 0 (0 = all history), got %d", months))
			}
			asOf := time.Now()
			if d, ok, err := parseAsOfFlag(flagAsOf); err != nil {
				return err
			} else if ok {
				asOf = actual.DateTime(d)
			}
			if err := checkLimit(flagLimit); err != nil {
				return err
			}
			opt := actual.AuditRulesOptions{}
			if months > 0 {
				opt.From = actual.DateInt(asOf.AddDate(0, -months, 0))
				opt.To = actual.DateInt(asOf)
			}
			return withMirror(cmd, flags, make([]actual.RuleAudit, 0), func(ctx context.Context, h *mirrorHandle) error {
				all, err := h.Ledger.AuditRules(ctx, opt)
				if err != nil {
					return err
				}
				rows := keepFindings(all, func(r actual.RuleAudit) []string { return r.Findings }, flagAll, flagLimit)
				return emitRows(cmd, flags, rows, func() error {
					w := cmd.OutOrStdout()
					if len(rows) == 0 {
						fmt.Fprintln(w, "No rule problems found.")
						return nil
					}
					table := make([][]string, 0, len(rows))
					for _, r := range rows {
						table = append(table, []string{r.ID, strings.Join(r.Findings, ", "), fmt.Sprint(r.MatchCount), r.LastMatch, r.Summary})
					}
					return printOrderedTable(w, []string{"RULE", "FINDINGS", "MATCHES", "LAST_MATCH", "SUMMARY"}, table)
				})
			})
		},
	}
	cmd.Flags().IntVar(&flagMonths, "months", defaultAuditRulesMonths, "Months of transaction history to evaluate rules against (0 = all)")
	cmd.Flags().BoolVar(&flagAll, "all", false, "Show every rule, not only rules with findings")
	cmd.Flags().StringVar(&flagAsOf, "as-of", "", "End of the evaluation window, YYYY-MM-DD or YYYYMMDD (default today)")
	cmd.Flags().IntVar(&flagLimit, "limit", 50, "Maximum rules to return (0 = all)")
	return cmd
}
