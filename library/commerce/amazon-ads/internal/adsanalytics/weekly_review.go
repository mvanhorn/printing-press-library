package adsanalytics

import "sort"

type WeeklyReviewOptions struct {
	TargetACOSPercent      float64
	NegateSpendThreshold   float64
	NegateMinClicks        int
	MaxBid                 float64
	MaxDailyBudget         float64
	MaxBidChangePercent    float64
	MaxBudgetChangePercent float64
	MaxTotalBudgetIncrease float64
	TotalBudget            float64
	Currency               string
}

type WeeklyReviewPlan struct {
	AdProduct  string               `json:"ad_product"`
	TargetACOS float64              `json:"target_acos"`
	Currency   string               `json:"currency"`
	Actions    []WeeklyReviewAction `json:"actions"`
	Skipped    []WeeklyReviewSkip   `json:"skipped,omitempty"`
	Summary    WeeklyReviewSummary  `json:"summary"`
}

type WeeklyReviewSummary struct {
	LowerBidActions         int     `json:"lower_bid_actions"`
	RaiseBidActions         int     `json:"raise_bid_actions"`
	NegativeKeywordActions  int     `json:"negative_keyword_actions"`
	BudgetActions           int     `json:"budget_actions"`
	EstimatedBudgetIncrease float64 `json:"estimated_budget_increase"`
}

type WeeklyReviewAction struct {
	Type           string         `json:"type"`
	Entity         ReviewEntity   `json:"entity"`
	CurrentBid     float64        `json:"current_bid,omitempty"`
	ProposedBid    float64        `json:"proposed_bid,omitempty"`
	CurrentBudget  float64        `json:"current_budget,omitempty"`
	ProposedBudget float64        `json:"proposed_budget,omitempty"`
	Currency       string         `json:"currency"`
	Reason         map[string]any `json:"reason"`
	Rollback       map[string]any `json:"rollback,omitempty"`
}

type ReviewEntity struct {
	Level      string `json:"level"`
	CampaignID string `json:"campaignId,omitempty"`
	AdGroupID  string `json:"adGroupId,omitempty"`
	KeywordID  string `json:"keywordId,omitempty"`
	TargetID   string `json:"targetId,omitempty"`
	MatchType  string `json:"matchType,omitempty"`
	Scope      string `json:"scope,omitempty"`
	Text       string `json:"text,omitempty"`
	Name       string `json:"name,omitempty"`
}

type WeeklyReviewSkip struct {
	Type   string       `json:"type"`
	Entity ReviewEntity `json:"entity"`
	Reason string       `json:"reason"`
}

func WeeklyReview(campaignRows []PerformanceRow, searchRows []SearchTermPerformance, keywordRows []KeywordPerformance, opts WeeklyReviewOptions) WeeklyReviewPlan {
	if opts.Currency == "" {
		opts.Currency = "USD"
	}
	targetACOS := opts.TargetACOSPercent / 100
	plan := WeeklyReviewPlan{
		AdProduct:  "sponsored_products",
		TargetACOS: targetACOS,
		Currency:   opts.Currency,
	}
	plan.Actions = append(plan.Actions, weeklyBidActions(keywordRows, targetACOS, opts)...)
	plan.Actions = append(plan.Actions, weeklyNegativeActions(searchRows, opts)...)
	plan.Actions = append(plan.Actions, weeklyBudgetActions(campaignRows, opts)...)
	sort.SliceStable(plan.Actions, func(i, j int) bool {
		if plan.Actions[i].Type == plan.Actions[j].Type {
			return plan.Actions[i].Entity.Name < plan.Actions[j].Entity.Name
		}
		return actionSortRank(plan.Actions[i].Type) < actionSortRank(plan.Actions[j].Type)
	})
	for _, action := range plan.Actions {
		switch action.Type {
		case "lower_bid":
			plan.Summary.LowerBidActions++
		case "raise_bid":
			plan.Summary.RaiseBidActions++
		case "create_negative_keyword":
			plan.Summary.NegativeKeywordActions++
		case "adjust_budget":
			plan.Summary.BudgetActions++
			if action.ProposedBudget > action.CurrentBudget {
				plan.Summary.EstimatedBudgetIncrease += action.ProposedBudget - action.CurrentBudget
			}
		}
	}
	return plan
}

func weeklyBidActions(rows []KeywordPerformance, targetACOS float64, opts WeeklyReviewOptions) []WeeklyReviewAction {
	var actions []WeeklyReviewAction
	for _, row := range rows {
		if row.KeywordID == "" || row.Keyword == "" || row.Bid <= 0 || row.Clicks <= 0 || row.Sales <= 0 {
			continue
		}
		acos := row.Spend / row.Sales
		conversionRate := row.ConversionRate
		if conversionRate == 0 {
			conversionRate = float64(row.Orders) / float64(row.Clicks)
		}
		aov := row.AverageOrderVal
		if aov == 0 && row.Orders > 0 {
			aov = row.Sales / float64(row.Orders)
		}
		if conversionRate <= 0 || aov <= 0 {
			continue
		}
		proposed := roundCurrency(targetACOS * aov * conversionRate)
		if proposed <= 0 {
			continue
		}
		proposed = capBidChange(row.Bid, proposed, opts)
		if proposed == row.Bid {
			continue
		}
		actionType := "lower_bid"
		if proposed > row.Bid {
			actionType = "raise_bid"
		}
		if actionType == "raise_bid" && acos > targetACOS {
			continue
		}
		actions = append(actions, WeeklyReviewAction{
			Type: actionType,
			Entity: ReviewEntity{
				Level:      "keyword",
				CampaignID: row.CampaignID,
				AdGroupID:  row.AdGroupID,
				KeywordID:  row.KeywordID,
				MatchType:  row.MatchType,
				Name:       row.Keyword,
			},
			CurrentBid:  row.Bid,
			ProposedBid: proposed,
			Currency:    opts.Currency,
			Reason: map[string]any{
				"acos":        acos,
				"target_acos": targetACOS,
				"spend":       row.Spend,
				"orders":      row.Orders,
				"clicks":      row.Clicks,
			},
			Rollback: map[string]any{"restore_bid": row.Bid},
		})
	}
	return actions
}

