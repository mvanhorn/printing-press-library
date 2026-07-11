// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import "testing"

func TestBlockedReason(t *testing.T) {
	cases := []struct {
		state, build, want string
	}{
		{"READY_FOR_SALE", "VALID", ""},
		{"IN_REVIEW", "VALID", ""},
		{"METADATA_REJECTED", "VALID", "metadata rejected — fix metadata"},
		{"REJECTED", "VALID", "version rejected — resubmit"},
		{"DEVELOPER_REJECTED", "VALID", "version rejected — resubmit"},
		{"INVALID_BINARY", "VALID", "invalid binary — upload a new build"},
		{"READY_FOR_SALE", "FAILED", "latest build failed"},
		{"READY_FOR_SALE", "INVALID", "latest build invalid"},
	}
	for _, tc := range cases {
		if got := blockedReason(tc.state, tc.build); got != tc.want {
			t.Errorf("blockedReason(%q,%q) = %q, want %q", tc.state, tc.build, got, tc.want)
		}
	}
}

func TestActionFlag(t *testing.T) {
	cases := []struct {
		state, build, want string
	}{
		{"READY_FOR_SALE", "VALID", "ok"},
		{"WAITING_FOR_REVIEW", "VALID", "in review"},
		{"IN_REVIEW", "VALID", "in review"},
		{"READY_FOR_SALE", "PROCESSING", "build processing"},
		{"METADATA_REJECTED", "VALID", "metadata rejected — fix metadata"},
		{"READY_FOR_SALE", "FAILED", "latest build failed"},
	}
	for _, tc := range cases {
		if got := actionFlag(tc.state, tc.build); got != tc.want {
			t.Errorf("actionFlag(%q,%q) = %q, want %q", tc.state, tc.build, got, tc.want)
		}
	}
}

func TestMeanRatingAndTrend(t *testing.T) {
	if got := meanRating(nil); got != 0 {
		t.Errorf("meanRating(nil) = %v, want 0", got)
	}
	revs := []ascReview{{Rating: 5}, {Rating: 3}, {Rating: 4}, {Rating: 4}}
	if got := meanRating(revs); got != 4.0 {
		t.Errorf("meanRating = %v, want 4.0", got)
	}
	// newest-first: two recent 2s vs two older 5s → strongly negative trend.
	dropping := []ascReview{{Rating: 2}, {Rating: 2}, {Rating: 5}, {Rating: 5}}
	if got := ratingTrend(dropping); got >= 0 {
		t.Errorf("ratingTrend(dropping) = %v, want negative", got)
	}
	if got := ratingTrend([]ascReview{{Rating: 5}}); got != 0 {
		t.Errorf("ratingTrend(<4 reviews) = %v, want 0 (insufficient data)", got)
	}
}

func TestReviewState(t *testing.T) {
	if got := (ascVersion{AppStoreState: "IN_REVIEW"}).state(); got != "IN_REVIEW" {
		t.Errorf("state() = %q, want IN_REVIEW", got)
	}
	if got := (ascVersion{AppVersion: "READY_FOR_DISTRIBUTION"}).state(); got != "READY_FOR_DISTRIBUTION" {
		t.Errorf("state() fallback = %q", got)
	}
}

func TestAtoiOr(t *testing.T) {
	if got := atoiOr("", 20); got != 20 {
		t.Errorf("atoiOr empty = %d, want 20", got)
	}
	if got := atoiOr("5", 20); got != 5 {
		t.Errorf("atoiOr 5 = %d, want 5", got)
	}
	if got := atoiOr("garbage", 20); got != 20 {
		t.Errorf("atoiOr garbage = %d, want 20", got)
	}
	if got := atoiOr("-5", 20); got != 20 {
		t.Errorf("atoiOr -5 = %d, want 20 (negatives would panic a slice)", got)
	}
}
