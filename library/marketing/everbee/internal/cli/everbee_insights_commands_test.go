package cli

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/everbee/internal/research"
	"github.com/mvanhorn/printing-press-library/library/marketing/everbee/internal/store"
)

func TestOpportunityShortlistCommandPrintsResearchEnvelope(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	cmd := RootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"opportunity", "shortlist", "--query", "teacher gift", "--no-refresh", "--agent"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out.String())
	}

	for _, field := range []string{"scope", "data_source", "freshness", "summary", "records", "evidence", "confidence", "coverage", "warnings", "next_actions"} {
		if _, ok := payload[field]; !ok {
			t.Fatalf("missing field %q in payload: %s", field, out.String())
		}
	}
	for _, field := range []string{"records", "evidence", "warnings", "next_actions"} {
		if _, ok := payload[field].([]any); !ok {
			t.Fatalf("field %q is not an array in payload: %s", field, out.String())
		}
	}
}

func TestOpportunityShortlistCommandUsesLocalSnapshotEvidence(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ctx := t.Context()
	db, err := store.OpenWithContext(ctx, defaultDBPath("github.com/mvanhorn/printing-press-library/library/marketing/everbee"))
	if err != nil {
		t.Fatalf("OpenWithContext() error = %v", err)
	}
	snapshotStore := research.NewSnapshotStore(db)
	scope := research.ResearchScope{Kind: research.ScopeQuery, Value: "teacher gift"}
	if err := snapshotStore.Save(ctx, research.Snapshot{
		Scope:     scope,
		Resources: []string{"product_analytics"},
		FetchedAt: time.Now().UTC(),
		FreshFor:  6 * time.Hour,
		Evidence: []research.EvidenceRecord{
			{ID: "p1", Resource: "product_analytics", Title: "Teacher Gift Mug", SearchableText: "teacher gift mug"},
			{ID: "p2", Resource: "product_analytics", Title: "Wedding Sign", SearchableText: "wedding sign"},
		},
		Coverage: research.Coverage{
			ResourceCounts:      map[string]int{"product_analytics": 2},
			EvidenceRecordCount: 2,
		},
	}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	cmd := RootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"opportunity", "shortlist", "--query", "teacher gift", "--no-refresh", "--agent"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var payload struct {
		DataSource string `json:"data_source"`
		Records    []struct {
			ID string `json:"id"`
		} `json:"records"`
		Evidence []struct {
			ID string `json:"id"`
		} `json:"evidence"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out.String())
	}
	if payload.DataSource != string(research.DataSourceLocal) {
		t.Fatalf("data_source = %s, want local", payload.DataSource)
	}
	if len(payload.Records) != 1 || payload.Records[0].ID != "p1" {
		t.Fatalf("records = %+v, want p1 only", payload.Records)
	}
	if len(payload.Evidence) != 1 || payload.Evidence[0].ID != "p1" {
		t.Fatalf("evidence = %+v, want p1 only", payload.Evidence)
	}
}

func TestOpportunityEvidenceAvoidsAmbiguousMixedResourceRawFallback(t *testing.T) {
	raw := json.RawMessage(`{"id":"p1","title":"Teacher Gift Mug"}`)
	evidence, warnings := opportunityEvidence(research.Snapshot{
		Resources:  []string{"product_analytics", "keyword_research"},
		RawRecords: []json.RawMessage{raw},
	})

	if len(evidence) != 0 {
		t.Fatalf("evidence = %d, want 0 for ambiguous mixed-resource raw fallback", len(evidence))
	}
	if len(warnings) != 1 {
		t.Fatalf("warnings = %d, want 1 for ambiguous mixed-resource raw fallback", len(warnings))
	}
}

func TestNicheScoreCommandUsesLocalSnapshotEvidence(t *testing.T) {
	scope := research.ResearchScope{Kind: research.ScopeKeyword, Value: "teacher gift"}
	assertResearchCommandUsesLocalEvidence(t, scope, []string{"niche", "score", "--keyword", "teacher gift", "--no-refresh", "--agent"}, []research.EvidenceRecord{
		{ID: "p1", Resource: "product_analytics", Title: "Teacher Gift Mug", Keywords: []string{"teacher gift"}, SearchableText: "teacher gift mug"},
		{ID: "p2", Resource: "product_analytics", Title: "Wedding Sign", Keywords: []string{"wedding sign"}, SearchableText: "wedding sign"},
	}, "p1")
}

func TestKeywordsClusterCommandUsesLocalSnapshotEvidence(t *testing.T) {
	scope := research.ResearchScope{Kind: research.ScopeKeyword, Value: "teacher gift"}
	assertResearchCommandUsesLocalEvidence(t, scope, []string{"keywords", "cluster", "--seed", "teacher gift", "--no-refresh", "--agent"}, []research.EvidenceRecord{
		{ID: "k1", Resource: "keyword_research", Keywords: []string{"teacher gift mug"}, SearchableText: "teacher gift keyword"},
		{ID: "k2", Resource: "keyword_research", Keywords: []string{"wedding sign"}, SearchableText: "wedding sign keyword"},
	}, "k1")
}

func TestTagsGapCommandUsesLocalSnapshotEvidence(t *testing.T) {
	scope := research.ResearchScope{Kind: research.ScopeQuery, Value: "teacher gift"}
	assertResearchCommandUsesLocalEvidence(t, scope, []string{"tags", "gap", "--query", "teacher gift", "--no-refresh", "--agent"}, []research.EvidenceRecord{
		{ID: "t1", Resource: "product_analytics", Title: "Teacher Gift Mug", Tags: []string{"teacher gift"}, SearchableText: "teacher gift mug"},
		{ID: "t2", Resource: "product_analytics", Title: "Wedding Sign", Tags: []string{"wedding"}, SearchableText: "wedding sign"},
	}, "t1")
}

func TestListingAuditCommandUsesLocalSnapshotEvidence(t *testing.T) {
	scope := research.ResearchScope{Kind: research.ScopeListing, Value: "123456789"}
	assertResearchCommandUsesLocalEvidence(t, scope, []string{"listing", "audit", "--listing-id", "123456789", "--no-refresh", "--agent"}, []research.EvidenceRecord{
		{ID: "p1", Resource: "product_analytics", ListingID: "123456789", Title: "Teacher Gift Mug", SearchableText: "listing 123456789 teacher gift"},
		{ID: "p2", Resource: "product_analytics", ListingID: "987654321", Title: "Wedding Sign", SearchableText: "listing 987654321 wedding"},
	}, "p1")
}

func TestShopGapsCommandUsesLocalSnapshotEvidence(t *testing.T) {
	scope := research.ResearchScope{Kind: research.ScopeShop, Value: "brightbeeshop"}
	assertResearchCommandUsesLocalEvidence(t, scope, []string{"shop", "gaps", "--shop", "brightbeeshop", "--no-refresh", "--agent"}, []research.EvidenceRecord{
		{ID: "s1", Resource: "shops", ShopName: "brightbeeshop", Title: "Teacher Gift Mug", SearchableText: "brightbeeshop teacher gift"},
		{ID: "s2", Resource: "shops", ShopName: "othershop", Title: "Wedding Sign", SearchableText: "othershop wedding"},
	}, "s1")
}

func TestCompetitorWatchCommandUsesLocalSnapshotEvidence(t *testing.T) {
	scope := research.ResearchScope{Kind: research.ScopeShop, Value: "brightbeeshop"}
	assertResearchCommandUsesLocalEvidence(t, scope, []string{"competitors", "watch", "--shop", "brightbeeshop", "--no-refresh", "--agent"}, []research.EvidenceRecord{
		{ID: "s1", Resource: "shops", ShopName: "brightbeeshop", Title: "Teacher Gift Mug", SearchableText: "brightbeeshop teacher gift"},
		{ID: "s2", Resource: "shops", ShopName: "othershop", Title: "Wedding Sign", SearchableText: "othershop wedding"},
	}, "s1")
}

func TestTrendsDiffCommandUsesLocalSnapshots(t *testing.T) {
	scope := research.ResearchScope{Kind: research.ScopeQuery, Value: "teacher gift"}
	now := time.Now().UTC()
	saveResearchSnapshots(t, []research.Snapshot{
		{Scope: scope, Resources: []string{"product_analytics"}, FetchedAt: now.Add(-24 * time.Hour), FreshFor: 6 * time.Hour, Evidence: []research.EvidenceRecord{{ID: "old", Resource: "product_analytics", Title: "Teacher Gift Mug", SearchableText: "teacher gift"}}},
		{Scope: scope, Resources: []string{"product_analytics"}, FetchedAt: now, FreshFor: 6 * time.Hour, Evidence: []research.EvidenceRecord{{ID: "new", Resource: "product_analytics", Title: "Teacher Gift Bundle", SearchableText: "teacher gift bundle"}}},
	})

	payload := executeResearchCommand(t, []string{"trends", "diff", "--query", "teacher gift", "--days", "7", "--no-refresh", "--agent"})
	assertPayloadHasEvidence(t, payload, "new")
}

func assertResearchCommandUsesLocalEvidence(t *testing.T, scope research.ResearchScope, args []string, evidence []research.EvidenceRecord, wantEvidenceID string) {
	t.Helper()
	saveResearchSnapshots(t, []research.Snapshot{{
		Scope:     scope,
		Resources: []string{"product_analytics", "keyword_research", "shops"},
		FetchedAt: time.Now().UTC(),
		FreshFor:  6 * time.Hour,
		Evidence:  evidence,
		Coverage: research.Coverage{
			ResourceCounts:      map[string]int{"product_analytics": len(evidence)},
			EvidenceRecordCount: len(evidence),
		},
	}})
	payload := executeResearchCommand(t, args)
	assertPayloadHasEvidence(t, payload, wantEvidenceID)
}

func saveResearchSnapshots(t *testing.T, snapshots []research.Snapshot) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	ctx := t.Context()
	db, err := store.OpenWithContext(ctx, defaultDBPath("github.com/mvanhorn/printing-press-library/library/marketing/everbee"))
	if err != nil {
		t.Fatalf("OpenWithContext() error = %v", err)
	}
	defer db.Close()
	snapshotStore := research.NewSnapshotStore(db)
	for _, snapshot := range snapshots {
		if err := snapshotStore.Save(ctx, snapshot); err != nil {
			t.Fatalf("Save() error = %v", err)
		}
	}
}

func executeResearchCommand(t *testing.T, args []string) map[string]any {
	t.Helper()
	cmd := RootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out.String())
	}
	for _, field := range []string{"scope", "data_source", "freshness", "summary", "records", "evidence", "confidence", "coverage", "warnings", "next_actions"} {
		if _, ok := payload[field]; !ok {
			t.Fatalf("missing field %q in payload: %s", field, out.String())
		}
	}
	return payload
}

func assertPayloadHasEvidence(t *testing.T, payload map[string]any, wantEvidenceID string) {
	t.Helper()
	evidence, ok := payload["evidence"].([]any)
	if !ok {
		t.Fatalf("evidence has type %T, want array", payload["evidence"])
	}
	if len(evidence) != 1 {
		t.Fatalf("evidence = %d, want 1", len(evidence))
	}
	record, ok := evidence[0].(map[string]any)
	if !ok {
		t.Fatalf("evidence record has type %T, want object", evidence[0])
	}
	if record["id"] != wantEvidenceID {
		t.Fatalf("evidence id = %v, want %s", record["id"], wantEvidenceID)
	}
}
