// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// TestNovelNicheHelpWires smoke-tests that the niche command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelNicheHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"niche", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("niche --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "niche"} {
		if !strings.Contains(help, want) {
			t.Fatalf("niche --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestNicheUsesCapturedProviderSchema(t *testing.T) {
	if nicheKeywordSortColumn != "searchVolume" {
		t.Fatalf("niche keyword sort = %q, want searchVolume", nicheKeywordSortColumn)
	}
	var keyword map[string]json.RawMessage
	if err := json.Unmarshal([]byte(`{
		"searchVolume":"1200",
		"competition":300,
		"competitionShops":45,
		"avgPrice":24.5,
		"volume":999999,
		"competingListings":999999,
		"competingShops":999999,
		"averagePrice":999999
	}`), &keyword); err != nil {
		t.Fatal(err)
	}
	view := nicheView{}
	applyNicheKeywordMetrics(&view, keyword)
	if view.SearchVolume != 1200 || view.CompetingListings != 300 || view.CompetingShops != 45 || view.AvgPrice != 24.5 {
		t.Fatalf("provider metrics mapped incorrectly: %+v", view)
	}
}

func TestListingAgeMonthsSupportsCapturedDateListed(t *testing.T) {
	now := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	var listing map[string]json.RawMessage
	if err := json.Unmarshal([]byte(`{"dateListed":"2025-07-01"}`), &listing); err != nil {
		t.Fatal(err)
	}
	age := listingAgeMonths(listing, now)
	if age < 6 || age > 6.1 {
		t.Fatalf("listing age = %v months, want about 6", age)
	}

	listing = map[string]json.RawMessage{"dateListed": json.RawMessage(`"not-a-date"`)}
	if got := listingAgeMonths(listing, now); got != 0 {
		t.Fatalf("invalid listing date age = %v, want 0", got)
	}
}
