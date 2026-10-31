// Copyright 2026 david and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/gusto/internal/gusto"
)

// prepAction is one planned or applied draft change.
type prepAction struct {
	Employee string  `json:"employee"`
	Action   string  `json:"action"`
	Before   string  `json:"before"`
	After    string  `json:"after"`
	Amount   float64 `json:"amount,omitempty"`
}

func newNovelPayrollsPrepCmd(flags *rootFlags) *cobra.Command {
	var t draftTarget
	var flagFrom string
	var flagRate string
	var flagStdin bool
	var flagNoCalculate bool

	cmd := &cobra.Command{
		Use:   "prep",
		Short: "Load a week's hours and mileage from a CSV onto the open draft, calculate it, and print the review totals",
		Long: "Load a pay period's inputs from a CSV onto the draft payroll, calculate it, and print the\n" +
			"review totals. CSV columns (header row, case-insensitive): employee, hours, miles,\n" +
			"reimbursement, description. Rows for the same employee are summed. Hours replace the\n" +
			"draft's regular hours; miles become one \"Mileage N mi x $R\" line; other reimbursements are\n" +
			"upserted by description. This command never submits.\n\n" +
			"Use this command to load a full week of hours and reimbursements onto the open draft and\n" +
			"calculate it. Do NOT use it to check a draft that is already prepared; use 'payrolls verify'\n" +
			"instead. Do NOT use it for a single mileage line; use 'payrolls mileage' instead.",
		Example:     "  gusto-pp-cli payrolls prep --period-end 2026-11-01 --from week.csv --rate 0.60 --dry-run",
		Annotations: map[string]string{"mcp:read-only": "false", "pp:data-source": "live", "mcp:read-flags": "from", "pp:happy-args": "--from=example-value;--rate=0.60", "pp:verified-by-proof": "payrolls-write-lifecycle.md"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) && strings.TrimSpace(flagFrom) == "" && !flagStdin {
				return writeDryRun(cmd.OutOrStdout(), flags, "payrolls prep")
			}
			if err := timesheetSourceErr(flagFrom, flagStdin); err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			rows, rate, err := loadTimesheet(flagFrom, flagRate, flagStdin)
			if err != nil {
				if dryRunOK(flags) && errors.Is(err, errOpenTimesheet) {
					return writeDryRun(cmd.OutOrStdout(), flags, "payrolls prep")
				}
				return usageErr(err)
			}
			if dryRunOK(flags) {
				// Plan purely from the CSV; the draft is not read under --dry-run.
				plan := make([]prepAction, 0)
				for _, r := range rows {
					if r.Hours != nil {
						plan = append(plan, prepAction{Employee: r.Employee, Action: "set regular hours", After: fmt.Sprintf("%.2f", *r.Hours)})
					}
					for _, l := range r.Expected(rate) {
						plan = append(plan, prepAction{Employee: r.Employee, Action: "upsert reimbursement", After: l.Description, Amount: l.Amount})
					}
				}
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "action": "payrolls prep", "plan": plan, "plan_source": "CSV only; lines that already match the draft are skipped at run time", "would": "apply the plan, calculate, and print review totals; never submit"}, flags)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			api, err := newGustoAPI(flags)
			if err != nil {
				return err
			}
			d, err := resolveDraft(ctx, api, t)
			if err != nil {
				return gustoErr(err)
			}
			// Resolve and safety-check every employee before the first write so
			// a bad row cannot leave the draft half-prepped.
			targets := make([]gusto.DraftEmployee, len(rows))
			seen := map[string]string{}
			for i, r := range rows {
				e, err := d.Employee(r.Employee)
				if err != nil {
					return usageErr(err)
				}
				if prev, dup := seen[e.UUID]; dup {
					return usageErr(fmt.Errorf("CSV names %s twice (%q and %q); use one spelling per employee", e.Name, prev, r.Employee))
				}
				seen[e.UUID] = r.Employee
				if err := e.WritableErr(); err != nil {
					return usageErr(err)
				}
				targets[i] = e
			}
			applied := make([]prepAction, 0)
			partial := func(err error) error {
				_ = printJSONFiltered(cmd.OutOrStdout(), map[string]any{"draft_id": d.ID, "applied": applied, "error": err.Error(), "submitted": false}, flags)
				return gustoErr(err)
			}
			for i, r := range rows {
				e := targets[i]
				if r.Hours != nil && gusto.RoundCents(*r.Hours) != gusto.RoundCents(e.RegularHours) {
					if err := api.SaveRegularHours(ctx, d.ID, e, *r.Hours); err != nil {
						return partial(err)
					}
					applied = append(applied, prepAction{Employee: e.Name, Action: "set regular hours", Before: fmt.Sprintf("%.2f", e.RegularHours), After: fmt.Sprintf("%.2f", *r.Hours)})
				}
				lines := e.Reimbursements
				changed := false
				for _, want := range r.Expected(rate) {
					before := lines
					lines, _ = gusto.UpsertLine(lines, want)
					if !sameLines(before, lines) {
						changed = true
						applied = append(applied, prepAction{Employee: e.Name, Action: "upsert reimbursement", After: want.Description, Amount: want.Amount})
					}
				}
				if changed {
					if err := api.SaveReimbursements(ctx, d.ID, e.UUID, lines); err != nil {
						return partial(err)
					}
				}
			}
			after, err := api.LoadDraft(ctx, d.ID, d.PeriodStart, d.PeriodEnd)
			if err != nil {
				return partial(err)
			}
			mismatches := gusto.Compare(rows, after, rate)
			view := map[string]any{"draft_id": d.ID, "applied": applied, "mismatches": mismatches, "submitted": false}
			var review *gusto.Review
			if !flagNoCalculate {
				r, err := api.CalculatedReview(ctx, d.ID)
				if err != nil {
					return partial(err)
				}
				review = &r
				view["review"] = r
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				w := cmd.OutOrStdout()
				for _, a := range applied {
					fmt.Fprintf(w, "%s: %s %s\n", a.Employee, a.Action, strings.TrimSpace(a.After+" "+moneyIf(a.Amount)))
				}
				if len(applied) == 0 {
					fmt.Fprintln(w, "Draft already matched the CSV; nothing changed.")
				}
				for _, m := range mismatches {
					fmt.Fprintf(w, "MISMATCH %s %s: want %s, got %s\n", m.Employee, m.Field, m.Want, m.Got)
				}
				if review != nil {
					fmt.Fprintln(w)
					printReview(cmd, after, *review)
				} else {
					fmt.Fprintln(w, "Inputs saved; not calculated (--no-calculate). Not submitted.")
				}
			} else if err := printJSONFiltered(cmd.OutOrStdout(), view, flags); err != nil {
				return err
			}
			if len(mismatches) > 0 {
				return apiErr(fmt.Errorf("%d input(s) did not read back as expected; run payrolls verify", len(mismatches)))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&t.periodEnd, "period-end", "", periodEndFlagHelp)
	cmd.Flags().StringVar(&t.draftID, "draft-id", "", draftIDFlagHelp)
	cmd.Flags().StringVar(&flagFrom, "from", "", "CSV file with employee, hours, miles, reimbursement, description columns")
	cmd.Flags().StringVar(&flagRate, "rate", "", "Mileage rate in dollars per mile, e.g. 0.60 (required when the CSV has miles)")
	cmd.Flags().BoolVar(&flagStdin, "stdin", false, timesheetStdinFlagHelp)
	cmd.Flags().BoolVar(&flagNoCalculate, "no-calculate", false, "Save inputs only; skip calculate and review")
	return cmd
}

// sameLines compares what Gusto stores (amount and description); the
// reimbursement type is metadata the save does not change.
func sameLines(a, b []gusto.Reimbursement) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if gusto.RoundCents(a[i].Amount) != gusto.RoundCents(b[i].Amount) || !gusto.SameDescription(a[i].Description, b[i].Description) {
			return false
		}
	}
	return true
}

func moneyIf(v float64) string {
	if v == 0 {
		return ""
	}
	return money(v)
}
