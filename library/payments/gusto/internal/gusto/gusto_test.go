// Copyright 2026 david and contributors. Licensed under Apache-2.0. See LICENSE.

package gusto

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func fixture(t *testing.T, name string) map[string]any {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name + ".json")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestParseSessionHeaders(t *testing.T) {
	cases := []struct {
		name, html, csrf, role string
		wantErr                bool
	}{
		{"signed in", `<meta name="csrf-token" content="tok+/=="><script>{"prefix":"x","role_id":1234567890123456}</script>`, "tok+/==", "1234567890123456", false},
		{"login page", `<title>Gusto Login</title>`, "", "", true},
		{"csrf only", `<meta name="csrf-token" content="abc">`, "", "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h, err := ParseSessionHeaders([]byte(c.html))
			if (err != nil) != c.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, c.wantErr)
			}
			if h.CSRFToken != c.csrf || h.RoleID != c.role {
				t.Fatalf("got %+v", h)
			}
		})
	}
}

func TestDraftIDRoundTrip(t *testing.T) {
	cases := []struct{ company, schedule, end string }{
		{"f290a54b-9dd2-1285-9772-7b50837a84af", "295d3235-6944-d6eb-7353-db20a504aecb", "2026-11-01"},
		{"a", "b", "2026-01-31"},
	}
	for _, c := range cases {
		id := DraftID(c.company, c.schedule, c.end)
		co, sc, en, err := ParseDraftID(id)
		if err != nil || co != c.company || sc != c.schedule || en != c.end {
			t.Fatalf("round trip %q -> %q %q %q %v", id, co, sc, en, err)
		}
	}
	for _, bad := range []string{"offcycle.a.b.2026-11-01", "regular.a.b", "regular.a.b.notadate"} {
		if _, _, _, err := ParseDraftID(bad); err == nil {
			t.Fatalf("ParseDraftID(%q) should fail", bad)
		}
	}
}

func TestParseHistoryAndInferSchedule(t *testing.T) {
	h := ParseHistory(fixture(t, "PayrollHistoryContainerPayrollSummaries"))
	if len(h) == 0 {
		t.Fatal("no history rows parsed")
	}
	if h[0].PeriodEnd != "2026-10-25" || h[0].Debit != 352.91 {
		t.Fatalf("first row = %+v", h[0])
	}
	s, err := InferSchedule(h)
	if err != nil {
		t.Fatal(err)
	}
	if s.LastPeriodEnd != "2026-10-25" || s.PeriodDays != 7 {
		t.Fatalf("schedule = %+v", s)
	}
	if got := s.NextPeriodEnd(); got != "2026-11-01" {
		t.Fatalf("NextPeriodEnd = %s", got)
	}
	if got := s.PeriodStart("2026-11-01"); got != "2026-10-26" {
		t.Fatalf("PeriodStart = %s", got)
	}
	if _, err := InferSchedule(nil); err == nil {
		t.Fatal("empty history should fail")
	}
}

func TestParseDraft(t *testing.T) {
	d := ParseDraft(fixture(t, "PayrollInputStep"), fixture(t, "PayrollInputStepDraftEmployeePayrollsByIds"))
	if d.PeriodStart != "2026-10-26" || d.PeriodEnd != "2026-11-01" || d.RunBy != "2026-11-05" {
		t.Fatalf("period = %+v", d)
	}
	if len(d.Employees) != 1 {
		t.Fatalf("employees = %d", len(d.Employees))
	}
	e := d.Employees[0]
	if e.Name != "Ava Example" || e.RegularHours != 1 || e.HourlyRate != 16 || e.JobUUID == "" {
		t.Fatalf("employee = %+v", e)
	}
	if len(e.Reimbursements) != 1 || e.Reimbursements[0].Amount != 1.23 {
		t.Fatalf("reimbursements = %+v", e.Reimbursements)
	}
	if _, err := d.Employee("ava"); err != nil {
		t.Fatalf("first-name lookup: %v", err)
	}
	if _, err := d.Employee("Nobody Here"); err == nil {
		t.Fatal("unknown employee should fail")
	}
}

func TestParseReview(t *testing.T) {
	r := ParseReview(fixture(t, "PayrollReviewStep"))
	if r.Debit != 18.72 || r.Gross != 16 || r.Reimbursements != 1.23 || r.Net != 16.01 {
		t.Fatalf("review = %+v", r)
	}
	if len(r.Employees) != 1 || r.Employees[0].Name != "Ava Example" {
		t.Fatalf("review employees = %+v", r.Employees)
	}
	if !strings.Contains(r.FundingAccount, "XXXX") {
		t.Fatalf("funding account = %q", r.FundingAccount)
	}
}

