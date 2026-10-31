// Copyright 2026 david and contributors. Licensed under Apache-2.0. See LICENSE.

package gusto

import (
	"context"
	"errors"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"
)

const dateLayout = "2006-01-02"

// DraftID builds Gusto's draft payroll id for a regular payroll:
// regular.<company uuid>.<pay schedule uuid>.<period end date>.
func DraftID(companyUUID, scheduleUUID, periodEnd string) string {
	return "regular." + companyUUID + "." + scheduleUUID + "." + periodEnd
}

// ParseDraftID splits a regular draft payroll id into its parts.
func ParseDraftID(id string) (companyUUID, scheduleUUID, periodEnd string, err error) {
	parts := strings.Split(id, ".")
	if len(parts) != 4 || parts[0] != "regular" {
		return "", "", "", fmt.Errorf("not a regular draft payroll id: %q", id)
	}
	if _, perr := time.Parse(dateLayout, parts[3]); perr != nil {
		return "", "", "", fmt.Errorf("draft payroll id %q has a bad period end date", id)
	}
	return parts[1], parts[2], parts[3], nil
}

// ProcessedPayroll is one row of Gusto's pay history.
type ProcessedPayroll struct {
	ID          string  `json:"id"`
	DraftID     string  `json:"draft_id"`
	Description string  `json:"description"`
	PeriodStart string  `json:"period_start"`
	PeriodEnd   string  `json:"period_end"`
	CheckDate   string  `json:"check_date"`
	DebitDate   string  `json:"debit_date"`
	Debit       float64 `json:"total_debit"`
	Tax         float64 `json:"total_payable_tax"`
	Employees   int     `json:"employees"`
	Status      string  `json:"status"`
	OffCycle    bool    `json:"off_cycle"`
}

// History returns the most recent processed payrolls, newest first.
func (a *API) History(ctx context.Context, first int) ([]ProcessedPayroll, error) {
	if first <= 0 {
		first = 10
	}
	data, err := a.Query(ctx, "PayrollHistoryContainerPayrollSummaries", map[string]any{
		"canViewPayrolls":             true,
		"processedPayrollFilter":      map[string]any{},
		"processedPayrollOrder":       map[string]any{"sortDirection": "DESC", "type": "CHECK_DATE"},
		"first":                       first,
		"payHistoryPhaseThreeEnabled": true,
	})
	if err != nil {
		return nil, err
	}
	return ParseHistory(data), nil
}

// ParseHistory converts a PayrollHistoryContainerPayrollSummaries response.
func ParseHistory(data map[string]any) []ProcessedPayroll {
	edges := List(data, "data", "company", "processedPayrolls", "edges")
	out := make([]ProcessedPayroll, 0, len(edges))
	for _, e := range edges {
		n, _ := e["node"].(map[string]any)
		if n == nil {
			continue
		}
		offCycle, _ := n["offCycle"].(bool)
		out = append(out, ProcessedPayroll{
			ID:          Str(n, "id"),
			DraftID:     Str(n, "payrollRisk", "id"),
			Description: Str(n, "description"),
			PeriodStart: Str(n, "paymentPeriodStartAt"),
			PeriodEnd:   Str(n, "paymentPeriodEndAt"),
			CheckDate:   Str(n, "checkDate"),
			DebitDate:   Str(n, "debitDate"),
			Debit:       Num(n, "totalDebitAmount"),
			Tax:         Num(n, "totalPayableTax"),
			Employees:   int(Num(n, "employeePayrollsCount")),
			Status:      Str(n, "payrollRisk", "paymentStatusInformation", "status"),
			OffCycle:    offCycle,
		})
	}
	return out
}

// Schedule is the regular pay schedule inferred from pay history.
type Schedule struct {
	CompanyUUID   string `json:"company_uuid"`
	ScheduleUUID  string `json:"schedule_uuid"`
	LastPeriodEnd string `json:"last_period_end"`
	PeriodDays    int    `json:"period_days"`
}

