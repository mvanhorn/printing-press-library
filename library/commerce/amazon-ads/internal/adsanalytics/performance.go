package adsanalytics

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
)

type PerformanceRow struct {
	CampaignID  string  `json:"campaign_id,omitempty"`
	Campaign    string  `json:"campaign,omitempty"`
	AdGroup     string  `json:"ad_group,omitempty"`
	ASIN        string  `json:"asin,omitempty"`
	TargetASIN  string  `json:"target_asin,omitempty"`
	SKU         string  `json:"sku,omitempty"`
	Date        string  `json:"date,omitempty"`
	Hour        int     `json:"hour,omitempty"`
	HasHour     bool    `json:"-"`
	Placement   string  `json:"placement,omitempty"`
	Spend       float64 `json:"spend"`
	Sales       float64 `json:"sales"`
	Orders      int     `json:"orders"`
	Clicks      int     `json:"clicks"`
	Impressions int     `json:"impressions"`
	Budget      float64 `json:"budget,omitempty"`
}

type PortfolioSummary struct {
	Spend       float64 `json:"spend"`
	Sales       float64 `json:"sales"`
	Orders      int     `json:"orders"`
	Clicks      int     `json:"clicks"`
	Impressions int     `json:"impressions"`
	ACOS        float64 `json:"acos,omitempty"`
	CPC         float64 `json:"cpc,omitempty"`
	CTR         float64 `json:"ctr,omitempty"`
	CVR         float64 `json:"cvr,omitempty"`
}

type CampaignSummary struct {
	CampaignID string `json:"campaign_id,omitempty"`
	Campaign   string `json:"campaign"`
	PortfolioSummary
	Budget float64 `json:"budget,omitempty"`
}

type PlacementSummary struct {
	Placement string `json:"placement"`
	PortfolioSummary
}

type CompetitorASINFinding struct {
	ASIN           string  `json:"asin"`
	Spend          float64 `json:"spend"`
	Sales          float64 `json:"sales"`
	Orders         int     `json:"orders"`
	Clicks         int     `json:"clicks"`
	ACOS           float64 `json:"acos,omitempty"`
	Recommendation string  `json:"recommendation"`
}

type SeasonalPlan struct {
	Period            string  `json:"period"`
	Spend             float64 `json:"spend"`
	Sales             float64 `json:"sales"`
	Orders            int     `json:"orders"`
	ACOS              float64 `json:"acos,omitempty"`
	RecommendedBudget float64 `json:"recommended_budget"`
	Recommendation    string  `json:"recommendation"`
}

type DaypartingCell struct {
	DayOfWeek string `json:"day_of_week"`
	Hour      int    `json:"hour"`
	PortfolioSummary
	Recommendation string `json:"recommendation,omitempty"`
}

type BudgetPacingFinding struct {
	Campaign       string  `json:"campaign"`
	Date           string  `json:"date,omitempty"`
	DailyBudget    float64 `json:"daily_budget"`
	Spend          float64 `json:"spend"`
	ExhaustedHour  int     `json:"exhausted_hour,omitempty"`
	Threshold      float64 `json:"threshold"`
	Recommendation string  `json:"recommendation"`
}

type ProductProfitability struct {
	ASIN            string  `json:"asin"`
	Name            string  `json:"name,omitempty"`
	Spend           float64 `json:"spend"`
	Sales           float64 `json:"sales"`
	Orders          int     `json:"orders"`
	COGS            float64 `json:"cogs,omitempty"`
	EstimatedFees   float64 `json:"estimated_fees,omitempty"`
	EstimatedProfit float64 `json:"estimated_profit"`
	ACOS            float64 `json:"acos,omitempty"`
}

func LoadPerformanceReport(path string) ([]PerformanceRow, error) {
	data, err := ReadReportFile(path)
	if err != nil {
		return nil, err
	}
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return nil, fmt.Errorf("report %s is empty", path)
	}
	if strings.HasPrefix(trimmed, "[") {
		var rows []PerformanceRow
		if err := json.Unmarshal(data, &rows); err != nil {
			return nil, fmt.Errorf("parsing JSON report %s: %w", path, err)
		}
		return rows, nil
	}
	if strings.HasPrefix(trimmed, "{") {
		var envelope map[string][]PerformanceRow
		if err := json.Unmarshal(data, &envelope); err != nil {
			return nil, fmt.Errorf("parsing JSON report %s: %w", path, err)
		}
		for _, key := range []string{"rows", "data", "items", "campaigns", "products"} {
			if rows := envelope[key]; len(rows) > 0 {
				return rows, nil
			}
		}
		return nil, fmt.Errorf("JSON report %s must be an array or include rows/data/items/campaigns/products", path)
	}
	rows, err := parsePerformanceCSV(strings.NewReader(trimmed))
	if err != nil {
		return nil, fmt.Errorf("parsing CSV report %s: %w", path, err)
	}
	return rows, nil
}