func TestMileage(t *testing.T) {
	cases := []struct {
		miles, rate float64
		amt         float64
		desc        string
	}{
		{5.2, 0.60, 3.12, "Mileage 5.2 mi x $0.60"},
		{7.8, 0.60, 4.68, "Mileage 7.8 mi x $0.60"},
		{10, 0.70, 7.00, "Mileage 10 mi x $0.70"},
		{0.333, 0.655, 0.22, "Mileage 0.33 mi x $0.655"},
		{100, 0.725, 72.50, "Mileage 100 mi x $0.725"},
		{12, 0.5, 6.00, "Mileage 12 mi x $0.50"},
	}
	for _, c := range cases {
		amt, desc := Mileage(c.miles, c.rate)
		if amt != c.amt || desc != c.desc {
			t.Fatalf("Mileage(%v,%v) = %v %q, want %v %q", c.miles, c.rate, amt, desc, c.amt, c.desc)
		}
	}
}

func TestUpsertAndRemoveReimbursement(t *testing.T) {
	base := []Reimbursement{{Amount: 3.12, Description: "Mileage 5.2 mi x $0.60"}, {Amount: 20, Description: "Parking"}}
	out, replaced := UpsertReimbursement(base, Reimbursement{Amount: 4.68, Description: "mileage 5.2 MI x $0.60"})
	if !replaced || len(out) != 2 || out[0].Amount != 4.68 {
		t.Fatalf("upsert replace = %+v %v", out, replaced)
	}
	out, replaced = UpsertReimbursement(base, Reimbursement{Amount: 9, Description: "Tolls"})
	if replaced || len(out) != 3 {
		t.Fatalf("upsert add = %+v %v", out, replaced)
	}
	dup := append(base, Reimbursement{Amount: 1, Description: "Parking"})
	out, replaced = UpsertReimbursement(dup, Reimbursement{Amount: 5, Description: "Parking"})
	if !replaced || len(out) != 2 {
		t.Fatalf("upsert collapses duplicates = %+v", out)
	}
	left, n := RemoveReimbursement(base, "parking")
	if n != 1 || len(left) != 1 {
		t.Fatalf("remove = %+v %d", left, n)
	}
	left, n = RemoveReimbursement(base, "nothing")
	if n != 0 || len(left) != 2 {
		t.Fatalf("remove none = %+v %d", left, n)
	}
}

func TestParseTimesheet(t *testing.T) {
	csv := "Employee,Hours,Miles,Reimbursement,Description\nAva Example,4,2.6,,\nava example,4,,12.50,Parking\nBen,20,,,\n"
	rows, err := ParseTimesheet(strings.NewReader(csv))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %+v", rows)
	}
	if *rows[0].Hours != 8 || rows[0].Miles != 2.6 || len(rows[0].Reimbursements) != 1 {
		t.Fatalf("ava = %+v", rows[0])
	}
	for _, bad := range []string{"", "name,hours\nx,1\n", "employee,hours\nx,abc\n", "employee,reimbursement\nx,5\n"} {
		if _, err := ParseTimesheet(strings.NewReader(bad)); err == nil {
			t.Fatalf("ParseTimesheet(%q) should fail", bad)
		}
	}
}

func TestCompare(t *testing.T) {
	hours := 20.0
	rows := []TimesheetRow{{Employee: "Ava Example", Hours: &hours, Miles: 5.2, Reimbursements: []Reimbursement{}}}
	match := Draft{Employees: []DraftEmployee{{Name: "Ava Example", RegularHours: 20, Reimbursements: []Reimbursement{{Amount: 3.12, Description: "Mileage 5.2 mi x $0.60"}}}}}
	if got := Compare(rows, match, 0.60); len(got) != 0 {
		t.Fatalf("matching draft reported %+v", got)
	}
	wrong := Draft{Employees: []DraftEmployee{{Name: "Ava Example", RegularHours: 16, Reimbursements: []Reimbursement{
		{Amount: 3.00, Description: "Mileage 5.2 mi x $0.60"},
		{Amount: 1, Description: "Snacks"},
		{Amount: 1, Description: "Snacks"},
	}}}}
	got := Compare(rows, wrong, 0.60)
	fields := map[string]bool{}
	for _, m := range got {
		fields[m.Field] = true
	}
	for _, want := range []string{"regular_hours", "reimbursement:Mileage 5.2 mi x $0.60", "reimbursement:Snacks", "duplicate_reimbursement"} {
		if !fields[want] {
			t.Fatalf("missing mismatch %q in %+v", want, got)
		}
	}
	missing := Compare([]TimesheetRow{{Employee: "Zed", Reimbursements: []Reimbursement{}}}, match, 0.60)
	if len(missing) != 2 || missing[0].Field != "employee" || missing[1].Want != "not in timesheet" {
		t.Fatalf("unknown employee plus unexplained draft employee = %+v", missing)
	}
}

