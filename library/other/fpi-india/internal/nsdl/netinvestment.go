package nsdl

import "strings"

// NetInvestmentRow is one period's FPI net-investment flow, parsed from
// NSDL's Yearwise.aspx GridView. Column order is fixed and confirmed against
// the live page markup (id="rpt"): Period | Equity | Debt-General Limit |
// Debt-VRR | Debt-FAR | Hybrid | MF-Equity | MF-Debt | MF-Hybrid |
// MF-SolutionOriented | MF-Other | AIF | Total | Cumulative.
type NetInvestmentRow struct {
	Period             string  `json:"period"`
	PeriodType         string  `json:"period_type"` // fy | cy | cy_monthly | quarterly | latest
	Currency           string  `json:"currency"`    // INR | USD
	Equity             float64 `json:"equity"`
	DebtGeneralLimit   float64 `json:"debt_general_limit"`
	DebtVRR            float64 `json:"debt_vrr"`
	DebtFAR            float64 `json:"debt_far"`
	Debt               float64 `json:"debt"` // sum of general + VRR + FAR
	Hybrid             float64 `json:"hybrid"`
	MFEquity           float64 `json:"mf_equity"`
	MFDebt             float64 `json:"mf_debt"`
	MFHybrid           float64 `json:"mf_hybrid"`
	MFSolutionOriented float64 `json:"mf_solution_oriented"`
	MFOther            float64 `json:"mf_other"`
	MutualFunds        float64 `json:"mutual_funds"` // sum of the five MF sub-columns
	AIF                float64 `json:"aif"`
	Total              float64 `json:"total"`
	Cumulative         float64 `json:"cumulative_total"`
}

// ParseNetInvestment parses the Yearwise.aspx GridView (id="rpt") into rows.
// Works for FY (RptType=5), CY (RptType=6), and CY-with-year (monthly
// breakdown) — all three render the identical 14-column layout, differing
// only in period label shape (financial year, calendar year, or month name).
func ParseNetInvestment(body []byte, periodType, currency string) ([]NetInvestmentRow, error) {
	t, err := ExtractTableByID(body, "rpt")
	if err != nil {
		return nil, err
	}
	var out []NetInvestmentRow
	for _, row := range t.Rows {
		if len(row) < 14 {
			continue
		}
		period := strings.TrimSpace(row[0])
		if period == "" {
			continue
		}
		// NSDL appends a lifetime "Total" row after the last real period —
		// a 12-value summary (one column short of the normal 13, so it
		// lacks a genuine cumulative figure), not an actual financial/
		// calendar year. Treating it as a period corrupts every derived
		// command (extremes/streaks/trend/yoy all see a fake "period" with
		// an inflated aggregate value). Skip it outright.
		if strings.EqualFold(period, "Total") || strings.Contains(strings.ToLower(period), "grand total") {
			continue
		}
		// The current in-progress year is suffixed "**" with a footnote
		// ("** up to <date>") elsewhere on the page — real, but the label
		// needs cleaning so period matching (e.g. --year lookups) and
		// display stay consistent with completed-year labels.
		period = strings.TrimSpace(strings.TrimSuffix(period, "**"))
		row[0] = period
		debtGeneral, _ := ParseNumber(row[2])
		debtVRR, _ := ParseNumber(row[3])
		debtFAR, _ := ParseNumber(row[4])
		mfEquity, _ := ParseNumber(row[6])
		mfDebt, _ := ParseNumber(row[7])
		mfHybrid, _ := ParseNumber(row[8])
		mfSolution, _ := ParseNumber(row[9])
		mfOther, _ := ParseNumber(row[10])
		equity, _ := ParseNumber(row[1])
		hybrid, _ := ParseNumber(row[5])
		aif, _ := ParseNumber(row[11])
		total, _ := ParseNumber(row[12])
		cumulative, _ := ParseNumber(row[13])
		out = append(out, NetInvestmentRow{
			Period:             row[0],
			PeriodType:         periodType,
			Currency:           currency,
			Equity:             equity,
			DebtGeneralLimit:   debtGeneral,
			DebtVRR:            debtVRR,
			DebtFAR:            debtFAR,
			Debt:               debtGeneral + debtVRR + debtFAR,
			Hybrid:             hybrid,
			MFEquity:           mfEquity,
			MFDebt:             mfDebt,
			MFHybrid:           mfHybrid,
			MFSolutionOriented: mfSolution,
			MFOther:            mfOther,
			MutualFunds:        mfEquity + mfDebt + mfHybrid + mfSolution + mfOther,
			AIF:                aif,
			Total:              total,
			Cumulative:         cumulative,
		})
	}
	return out, nil
}

// AssetValue returns the figure for a named asset class from a row, used by
// commands filtering with --asset equity|debt|hybrid|mutual_funds|aif|total.
func (r NetInvestmentRow) AssetValue(asset string) (float64, bool) {
	switch asset {
	case "equity":
		return r.Equity, true
	case "debt":
		return r.Debt, true
	case "hybrid":
		return r.Hybrid, true
	case "mutual_funds", "mf":
		return r.MutualFunds, true
	case "aif":
		return r.AIF, true
	case "total", "":
		return r.Total, true
	default:
		return 0, false
	}
}
