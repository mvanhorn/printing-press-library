package research

import "fmt"

// BuildShopGaps ranks matching shop evidence that may reveal assortment or tag gaps.
func BuildShopGaps(scope ResearchScope, evidence []EvidenceRecord, plan ResearchPlan) ResponseEnvelope {
	matches := rankMatchingEvidence(scope, evidence, defaultInsightLimit, "shop gap")
	return insightEnvelope(scope, plan, fmt.Sprintf("Found %d shop gap signals for %q.", len(matches.records), scope.Value), matches.records, matches.evidence, []string{
		"Compare matching shop signals against your current assortment.",
		"Refresh shop and product research before changing catalog priorities.",
	})
}

// BuildCompetitorWatch summarizes the newest matching competitor snapshot.
func BuildCompetitorWatch(scope ResearchScope, snapshots []Snapshot, plan ResearchPlan) ResponseEnvelope {
	snapshot, ok := newestTrendSnapshot(scope, snapshots)
	if !ok {
		return insightEnvelope(scope, plan, fmt.Sprintf("No competitor evidence found for %q.", scope.Value), nil, nil, []string{
			"Run a targeted competitor refresh for this shop.",
		})
	}
	matches := rankMatchingEvidence(scope, snapshot.Evidence, defaultInsightLimit, "competitor watch")
	return insightEnvelope(scope, plan, fmt.Sprintf("Watched competitor %q with %d current signals.", scope.Value, len(matches.records)), matches.records, matches.evidence, []string{
		"Compare newest competitor tags, products, and pricing against earlier snapshots.",
		"Refresh competitor research on a schedule to build trend history.",
	})
}
