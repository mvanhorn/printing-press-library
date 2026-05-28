// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

// PATCH(amend-2026-05-20: value-compare) — Pure comparison math.
// Separated for unit-testability. No I/O, no time, no globals.

package valuation

// Comparison is the structured cash-vs-points outcome the CLI surfaces
// in its results envelope.
type Comparison struct {
	CashUSD           float64 `json:"cash_usd"`
	Miles             int     `json:"miles"`
	TaxesUSD          float64 `json:"taxes_usd"`
	BaselineCPPCents  float64 `json:"baseline_cpp_cents"`
	CashSavedUSD      float64 `json:"cash_saved_usd"`
	EffectiveCPPCents float64 `json:"effective_cpp_cents"`
	Multiple          float64 `json:"multiple"`
	TPGValuedUSD      float64 `json:"tpg_valued_usd"`
}

// EffectiveCPP returns the cents-per-point you get from a redemption.
// Returns 0 when miles is zero (avoids divide-by-zero on degenerate
// inputs).
func EffectiveCPP(cashUSD, taxesUSD float64, miles int) float64 {
	if miles <= 0 {
		return 0
	}
	saved := cashUSD - taxesUSD
	return (saved * 100) / float64(miles)
}

// Multiple returns effectiveCPP / baselineCPP. Returns 0 when baseline
// is zero.
func Multiple(effectiveCPP, baselineCPP float64) float64 {
	if baselineCPP <= 0 {
		return 0
	}
	return effectiveCPP / baselineCPP
}

// TPGValuedUSD returns the apples-to-apples dollar cost of paying with
// points at the baseline cents-per-point valuation.
func TPGValuedUSD(miles int, baselineCPP, taxesUSD float64) float64 {
	if miles <= 0 || baselineCPP <= 0 {
		return taxesUSD
	}
	return (float64(miles)*baselineCPP)/100 + taxesUSD
}

// Compare assembles all of the math into one struct. Caller passes
// the cash + award + baseline; Compare rounds nothing — the CLI
// renderer is in charge of display precision.
func Compare(cashUSD float64, miles int, taxesUSD, baselineCPP float64) Comparison {
	eff := EffectiveCPP(cashUSD, taxesUSD, miles)
	return Comparison{
		CashUSD:           cashUSD,
		Miles:             miles,
		TaxesUSD:          taxesUSD,
		BaselineCPPCents:  baselineCPP,
		CashSavedUSD:      cashUSD - taxesUSD,
		EffectiveCPPCents: eff,
		Multiple:          Multiple(eff, baselineCPP),
		TPGValuedUSD:      TPGValuedUSD(miles, baselineCPP, taxesUSD),
	}
}
