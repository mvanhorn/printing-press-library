// Copyright 2026 ardihanan and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestRetriesElapsed(t *testing.T) {
	// ladder 2,5,10,90,210 -> cumulative 2,7,17,107,317
	tests := []struct {
		age  int
		want int
	}{
		{age: 0, want: 0},
		{age: 1, want: 0},
		{age: 2, want: 1},
		{age: 6, want: 1},
		{age: 7, want: 2},
		{age: 17, want: 3},
		{age: 100, want: 3},
		{age: 107, want: 4},
		{age: 317, want: 5},
		{age: 100000, want: 5},
	}
	for _, tt := range tests {
		if got := retriesElapsed(tt.age, webhookRetryLadder); got != tt.want {
			t.Errorf("retriesElapsed(%d) = %d, want %d", tt.age, got, tt.want)
		}
	}
}

func TestNextRetryIn(t *testing.T) {
	if got := nextRetryIn(0, webhookRetryLadder); got == "" {
		t.Errorf("nextRetryIn(0) should be non-empty")
	}
	if got := nextRetryIn(len(webhookRetryLadder), webhookRetryLadder); got != "" {
		t.Errorf("nextRetryIn(exhausted) = %q, want empty", got)
	}
}

func TestDisbursementIsPending(t *testing.T) {
	for _, s := range []string{"pending", "Processing", "PENDING"} {
		if !disbursementIsPending(s) {
			t.Errorf("disbursementIsPending(%q) = false, want true", s)
		}
	}
	for _, s := range []string{"completed", "failed", ""} {
		if disbursementIsPending(s) {
			t.Errorf("disbursementIsPending(%q) = true, want false", s)
		}
	}
}
