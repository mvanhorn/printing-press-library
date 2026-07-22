// internal/domain/result_test.go
package domain

import "testing"

func TestResultZeroValueOmitsEmpty(t *testing.T) {
	r := Result{Provider: "nyrr", Runner: "Sample Runner", Bib: "1234"}
	if r.OverallPlace != 0 {
		t.Fatalf("expected zero OverallPlace, got %d", r.OverallPlace)
	}
	if len(r.Splits) != 0 {
		t.Fatalf("expected no splits, got %d", len(r.Splits))
	}
}

func TestEventCarriesProviderRouting(t *testing.T) {
	e := Event{Provider: "mika", ID: "berlin-2025", Year: 2025}
	if e.Provider != "mika" || e.Year != 2025 {
		t.Fatal("event fields not retained")
	}
}

func TestAthleteAndResultExtras(t *testing.T) {
	a := Athlete{Provider: "athlinks", ID: "338681853", Name: "Sample Athlete", City: "amadora", Age: 41}
	if a.ID != "338681853" || a.Age != 41 {
		t.Fatal("athlete fields not retained")
	}
	r := Result{Date: "2024-05-01", Distance: "Marathon"}
	if r.Date != "2024-05-01" || r.Distance != "Marathon" {
		t.Fatal("result extras not retained")
	}
}
