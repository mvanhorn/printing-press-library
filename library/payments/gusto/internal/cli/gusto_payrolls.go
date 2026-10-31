// Copyright 2026 david and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/gusto/internal/gusto"
)

// Hand-built Gusto payroll commands. Gusto's web API is GraphQL, so the
// generator only emits scaffolding; these commands are the real surface.
func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		payrolls, _, err := root.Find([]string{"payrolls"})
		if err != nil || payrolls == nil || payrolls.Name() != "payrolls" {
			return
		}
		addNovelCommandIfAbsent(payrolls, newPayrollsHistoryCmd(flags))
		addNovelCommandIfAbsent(payrolls, newPayrollsDueCmd(flags))
		addNovelCommandIfAbsent(payrolls, newPayrollsShowCmd(flags))
		addNovelCommandIfAbsent(payrolls, newPayrollsHoursCmd(flags))
		addNovelCommandIfAbsent(payrolls, newPayrollsReimburseCmd(flags))
		addNovelCommandIfAbsent(payrolls, newPayrollsReviewCmd(flags))
	})
}

// draftTarget names the draft payroll a command works on.
type draftTarget struct {
	periodEnd string
	draftID   string
}

// Shared help text for the draft-target flags each command declares.
const (
	periodEndFlagHelp = "Last day of the pay period, YYYY-MM-DD (default: the period after the last processed payroll)"
	draftIDFlagHelp   = "Gusto draft payroll id (regular.<company>.<schedule>.<period-end>); overrides --period-end"
)

// newGustoAPI builds the GraphQL caller on the generated client.
func newGustoAPI(flags *rootFlags) (*gusto.API, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	return gusto.New(c), nil
}

// gustoErr maps session failures to the auth exit code.
func gustoErr(err error) error {
	if err == nil {
		return nil
	}
	if err == gusto.ErrNotSignedIn || strings.Contains(err.Error(), "not signed in") {
		return authErr(err)
	}
	return classifyAPIErrorOnly(err)
}

// resolveDraft loads the target draft payroll.
func resolveDraft(ctx context.Context, api *gusto.API, t draftTarget) (gusto.Draft, error) {
	return resolveDraftWith(ctx, api, t, nil)
}

// resolveDraftWith is resolveDraft for a caller that already loaded payroll
// history; a nil history is fetched when the draft id must be inferred.
func resolveDraftWith(ctx context.Context, api *gusto.API, t draftTarget, history []gusto.ProcessedPayroll) (gusto.Draft, error) {
	draftID := strings.TrimSpace(t.draftID)
	periodEnd := strings.TrimSpace(t.periodEnd)
	var sched gusto.Schedule
	if draftID == "" {
		if history == nil {
			h, err := api.History(ctx, 5)
			if err != nil {
				return gusto.Draft{}, err
			}
			history = h
		}
		var err error
		sched, err = gusto.InferSchedule(history)
		if err != nil {
			return gusto.Draft{}, usageErr(err)
		}
		if periodEnd == "" {
			periodEnd = sched.NextPeriodEnd()
		}
		draftID = gusto.DraftID(sched.CompanyUUID, sched.ScheduleUUID, periodEnd)
	} else {
		_, _, end, err := gusto.ParseDraftID(draftID)
		if err != nil {
			return gusto.Draft{}, usageErr(err)
		}
		periodEnd = end
		sched = gusto.Schedule{PeriodDays: 7}
	}
	if sched.PeriodDays == 0 {
		sched.PeriodDays = 7
	}
	return api.LoadDraft(ctx, draftID, sched.PeriodStart(periodEnd), periodEnd)
}

var money = gusto.Money

func newPayrollsHistoryCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:         "history",
		Short:       "List processed payrolls with period, check date and total debit",
		Example:     "  gusto-pp-cli payrolls history --limit 5",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "payrolls history")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			api, err := newGustoAPI(flags)
			if err != nil {
				return err
			}
			rows, err := api.History(ctx, limit)
			if err != nil {
				return gustoErr(err)
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No processed payrolls found.")
				return nil
			}
			items := make([]map[string]any, 0, len(rows))
			for _, r := range rows {
				items = append(items, map[string]any{"period": r.PeriodStart + " to " + r.PeriodEnd, "check_date": r.CheckDate, "debit": money(r.Debit), "employees": r.Employees, "status": r.Status})
			}
			return printAutoTable(cmd.OutOrStdout(), items)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum payrolls to return")
	return cmd
}

func newPayrollsDueCmd(flags *rootFlags) *cobra.Command {
	var t draftTarget
	cmd := &cobra.Command{
		Use:         "due",
		Short:       "Show the next draft payroll: pay period, submit-by deadline and check date",
		Example:     "  gusto-pp-cli payrolls due",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "payrolls due")
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
			view := map[string]any{
				"draft_id": d.ID, "period_start": d.PeriodStart, "period_end": d.PeriodEnd,
				"submit_by": d.RunBy, "submit_by_cutoff": d.RunByCutoff, "check_date": d.CheckDate,
				"state": d.State, "pay_schedule": d.Schedule, "employees": len(d.Employees),
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s payroll %s to %s\nSubmit by: %s (%s)\nPayday:    %s\nState:     %s\n", d.Schedule, d.PeriodStart, d.PeriodEnd, d.RunBy, d.RunByCutoff, d.CheckDate, d.State)
			return nil
		},
	}
	cmd.Flags().StringVar(&t.periodEnd, "period-end", "", periodEndFlagHelp)
	cmd.Flags().StringVar(&t.draftID, "draft-id", "", draftIDFlagHelp)
	return cmd
}

func newPayrollsShowCmd(flags *rootFlags) *cobra.Command {
	var t draftTarget
	cmd := &cobra.Command{
		Use:         "show",
		Short:       "Show a draft payroll's inputs per employee: hours, bonus and reimbursement lines",
		Example:     "  gusto-pp-cli payrolls show --period-end 2026-11-01",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "payrolls show")
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
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), d, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Draft %s to %s (submit by %s)\n", d.PeriodStart, d.PeriodEnd, d.RunBy)
			items := make([]map[string]any, 0, len(d.Employees))
			for _, e := range d.Employees {
				lines := make([]string, 0, len(e.Reimbursements))
				for _, r := range e.Reimbursements {
					lines = append(lines, fmt.Sprintf("%s %s", money(r.Amount), r.Description))
				}
				items = append(items, map[string]any{"employee": e.Name, "regular_hours": e.RegularHours, "rate": money(e.HourlyRate), "bonus": money(e.Bonus), "reimbursements": strings.Join(lines, "; ")})
			}
			return printAutoTable(cmd.OutOrStdout(), items)
		},
	}
	cmd.Flags().StringVar(&t.periodEnd, "period-end", "", periodEndFlagHelp)
	cmd.Flags().StringVar(&t.draftID, "draft-id", "", draftIDFlagHelp)
	return cmd
}

