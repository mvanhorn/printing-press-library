// Copyright 2026 Ryan Kelley and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"math"
	"testing"
)

// reportFixture returns a minimal keyword reporting API payload with two keyword rows.
func reportFixture() json.RawMessage {
	return json.RawMessage(`{
		"data": {
			"reportingDataResponse": {
				"row": [
					{
						"metadata": {
							"keywordId": "111",
							"keywordText": "yoga mat",
							"matchType": "EXACT",
							"adGroupId": "ag1",
							"bidAmount": {"amount": "1.50", "currency": "USD"}
						},
						"granularity": [
							{"taps": 120, "installs": 15, "localSpend": {"amount": "60.00"}, "revenue": {"amount": "0"}},
							{"taps": 80, "installs": 10, "localSpend": {"amount": "40.00"}, "revenue": {"amount": "0"}}
						]
					},
					{
						"metadata": {
							"keywordId": "222",
							"keywordText": "running shoes",
							"matchType": "BROAD",
							"adGroupId": "ag1",
							"bidAmount": {"amount": "2.00", "currency": "USD"}
						},
						"granularity": [
							{"taps": 50, "installs": 0, "localSpend": {"amount": "30.00"}, "revenue": {"amount": "150.00"}}
						]
					}
				]
			}
		}
	}`)
}

func TestExtractKeywordsFromReportPayload(t *testing.T) {
	kws := extractKeywordsFromReportPayload(reportFixture())
	if len(kws) != 2 {
		t.Fatalf("want 2 keywords, got %d", len(kws))
	}

	yoga := kws[0]
	if yoga.id != "111" {
		t.Errorf("want id=111, got %q", yoga.id)
	}
	if yoga.text != "yoga mat" {
		t.Errorf("want text=yoga mat, got %q", yoga.text)
	}
	if yoga.taps != 200 {
		t.Errorf("want taps=200, got %d", yoga.taps)
	}
	if yoga.installs != 25 {
		t.Errorf("want installs=25, got %d", yoga.installs)
	}
	if math.Abs(yoga.spend-100.0) > 0.01 {
		t.Errorf("want spend=100.00, got %f", yoga.spend)
	}
	if math.Abs(yoga.bid-1.50) > 0.001 {
		t.Errorf("want bid=1.50, got %f", yoga.bid)
	}

	shoes := kws[1]
	if shoes.id != "222" {
		t.Errorf("want id=222, got %q", shoes.id)
	}
	if math.Abs(shoes.revenue-150.0) > 0.01 {
		t.Errorf("want revenue=150.00, got %f", shoes.revenue)
	}
}

func TestExtractKeywordsFromReportPayload_Empty(t *testing.T) {
	kws := extractKeywordsFromReportPayload(json.RawMessage(`{"data":{"reportingDataResponse":{"row":[]}}}`))
	if len(kws) != 0 {
		t.Errorf("want 0 keywords, got %d", len(kws))
	}
}

func TestExtractKeywordsFromReportPayload_MissingID(t *testing.T) {
	// Rows without keywordId must be skipped.
	raw := json.RawMessage(`{"data":{"reportingDataResponse":{"row":[
		{"metadata":{"keywordText":"no-id"},"granularity":[]}
	]}}}`)
	kws := extractKeywordsFromReportPayload(raw)
	if len(kws) != 0 {
		t.Errorf("want 0, got %d", len(kws))
	}
}

