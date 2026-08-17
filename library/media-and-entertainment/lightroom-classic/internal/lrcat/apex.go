// Copyright 2026 Micah Baldwin and contributors. Licensed under Apache-2.0. See LICENSE.
package lrcat

import (
	"fmt"
	"math"
)

// ApertureFromAPEX converts a Lightroom-stored APEX aperture value (Av) to a
// human f-stop string. f-number = 2^(Av/2).
func ApertureFromAPEX(av float64) string {
	f := math.Pow(2, av/2)
	// Round to one decimal, drop trailing .0 for clean stops like f/8.
	r := math.Round(f*10) / 10
	if r == math.Trunc(r) {
		return fmt.Sprintf("f/%.0f", r)
	}
	return fmt.Sprintf("f/%.1f", r)
}

// ShutterFromAPEX converts a Lightroom-stored APEX shutter value (Tv) to a
// human exposure string. seconds = 2^(-Tv).
func ShutterFromAPEX(tv float64) string {
	secs := math.Pow(2, -tv)
	if secs >= 1 {
		r := math.Round(secs*10) / 10
		if r == math.Trunc(r) {
			return fmt.Sprintf("%.0fs", r)
		}
		return fmt.Sprintf("%.1fs", r)
	}
	denom := math.Round(1 / secs)
	return fmt.Sprintf("1/%.0f", denom)
}
