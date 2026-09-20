package nepraparse

import (
	"errors"
	"testing"
	"time"
)

// TestMonthsAreFiscalNotCalendar is the mis-dating trap. Column position 7 in
// the monthly block is January, not July; a parser that treats fiscal
// position as a calendar month number shifts every observation by six months.
func TestMonthsAreFiscalNotCalendar(t *testing.T) {
	tests := []struct {
		m            Month
		wantLabel    string
		wantFiscal   int
		wantCalendar time.Month
	}{
		{Jul, "Jul", 1, time.July},
		{Aug, "Aug", 2, time.August},
		{Sep, "Sep", 3, time.September},
		{Oct, "Oct", 4, time.October},
		{Nov, "Nov", 5, time.November},
		{Dec, "Dec", 6, time.December},
		{Jan, "Jan", 7, time.January},
		{Feb, "Feb", 8, time.February},
		{Mar, "Mar", 9, time.March},
		{Apr, "Apr", 10, time.April},
		{May, "May", 11, time.May},
		{Jun, "Jun", 12, time.June},
	}
	for _, tc := range tests {
		t.Run(tc.wantLabel, func(t *testing.T) {
			if got := tc.m.String(); got != tc.wantLabel {
				t.Errorf("String() = %q, want %q", got, tc.wantLabel)
			}
			if got := tc.m.FiscalIndex(); got != tc.wantFiscal {
				t.Errorf("FiscalIndex() = %d, want %d", got, tc.wantFiscal)
			}
			if got := tc.m.Calendar(); got != tc.wantCalendar {
				t.Errorf("Calendar() = %v, want %v", got, tc.wantCalendar)
			}
			if !tc.m.Valid() {
				t.Error("Valid() = false")
			}
		})
	}
	// The trap, stated as an assertion: fiscal position 7 is January.
	if Month(7).Calendar() != time.January {
		t.Errorf("fiscal position 7 maps to %v, want January", Month(7).Calendar())
	}
	if MonthsInFiscalOrder[0] != Jul || MonthsInFiscalOrder[11] != Jun {
		t.Errorf("MonthsInFiscalOrder = %v, want Jul first and Jun last", MonthsInFiscalOrder)
	}
	if Month(0).Valid() || Month(13).Valid() {
		t.Error("out-of-range months must report Valid() = false")
	}
}

func TestFiscalYearPeriod(t *testing.T) {
	fy := FiscalYear{Start: 2023, End: 2024}
	tests := []struct {
		m       Month
		wantY   int
		wantMon time.Month
		wantKey string
	}{
		{Jul, 2023, time.July, "2023-07"},
		{Dec, 2023, time.December, "2023-12"},
		{Jan, 2024, time.January, "2024-01"},
		{Jun, 2024, time.June, "2024-06"},
	}
	for _, tc := range tests {
		y, mon, ok := fy.Period(tc.m)
		if !ok {
			t.Fatalf("Period(%v) not ok", tc.m)
		}
		if y != tc.wantY || mon != tc.wantMon {
			t.Errorf("Period(%v) = %d/%v, want %d/%v", tc.m, y, mon, tc.wantY, tc.wantMon)
		}
		key, ok := fy.PeriodKey(tc.m)
		if !ok || key != tc.wantKey {
			t.Errorf("PeriodKey(%v) = %q/%v, want %q", tc.m, key, ok, tc.wantKey)
		}
	}
	if _, _, ok := fy.Period(Month(0)); ok {
		t.Error("Period on an invalid month must report ok = false")
	}
	if _, _, ok := (FiscalYear{}).Period(Jul); ok {
		t.Error("Period on a zero fiscal year must report ok = false")
	}
}

func TestParseFiscalYear(t *testing.T) {
	tests := []struct {
		in      string
		want    FiscalYear
		wantErr bool
	}{
		{in: "2023-24", want: FiscalYear{2023, 2024}},
		{in: "2017-18", want: FiscalYear{2017, 2018}},
		{in: "FY 2020-21", want: FiscalYear{2020, 2021}},
		{in: "FY2020-21", want: FiscalYear{2020, 2021}},
		{in: "2020-2021", want: FiscalYear{2020, 2021}},
		{in: "1999-00", want: FiscalYear{1999, 2000}},
		{in: "2023/24", want: FiscalYear{2023, 2024}},
		{in: "2023-25", wantErr: true},
		{in: "2023", wantErr: true},
		{in: "twenty-three", wantErr: true},
		{in: "", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			got, err := ParseFiscalYear(tc.in)
			if tc.wantErr {
				if !errors.Is(err, ErrFiscalYear) {
					t.Fatalf("err = %v, want %v", err, ErrFiscalYear)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseFiscalYear(%q): %v", tc.in, err)
			}
			if got != tc.want {
				t.Errorf("= %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestFiscalYearLabels(t *testing.T) {
	fy := FiscalYear{2017, 2018}
	if got := fy.Label(); got != "2017-18" {
		t.Errorf("Label() = %q, want %q", got, "2017-18")
	}
	if got := fy.BandLabel(); got != "FY 2017-18" {
		t.Errorf("BandLabel() = %q, want %q", got, "FY 2017-18")
	}
	if (FiscalYear{}).Zero() != true {
		t.Error("the zero FiscalYear must report Zero() = true")
	}
}