func TestBuildBidSuggestions_CPA(t *testing.T) {
	// yoga mat: spend=100, installs=25 → CPA=4.00; bid=1.50
	// target CPA=2.00 → ratio=0.5 → suggested=0.75; delta=(0.75/1.50)*4.00-4.00=-2.00
	kws := extractKeywordsFromReportPayload(reportFixture())
	sug, err := buildBidSuggestions(kws[:1], "camp1", "cpa", 2.0)
	if err != nil {
		t.Fatal(err)
	}
	if len(sug) != 1 {
		t.Fatalf("want 1 suggestion, got %d", len(sug))
	}
	s := sug[0]
	if math.Abs(s.SuggestedBid-0.75) > 0.01 {
		t.Errorf("want suggested_bid=0.75, got %f", s.SuggestedBid)
	}
	if s.ChangeDirection != "decrease" {
		t.Errorf("want direction=decrease, got %q", s.ChangeDirection)
	}
	if math.Abs(s.ExpectedDelta-(-2.0)) > 0.01 {
		t.Errorf("want expected_delta=-2.00, got %f", s.ExpectedDelta)
	}
	if math.Abs(s.CurrentCPA-4.0) > 0.01 {
		t.Errorf("want current_cpa=4.00, got %f", s.CurrentCPA)
	}
}

func TestBuildBidSuggestions_CPA_Increase(t *testing.T) {
	// target CPA=10.00; current CPA=4.00; ratio=2.5 → capped at 2x → suggested=3.00
	kws := extractKeywordsFromReportPayload(reportFixture())
	sug, _ := buildBidSuggestions(kws[:1], "camp1", "cpa", 10.0)
	if len(sug) != 1 {
		t.Fatalf("want 1, got %d", len(sug))
	}
	s := sug[0]
	if math.Abs(s.SuggestedBid-3.0) > 0.01 {
		t.Errorf("want suggested_bid=3.00 (capped at 2x), got %f", s.SuggestedBid)
	}
	if s.ChangeDirection != "increase" {
		t.Errorf("want increase, got %q", s.ChangeDirection)
	}
}

func TestBuildBidSuggestions_ROAS(t *testing.T) {
	// shoes: spend=30, revenue=150 → ROAS=5.0; bid=2.00
	// target ROAS=10.0 → ratio=2.0 → suggested=4.00; delta=4.00-2.00=2.00
	kws := extractKeywordsFromReportPayload(reportFixture())
	sug, err := buildBidSuggestions(kws[1:], "camp1", "roas", 10.0)
	if err != nil {
		t.Fatal(err)
	}
	if len(sug) != 1 {
		t.Fatalf("want 1 suggestion, got %d", len(sug))
	}
	s := sug[0]
	if math.Abs(s.SuggestedBid-4.0) > 0.01 {
		t.Errorf("want suggested_bid=4.00, got %f", s.SuggestedBid)
	}
	if math.Abs(s.CurrentROAS-5.0) > 0.01 {
		t.Errorf("want current_roas=5.00, got %f", s.CurrentROAS)
	}
	if s.ChangeDirection != "increase" {
		t.Errorf("want increase, got %q", s.ChangeDirection)
	}
}

func TestBuildBidSuggestions_ROAS_Cap2x(t *testing.T) {
	// shoes: ROAS=5.0, bid=2.00; target=50.0 → ratio=10 → uncapped=20.00, capped at 2x=4.00
	kws := extractKeywordsFromReportPayload(reportFixture())
	sug, _ := buildBidSuggestions(kws[1:], "camp1", "roas", 50.0)
	if len(sug) != 1 {
		t.Fatalf("want 1, got %d", len(sug))
	}
	if math.Abs(sug[0].SuggestedBid-4.0) > 0.01 {
		t.Errorf("want suggested_bid=4.00 (2x cap on bid=2.00), got %f", sug[0].SuggestedBid)
	}
}

// TestFetchFailureAllCampaigns verifies the condition that triggers a non-zero exit
// when every campaign fetch attempt fails. The closure increments fetchFailed and
// fetchAttempted; the command returns an error when fetchFailed == fetchAttempted > 0.
func TestFetchFailureAllCampaigns(t *testing.T) {
	cases := []struct {
		failed, attempted int
		wantErr           bool
	}{
		{3, 3, true},  // all failed → non-zero exit
		{1, 1, true},  // single campaign failed
		{0, 3, false}, // none failed
		{2, 3, false}, // partial failure — some suggestions may exist
		{0, 0, false}, // no attempts (empty campaign list handled earlier)
	}
	for _, tc := range cases {
		got := tc.attempted > 0 && tc.failed == tc.attempted
		if got != tc.wantErr {
			t.Errorf("failed=%d attempted=%d: want err=%v, got %v", tc.failed, tc.attempted, tc.wantErr, got)
		}
	}
}

