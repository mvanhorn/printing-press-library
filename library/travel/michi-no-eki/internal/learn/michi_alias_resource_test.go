package learn

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/michi-no-eki/internal/learn/patterns"
)

func TestMichiAliasRecallRespectsCachedResourceIdentity(t *testing.T) {
	for _, tc := range []struct {
		name, payload, wantMatch, query        string
		wantFound, wantCrossAlias, wantMissing bool
	}{
		{"conflicting cached resource", `{"name":"Beta"}`, EntityMatchMismatch, "A1 widget today", false, false, false},
		{"cached true alias", `{"name":"Alpha"}`, EntityMatchExact, "A1 widget today", true, true, false},
		{"missing cached resource", "", EntityMatchExact, "A1 widget today", true, true, true},
		{"unknown cached identity", `{}`, EntityMatchPartial, "A1 widget today", true, false, false},
		{"ambiguous query cannot switch teaching meaning", `{"name":"Beta"}`, EntityMatchMismatch, "Z1 widget today", false, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := openRecallCanonicalTestDB(t)
			seedCanonicalLookup(t, db, "alpha_kind", "Alpha-Widget-Canonical", []string{"Alpha", "A1", "Z1"})
			seedCanonicalLookup(t, db, "beta_kind", "Beta-Widget-Canonical", []string{"Beta", "Z1"})
			seedCanonicalLearning(t, db, "alpha widget today", `["Alpha"]`, "resource-1", "widgets")
			if tc.payload != "" {
				if _, err := db.Exec(`INSERT INTO resources(resource_type,id,data) VALUES('widgets','resource-1',?)`, tc.payload); err != nil {
					t.Fatal(err)
				}
			}
			got, err := Recall(context.Background(), db, tc.query, Opts{EntityConfig: canonicalTestConfig(), ResourceTypeFields: map[string][]string{"widgets": {"name"}}, DebugMismatches: true})
			if err != nil {
				t.Fatal(err)
			}
			if got.Found != tc.wantFound {
				t.Fatalf("found=%v want %v; result=%+v", got.Found, tc.wantFound, got)
			}
			var hit Hit
			if tc.wantFound {
				if len(got.Results) != 1 || len(got.Mismatches) != 0 {
					t.Fatalf("unexpected results/mismatches: %+v", got)
				}
				hit = got.Results[0]
			} else {
				if len(got.Results) != 0 || len(got.Mismatches) != 1 {
					t.Fatalf("conflicting cached identity was admitted: %+v", got)
				}
				hit = got.Mismatches[0]
			}
			if hit.EntityMatch != tc.wantMatch {
				t.Fatalf("match=%q want %q; hit=%+v", hit.EntityMatch, tc.wantMatch, hit)
			}
			warnings := strings.Join(hit.Warnings, " ")
			if strings.Contains(warnings, WarningCrossAliasMatch) != tc.wantCrossAlias {
				t.Fatalf("cross alias warning: %+v", hit)
			}
			if strings.Contains(warnings, WarningResourceNotInStore) != tc.wantMissing {
				t.Fatalf("missing resource warning: %+v", hit)
			}
		})
	}
}

func TestMichiPatternRecallRespectsCachedResourceIdentity(t *testing.T) {
	for _, tc := range []struct {
		name, query, payload, resourceID string
		withTeaching, wantFound          bool
	}{
		{"pattern conflicting cached resource", "find Alpha details", `{"name":"Beta"}`, "resource-alpha", false, false},
		{"true cached pattern alias", "find A1 details", `{"name":"Alpha"}`, "resource-a1", false, true},
		{"unknown identity keeps verified identifier", "find Alpha details", `{}`, "resource-alpha", false, true},
		{"pattern cannot re-admit conflicting teaching", "find Alpha details", `{"name":"Beta"}`, "resource-alpha", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := openRecallCanonicalTestDB(t)
			seedCanonicalLookup(t, db, "alpha_kind", "Alpha-Widget-Canonical", []string{"Alpha", "A1"})
			seedCanonicalLookup(t, db, "beta_kind", "Beta-Widget-Canonical", []string{"Beta"})
			if _, err := db.Exec(`INSERT INTO resources(resource_type,id,data) VALUES('widgets',?,?)`, tc.resourceID, tc.payload); err != nil {
				t.Fatal(err)
			}
			_, _, err := patterns.Upsert(db, patterns.Pattern{QueryTemplate: "find {entity} details", ResourceTemplate: "resource-{entity:lowercase}", ResourceType: "widgets", Strategy: patterns.StrategySubstitute, EntityKind: "lowercase", Source: patterns.SourceTaught})
			if err != nil {
				t.Fatal(err)
			}
			if tc.withTeaching {
				seedCanonicalLearning(t, db, "find alpha details", `["Alpha"]`, tc.resourceID, "widgets")
			}
			got, err := Recall(context.Background(), db, tc.query, Opts{EntityConfig: canonicalTestConfig(), ResourceTypeFields: map[string][]string{"widgets": {"name"}}, DebugMismatches: true})
			if err != nil {
				t.Fatal(err)
			}
			if got.Found != tc.wantFound {
				t.Fatalf("found=%v want %v; result=%+v", got.Found, tc.wantFound, got)
			}
			if tc.wantFound {
				if len(got.Results) != 1 || got.Results[0].Source != SourcePattern || got.Results[0].EntityMatch != EntityMatchExact {
					t.Fatalf("valid pattern contract regressed: %+v", got)
				}
			} else {
				if len(got.Results) != 0 || len(got.Mismatches) == 0 {
					t.Fatalf("known resource mismatch must be surfaced: %+v", got)
				}
				for _, h := range got.Mismatches {
					if h.EntityMatch != EntityMatchMismatch {
						t.Fatalf("false mismatch evidence: %+v", h)
					}
				}
			}
		})
	}
}

