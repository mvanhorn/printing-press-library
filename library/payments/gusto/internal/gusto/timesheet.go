// Copyright 2026 david and contributors. Licensed under Apache-2.0. See LICENSE.

package gusto

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// TimesheetRow is one employee's inputs for a pay period.
type TimesheetRow struct {
	Employee       string          `json:"employee"`
	Hours          *float64        `json:"hours,omitempty"`
	Miles          float64         `json:"miles"`
	Reimbursements []Reimbursement `json:"reimbursements"`
}

// ParseTimesheet reads a CSV with a header row. Recognized columns
// (case-insensitive): employee, hours, miles, reimbursement, description.
// Several rows for the same employee are summed (hours, miles) or appended
// (reimbursement lines), so a daily log works as well as a weekly total.
func ParseTimesheet(r io.Reader) ([]TimesheetRow, error) {
	cr := csv.NewReader(r)
	cr.TrimLeadingSpace = true
	cr.FieldsPerRecord = -1
	records, err := cr.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("reading timesheet CSV: %w", err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("timesheet CSV is empty")
	}
	col := map[string]int{}
	for i, h := range records[0] {
		col[strings.ToLower(strings.TrimSpace(h))] = i
	}
	if _, ok := col["employee"]; !ok {
		return nil, fmt.Errorf("timesheet CSV needs an 'employee' column (have: %s)", strings.Join(records[0], ", "))
	}
	cell := func(rec []string, name string) string {
		i, ok := col[name]
		if !ok || i >= len(rec) {
			return ""
		}
		return strings.TrimSpace(rec[i])
	}
	num := func(rec []string, name string, line int) (float64, bool, error) {
		s := strings.TrimPrefix(cell(rec, name), "$")
		if s == "" {
			return 0, false, nil
		}
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0, false, fmt.Errorf("line %d: %s %q is not a number", line, name, s)
		}
		if v < 0 {
			return 0, false, fmt.Errorf("line %d: %s %q is negative", line, name, s)
		}
		return v, true, nil
	}
	var b timesheetBuilder
	for i, rec := range records[1:] {
		line := i + 2
		name := cell(rec, "employee")
		if name == "" {
			continue
		}
		row := b.row(name)
		if h, has, err := num(rec, "hours", line); err != nil {
			return nil, err
		} else if has {
			row.addHours(h)
		}
		if m, has, err := num(rec, "miles", line); err != nil {
			return nil, err
		} else if has {
			row.Miles += m
		}
		if amt, has, err := num(rec, "reimbursement", line); err != nil {
			return nil, err
		} else if has && amt != 0 {
			desc := cell(rec, "description")
			if desc == "" {
				return nil, fmt.Errorf("line %d: reimbursement %.2f needs a description", line, amt)
			}
			row.Reimbursements = append(row.Reimbursements, Reimbursement{Amount: amt, Description: desc})
		}
	}
	return b.rows(), nil
}

// ParseTimesheetJSON reads the same inputs as a JSON array, for agents:
// [{"employee":"Ava","hours":20,"miles":5.2,"reimbursements":[{"amount":12,"description":"Parking"}]}].
// Entries for the same employee are combined as in ParseTimesheet.
func ParseTimesheetJSON(r io.Reader) ([]TimesheetRow, error) {
	var in []struct {
		Employee       string          `json:"employee"`
		Hours          *float64        `json:"hours"`
		Miles          float64         `json:"miles"`
		Reimbursements []Reimbursement `json:"reimbursements"`
	}
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		return nil, fmt.Errorf("reading timesheet JSON: %w", err)
	}
	var b timesheetBuilder
	for i, e := range in {
		name := strings.TrimSpace(e.Employee)
		if name == "" {
			return nil, fmt.Errorf("entry %d: employee is required", i)
		}
		if (e.Hours != nil && *e.Hours < 0) || e.Miles < 0 {
			return nil, fmt.Errorf("entry %d: hours and miles must not be negative", i)
		}
		row := b.row(name)
		if e.Hours != nil {
			row.addHours(*e.Hours)
		}
		row.Miles += e.Miles
		for _, l := range e.Reimbursements {
			if l.Amount < 0 {
				return nil, fmt.Errorf("entry %d: reimbursement %.2f is negative", i, l.Amount)
			}
			if l.Amount == 0 {
				continue
			}
			if strings.TrimSpace(l.Description) == "" {
				return nil, fmt.Errorf("entry %d: reimbursement %.2f needs a description", i, l.Amount)
			}
			row.Reimbursements = append(row.Reimbursements, Reimbursement{Amount: l.Amount, Description: l.Description})
		}
	}
	return b.rows(), nil
}

// timesheetBuilder combines rows by employee name (case-insensitive), in
// first-seen order.
type timesheetBuilder struct {
	order  []string
	byName map[string]*TimesheetRow
}