func TestReviewConsistent(t *testing.T) {
	cases := []struct {
		name  string
		r     Review
		want  bool
		total float64
	}{
		{"fresh", Review{Debit: 73.08, Employees: []ReviewEmployee{{EmployerCost: 73.08}}}, true, 73.08},
		{"stale", Review{Debit: 64.93, Employees: []ReviewEmployee{{EmployerCost: 73.08}}}, false, 73.08},
		{"two employees", Review{Debit: 30.5, Employees: []ReviewEmployee{{EmployerCost: 10.25}, {EmployerCost: 20.25}}}, true, 30.5},
		{"empty draft", Review{}, true, 0},
	}
	for _, c := range cases {
		if got := c.r.Consistent(); got != c.want || c.r.EmployerCostTotal() != c.total {
			t.Fatalf("%s: Consistent=%v total=%v", c.name, got, c.r.EmployerCostTotal())
		}
	}
	if !ParseReview(fixture(t, "PayrollReviewStep")).Consistent() {
		t.Fatal("captured review should be consistent")
	}
}

func TestEmployeeMatchingIsStrict(t *testing.T) {
	d := Draft{Employees: []DraftEmployee{{UUID: "u1", Name: "Ava Smith"}, {UUID: "u2", Name: "Ava Jones"}, {UUID: "u3", Name: "Joanna Lee"}}}
	if _, err := d.Employee("Ava"); err == nil || !strings.Contains(err.Error(), "matches 2 people") {
		t.Fatalf("shared first name should be ambiguous, got %v", err)
	}
	if e, err := d.Employee("ava jones"); err != nil || e.UUID != "u2" {
		t.Fatalf("exact full name: %+v %v", e, err)
	}
	if e, err := d.Employee("Joanna"); err != nil || e.UUID != "u3" {
		t.Fatalf("unique first name: %+v %v", e, err)
	}
	if _, err := d.Employee("ann"); err == nil || !strings.Contains(err.Error(), "did you mean Joanna Lee") {
		t.Fatalf("partial name must not match, got %v", err)
	}
	if e, err := d.Employee("u1"); err != nil || e.Name != "Ava Smith" {
		t.Fatalf("uuid: %+v %v", e, err)
	}
}

func TestWritableErr(t *testing.T) {
	cases := []struct {
		name string
		e    DraftEmployee
		ok   bool
	}{
		{"plain", DraftEmployee{Name: "A", Jobs: 1, Reimbursements: []Reimbursement{{Amount: 1, Description: "x", Type: ItemizedReimbursementType}}}, true},
		{"cli-built line", DraftEmployee{Name: "A", Jobs: 1, Reimbursements: []Reimbursement{{Amount: 1, Description: "x"}}}, true},
		{"two jobs", DraftEmployee{Name: "A", Jobs: 2}, false},
		{"recurring", DraftEmployee{Name: "A", Jobs: 1, Reimbursements: []Reimbursement{{Amount: 50, Description: "Phone", Type: "RECURRING_REIMBURSEMENT"}}}, false},
		{"unnamed amount", DraftEmployee{Name: "A", Jobs: 1, Reimbursements: []Reimbursement{{Amount: 5, Type: "UNNAMED_ITEMIZED_REIMBURSEMENT"}}}, false},
	}
	for _, c := range cases {
		if err := c.e.WritableErr(); (err == nil) != c.ok {
			t.Fatalf("%s: err=%v", c.name, err)
		}
	}
}

