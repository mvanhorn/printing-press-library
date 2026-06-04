package research

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

const defaultInsightLimit = 25

// BuildNicheScore scores matching product and keyword evidence for a niche.
func BuildNicheScore(scope ResearchScope, evidence []EvidenceRecord, plan ResearchPlan) ResponseEnvelope {
	matches := rankMatchingEvidence(scope, evidence, 10, "niche")
	return insightEnvelope(scope, plan, fmt.Sprintf("Scored niche %q with %d matching signals.", scope.Value, len(matches.records)), matches.records, matches.evidence, []string{
		"Compare top signals against current listing inventory.",
		"Refresh targeted EverBee research before final product decisions.",
	})
}

// BuildKeywordClusters groups matching keyword evidence by shared terms.
func BuildKeywordClusters(scope ResearchScope, evidence []EvidenceRecord, limit int, plan ResearchPlan) ResponseEnvelope {
	if limit <= 0 {
		limit = defaultInsightLimit
	}
	matches := matchingEvidence(scope, evidence)
	clusters := map[string]*InsightRecord{}
	for _, record := range matches {
		terms := append([]string{}, record.Keywords...)
		if len(terms) == 0 {
			terms = append(terms, record.Tags...)
		}
		if len(terms) == 0 {
			terms = append(terms, opportunityLabel(record))
		}
		for _, term := range terms {
			term = strings.TrimSpace(term)
			if term == "" || !termMatchesScope(scope, term+" "+record.SearchableText) {
				continue
			}
			key := strings.ToLower(term)
			cluster := clusters[key]
			if cluster == nil {
				cluster = &InsightRecord{ID: "cluster:" + key, Label: term}
				clusters[key] = cluster
			}
			cluster.Score += 1
			cluster.EvidenceIDs = appendUnique(cluster.EvidenceIDs, record.ID)
			cluster.Reasons = appendUnique(cluster.Reasons, "shared keyword term")
		}
	}
	records := insightMapValues(clusters, limit)
	return insightEnvelope(scope, plan, fmt.Sprintf("Clustered %d keyword groups for %q.", len(records), scope.Value), records, evidenceForRecords(matches, records), []string{
		"Use clusters to group listing tags and product variants.",
		"Refresh keyword research when cluster coverage is thin.",
	})
}

// BuildTagGap ranks matching tags that can fill listing or niche gaps.
func BuildTagGap(scope ResearchScope, evidence []EvidenceRecord, limit int, plan ResearchPlan) ResponseEnvelope {
	if limit <= 0 {
		limit = defaultInsightLimit
	}
	matches := matchingEvidence(scope, evidence)
	tagScores := map[string]*InsightRecord{}
	for _, record := range matches {
		for _, tag := range record.Tags {
			tag = strings.TrimSpace(tag)
			if tag == "" {
				continue
			}
			key := strings.ToLower(tag)
			item := tagScores[key]
			if item == nil {
				item = &InsightRecord{ID: "tag:" + key, Label: tag, Reasons: []string{"appears in matching EverBee evidence"}}
				tagScores[key] = item
			}
			item.Score += 1 + metricBoost(record)
			item.EvidenceIDs = appendUnique(item.EvidenceIDs, record.ID)
		}
	}
	records := insightMapValues(tagScores, limit)
	return insightEnvelope(scope, plan, fmt.Sprintf("Found %d tag gap candidates for %q.", len(records), scope.Value), records, evidenceForRecords(matches, records), []string{
		"Compare suggested tags against the target listing tags.",
		"Prioritize tags supported by multiple product or shop records.",
	})
}

type rankedEvidence struct {
	records  []InsightRecord
	evidence []EvidenceRecord
}

func rankMatchingEvidence(scope ResearchScope, evidence []EvidenceRecord, limit int, reasonPrefix string) rankedEvidence {
	if limit <= 0 {
		limit = defaultInsightLimit
	}
	type candidate struct {
		record  EvidenceRecord
		insight InsightRecord
	}
	candidates := make([]candidate, 0, len(evidence))
	for _, record := range evidence {
		score, reasons := scoreEvidenceForScope(scope, record)
		if score <= 0 {
			continue
		}
		candidates = append(candidates, candidate{
			record: record,
			insight: InsightRecord{
				ID:          record.ID,
				Label:       opportunityLabel(record),
				Score:       score,
				Reasons:     append([]string{reasonPrefix + " match"}, reasons...),
				EvidenceIDs: []string{record.ID},
			},
		})
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].insight.Score == candidates[j].insight.Score {
			return candidates[i].insight.Label < candidates[j].insight.Label
		}
		return candidates[i].insight.Score > candidates[j].insight.Score
	})
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	ranked := rankedEvidence{
		records:  make([]InsightRecord, 0, len(candidates)),
		evidence: make([]EvidenceRecord, 0, len(candidates)),
	}
	for _, candidate := range candidates {
		ranked.records = append(ranked.records, candidate.insight)
		ranked.evidence = append(ranked.evidence, candidate.record)
	}
	return ranked
}

