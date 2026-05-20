// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

package valuation

import (
	"math"
	"testing"
)

// roundCents is a small helper for tests. Lives in _test.go so it is
// excluded from the production binary.
func roundCents(v float64) float64 {
	return math.Round(v*100) / 100
}

func TestEffectiveCPP_FCOSEAExample(t *testing.T) {
	// FCO->SEA Aug 30: cash $1766.23, award 30000 + $64.53. cpp ~= 5.6723.
	got := EffectiveCPP(1766.23, 64.53, 30000)
	want := 5.6723333333333335
	if math.Abs(got-want) > 0.001 {
		t.Errorf("EffectiveCPP = %v; want ~%v", got, want)
	}
}

func TestEffectiveCPP_ZeroMiles(t *testing.T) {
	if got := EffectiveCPP(1000, 50, 0); got != 0 {
		t.Errorf("EffectiveCPP with 0 miles = %v; want 0", got)
	}
}

func TestMultiple_AboveBaseline(t *testing.T) {
	got := Multiple(5.6723, 1.4)
	want := 4.0516
	if math.Abs(got-want) > 0.001 {
		t.Errorf("Multiple = %v; want ~%v", got, want)
	}
}

func TestMultiple_ZeroBaseline(t *testing.T) {
	if got := Multiple(5.0, 0); got != 0 {
		t.Errorf("Multiple with 0 baseline = %v; want 0", got)
	}
}

func TestTPGValuedUSD_FCOSEAExample(t *testing.T) {
	got := TPGValuedUSD(30000, 1.4, 64.53)
	want := 484.53
	if math.Abs(got-want) > 0.01 {
		t.Errorf("TPGValuedUSD = %v; want %v", got, want)
	}
}

func TestTPGValuedUSD_ZeroMiles(t *testing.T) {
	// When miles is 0, the points option is effectively just taxes.
	if got := TPGValuedUSD(0, 1.4, 64.53); got != 64.53 {
		t.Errorf("TPGValuedUSD with 0 miles = %v; want 64.53", got)
	}
}

func TestCompare_FCOSEAExample(t *testing.T) {
	c := Compare(1766.23, 30000, 64.53, 1.4)
	if c.CashUSD != 1766.23 {
		t.Errorf("CashUSD = %v; want 1766.23", c.CashUSD)
	}
	if c.Miles != 30000 {
		t.Errorf("Miles = %v; want 30000", c.Miles)
	}
	if c.TaxesUSD != 64.53 {
		t.Errorf("TaxesUSD = %v; want 64.53", c.TaxesUSD)
	}
	if c.BaselineCPPCents != 1.4 {
		t.Errorf("BaselineCPPCents = %v; want 1.4", c.BaselineCPPCents)
	}
	if math.Abs(c.CashSavedUSD-1701.70) > 0.01 {
		t.Errorf("CashSavedUSD = %v; want ~1701.70", c.CashSavedUSD)
	}
	if math.Abs(c.EffectiveCPPCents-5.6723) > 0.001 {
		t.Errorf("EffectiveCPPCents = %v; want ~5.6723", c.EffectiveCPPCents)
	}
	if math.Abs(c.Multiple-4.05) > 0.01 {
		t.Errorf("Multiple = %v; want ~4.05", c.Multiple)
	}
	if math.Abs(c.TPGValuedUSD-484.53) > 0.01 {
		t.Errorf("TPGValuedUSD = %v; want ~484.53", c.TPGValuedUSD)
	}
}

func TestRoundCents(t *testing.T) {
	if got := roundCents(1.234); got != 1.23 {
		t.Errorf("roundCents(1.234) = %v; want 1.23", got)
	}
	if got := roundCents(1.235); got != 1.24 {
		t.Errorf("roundCents(1.235) = %v; want 1.24", got)
	}
}
