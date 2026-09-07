// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cdcpdf

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Layout identifies one of CDC's report shapes. The report file name does NOT
// change when the layout does, so layout is detected from content, never from
// the name or the vintage date.
type Layout string

const (
	// LayoutEquity2025 is the print-driver era part A:
	//   seq ISIN NAME [markers] SHORT DATE STATUS shares paidup_incl pct_incl paidup_excl
	LayoutEquity2025 Layout = "equity-2025"
	// LayoutEquity2024 is the Excel era part A. It carries a Market Value column
	// and TWO percentage columns, and has NO status column:
	//   seq ISIN NAME [markers] SHORT DATE shares mktval paidup_incl pct_incl paidup_excl pct_excl
	LayoutEquity2024 Layout = "equity-2024"
	// LayoutFunds2025 is part B (open-end / ETF funds / saving certificates):
	//   seq ISIN NAME SYMBOL DATE STATUS units
	// Funds have units, not share capital, so there is no paid-up or percentage
	// column and none may be invented for these rows.
	LayoutFunds2025 Layout = "funds-2025"
	// LayoutDebt is the sukuk / TFC / commercial-paper shape. It REORDERS the
	// columns relative to the equity layouts -- status comes BEFORE the live date
	// -- and adds a maturity date in a fifth date format (DD-Mon-YY):
	//   seq ISIN NAME SYMBOL STATUS live_date maturity_date units [paidup pct]
	// It appears interleaved with equity rows inside the SAME report part, which
	// is the "column layouts differ per instrument type" trap.
	LayoutDebt    Layout = "debt"
	LayoutUnknown Layout = "unknown"
)

// numericFields is the exact count of numeric cells each layout must yield. A
// row that does not match its layout's count is reported as a finding rather
// than silently coerced -- a short row here is how a column shift becomes
// invisible.
var numericFields = map[Layout]int{
	LayoutEquity2025: 4,
	LayoutEquity2024: 6,
	LayoutFunds2025:  1,
	LayoutDebt:       0, // variable: 1 (funds-style) to 3 (equity-style); validated by range
}

func (l Layout) NumericFields() int { return numericFields[l] }

// anyDate reports whether tok is any of CDC's live-date encodings.
func anyDate(tok string) bool { return dateRe.MatchString(tok) || numDateRe.MatchString(tok) }

// dateAmbiguous reports whether a numeric date could be read either as
// DD/MM/YYYY or MM/DD/YYYY. Both orders occur in one report, so an ambiguous
// value must be surfaced, never normalised.
func dateAmbiguous(tok string) bool {
	if !numDateRe.MatchString(tok) {
		return false
	}
	parts := strings.Split(tok, "/")
	a, _ := strconv.Atoi(parts[0])
	b, _ := strconv.Atoi(parts[1])
	return a <= 12 && b <= 12
}

// Row is one security's line in a penetration report. Fields absent from a
// layout stay nil so "not published for this instrument class" is distinct from
// "published as zero".
type Row struct {
	Seq          int      `json:"seq"`
	ISIN         string   `json:"isin"`
	Name         string   `json:"name"`
	NameMarkers  []string `json:"name_markers,omitempty"`
	Symbol       string   `json:"symbol"`
	LiveDate     string   `json:"live_date"`
	Status       string   `json:"status,omitempty"`
	MaturityDate string   `json:"maturity_date,omitempty"`
	SharesInCDS  *float64 `json:"shares_in_cds"`
	MarketValue  *float64 `json:"market_value,omitempty"`
	PaidUpIncl   *float64 `json:"paid_up_incl_gop,omitempty"`
	PctIncl      *float64 `json:"pct_incl_gop,omitempty"`
	PaidUpExcl   *float64 `json:"paid_up_excl_gop,omitempty"`
	PctExcl      *float64 `json:"pct_excl_gop,omitempty"`
	Layout       Layout   `json:"layout"`
}

