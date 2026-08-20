package adsanalytics

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
)

type AutoNegatePlan struct {
	CampaignID string  `json:"campaign_id,omitempty"`
	Campaign   string  `json:"campaign,omitempty"`
	AdGroupID  string  `json:"ad_group_id,omitempty"`
	AdGroup    string  `json:"ad_group,omitempty"`
	SearchTerm string  `json:"search_term"`
	Spend      float64 `json:"spend"`
	Clicks     int     `json:"clicks"`
	MatchType  string  `json:"match_type"`
	Action     string  `json:"action"`
	Reason     string  `json:"reason"`
}

type AutoPromotePlan struct {
	CampaignID  string  `json:"campaign_id,omitempty"`
	Campaign    string  `json:"campaign,omitempty"`
	AdGroupID   string  `json:"ad_group_id,omitempty"`
	AdGroup     string  `json:"ad_group,omitempty"`
	SearchTerm  string  `json:"search_term"`
	Spend       float64 `json:"spend"`
	Sales       float64 `json:"sales"`
	Conversions int     `json:"conversions"`
	ACOS        float64 `json:"acos"`
	MatchType   string  `json:"match_type"`
	Action      string  `json:"action"`
	Reason      string  `json:"reason"`
}

type BudgetRebalancePlan struct {
	CampaignID      string  `json:"campaign_id,omitempty"`
	Campaign        string  `json:"campaign"`
	CurrentBudget   float64 `json:"current_budget,omitempty"`
	Recommended     float64 `json:"recommended_budget"`
	Spend           float64 `json:"spend"`
	Sales           float64 `json:"sales"`
	ACOS            float64 `json:"acos,omitempty"`
	AllocationScore float64 `json:"allocation_score"`
	Action          string  `json:"action"`
}

type BidRule struct {
	Name          string  `json:"name"`
	MinACOS       float64 `json:"min_acos,omitempty"`
	MaxACOS       float64 `json:"max_acos,omitempty"`
	MinSpend      float64 `json:"min_spend,omitempty"`
	MaxSpend      float64 `json:"max_spend,omitempty"`
	MinOrders     int     `json:"min_orders,omitempty"`
	Action        string  `json:"action"`
	ChangePercent float64 `json:"change_percent,omitempty"`
	SetBid        float64 `json:"set_bid,omitempty"`
}

type BidRulePlan struct {
	Rule           string  `json:"rule"`
	KeywordID      string  `json:"keyword_id,omitempty"`
	Keyword        string  `json:"keyword"`
	Campaign       string  `json:"campaign,omitempty"`
	AdGroup        string  `json:"ad_group,omitempty"`
	CurrentBid     float64 `json:"current_bid,omitempty"`
	RecommendedBid float64 `json:"recommended_bid"`
	ACOS           float64 `json:"acos,omitempty"`
	Spend          float64 `json:"spend"`
	Orders         int     `json:"orders"`
	Action         string  `json:"action"`
	Reason         string  `json:"reason"`
}