// InferSchedule finds the company and schedule from the newest regular
// processed payroll, and the period length from consecutive period ends.
func InferSchedule(history []ProcessedPayroll) (Schedule, error) {
	regular := make([]ProcessedPayroll, 0, len(history))
	for _, p := range history {
		if p.OffCycle || !strings.HasPrefix(p.DraftID, "regular.") {
			continue
		}
		regular = append(regular, p)
	}
	if len(regular) == 0 {
		return Schedule{}, errors.New("no processed regular payroll found to infer the pay schedule; pass --draft-id")
	}
	sort.Slice(regular, func(i, j int) bool { return regular[i].PeriodEnd > regular[j].PeriodEnd })
	company, schedule, end, err := ParseDraftID(regular[0].DraftID)
	if err != nil {
		return Schedule{}, err
	}
	days := 7
	if len(regular) > 1 {
		a, errA := time.Parse(dateLayout, regular[0].PeriodEnd)
		b, errB := time.Parse(dateLayout, regular[1].PeriodEnd)
		if errA == nil && errB == nil {
			if d := int(a.Sub(b).Hours() / 24); d > 0 && d <= 31 {
				days = d
			}
		}
	}
	return Schedule{CompanyUUID: company, ScheduleUUID: schedule, LastPeriodEnd: end, PeriodDays: days}, nil
}

// NextPeriodEnd is the period end after the last processed payroll.
func (s Schedule) NextPeriodEnd() string {
	t, err := time.Parse(dateLayout, s.LastPeriodEnd)
	if err != nil {
		return ""
	}
	return t.AddDate(0, 0, s.PeriodDays).Format(dateLayout)
}

// PeriodStart is the first day of the period that ends on periodEnd.
func (s Schedule) PeriodStart(periodEnd string) string {
	t, err := time.Parse(dateLayout, periodEnd)
	if err != nil {
		return ""
	}
	return t.AddDate(0, 0, -(s.PeriodDays - 1)).Format(dateLayout)
}

// Reimbursement is one itemized reimbursement line on a draft.
type Reimbursement struct {
	Amount      float64 `json:"amount"`
	Description string  `json:"description"`
	// Type is Gusto's reimbursementType (empty for lines this CLI builds).
	// Only itemized one-time lines round-trip safely through the save
	// mutation; see DraftEmployee.WritableErr.
	Type string `json:"type,omitempty"`
}

// ItemizedReimbursementType is the only type this CLI writes back.
const ItemizedReimbursementType = "ITEMIZED_PAYROLL_REIMBURSEMENT"

// DraftEmployee is one employee's inputs on a draft payroll.
type DraftEmployee struct {
	UUID           string          `json:"employee_uuid"`
	Name           string          `json:"name"`
	JobUUID        string          `json:"job_uuid"`
	HourlyRate     float64         `json:"hourly_rate"`
	RegularHours   float64         `json:"regular_hours"`
	OvertimeHours  float64         `json:"overtime_hours"`
	DoubleOTHours  float64         `json:"double_overtime_hours,omitempty"`
	MissedBreak    *float64        `json:"missed_break_hours,omitempty"`
	Bonus          float64         `json:"bonus"`
	Gross          float64         `json:"gross"`
	Jobs           int             `json:"jobs"`
	Reimbursements []Reimbursement `json:"reimbursements"`
}

// WritableErr explains why the CLI must not write this employee's inputs.
// Gusto's reimbursement save replaces the whole list and the hours save
// targets one job, so anything the CLI cannot send back unchanged (recurring
// or unnamed reimbursements, several jobs) would be lost or moved.
func (e DraftEmployee) WritableErr() error {
	if e.Jobs > 1 {
		return fmt.Errorf("%s has %d jobs on this draft; edit them in app.gusto.com (this CLI writes single-job employees only)", e.Name, e.Jobs)
	}
	for _, r := range e.Reimbursements {
		if r.Type != "" && r.Type != ItemizedReimbursementType {
			return fmt.Errorf("%s has a %s reimbursement (%q); edit reimbursements in app.gusto.com so it is not lost", e.Name, r.Type, r.Description)
		}
	}
	return nil
}