func TestUpsertMileageKeepsOneLine(t *testing.T) {
	a100, d100 := Mileage(100, 0.60)
	lines, _ := UpsertMileage(nil, Reimbursement{Amount: a100, Description: d100})
	lines = append(lines, Reimbursement{Amount: 20, Description: "Parking"})
	a120, d120 := Mileage(120, 0.60)
	lines, replaced := UpsertMileage(lines, Reimbursement{Amount: a120, Description: d120})
	if !replaced || len(lines) != 2 || lines[0].Amount != 72 || lines[0].Description != "Mileage 120 mi x $0.60" {
		t.Fatalf("mileage correction should replace: %+v", lines)
	}
	amt, desc := Mileage(12.345, 0.60)
	if desc != "Mileage 12.35 mi x $0.60" || amt != 7.41 {
		t.Fatalf("rounded miles drive both: %v %q", amt, desc)
	}
}

func TestCompareFlagsUnlistedEmployee(t *testing.T) {
	h := 4.0
	rows := []TimesheetRow{{Employee: "Ava Example", Hours: &h, Reimbursements: []Reimbursement{}}}
	d := Draft{Employees: []DraftEmployee{{UUID: "a", Name: "Ava Example", RegularHours: 4}, {UUID: "b", Name: "Ben Other", RegularHours: 8}}}
	got := Compare(rows, d, 0.60)
	if len(got) != 1 || got[0].Employee != "Ben Other" || got[0].Want != "not in timesheet" {
		t.Fatalf("unlisted employee with hours = %+v", got)
	}
	d.Employees[1].RegularHours = 0
	if got := Compare(rows, d, 0.60); len(got) != 0 {
		t.Fatalf("idle unlisted employee should pass: %+v", got)
	}
}

func TestIsMileageMatchesOnlyGeneratedLines(t *testing.T) {
	_, gen := Mileage(5.2, 0.725)
	cases := map[string]bool{
		gen:                      true,
		"Mileage 5.2 mi x $0.60": true,
		"Mileage to airport":     false,
		"Mileage reimbursement":  false,
		"Parking":                false,
	}
	for desc, want := range cases {
		if got := (Reimbursement{Description: desc}).IsMileage(); got != want {
			t.Fatalf("IsMileage(%q) = %v, want %v", desc, got, want)
		}
	}
	out, replaced := UpsertMileage([]Reimbursement{{Amount: 20, Description: "Mileage to airport"}}, Reimbursement{Amount: 3.12, Description: "Mileage 5.2 mi x $0.60"})
	if replaced || len(out) != 2 {
		t.Fatalf("UpsertMileage replaced a user line: %+v", out)
	}
}

func TestParseDraftKeepsOtherHourKinds(t *testing.T) {
	detail := map[string]any{"data": map[string]any{"company": map[string]any{"draftPayroll": map[string]any{
		"draftEmployeePayrollsByIds": []any{map[string]any{
			"employee": map[string]any{"uuid": "emp-1", "preferredFullName": "Ava Example"},
			"allWorkedHours": []any{map[string]any{
				"regularHours": "10", "overtimeHours": "1", "doubleOvertimeHours": "2", "missedBreakHours": "0.5",
				"employeeJobCompensation": map[string]any{"uuid": "job-1"},
			}},
		}},
	}}}}
	d := ParseDraft(map[string]any{}, detail)
	e := d.Employees[0]
	if e.DoubleOTHours != 2 || e.MissedBreak == nil || *e.MissedBreak != 0.5 {
		t.Fatalf("other hour kinds not parsed: %+v", e)
	}
}

func TestParseTimesheetRejectsNegative(t *testing.T) {
	for _, csv := range []string{"employee,hours\nAva,-2\n", "employee,miles\nAva,-5\n", "employee,reimbursement,description\nAva,-1,Parking\n"} {
		if _, err := ParseTimesheet(strings.NewReader(csv)); err == nil {
			t.Fatalf("expected an error for %q", csv)
		}
	}
}

func TestParseTimesheetJSON(t *testing.T) {
	rows, err := ParseTimesheetJSON(strings.NewReader(`[{"employee":"Ava Example","hours":12,"miles":5.2},{"employee":"ava example","hours":8,"reimbursements":[{"amount":12,"description":"Parking"}]}]`))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || *rows[0].Hours != 20 || rows[0].Miles != 5.2 || len(rows[0].Reimbursements) != 1 {
		t.Fatalf("rows = %+v", rows)
	}
	for _, bad := range []string{`[{"hours":1}]`, `[{"employee":"A","hours":-1}]`, `[{"employee":"A","reimbursements":[{"amount":5}]}]`, `[{"employee":"A","bogus":1}]`, `{}`} {
		if _, err := ParseTimesheetJSON(strings.NewReader(bad)); err == nil {
			t.Fatalf("expected an error for %s", bad)
		}
	}
}
