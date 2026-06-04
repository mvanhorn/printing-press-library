package research

import (
	"fmt"
	"sort"
	"time"
)

// BuildTrendsDiff compares the newest two matching snapshots for the requested scope.
func BuildTrendsDiff(scope ResearchScope, snapshots []Snapshot, days int, plan ResearchPlan) ResponseEnvelope {
	if days <= 0 {
		days = 7
	}
	matches := matchingSnapshots(scope, snapshots)
	if len(matches) == 0 {
		return insightEnvelope(scope, plan, fmt.Sprintf("No trend snapshots found for %q.", scope.Value), nil, nil, []string{
			"Run a targeted refresh now and repeat later to build trend history.",
		})
	}
	sort.SliceStable(matches, func(i, j int) bool {
		return matches[i].FetchedAt.After(matches[j].FetchedAt)
	})
	matches = snapshotsWithinLookback(matches, days)
	if len(matches) == 1 {
		envelope := insightEnvelope(scope, plan, fmt.Sprintf("Only one trend snapshot found for %q.", scope.Value), nil, matches[0].Evidence, []string{
			"Capture another snapshot before treating trend direction as reliable.",
		})
		envelope.Confidence = 0.25
		envelope.Warnings = appendUnique(envelope.Warnings, "trend diff needs at least two snapshots")
		return envelope
	}
	newest := matches[0]
	previous := matches[1]
	latestEvidence := newest.Evidence
	record := InsightRecord{
		ID:          "trend:" + latestEvidence[0].ID,
		Label:       fmt.Sprintf("%s trend over %d days", scope.Value, days),
		Score:       float64(len(newest.Evidence) - len(previous.Evidence)),
		Reasons:     []string{fmt.Sprintf("compared %s to %s", formatSnapshotTime(previous.FetchedAt), formatSnapshotTime(newest.FetchedAt))},
		EvidenceIDs: []string{latestEvidence[0].ID},
	}
	return insightEnvelope(scope, plan, fmt.Sprintf("Compared two trend snapshots for %q.", scope.Value), []InsightRecord{record}, latestEvidence[:1], []string{
		"Review changed terms and competitor records before making catalog changes.",
		"Keep refreshing this scope to strengthen trend confidence.",
	})
}

func newestTrendSnapshot(scope ResearchScope, snapshots []Snapshot) (Snapshot, bool) {
	matches := matchingSnapshots(scope, snapshots)
	if len(matches) == 0 {
		return Snapshot{}, false
	}
	sort.SliceStable(matches, func(i, j int) bool {
		return matches[i].FetchedAt.After(matches[j].FetchedAt)
	})
	return matches[0], true
}

func snapshotsWithinLookback(snapshots []Snapshot, days int) []Snapshot {
	if len(snapshots) == 0 || snapshots[0].FetchedAt.IsZero() {
		return snapshots
	}
	cutoff := snapshots[0].FetchedAt.Add(-time.Duration(days) * 24 * time.Hour)
	filtered := make([]Snapshot, 0, len(snapshots))
	for _, snapshot := range snapshots {
		if snapshot.FetchedAt.IsZero() || !snapshot.FetchedAt.Before(cutoff) {
			filtered = append(filtered, snapshot)
		}
	}
	return filtered
}

func matchingSnapshots(scope ResearchScope, snapshots []Snapshot) []Snapshot {
	matches := make([]Snapshot, 0, len(snapshots))
	for _, snapshot := range snapshots {
		if snapshot.Scope != scope {
			continue
		}
		evidence := matchingEvidence(scope, snapshot.Evidence)
		if len(evidence) == 0 {
			continue
		}
		snapshot.Evidence = evidence
		matches = append(matches, snapshot)
	}
	return matches
}

func formatSnapshotTime(value time.Time) string {
	if value.IsZero() {
		return "unknown"
	}
	return value.Format("2006-01-02")
}