// ReimbursementTotal sums the employee's reimbursement lines.
func (e DraftEmployee) ReimbursementTotal() float64 {
	total := 0.0
	for _, r := range e.Reimbursements {
		total += r.Amount
	}
	return RoundCents(total)
}

// Draft is a draft payroll with its inputs.
type Draft struct {
	ID          string          `json:"draft_id"`
	PeriodStart string          `json:"period_start"`
	PeriodEnd   string          `json:"period_end"`
	RunBy       string          `json:"submit_by"`
	RunByCutoff string          `json:"submit_by_cutoff"`
	CheckDate   string          `json:"check_date"`
	State       string          `json:"state"`
	Schedule    string          `json:"pay_schedule"`
	DebitTotal  float64         `json:"total_to_be_debited"`
	Employees   []DraftEmployee `json:"employees"`
}

// Employee finds an employee on the draft by name (case-insensitive,
// full or first name) or uuid.
func (d Draft) Employee(nameOrUUID string) (DraftEmployee, error) {
	q := strings.ToLower(strings.TrimSpace(nameOrUUID))
	if q == "" {
		if len(d.Employees) == 1 {
			return d.Employees[0], nil
		}
		return DraftEmployee{}, fmt.Errorf("draft has %d employees; pass --employee", len(d.Employees))
	}
	// Exact uuid or full name wins; a first name must belong to one person.
	// Partial names are never accepted, because these commands write.
	var firstMatches []DraftEmployee
	var suggestions []string
	for _, e := range d.Employees {
		full := strings.ToLower(e.Name)
		first := strings.ToLower(strings.SplitN(e.Name, " ", 2)[0])
		if e.UUID == nameOrUUID || full == q {
			return e, nil
		}
		if first == q {
			firstMatches = append(firstMatches, e)
		} else if strings.Contains(full, q) {
			suggestions = append(suggestions, e.Name)
		}
	}
	if len(firstMatches) == 1 {
		return firstMatches[0], nil
	}
	if len(firstMatches) > 1 {
		return DraftEmployee{}, fmt.Errorf("employee %q matches %d people on the draft; use the full name", nameOrUUID, len(firstMatches))
	}
	if len(suggestions) > 0 {
		return DraftEmployee{}, fmt.Errorf("employee %q is not an exact name on this draft; did you mean %s?", nameOrUUID, strings.Join(suggestions, ", "))
	}
	return DraftEmployee{}, fmt.Errorf("employee %q is not on this draft payroll", nameOrUUID)
}

// LoadDraft reads a draft payroll's period, deadline and per-employee inputs.
func (a *API) LoadDraft(ctx context.Context, draftID, periodStart, periodEnd string) (Draft, error) {
	step, err := a.Query(ctx, "PayrollInputStep", map[string]any{
		"between":                       map[string]any{"startDate": periodStart, "endDate": periodEnd},
		"draftPayrollId":                draftID,
		"featureFlagInputs":             []any{},
		"hasPaySchedule":                true,
		"shouldQueryTimeTrackingFields": false,
	})
	if err != nil {
		return Draft{}, err
	}
	if Get(step, "data", "company", "draftPayroll") == nil {
		return Draft{}, fmt.Errorf("no draft payroll %s; check --period-end", draftID)
	}
	meta, err := a.Query(ctx, "PayrollInputStepDraftEmployeePayrollsMetadata", map[string]any{"draftPayrollId": draftID})
	if err != nil {
		return Draft{}, err
	}
	ids := make([]any, 0)
	for _, m := range List(meta, "data", "company", "draftPayroll", "draftEmployeePayrollsMetadata") {
		if dismissed, _ := m["isDismissedInPayPeriod"].(bool); dismissed {
			continue
		}
		if id := Str(m, "employeeUuid"); id != "" {
			ids = append(ids, id)
		}
	}
	detail := map[string]any{}
	if len(ids) > 0 {
		detail, err = a.Query(ctx, "PayrollInputStepDraftEmployeePayrollsByIds", map[string]any{
			"includeAnomalyChecks":          false,
			"draftPayrollId":                draftID,
			"ids":                           ids,
			"includeNetPay":                 false,
			"includePreviewPay":             false,
			"includeTaxOverrideInformation": false,
			"includeWorkLocation":           false,
		})
		if err != nil {
			return Draft{}, err
		}
	}
	return ParseDraft(step, detail), nil
}

