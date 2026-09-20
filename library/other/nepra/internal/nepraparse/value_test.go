package nepraparse

import (
	"reflect"
	"testing"
)

func TestParseValue(t *testing.T) {
	tests := []struct {
		name      string
		cell      string
		wantState CellState
		wantNum   float64
		wantOK    bool
		wantRaw   string
		wantComma bool
	}{
		{
			name: "a plain number", cell: "94.94",
			wantState: StateNumeric, wantNum: 94.94, wantOK: true, wantRaw: "94.94",
		},
		{
			name: "a real measured zero is data, not absence", cell: "0.00",
			wantState: StateNumeric, wantNum: 0, wantOK: true, wantRaw: "0.00",
		},
		{
			name: "thousands separator, integer", cell: "3,478",
			wantState: StateNumeric, wantNum: 3478, wantOK: true, wantRaw: "3,478", wantComma: true,
		},
		{
			name: "thousands separator, decimal", cell: "2,456.58",
			wantState: StateNumeric, wantNum: 2456.58, wantOK: true, wantRaw: "2,456.58", wantComma: true,
		},
		{
			name: "two-group thousands separator", cell: "13,365.93",
			wantState: StateNumeric, wantNum: 13365.93, wantOK: true, wantRaw: "13,365.93", wantComma: true,
		},
		{
			name: "an empty cell is NOT REPORTED", cell: "",
			wantState: StateNotReported,
		},
		{
			name: "an NBSP cell is NOT REPORTED", cell: " ",
			wantState: StateNotReported,
		},
		{
			name: "an NBSP among spaces is still NOT REPORTED", cell: "  \n  ",
			wantState: StateNotReported,
		},
		{
			name: "DELICENSED", cell: "DELICENSED",
			wantState: StateDelicensed, wantRaw: "DELICENSED",
		},
		{
			name: "DECOMMISSIONED", cell: "DECOMMISSIONED",
			wantState: StateDecommissioned, wantRaw: "DECOMMISSIONED",
		},
		{
			name: "Export to K.Electric, wrapped across source lines", cell: "Export\n  to K.Electric",
			wantState: StateExportToKElectric, wantRaw: "Export to K.Electric",
		},
		{
			name: "the stray semicolon in the padding column is unmodelled text, not a number", cell: ";",
			wantState: StateUnknownText, wantRaw: ";",
		},
		{
			name: "mis-grouped commas are not silently accepted", cell: "1,23",
			wantState: StateUnknownText, wantRaw: "1,23",
		},
		{
			name: "a bare dash is not a zero", cell: "-",
			wantState: StateUnknownText, wantRaw: "-",
		},
		{
			name: "a negative number still parses", cell: "-1.50",
			wantState: StateNumeric, wantNum: -1.5, wantOK: true, wantRaw: "-1.50",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			v := ParseValue(tc.cell)
			if v.State() != tc.wantState {
				t.Errorf("State() = %v, want %v", v.State(), tc.wantState)
			}
			num, ok := v.Float64()
			if ok != tc.wantOK {
				t.Fatalf("Float64() ok = %v, want %v", ok, tc.wantOK)
			}
			if ok && num != tc.wantNum {
				t.Errorf("Float64() = %v, want %v", num, tc.wantNum)
			}
			if !ok && num != 0 {
				t.Errorf("Float64() returned %v with ok=false; callers must not see a value there", num)
			}
			if v.Raw() != tc.wantRaw {
				t.Errorf("Raw() = %q, want %q", v.Raw(), tc.wantRaw)
			}
			if v.HadThousandsSeparator() != tc.wantComma {
				t.Errorf("HadThousandsSeparator() = %v, want %v", v.HadThousandsSeparator(), tc.wantComma)
			}
			if v.Present() != (tc.wantState == StateNumeric) {
				t.Errorf("Present() = %v, want %v", v.Present(), tc.wantState == StateNumeric)
			}
		})
	}
}

// TestStatusSentinelsNeverBecomeNumbers is the ordering requirement: commas
// are stripped only after a cell has been classified as numeric, so no
// sentinel can be coerced through strconv on the way past.
func TestStatusSentinelsNeverBecomeNumbers(t *testing.T) {
	for _, cell := range []string{"DELICENSED", "DECOMMISSIONED", "Export to K.Electric", ";", "n/a", "TBD"} {
		v := ParseValue(cell)
		if _, ok := v.Float64(); ok {
			t.Errorf("ParseValue(%q) produced a number", cell)
		}
		if v.State() == StateNotReported {
			t.Errorf("ParseValue(%q) collapsed to NOT REPORTED; text state must be preserved", cell)
		}
	}
}

// TestValueHidesItsNumberBehindState enforces the API rule that no bare
// float can leak: Value has no exported fields, so a caller cannot reach the
// number without going through Float64 and handling the absent case.
func TestValueHidesItsNumberBehindState(t *testing.T) {
	rt := reflect.TypeOf(Value{})
	for i := 0; i < rt.NumField(); i++ {
		if f := rt.Field(i); f.IsExported() {
			t.Errorf("Value.%s is exported; a bare field lets a caller read a number without its state", f.Name)
		}
	}
}

func TestCellStateNames(t *testing.T) {
	tests := []struct {
		state      CellState
		want       string
		wantStatus bool
	}{
		{StateNumeric, "numeric", false},
		{StateNotReported, "not_reported", false},
		{StateDelicensed, "delicensed", true},
		{StateDecommissioned, "decommissioned", true},
		{StateExportToKElectric, "export_to_k_electric", true},
		{StateUnknownText, "unknown_text", false},
	}
	if len(tests) != 6 {
		t.Fatal("all five modelled states plus the unmodelled bucket must be covered")
	}
	for _, tc := range tests {
		if got := tc.state.String(); got != tc.want {
			t.Errorf("State(%d).String() = %q, want %q", tc.state, got, tc.want)
		}
		if got := tc.state.Status(); got != tc.wantStatus {
			t.Errorf("%s.Status() = %v, want %v", tc.want, got, tc.wantStatus)
		}
	}
}

func TestValueString(t *testing.T) {
	tests := []struct {
		cell string
		want string
	}{
		{"2,456.58", "2,456.58"},
		{"", "<not reported>"},
		{"DELICENSED", "<delicensed: DELICENSED>"},
		{";", "<unknown_text: ;>"},
	}
	for _, tc := range tests {
		if got := ParseValue(tc.cell).String(); got != tc.want {
			t.Errorf("ParseValue(%q).String() = %q, want %q", tc.cell, got, tc.want)
		}
	}
}