func newPayrollsHoursCmd(flags *rootFlags) *cobra.Command {
	group := &cobra.Command{Use: "hours", Short: "Edit hours on a draft payroll"}
	var t draftTarget
	var employee, hours string
	set := &cobra.Command{
		Use:         "set",
		Short:       "Set an employee's regular hours on the draft payroll",
		Example:     "  gusto-pp-cli payrolls hours set --period-end 2026-11-01 --employee 'Ava Example' --hours 20",
		Annotations: map[string]string{"pp:data-source": "live", "pp:happy-args": "--employee=example-value;--hours=0"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "payrolls hours set")
			}
			h, err := strconv.ParseFloat(strings.TrimSpace(hours), 64)
			if err != nil || h < 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--hours must be a number of hours, e.g. --hours 20"))
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
			e, err := d.Employee(employee)
			if err != nil {
				return usageErr(err)
			}
			if err := e.WritableErr(); err != nil {
				return usageErr(err)
			}
			if err := api.SaveRegularHours(ctx, d.ID, e, h); err != nil {
				return gustoErr(err)
			}
			after, err := api.LoadDraft(ctx, d.ID, d.PeriodStart, d.PeriodEnd)
			if err != nil {
				return gustoErr(err)
			}
			saved, lookupErr := after.Employee(e.UUID)
			ok := lookupErr == nil && gusto.RoundCents(saved.RegularHours) == gusto.RoundCents(h)
			view := map[string]any{"draft_id": d.ID, "employee": e.Name, "regular_hours_before": e.RegularHours, "regular_hours": saved.RegularHours, "saved": ok}
			if !ok {
				if err := printJSONFiltered(cmd.OutOrStdout(), view, flags); err != nil {
					return err
				}
				return apiErr(fmt.Errorf("gusto did not show %.2f regular hours for %s after saving", h, e.Name))
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s: regular hours %.2f -> %.2f (read back from Gusto)\n", e.Name, e.RegularHours, saved.RegularHours)
			return nil
		},
	}
	set.Flags().StringVar(&t.periodEnd, "period-end", "", periodEndFlagHelp)
	set.Flags().StringVar(&t.draftID, "draft-id", "", draftIDFlagHelp)
	set.Flags().StringVar(&employee, "employee", "", "Employee name (full or first) or uuid; optional when the draft has one employee")
	set.Flags().StringVar(&hours, "hours", "", "Regular hours for the pay period")
	group.AddCommand(set)
	return group
}

func newPayrollsReimburseCmd(flags *rootFlags) *cobra.Command {
	group := &cobra.Command{Use: "reimburse", Short: "Add, list and remove one-time reimbursements on a draft payroll"}
	group.AddCommand(newReimburseAddCmd(flags), newReimburseListCmd(flags), newReimburseRemoveCmd(flags))
	return group
}

func newReimburseAddCmd(flags *rootFlags) *cobra.Command {
	var t draftTarget
	var employee, amount, description string
	cmd := &cobra.Command{
		Use:         "add",
		Short:       "Add a one-time reimbursement line; a line with the same description is replaced, not duplicated",
		Example:     "  gusto-pp-cli payrolls reimburse add --period-end 2026-11-01 --employee 'Ava Example' --amount 12.50 --description 'Parking'",
		Annotations: map[string]string{"pp:data-source": "live", "pp:happy-args": "--employee=example-value;--amount=0;--description=example-value"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "payrolls reimburse add")
			}
			amt, err := strconv.ParseFloat(strings.TrimPrefix(strings.TrimSpace(amount), "$"), 64)
			if err != nil || amt <= 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--amount must be a positive dollar amount, e.g. --amount 12.50"))
			}
			if strings.TrimSpace(description) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--description is required; it prints on the paystub"))
			}
			return addReimbursementLine(cmd, flags, t, employee, gusto.Reimbursement{Amount: gusto.RoundCents(amt), Description: strings.TrimSpace(description)})
		},
	}
	cmd.Flags().StringVar(&t.periodEnd, "period-end", "", periodEndFlagHelp)
	cmd.Flags().StringVar(&t.draftID, "draft-id", "", draftIDFlagHelp)
	cmd.Flags().StringVar(&employee, "employee", "", "Employee name (full or first) or uuid; optional when the draft has one employee")
	cmd.Flags().StringVar(&amount, "amount", "", "Reimbursement amount in dollars")
	cmd.Flags().StringVar(&description, "description", "", "Paystub description, e.g. 'Parking'")
	return cmd
}

