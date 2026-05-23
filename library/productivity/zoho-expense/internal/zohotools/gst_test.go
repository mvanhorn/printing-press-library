package zohotools

import (
	"math"
	"testing"
)

func TestComputeSplit_IntraState18Percent(t *testing.T) {
	got := ComputeSplit(1180.0, 18.0, true)
	if !approxEqual(got.Base, 1000.0) {
		t.Errorf("Base: want 1000.00, got %.2f", got.Base)
	}
	if !approxEqual(got.CGST, 90.0) {
		t.Errorf("CGST: want 90.00, got %.2f", got.CGST)
	}
	if !approxEqual(got.SGST, 90.0) {
		t.Errorf("SGST: want 90.00, got %.2f", got.SGST)
	}
	if got.IGST != 0 {
		t.Errorf("IGST: want 0 for intra-state, got %.2f", got.IGST)
	}
	if !got.IntraState {
		t.Errorf("IntraState: want true")
	}
}

func TestComputeSplit_InterStateIGST(t *testing.T) {
	got := ComputeSplit(1180.0, 18.0, false)
	if !approxEqual(got.Base, 1000.0) {
		t.Errorf("Base: want 1000.00, got %.2f", got.Base)
	}
	if !approxEqual(got.IGST, 180.0) {
		t.Errorf("IGST: want 180.00, got %.2f", got.IGST)
	}
	if got.CGST != 0 || got.SGST != 0 {
		t.Errorf("CGST/SGST: want 0 for inter-state, got %.2f/%.2f", got.CGST, got.SGST)
	}
}

func TestComputeSplit_ZeroTax(t *testing.T) {
	got := ComputeSplit(500.0, 0.0, true)
	if got.Base != 500.0 {
		t.Errorf("Base: want 500.00 with zero tax, got %.2f", got.Base)
	}
	if got.CGST != 0 || got.SGST != 0 || got.IGST != 0 {
		t.Errorf("All tax components should be 0; got CGST=%.2f SGST=%.2f IGST=%.2f", got.CGST, got.SGST, got.IGST)
	}
}

func TestComputeSplit_ZeroTotal(t *testing.T) {
	got := ComputeSplit(0.0, 18.0, true)
	if got.Base != 0 || got.CGST != 0 || got.SGST != 0 {
		t.Errorf("All components should be 0 for zero total; got Base=%.2f CGST=%.2f SGST=%.2f", got.Base, got.CGST, got.SGST)
	}
}

func TestComputeSplit_5PercentIntraState(t *testing.T) {
	// GST 5% intra-state on a 525 INR total → 500 base, 12.50 CGST + 12.50 SGST
	got := ComputeSplit(525.0, 5.0, true)
	if !approxEqual(got.Base, 500.0) {
		t.Errorf("Base: want 500.00, got %.2f", got.Base)
	}
	if !approxEqual(got.CGST, 12.5) {
		t.Errorf("CGST: want 12.50, got %.2f", got.CGST)
	}
	if !approxEqual(got.SGST, 12.5) {
		t.Errorf("SGST: want 12.50, got %.2f", got.SGST)
	}
}

// Paise-invariant: Base + CGST + SGST must equal Total for intra-state and
// Base + IGST for inter-state. ₹100 @ 18% is the canonical regression from
// Greptile P1 finding (naive halves drifted 1 paise to 100.01).
func TestComputeSplit_SumEqualsTotal(t *testing.T) {
	cases := []struct {
		name  string
		total float64
		pct   float64
		intra bool
	}{
		{"100 @ 18% intra", 100.00, 18.0, true},
		{"99.99 @ 12% intra", 99.99, 12.0, true},
		{"1234.56 @ 5% intra", 1234.56, 5.0, true},
		{"1180 @ 18% intra", 1180.00, 18.0, true},
		{"100 @ 18% inter", 100.00, 18.0, false},
		{"4999 @ 28% inter", 4999.00, 28.0, false},
	}
	for _, c := range cases {
		got := ComputeSplit(c.total, c.pct, c.intra)
		var sum float64
		if c.intra {
			sum = got.Base + got.CGST + got.SGST
		} else {
			sum = got.Base + got.IGST
		}
		if !approxEqual(sum, got.Total) {
			t.Errorf("%s: sum=%.4f != total=%.2f (Base=%.2f CGST=%.2f SGST=%.2f IGST=%.2f)",
				c.name, sum, got.Total, got.Base, got.CGST, got.SGST, got.IGST)
		}
	}
}

func TestRound2(t *testing.T) {
	cases := []struct{ in, want float64 }{
		{123.456, 123.46},
		{0.005, 0.01}, // rounds half away from zero per math.Round
		{99.991, 99.99},
	}
	for _, c := range cases {
		got := round2(c.in)
		if !approxEqual(got, c.want) {
			t.Errorf("round2(%v): want %.2f, got %.2f", c.in, c.want, got)
		}
	}
}

func approxEqual(a, b float64) bool {
	return math.Abs(a-b) < 0.005
}
