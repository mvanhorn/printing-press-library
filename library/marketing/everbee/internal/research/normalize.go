package research

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/everbee/internal/store"
)

func NormalizeRecords(resourceType string, rawRecords []json.RawMessage) ([]EvidenceRecord, Coverage) {
	coverage := Coverage{
		ResourceCounts: map[string]int{resourceType: 0},
		RawRecordCount: len(rawRecords),
	}
	evidence := make([]EvidenceRecord, 0, len(rawRecords))

	for _, raw := range rawRecords {
		var obj map[string]any
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.UseNumber()
		if err := decoder.Decode(&obj); err != nil {
			continue
		}

		record := EvidenceRecord{
			ID:               firstID(obj, "listing_id", "id", "product_id", "keyword_id", "shop_id"),
			Resource:         resourceType,
			Title:            firstString(obj, "title", "name"),
			ShopName:         firstString(obj, "shop_name", "shop"),
			ListingID:        firstID(obj, "listing_id"),
			Tags:             firstStringSlice(obj, "tags", "tag_list"),
			Keywords:         firstStringSlice(obj, "keywords", "keyword", "query"),
			Price:            firstFloatPtr(obj, "price"),
			EstimatedSales:   firstFloatPtr(obj, "estimated_sales", "sales"),
			EstimatedRevenue: firstFloatPtr(obj, "estimated_revenue", "revenue"),
			Rank:             firstInt(obj, "rank", "position"),
			SearchableText:   searchableEvidenceText(obj),
		}
		if record.ID == "" {
			record.ID = record.Title
		}
		if record.ListingID == "" {
			record.ListingID = firstID(obj, "listing_id")
		}

		if !hasMeaningfulEvidence(record) {
			continue
		}
		evidence = append(evidence, record)
		coverage.ResourceCounts[resourceType]++
	}

	coverage.EvidenceRecordCount = len(evidence)
	return evidence, coverage
}

func hasMeaningfulEvidence(record EvidenceRecord) bool {
	return record.ID != "" ||
		record.Title != "" ||
		record.ShopName != "" ||
		record.ListingID != "" ||
		len(record.Keywords) > 0 ||
		len(record.Tags) > 0 ||
		record.Price != nil ||
		record.EstimatedSales != nil ||
		record.EstimatedRevenue != nil ||
		record.Rank != 0 ||
		record.SearchableText != ""
}

func NoDataEnvelope(scope ResearchScope, plan ResearchPlan, nextActions []string) ResponseEnvelope {
	return ResponseEnvelope{
		Scope:       scope,
		DataSource:  plan.DataSource,
		Freshness:   plan.Freshness,
		Summary:     "No usable EverBee research data found for this scope.",
		Confidence:  0,
		Coverage:    Coverage{},
		Warnings:    plan.Warnings,
		NextActions: nextActions,
	}
}

func ConfidenceForCoverage(coverage Coverage) float64 {
	if coverage.RawRecordCount <= 0 {
		return 0
	}
	confidence := float64(coverage.EvidenceRecordCount) / float64(coverage.RawRecordCount)
	if confidence < 0 {
		return 0
	}
	if confidence > 1 {
		return 1
	}
	return confidence
}

func firstID(obj map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := store.LookupFieldValue(obj, key); value != nil {
			if id := store.FormatResourceID(value); id != "" && id != "<nil>" {
				return id
			}
		}
	}
	return ""
}

func firstString(obj map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := store.LookupFieldValue(obj, key); value != nil {
			if text := strings.TrimSpace(fmt.Sprint(value)); text != "" && text != "<nil>" {
				return text
			}
		}
	}
	return ""
}

func firstStringSlice(obj map[string]any, keys ...string) []string {
	for _, key := range keys {
		value := store.LookupFieldValue(obj, key)
		items := stringSliceValue(value)
		if len(items) > 0 {
			return items
		}
	}
	return nil
}

func firstFloatPtr(obj map[string]any, keys ...string) *float64 {
	for _, key := range keys {
		if value := store.LookupFieldValue(obj, key); value != nil {
			if number, ok := floatValue(value); ok {
				return &number
			}
		}
	}
	return nil
}

func firstInt(obj map[string]any, keys ...string) int {
	for _, key := range keys {
		if value := store.LookupFieldValue(obj, key); value != nil {
			if number, ok := floatValue(value); ok {
				return int(number)
			}
		}
	}
	return 0
}

func stringSliceValue(value any) []string {
	switch typed := value.(type) {
	case nil:
		return nil
	case []string:
		return cleanStrings(typed)
	case []any:
		items := make([]string, 0, len(typed))
		for _, item := range typed {
			items = append(items, fmt.Sprint(item))
		}
		return cleanStrings(items)
	case string:
		text := strings.TrimSpace(typed)
		if text == "" {
			return nil
		}
		if strings.HasPrefix(text, "[") {
			var items []string
			if err := json.Unmarshal([]byte(text), &items); err == nil {
				return cleanStrings(items)
			}
			var generic []any
			if err := json.Unmarshal([]byte(text), &generic); err == nil {
				items = make([]string, 0, len(generic))
				for _, item := range generic {
					items = append(items, fmt.Sprint(item))
				}
				return cleanStrings(items)
			}
		}
		return cleanStrings(strings.Split(text, ","))
	default:
		return cleanStrings([]string{fmt.Sprint(value)})
	}
}

func cleanStrings(values []string) []string {
	cleaned := make([]string, 0, len(values))
	for _, value := range values {
		text := strings.TrimSpace(value)
		if text != "" {
			cleaned = append(cleaned, text)
		}
	}
	return cleaned
}

func floatValue(value any) (float64, bool) {
	switch typed := value.(type) {
	case json.Number:
		number, err := typed.Float64()
		return number, err == nil
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case string:
		number, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return number, err == nil
	default:
		number, err := strconv.ParseFloat(fmt.Sprint(value), 64)
		return number, err == nil
	}
}

func searchableEvidenceText(obj map[string]any) string {
	fields := []string{
		firstString(obj, "title", "name"),
		firstString(obj, "shop_name", "shop"),
		strings.Join(firstStringSlice(obj, "tags", "tag_list"), " "),
		strings.Join(firstStringSlice(obj, "keywords", "keyword", "query"), " "),
	}
	return strings.TrimSpace(strings.Join(cleanStrings(fields), " "))
}
