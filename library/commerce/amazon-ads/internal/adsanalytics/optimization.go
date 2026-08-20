package adsanalytics

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
)

type KeywordPerformance struct {
	KeywordID       string  `json:"keyword_id,omitempty"`
	CampaignID      string  `json:"campaign_id,omitempty"`
	Campaign        string  `json:"campaign,omitempty"`
	AdGroupID       string  `json:"ad_group_id,omitempty"`
	AdGroup         string  `json:"ad_group,omitempty"`
	Keyword         string  `json:"keyword"`
	MatchType       string  `json:"match_type,omitempty"`
	Date            string  `json:"date,omitempty"`
	Bid             float64 `json:"bid,omitempty"`
	CPC             float64 `json:"cpc,omitempty"`
	Spend           float64 `json:"spend"`
	Sales           float64 `json:"sales"`
	Orders          int     `json:"orders"`
	Clicks          int     `json:"clicks"`
	ConversionRate  float64 `json:"conversion_rate,omitempty"`
	AverageOrderVal float64 `json:"average_order_value,omitempty"`
}

type BidRecommendation struct {
	Keyword         string  `json:"keyword"`
	Campaign        string  `json:"campaign,omitempty"`
	AdGroup         string  `json:"ad_group,omitempty"`
	CurrentBid      float64 `json:"current_bid,omitempty"`
	RecommendedBid  float64 `json:"recommended_bid"`
	DeltaPercent    float64 `json:"delta_percent,omitempty"`
	Action          string  `json:"action"`
	ConversionRate  float64 `json:"conversion_rate"`
	AverageOrderVal float64 `json:"average_order_value"`
	Reason          string  `json:"reason"`
}

type KeywordDecayFinding struct {
	Keyword             string  `json:"keyword"`
	Campaign            string  `json:"campaign,omitempty"`
	AdGroup             string  `json:"ad_group,omitempty"`
	BaselineACOS        float64 `json:"baseline_acos,omitempty"`
	CurrentACOS         float64 `json:"current_acos,omitempty"`
	ACOSIncreasePercent float64 `json:"acos_increase_percent,omitempty"`
	BaselineCVR         float64 `json:"baseline_cvr,omitempty"`
	CurrentCVR          float64 `json:"current_cvr,omitempty"`
	CVRDropPercent      float64 `json:"cvr_drop_percent,omitempty"`
	Spend               float64 `json:"spend"`
	Reason              string  `json:"reason"`
}

type KeywordLifecycleFinding struct {
	Keyword  string  `json:"keyword"`
	Campaign string  `json:"campaign,omitempty"`
	AdGroup  string  `json:"ad_group,omitempty"`
	Stage    string  `json:"stage"`
	Spend    float64 `json:"spend"`
	Sales    float64 `json:"sales"`
	Orders   int     `json:"orders"`
	ACOS     float64 `json:"acos,omitempty"`
	Reason   string  `json:"reason"`
}

type BidHistoryPoint struct {
	Date           string  `json:"date,omitempty"`
	Keyword        string  `json:"keyword"`
	Campaign       string  `json:"campaign,omitempty"`
	AdGroup        string  `json:"ad_group,omitempty"`
	Bid            float64 `json:"bid,omitempty"`
	CPC            float64 `json:"cpc,omitempty"`
	Spend          float64 `json:"spend"`
	Sales          float64 `json:"sales"`
	Orders         int     `json:"orders"`
	Clicks         int     `json:"clicks"`
	ConversionRate float64 `json:"conversion_rate,omitempty"`
	ACOS           float64 `json:"acos,omitempty"`
}

func LoadKeywordPerformanceReport(path string) ([]KeywordPerformance, error) {
	data, err := ReadReportFile(path)
	if err != nil {
		return nil, err
	}
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return nil, fmt.Errorf("report %s is empty", path)
	}
	if strings.HasPrefix(trimmed, "[") {
		var rows []KeywordPerformance
		if err := json.Unmarshal(data, &rows); err != nil {
			return nil, fmt.Errorf("parsing JSON report %s: %w", path, err)
		}
		normalizeKeywordRows(rows)
		return rows, nil
	}
	if strings.HasPrefix(trimmed, "{") {
		var envelope map[string][]KeywordPerformance
		if err := json.Unmarshal(data, &envelope); err != nil {
			return nil, fmt.Errorf("parsing JSON report %s: %w", path, err)
		}
		for _, key := range []string{"rows", "data", "items", "keywords"} {
			if rows := envelope[key]; len(rows) > 0 {
				normalizeKeywordRows(rows)
				return rows, nil
			}
		}
		return nil, fmt.Errorf("JSON report %s must be an array or include rows/data/items/keywords", path)
	}
	rows, err := parseKeywordCSV(strings.NewReader(trimmed))
	if err != nil {
		return nil, fmt.Errorf("parsing CSV report %s: %w", path, err)
	}
	return rows, nil
}

