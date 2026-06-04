package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/everbee/internal/research"
	"github.com/mvanhorn/printing-press-library/library/marketing/everbee/internal/store"

	"github.com/spf13/cobra"
)

type researchOptions struct {
	maxAge    time.Duration
	refresh   bool
	noRefresh bool
	limit     int
	resources []string
}

func addResearchFlags(cmd *cobra.Command, opts *researchOptions) {
	cmd.Flags().DurationVar(&opts.maxAge, "max-age", 6*time.Hour, "Maximum age for local EverBee research data")
	cmd.Flags().BoolVar(&opts.refresh, "refresh", false, "Force a targeted EverBee refresh before analysis")
	cmd.Flags().BoolVar(&opts.noRefresh, "no-refresh", false, "Use local EverBee research data only")
}

type researchResult struct {
	Plan     research.ResearchPlan
	Snapshot research.Snapshot
}

type researchFetcher interface {
	Fetch(context.Context, research.ResearchScope, []string, int) (research.Snapshot, error)
}

type researchRuntime struct {
	fetcher researchFetcher
	now     func() time.Time
}

func (r researchRuntime) resolve(ctx context.Context, scope research.ResearchScope, opts researchOptions, snapshots []research.Snapshot) (researchResult, error) {
	now := time.Now().UTC()
	if r.now != nil {
		now = r.now().UTC()
	}

	usableSnapshots := usableRuntimeSnapshots(snapshots)
	planSnapshots := usableSnapshots
	if opts.refresh && !opts.noRefresh {
		planSnapshots = nil
	}
	plan := research.PlanFreshness(scope, planSnapshots, research.PlanOptions{
		Now:        now,
		MaxAge:     opts.maxAge,
		CanRefresh: r.fetcher != nil,
		NoRefresh:  opts.noRefresh,
	})

	switch plan.Decision {
	case research.DecisionUseLocal, research.DecisionFallbackLocal:
		if plan.Snapshot == nil || !snapshotHasUsableEvidence(*plan.Snapshot) {
			return researchResult{Plan: plan}, nil
		}
		return researchResult{Plan: plan, Snapshot: *plan.Snapshot}, nil
	case research.DecisionInsufficientData:
		return researchResult{Plan: plan}, nil
	case research.DecisionRefreshTargeted:
		return r.resolveRefresh(ctx, scope, opts, snapshots, plan, now)
	default:
		return researchResult{Plan: plan}, nil
	}
}

func (r researchRuntime) resolveRefresh(ctx context.Context, scope research.ResearchScope, opts researchOptions, snapshots []research.Snapshot, plan research.ResearchPlan, now time.Time) (researchResult, error) {
	snapshot, err := r.fetcher.Fetch(ctx, scope, opts.resources, opts.limit)
	if snapshotHasUsableEvidence(snapshot) {
		snapshot.Scope = scope
		if snapshot.FetchedAt.IsZero() {
			snapshot.FetchedAt = now
		}
		if snapshot.FreshFor <= 0 {
			snapshot.FreshFor = opts.maxAge
		}
		plan.Snapshot = &snapshot
		plan.DataSource = research.DataSourceRefreshed
		plan.Warnings = append(plan.Warnings, snapshot.Warnings...)
		if err != nil {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("targeted EverBee refresh persistence failed: %v", err))
		}
		plan.Freshness = research.Freshness{
			FetchedAt:     snapshot.FetchedAt,
			AgeSeconds:    0,
			MaxAgeSeconds: int64(opts.maxAge.Seconds()),
			Fresh:         true,
		}
		return researchResult{Plan: plan, Snapshot: snapshot}, nil
	}

	warnings := append([]string{}, snapshot.Warnings...)
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("targeted EverBee refresh failed: %v", err))
	} else {
		warnings = append(warnings, "targeted EverBee refresh returned no usable evidence")
	}
	return fallbackOrInsufficient(scope, snapshots, opts.maxAge, now, warnings), nil
}

func fallbackOrInsufficient(scope research.ResearchScope, snapshots []research.Snapshot, maxAge time.Duration, now time.Time, warnings []string) researchResult {
	if fallback, ok := newestRuntimeSnapshot(scope, usableRuntimeSnapshots(snapshots)); ok {
		fallbackPlan := research.PlanFreshness(scope, []research.Snapshot{fallback}, research.PlanOptions{
			Now:       now,
			MaxAge:    maxAge,
			NoRefresh: true,
		})
		fallbackPlan.Decision = research.DecisionFallbackLocal
		fallbackPlan.DataSource = research.DataSourceStaleLocalFallback
		fallbackPlan.Warnings = append(fallbackPlan.Warnings, warnings...)
		return researchResult{Plan: fallbackPlan, Snapshot: fallback}
	}

	return researchResult{Plan: research.ResearchPlan{
		Scope:      scope,
		Decision:   research.DecisionInsufficientData,
		DataSource: research.DataSourceNone,
		Warnings:   warnings,
	}}
}

