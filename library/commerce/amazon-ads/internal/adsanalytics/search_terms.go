package adsanalytics

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
)

type SearchTermPerformance struct {
	CampaignID  string  `json:"campaign_id,omitempty"`
	Campaign    string  `json:"campaign,omitempty"`
	AdGroupID   string  `json:"ad_group_id,omitempty"`
	AdGroup     string  `json:"ad_group,omitempty"`
	SearchTerm  string  `json:"search_term"`
	Keyword     string  `json:"keyword,omitempty"`
	Spend       float64 `json:"spend"`
	Sales       float64 `json:"sales"`
	Conversions int     `json:"conversions"`
	Clicks      int     `json:"clicks"`
	Impressions int     `json:"impressions"`
}

type SearchTermRecommendation struct {
	Action      string  `json:"action"`
	SearchTerm  string  `json:"search_term"`
	Campaign    string  `json:"campaign,omitempty"`
	AdGroup     string  `json:"ad_group,omitempty"`
	Spend       float64 `json:"spend"`
	Sales       float64 `json:"sales"`
	Conversions int     `json:"conversions"`
	ACOS        float64 `json:"acos,omitempty"`
	Reason      string  `json:"reason"`
}

type CannibalizationFinding struct {
	SearchTerm     string   `json:"search_term"`
	WinnerCampaign string   `json:"winner_campaign"`
	Occurrences    int      `json:"occurrences"`
	Campaigns      []string `json:"campaigns"`
	TotalSpend     float64  `json:"total_spend"`
	TotalSales     float64  `json:"total_sales"`
	ExtraCPC       float64  `json:"extra_cpc,omitempty"`
	WinnerACOS     float64  `json:"winner_acos,omitempty"`
	LoserSpend     float64  `json:"loser_spend"`
	Recommendation string   `json:"recommendation"`
}

type KeywordOpportunity struct {
	SearchTerm     string  `json:"search_term"`
	SourceCampaign string  `json:"source_campaign,omitempty"`
	SourceAdGroup  string  `json:"source_ad_group,omitempty"`
	Spend          float64 `json:"spend"`
	Sales          float64 `json:"sales"`
	Conversions    int     `json:"conversions"`
	ACOS           float64 `json:"acos,omitempty"`
	SuggestedMatch string  `json:"suggested_match"`
	Reason         string  `json:"reason"`
}

func LoadSearchTermReport(path string) ([]SearchTermPerformance, error) {
	data, err := ReadReportFile(path)
	if err != nil {
		return nil, err
	}
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return nil, fmt.Errorf("report %s is empty", path)
	}
	if strings.HasPrefix(trimmed, "[") {
		var rows []SearchTermPerformance
		if err := json.Unmarshal(data, &rows); err != nil {
			return nil, fmt.Errorf("parsing JSON report %s: %w", path, err)
		}
		return rows, nil
	}
	if strings.HasPrefix(trimmed, "{") {
		var envelope map[string][]SearchTermPerformance
		if err := json.Unmarshal(data, &envelope); err != nil {
			return nil, fmt.Errorf("parsing JSON report %s: %w", path, err)
		}
		for _, key := range []string{"rows", "data", "items", "search_terms"} {
			if rows := envelope[key]; len(rows) > 0 {
				return rows, nil
			}
		}
		return nil, fmt.Errorf("JSON report %s must be an array or include rows/data/items/search_terms", path)
	}
	rows, err := parseSearchTermCSV(strings.NewReader(trimmed))
	if err != nil {
		return nil, fmt.Errorf("parsing CSV report %s: %w", path, err)
	}
	return rows, nil
}