func LoadBidRules(path string) ([]BidRule, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading bid rules %s: %w", path, err)
	}
	var rules []BidRule
	if err := json.Unmarshal(data, &rules); err == nil {
		return validateBidRules(rules)
	}
	var envelope struct {
		Rules []BidRule `json:"rules"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("parsing bid rules %s: %w", path, err)
	}
	return validateBidRules(envelope.Rules)
}

func validateBidRules(rules []BidRule) ([]BidRule, error) {
	if len(rules) == 0 {
		return nil, fmt.Errorf("at least one bid rule is required")
	}
	for i := range rules {
		if rules[i].Name == "" {
			rules[i].Name = fmt.Sprintf("rule-%d", i+1)
		}
		switch rules[i].Action {
		case "increase", "decrease", "set":
			// valid
		default:
			return nil, fmt.Errorf("rule %q action must be increase, decrease, or set", rules[i].Name)
		}
		if rules[i].Action == "set" && rules[i].SetBid <= 0 {
			return nil, fmt.Errorf("rule %q set_bid must be greater than zero", rules[i].Name)
		}
		if rules[i].Action != "set" && rules[i].ChangePercent <= 0 {
			return nil, fmt.Errorf("rule %q change_percent must be greater than zero", rules[i].Name)
		}
	}
	return rules, nil
}

func AutoNegate(rows []SearchTermPerformance, threshold float64, minClicks int) []AutoNegatePlan {
	var plans []AutoNegatePlan
	for _, row := range rows {
		if row.Conversions != 0 || row.Spend < threshold || row.Clicks < minClicks {
			continue
		}
		plans = append(plans, AutoNegatePlan{
			CampaignID: row.CampaignID,
			Campaign:   row.Campaign,
			AdGroupID:  row.AdGroupID,
			AdGroup:    row.AdGroup,
			SearchTerm: row.SearchTerm,
			Spend:      row.Spend,
			Clicks:     row.Clicks,
			MatchType:  "negativeExact",
			Action:     "create_negative_keyword",
			Reason:     "zero conversions above spend and click thresholds",
		})
	}
	sort.SliceStable(plans, func(i, j int) bool {
		return plans[i].Spend > plans[j].Spend
	})
	return plans
}

func AutoPromote(rows []SearchTermPerformance, minConversions int, maxACOSPercent float64) []AutoPromotePlan {
	maxACOS := maxACOSPercent / 100
	var plans []AutoPromotePlan
	for _, row := range rows {
		if row.Conversions < minConversions || row.Sales <= 0 {
			continue
		}
		acos := row.Spend / row.Sales
		if acos > maxACOS {
			continue
		}
		plans = append(plans, AutoPromotePlan{
			CampaignID:  row.CampaignID,
			Campaign:    row.Campaign,
			AdGroupID:   row.AdGroupID,
			AdGroup:     row.AdGroup,
			SearchTerm:  row.SearchTerm,
			Spend:       row.Spend,
			Sales:       row.Sales,
			Conversions: row.Conversions,
			ACOS:        acos,
			MatchType:   "exact",
			Action:      "create_exact_keyword",
			Reason:      "converting search term is under max ACOS",
		})
	}
	sort.SliceStable(plans, func(i, j int) bool {
		if plans[i].ACOS == plans[j].ACOS {
			return plans[i].Conversions > plans[j].Conversions
		}
		return plans[i].ACOS < plans[j].ACOS
	})
	return plans
}

func BudgetRebalance(campaigns []CampaignSummary, totalBudget float64) []BudgetRebalancePlan {
	if totalBudget <= 0 || len(campaigns) == 0 {
		return nil
	}
	scores := make([]float64, len(campaigns))
	totalScore := 0.0
	for i, campaign := range campaigns {
		score := campaign.Sales
		if campaign.ACOS > 0 {
			score = score / campaign.ACOS
		}
		if score <= 0 {
			score = 0.01
		}
		scores[i] = score
		totalScore += score
	}
	plans := make([]BudgetRebalancePlan, 0, len(campaigns))
	for i, campaign := range campaigns {
		recommended := totalBudget * (scores[i] / totalScore)
		action := "keep"
		if campaign.Budget > 0 {
			delta := (recommended - campaign.Budget) / campaign.Budget
			switch {
			case delta > 0.10:
				action = "increase"
			case delta < -0.10:
				action = "decrease"
			}
		} else {
			action = "set"
		}
		plans = append(plans, BudgetRebalancePlan{
			CampaignID:      campaign.CampaignID,
			Campaign:        campaign.Campaign,
			CurrentBudget:   campaign.Budget,
			Recommended:     recommended,
			Spend:           campaign.Spend,
			Sales:           campaign.Sales,
			ACOS:            campaign.ACOS,
			AllocationScore: scores[i],
			Action:          action,
		})
	}
	sort.SliceStable(plans, func(i, j int) bool {
		return plans[i].Recommended > plans[j].Recommended
	})
	return plans
}

func ApplyBidRules(rows []KeywordPerformance, rules []BidRule) []BidRulePlan {
	var plans []BidRulePlan
	for _, row := range rows {
		acos := 0.0
		if row.Sales > 0 {
			acos = row.Spend / row.Sales
		}
		for _, rule := range rules {
			if !bidRuleMatches(row, acos, rule) {
				continue
			}
			recommended := row.Bid
			switch rule.Action {
			case "increase":
				recommended = row.Bid * (1 + rule.ChangePercent/100)
			case "decrease":
				recommended = row.Bid * (1 - rule.ChangePercent/100)
			case "set":
				recommended = rule.SetBid
			}
			if recommended < 0.02 {
				recommended = 0.02
			}
			recommended = roundCurrency(recommended)
			plans = append(plans, BidRulePlan{
				Rule:           rule.Name,
				KeywordID:      row.KeywordID,
				Keyword:        row.Keyword,
				Campaign:       row.Campaign,
				AdGroup:        row.AdGroup,
				CurrentBid:     row.Bid,
				RecommendedBid: recommended,
				ACOS:           acos,
				Spend:          row.Spend,
				Orders:         row.Orders,
				Action:         rule.Action + "_bid",
				Reason:         fmt.Sprintf("matched bid rule %q", rule.Name),
			})
			break
		}
	}
	sort.SliceStable(plans, func(i, j int) bool {
		if plans[i].Rule == plans[j].Rule {
			return plans[i].Keyword < plans[j].Keyword
		}
		return plans[i].Rule < plans[j].Rule
	})
	return plans
}

func roundCurrency(value float64) float64 {
	return math.Round(value*100) / 100
}

func bidRuleMatches(row KeywordPerformance, acos float64, rule BidRule) bool {
	if row.Keyword == "" || row.Bid <= 0 {
		return false
	}
	if rule.MinSpend > 0 && row.Spend < rule.MinSpend {
		return false
	}
	if rule.MaxSpend > 0 && row.Spend > rule.MaxSpend {
		return false
	}
	if rule.MinOrders > 0 && row.Orders < rule.MinOrders {
		return false
	}
	if rule.MinACOS > 0 && acos < rule.MinACOS/100 {
		return false
	}
	if rule.MaxACOS > 0 && acos > rule.MaxACOS/100 {
		return false
	}
	return true
}