func (b *timesheetBuilder) row(name string) *TimesheetRow {
	if b.byName == nil {
		b.byName = map[string]*TimesheetRow{}
	}
	key := strings.ToLower(name)
	row, ok := b.byName[key]
	if !ok {
		row = &TimesheetRow{Employee: name, Reimbursements: make([]Reimbursement, 0)}
		b.byName[key] = row
		b.order = append(b.order, key)
	}
	return row
}

func (b *timesheetBuilder) rows() []TimesheetRow {
	out := make([]TimesheetRow, 0, len(b.order))
	for _, k := range b.order {
		out = append(out, *b.byName[k])
	}
	return out
}

func (t *TimesheetRow) addHours(h float64) {
	if t.Hours != nil {
		h += *t.Hours
	}
	t.Hours = &h
}

// Expected returns the reimbursement lines a timesheet row should produce at
// a per-mile rate (mileage first, then itemized lines).
func (t TimesheetRow) Expected(rate float64) []Reimbursement {
	lines := make([]Reimbursement, 0, len(t.Reimbursements)+1)
	if t.Miles > 0 {
		amt, desc := Mileage(t.Miles, rate)
		lines = append(lines, Reimbursement{Amount: amt, Description: desc})
	}
	return append(lines, t.Reimbursements...)
}

// Mismatch is one difference between the timesheet and the draft.
type Mismatch struct {
	Employee string `json:"employee"`
	Field    string `json:"field"`
	Want     string `json:"want"`
	Got      string `json:"got"`
	// Detail carries the full explanation when Got is a short status.
	Detail string `json:"detail,omitempty"`
}

// Compare diffs timesheet rows against a draft at a per-mile rate. It reports
// hours that differ, expected reimbursement lines that are missing or have a
// different amount, lines on the draft that the timesheet does not explain,
// and duplicate descriptions.
func Compare(rows []TimesheetRow, d Draft, rate float64) []Mismatch {
	out := make([]Mismatch, 0)
	covered := map[string]bool{}
	for _, row := range rows {
		e, err := d.Employee(row.Employee)
		if err != nil {
			out = append(out, Mismatch{Employee: row.Employee, Field: "employee", Want: "on draft", Got: "not on draft", Detail: err.Error()})
			continue
		}
		covered[e.UUID] = true
		if row.Hours != nil && RoundCents(*row.Hours) != RoundCents(e.RegularHours) {
			out = append(out, Mismatch{Employee: e.Name, Field: "regular_hours", Want: fmtNum(*row.Hours), Got: fmtNum(e.RegularHours)})
		}
		seen := map[string]int{}
		for _, l := range e.Reimbursements {
			seen[descKey(l.Description)]++
		}
		for desc, n := range seen {
			if n > 1 {
				out = append(out, Mismatch{Employee: e.Name, Field: "duplicate_reimbursement", Want: "1 line", Got: fmt.Sprintf("%d lines %q", n, desc)})
			}
		}
		expected := row.Expected(rate)
		explained := map[string]bool{}
		for _, want := range expected {
			key := descKey(want.Description)
			explained[key] = true
			found := false
			for _, got := range e.Reimbursements {
				if descKey(got.Description) == key {
					found = true
					if RoundCents(got.Amount) != RoundCents(want.Amount) {
						out = append(out, Mismatch{Employee: e.Name, Field: "reimbursement:" + want.Description, Want: Money(want.Amount), Got: Money(got.Amount)})
					}
					break
				}
			}
			if !found {
				out = append(out, Mismatch{Employee: e.Name, Field: "reimbursement:" + want.Description, Want: Money(want.Amount), Got: "missing"})
			}
		}
		for _, got := range e.Reimbursements {
			if !explained[descKey(got.Description)] {
				out = append(out, Mismatch{Employee: e.Name, Field: "reimbursement:" + got.Description, Want: "not in timesheet", Got: Money(got.Amount)})
			}
		}
	}
	// A draft employee with hours or reimbursements that the timesheet does
	// not mention is unexpected pay; verify gates the handoff, so flag it.
	for _, e := range d.Employees {
		if covered[e.UUID] {
			continue
		}
		if RoundCents(e.RegularHours) > 0 || len(e.Reimbursements) > 0 {
			out = append(out, Mismatch{Employee: e.Name, Field: "employee", Want: "not in timesheet", Got: fmt.Sprintf("%s hours, %s reimbursements", fmtNum(e.RegularHours), Money(e.ReimbursementTotal()))})
		}
	}
	return out
}

func fmtNum(v float64) string { return strconv.FormatFloat(RoundCents(v), 'f', -1, 64) }

// Money formats dollars for display.
func Money(v float64) string { return fmt.Sprintf("$%.2f", v) }

// descKey is the map key form of SameDescription.
func descKey(s string) string { return strings.ToLower(strings.TrimSpace(s)) }