func parseSearchTermCSV(r io.Reader) ([]SearchTermPerformance, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1
	records, err := cr.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) < 2 {
		return nil, fmt.Errorf("CSV report must include a header and at least one row")
	}
	headers := make(map[string]int, len(records[0]))
	for i, h := range records[0] {
		headers[normalizeHeader(h)] = i
	}
	value := func(row []string, names ...string) string {
		for _, name := range names {
			if idx, ok := headers[normalizeHeader(name)]; ok && idx < len(row) {
				return row[idx]
			}
		}
		return ""
	}
	rows := make([]SearchTermPerformance, 0, len(records)-1)
	for _, record := range records[1:] {
		row := SearchTermPerformance{
			CampaignID:  value(record, "campaign id", "campaignId"),
			Campaign:    value(record, "campaign", "campaign name", "campaignName"),
			AdGroupID:   value(record, "ad group id", "adGroupId"),
			AdGroup:     value(record, "ad group", "ad group name", "adGroupName"),
			SearchTerm:  value(record, "search term", "customer search term", "customerSearchTerm", "query"),
			Keyword:     value(record, "keyword", "targeting", "targeting expression"),
			Spend:       parseNumber(value(record, "spend", "cost")),
			Sales:       parseNumber(value(record, "sales", "sales7d", "14 day total sales", "total sales")),
			Conversions: int(parseNumber(value(record, "conversions", "purchases", "purchases7d", "orders", "orders7d"))),
			Clicks:      int(parseNumber(value(record, "clicks"))),
			Impressions: int(parseNumber(value(record, "impressions"))),
		}
		if row.SearchTerm == "" {
			continue
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func KeywordCannibalization(rows []SearchTermPerformance) []CannibalizationFinding {
	type aggregate struct {
		row       SearchTermPerformance
		campaigns map[string]struct{}
	}
	byTerm := map[string]*aggregate{}
	for _, row := range rows {
		term := strings.ToLower(strings.TrimSpace(row.SearchTerm))
		if term == "" {
			continue
		}
		item := byTerm[term]
		if item == nil {
			item = &aggregate{row: SearchTermPerformance{SearchTerm: row.SearchTerm}, campaigns: map[string]struct{}{}}
			byTerm[term] = item
		}
		item.row.Spend += row.Spend
		item.row.Sales += row.Sales
		item.row.Conversions += row.Conversions
		item.row.Clicks += row.Clicks
		item.row.Impressions += row.Impressions
		if row.Campaign != "" {
			item.campaigns[row.Campaign] = struct{}{}
		}
	}

	var findings []CannibalizationFinding
	for termKey, item := range byTerm {
		if len(item.campaigns) < 2 {
			continue
		}
		var termRows []SearchTermPerformance
		for _, row := range rows {
			if strings.EqualFold(strings.TrimSpace(row.SearchTerm), termKey) {
				termRows = append(termRows, row)
			}
		}
		sort.SliceStable(termRows, func(i, j int) bool {
			iAcos, jAcos := rowACOS(termRows[i]), rowACOS(termRows[j])
			if iAcos == jAcos {
				return termRows[i].Sales > termRows[j].Sales
			}
			if iAcos == 0 {
				return false
			}
			if jAcos == 0 {
				return true
			}
			return iAcos < jAcos
		})
		winner := termRows[0]
		totalSpend := 0.0
		totalSales := 0.0
		loserSpend := 0.0
		loserClicks := 0
		campaigns := make([]string, 0, len(item.campaigns))
		for campaign := range item.campaigns {
			campaigns = append(campaigns, campaign)
		}
		sort.Strings(campaigns)
		for _, row := range termRows {
			totalSpend += row.Spend
			totalSales += row.Sales
			if row.Campaign != winner.Campaign {
				loserSpend += row.Spend
				loserClicks += row.Clicks
			}
		}
		extraCPC := 0.0
		if loserClicks > 0 && winner.Clicks > 0 {
			extraCPC = (loserSpend / float64(loserClicks)) - (winner.Spend / float64(winner.Clicks))
		}
		findings = append(findings, CannibalizationFinding{
			SearchTerm:     item.row.SearchTerm,
			WinnerCampaign: winner.Campaign,
			Occurrences:    len(termRows),
			Campaigns:      campaigns,
			TotalSpend:     totalSpend,
			TotalSales:     totalSales,
			ExtraCPC:       extraCPC,
			WinnerACOS:     rowACOS(winner),
			LoserSpend:     loserSpend,
			Recommendation: fmt.Sprintf("consolidate search term into %s and review negatives in overlapping campaigns", winner.Campaign),
		})
	}
	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].LoserSpend == findings[j].LoserSpend {
			return findings[i].SearchTerm < findings[j].SearchTerm
		}
		return findings[i].LoserSpend > findings[j].LoserSpend
	})
	return findings
}

