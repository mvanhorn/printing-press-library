package research

import "time"

func PlanFreshness(scope ResearchScope, snapshots []Snapshot, opts PlanOptions) ResearchPlan {
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}

	matchingSnapshot, found := newestMatchingSnapshot(scope, snapshots)
	if !found {
		if opts.CanRefresh && !opts.NoRefresh {
			return ResearchPlan{
				Scope:      scope,
				Decision:   DecisionRefreshTargeted,
				DataSource: DataSourceRefreshed,
			}
		}

		return ResearchPlan{
			Scope:      scope,
			Decision:   DecisionInsufficientData,
			DataSource: DataSourceNone,
		}
	}

	freshness := snapshotFreshness(matchingSnapshot, now, opts.MaxAge)
	if freshness.Fresh {
		return ResearchPlan{
			Scope:      scope,
			Decision:   DecisionUseLocal,
			DataSource: DataSourceLocal,
			Snapshot:   &matchingSnapshot,
			Freshness:  freshness,
		}
	}

	if opts.CanRefresh && !opts.NoRefresh {
		return ResearchPlan{
			Scope:      scope,
			Decision:   DecisionRefreshTargeted,
			DataSource: DataSourceRefreshed,
			Snapshot:   &matchingSnapshot,
			Freshness:  freshness,
		}
	}

	return ResearchPlan{
		Scope:      scope,
		Decision:   DecisionFallbackLocal,
		DataSource: DataSourceStaleLocalFallback,
		Snapshot:   &matchingSnapshot,
		Freshness:  freshness,
		Warnings:   []string{"using stale local research because refresh is unavailable"},
	}
}

func newestMatchingSnapshot(scope ResearchScope, snapshots []Snapshot) (Snapshot, bool) {
	var newest Snapshot
	found := false

	for _, snapshot := range snapshots {
		if snapshot.Scope != scope {
			continue
		}
		if !found || snapshot.FetchedAt.After(newest.FetchedAt) {
			newest = snapshot
			found = true
		}
	}

	return newest, found
}

func snapshotFreshness(snapshot Snapshot, now time.Time, maxAge time.Duration) Freshness {
	if maxAge <= 0 && snapshot.FreshFor > 0 {
		maxAge = snapshot.FreshFor
	}

	age := now.Sub(snapshot.FetchedAt)
	if age < 0 {
		age = 0
	}

	return Freshness{
		FetchedAt:     snapshot.FetchedAt,
		AgeSeconds:    int64(age.Seconds()),
		MaxAgeSeconds: int64(maxAge.Seconds()),
		Fresh:         maxAge <= 0 || age <= maxAge,
	}
}
