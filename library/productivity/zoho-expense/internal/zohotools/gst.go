package zohotools

import "math"

// GSTSplit is the India-specific tax breakdown for a single expense.
// IntraState=true means CGST+SGST split; false means single IGST.
type GSTSplit struct {
	ExpenseID    string  `json:"expense_id"`
	MerchantName string  `json:"merchant_name,omitempty"`
	ExpenseDate  string  `json:"expense_date,omitempty"`
	Base         float64 `json:"base_amount"`
	CGST         float64 `json:"cgst"`
	SGST         float64 `json:"sgst"`
	IGST         float64 `json:"igst"`
	Total        float64 `json:"total"`
	TaxPct       float64 `json:"tax_percentage"`
	IntraState   bool    `json:"intra_state"`
}

// ComputeSplit derives the base + tax components from an inclusive total.
// Uses the standard Indian GST inclusive formula:
//
//	base = total / (1 + tax_pct/100)
//	tax  = total - base
//
// Splits the tax across CGST/SGST when intraState, otherwise IGST.
// Returned values are rounded to 2 decimals (paise resolution).
func ComputeSplit(total, taxPercentage float64, intraState bool) GSTSplit {
	out := GSTSplit{
		Total:      round2(total),
		TaxPct:     taxPercentage,
		IntraState: intraState,
	}
	if taxPercentage <= 0 || total <= 0 {
		out.Base = round2(total)
		return out
	}
	base := total / (1 + taxPercentage/100)
	tax := total - base
	out.Base = round2(base)
	if intraState {
		half := tax / 2
		out.CGST = round2(half)
		out.SGST = round2(half)
	} else {
		out.IGST = round2(tax)
	}
	return out
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