// ParseDraft combines PayrollInputStep and ...DraftEmployeePayrollsByIds.
func ParseDraft(step, detail map[string]any) Draft {
	dp := Get(step, "data", "company", "draftPayroll")
	d := Draft{
		ID:          Str(dp, "id"),
		PeriodStart: Str(dp, "payPeriod", "startDate"),
		PeriodEnd:   Str(dp, "payPeriod", "endDate"),
		RunBy:       Str(dp, "payPeriod", "runPayrollBy"),
		RunByCutoff: Str(dp, "payPeriod", "runPayrollByBankCutoffTime"),
		CheckDate:   Str(dp, "payPeriod", "checkDate"),
		State:       Str(dp, "submissionStatus", "state"),
		Schedule:    Str(dp, "paySchedule", "name"),
		DebitTotal:  Num(dp, "totalAmountToBeDebited"),
		Employees:   make([]DraftEmployee, 0),
	}
	for _, n := range List(detail, "data", "company", "draftPayroll", "draftEmployeePayrollsByIds") {
		e := DraftEmployee{
			UUID:           Str(n, "employee", "uuid"),
			Name:           Str(n, "employee", "preferredFullName"),
			JobUUID:        Str(n, "primaryJobApplicableCompensation", "employeeJob", "uuid"),
			HourlyRate:     Num(n, "primaryJobApplicableCompensation", "paymentAmount"),
			Gross:          Num(n, "draftGrossPayBreakdownSummary", "amount"),
			Reimbursements: make([]Reimbursement, 0),
		}
		if e.UUID == "" {
			e.UUID = Str(n, "employee", "id")
		}
		jobs := map[string]bool{}
		for _, wh := range List(n, "allWorkedHours") {
			e.RegularHours += Num(wh, "regularHours")
			e.OvertimeHours += Num(wh, "overtimeHours")
			e.DoubleOTHours += Num(wh, "doubleOvertimeHours")
			if Get(wh, "missedBreakHours") != nil {
				mb := Num(wh, "missedBreakHours")
				if e.MissedBreak != nil {
					mb += *e.MissedBreak
				}
				e.MissedBreak = &mb
			}
			jobs[Str(wh, "employeeJobCompensation", "uuid")] = true
		}
		e.Jobs = len(jobs)
		if multi, _ := n["hasMultipleJobs"].(bool); multi && e.Jobs < 2 {
			e.Jobs = 2
		}
		for _, ap := range List(n, "allAdditionalPays") {
			if strings.EqualFold(Str(ap, "payType", "name"), "Bonus") {
				e.Bonus += Num(ap, "amount")
			}
		}
		for _, r := range List(n, "allReimbursements") {
			amt := Num(r, "amount")
			desc := Str(r, "description")
			typ := Str(r, "reimbursementType")
			if Str(r, "__typename") == "DraftEmployeePayrollUnnamedItemizedReimbursement" {
				if amt == 0 {
					continue
				}
				typ = "UNNAMED_ITEMIZED_REIMBURSEMENT"
			}
			if desc == "" && amt == 0 {
				continue
			}
			e.Reimbursements = append(e.Reimbursements, Reimbursement{Amount: amt, Description: desc, Type: typ})
		}
		d.Employees = append(d.Employees, e)
	}
	return d
}