func parsePerformanceCSV(r io.Reader) ([]PerformanceRow, error) {
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
	rows := make([]PerformanceRow, 0, len(records)-1)
	for _, record := range records[1:] {
		dateValue := value(record, "date", "start date", "startDate", "report date", "reportDate")
		hour, hasHour := parseHourValue(value(record, "hour", "hour of day", "hourOfDay", "time of day", "timeOfDay"))
		row := PerformanceRow{
			CampaignID:  value(record, "campaign id", "campaignId"),
			Campaign:    value(record, "campaign", "campaign name", "campaignName"),
			AdGroup:     value(record, "ad group", "ad group name", "adGroupName"),
			ASIN:        value(record, "asin", "advertised asin", "advertisedAsin", "purchased asin", "purchasedAsin"),
			TargetASIN:  value(record, "target asin", "targeting asin", "targetingAsin", "targeting expression", "targetingExpression", "purchased asin", "purchasedAsin"),
			SKU:         value(record, "sku", "advertised sku", "advertisedSku"),
			Date:        dateValue,
			Hour:        hour,
			HasHour:     hasHour,
			Placement:   value(record, "placement", "placement classification", "placementClassification"),
			Spend:       parseNumber(value(record, "spend", "cost")),
			Sales:       parseNumber(value(record, "sales", "sales7d", "14 day total sales", "total sales")),
			Orders:      int(parseNumber(value(record, "orders", "orders7d", "purchases", "purchases7d", "conversions"))),
			Clicks:      int(parseNumber(value(record, "clicks"))),
			Impressions: int(parseNumber(value(record, "impressions"))),
			Budget:      parseNumber(value(record, "budget", "daily budget", "dailyBudget")),
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func PortfolioDashboard(rows []PerformanceRow) PortfolioSummary {
	var summary PortfolioSummary
	for _, row := range rows {
		summary.Spend += row.Spend
		summary.Sales += row.Sales
		summary.Orders += row.Orders
		summary.Clicks += row.Clicks
		summary.Impressions += row.Impressions
	}
	fillDerived(&summary)
	return summary
}

func CampaignComparison(rows []PerformanceRow) []CampaignSummary {
	byCampaign := map[string]*CampaignSummary{}
	for _, row := range rows {
		key := row.Campaign
		if key == "" {
			key = "(unknown)"
		}
		item := byCampaign[key]
		if item == nil {
			item = &CampaignSummary{Campaign: key, CampaignID: row.CampaignID}
			byCampaign[key] = item
		}
		if item.CampaignID == "" {
			item.CampaignID = row.CampaignID
		}
		item.Spend += row.Spend
		item.Sales += row.Sales
		item.Orders += row.Orders
		item.Clicks += row.Clicks
		item.Impressions += row.Impressions
		if row.Budget > item.Budget {
			item.Budget = row.Budget
		}
	}
	out := make([]CampaignSummary, 0, len(byCampaign))
	for _, item := range byCampaign {
		fillDerived(&item.PortfolioSummary)
		out = append(out, *item)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Spend > out[j].Spend
	})
	return out
}

func PlacementAnalysis(rows []PerformanceRow) []PlacementSummary {
	byPlacement := map[string]*PlacementSummary{}
	for _, row := range rows {
		key := row.Placement
		if key == "" {
			key = "(unknown)"
		}
		item := byPlacement[key]
		if item == nil {
			item = &PlacementSummary{Placement: key}
			byPlacement[key] = item
		}
		item.Spend += row.Spend
		item.Sales += row.Sales
		item.Orders += row.Orders
		item.Clicks += row.Clicks
		item.Impressions += row.Impressions
	}
	out := make([]PlacementSummary, 0, len(byPlacement))
	for _, item := range byPlacement {
		fillDerived(&item.PortfolioSummary)
		out = append(out, *item)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Spend > out[j].Spend
	})
	return out
}

func CompetitorASINMining(rows []PerformanceRow, ownASIN string) []CompetitorASINFinding {
	byASIN := map[string]*CompetitorASINFinding{}
	for _, row := range rows {
		key := row.TargetASIN
		if key == "" || key == row.ASIN || (ownASIN != "" && key == ownASIN) {
			continue
		}
		item := byASIN[key]
		if item == nil {
			item = &CompetitorASINFinding{ASIN: key}
			byASIN[key] = item
		}
		item.Spend += row.Spend
		item.Sales += row.Sales
		item.Orders += row.Orders
		item.Clicks += row.Clicks
	}
	out := make([]CompetitorASINFinding, 0, len(byASIN))
	for _, item := range byASIN {
		if item.Sales > 0 {
			item.ACOS = item.Spend / item.Sales
			item.Recommendation = "keep harvesting this competitor ASIN; sales exceed spend"
		} else {
			item.Recommendation = "review as a negative product target; spend has not produced sales"
		}
		out = append(out, *item)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Sales == out[j].Sales {
			return out[i].Spend > out[j].Spend
		}
		return out[i].Sales > out[j].Sales
	})
	return out
}

