package cli

import (
	"encoding/json"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var insightTokenRE = regexp.MustCompile(`[a-z0-9]+`)

func placeholders(n int) string {
	if n <= 0 {
		return "?"
	}
	parts := make([]string, n)
	for i := range parts {
		parts[i] = "?"
	}
	return strings.Join(parts, ",")
}

func queryArgs(values []string, limit int) []any {
	args := make([]any, 0, len(values)+1)
	for _, value := range values {
		args = append(args, value)
	}
	args = append(args, limit)
	return args
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func normalizeTokens(value string) []string {
	raw := insightTokenRE.FindAllString(strings.ToLower(value), -1)
	seen := map[string]bool{}
	var out []string
	for _, token := range raw {
		if len(token) < 2 || seen[token] || isStopTerm(token) {
			continue
		}
		seen[token] = true
		out = append(out, token)
	}
	return out
}

func matchesTokens(text string, tokens []string) bool {
	if len(tokens) == 0 {
		return true
	}
	for _, token := range tokens {
		if !strings.Contains(text, token) {
			return false
		}
	}
	return true
}

func searchableRecordText(data map[string]any) string {
	var parts []string
	collectSearchableValues(data, &parts)
	return strings.ToLower(strings.Join(parts, " "))
}

func collectSearchableValues(value any, parts *[]string) {
	switch typed := value.(type) {
	case string:
		if strings.TrimSpace(typed) != "" {
			*parts = append(*parts, typed)
		}
	case []any:
		for _, item := range typed {
			collectSearchableValues(item, parts)
		}
	case map[string]any:
		for _, item := range typed {
			collectSearchableValues(item, parts)
		}
	case []string:
		for _, item := range typed {
			collectSearchableValues(item, parts)
		}
	}
}

func scoreRecord(data map[string]any, text string, tokens []string) (float64, []string) {
	var score float64
	var reasons []string
	for _, token := range tokens {
		if strings.Contains(text, token) {
			score += 10
			reasons = append(reasons, "matches "+token)
		}
	}
	for _, key := range []string{"sales", "revenue", "volume", "score", "competition", "reviews", "price"} {
		if v, ok := findNumeric(data, key); ok {
			score += math.Log10(math.Abs(v)+1) * 5
			reasons = append(reasons, "has "+key)
		}
	}
	return math.Round(score*10) / 10, reasons
}

func aggregateScore(records []everbeeRecord) float64 {
	if len(records) == 0 {
		return 0
	}
	var total float64
	for _, rec := range records {
		total += rec.Score
	}
	return total / float64(len(records))
}

func topRecords(records []everbeeRecord, limit int) []everbeeRecord {
	if limit <= 0 || len(records) <= limit {
		return records
	}
	return records[:limit]
}

func emptyInsightMessage(records []everbeeRecord) string {
	if len(records) > 0 {
		return ""
	}
	return "No matching local EverBee snapshots found. Run sync or fetch product, keyword, and shop data first."
}

func snapshotMessage(records []everbeeRecord) string {
	if len(records) > 1 {
		return ""
	}
	return "Trend and watch commands need multiple saved snapshots for true diffs. Current output shows available matching records."
}

func extractTopTerms(records []everbeeRecord, limit int) []map[string]any {
	counts := map[string]int{}
	for _, rec := range records {
		for _, token := range normalizeTokens(rec.Text) {
			if len(token) < 4 || isStopTerm(token) {
				continue
			}
			counts[token]++
		}
	}
	type pair struct {
		Term  string
		Count int
	}
	var pairs []pair
	for term, count := range counts {
		pairs = append(pairs, pair{Term: term, Count: count})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].Count == pairs[j].Count {
			return pairs[i].Term < pairs[j].Term
		}
		return pairs[i].Count > pairs[j].Count
	})
	if len(pairs) > limit {
		pairs = pairs[:limit]
	}
	out := make([]map[string]any, 0, len(pairs))
	for _, p := range pairs {
		out = append(out, map[string]any{"term": p.Term, "count": p.Count})
	}
	return out
}

func clusterRecords(records []everbeeRecord) []map[string]any {
	clusters := map[string][]string{}
	for _, rec := range records {
		for _, token := range normalizeTokens(rec.Text) {
			if len(token) < 5 || isStopTerm(token) {
				continue
			}
			clusters[token] = append(clusters[token], rec.ID)
		}
	}
	type cluster struct {
		Term string
		IDs  []string
	}
	var rows []cluster
	for term, ids := range clusters {
		if len(ids) < 1 {
			continue
		}
		rows = append(rows, cluster{Term: term, IDs: uniqueStrings(ids)})
	}
	sort.Slice(rows, func(i, j int) bool {
		if len(rows[i].IDs) == len(rows[j].IDs) {
			return rows[i].Term < rows[j].Term
		}
		return len(rows[i].IDs) > len(rows[j].IDs)
	})
	if len(rows) > 20 {
		rows = rows[:20]
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, map[string]any{"term": row.Term, "record_count": len(row.IDs), "record_ids": row.IDs})
	}
	return out
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		if seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func isStopTerm(token string) bool {
	switch token {
	case "data", "null", "true", "false", "name", "type", "shop", "etsy", "http", "https", "default", "keyword", "product",
		"opportunity", "shortlist", "score", "cluster", "watch", "audit", "diff", "gap", "gaps", "niche", "research", "search":
		return true
	default:
		return false
	}
}

func findNumeric(data map[string]any, wanted string) (float64, bool) {
	for key, value := range data {
		if strings.Contains(strings.ToLower(key), wanted) {
			if n, ok := numericValue(value); ok {
				return n, true
			}
		}
		switch nested := value.(type) {
		case map[string]any:
			if n, ok := findNumeric(nested, wanted); ok {
				return n, true
			}
		case []any:
			for _, item := range nested {
				child, ok := item.(map[string]any)
				if !ok {
					continue
				}
				if n, ok := findNumeric(child, wanted); ok {
					return n, true
				}
			}
		}
	}
	return 0, false
}

func numericValue(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	case json.Number:
		n, err := v.Float64()
		return n, err == nil
	case string:
		clean := strings.Trim(strings.ReplaceAll(v, ",", ""), "$ ")
		n, err := strconv.ParseFloat(clean, 64)
		return n, err == nil
	default:
		return 0, false
	}
}
