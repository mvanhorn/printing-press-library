// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.
//
// PATCH(digg-rankings-and-min-starrers): tests for the ParseStats type.

package diggparse

import (
	"errors"
	"strings"
	"testing"
)

func TestParseStats_AddTallies(t *testing.T) {
	var s ParseStats
	s.Add(nil)
	s.Add(errors.New("bad1"))
	s.Add(nil)
	s.Add(errors.New("bad2"))
	if s.Attempted != 4 {
		t.Errorf("Attempted = %d, want 4", s.Attempted)
	}
	if s.Decoded != 2 {
		t.Errorf("Decoded = %d, want 2", s.Decoded)
	}
	if s.Skipped != 2 {
		t.Errorf("Skipped = %d, want 2", s.Skipped)
	}
	if len(s.Errors) != 2 {
		t.Errorf("Errors len = %d, want 2", len(s.Errors))
	}
}

func TestParseStats_ErrorsBounded(t *testing.T) {
	var s ParseStats
	for i := 0; i < ParseStatsMaxErrors+10; i++ {
		s.Add(errors.New("e"))
	}
	if got := len(s.Errors); got != ParseStatsMaxErrors {
		t.Errorf("Errors len = %d, want %d", got, ParseStatsMaxErrors)
	}
	if s.Attempted != ParseStatsMaxErrors+10 {
		t.Errorf("Attempted = %d, want %d", s.Attempted, ParseStatsMaxErrors+10)
	}
	if s.Skipped != ParseStatsMaxErrors+10 {
		t.Errorf("Skipped = %d, want %d", s.Skipped, ParseStatsMaxErrors+10)
	}
}

func TestParseStats_SkipRatio(t *testing.T) {
	cases := []struct {
		attempted, decoded, skipped int
		want                        float64
	}{
		{0, 0, 0, 0},
		{10, 10, 0, 0},
		{10, 0, 10, 1.0},
		{10, 7, 3, 0.3},
		{4, 1, 3, 0.75},
	}
	for _, tc := range cases {
		s := ParseStats{Attempted: tc.attempted, Decoded: tc.decoded, Skipped: tc.skipped}
		if got := s.SkipRatio(); got != tc.want {
			t.Errorf("SkipRatio(%d/%d/%d) = %v, want %v",
				tc.attempted, tc.decoded, tc.skipped, got, tc.want)
		}
	}
}

func TestParseStats_Threshold(t *testing.T) {
	t.Run("empty stats never trip", func(t *testing.T) {
		var s ParseStats
		if err := s.Threshold(0.1); err != nil {
			t.Errorf("Threshold(0.1) on empty = %v, want nil", err)
		}
	})

	t.Run("below threshold returns nil", func(t *testing.T) {
		s := ParseStats{Attempted: 10, Decoded: 9, Skipped: 1}
		if err := s.Threshold(0.5); err != nil {
			t.Errorf("Threshold(0.5) with 10%% skip = %v, want nil", err)
		}
	})

	t.Run("at or above threshold returns ThresholdError", func(t *testing.T) {
		s := ParseStats{Attempted: 10, Decoded: 5, Skipped: 5}
		s.Errors = append(s.Errors, errors.New("decode failed: bad token"))
		err := s.Threshold(0.5)
		if err == nil {
			t.Fatal("Threshold(0.5) with 50%% skip = nil, want ThresholdError")
		}
		var te *ThresholdError
		if !errors.As(err, &te) {
			t.Fatalf("err type = %T, want *ThresholdError", err)
		}
		if te.MaxRatio != 0.5 || te.Stats.Skipped != 5 {
			t.Errorf("ThresholdError = %+v, want MaxRatio=0.5 Skipped=5", te)
		}
		if !strings.Contains(err.Error(), "0.50") {
			t.Errorf("error message missing skip ratio: %q", err.Error())
		}
	})

	t.Run("disabled when maxRatio out of range", func(t *testing.T) {
		s := ParseStats{Attempted: 10, Decoded: 0, Skipped: 10}
		for _, r := range []float64{-0.1, 1.5, 2.0} {
			if err := s.Threshold(r); err != nil {
				t.Errorf("Threshold(%v) on out-of-range = %v, want nil (disabled)", r, err)
			}
		}
	})

	t.Run("zero ratio = zero tolerance", func(t *testing.T) {
		// 0 is in-range and means "any skip trips" — distinct from
		// negative-or-out-of-range which disables the gate.
		oneSkip := ParseStats{Attempted: 10, Decoded: 9, Skipped: 1,
			Errors: []error{errors.New("one skip")}}
		if err := oneSkip.Threshold(0); err == nil {
			t.Error("Threshold(0) with one skip = nil, want ThresholdError (zero tolerance)")
		}
		// Empty stats still don't trip — Attempted==0 short-circuit
		// applies even at zero tolerance.
		empty := ParseStats{}
		if err := empty.Threshold(0); err != nil {
			t.Errorf("Threshold(0) on empty = %v, want nil", err)
		}
		// Clean parse (zero skips) doesn't trip even at zero tolerance.
		clean := ParseStats{Attempted: 10, Decoded: 10, Skipped: 0}
		if err := clean.Threshold(0); err != nil {
			t.Errorf("Threshold(0) with zero skips = %v, want nil", err)
		}
	})

	t.Run("ratio of 1 only trips at total failure", func(t *testing.T) {
		// 99% failure rate is below threshold of 1.0 — the gate only
		// trips when SkipRatio() >= maxRatio, and we never reach 1.0
		// without total failure.
		ninetyNine := ParseStats{Attempted: 100, Decoded: 1, Skipped: 99}
		if err := ninetyNine.Threshold(1.0); err != nil {
			t.Errorf("Threshold(1.0) at 99%% skip = %v, want nil (only trips at 100%%)", err)
		}
		total := ParseStats{Attempted: 10, Decoded: 0, Skipped: 10,
			Errors: []error{errors.New("total")}}
		if err := total.Threshold(1.0); err == nil {
			t.Error("Threshold(1.0) at 100%% skip = nil, want ThresholdError")
		}
	})

	t.Run("Unwrap exposes first decode error", func(t *testing.T) {
		sentinel := errors.New("sentinel")
		s := ParseStats{Attempted: 2, Skipped: 2, Errors: []error{sentinel, errors.New("other")}}
		err := s.Threshold(0.5)
		if !errors.Is(err, sentinel) {
			t.Errorf("errors.Is should match sentinel via Unwrap, got %v", err)
		}
	})
}