func NewKeywordOpportunities(rows []SearchTermPerformance, minConversions int, targetACOSPercent float64) []KeywordOpportunity {
	targetACOS := targetACOSPercent / 100
	seenExact := map[string]struct{}{}
	for _, row := range rows {
		keyword := strings.ToLower(strings.TrimSpace(row.Keyword))
		if keyword == "" {
			continue
		}
		if strings.Contains(strings.ToLower(row.Keyword), "exact") || strings.Contains(strings.ToLower(row.AdGroup), "exact") || strings.Contains(strings.ToLower(row.Campaign), "exact") {
			seenExact[keyword] = struct{}{}
		}
	}

	var out []KeywordOpportunity
	for _, row := range rows {
		term := strings.TrimSpace(row.SearchTerm)
		if term == "" || row.Conversions < minConversions || row.Sales <= 0 {
			continue
		}
		acos := row.Spend / row.Sales
		if acos > targetACOS {
			continue
		}
		if _, ok := seenExact[strings.ToLower(term)]; ok {
			continue
		}
		out = append(out, KeywordOpportunity{
			SearchTerm:     term,
			SourceCampaign: row.Campaign,
			SourceAdGroup:  row.AdGroup,
			Spend:          row.Spend,
			Sales:          row.Sales,
			Conversions:    row.Conversions,
			ACOS:           acos,
			SuggestedMatch: "exact",
			Reason:         "converting broad/auto search term is under target ACOS and has no obvious exact-match coverage",
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Conversions == out[j].Conversions {
			return out[i].ACOS < out[j].ACOS
		}
		return out[i].Conversions > out[j].Conversions
	})
	return out
}

func rowACOS(row SearchTermPerformance) float64 {
	if row.Sales <= 0 {
		return 0
	}
	return row.Spend / row.Sales
}

func SearchTermMining(rows []SearchTermPerformance, minConversions int, negateThreshold, targetACOSPercent float64) []SearchTermRecommendation {
	targetACOS := targetACOSPercent / 100
	var recs []SearchTermRecommendation
	for _, row := range rows {
		acos := 0.0
		if row.Sales > 0 {
			acos = row.Spend / row.Sales
		}
		switch {
		case row.Conversions >= minConversions && row.Sales > 0 && acos <= targetACOS:
			recs = append(recs, SearchTermRecommendation{
				Action:      "promote_exact",
				SearchTerm:  row.SearchTerm,
				Campaign:    row.Campaign,
				AdGroup:     row.AdGroup,
				Spend:       row.Spend,
				Sales:       row.Sales,
				Conversions: row.Conversions,
				ACOS:        acos,
				Reason:      "converting search term is under target ACOS",
			})
		case row.Spend >= negateThreshold && row.Conversions == 0:
			recs = append(recs, SearchTermRecommendation{
				Action:     "negative_exact",
				SearchTerm: row.SearchTerm,
				Campaign:   row.Campaign,
				AdGroup:    row.AdGroup,
				Spend:      row.Spend,
				Sales:      row.Sales,
				Reason:     "spent past threshold with zero conversions",
			})
		}
	}
	sortRecommendations(recs)
	return recs
}

func WastedSpend(rows []SearchTermPerformance, threshold float64) []SearchTermRecommendation {
	var recs []SearchTermRecommendation
	for _, row := range rows {
		if row.Spend >= threshold && row.Conversions == 0 {
			recs = append(recs, SearchTermRecommendation{
				Action:     "negative_exact",
				SearchTerm: row.SearchTerm,
				Campaign:   row.Campaign,
				AdGroup:    row.AdGroup,
				Spend:      row.Spend,
				Sales:      row.Sales,
				Reason:     "zero-conversion spend",
			})
		}
	}
	sortRecommendations(recs)
	return recs
}

func sortRecommendations(recs []SearchTermRecommendation) {
	sort.SliceStable(recs, func(i, j int) bool {
		if recs[i].Spend == recs[j].Spend {
			return recs[i].SearchTerm < recs[j].SearchTerm
		}
		return recs[i].Spend > recs[j].Spend
	})
}

func normalizeHeader(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	replacer := strings.NewReplacer(" ", "", "_", "", "-", "", ".", "")
	return replacer.Replace(s)
}

func parseNumber(raw string) float64 {
	raw = strings.TrimSpace(raw)
	raw = strings.Trim(raw, "$%")
	raw = strings.ReplaceAll(raw, ",", "")
	if raw == "" {
		return 0
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0
	}
	return value
}