func TestMichiPatternUsesOnlyItsActualBoundEntity(t *testing.T) {
	for _, tc := range []struct {
		name, query, payload, id string
		wantFound                bool
	}{
		{"unrelated alias cannot promote", "find Alpha B1 details", `{"name":"Beta"}`, "resource-alpha", false},
		{"unrelated literal cannot match", "find Alpha Beta details", `{"name":"Beta"}`, "resource-alpha", false},
		{"true bound literal still matches", "find Alpha B1 details", `{"name":"Alpha"}`, "resource-alpha", true},
		{"true bound alias still matches", "find A1 Beta details", `{"name":"Alpha"}`, "resource-a1", true},
		{"unknown identity retains verified ID", "find Alpha B1 details", `{}`, "resource-alpha", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := openRecallCanonicalTestDB(t)
			seedCanonicalLookup(t, db, "alpha_kind", "Alpha-Widget-Canonical", []string{"Alpha", "A1"})
			seedCanonicalLookup(t, db, "beta_kind", "Beta-Widget-Canonical", []string{"Beta", "B1"})
			if _, err := db.Exec(`INSERT INTO resources(resource_type,id,data) VALUES('widgets',?,?)`, tc.id, tc.payload); err != nil {
				t.Fatal(err)
			}
			if _, _, err := patterns.Upsert(db, patterns.Pattern{QueryTemplate: "find {entity} details", ResourceTemplate: "resource-{entity:lowercase}", ResourceType: "widgets", Strategy: patterns.StrategySubstitute, EntityKind: "lowercase", Source: patterns.SourceTaught}); err != nil {
				t.Fatal(err)
			}
			got, err := Recall(context.Background(), db, tc.query, Opts{EntityConfig: canonicalTestConfig(), ResourceTypeFields: map[string][]string{"widgets": {"name"}}, DebugMismatches: true})
			if err != nil {
				t.Fatal(err)
			}
			if got.Found != tc.wantFound {
				t.Fatalf("wrong bound entity result: %+v", got)
			}
			if tc.wantFound {
				if len(got.Results) != 1 || got.Results[0].Source != SourcePattern || got.Results[0].EntityMatch != EntityMatchExact {
					t.Fatalf("valid bound entity regressed: %+v", got)
				}
			} else if len(got.Results) != 0 || len(got.Mismatches) != 1 || got.Mismatches[0].EntityMatch != EntityMatchMismatch {
				t.Fatalf("unrelated query entity admitted conflicting bound resource: %+v", got)
			}
		})
	}
}

