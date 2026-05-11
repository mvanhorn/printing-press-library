// Copyright 2026 bossriceshark. Licensed under Apache-2.0. See LICENSE.
// PATCH: Cover the added Search Analytics workflow helpers without live API calls.

package cli

import (
	"bytes"
	"testing"
)

func TestSearchAnalyticsWorkflowCommandsRegistered(t *testing.T) {
	root := RootCmd()
	if cmd, _, err := root.Find([]string{"branded-split"}); err != nil || cmd == nil || cmd.Name() != "brand-vs-nonbrand-split" {
		t.Fatalf("branded-split alias resolved to %v, %v; want brand-vs-nonbrand-split", cmd, err)
	}
	if cmd, _, err := root.Find([]string{"brand-split"}); err != nil || cmd == nil || cmd.Name() != "brand-vs-nonbrand-split" {
		t.Fatalf("brand-split alias resolved to %v, %v; want brand-vs-nonbrand-split", cmd, err)
	}
	if cmd, _, err := root.Find([]string{"page-query-breakdown"}); err != nil || cmd == nil || cmd.Name() != "page-queries" {
		t.Fatalf("page-query-breakdown alias resolved to %v, %v; want page-queries", cmd, err)
	}
}

func TestSearchAnalyticsWorkflowDryRunSkipsRequiredWorkflowFlags(t *testing.T) {
	for _, args := range [][]string{
		{"brand-vs-nonbrand-split", "sc-domain:example.com", "--dry-run"},
		{"page-queries", "sc-domain:example.com", "https://example.com/page", "--dry-run"},
	} {
		root := RootCmd()
		var out bytes.Buffer
		root.SetOut(&out)
		root.SetErr(&out)
		root.SetArgs(args)
		if err := root.Execute(); err != nil {
			t.Fatalf("RootCmd(%v) returned error: %v", args, err)
		}
	}
}

func TestNormalizeSearchAnalyticsRowLimitRejectsZero(t *testing.T) {
	if _, err := normalizeSearchAnalyticsRowLimit(0); err == nil {
		t.Fatal("expected --row-limit 0 to return an error")
	}
	if _, err := normalizeSearchAnalyticsRowLimit(-1); err == nil {
		t.Fatal("expected negative --row-limit to return an error")
	}
}

func TestNormalizeSearchAnalyticsRowLimitCapsAtMax(t *testing.T) {
	got, err := normalizeSearchAnalyticsRowLimit(maxSearchAnalyticsRowLimit + 1)
	if err != nil {
		t.Fatalf("normalizeSearchAnalyticsRowLimit returned error: %v", err)
	}
	if got != maxSearchAnalyticsRowLimit {
		t.Fatalf("row limit: want %d, got %d", maxSearchAnalyticsRowLimit, got)
	}
}

func TestAddSearchAnalyticsMetadataMarksTruncatedAtLimit(t *testing.T) {
	result := map[string]any{}
	rows := []searchAnalyticsRow{{}, {}}
	addSearchAnalyticsMetadata(result, rows, len(rows))

	if result["rows_returned"] != len(rows) {
		t.Fatalf("rows_returned: want %d, got %v", len(rows), result["rows_returned"])
	}
	if result["row_limit"] != len(rows) {
		t.Fatalf("row_limit: want %d, got %v", len(rows), result["row_limit"])
	}
	if result["truncated"] != true {
		t.Fatalf("truncated: want true, got %v", result["truncated"])
	}
}

func TestAddSearchAnalyticsMetadataLeavesPartialResultsUntruncated(t *testing.T) {
	result := map[string]any{}
	rows := []searchAnalyticsRow{{}}
	addSearchAnalyticsMetadata(result, rows, len(rows)+1)

	if result["rows_returned"] != len(rows) {
		t.Fatalf("rows_returned: want %d, got %v", len(rows), result["rows_returned"])
	}
	if result["truncated"] != false {
		t.Fatalf("truncated: want false, got %v", result["truncated"])
	}
}

func TestBuildBrandMatcher_TermsAreCaseInsensitive(t *testing.T) {
	matcher, err := buildBrandMatcher([]string{"Example"}, "")
	if err != nil {
		t.Fatalf("buildBrandMatcher returned error: %v", err)
	}
	if !matcher("example login") {
		t.Fatal("expected exact lowercase term match")
	}
	if !matcher("EXAMPLE apartments") {
		t.Fatal("expected case-insensitive term match")
	}
	if matcher("apartments near me") {
		t.Fatal("did not expect non-brand query to match")
	}
}

func TestBuildBrandMatcher_RegexAndTermsAreORed(t *testing.T) {
	matcher, err := buildBrandMatcher([]string{"example"}, `acme\s+co`)
	if err != nil {
		t.Fatalf("buildBrandMatcher returned error: %v", err)
	}
	if !matcher("example pricing") {
		t.Fatal("expected term match")
	}
	if !matcher("ACME CO reviews") {
		t.Fatal("expected case-insensitive regex match")
	}
}

func TestBuildBrandMatcher_RequiresClassifier(t *testing.T) {
	if _, err := buildBrandMatcher(nil, ""); err == nil {
		t.Fatal("expected error when no brand classifier is provided")
	}
}

func TestSearchSummaryUsesImpressionWeightedPosition(t *testing.T) {
	summary := searchMetricSummary{}
	addSearchSummary(&summary, searchAnalyticsRow{Clicks: 1, Impressions: 10, Position: 2})
	addSearchSummary(&summary, searchAnalyticsRow{Clicks: 3, Impressions: 30, Position: 6})
	finalizeSearchSummary(&summary)

	if summary.Clicks != 4 {
		t.Fatalf("clicks: want 4, got %v", summary.Clicks)
	}
	if summary.Impressions != 40 {
		t.Fatalf("impressions: want 40, got %v", summary.Impressions)
	}
	if summary.CTR != 0.1 {
		t.Fatalf("ctr: want 0.1, got %v", summary.CTR)
	}
	if summary.Position != 5 {
		t.Fatalf("position: want 5, got %v", summary.Position)
	}
}
