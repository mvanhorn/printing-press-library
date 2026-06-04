package research

import "fmt"

// BuildListingAudit audits matching listing evidence for keyword and tag fit.
func BuildListingAudit(scope ResearchScope, evidence []EvidenceRecord, limit int, plan ResearchPlan) ResponseEnvelope {
	matches := rankMatchingEvidence(scope, evidence, limit, "listing audit")
	for i := range matches.records {
		matches.records[i].Reasons = appendUnique(matches.records[i].Reasons, "review title, tags, and keyword fit")
	}
	return insightEnvelope(scope, plan, fmt.Sprintf("Audited listing %q with %d matching evidence records.", scope.Value, len(matches.records)), matches.records, matches.evidence, []string{
		"Compare listing tags against keyword evidence.",
		"Refresh listing-scoped research before editing production listings.",
	})
}
