// Package portstats provides pure-logic statistical helpers for portfolio
// analysis: Pearson correlation, daily-returns conversion, simple
// summary stats. Lives outside the cli package so the math is unit-testable
// without a Cobra/SQLite harness.
package portstats

import (
	"errors"
	"math"
)

// ErrLength is returned when two paired series differ in length.
var ErrLength = errors.New("portstats: input slices must have equal length")

// ErrTooShort is returned when fewer than two paired observations are
// available — Pearson is undefined on a single point.
var ErrTooShort = errors.New("portstats: need at least 2 paired observations")

// Pearson returns the Pearson correlation coefficient between a and b.
// Values lie in [-1, 1]. Returns ErrLength if the slices differ in length
// and ErrTooShort if there are fewer than two observations. When either
// series has zero variance (all values identical), Pearson returns 0 — the
// mathematical limit is undefined but 0 is the conventional "no signal"
// substitute and prevents callers from receiving NaN that would poison
// downstream sorts.
func Pearson(a, b []float64) (float64, error) {
	if len(a) != len(b) {
		return 0, ErrLength
	}
	n := len(a)
	if n < 2 {
		return 0, ErrTooShort
	}
	var sumA, sumB float64
	for i := 0; i < n; i++ {
		sumA += a[i]
		sumB += b[i]
	}
	meanA := sumA / float64(n)
	meanB := sumB / float64(n)

	var num, denomA, denomB float64
	for i := 0; i < n; i++ {
		da := a[i] - meanA
		db := b[i] - meanB
		num += da * db
		denomA += da * da
		denomB += db * db
	}
	if denomA == 0 || denomB == 0 {
		return 0, nil
	}
	r := num / math.Sqrt(denomA*denomB)
	// Numerical guard: cap inside [-1, 1] in case of floating-point drift.
	if r > 1 {
		r = 1
	} else if r < -1 {
		r = -1
	}
	return r, nil
}

// DailyReturns converts a slice of closing prices into a slice of
// (today-yesterday)/yesterday returns. Length of the result is len(prices)-1.
// Zero or negative previous-day prices are skipped (return is 0 for that
// pair) so a malformed feed cannot crash callers; the gap in length
// equality between two series is preserved by the caller using
// PairedReturns when correlating.
func DailyReturns(prices []float64) []float64 {
	if len(prices) < 2 {
		return nil
	}
	out := make([]float64, 0, len(prices)-1)
	for i := 1; i < len(prices); i++ {
		prev := prices[i-1]
		if prev <= 0 {
			out = append(out, 0)
			continue
		}
		out = append(out, (prices[i]-prev)/prev)
	}
	return out
}

// PairedReturns converts two equal-length price series to their daily
// returns. The shorter input wins on length-mismatch (truncated to the
// minimum), to keep downstream Pearson happy without surfacing an error
// for benign feed gaps.
func PairedReturns(a, b []float64) ([]float64, []float64) {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	if n < 2 {
		return nil, nil
	}
	ra := DailyReturns(a[:n])
	rb := DailyReturns(b[:n])
	return ra, rb
}