// Finding is a per-row extraction problem. Callers persist these; `--strict`
// emit paths refuse a vintage that carries any unexplained finding.
type Finding struct {
	ISIN      string `json:"isin,omitempty"`
	Line      string `json:"line"`
	CheckName string `json:"check_name"`
	Severity  string `json:"severity"`
	Detail    string `json:"detail"`
}

// Parsed is the outcome of parsing one report part.
type Parsed struct {
	Title    string    `json:"title"`
	Layout   Layout    `json:"layout"`
	Rows     []Row     `json:"rows"`
	Findings []Finding `json:"findings"`
	Skipped  int       `json:"skipped_non_data_lines"`
}

var (
	isinRe = regexp.MustCompile(`^[A-Z]{2}[A-Z0-9]{9}[0-9]$`)
	dateRe = regexp.MustCompile(`^\d{1,2}/[A-Za-z]{3}/\d{4}$`)
	seqRe  = regexp.MustCompile(`^\d{1,6}$`)
	numRe  = regexp.MustCompile(`^\(?-?[\d,]*\.?\d*\)?$`)
	// Status words observed across vintages. Anything else in the status
	// position is surfaced as a finding rather than absorbed into the name.
	statusWords = map[string]bool{
		"LISTED": true, "UNLISTED": true, "UN-LISTED": true, "DELISTED": true,
		"DE-LISTED": true, "SUSPENDED": true, "FREEZE": true, "FROZEN": true,
		"ACTIVE": true, "INACTIVE": true,
	}
	// numDateRe is CDC's FOURTH date encoding and it is AMBIGUOUS. Observed both
	// 10/24/2013 (only valid as MM/DD/YYYY) and 13/03/2025 (only valid as
	// DD/MM/YYYY) in the SAME report, so the order cannot be inferred globally.
	// The raw string is preserved and the ambiguity is flagged; a date is never
	// silently normalised into a guess.
	numDateRe = regexp.MustCompile(`^\d{1,2}/\d{1,2}/\d{4}$`)
	// maturityRe is the FIFTH encoding, used only by debt instruments.
	maturityRe = regexp.MustCompile(`^\d{1,2}-[A-Za-z]{3}-\d{2}$`)
	// markerGlyphRe matches the non-ASCII marker glyphs CDC embeds in names.
	markerGlyphRe = regexp.MustCompile(`^[\x{25D8}\x{25CF}\x{25AA}\x{2022}*]+$`)
)

// DetectLayout classifies a report part from its header region. Detection uses
// the instrument-class title plus the presence of era-specific column headers;
// the vintage date is deliberately NOT an input, because CDC changed the column
// set without renaming the file.
func DetectLayout(text string) (Layout, string) {
	head := text
	if len(head) > 4000 {
		head = head[:4000]
	}
	upper := strings.ToUpper(strings.Join(strings.Fields(head), " "))

	title := ""
	for _, l := range strings.Split(text, "\n") {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(strings.ToUpper(t), "DETAILS OF") {
			title = t
			break
		}
	}

	isFunds := strings.Contains(upper, "OPEN END") ||
		strings.Contains(upper, "ETF FUNDS") ||
		strings.Contains(upper, "UNITS AVAILABLE")
	if isFunds {
		return LayoutFunds2025, title
	}
	if strings.Contains(upper, "ORDINARY") || strings.Contains(upper, "MODARABA") {
		// "Market Value" is the Excel-era tell; "STATUS" is the print-driver tell.
		if strings.Contains(upper, "MARKET VALUE") {
			return LayoutEquity2024, title
		}
		if strings.Contains(upper, "STATUS") {
			return LayoutEquity2025, title
		}
	}
	return LayoutUnknown, title
}

