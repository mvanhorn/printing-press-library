package adsanalytics

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

type ShareOfVoiceRow struct {
	ASIN            string  `json:"asin,omitempty"`
	Keyword         string  `json:"keyword"`
	Impressions     int     `json:"impressions"`
	ImpressionShare float64 `json:"impression_share,omitempty"`
	Rank            int     `json:"rank,omitempty"`
	Date            string  `json:"date,omitempty"`
}

type ShareOfVoiceFinding struct {
	Keyword             string  `json:"keyword"`
	ASIN                string  `json:"asin,omitempty"`
	Impressions         int     `json:"impressions"`
	AverageShare        float64 `json:"average_share,omitempty"`
	BestRank            int     `json:"best_rank,omitempty"`
	Recommendation      string  `json:"recommendation"`
	ObservationCount    int     `json:"observation_count"`
	LowVisibilityReason string  `json:"low_visibility_reason,omitempty"`
}

func LoadShareOfVoiceReport(path string) ([]ShareOfVoiceRow, error) {
	data, err := ReadReportFile(path)
	if err != nil {
		return nil, err
	}
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return nil, fmt.Errorf("report %s is empty", path)
	}
	if strings.HasPrefix(trimmed, "[") {
		var rows []ShareOfVoiceRow
		if err := json.Unmarshal(data, &rows); err != nil {
			return nil, fmt.Errorf("parsing JSON report %s: %w", path, err)
		}
		return rows, nil
	}
	if strings.HasPrefix(trimmed, "{") {
		var envelope map[string][]ShareOfVoiceRow
		if err := json.Unmarshal(data, &envelope); err != nil {
			return nil, fmt.Errorf("parsing JSON report %s: %w", path, err)
		}
		for _, key := range []string{"rows", "data", "items", "keywords"} {
			if rows := envelope[key]; len(rows) > 0 {
				return rows, nil
			}
		}
		return nil, fmt.Errorf("JSON report %s must be an array or include rows/data/items/keywords", path)
	}
	rows, err := parseShareOfVoiceCSV(strings.NewReader(trimmed))
	if err != nil {
		return nil, fmt.Errorf("parsing CSV report %s: %w", path, err)
	}
	return rows, nil
}

func parseShareOfVoiceCSV(r io.Reader) ([]ShareOfVoiceRow, error) {
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
	rows := make([]ShareOfVoiceRow, 0, len(records)-1)
	for _, record := range records[1:] {
		keyword := value(record, "keyword", "search term", "query")
		if keyword == "" {
			continue
		}
		rows = append(rows, ShareOfVoiceRow{
			ASIN:            value(record, "asin", "advertised asin", "advertisedAsin"),
			Keyword:         keyword,
			Impressions:     int(parseNumber(value(record, "impressions"))),
			ImpressionShare: parsePercent(value(record, "impression share", "impressionShare", "share of voice", "shareOfVoice")),
			Rank:            int(parseNumber(value(record, "rank", "organic rank", "ad rank", "search rank"))),
			Date:            value(record, "date", "report date", "reportDate"),
		})
	}
	return rows, nil
}

func ShareOfVoice(rows []ShareOfVoiceRow, asin string, keywords []string, lowShareThreshold float64) []ShareOfVoiceFinding {
	if lowShareThreshold <= 0 {
		lowShareThreshold = 0.10
	}
	wanted := map[string]struct{}{}
	for _, keyword := range keywords {
		keyword = strings.ToLower(strings.TrimSpace(keyword))
		if keyword != "" {
			wanted[keyword] = struct{}{}
		}
	}
	type aggregate struct {
		row        ShareOfVoiceFinding
		shareSum   float64
		shareCount int
	}
	byKey := map[string]*aggregate{}
	for _, row := range rows {
		if asin != "" && !strings.EqualFold(row.ASIN, asin) {
			continue
		}
		keywordKey := strings.ToLower(strings.TrimSpace(row.Keyword))
		if len(wanted) > 0 {
			if _, ok := wanted[keywordKey]; !ok {
				continue
			}
		}
		key := strings.ToLower(row.ASIN) + "\x00" + keywordKey
		item := byKey[key]
		if item == nil {
			item = &aggregate{row: ShareOfVoiceFinding{Keyword: row.Keyword, ASIN: row.ASIN}}
			byKey[key] = item
		}
		item.row.Impressions += row.Impressions
		item.row.ObservationCount++
		if row.ImpressionShare > 0 {
			item.shareSum += row.ImpressionShare
			item.shareCount++
		}
		if row.Rank > 0 && (item.row.BestRank == 0 || row.Rank < item.row.BestRank) {
			item.row.BestRank = row.Rank
		}
	}
	out := make([]ShareOfVoiceFinding, 0, len(byKey))
	for _, item := range byKey {
		if item.shareCount > 0 {
			item.row.AverageShare = item.shareSum / float64(item.shareCount)
		}
		switch {
		case item.row.AverageShare > 0 && item.row.AverageShare < lowShareThreshold:
			item.row.Recommendation = "low share of voice; consider bid, budget, or relevance improvements"
			item.row.LowVisibilityReason = "average impression share is below threshold"
		case item.row.Impressions == 0:
			item.row.Recommendation = "no impressions observed; check eligibility and targeting"
			item.row.LowVisibilityReason = "no impressions observed"
		default:
			item.row.Recommendation = "monitor share of voice trend"
		}
		out = append(out, item.row)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].AverageShare == out[j].AverageShare {
			return out[i].Impressions > out[j].Impressions
		}
		return out[i].AverageShare < out[j].AverageShare
	})
	return out
}

func parsePercent(raw string) float64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	percent := strings.Contains(raw, "%")
	value := parseNumber(raw)
	if percent || value > 1 {
		value = value / 100
	}
	return value
}
