package nepraparse

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"time"
)

// ErrFiscalYear is returned for a fiscal-year label this package cannot read.
var ErrFiscalYear = errors.New("nepraparse: unrecognised fiscal year")

// Month is a position in NEPRA's fiscal year, 1 = Jul .. 12 = Jun.
//
// The workbooks publish months in FISCAL order Jul->Jun. Treating column 7 as
// January because it is the seventh month column mis-dates every observation
// by up to six months, so this type is deliberately not time.Month and the
// conversion is explicit ([Month.Calendar], [FiscalYear.Period]).
type Month uint8

// The twelve fiscal months, in the order the columns appear.
const (
	Jul Month = iota + 1
	Aug
	Sep
	Oct
	Nov
	Dec
	Jan
	Feb
	Mar
	Apr
	May
	Jun
)

// MonthsInFiscalOrder is the exact column order of the monthly block.
var MonthsInFiscalOrder = [12]Month{Jul, Aug, Sep, Oct, Nov, Dec, Jan, Feb, Mar, Apr, May, Jun}

var monthLabels = [...]string{
	Jul: "Jul", Aug: "Aug", Sep: "Sep", Oct: "Oct", Nov: "Nov", Dec: "Dec",
	Jan: "Jan", Feb: "Feb", Mar: "Mar", Apr: "Apr", May: "May", Jun: "Jun",
}

var monthCalendar = [...]time.Month{
	Jul: time.July, Aug: time.August, Sep: time.September,
	Oct: time.October, Nov: time.November, Dec: time.December,
	Jan: time.January, Feb: time.February, Mar: time.March,
	Apr: time.April, May: time.May, Jun: time.June,
}

// String returns the workbook's own three-letter label, e.g. "Jul".
func (m Month) String() string {
	if int(m) < len(monthLabels) && monthLabels[m] != "" {
		return monthLabels[m]
	}
	return "Month(" + strconv.Itoa(int(m)) + ")"
}

// FiscalIndex returns the 1-based position in the fiscal year, 1 for Jul.
func (m Month) FiscalIndex() int { return int(m) }

// Calendar returns the calendar month. Jul..Dec fall in the first calendar
// year of the fiscal label and Jan..Jun in the second; use
// [FiscalYear.Period] to get the year too.
func (m Month) Calendar() time.Month {
	if int(m) < len(monthCalendar) && monthCalendar[m] != 0 {
		return monthCalendar[m]
	}
	return 0
}

// Valid reports whether m is one of the twelve fiscal months.
func (m Month) Valid() bool { return m >= Jul && m <= Jun }

// FiscalYear is a NEPRA fiscal year: 1 July of Start to 30 June of End.
type FiscalYear struct {
	Start int // calendar year containing July
	End   int // calendar year containing June
}

var fyRE = regexp.MustCompile(`^(?:FY\s*)?(\d{4})\s*[-/]\s*(\d{2}|\d{4})$`)

// ParseFiscalYear reads the labels these workbooks use: "2023-24", and also
// "FY 2023-24" and "2023-2024".
func ParseFiscalYear(s string) (FiscalYear, error) {
	m := fyRE.FindStringSubmatch(collapseText(s))
	if m == nil {
		return FiscalYear{}, fmt.Errorf("%w: %q", ErrFiscalYear, s)
	}
	start, err := strconv.Atoi(m[1])
	if err != nil {
		return FiscalYear{}, fmt.Errorf("%w: %q", ErrFiscalYear, s)
	}
	end, err := strconv.Atoi(m[2])
	if err != nil {
		return FiscalYear{}, fmt.Errorf("%w: %q", ErrFiscalYear, s)
	}
	if len(m[2]) == 2 {
		end = start/100*100 + end
		if end < start {
			end += 100
		}
	}
	if end != start+1 {
		return FiscalYear{}, fmt.Errorf("%w: %q spans %d years, expected one", ErrFiscalYear, s, end-start)
	}
	return FiscalYear{Start: start, End: end}, nil
}

// Label renders the fiscal year the way the file names it, e.g. "2023-24".
func (f FiscalYear) Label() string {
	return fmt.Sprintf("%04d-%02d", f.Start, f.End%100)
}

// BandLabel renders the header band's own text, e.g. "FY 2023-24". The band
// cell is the only place in the document that states which year the file is.
func (f FiscalYear) BandLabel() string { return "FY " + f.Label() }

// Zero reports whether the fiscal year is unset.
func (f FiscalYear) Zero() bool { return f.Start == 0 && f.End == 0 }

// Period maps a fiscal month onto its calendar year and month. Jul..Dec land
// in Start, Jan..Jun in End.
func (f FiscalYear) Period(m Month) (year int, month time.Month, ok bool) {
	if !m.Valid() || f.Zero() {
		return 0, 0, false
	}
	if m <= Dec {
		return f.Start, m.Calendar(), true
	}
	return f.End, m.Calendar(), true
}

// PeriodKey renders a fiscal month as a sortable calendar key, "2023-07" for
// Jul of FY2023-24 and "2024-06" for Jun. Useful as a panel index.
func (f FiscalYear) PeriodKey(m Month) (string, bool) {
	y, mo, ok := f.Period(m)
	if !ok {
		return "", false
	}
	return fmt.Sprintf("%04d-%02d", y, int(mo)), true
}

// normaliseFYArg accepts the fy argument in any of the forms a caller is
// likely to have to hand, including a bare band label.
func normaliseFYArg(fy string) (FiscalYear, error) {
	// An EMPTY argument means "do not cross-check", which is a documented
	// choice. A WHITESPACE-ONLY argument is a caller bug — very likely an
	// NBSP-padded cell that was passed straight through — and collapsing it
	// to "" would silently disable the one check that stops a whole file
	// being dated to the wrong fiscal year. So the two are distinguished
	// before collapsing.
	if fy == "" {
		return FiscalYear{}, nil
	}
	collapsed := collapseText(fy)
	if collapsed == "" {
		return FiscalYear{}, fmt.Errorf("nepraparse: fiscal year argument %q is whitespace only; pass \"\" to skip the cross-check deliberately", fy)
	}
	return ParseFiscalYear(collapsed)
}