func parseKeywordCSV(r io.Reader) ([]KeywordPerformance, error) {
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
	rows := make([]KeywordPerformance, 0, len(records)-1)
	for _, record := range records[1:] {
		row := KeywordPerformance{
			KeywordID:  value(record, "keyword id", "keywordId"),
			CampaignID: value(record, "campaign id", "campaignId"),
			Campaign:   value(record, "campaign", "campaign name", "campaignName"),
			AdGroupID:  value(record, "ad group id", "adGroupId"),
			AdGroup:    value(record, "ad group", "ad group name", "adGroupName"),
			Keyword:    value(record, "keyword", "targeting", "targeting expression"),
			MatchType:  value(record, "match type", "matchType"),
			Date:       value(record, "date", "start date", "startDate", "report date", "reportDate", "snapshot_at", "snapshotAt"),
			Bid:        parseNumber(value(record, "bid", "keyword bid", "keywordBid")),
			CPC:        parseNumber(value(record, "cpc", "cost per click", "costPerClick")),
			Spend:      parseNumber(value(record, "spend", "cost")),
			Sales:      parseNumber(value(record, "sales", "sales7d", "14 day total sales", "total sales")),
			Orders:     int(parseNumber(value(record, "orders", "orders7d", "purchases", "purchases7d", "conversions"))),
			Clicks:     int(parseNumber(value(record, "clicks"))),
		}
		if row.Keyword == "" {
			continue
		}
		rows = append(rows, row)
	}
	normalizeKeywordRows(rows)
	return rows, nil
}

func BidOptimizer(rows []KeywordPerformance, targetACOSPercent float64) []BidRecommendation {
	targetACOS := targetACOSPercent / 100
	var recs []BidRecommendation
	for _, row := range rows {
		if row.Keyword == "" || row.Clicks <= 0 {
			continue
		}
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
		recommended := targetACOS * aov * conversionRate
		action := "on_target"
		reason := "current bid is within 10% of target-derived bid"
		delta := 0.0
		if row.Bid > 0 {
			delta = (recommended - row.Bid) / row.Bid
			switch {
			case delta > 0.10:
				action = "raise"
				reason = "target-derived bid is more than 10% above current bid"
			case delta < -0.10:
				action = "lower"
				reason = "target-derived bid is more than 10% below current bid"
			}
		} else {
			action = "set"
			reason = "no current bid was present in the report"
		}
		recs = append(recs, BidRecommendation{
			Keyword:         row.Keyword,
			Campaign:        row.Campaign,
			AdGroup:         row.AdGroup,
			CurrentBid:      row.Bid,
			RecommendedBid:  recommended,
			DeltaPercent:    delta * 100,
			Action:          action,
			ConversionRate:  conversionRate,
			AverageOrderVal: aov,
			Reason:          reason,
		})
	}
	sort.SliceStable(recs, func(i, j int) bool {
		if recs[i].Action == recs[j].Action {
			return recs[i].Keyword < recs[j].Keyword
		}
		return actionRank(recs[i].Action) < actionRank(recs[j].Action)
	})
	return recs
}

func KeywordDecay(baseline, current []KeywordPerformance, thresholdPercent, minSpend float64) []KeywordDecayFinding {
	baseByKeyword := map[string]KeywordPerformance{}
	for _, row := range baseline {
		key := strings.ToLower(strings.TrimSpace(row.Keyword))
		if key == "" {
			continue
		}
		baseByKeyword[key] = mergeKeywordPerformance(baseByKeyword[key], row)
	}
	curByKeyword := map[string]KeywordPerformance{}
	for _, row := range current {
		key := strings.ToLower(strings.TrimSpace(row.Keyword))
		if key == "" {
			continue
		}
		curByKeyword[key] = mergeKeywordPerformance(curByKeyword[key], row)
	}

	var findings []KeywordDecayFinding
	for key, cur := range curByKeyword {
		if cur.Spend < minSpend {
			continue
		}
		base, ok := baseByKeyword[key]
		if !ok {
			continue
		}
		baseACOS, curACOS := keywordACOS(base), keywordACOS(cur)
		baseCVR, curCVR := keywordCVR(base), keywordCVR(cur)
		acosIncrease := percentIncrease(baseACOS, curACOS)
		cvrDrop := percentDrop(baseCVR, curCVR)
		if acosIncrease < thresholdPercent && cvrDrop < thresholdPercent {
			continue
		}
		reason := "keyword performance degraded"
		if acosIncrease >= thresholdPercent && cvrDrop >= thresholdPercent {
			reason = "ACOS increased and conversion rate dropped past threshold"
		} else if acosIncrease >= thresholdPercent {
			reason = "ACOS increased past threshold"
		} else {
			reason = "conversion rate dropped past threshold"
		}
		findings = append(findings, KeywordDecayFinding{
			Keyword:             cur.Keyword,
			Campaign:            cur.Campaign,
			AdGroup:             cur.AdGroup,
			BaselineACOS:        roundMetric(baseACOS),
			CurrentACOS:         roundMetric(curACOS),
			ACOSIncreasePercent: roundMetric(acosIncrease),
			BaselineCVR:         roundMetric(baseCVR),
			CurrentCVR:          roundMetric(curCVR),
			CVRDropPercent:      roundMetric(cvrDrop),
			Spend:               cur.Spend,
			Reason:              reason,
		})
	}
	sort.SliceStable(findings, func(i, j int) bool {
		return findings[i].Spend > findings[j].Spend
	})
	return findings
}

