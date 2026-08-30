// Copyright 2026 Jon Gouveia and contributors. Licensed under Apache-2.0. See LICENSE.

package wer

import (
	"math"
	"testing"
)

func TestRate(t *testing.T) {
	cases := []struct {
		name       string
		reference  string
		hypothesis string
		want       float64
	}{
		{name: "identical", reference: "a b c", hypothesis: "a b c", want: 0},
		{name: "one substitution in three words", reference: "a b c", hypothesis: "a b d", want: 1.0 / 3.0},
		{name: "one deletion", reference: "a b c", hypothesis: "a b", want: 1.0 / 3.0},
		{name: "one insertion", reference: "a b c", hypothesis: "a b c d", want: 1.0 / 3.0},
		{name: "casing and punctuation are normalized away", reference: "The quick brown fox.", hypothesis: "the QUICK brown fox", want: 0},
		{name: "an empty transcript scores as a total miss", reference: "a b c", hypothesis: "", want: 1},
		{name: "an empty reference with an empty transcript scores zero", reference: "", hypothesis: "", want: 0},
		{name: "an empty reference with words scores one", reference: "", hypothesis: "a", want: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Rate(tc.reference, tc.hypothesis)
			if math.Abs(got-tc.want) > 1e-9 {
				t.Fatalf("Rate(%q, %q) = %v, want %v", tc.reference, tc.hypothesis, got, tc.want)
			}
		})
	}
}

func TestVerdict(t *testing.T) {
	cases := []struct {
		rate float64
		want string
	}{
		{0, VerdictPass},
		{0.149, VerdictPass},
		{0.15, VerdictWarn},
		{0.299, VerdictWarn},
		{0.30, VerdictFail},
		{1, VerdictFail},
	}
	for _, tc := range cases {
		if got := Verdict(tc.rate); got != tc.want {
			t.Fatalf("Verdict(%v) = %q, want %q", tc.rate, got, tc.want)
		}
	}
}

func TestNormalize(t *testing.T) {
	got := Normalize("  The, quick -- brown fox's DAWN!  ")
	want := []string{"the", "quick", "brown", "fox's", "dawn"}
	if len(got) != len(want) {
		t.Fatalf("Normalize returned %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Normalize[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
