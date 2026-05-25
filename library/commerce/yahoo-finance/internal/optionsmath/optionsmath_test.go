package optionsmath

import (
	"math"
	"testing"
)

func TestAnnualizedYield(t *testing.T) {
	cases := []struct {
		name   string
		bid    float64
		spot   float64
		dte    int
		expect float64
	}{
		{"AAPL 45d ATM", 4.50, 175.0, 45, 0.20856},
		{"shallow OTM", 1.00, 100.0, 30, 0.12166},
		{"deep ITM", 10.0, 50.0, 60, 1.21666},
		{"zero spot", 4.50, 0, 45, 0},
		{"zero dte", 4.50, 175.0, 0, 0},
		{"negative bid", -1, 175.0, 45, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := AnnualizedYield(c.bid, c.spot, c.dte)
			if math.Abs(got-c.expect) > 0.0005 {
				t.Errorf("AnnualizedYield(%v,%v,%v) = %.5f, want %.5f", c.bid, c.spot, c.dte, got, c.expect)
			}
		})
	}
}

func TestBreakevenPrice(t *testing.T) {
	cases := []struct {
		name   string
		spot   float64
		bid    float64
		expect float64
	}{
		{"typical", 100, 4, 96},
		{"no credit", 100, 0, 100},
		{"large credit", 50, 10, 40},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := BreakevenPrice(c.spot, c.bid)
			if math.Abs(got-c.expect) > 1e-9 {
				t.Errorf("BreakevenPrice(%v,%v) = %v, want %v", c.spot, c.bid, got, c.expect)
			}
		})
	}
}

func TestAnnualizedYieldNetOfStrike(t *testing.T) {
	cases := []struct {
		name   string
		bid    float64
		spot   float64
		strike float64
		dte    int
		expect float64
	}{
		{"OTM call passes", 4.50, 175, 180, 45, 0.20856},
		{"ITM call zeroed", 10, 50, 40, 30, 0},
		{"ATM call passes", 1, 100, 100, 30, 0.12166},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := AnnualizedYieldNetOfStrike(c.bid, c.spot, c.strike, c.dte)
			if math.Abs(got-c.expect) > 0.001 {
				t.Errorf("AnnualizedYieldNetOfStrike(%v,%v,%v,%v) = %.5f, want %.5f",
					c.bid, c.spot, c.strike, c.dte, got, c.expect)
			}
		})
	}
}
