package adsanalytics

import "testing"

func TestWeeklyReviewPlansMutationReadyActionsWithCaps(t *testing.T) {
	t.Parallel()
	plan := WeeklyReview(
		[]PerformanceRow{{CampaignID: "c1", Campaign: "Core", Spend: 10, Sales: 100, Orders: 4, Budget: 100}},
		[]SearchTermPerformance{{CampaignID: "c1", AdGroupID: "a1", SearchTerm: "bad query", Spend: 20, Clicks: 30, Conversions: 0}},
		[]KeywordPerformance{{CampaignID: "c1", AdGroupID: "a1", KeywordID: "k1", Keyword: "blue widget", MatchType: "exact", Bid: 1.20, Spend: 30, Sales: 60, Orders: 1, Clicks: 20}},
		WeeklyReviewOptions{TargetACOSPercent: 25, NegateSpendThreshold: 10, NegateMinClicks: 20, TotalBudget: 150, MaxBidChangePercent: 25, MaxBudgetChangePercent: 10, MaxTotalBudgetIncrease: 5, Currency: "USD"},
	)
	if len(plan.Actions) == 0 {
		t.Fatalf("expected actions")
	}
	var sawBid, sawNeg bool
	for _, action := range plan.Actions {
		if action.Type == "lower_bid" {
			sawBid = true
			if action.Entity.KeywordID != "k1" || action.CurrentBid != 1.20 || action.ProposedBid < 0.89 || action.ProposedBid > 0.91 {
				t.Fatalf("bid action = %+v", action)
			}
			if action.Rollback["restore_bid"] != 1.20 {
				t.Fatalf("bid rollback = %+v", action.Rollback)
			}
		}
		if action.Type == "create_negative_keyword" {
			sawNeg = true
			if action.Entity.Scope != "ad_group_negative" || action.Entity.MatchType != "negativeExact" {
				t.Fatalf("negative action = %+v", action)
			}
		}
	}
	if !sawBid || !sawNeg {
		t.Fatalf("plan missing expected action types: %+v", plan.Actions)
	}
}

func TestWeeklyReviewPropagatesConfiguredCurrency(t *testing.T) {
	t.Parallel()
	plan := WeeklyReview(
		nil,
		[]SearchTermPerformance{{CampaignID: "c1", AdGroupID: "a1", SearchTerm: "bad query", Spend: 20, Clicks: 30, Conversions: 0}},
		nil,
		WeeklyReviewOptions{TargetACOSPercent: 25, NegateSpendThreshold: 10, NegateMinClicks: 20, Currency: "EUR"},
	)
	if plan.Currency != "EUR" {
		t.Fatalf("plan currency = %q, want EUR", plan.Currency)
	}
	if len(plan.Actions) != 1 {
		t.Fatalf("actions = %+v, want one action", plan.Actions)
	}
	if plan.Actions[0].Currency != "EUR" {
		t.Fatalf("action currency = %q, want EUR", plan.Actions[0].Currency)
	}
}

func TestCapBudgetChangeAppliesMinimumDailyBudget(t *testing.T) {
	t.Parallel()
	if got := capBudgetChange(10, 0.25, WeeklyReviewOptions{MaxBudgetChangePercent: 0}); got != 1 {
		t.Fatalf("capBudgetChange floor = %.2f, want 1.00", got)
	}
}

func TestCapBudgetChangeHonorsMaxDailyBudgetAfterPercentFloor(t *testing.T) {
	t.Parallel()
	got := capBudgetChange(100, 150, WeeklyReviewOptions{MaxDailyBudget: 80, MaxBudgetChangePercent: 10})
	if got != 80 {
		t.Fatalf("capBudgetChange conflicting max daily/percent cap = %.2f, want 80.00", got)
	}
}

func TestCapBidChangeHonorsMaxBidAfterPercentFloor(t *testing.T) {
	t.Parallel()
	got := capBidChange(2, 0.50, WeeklyReviewOptions{MaxBid: 1, MaxBidChangePercent: 25})
	if got != 1 {
		t.Fatalf("capBidChange conflicting max bid/percent cap = %.2f, want 1.00", got)
	}
}