// ParseRows extracts every data row from one report part.
//
// Rows are located by anchoring on two stable tokens -- a 12-character ISIN and
// a D/Mon/YYYY live date -- rather than by column position, because PDFKit
// linearises print-driver output and column x-offsets are not recoverable.
func ParseRows(text string) *Parsed {
	layout, title := DetectLayout(text)
	out := &Parsed{Title: title, Layout: layout, Rows: []Row{}, Findings: []Finding{}}
	want := layout.NumericFields()

	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		fields := splitRowTokens(line)
		if len(fields) < 5 || !seqRe.MatchString(fields[0]) || !isinRe.MatchString(fields[1]) {
			out.Skipped++
			continue
		}

		dateIdx := -1
		for i := 2; i < len(fields); i++ {
			if anyDate(fields[i]) {
				dateIdx = i
				break
			}
		}
		if dateIdx < 3 {
			out.Findings = append(out.Findings, Finding{
				ISIN: fields[1], Line: line, CheckName: "row_no_live_date",
				Severity: "error", Detail: "no D/Mon/YYYY live date found after the ISIN",
			})
			continue
		}

		seq, _ := strconv.Atoi(fields[0])
		row := Row{Seq: seq, ISIN: fields[1], LiveDate: fields[dateIdx], Layout: layout}

		// The token immediately before the date is the short name / symbol; the
		// tokens between the ISIN and it are the security name, which may carry
		// trailing FREEZE / * / *** markers.
		// DEBT ORDERING: status sits immediately BEFORE the live date, so the
		// token before the date is a status word rather than the symbol.
		symIdx := dateIdx - 1
		rowLayout := layout
		if symIdx >= 3 && statusWords[strings.ToUpper(fields[symIdx])] {
			row.Status = normStatus(fields[symIdx])
			symIdx--
			rowLayout = LayoutDebt
		}
		row.Symbol = fields[symIdx]
		row.Name, row.NameMarkers = splitNameMarkers(fields[2:symIdx])

		if dateAmbiguous(row.LiveDate) {
			out.Findings = append(out.Findings, Finding{
				ISIN: row.ISIN, Line: line, CheckName: "live_date_ambiguous_order",
				Severity: "warning",
				Detail:   fmt.Sprintf("numeric date %q is valid as both DD/MM/YYYY and MM/DD/YYYY; raw value preserved, not normalised", row.LiveDate),
			})
		}

		rest := fields[dateIdx+1:]
		// A debt row carries a maturity date in DD-Mon-YY right after the live date.
		if len(rest) > 0 && maturityRe.MatchString(rest[0]) {
			row.MaturityDate = rest[0]
			rest = rest[1:]
			rowLayout = LayoutDebt
		}
		if row.Status == "" && len(rest) > 0 && statusWords[strings.ToUpper(rest[0])] {
			row.Status = normStatus(rest[0])
			rest = rest[1:]
		}
		row.Layout = rowLayout
		if rowLayout == LayoutDebt {
			want = 0
		} else {
			want = layout.NumericFields()
		}

		nums := make([]*float64, 0, len(rest))
		for _, tok := range rest {
			v, ok := parseCell(tok)
			if !ok {
				out.Findings = append(out.Findings, Finding{
					ISIN: row.ISIN, Line: line, CheckName: "row_unparseable_cell",
					Severity: "warning", Detail: fmt.Sprintf("cell %q is not numeric", tok),
				})
				continue
			}
			nums = append(nums, v)
		}

		if rowLayout == LayoutDebt {
			if len(nums) < 1 || len(nums) > 3 {
				out.Findings = append(out.Findings, Finding{
					ISIN: row.ISIN, Line: line, CheckName: "debt_row_numeric_field_count",
					Severity: "error",
					Detail:   fmt.Sprintf("debt layout expects 1-3 numeric cells, row yielded %d", len(nums)),
				})
				continue
			}
			row.SharesInCDS = nums[0]
			if len(nums) >= 2 {
				row.PaidUpIncl = nums[1]
			}
			if len(nums) >= 3 {
				row.PctIncl = nums[2]
			}
			out.Rows = append(out.Rows, row)
			continue
		}
		if want > 0 && len(nums) != want {
			out.Findings = append(out.Findings, Finding{
				ISIN: row.ISIN, Line: line, CheckName: "row_numeric_field_count",
				Severity: "error",
				Detail: fmt.Sprintf("layout %s expects %d numeric cells, row yielded %d",
					layout, want, len(nums)),
			})
			continue
		}
		assignNumerics(&row, layout, nums)
		out.Rows = append(out.Rows, row)
	}
	return out
}

