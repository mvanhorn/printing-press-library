package agent

import (
	"testing"
	"time"
)

// TestDecayFresh confirms a fresh account lands near 100.
func TestDecayFresh(t *testing.T) {
	now := time.Now().UTC()
	yesterday := now.Add(-24 * time.Hour)
	in := DecayInput{
		Account:            Account{ID: "001FRESH", Name: "Fresh Co"},
		LastActivityDate:   &yesterday,
		LastOppStageChange: &yesterday,
		LastCaseDate:       &yesterday,
		LastChatterDate:    &yesterday,
		OpenOppCount:       2,
		OpenCaseCount:      0,
	}
	result := ScoreDecay(in)
	if result.Score < 95 {
		t.Fatalf("expected score >= 95 for fresh account, got %d", result.Score)
	}
	if len(result.Signals) != 4 {
		t.Fatalf("expected 4 signals, got %d", len(result.Signals))
	}
	for _, s := range result.Signals {
		if s.Severity != "ok" {
			t.Fatalf("expected all signals ok, got %+v", s)
		}
	}
}

// TestDecayStale confirms no activity in 90 days drops the score meaningfully.
func TestDecayStale(t *testing.T) {
	now := time.Now().UTC()
	ninetyDaysAgo := now.Add(-91 * 24 * time.Hour)
	in := DecayInput{
		Account:            Account{ID: "001STALE", Name: "Stale Co"},
		LastActivityDate:   &ninetyDaysAgo,
		LastOppStageChange: &ninetyDaysAgo,
		LastCaseDate:       &ninetyDaysAgo,
		LastChatterDate:    &ninetyDaysAgo,
	}
	result := ScoreDecay(in)
	if result.Score > 50 {
		t.Fatalf("expected score <= 50 for 90-day-stale account, got %d", result.Score)
	}
	critical := 0
	for _, s := range result.Signals {
		if s.Severity == "critical" {
			critical++
		}
	}
	if critical < 2 {
		t.Fatalf("expected >= 2 critical signals, got %d", critical)
	}
}

// TestDecayNoData confirms missing data is scored as maximum penalty.
func TestDecayNoData(t *testing.T) {
	in := DecayInput{Account: Account{ID: "001NONE"}}
	result := ScoreDecay(in)
	if result.Score > 20 {
		t.Fatalf("expected severe penalty for no-data account, got %d", result.Score)
	}
}