// SaveRegularHours sets the employee's regular hours on the draft.
func (a *API) SaveRegularHours(ctx context.Context, draftID string, e DraftEmployee, hours float64) error {
	if e.JobUUID == "" {
		return fmt.Errorf("employee %s has no primary hourly job on this draft", e.Name)
	}
	_, err := a.Mutate(ctx, "AutoSaveWorkedHours", map[string]any{
		"employeeUuid": e.UUID,
		"input": map[string]any{
			"payrollActivitySessionId": a.sessionID,
			"uniqueIdentifier":         draftID,
			"workedHours": []any{map[string]any{
				"employeeUuid": e.UUID,
				"workedHours": []any{map[string]any{
					// Send the other hour kinds back unchanged; the save
					// replaces the whole row.
					"doubleOvertimeHours": e.DoubleOTHours,
					"employeeJobUuid":     e.JobUUID,
					"missedBreakHours":    e.MissedBreak,
					"overtimeHours":       e.OvertimeHours,
					"regularHours":        hours,
				}},
			}},
		},
	})
	if err != nil {
		return err
	}
	_, err = a.Mutate(ctx, "MarkCompanyTouchedHours", map[string]any{})
	return err
}

// SaveReimbursements replaces the employee's itemized reimbursement list on
// the draft. Gusto's mutation takes the complete list, so callers pass every
// line that should remain.
func (a *API) SaveReimbursements(ctx context.Context, draftID, employeeUUID string, lines []Reimbursement) error {
	items := make([]any, 0, len(lines)+1)
	for _, r := range lines {
		items = append(items, map[string]any{
			"amount":                  RoundCents(r.Amount),
			"description":             r.Description,
			"employeeReimbursementId": NewUUID(),
		})
	}
	items = append(items, map[string]any{"amount": 0})
	_, err := a.Mutate(ctx, "AutoSaveReimbursements", map[string]any{
		"employeeUuid": employeeUUID,
		"input": map[string]any{
			"payrollActivitySessionId": a.sessionID,
			"reimbursements":           []any{map[string]any{"employeeUuid": employeeUUID, "itemizedReimbursements": items}},
			"uniqueIdentifier":         draftID,
		},
	})
	return err
}

