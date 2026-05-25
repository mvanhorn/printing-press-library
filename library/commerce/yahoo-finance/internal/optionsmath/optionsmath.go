// Package optionsmath provides pure-logic helpers for options pricing math.
// Kept separate from the cli package so the formulas are unit-testable without
// a Cobra/SQLite harness.
package optionsmath

import "math"

// AnnualizedYield returns the annualized premium yield of a covered call:
//
//	(callBid / spot) * (365 / dte)
//
// callBid is the option's bid premium (per share), spot is the underlying's
// current price (per share), dte is days-to-expiration. Yield is expressed
// as a decimal — 0.08 means 8% annualized.
//
// Returns 0 when spot <= 0 or dte <= 0; negative inputs are clamped to 0 so
// callers never see negative or NaN yields when the input data is missing
// or malformed.
func AnnualizedYield(callBid, spot float64, dte int) float64 {
	if spot <= 0 || dte <= 0 {
		return 0
	}
	if callBid <= 0 {
		return 0
	}
	premiumYield := callBid / spot
	annualFactor := 365.0 / float64(dte)
	return premiumYield * annualFactor
}

// BreakevenPrice returns the breakeven price for a covered call: spot
// minus the credit received. The seller breaks even at this price by
// expiration; below it the position loses money net of premium.
func BreakevenPrice(spot, callBid float64) float64 {
	if math.IsNaN(spot) || math.IsNaN(callBid) {
		return 0
	}
	return spot - callBid
}

// AnnualizedYieldNetOfStrike returns the annualized yield only on the
// premium captured up to the strike, ignoring upside above the strike.
// Useful when the holder wants to model called-away returns.
func AnnualizedYieldNetOfStrike(callBid, spot, strike float64, dte int) float64 {
	if strike < spot {
		// ITM call: premium is mostly intrinsic; treat as zero net yield.
		return 0
	}
	return AnnualizedYield(callBid, spot, dte)
}