func weeklyNegativeActions(rows []SearchTermPerformance, opts WeeklyReviewOptions) []WeeklyReviewAction {
	plans := AutoNegate(rows, opts.NegateSpendThreshold, opts.NegateMinClicks)
	actions := make([]WeeklyReviewAction, 0, len(plans))
	for _, plan := range plans {
		actions = append(actions, WeeklyReviewAction{
			Type: "create_negative_keyword",
			Entity: ReviewEntity{
				Level:      "search_term",
				CampaignID: plan.CampaignID,
				AdGroupID:  plan.AdGroupID,
				Scope:      "ad_group_negative",
				MatchType:  "negativeExact",
				Text:       plan.SearchTerm,
				Name:       plan.SearchTerm,
			},
			Currency: opts.Currency,
			Reason: map[string]any{
				"spend":   plan.Spend,
				"clicks":  plan.Clicks,
				"orders":  0,
				"message": "zero orders above spend and click thresholds",
			},
		})
	}
	return actions
}

func weeklyBudgetActions(rows []PerformanceRow, opts WeeklyReviewOptions) []WeeklyReviewAction {
	if opts.TotalBudget <= 0 || len(rows) == 0 {
		return nil
	}
	campaigns := CampaignComparison(rows)
	rebalances := BudgetRebalance(campaigns, opts.TotalBudget)
	actions := make([]WeeklyReviewAction, 0, len(rebalances))
	totalIncrease := 0.0
	for _, item := range rebalances {
		if item.CampaignID == "" || item.CurrentBudget <= 0 || item.Action == "keep" {
			continue
		}
		proposed := capBudgetChange(item.CurrentBudget, item.Recommended, opts)
		if opts.MaxTotalBudgetIncrease > 0 && proposed > item.CurrentBudget {
			remaining := opts.MaxTotalBudgetIncrease - totalIncrease
			if remaining <= 0 {
				continue
			}
			if proposed-item.CurrentBudget > remaining {
				proposed = item.CurrentBudget + remaining
			}
		}
		proposed = roundCurrency(proposed)
		if proposed == item.CurrentBudget {
			continue
		}
		if proposed > item.CurrentBudget {
			totalIncrease += proposed - item.CurrentBudget
		}
		actions = append(actions, WeeklyReviewAction{
			Type: "adjust_budget",
			Entity: ReviewEntity{
				Level:      "campaign",
				CampaignID: item.CampaignID,
				Name:       item.Campaign,
			},
			CurrentBudget:  item.CurrentBudget,
			ProposedBudget: proposed,
			Currency:       opts.Currency,
			Reason: map[string]any{
				"acos":               item.ACOS,
				"spend":              item.Spend,
				"sales":              item.Sales,
				"allocation_score":   item.AllocationScore,
				"recommended_budget": item.Recommended,
			},
			Rollback: map[string]any{"restore_budget": item.CurrentBudget},
		})
	}
	return actions
}

func capBidChange(current, proposed float64, opts WeeklyReviewOptions) float64 {
	if opts.MaxBidChangePercent > 0 {
		maxDelta := current * (opts.MaxBidChangePercent / 100)
		if proposed > current+maxDelta {
			proposed = current + maxDelta
		}
		if proposed < current-maxDelta {
			proposed = current - maxDelta
		}
	}
	if proposed < 0.02 {
		proposed = 0.02
	}
	if opts.MaxBid > 0 && proposed > opts.MaxBid {
		proposed = opts.MaxBid
	}
	return roundCurrency(proposed)
}

func capBudgetChange(current, proposed float64, opts WeeklyReviewOptions) float64 {
	if opts.MaxBudgetChangePercent > 0 {
		maxDelta := current * (opts.MaxBudgetChangePercent / 100)
		if proposed > current+maxDelta {
			proposed = current + maxDelta
		}
		if proposed < current-maxDelta {
			proposed = current - maxDelta
		}
	}
	if proposed < 1 {
		proposed = 1
	}
	if opts.MaxDailyBudget > 0 && proposed > opts.MaxDailyBudget {
		proposed = opts.MaxDailyBudget
	}
	return proposed
}

func actionSortRank(action string) int {
	switch action {
	case "lower_bid":
		return 1
	case "raise_bid":
		return 2
	case "create_negative_keyword":
		return 3
	case "adjust_budget":
		return 4
	default:
		return 99
	}
}