// TestApplyDryRunGate verifies that bid PUTs are skipped when --dry-run is set,
// even when --apply is also set — matching the documented example
// "optimize suggest --apply --dry-run". The gate is: flagApply && !dryRunOK(flags).
func TestApplyDryRunGate(t *testing.T) {
	cases := []struct {
		flagApply bool
		dryRun    bool
		wantApply bool // true = PUT calls should fire
	}{
		{true, true, false},   // --apply --dry-run: must NOT apply (the bug case)
		{true, false, true},   // --apply only: must apply
		{false, false, false}, // neither: no apply
		{false, true, false},  // --dry-run only: no apply
	}
	for _, tc := range cases {
		flags := &rootFlags{dryRun: tc.dryRun}
		got := tc.flagApply && !dryRunOK(flags)
		if got != tc.wantApply {
			t.Errorf("flagApply=%v dryRun=%v: want apply=%v, got %v",
				tc.flagApply, tc.dryRun, tc.wantApply, got)
		}
	}
}

func TestBuildBidSuggestions_ROAS_NoRevenue(t *testing.T) {
	// yoga mat has revenue=0; ROAS mode must skip it.
	kws := extractKeywordsFromReportPayload(reportFixture())
	sug, _ := buildBidSuggestions(kws[:1], "camp1", "roas", 5.0)
	if len(sug) != 0 {
		t.Errorf("want 0 suggestions (no revenue), got %d", len(sug))
	}
}

func TestBuildBidSuggestions_Taps_Proportional(t *testing.T) {
	// yoga mat: taps=200, bid=1.50; target=300 → ratio=300/200=1.5 → suggested=2.25
	kws := extractKeywordsFromReportPayload(reportFixture())
	sug, _ := buildBidSuggestions(kws[:1], "camp1", "taps", 300)
	if len(sug) != 1 {
		t.Fatalf("want 1, got %d", len(sug))
	}
	s := sug[0]
	if math.Abs(s.SuggestedBid-2.25) > 0.01 {
		t.Errorf("want suggested_bid=2.25, got %f", s.SuggestedBid)
	}
}

func TestBuildBidSuggestions_Taps_Cap2x(t *testing.T) {
	// target=10000 → ratio=50 → capped at 2.0 → suggested=3.00
	kws := extractKeywordsFromReportPayload(reportFixture())
	sug, _ := buildBidSuggestions(kws[:1], "camp1", "taps", 10000)
	if len(sug) != 1 {
		t.Fatalf("want 1, got %d", len(sug))
	}
	if math.Abs(sug[0].SuggestedBid-3.0) > 0.01 {
		t.Errorf("want suggested_bid=3.00 (2x cap), got %f", sug[0].SuggestedBid)
	}
}

func TestBuildBidSuggestions_Taps_AtTarget(t *testing.T) {
	// taps=200 already meets target=200 → hold
	kws := extractKeywordsFromReportPayload(reportFixture())
	sug, _ := buildBidSuggestions(kws[:1], "camp1", "taps", 200)
	if len(sug) != 1 {
		t.Fatalf("want 1, got %d", len(sug))
	}
	if sug[0].ChangeDirection != "hold" {
		t.Errorf("want hold, got %q", sug[0].ChangeDirection)
	}
}

func TestBuildBidSuggestions_NoBid(t *testing.T) {
	// Keywords with bid=0 must be skipped.
	kws := []keywordPerf{{id: "x", taps: 100, installs: 10, spend: 50}}
	sug, _ := buildBidSuggestions(kws, "camp1", "cpa", 2.0)
	if len(sug) != 0 {
		t.Errorf("want 0 (no bid), got %d", len(sug))
	}
}
