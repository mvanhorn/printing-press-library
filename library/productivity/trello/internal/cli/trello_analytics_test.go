// Hand-authored tests for the shared Trello analytics helpers.

package cli

import (
	"testing"
	"time"
)

func TestParseTrelloTime(t *testing.T) {
	cases := []struct {
		in    string
		wantOK bool
	}{
		{"2026-06-02T19:58:50Z", true},
		{"2026-06-02", true},
		{"", false},
		{"not-a-date", false},
	}
	for _, tc := range cases {
		_, ok := parseTrelloTime(tc.in)
		if ok != tc.wantOK {
			t.Fatalf("parseTrelloTime(%q) ok=%v, want %v", tc.in, ok, tc.wantOK)
		}
	}
}

func TestWeekStart(t *testing.T) {
	// Wednesday 2026-06-03 should roll back to Monday 2026-06-01.
	wed := time.Date(2026, 6, 3, 12, 0, 0, 0, time.UTC)
	got := weekStart(wed)
	want := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("weekStart(%v) = %v, want %v", wed, got, want)
	}
	// Monday stays Monday.
	mon := time.Date(2026, 6, 1, 9, 0, 0, 0, time.UTC)
	if got := weekStart(mon); !got.Equal(want) {
		t.Fatalf("weekStart(monday) = %v, want %v", got, want)
	}
}

func TestPercentile(t *testing.T) {
	xs := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	if got := percentile(xs, 0.5); got < 5 || got > 6 {
		t.Fatalf("median = %v, want ~5.5", got)
	}
	if got := percentile(xs, 0.9); got < 9 || got > 9.2 {
		t.Fatalf("p90 = %v, want ~9.1", got)
	}
	if got := percentile(nil, 0.5); got != 0 {
		t.Fatalf("percentile(nil) = %v, want 0", got)
	}
	if got := percentile([]float64{42}, 0.9); got != 42 {
		t.Fatalf("percentile(single) = %v, want 42", got)
	}
}

func TestMean(t *testing.T) {
	if got := mean([]float64{2, 4, 6}); got != 4 {
		t.Fatalf("mean = %v, want 4", got)
	}
	if got := mean(nil); got != 0 {
		t.Fatalf("mean(nil) = %v, want 0", got)
	}
}

func TestIsCompletionAction(t *testing.T) {
	mk := func(after string) trelloAction {
		var a trelloAction
		a.Type = "updateCard"
		a.Data.ListAfter.Name = after
		return a
	}
	if !isCompletionAction(mk("Done")) {
		t.Fatal("Done should be a completion")
	}
	if !isCompletionAction(mk("Shipped to prod")) {
		t.Fatal("Shipped should be a completion")
	}
	if isCompletionAction(mk("In Progress")) {
		t.Fatal("In Progress is not a completion")
	}
}