func usableRuntimeSnapshots(snapshots []research.Snapshot) []research.Snapshot {
	usable := make([]research.Snapshot, 0, len(snapshots))
	for _, snapshot := range snapshots {
		if snapshotHasUsableEvidence(snapshot) {
			usable = append(usable, snapshot)
		}
	}
	return usable
}

func snapshotHasUsableEvidence(snapshot research.Snapshot) bool {
	return len(snapshot.Evidence) > 0 || snapshot.Coverage.EvidenceRecordCount > 0
}

func newestRuntimeSnapshot(scope research.ResearchScope, snapshots []research.Snapshot) (research.Snapshot, bool) {
	var newest research.Snapshot
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

type liveResearchFetcher struct {
	flags         *rootFlags
	snapshotStore *research.SnapshotStore
	maxAge        time.Duration
	now           func() time.Time
	get           func(context.Context, string, map[string]string) (json.RawMessage, error)
}

func (f liveResearchFetcher) Fetch(ctx context.Context, scope research.ResearchScope, resources []string, limit int) (research.Snapshot, error) {
	if limit <= 0 {
		limit = 25
	}
	get := f.get
	if get == nil {
		client, err := f.flags.newClient()
		if err != nil {
			return research.Snapshot{}, err
		}
		get = client.Get
	}

	now := time.Now().UTC()
	if f.now != nil {
		now = f.now().UTC()
	}
	snapshot := research.Snapshot{
		Scope:     scope,
		Resources: resources,
		FetchedAt: now,
		FreshFor:  f.maxAge,
		Coverage:  research.Coverage{ResourceCounts: map[string]int{}},
	}

	for _, resourceType := range resources {
		snapshot.Coverage.ResourceCounts[resourceType] = 0
		path, ok := researchResourcePath(resourceType)
		if !ok {
			snapshot.Warnings = append(snapshot.Warnings, "unsupported research resource: "+resourceType)
			continue
		}
		raw, err := get(ctx, path, researchParams(scope, limit))
		if err != nil {
			snapshot.Warnings = append(snapshot.Warnings, fmt.Sprintf("fetch %s failed: %v", resourceType, err))
			continue
		}
		records := extractResearchRawRecords(raw)
		evidence, coverage := research.NormalizeRecords(resourceType, records)
		snapshot.RawRecords = append(snapshot.RawRecords, records...)
		snapshot.Evidence = append(snapshot.Evidence, evidence...)
		snapshot.Coverage.RawRecordCount += coverage.RawRecordCount
		snapshot.Coverage.EvidenceRecordCount += coverage.EvidenceRecordCount
		for resource, count := range coverage.ResourceCounts {
			snapshot.Coverage.ResourceCounts[resource] += count
		}
	}

	if !snapshotHasUsableEvidence(snapshot) {
		return snapshot, fmt.Errorf("targeted EverBee refresh returned no usable evidence")
	}
	if f.snapshotStore != nil {
		if err := f.snapshotStore.Save(ctx, snapshot); err != nil {
			return snapshot, err
		}
	}
	return snapshot, nil
}

func researchResourcePath(resourceType string) (string, bool) {
	switch resourceType {
	case "product_analytics":
		return "/product_analytics/default_product_analytics", true
	case "keyword_research":
		return "/keyword_research/default_keyword_suggestion", true
	case "shops":
		return "/shops", true
	default:
		return "", false
	}
}

func researchParams(scope research.ResearchScope, limit int) map[string]string {
	params := map[string]string{"page": "1", "per_page": strconv.Itoa(limit)}
	switch scope.Kind {
	case research.ScopeQuery:
		params["q"] = scope.Value
		params["query"] = scope.Value
		params["keyword"] = scope.Value
	case research.ScopeKeyword:
		params["q"] = scope.Value
		params["query"] = scope.Value
		params["keyword"] = scope.Value
	case research.ScopeShop:
		params["shop"] = scope.Value
		params["shop_name"] = scope.Value
	case research.ScopeListing:
		params["listing_id"] = scope.Value
	}
	return params
}

func extractResearchRawRecords(raw json.RawMessage) []json.RawMessage {
	var top any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&top); err != nil {
		return nil
	}
	return rawMessagesFromValue(top)
}

func rawMessagesFromValue(value any) []json.RawMessage {
	switch typed := value.(type) {
	case []any:
		return marshalRawItems(typed)
	case map[string]any:
		for _, key := range []string{"data", "results", "items"} {
			if items, ok := typed[key].([]any); ok {
				return marshalRawItems(items)
			}
		}
		raw, err := json.Marshal(typed)
		if err != nil {
			return nil
		}
		return []json.RawMessage{raw}
	default:
		return nil
	}
}

func marshalRawItems(items []any) []json.RawMessage {
	rawRecords := make([]json.RawMessage, 0, len(items))
	for _, item := range items {
		raw, err := json.Marshal(item)
		if err == nil {
			rawRecords = append(rawRecords, raw)
		}
	}
	return rawRecords
}

func researchStoreFromDB(db *store.Store) *research.SnapshotStore {
	if db == nil {
		return nil
	}
	return research.NewSnapshotStore(db)
}