func matchingEvidence(scope ResearchScope, evidence []EvidenceRecord) []EvidenceRecord {
	matches := make([]EvidenceRecord, 0, len(evidence))
	for _, record := range evidence {
		score, _ := scoreEvidenceForScope(scope, record)
		if score > 0 {
			matches = append(matches, record)
		}
	}
	return matches
}

func scoreEvidenceForScope(scope ResearchScope, record EvidenceRecord) (float64, []string) {
	value := strings.ToLower(strings.TrimSpace(scope.Value))
	if value == "" {
		return 0, nil
	}
	var score float64
	reasons := []string{}
	switch scope.Kind {
	case ScopeListing:
		if strings.EqualFold(record.ListingID, scope.Value) || strings.EqualFold(record.ID, scope.Value) {
			score += 20
			reasons = append(reasons, "listing id matched")
		}
	case ScopeShop:
		if strings.EqualFold(record.ShopName, scope.Value) {
			score += 20
			reasons = append(reasons, "shop matched")
		}
	}
	tokens := tokenSet(value)
	for _, text := range []string{record.Title, record.ShopName, record.ListingID, record.SearchableText} {
		if matches := countMatches(tokens, text); matches > 0 {
			score += float64(matches * 4)
			reasons = append(reasons, "text matched")
		}
	}
	score += float64(countSliceMatches(tokens, record.Keywords) * 5)
	score += float64(countSliceMatches(tokens, record.Tags) * 3)
	score += metricBoost(record)
	if score <= metricBoost(record) {
		return 0, nil
	}
	return score, reasons
}

func termMatchesScope(scope ResearchScope, value string) bool {
	tokens := tokenSet(scope.Value)
	return countMatches(tokens, value) > 0
}

func metricBoost(record EvidenceRecord) float64 {
	var boost float64
	if record.EstimatedSales != nil {
		boost += math.Min(*record.EstimatedSales/100, 5)
	}
	if record.EstimatedRevenue != nil {
		boost += math.Min(*record.EstimatedRevenue/1000, 5)
	}
	if record.Rank > 0 {
		boost += math.Max(0, 5-float64(record.Rank)/20)
	}
	return boost
}

func insightEnvelope(scope ResearchScope, plan ResearchPlan, summary string, records []InsightRecord, evidence []EvidenceRecord, nextActions []string) ResponseEnvelope {
	coverage := opportunityCoverage(plan, evidence)
	return ResponseEnvelope{
		Scope:       scope,
		DataSource:  plan.DataSource,
		Freshness:   plan.Freshness,
		Summary:     summary,
		Records:     records,
		Evidence:    evidence,
		Confidence:  confidenceFromCoverage(records, evidence, coverage),
		Coverage:    coverage,
		Warnings:    append([]string{}, plan.Warnings...),
		NextActions: nextActions,
	}
}

func confidenceFromCoverage(records []InsightRecord, evidence []EvidenceRecord, coverage Coverage) float64 {
	if len(records) == 0 || len(evidence) == 0 {
		return 0
	}
	if coverage.RawRecordCount > 0 {
		return math.Round(ConfidenceForCoverage(coverage)*100) / 100
	}
	confidence := 0.4 + math.Min(0.5, float64(len(records))/float64(len(evidence))*0.5)
	return math.Round(math.Min(1, confidence)*100) / 100
}

func insightMapValues(values map[string]*InsightRecord, limit int) []InsightRecord {
	records := make([]InsightRecord, 0, len(values))
	for _, value := range values {
		records = append(records, *value)
	}
	sort.SliceStable(records, func(i, j int) bool {
		if records[i].Score == records[j].Score {
			return records[i].Label < records[j].Label
		}
		return records[i].Score > records[j].Score
	})
	if len(records) > limit {
		return records[:limit]
	}
	return records
}

func evidenceForRecords(evidence []EvidenceRecord, records []InsightRecord) []EvidenceRecord {
	ids := map[string]struct{}{}
	for _, record := range records {
		for _, id := range record.EvidenceIDs {
			ids[id] = struct{}{}
		}
	}
	filtered := make([]EvidenceRecord, 0, len(ids))
	for _, record := range evidence {
		if _, ok := ids[record.ID]; ok {
			filtered = append(filtered, record)
		}
	}
	return filtered
}

func appendUnique(values []string, value string) []string {
	if value == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}