// Calculate asks Gusto to calculate the draft and waits until it finishes.
// It never submits.
func (a *API) Calculate(ctx context.Context, draftID string) error {
	if _, err := a.Mutate(ctx, "CalculateDraftPayroll", map[string]any{"input": map[string]any{"uniqueIdentifier": draftID}}); err != nil {
		return err
	}
	for i := 0; i < 30; i++ {
		st, err := a.Query(ctx, "PollPayrollSubmissionStatus", map[string]any{"draftPayrollId": draftID})
		if err != nil {
			return err
		}
		if Num(st, "data", "company", "draftPayroll", "submissionStatus", "calculationProgressPercentage") >= 100 {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
	return errors.New("gusto did not finish calculating the payroll in 30s")
}

// CalculatedReview calculates the draft and returns review totals once they
// are consistent. Gusto's poll can still report 100% from the previous
// calculation right after a new one starts, so the review is re-read until
// the total debit equals the sum of per-employee employer cost.
func (a *API) CalculatedReview(ctx context.Context, draftID string) (Review, error) {
	if err := a.Calculate(ctx, draftID); err != nil {
		return Review{}, err
	}
	var r Review
	var err error
	for i := 0; i < 8; i++ {
		r, err = a.LoadReview(ctx, draftID)
		if err != nil {
			return Review{}, err
		}
		if r.Consistent() {
			return r, nil
		}
		select {
		case <-ctx.Done():
			return Review{}, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return r, fmt.Errorf("gusto review totals did not settle: total debit %.2f does not match employer cost %.2f", r.Debit, r.EmployerCostTotal())
}

// EmployerCostTotal sums per-employee employer cost.
func (r Review) EmployerCostTotal() float64 {
	total := 0.0
	for _, e := range r.Employees {
		total += e.EmployerCost
	}
	return RoundCents(total)
}

// Consistent reports whether the payroll-level debit agrees with the
// per-employee amounts (a stale calculation shows a different debit).
func (r Review) Consistent() bool {
	return RoundCents(r.Debit) == r.EmployerCostTotal()
}

// ReviewEmployee is one employee's calculated amounts.
type ReviewEmployee struct {
	UUID           string  `json:"employee_uuid"`
	Name           string  `json:"name"`
	Gross          float64 `json:"gross"`
	Reimbursements float64 `json:"reimbursements"`
	EmployeeTaxes  float64 `json:"employee_taxes"`
	EmployerTaxes  float64 `json:"employer_taxes"`
	Deductions     float64 `json:"deductions"`
	Net            float64 `json:"net_pay"`
	EmployerCost   float64 `json:"employer_cost"`
}

// Review is the calculated summary Gusto shows before Submit.
type Review struct {
	DraftID        string           `json:"draft_id"`
	Gross          float64          `json:"gross"`
	Reimbursements float64          `json:"reimbursements"`
	EmployeeTaxes  float64          `json:"employee_taxes"`
	EmployerTaxes  float64          `json:"employer_taxes"`
	Net            float64          `json:"net_pay"`
	Debit          float64          `json:"total_debit"`
	CheckDate      string           `json:"check_date"`
	FundingAccount string           `json:"funding_account"`
	Submitted      bool             `json:"submitted"`
	Employees      []ReviewEmployee `json:"employees"`
}

// LoadReview reads the calculated review totals for a draft.
// Review returns the draft's totals: recalculated first when calculate is
// true, otherwise the totals Gusto already stored.
func (a *API) Review(ctx context.Context, draftID string, calculate bool) (Review, error) {
	if calculate {
		return a.CalculatedReview(ctx, draftID)
	}
	return a.LoadReview(ctx, draftID)
}

func (a *API) LoadReview(ctx context.Context, draftID string) (Review, error) {
	data, err := a.Query(ctx, "PayrollReviewStep", map[string]any{"draftPayrollId": draftID, "hasAccessToPayrollBridge": true})
	if err != nil {
		return Review{}, err
	}
	return ParseReview(data), nil
}

// ParseReview converts a PayrollReviewStep response.
func ParseReview(data map[string]any) Review {
	dp := Get(data, "data", "company", "draftPayroll")
	r := Review{
		DraftID:   Str(dp, "id"),
		Debit:     Num(dp, "totalAmountToBeDebited"),
		CheckDate: Str(dp, "payPeriod", "checkDate"),
		Employees: make([]ReviewEmployee, 0),
	}
	for _, f := range List(data, "data", "company", "achFundingMethods") {
		r.FundingAccount = strings.TrimSpace(Str(f, "bankName") + " " + Str(f, "hiddenAccountNumber"))
		break
	}
	for _, n := range List(dp, "includedDraftEmployeePayrolls", "nodes") {
		amt := Get(n, "draftEmployeePayrollCalculatedAmounts")
		first := Str(n, "employee", "preferredFirstName")
		if first == "" {
			first = Str(n, "employee", "legalFirstName")
		}
		e := ReviewEmployee{
			UUID:           Str(n, "employee", "id"),
			Name:           strings.TrimSpace(first + " " + Str(n, "employee", "lastName")),
			Gross:          Num(amt, "grossPay"),
			Reimbursements: Num(amt, "totalReimbursementAmount"),
			EmployeeTaxes:  Num(amt, "totalEmployeeTax"),
			EmployerTaxes:  Num(amt, "employerTax"),
			Deductions:     Num(amt, "totalDeductionAmount"),
			Net:            Num(amt, "checkAmount"),
			EmployerCost:   Num(amt, "employerCost"),
		}
		r.Gross += e.Gross
		r.Reimbursements += e.Reimbursements
		r.EmployeeTaxes += e.EmployeeTaxes
		r.EmployerTaxes += e.EmployerTaxes
		r.Net += e.Net
		r.Employees = append(r.Employees, e)
	}
	r.Gross, r.Reimbursements, r.Net = RoundCents(r.Gross), RoundCents(r.Reimbursements), RoundCents(r.Net)
	r.EmployeeTaxes, r.EmployerTaxes = RoundCents(r.EmployeeTaxes), RoundCents(r.EmployerTaxes)
	return r
}

// --- pure helpers ---------------------------------------------------------

// RoundCents rounds to two decimals.
func RoundCents(v float64) float64 { return math.Round(v*100) / 100 }

// Mileage returns the reimbursement amount and paystub description for a
// mileage figure at a per-mile rate. Miles are rounded once so the
// description and the amount agree.
func Mileage(miles, rate float64) (float64, string) {
	miles = RoundCents(miles)
	desc := fmt.Sprintf("Mileage %s mi x $%s", trimFloat(miles), rateString(rate))
	return RoundCents(miles * rate), desc
}

// mileageRe matches the exact description Mileage writes, so a user line such
// as "Mileage to airport" is never mistaken for it.
var mileageRe = regexp.MustCompile(`^Mileage [0-9.]+ mi x \$[0-9.]+$`)

// IsMileage reports whether a line is a mileage line written by Mileage.
func (r Reimbursement) IsMileage() bool {
	return mileageRe.MatchString(strings.TrimSpace(r.Description))
}

// rateString prints a per-mile rate with 2 to 4 decimals, so a rate such as
// 0.725 is not shown as 0.72 next to an amount computed from 0.725.
func rateString(rate float64) string {
	s := strings.TrimRight(fmt.Sprintf("%.4f", rate), "0")
	if i := strings.IndexByte(s, '.'); i >= 0 && len(s)-i-1 < 2 {
		s += strings.Repeat("0", 2-(len(s)-i-1))
	}
	return s
}

func trimFloat(v float64) string {
	s := fmt.Sprintf("%.2f", v)
	s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	return s
}

// SameDescription compares reimbursement descriptions the way Gusto's
// paystub reads them: case-insensitive, ignoring surrounding space.
func SameDescription(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}

// UpsertLine adds a line, replacing the line it supersedes so re-running a
// command is idempotent. A mileage line replaces any existing mileage line
// (one per employee); any other line replaces a line with the same
// description.
func UpsertLine(lines []Reimbursement, line Reimbursement) ([]Reimbursement, bool) {
	if line.IsMileage() {
		return UpsertMileage(lines, line)
	}
	return UpsertReimbursement(lines, line)
}

// UpsertMileage keeps one mileage line per employee: any existing mileage
// line is replaced, so correcting the miles never leaves two lines.
func UpsertMileage(lines []Reimbursement, line Reimbursement) ([]Reimbursement, bool) {
	return upsert(lines, line, Reimbursement.IsMileage)
}

// UpsertReimbursement replaces a line with the same description.
func UpsertReimbursement(lines []Reimbursement, line Reimbursement) ([]Reimbursement, bool) {
	return upsert(lines, line, func(l Reimbursement) bool { return SameDescription(l.Description, line.Description) })
}

// upsert puts line in place of the first line that matches, drops later
// matches, and appends line when nothing matched.
func upsert(lines []Reimbursement, line Reimbursement, match func(Reimbursement) bool) ([]Reimbursement, bool) {
	out := make([]Reimbursement, 0, len(lines)+1)
	replaced := false
	for _, l := range lines {
		if match(l) {
			if !replaced {
				out = append(out, line)
				replaced = true
			}
			continue
		}
		out = append(out, l)
	}
	if !replaced {
		out = append(out, line)
	}
	return out, replaced
}

// RemoveReimbursement drops lines whose description matches.
func RemoveReimbursement(lines []Reimbursement, description string) ([]Reimbursement, int) {
	out := make([]Reimbursement, 0, len(lines))
	removed := 0
	for _, l := range lines {
		if SameDescription(l.Description, description) {
			removed++
			continue
		}
		out = append(out, l)
	}
	return out, removed
}
