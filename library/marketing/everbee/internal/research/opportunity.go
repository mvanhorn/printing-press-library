package research

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode"
)

const defaultOpportunityLimit = 25

// BuildOpportunityShortlist ranks evidence records that match the query scope.
func BuildOpportunityShortlist(scope ResearchScope, evidence []EvidenceRecord, limit int, plan ResearchPlan) ResponseEnvelope {
	if limit <= 0 {
		limit = defaultOpportunityLimit
	}

	queryTokens := tokenSet(scope.Value)
	ranked := make([]opportunityCandidate, 0, len(evidence))
	for _, record := range evidence {
		score, reasons := scoreOpportunity(record, queryTokens)
		if score <= 0 {
			continue
		}
		ranked = append(ranked, opportunityCandidate{
			record:  record,
			insight: InsightRecord{ID: record.ID, Label: opportunityLabel(record), Score: score, Reasons: reasons, EvidenceIDs: []string{record.ID}},
		})
	}

	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].insight.Score == ranked[j].insight.Score {
			if ranked[i].insight.Label == ranked[j].insight.Label {
				return ranked[i].insight.ID < ranked[j].insight.ID
			}
			return ranked[i].insight.Label < ranked[j].insight.Label
		}
		return ranked[i].insight.Score > ranked[j].insight.Score
	})
	if len(ranked) > limit {
		ranked = ranked[:limit]
	}

	records := make([]InsightRecord, 0, len(ranked))
	matchingEvidence := make([]EvidenceRecord, 0, len(ranked))
	for _, candidate := range ranked {
		records = append(records, candidate.insight)
		matchingEvidence = append(matchingEvidence, candidate.record)
	}

	return ResponseEnvelope{
		Scope:       scope,
		DataSource:  plan.DataSource,
		Freshness:   plan.Freshness,
		Summary:     opportunitySummary(scope, len(records)),
		Records:     records,
		Evidence:    matchingEvidence,
		Confidence:  opportunityConfidence(records, evidence),
		Coverage:    opportunityCoverage(plan, evidence),
		Warnings:    append([]string{}, plan.Warnings...),
		NextActions: opportunityNextActions(),
	}
}

type opportunityCandidate struct {
	record  EvidenceRecord
	insight InsightRecord
}

func scoreOpportunity(record EvidenceRecord, queryTokens map[string]struct{}) (float64, []string) {
	if len(queryTokens) == 0 {
		return 0, nil
	}

	var score float64
	var reasons []string
	if matches := countMatches(queryTokens, record.Title); matches > 0 {
		score += float64(matches) * 6
		reasons = append(reasons, "title matches query")
	}
	if matches := countSliceMatches(queryTokens, record.Tags); matches > 0 {
		score += float64(matches) * 4
		reasons = append(reasons, "tags match query")
	}
	if matches := countSliceMatches(queryTokens, record.Keywords); matches > 0 {
		score += float64(matches) * 4
		reasons = append(reasons, "keywords match query")
	}
	if matches := countMatches(queryTokens, record.SearchableText); matches > 0 {
		score += float64(matches) * 2
		reasons = append(reasons, "search text matches query")
	}
	if len(reasons) == 0 {
		return 0, nil
	}

	if record.EstimatedSales != nil && *record.EstimatedSales > 0 {
		score += math.Log1p(*record.EstimatedSales)
		reasons = append(reasons, "estimated sales support demand")
	}
	if record.EstimatedRevenue != nil && *record.EstimatedRevenue > 0 {
		score += math.Log1p(*record.EstimatedRevenue) / 2
		reasons = append(reasons, "estimated revenue support demand")
	}
	if record.Rank > 0 {
		score += 10 / float64(record.Rank+1)
		reasons = append(reasons, "ranking position supports relevance")
	}
	return math.Round(score*100) / 100, reasons
}

func opportunityLabel(record EvidenceRecord) string {
	if strings.TrimSpace(record.Title) != "" {
		return record.Title
	}
	if strings.TrimSpace(record.ListingID) != "" {
		return record.ListingID
	}
	if strings.TrimSpace(record.ID) != "" {
		return record.ID
	}
	return "untitled opportunity"
}

func opportunitySummary(scope ResearchScope, count int) string {
	if count == 0 {
		return fmt.Sprintf("No matching opportunity evidence found for %q.", scope.Value)
	}
	if count == 1 {
		return fmt.Sprintf("Ranked 1 opportunity for %q.", scope.Value)
	}
	return fmt.Sprintf("Ranked %d opportunities for %q.", count, scope.Value)
}

func opportunityConfidence(records []InsightRecord, evidence []EvidenceRecord) float64 {
	if len(records) == 0 || len(evidence) == 0 {
		return 0
	}
	coverage := float64(len(records)) / float64(len(evidence))
	topScore := records[0].Score / 40
	confidence := 0.35 + math.Min(0.35, coverage*0.35) + math.Min(0.30, topScore*0.30)
	return math.Round(math.Min(1, confidence)*100) / 100
}

func opportunityCoverage(plan ResearchPlan, evidence []EvidenceRecord) Coverage {
	if plan.Snapshot != nil && plan.Snapshot.Coverage.EvidenceRecordCount > 0 {
		return plan.Snapshot.Coverage
	}
	coverage := Coverage{
		ResourceCounts:      map[string]int{},
		EvidenceRecordCount: len(evidence),
	}
	for _, record := range evidence {
		if record.Resource != "" {
			coverage.ResourceCounts[record.Resource]++
		}
	}
	return coverage
}

func opportunityNextActions() []string {
	return []string{
		"Run niche score for the top matched query.",
		"Audit tags for the strongest matching listings.",
		"Refresh targeted EverBee data before making listing decisions if freshness is stale.",
	}
}

func countSliceMatches(queryTokens map[string]struct{}, values []string) int {
	matches := 0
	for _, value := range values {
		matches += countMatches(queryTokens, value)
	}
	return matches
}

func countMatches(queryTokens map[string]struct{}, value string) int {
	matches := 0
	for token := range tokenSet(value) {
		if _, ok := queryTokens[token]; ok {
			matches++
		}
	}
	return matches
}

func tokenSet(value string) map[string]struct{} {
	tokens := map[string]struct{}{}
	for _, token := range strings.FieldsFunc(strings.ToLower(value), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		if token != "" {
			tokens[token] = struct{}{}
		}
	}
	return tokens
}
