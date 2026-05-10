// Copyright 2026 dawid-piaskowski. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"math"
	"testing"
)

func round2(v float64) float64 { return math.Round(v*100) / 100 }

func TestCalcSozialplan_BaseFormula(t *testing.T) {
	// 8 years × 4500 € × 0.75 = 27000 €
	r := calcSozialplan(sozialplanInput{MonthlySalary: 4500, YearsService: 8, Factor: 0.75}, 0)
	if r.BaseAmount != 27000 {
		t.Errorf("base: want 27000, got %.2f", r.BaseAmount)
	}
	if r.FinalAmount != 27000 {
		t.Errorf("final (no adjustments): want 27000, got %.2f", r.FinalAmount)
	}
}

func TestCalcSozialplan_DisabilityBonus(t *testing.T) {
	// base = 10000, +25% = 2500 → 12500
	r := calcSozialplan(sozialplanInput{MonthlySalary: 5000, YearsService: 2, Factor: 1.0, Disabled: true}, 0)
	if r.BaseAmount != 10000 {
		t.Errorf("base: want 10000, got %.2f", r.BaseAmount)
	}
	if r.FinalAmount != 12500 {
		t.Errorf("final with disability: want 12500, got %.2f", r.FinalAmount)
	}
	if len(r.Adjustments) != 1 {
		t.Errorf("expected 1 adjustment, got %d", len(r.Adjustments))
	}
}

func TestCalcSozialplan_ChildrenBonus(t *testing.T) {
	// base = 10000, 2 children × 10% = +2000 → 12000
	r := calcSozialplan(sozialplanInput{MonthlySalary: 5000, YearsService: 2, Factor: 1.0, Children: 2}, 0)
	if r.FinalAmount != 12000 {
		t.Errorf("final with 2 children: want 12000, got %.2f", r.FinalAmount)
	}
}

func TestCalcSozialplan_ChildrenCappedAt3(t *testing.T) {
	// 5 children should be treated as 3 → base × 30%
	r3 := calcSozialplan(sozialplanInput{MonthlySalary: 5000, YearsService: 2, Factor: 1.0, Children: 3}, 0)
	r5 := calcSozialplan(sozialplanInput{MonthlySalary: 5000, YearsService: 2, Factor: 1.0, Children: 5}, 0)
	if r3.FinalAmount != r5.FinalAmount {
		t.Errorf("children cap: 3-child result %.2f != 5-child result %.2f", r3.FinalAmount, r5.FinalAmount)
	}
}

func TestCalcSozialplan_AgeBonus(t *testing.T) {
	// base = 10000, age ≥55 → +5% = +500 → 10500
	r := calcSozialplan(sozialplanInput{MonthlySalary: 5000, YearsService: 2, Factor: 1.0, Age: 55}, 0)
	if r.FinalAmount != 10500 {
		t.Errorf("final with age 55: want 10500, got %.2f", r.FinalAmount)
	}
	// age 54 should not trigger bonus
	rNo := calcSozialplan(sozialplanInput{MonthlySalary: 5000, YearsService: 2, Factor: 1.0, Age: 54}, 0)
	if rNo.FinalAmount != 10000 {
		t.Errorf("final with age 54: want 10000, got %.2f", rNo.FinalAmount)
	}
}

func TestCalcSozialplan_AllAdjustmentsCombined(t *testing.T) {
	// base = 5 × 4000 × 1.0 = 20000
	// +25% disability = +5000
	// +10% × 2 children = +4000
	// +5% age ≥55 = +1000
	// total = 30000
	r := calcSozialplan(sozialplanInput{
		MonthlySalary: 4000, YearsService: 5, Factor: 1.0,
		Disabled: true, Children: 2, Age: 60,
	}, 0)
	if r.FinalAmount != 30000 {
		t.Errorf("combined adjustments: want 30000, got %.2f", r.FinalAmount)
	}
	if len(r.Adjustments) != 3 {
		t.Errorf("expected 3 adjustments, got %d", len(r.Adjustments))
	}
}

func TestCalcSozialplan_MaxCapApplied(t *testing.T) {
	// base = 20000, cap at 15000
	r := calcSozialplan(sozialplanInput{MonthlySalary: 4000, YearsService: 5, Factor: 1.0}, 15000)
	if r.FinalAmount != 15000 {
		t.Errorf("capped: want 15000, got %.2f", r.FinalAmount)
	}
	if r.MaxCap != 15000 {
		t.Errorf("MaxCap field: want 15000, got %.2f", r.MaxCap)
	}
}

func TestCalcSozialplan_MaxCapNotAppliedWhenBelowCap(t *testing.T) {
	// final < cap → cap not applied
	r := calcSozialplan(sozialplanInput{MonthlySalary: 4000, YearsService: 5, Factor: 1.0}, 100000)
	if r.FinalAmount != 20000 {
		t.Errorf("below cap: want 20000, got %.2f", r.FinalAmount)
	}
	if r.MaxCap != 0 {
		t.Errorf("MaxCap should be 0 when not applied, got %.2f", r.MaxCap)
	}
}

func TestCalcSozialplan_FractionalYears(t *testing.T) {
	// 2.5 years × 4000 × 1.0 = 10000
	r := calcSozialplan(sozialplanInput{MonthlySalary: 4000, YearsService: 2.5, Factor: 1.0}, 0)
	if r.BaseAmount != 10000 {
		t.Errorf("fractional years: want 10000, got %.2f", r.BaseAmount)
	}
}
