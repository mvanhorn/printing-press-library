package sync

import (
	"errors"
	"testing"
)

func TestParseBudgetAndGateCutoff(t *testing.T) {
	budget, ok, err := ParseBudget("api-usage=81000/100000")
	if err != nil {
		t.Fatalf("parse budget: %v", err)
	}
	if !ok {
		t.Fatalf("parse budget ok = false, want true")
	}
	if budget.Used != 81000 || budget.Limit != 100000 {
		t.Fatalf("budget = %+v, want 81000/100000", budget)
	}
	if got := budget.Utilization(); got != 0.81 {
		t.Fatalf("utilization = %.2f, want 0.81", got)
	}
	gate := NewGate()
	if err := gate.UpdateFromHeader("api-usage=79999/100000"); err != nil {
		t.Fatalf("update header: %v", err)
	}
	if err := gate.Check(); err != nil {
		t.Fatalf("check below cutoff: %v", err)
	}
	if err := gate.UpdateFromHeader("api-usage=80000/100000"); err != nil {
		t.Fatalf("update header: %v", err)
	}
	if err := gate.Check(); !errors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("check at cutoff = %v, want ErrBudgetExceeded", err)
	}
}

func TestParseBudgetIgnoresUnrelatedHeader(t *testing.T) {
	_, ok, err := ParseBudget("per-app-api-usage=1/10")
	if err != nil {
		t.Fatalf("parse unrelated header: %v", err)
	}
	if ok {
		t.Fatalf("ok = true, want false")
	}
}