func SeasonalPlanner(rows []PerformanceRow, budgetMultiplier float64) []SeasonalPlan {
	if budgetMultiplier <= 0 {
		budgetMultiplier = 1
	}
	byPeriod := map[string]*SeasonalPlan{}
	for _, row := range rows {
		period := monthPeriod(row.Date)
		if period == "" {
			continue
		}
		item := byPeriod[period]
		if item == nil {
			item = &SeasonalPlan{Period: period}
			byPeriod[period] = item
		}
		item.Spend += row.Spend
		item.Sales += row.Sales
		item.Orders += row.Orders
	}
	out := make([]SeasonalPlan, 0, len(byPeriod))
	for _, item := range byPeriod {
		if item.Sales > 0 {
			item.ACOS = item.Spend / item.Sales
		}
		switch {
		case item.Sales == 0:
			item.RecommendedBudget = item.Spend * 0.5
			item.Recommendation = "reduce budget until this seasonal period shows sales"
		case item.ACOS <= 0.25:
			item.RecommendedBudget = item.Spend * budgetMultiplier
			item.Recommendation = "scale budget for this seasonal period"
		case item.ACOS >= 0.50:
			item.RecommendedBudget = item.Spend * 0.75
			item.Recommendation = "tighten budget or bids before this seasonal period"
		default:
			item.RecommendedBudget = item.Spend
			item.Recommendation = "hold budget near historical spend"
		}
		out = append(out, *item)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Period < out[j].Period
	})
	return out
}

func DaypartingAnalysis(rows []PerformanceRow) []DaypartingCell {
	byCell := map[string]*DaypartingCell{}
	for _, row := range rows {
		day, hour, ok := daypart(row)
		if !ok {
			continue
		}
		key := fmt.Sprintf("%s:%02d", day, hour)
		cell := byCell[key]
		if cell == nil {
			cell = &DaypartingCell{DayOfWeek: day, Hour: hour}
			byCell[key] = cell
		}
		cell.Spend += row.Spend
		cell.Sales += row.Sales
		cell.Orders += row.Orders
		cell.Clicks += row.Clicks
		cell.Impressions += row.Impressions
	}
	out := make([]DaypartingCell, 0, len(byCell))
	for _, cell := range byCell {
		fillDerived(&cell.PortfolioSummary)
		switch {
		case cell.Orders > 0 && cell.ACOS > 0 && cell.ACOS <= 0.25:
			cell.Recommendation = "strong conversion window; consider higher bids or budget rules"
		case cell.Spend > 0 && cell.Orders == 0:
			cell.Recommendation = "spend without orders; consider lowering bids during this window"
		default:
			cell.Recommendation = "monitor"
		}
		out = append(out, *cell)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].DayOfWeek == out[j].DayOfWeek {
			return out[i].Hour < out[j].Hour
		}
		return weekdayRank(out[i].DayOfWeek) < weekdayRank(out[j].DayOfWeek)
	})
	return out
}

func BudgetPacing(rows []PerformanceRow, threshold float64, earlyHour int) []BudgetPacingFinding {
	if threshold <= 0 || threshold > 1 {
		threshold = 0.9
	}
	if earlyHour <= 0 || earlyHour > 23 {
		earlyHour = 18
	}
	type bucket struct {
		campaign string
		date     string
		budget   float64
		byHour   map[int]float64
	}
	buckets := map[string]*bucket{}
	for _, row := range rows {
		day, hour, ok := daypart(row)
		if !ok || row.Budget <= 0 {
			continue
		}
		date := reportDate(row.Date)
		if date == "" {
			date = day
		}
		campaign := row.Campaign
		if campaign == "" {
			campaign = "(unknown)"
		}
		key := campaign + "\x00" + date
		item := buckets[key]
		if item == nil {
			item = &bucket{campaign: campaign, date: date, byHour: map[int]float64{}}
			buckets[key] = item
		}
		if row.Budget > item.budget {
			item.budget = row.Budget
		}
		item.byHour[hour] += row.Spend
	}
	var findings []BudgetPacingFinding
	for _, item := range buckets {
		cumulative := 0.0
		exhaustedHour := -1
		for hour := 0; hour <= 23; hour++ {
			cumulative += item.byHour[hour]
			if exhaustedHour == -1 && cumulative >= item.budget*threshold {
				exhaustedHour = hour
			}
		}
		if exhaustedHour >= 0 && exhaustedHour <= earlyHour {
			findings = append(findings, BudgetPacingFinding{
				Campaign:       item.campaign,
				Date:           item.date,
				DailyBudget:    item.budget,
				Spend:          cumulative,
				ExhaustedHour:  exhaustedHour,
				Threshold:      threshold,
				Recommendation: "campaign is pacing early; consider daypart rules, lower morning bids, or higher daily budget",
			})
		}
	}
	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].ExhaustedHour == findings[j].ExhaustedHour {
			return findings[i].Spend > findings[j].Spend
		}
		return findings[i].ExhaustedHour < findings[j].ExhaustedHour
	})
	return findings
}

