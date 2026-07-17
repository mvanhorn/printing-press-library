// Copyright 2026 waveriderai and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestParseRevenueFromSlug(t *testing.T) {
	cases := []struct {
		slug string
		want int64
	}{
		{"i-turned-my-hobby-into-120k-month-apps", 120000},
		{"starting-a-travel-backpack-business-and-growing-to-20k-month", 20000},
		{"dollop-gourmet-how-heather-saffer-grew-her-vegan-frosting-business-to-30k-mo", 30000},
		{"how-emma-lovell-built-a-26k-mo-baby-stroller-business", 26000},
		{"starting-a-90k-month-respirator-mask-business", 90000},
		{"how-i-grew-my-online-booze-business-to-270k-month", 270000},
		{"a-2m-month-saas-story", 2000000},
		{"no-revenue-here", 0},
		{"just-some-ideas", 0},
		{"", 0},
	}
	for _, c := range cases {
		if got := parseRevenueFromSlug(c.slug); got != c.want {
			t.Errorf("parseRevenueFromSlug(%q) = %d, want %d", c.slug, got, c.want)
		}
	}
}

func TestHumanizeSlug(t *testing.T) {
	cases := []struct {
		slug string
		want string
	}{
		{"i-turned-my-hobby-into-120k-month-apps", "I Turned My Hobby Into 120k Month Apps"},
		{"newsletter", "Newsletter"},
		{"", ""},
	}
	for _, c := range cases {
		if got := humanizeSlug(c.slug); got != c.want {
			t.Errorf("humanizeSlug(%q) = %q, want %q", c.slug, got, c.want)
		}
	}
}

func TestRevenueBucket(t *testing.T) {
	cases := []struct {
		rev  int64
		want string
	}{
		{0, "0"},
		{-5, "0"},
		{5000, "1-9999"},
		{9999, "1-9999"},
		{10000, "10k-49k"},
		{49999, "10k-49k"},
		{50000, "50k-99k"},
		{99999, "50k-99k"},
		{100000, "100k+"},
		{5000000, "100k+"},
	}
	for _, c := range cases {
		if got := revenueBucket(c.rev); got != c.want {
			t.Errorf("revenueBucket(%d) = %q, want %q", c.rev, got, c.want)
		}
	}
}