// addReimbursementLine upserts one line and reads the draft back.
func addReimbursementLine(cmd *cobra.Command, flags *rootFlags, t draftTarget, employee string, line gusto.Reimbursement) error {
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
	e, err := d.Employee(employee)
	if err != nil {
		return usageErr(err)
	}
	if err := e.WritableErr(); err != nil {
		return usageErr(err)
	}
	lines, replaced := gusto.UpsertLine(e.Reimbursements, line)
	if err := api.SaveReimbursements(ctx, d.ID, e.UUID, lines); err != nil {
		return gustoErr(err)
	}
	after, err := api.LoadDraft(ctx, d.ID, d.PeriodStart, d.PeriodEnd)
	if err != nil {
		return gustoErr(err)
	}
	saved, _ := after.Employee(e.UUID)
	found := false
	for _, r := range saved.Reimbursements {
		if gusto.SameDescription(r.Description, line.Description) && gusto.RoundCents(r.Amount) == line.Amount {
			found = true
		}
	}
	view := map[string]any{
		"draft_id": d.ID, "employee": e.Name, "line": line, "replaced_existing": replaced,
		"saved": found, "reimbursements": saved.Reimbursements, "reimbursement_total": saved.ReimbursementTotal(),
	}
	if !found {
		if err := printJSONFiltered(cmd.OutOrStdout(), view, flags); err != nil {
			return err
		}
		return apiErr(fmt.Errorf("gusto did not show %q at %s after saving", line.Description, money(line.Amount)))
	}
	if !wantsHumanTable(cmd.OutOrStdout(), flags) {
		return printJSONFiltered(cmd.OutOrStdout(), view, flags)
	}
	verb := "Added"
	if replaced {
		verb = "Replaced"
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s %s %q for %s. Reimbursements now %s (read back from Gusto).\n", verb, money(line.Amount), line.Description, e.Name, money(saved.ReimbursementTotal()))
	return nil
}

func newReimburseListCmd(flags *rootFlags) *cobra.Command {
	var t draftTarget
	var employee string
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List reimbursement lines on a draft payroll",
		Example:     "  gusto-pp-cli payrolls reimburse list --period-end 2026-11-01",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "payrolls reimburse list")
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
			type row struct {
				Employee    string  `json:"employee"`
				Amount      float64 `json:"amount"`
				Description string  `json:"description"`
			}
			only := ""
			if employee != "" {
				m, err := d.Employee(employee)
				if err != nil {
					return usageErr(err)
				}
				only = m.UUID
			}
			rows := make([]row, 0)
			for _, e := range d.Employees {
				if only != "" && e.UUID != only {
					continue
				}
				for _, r := range e.Reimbursements {
					rows = append(rows, row{Employee: e.Name, Amount: r.Amount, Description: r.Description})
				}
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No reimbursements on this draft.")
				return nil
			}
			items := make([]map[string]any, 0, len(rows))
			for _, r := range rows {
				items = append(items, map[string]any{"employee": r.Employee, "amount": money(r.Amount), "description": r.Description})
			}
			return printAutoTable(cmd.OutOrStdout(), items)
		},
	}
	cmd.Flags().StringVar(&t.periodEnd, "period-end", "", periodEndFlagHelp)
	cmd.Flags().StringVar(&t.draftID, "draft-id", "", draftIDFlagHelp)
	cmd.Flags().StringVar(&employee, "employee", "", "Only this employee (name or uuid)")
	return cmd
}