func assignNumerics(row *Row, layout Layout, n []*float64) {
	switch layout {
	case LayoutEquity2025:
		row.SharesInCDS, row.PaidUpIncl, row.PctIncl, row.PaidUpExcl = n[0], n[1], n[2], n[3]
	case LayoutEquity2024:
		row.SharesInCDS, row.MarketValue = n[0], n[1]
		row.PaidUpIncl, row.PctIncl, row.PaidUpExcl, row.PctExcl = n[2], n[3], n[4], n[5]
	case LayoutFunds2025:
		row.SharesInCDS = n[0]
	}
}

// splitRowTokens splits on whitespace, then repairs the observed concatenation
// where a status word runs directly into a following dash cell -- part B emits
// "... 19/Jan/2007 LISTED-" when units are zero, and treating that as one token
// loses the zero and corrupts the field count.
func splitRowTokens(line string) []string {
	fields := strings.Fields(line)
	out := make([]string, 0, len(fields)+2)
	for _, f := range fields {
		if len(f) > 1 && strings.HasSuffix(f, "-") {
			base := strings.TrimSuffix(f, "-")
			if statusWords[strings.ToUpper(base)] {
				out = append(out, base, "-")
				continue
			}
		}
		out = append(out, f)
	}
	return out
}

// splitNameMarkers peels trailing status markers off the security name. CDC
// embeds them IN the name string rather than in a column: a trailing "*",
// "**", "***", or a "- FREEZE" / "(FREEZE)" suffix.
// normStatus folds CDC's spelling variants onto one value. UN-LISTED and
// UNLISTED are the same status and must not split a group-by.
func normStatus(s string) string {
	u := strings.ToUpper(strings.TrimSpace(s))
	u = strings.ReplaceAll(u, "-", "")
	switch u {
	case "UNLISTED":
		return "UNLISTED"
	case "DELISTED":
		return "DELISTED"
	}
	return strings.ToUpper(strings.TrimSpace(s))
}

func splitNameMarkers(toks []string) (string, []string) {
	markers := []string{}
	end := len(toks)
	for end > 0 {
		t := strings.ToUpper(strings.Trim(toks[end-1], "()"))
		switch {
		case markerGlyphRe.MatchString(toks[end-1]):
			markers = append([]string{toks[end-1]}, markers...)
			end--
		case t == "FREEZE" || t == "FROZEN":
			markers = append([]string{strings.Trim(toks[end-1], "()")}, markers...)
			end--
		case t == "-" && len(markers) > 0:
			end--
		default:
			return strings.Join(toks[:end], " "), markers
		}
	}
	return strings.Join(toks[:end], " "), markers
}

// parseCell decodes one numeric cell. A bare "-" is CDC's ZERO, not a missing
// value. Parenthesised negatives are accepted defensively even though a full
// scan of the 2024 vintage found none -- the MUFAP convention inverted a whole
// market when it went undecoded, and the cost of handling it here is one branch.
func parseCell(tok string) (*float64, bool) {
	t := strings.TrimSpace(tok)
	if t == "-" || t == "" {
		z := 0.0
		return &z, true
	}
	if !numRe.MatchString(t) {
		return nil, false
	}
	neg := strings.HasPrefix(t, "(") && strings.HasSuffix(t, ")")
	t = strings.Trim(t, "()")
	t = strings.ReplaceAll(t, ",", "")
	if t == "" || t == "-" {
		z := 0.0
		return &z, true
	}
	v, err := strconv.ParseFloat(t, 64)
	if err != nil {
		return nil, false
	}
	if neg {
		v = -v
	}
	return &v, true
}
