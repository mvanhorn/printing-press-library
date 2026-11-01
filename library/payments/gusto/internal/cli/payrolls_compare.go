// Copyright 2026 david and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/gusto/internal/gusto"
)

// compareLine is one total compared across two payrolls.
type compareLine struct {
	Field    string  `json:"field"`
	Draft    float64 `json:"draft"`
	Previous float64 `json:"previous"`
	Change   float64 `json:"change"`
}

func newNovelPayrollsCompareCmd(flags *rootFlags) *cobra.Command {
	var t draftTarget
	var noCalculate bool

	cmd := &cobra.Command{
		Use:   "compare",
		Short: "See how this draft's gross, reimbursements, taxes, net and debit moved against the last processed payroll.",
		Long: "Compare the draft payroll's calculated totals with the most recent processed payroll and\n" +
			"show the change in total debit and payable tax, plus the draft's gross, reimbursements, taxes\n" +
			"and net. Calculates the draft first unless --no-calculate is set. Never submits.\n\n" +
			"Use this command to see how this draft's totals moved against the last processed payroll.\n" +
			"Do NOT use it to check the draft against the source timesheet; use 'payrolls verify' instead.",
		Example:     "  gusto-pp-cli payrolls compare --period-end 2026-11-01 --agent",
		Annotations: map[string]string{"pp:data-source": "live", "pp:live-happy-path": "true", "pp:happy-args": "--no-calculate"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "payrolls compare")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			api, err := newGustoAPI(flags)
			if err != nil {
				return err
			}
			history, err := api.History(ctx, 5)
			if err != nil {
				return gustoErr(err)
			}
			d, err := resolveDraftWith(ctx, api, t, history)
			if err != nil {
				return gustoErr(err)
			}
			r, err := api.Review(ctx, d.ID, !noCalculate)
			if err != nil {
				return gustoErr(err)
			}
			var prev *gusto.ProcessedPayroll
			for i := range history {
				if !history[i].OffCycle && history[i].PeriodEnd < d.PeriodEnd {
					prev = &history[i]
					break
				}
			}
			lines := make([]compareLine, 0, 2)
			if prev != nil {
				lines = append(lines,
					compareLine{Field: "total_debit", Draft: r.Debit, Previous: prev.Debit, Change: gusto.RoundCents(r.Debit - prev.Debit)},
					compareLine{Field: "total_payable_tax", Draft: gusto.RoundCents(r.EmployeeTaxes + r.EmployerTaxes), Previous: prev.Tax, Change: gusto.RoundCents(r.EmployeeTaxes + r.EmployerTaxes - prev.Tax)},
				)
			}
			view := map[string]any{"draft_id": d.ID, "period_end": d.PeriodEnd, "draft": r, "previous": prev, "changes": lines}
			uncalculated := r.Gross == 0 && r.Debit == 0
			if prev == nil {
				view["note"] = "no earlier processed regular payroll to compare against"
			} else if uncalculated {
				view["note"] = uncalculatedDraftNote(noCalculate)
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Draft %s to %s: gross %s, reimbursements %s, employee taxes %s, company taxes %s, net %s, debit %s\n",
				d.PeriodStart, d.PeriodEnd, money(r.Gross), money(r.Reimbursements), money(r.EmployeeTaxes), money(r.EmployerTaxes), money(r.Net), money(r.Debit))
			if prev == nil {
				fmt.Fprintln(cmd.OutOrStdout(), "No earlier processed payroll to compare against.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Previous %s to %s: debit %s, payable tax %s\n", prev.PeriodStart, prev.PeriodEnd, money(prev.Debit), money(prev.Tax))
			for _, l := range lines {
				fmt.Fprintf(cmd.OutOrStdout(), "  %-18s %s -> %s (%s)\n", l.Field, money(l.Previous), money(l.Draft), signedMoney(l.Change))
			}
			if uncalculated {
				fmt.Fprintf(cmd.OutOrStdout(), "Note: %s.\n", uncalculatedDraftNote(noCalculate))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&t.periodEnd, "period-end", "", periodEndFlagHelp)
	cmd.Flags().StringVar(&t.draftID, "draft-id", "", draftIDFlagHelp)
	cmd.Flags().BoolVar(&noCalculate, "no-calculate", false, "Use the draft's last calculated totals without recalculating")
	return cmd
}

// signedMoney formats a change in dollars with an explicit sign, e.g. -$352.91.
func signedMoney(v float64) string {
	if v < 0 {
		return "-" + money(-v)
	}
	return "+" + money(v)
}

// uncalculatedDraftNote explains an all-zero draft so the change lines are not
// read as a real drop in pay.
func uncalculatedDraftNote(noCalculate bool) string {
	if noCalculate {
		return "the draft's totals are $0.00, so it has probably not been calculated yet; run without --no-calculate to compare calculated totals"
	}
	return "the draft's totals are $0.00 after calculating; check that hours or reimbursements have been entered"
}