func newReimburseRemoveCmd(flags *rootFlags) *cobra.Command {
	var t draftTarget
	var employee, description string
	cmd := &cobra.Command{
		Use:         "remove",
		Short:       "Remove reimbursement lines with a matching description from a draft payroll",
		Example:     "  gusto-pp-cli payrolls reimburse remove --period-end 2026-11-01 --employee 'Ava Example' --description 'Parking'",
		Annotations: map[string]string{"pp:data-source": "live", "pp:happy-args": "--employee=example-value;--description=example-value"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "payrolls reimburse remove")
			}
			if strings.TrimSpace(description) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--description is required"))
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
			e, err := d.Employee(employee)
			if err != nil {
				return usageErr(err)
			}
			if err := e.WritableErr(); err != nil {
				return usageErr(err)
			}
			lines, removed := gusto.RemoveReimbursement(e.Reimbursements, description)
			if removed == 0 {
				return notFoundErr(fmt.Errorf("no reimbursement %q for %s on this draft", description, e.Name))
			}
			if err := api.SaveReimbursements(ctx, d.ID, e.UUID, lines); err != nil {
				return gustoErr(err)
			}
			after, err := api.LoadDraft(ctx, d.ID, d.PeriodStart, d.PeriodEnd)
			if err != nil {
				return gustoErr(err)
			}
			saved, _ := after.Employee(e.UUID)
			_, still := gusto.RemoveReimbursement(saved.Reimbursements, description)
			view := map[string]any{"draft_id": d.ID, "employee": e.Name, "removed": removed, "reimbursements": saved.Reimbursements, "reimbursement_total": saved.ReimbursementTotal(), "saved": still == 0}
			if still > 0 {
				if err := printJSONFiltered(cmd.OutOrStdout(), view, flags); err != nil {
					return err
				}
				return apiErr(fmt.Errorf("gusto still shows %d %q line(s) for %s after saving", still, description, e.Name))
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed %d line(s) %q for %s. Reimbursements now %s.\n", removed, description, e.Name, money(saved.ReimbursementTotal()))
			return nil
		},
	}
	cmd.Flags().StringVar(&t.periodEnd, "period-end", "", periodEndFlagHelp)
	cmd.Flags().StringVar(&t.draftID, "draft-id", "", draftIDFlagHelp)
	cmd.Flags().StringVar(&employee, "employee", "", "Employee name (full or first) or uuid; optional when the draft has one employee")
	cmd.Flags().StringVar(&description, "description", "", "Description of the line(s) to remove")
	return cmd
}

func newPayrollsReviewCmd(flags *rootFlags) *cobra.Command {
	var t draftTarget
	var noCalculate bool
	cmd := &cobra.Command{
		Use:   "review",
		Short: "Calculate the draft payroll and show gross, reimbursements, taxes, net and total debit; never submits",
		Long: "Calculate the draft payroll and show the totals Gusto's Review page shows before Submit.\n" +
			"This command never submits. A person submits the payroll in app.gusto.com.",
		Example:     "  gusto-pp-cli payrolls review --period-end 2026-11-01",
		Annotations: map[string]string{"pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "payrolls review")
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
			r, err := api.Review(ctx, d.ID, !noCalculate)
			if err != nil {
				return gustoErr(err)
			}
			view := map[string]any{"review": r, "submit_by": d.RunBy, "submit_by_cutoff": d.RunByCutoff, "period_start": d.PeriodStart, "period_end": d.PeriodEnd, "submitted": false, "next_step": "A person submits this payroll in app.gusto.com."}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			printReview(cmd, d, r)
			return nil
		},
	}
	cmd.Flags().StringVar(&t.periodEnd, "period-end", "", periodEndFlagHelp)
	cmd.Flags().StringVar(&t.draftID, "draft-id", "", draftIDFlagHelp)
	cmd.Flags().BoolVar(&noCalculate, "no-calculate", false, "Read the last calculated totals without recalculating")
	return cmd
}

func printReview(cmd *cobra.Command, d gusto.Draft, r gusto.Review) {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "Payroll %s to %s, payday %s, submit by %s\n\n", d.PeriodStart, d.PeriodEnd, r.CheckDate, d.RunBy)
	items := make([]map[string]any, 0, len(r.Employees))
	for _, e := range r.Employees {
		items = append(items, map[string]any{"employee": e.Name, "gross": money(e.Gross), "reimbursements": money(e.Reimbursements), "employee_taxes": money(e.EmployeeTaxes), "net_pay": money(e.Net)})
	}
	_ = printAutoTable(w, items)
	fmt.Fprintf(w, "\nGross %s | Reimbursements %s | Employee taxes %s | Company taxes %s\nTotal debit %s from %s\nNot submitted. Submit in app.gusto.com.\n",
		money(r.Gross), money(r.Reimbursements), money(r.EmployeeTaxes), money(r.EmployerTaxes), money(r.Debit), r.FundingAccount)
}
