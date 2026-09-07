// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cdcpdf

import "testing"

// All fixtures below are verbatim lines from real CDC reports extracted via
// PDFKit, so a regression here means the parser drifted from the real source.

const hdrEquity2025 = "DETAILS OF ORDINARY, PREFERENCE SHARES & MODARABA CERTIFICATES\n" +
	"S. No. Security Id Security Name Short Name Live Date STATUS Shares Available in\nCDS\n"
const hdrEquity2024 = "DETAILS OF ORDINARY, PREFERENCE SHARES & MODARABA CERTIFICATES\n" +
	"Shares in CDS\nS. No. Security Id Security Name Short\nName\nAvailable Market Value Shares/Units % of Shares\n"
const hdrFunds = "DETAILS OF OPEN END / ETF FUNDS / SAVING CERTIFICATES\n" +
	"S.\nNo. Security Id Security Name Security\nSymbol Live date Status Units Available in\nCDS\n"

func TestDetectLayout(t *testing.T) {
	cases := []struct {
		name string
		text string
		want Layout
	}{
		{"equity 2025 print-driver", hdrEquity2025, LayoutEquity2025},
		{"equity 2024 excel", hdrEquity2024, LayoutEquity2024},
		{"funds part B", hdrFunds, LayoutFunds2025},
		{"unrecognised", "SOMETHING ELSE ENTIRELY\n", LayoutUnknown},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, _ := DetectLayout(c.text)
			if got != c.want {
				t.Fatalf("DetectLayout = %q, want %q", got, c.want)
			}
		})
	}
}

func TestParseEquity2025(t *testing.T) {
	text := hdrEquity2025 +
		"1 PK0000201019 EFU GENERAL INSURANCE LIMITED EFUG 10/Jun/1998 LISTED 193,592,231 200,000,000 96.80 200,000,000\n" +
		"3 PK0000401015 HAYDARI CONSTRUCTION COMPANY LIMITED - FREEZE *** HADC 11/Mar/2004 LISTED 5,882,415 6,400,000 91.91 6,400,000\n" +
		"4 PK0000501012 K-ELECTRIC LIMITED * KEL 8/Jul/1998 LISTED 20,883,763,529 27,615,194,246 75.62 20,888,281,969\n" +
		"29 PK0004201015 WAZIR ALI INDUSTRIES LIMITED WAZIR 23/Aug/1999 UN-LISTED 7,799,000 7,799,000 100.00 7,799,000\n"
	p := ParseRows(text)
	if len(p.Rows) != 4 {
		t.Fatalf("rows = %d, want 4 (findings: %+v)", len(p.Rows), p.Findings)
	}

	r := p.Rows[0]
	if r.ISIN != "PK0000201019" || r.Symbol != "EFUG" || r.Name != "EFU GENERAL INSURANCE LIMITED" {
		t.Errorf("row0 identity wrong: %+v", r)
	}
	if r.Status != "LISTED" || r.LiveDate != "10/Jun/1998" {
		t.Errorf("row0 status/date wrong: %q %q", r.Status, r.LiveDate)
	}
	if r.SharesInCDS == nil || *r.SharesInCDS != 193592231 {
		t.Errorf("row0 shares wrong: %v", r.SharesInCDS)
	}
	if r.PctIncl == nil || *r.PctIncl != 96.80 {
		t.Errorf("row0 pct wrong: %v", r.PctIncl)
	}
	// Market Value does not exist in the 2025 layout and must stay nil rather
	// than being back-filled with a zero.
	if r.MarketValue != nil {
		t.Errorf("row0 MarketValue should be nil in 2025 layout, got %v", *r.MarketValue)
	}

	// FREEZE markers live inside the name string, not a column.
	if got := p.Rows[1]; got.Name != "HAYDARI CONSTRUCTION COMPANY LIMITED" || len(got.NameMarkers) == 0 {
		t.Errorf("row1 name/markers wrong: name=%q markers=%v", got.Name, got.NameMarkers)
	}
	// UN-LISTED must normalise so a group-by does not split.
	if got := p.Rows[3].Status; got != "UNLISTED" {
		t.Errorf("UN-LISTED should normalise to UNLISTED, got %q", got)
	}
	// K-Electric is the GoP case: excl differs from incl.
	kel := p.Rows[2]
	if kel.PaidUpIncl == nil || kel.PaidUpExcl == nil || *kel.PaidUpExcl >= *kel.PaidUpIncl {
		t.Errorf("KEL should have excl < incl: %v %v", kel.PaidUpIncl, kel.PaidUpExcl)
	}
}

func TestParseFundsPartB_StatusDashConcatenation(t *testing.T) {
	// The zero-units case emits "LISTED-" with no separator. Splitting it wrong
	// loses the zero AND corrupts the numeric field count.
	text := hdrFunds +
		"1 PK0081406016 JS LARGE CAP. FUND JSLCFO 20/Sep/2010 LISTED 711,863\n" +
		"3 PK0082106029 PAKISTAN INCOME FUND PAIF 19/Jan/2007 LISTED-\n"
	p := ParseRows(text)
	if len(p.Rows) != 2 {
		t.Fatalf("rows = %d, want 2 (findings %+v)", len(p.Rows), p.Findings)
	}
	if p.Rows[1].Status != "LISTED" {
		t.Errorf("concatenated status not split: %q", p.Rows[1].Status)
	}
	if p.Rows[1].SharesInCDS == nil || *p.Rows[1].SharesInCDS != 0 {
		t.Errorf("dash should decode to ZERO, got %v", p.Rows[1].SharesInCDS)
	}
	// Funds have units, not share capital. Inventing a percentage here would be
	// fabrication.
	if p.Rows[0].PaidUpIncl != nil || p.Rows[0].PctIncl != nil {
		t.Errorf("funds rows must not carry paid-up capital or a percentage")
	}
}