func TestMichiRejectedPatternDoesNotHideLaterValidBinding(t *testing.T) {
	for _, invalidFirst := range []bool{true, false} {
		for _, resultLimit := range []int{1, 10} {
			t.Run(map[bool]string{true: "invalid-first", false: "valid-first"}[invalidFirst]+map[int]string{1: "/limit-one", 10: "/default"}[resultLimit], func(t *testing.T) {
				db := openRecallCanonicalTestDB(t)
				if _, err := db.Exec(`INSERT INTO resources(resource_type,id,data) VALUES('widgets','resource-target','{"name":"Beta"}')`); err != nil {
					t.Fatal(err)
				}
				seedCanonicalLookup(t, db, "alpha-map", "Alpha", []string{"target"})
				seedCanonicalLookup(t, db, "beta-map", "Beta", []string{"target"})
				patternsToInsert := []patterns.Pattern{
					{QueryTemplate: "find {entity} details", ResourceTemplate: "resource-{entity:alpha-map}", ResourceType: "widgets", Strategy: patterns.StrategySubstitute, EntityKind: "alpha-map", Source: patterns.SourceTaught},
					{QueryTemplate: "details {entity} find", ResourceTemplate: "resource-{entity:beta-map}", ResourceType: "widgets", Strategy: patterns.StrategySubstitute, EntityKind: "beta-map", Source: patterns.SourceTaught},
				}
				for _, pattern := range patternsToInsert {
					if _, _, err := patterns.Upsert(db, pattern); err != nil {
						t.Fatal(err)
					}
				}
				alphaTime, betaTime := "2026-01-01 00:00:00", "2026-01-02 00:00:00"
				if invalidFirst {
					alphaTime, betaTime = betaTime, alphaTime
				}
				if _, err := db.Exec(`UPDATE search_patterns SET last_observed_at=CASE entity_kind WHEN 'alpha-map' THEN ? ELSE ? END`, alphaTime, betaTime); err != nil {
					t.Fatal(err)
				}
				got, err := Recall(context.Background(), db, "find Alpha Beta details", Opts{EntityConfig: canonicalTestConfig(), ResourceTypeFields: map[string][]string{"widgets": {"name"}}, PatternKinds: []string{"alpha-map", "beta-map"}, DebugMismatches: true, Limit: resultLimit})
				if err != nil {
					t.Fatal(err)
				}
				if !got.Found || len(got.Results) != 1 || got.Results[0].ResourceID != "resource-target" || got.Results[0].EntityMatch != EntityMatchExact {
					t.Fatalf("rejected pattern hid later valid binding: %+v", got)
				}
			})
		}
	}
}

func TestMichiAcceptedPatternHitsRemainDeduplicated(t *testing.T) {
	db := openRecallCanonicalTestDB(t)
	if _, err := db.Exec(`INSERT INTO resources(resource_type,id,data) VALUES('widgets','resource-beta','{"name":"Beta"}')`); err != nil {
		t.Fatal(err)
	}
	for _, template := range []string{"find {entity} details", "details {entity} find"} {
		if _, _, err := patterns.Upsert(db, patterns.Pattern{QueryTemplate: template, ResourceTemplate: "resource-{entity:lowercase}", ResourceType: "widgets", Strategy: patterns.StrategySubstitute, EntityKind: "lowercase", Source: patterns.SourceTaught}); err != nil {
			t.Fatal(err)
		}
	}
	got, err := Recall(context.Background(), db, "find Beta details", Opts{EntityConfig: canonicalTestConfig(), ResourceTypeFields: map[string][]string{"widgets": {"name"}}})
	if err != nil {
		t.Fatal(err)
	}
	if !got.Found || len(got.Results) != 1 {
		t.Fatalf("accepted duplicate pattern IDs leaked: %+v", got)
	}
}

func TestMichiStandalonePatternApplyRetainsCandidateCaps(t *testing.T) {
	db := openRecallCanonicalTestDB(t)
	for i := 0; i < 12; i++ {
		suffix := strconv.Itoa(i)
		id := "resource-" + suffix + "-beta"
		if _, err := db.Exec(`INSERT INTO resources(resource_type,id,data) VALUES('widgets',?,'{"name":"Beta"}')`, id); err != nil {
			t.Fatal(err)
		}
		if _, _, err := patterns.Upsert(db, patterns.Pattern{QueryTemplate: "find {entity} details", ResourceTemplate: "resource-" + suffix + "-{entity:lowercase}", ResourceType: "widgets", Strategy: patterns.StrategySubstitute, EntityKind: "lowercase", Source: patterns.SourceTaught}); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		name string
		opts patterns.Opts
		want int
	}{{"default", patterns.Opts{}, 10}, {"explicit", patterns.Opts{Limit: 2}, 2}, {"internal filtering candidates", patterns.Opts{NoLimit: true}, 12}} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := patterns.Apply(context.Background(), db, "find Beta details", "details find", []string{"Beta"}, tc.opts)
			if err != nil || len(got) != tc.want {
				t.Fatalf("standalone candidate cap got=%d want=%d err=%v", len(got), tc.want, err)
			}
		})
	}
}
