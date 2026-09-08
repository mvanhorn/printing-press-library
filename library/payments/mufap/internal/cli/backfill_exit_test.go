// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"errors"
	"testing"
)

// Exiting 0 on an incomplete backfill is the failure this command exists to
// avoid: a script sees success, accepts a partially populated mirror, and every
// later cross-section is computed over a panel with holes. A sibling CLI
// records the same shape -- a backfill that stored 33 of 131 dates and said
// nothing about it.
//
// These tests pin the exit condition, and in particular pin what must NOT
// fail: gating on len(Errors) > 0 would be wrong, because Errors also carries
// advisory entries recorded on runs that stored everything asked of them.

func exitCodeOf(t *testing.T, err error) int {
	t.Helper()
	if err == nil {
		return 0
	}
	var ce *cliError
	if !errors.As(err, &ce) {
		t.Fatalf("error is not typed, so the process exit code would be generic: %v", err)
	}
	return ce.code
}

func TestBackfillIncompleteExitsNonZero(t *testing.T) {
	cases := []struct {
		name string
		sum  backfillSummary
		want int
	}{
		{
			name: "clean run",
			sum:  backfillSummary{DatesInRange: 3, DatesAttempted: 3, DatesStored: 3},
			want: 0,
		},
		{
			name: "dates asked for and not stored",
			sum: backfillSummary{
				DatesInRange: 3, DatesAttempted: 3, DatesStored: 2,
				DatesIncomplete: []string{"2026-09-02"},
			},
			want: 5,
		},
		{
			name: "run budget expired early",
			sum: backfillSummary{
				DatesInRange: 30, DatesAttempted: 4, DatesStored: 4,
				Truncated: true,
			},
			want: 5,
		},
		{
			name: "both incomplete and truncated",
			sum: backfillSummary{
				DatesIncomplete: []string{"2026-09-02", "2026-09-03"},
				Truncated:       true,
			},
			want: 5,
		},
		// --- the cases that must stay exit 0 ---
		{
			name: "published-nothing days are not failures",
			sum: backfillSummary{
				DatesInRange: 3, DatesAttempted: 3, DatesStored: 1,
				ZeroRowDates: []string{"2026-09-05", "2026-09-06"},
			},
			want: 0,
		},
		{
			name: "advisory error only: reference tab is not a dated panel",
			sum: backfillSummary{
				DatesInRange: 1, DatesAttempted: 1, DatesStored: 1,
				Errors: []string{"tab pricing is current reference data, not a dated panel"},
			},
			want: 0,
		},
		{
			name: "advisory error only: tab has no validity-date column",
			sum: backfillSummary{
				DatesInRange: 1, DatesAttempted: 1, DatesStored: 1,
				Errors: []string{"tab payout has no validity-date column; rows keyed on the requested date"},
			},
			want: 0,
		},
		{
			name: "forward-dated dates are unfetchable, not missed",
			sum: backfillSummary{
				DatesInRange: 2, DatesAttempted: 1, DatesStored: 1,
				DatesForwardDated: []string{"2099-01-01"},
			},
			want: 0,
		},
		{
			name: "everything already mirrored",
			sum: backfillSummary{
				DatesInRange: 5, DatesAttempted: 0, DatesStored: 0, DatesSkipped: 5,
			},
			want: 0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := exitCodeOf(t, backfillIncompleteErr(tc.sum))
			if got != tc.want {
				t.Errorf("exit code = %d, want %d", got, tc.want)
			}
		})
	}
}

// The message has to name the remedy, because the whole point of failing is
// that a human or a script goes and retries the missing dates.
func TestBackfillIncompleteErrorNamesTheRemedy(t *testing.T) {
	err := backfillIncompleteErr(backfillSummary{DatesIncomplete: []string{"2026-09-02"}})
	if err == nil {
		t.Fatal("expected an error")
	}
	msg := err.Error()
	for _, want := range []string{"incomplete", "re-run"} {
		if !contains(msg, want) {
			t.Errorf("message %q should mention %q", msg, want)
		}
	}

	tr := backfillIncompleteErr(backfillSummary{Truncated: true})
	if tr == nil {
		t.Fatal("a truncated run must fail")
	}
	if !contains(tr.Error(), "budget") {
		t.Errorf("truncated message %q should say the budget expired", tr.Error())
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// Every backfill leaf must declare the exit code it can now return, or the
// dogfood matrix scores an honest typed failure as a probe failure.
func TestBackfillLeavesDeclareTypedExitCodes(t *testing.T) {
	parent := newNovelBackfillCmd(&rootFlags{})
	leaves := parent.Commands()
	if len(leaves) == 0 {
		t.Fatal("backfill has no subcommands")
	}
	for _, c := range leaves {
		got := c.Annotations["pp:typed-exit-codes"]
		if got == "" {
			t.Errorf("backfill %s does not declare pp:typed-exit-codes; it can now exit 5", c.Name())
			continue
		}
		if !contains(got, "5") {
			t.Errorf("backfill %s declares %q, which omits the incomplete-run exit code 5", c.Name(), got)
		}
	}
}