func TestParseDebtLayout_StatusBeforeDateAndMaturity(t *testing.T) {
	// Debt reorders the columns: status precedes the live date, and a maturity
	// date in DD-Mon-YY follows it.
	text := hdrEquity2025 +
		"2 PK0000504461 K-ELECTRIC LIMITED - SUKUK-6 KELSC6 LISTED 24/Mar/2023 22-Nov-29 67,000\n" +
		"3 PK0000504669 K-ELECTRIC LIMITED - SHORT TERM SUKUK-32 (STS-32) KELSTS32 UN-LISTED 10/Jul/2025 12-Dec-25 10,000\n"
	p := ParseRows(text)
	if len(p.Rows) != 2 {
		t.Fatalf("rows = %d, want 2 (findings %+v)", len(p.Rows), p.Findings)
	}
	r := p.Rows[0]
	if r.Layout != LayoutDebt {
		t.Errorf("row should be tagged debt layout, got %q", r.Layout)
	}
	if r.Symbol != "KELSC6" {
		t.Errorf("symbol wrong: %q (status must not be mistaken for the symbol)", r.Symbol)
	}
	if r.Status != "LISTED" || r.LiveDate != "24/Mar/2023" || r.MaturityDate != "22-Nov-29" {
		t.Errorf("debt fields wrong: status=%q live=%q maturity=%q", r.Status, r.LiveDate, r.MaturityDate)
	}
	if p.Rows[1].Status != "UNLISTED" {
		t.Errorf("debt UN-LISTED normalisation failed: %q", p.Rows[1].Status)
	}
}

func TestAmbiguousNumericDateIsFlaggedNotNormalised(t *testing.T) {
	// 13/03/2025 is unambiguous (DD/MM). 10/12/2013 is ambiguous and must be
	// flagged, with the raw string preserved.
	text := hdrEquity2025 +
		"1687 PKAA60001016 ZEBRA CONSULTANTS GLOBAL (PRIVATE) LIMITED ULS-640 13/03/2025 UN-LISTED 1,000 1,000 100.00 1,000\n" +
		"628 PK0106701011 GUL AHMED WIND POWER LIMITED GAWPL 10/12/2013 UN-LISTED 321,798,065 321,798,065 100.00 321,798,065\n"
	p := ParseRows(text)
	if len(p.Rows) != 2 {
		t.Fatalf("rows = %d, want 2 (findings %+v)", len(p.Rows), p.Findings)
	}
	if p.Rows[0].LiveDate != "13/03/2025" || p.Rows[1].LiveDate != "10/12/2013" {
		t.Errorf("raw dates must be preserved verbatim: %q %q", p.Rows[0].LiveDate, p.Rows[1].LiveDate)
	}
	var flagged int
	for _, f := range p.Findings {
		if f.CheckName == "live_date_ambiguous_order" {
			flagged++
		}
	}
	if flagged != 1 {
		t.Errorf("expected exactly 1 ambiguity finding (only 10/12/2013), got %d", flagged)
	}
}

func TestParseCell(t *testing.T) {
	cases := []struct {
		in     string
		want   float64
		wantOK bool
	}{
		{"1,183,294", 1183294, true},
		{"999,999,930.00", 999999930, true},
		{"-", 0, true}, // CDC's zero, NOT missing
		{"", 0, true},
		{"96.80", 96.80, true},
		{"(4.97)", -4.97, true}, // defensive: none found in CDC, but cheap to honour
		{"UN-LISTED", 0, false},
		{"12-Apr-23", 0, false},
	}
	for _, c := range cases {
		got, ok := parseCell(c.in)
		if ok != c.wantOK {
			t.Errorf("parseCell(%q) ok = %v, want %v", c.in, ok, c.wantOK)
			continue
		}
		if ok && *got != c.want {
			t.Errorf("parseCell(%q) = %v, want %v", c.in, *got, c.want)
		}
	}
}

func TestFieldCountMismatchIsAFindingNotASilentDrop(t *testing.T) {
	// A short equity row must produce an explicit finding. Silently coercing it
	// is how a column shift becomes invisible.
	text := hdrEquity2025 + "9 PK0000601010 MURREE BREWERY COMPANY LIMITED MUREB 22/Sep/2000 LISTED 13,265,273\n"
	p := ParseRows(text)
	if len(p.Rows) != 0 {
		t.Fatalf("short row should not be accepted, got %d rows", len(p.Rows))
	}
	if len(p.Findings) == 0 || p.Findings[0].CheckName != "row_numeric_field_count" {
		t.Fatalf("expected a row_numeric_field_count finding, got %+v", p.Findings)
	}
	if p.Findings[0].Severity != "error" {
		t.Errorf("field-count mismatch must be severity error, got %q", p.Findings[0].Severity)
	}
}