func monthPeriod(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if len(raw) >= 7 && raw[4] == '-' {
		return raw[:7]
	}
	for _, layout := range []string{"1/2/2006", "01/02/2006", "2006/01/02", time.RFC3339} {
		if ts, err := time.Parse(layout, raw); err == nil {
			return ts.Format("2006-01")
		}
	}
	return ""
}

func daypart(row PerformanceRow) (string, int, bool) {
	if ts, ok := parseReportTime(row.Date); ok {
		return ts.Weekday().String(), ts.Hour(), true
	}
	if row.HasHour || row.Hour > 0 {
		if row.Hour < 0 || row.Hour > 23 {
			return "", 0, false
		}
		day := "Unknown"
		if ts, ok := parseReportDate(row.Date); ok {
			day = ts.Weekday().String()
		}
		return day, row.Hour, true
	}
	return "", 0, false
}

func reportDate(raw string) string {
	if ts, ok := parseReportDate(raw); ok {
		return ts.Format("2006-01-02")
	}
	if strings.TrimSpace(raw) != "" && len(raw) >= 10 {
		return strings.TrimSpace(raw)[:10]
	}
	return ""
}

func parseReportTime(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	for _, layout := range []string{
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006/01/02 15:04:05",
		"2006/01/02 15:04",
		"01/02/2006 15:04",
		"1/2/2006 15:04",
	} {
		if ts, err := time.Parse(layout, raw); err == nil {
			return ts, true
		}
	}
	return time.Time{}, false
}

func parseReportDate(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if ts, ok := parseReportTime(raw); ok {
		return ts, true
	}
	for _, layout := range []string{"2006-01-02", "1/2/2006", "01/02/2006", "2006/01/02"} {
		if ts, err := time.Parse(layout, raw); err == nil {
			return ts, true
		}
	}
	return time.Time{}, false
}

func parseHourValue(raw string) (int, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, false
	}
	hour := int(parseNumber(raw))
	return hour, hour >= 0 && hour <= 23
}

func weekdayRank(day string) int {
	switch day {
	case "Sunday":
		return 0
	case "Monday":
		return 1
	case "Tuesday":
		return 2
	case "Wednesday":
		return 3
	case "Thursday":
		return 4
	case "Friday":
		return 5
	case "Saturday":
		return 6
	default:
		return 7
	}
}

func ProductAdProfitability(rows []PerformanceRow, costs map[string]ProductCost, feePercent float64) []ProductProfitability {
	byASIN := map[string]*ProductProfitability{}
	for _, row := range rows {
		key := row.ASIN
		if key == "" {
			continue
		}
		item := byASIN[key]
		if item == nil {
			item = &ProductProfitability{ASIN: key}
			if cost, ok := costs[key]; ok {
				item.Name = cost.Name
				item.COGS = cost.COGS
			}
			byASIN[key] = item
		}
		item.Spend += row.Spend
		item.Sales += row.Sales
		item.Orders += row.Orders
	}
	out := make([]ProductProfitability, 0, len(byASIN))
	for _, item := range byASIN {
		if item.Sales > 0 {
			item.ACOS = item.Spend / item.Sales
		}
		item.EstimatedFees = item.Sales * (feePercent / 100)
		item.EstimatedProfit = item.Sales - (item.COGS * float64(item.Orders)) - item.EstimatedFees - item.Spend
		out = append(out, *item)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].EstimatedProfit < out[j].EstimatedProfit
	})
	return out
}

func fillDerived(summary *PortfolioSummary) {
	if summary.Sales > 0 {
		summary.ACOS = summary.Spend / summary.Sales
	}
	if summary.Clicks > 0 {
		summary.CPC = summary.Spend / float64(summary.Clicks)
		summary.CVR = float64(summary.Orders) / float64(summary.Clicks)
	}
	if summary.Impressions > 0 {
		summary.CTR = float64(summary.Clicks) / float64(summary.Impressions)
	}
}