func KeywordLifecycle(rows []KeywordPerformance, targetACOSPercent float64) []KeywordLifecycleFinding {
	targetACOS := targetACOSPercent / 100
	var out []KeywordLifecycleFinding
	for _, row := range rows {
		acos := keywordACOS(row)
		stage := "discovery"
		reason := "keyword has traffic but limited conversion data"
		switch {
		case row.Orders == 0 && row.Spend >= 15:
			stage = "neglected"
			reason = "spend is accumulating without orders"
		case row.Orders >= 3 && acos > 0 && acos <= targetACOS:
			stage = "graduation"
			reason = "converting under target ACOS; candidate for exact-match graduation"
		case row.Orders >= 5 && acos > 0 && acos <= targetACOS*1.2:
			stage = "maturity"
			reason = "keyword is converting near target ACOS"
		case row.Orders > 0 && acos > targetACOS*1.5:
			stage = "decline"
			reason = "keyword is converting but above acceptable ACOS"
		}
		out = append(out, KeywordLifecycleFinding{
			Keyword:  row.Keyword,
			Campaign: row.Campaign,
			AdGroup:  row.AdGroup,
			Stage:    stage,
			Spend:    row.Spend,
			Sales:    row.Sales,
			Orders:   row.Orders,
			ACOS:     acos,
			Reason:   reason,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Stage == out[j].Stage {
			return out[i].Spend > out[j].Spend
		}
		return lifecycleRank(out[i].Stage) < lifecycleRank(out[j].Stage)
	})
	return out
}

func BidHistory(rows []KeywordPerformance, keyword string) []BidHistoryPoint {
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	var out []BidHistoryPoint
	for _, row := range rows {
		if keyword != "" && !strings.EqualFold(strings.TrimSpace(row.Keyword), keyword) {
			continue
		}
		out = append(out, BidHistoryPoint{
			Date:           row.Date,
			Keyword:        row.Keyword,
			Campaign:       row.Campaign,
			AdGroup:        row.AdGroup,
			Bid:            row.Bid,
			CPC:            row.CPC,
			Spend:          row.Spend,
			Sales:          row.Sales,
			Orders:         row.Orders,
			Clicks:         row.Clicks,
			ConversionRate: keywordCVR(row),
			ACOS:           keywordACOS(row),
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Keyword < out[j].Keyword
	})
	return out
}

func mergeKeywordPerformance(a, b KeywordPerformance) KeywordPerformance {
	if a.Keyword == "" {
		a.Keyword = b.Keyword
		a.Campaign = b.Campaign
		a.AdGroup = b.AdGroup
		a.MatchType = b.MatchType
		a.Bid = b.Bid
	}
	a.Spend += b.Spend
	a.Sales += b.Sales
	a.Orders += b.Orders
	a.Clicks += b.Clicks
	return a
}

func keywordACOS(row KeywordPerformance) float64 {
	if row.Sales <= 0 {
		return 0
	}
	return row.Spend / row.Sales
}

func keywordCVR(row KeywordPerformance) float64 {
	if row.Clicks <= 0 {
		return 0
	}
	return float64(row.Orders) / float64(row.Clicks)
}

func percentIncrease(old, current float64) float64 {
	if old <= 0 || current <= old {
		return 0
	}
	return ((current - old) / old) * 100
}

func percentDrop(old, current float64) float64 {
	if old <= 0 || current >= old {
		return 0
	}
	return ((old - current) / old) * 100
}

func lifecycleRank(stage string) int {
	switch stage {
	case "neglected":
		return 0
	case "decline":
		return 1
	case "graduation":
		return 2
	case "maturity":
		return 3
	default:
		return 4
	}
}

func roundMetric(value float64) float64 {
	return math.Round(value*10000) / 10000
}

func normalizeKeywordRows(rows []KeywordPerformance) {
	for i := range rows {
		if rows[i].ConversionRate == 0 && rows[i].Clicks > 0 {
			rows[i].ConversionRate = float64(rows[i].Orders) / float64(rows[i].Clicks)
		}
		if rows[i].AverageOrderVal == 0 && rows[i].Orders > 0 {
			rows[i].AverageOrderVal = rows[i].Sales / float64(rows[i].Orders)
		}
	}
}

func actionRank(action string) int {
	switch action {
	case "lower":
		return 0
	case "raise":
		return 1
	case "set":
		return 2
	default:
		return 3
	}
}
